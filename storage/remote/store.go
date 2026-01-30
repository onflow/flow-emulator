/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package remote

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/badgercache"
	"github.com/onflow/flow-emulator/storage/sqlite"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils"
)

// Configuration
const (
	blockBuffer = 10 // Buffer to allow for block propagation
)

type Store struct {
	*sqlite.Store
	executionClient executiondata.ExecutionDataAPIClient
	accessClient    access.AccessAPIClient
	grpcConn        *grpc.ClientConn
	host            string
	chainID         flowgo.ChainID
	forkHeight      uint64
	forkBlockID     flowgo.Identifier
	forkCacheDir    string
	logger          *zerolog.Logger
	cache           *badgercache.Cache
}

type Option func(*Store)

// WithForkHost configures the remote access/observer node gRPC endpoint.
// Expects raw host:port with no scheme.
func WithForkHost(host string) Option {
	return func(store *Store) {
		store.host = host
	}
}

// WithRPCHost sets access/observer node host. Deprecated: use WithForkHost.
func WithRPCHost(host string, chainID flowgo.ChainID) Option {
	return func(store *Store) {
		// Keep legacy behavior: set host and (optionally) chainID for validation.
		store.host = host
		store.chainID = chainID
	}
}

// WithStartBlockHeight sets the start height for the store.
// WithForkHeight sets the pinned fork height.
func WithForkHeight(height uint64) Option {
	return func(store *Store) {
		store.forkHeight = height
	}
}

// WithStartBlockHeight is deprecated: use WithForkHeight.
func WithStartBlockHeight(height uint64) Option { return WithForkHeight(height) }

// WithForkCacheDir sets the directory for fork register cache.
func WithForkCacheDir(dir string) Option {
	return func(store *Store) {
		store.forkCacheDir = dir
	}
}

// WithClient can set an rpc host client
//
// This is mostly use for testing.
func WithClient(
	executionClient executiondata.ExecutionDataAPIClient,
	accessClient access.AccessAPIClient,
) Option {
	return func(store *Store) {
		store.executionClient = executionClient
		store.accessClient = accessClient
	}
}

func New(provider *sqlite.Store, logger *zerolog.Logger, options ...Option) (*Store, error) {
	store := &Store{
		Store:  provider,
		logger: logger,
	}

	for _, opt := range options {
		opt(store)
	}

	if store.executionClient == nil {
		if store.host == "" {
			return nil, fmt.Errorf("rpc host must be provided")
		}

		conn, err := grpc.NewClient(
			store.host,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*1024)),
			utils.DefaultGRPCRetryInterceptor(),
		)
		if err != nil {
			return nil, fmt.Errorf("could not connect to rpc host: %w", err)
		}

		store.grpcConn = conn
		store.executionClient = executiondata.NewExecutionDataAPIClient(conn)
		store.accessClient = access.NewAccessAPIClient(conn)
	}

	params, err := store.accessClient.GetNetworkParameters(context.Background(), &access.GetNetworkParametersRequest{})
	if err != nil {
		return nil, fmt.Errorf("could not get network parameters: %w", err)
	}

	// If a chainID was provided (legacy path), validate it matches the remote. If not provided, skip.
	if store.chainID != "" {
		if params.ChainId != store.chainID.String() {
			return nil, fmt.Errorf("chain ID of rpc host does not match chain ID provided in config: %s != %s", params.ChainId, store.chainID)
		}
	}

	// Record remote chain ID if not already set via options
	if store.chainID == "" {
		store.chainID = flowgo.ChainID(params.ChainId)
	}

	if err := store.initializeStartBlock(context.Background()); err != nil {
		return nil, err
	}

	// Initialize fork register cache (only if not using --persist mode)
	// When forkCacheDir is explicitly set to empty string, skip Badger cache (SQLite handles caching)
	if store.forkCacheDir != "" {
		cacheConfig := badgercache.GetDefaultConfig()
		cacheConfig.BaseDir = store.forkCacheDir
		cache, err := badgercache.New(store.chainID.String(), store.forkBlockID.String(), cacheConfig, logger)
		if err != nil {
			// Warn but continue - cache is a performance optimization, not a hard requirement
			// This allows emulator to start even if cache dir has permission issues
			logger.Warn().Err(err).
				Str("cacheDir", store.forkCacheDir).
				Msg("⚠️  Failed to initialize fork cache - performance will be degraded (all reads will hit RPC)")
		} else {
			store.cache = cache
			logger.Info().
				Str("cacheDir", store.forkCacheDir).
				Str("network", store.chainID.String()).
				Str("blockID", store.forkBlockID.String()[:12]).
				Msg("✓ Fork cache enabled")
		}
	}

	store.DataGetter = store
	store.DataSetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

// initializeStartBlock initializes and stores the fork height and local latest height.
func (s *Store) initializeStartBlock(ctx context.Context) error {
	// the fork height may already be set in the db if restarting from persistent store
	forkHeight, err := s.ForkedBlockHeight(ctx)
	if err == nil && forkHeight > 0 {
		s.forkHeight = forkHeight
		blockResp, err := s.accessClient.GetBlockHeaderByHeight(ctx, &access.GetBlockHeaderByHeightRequest{Height: s.forkHeight})
		if err != nil {
			return fmt.Errorf("could not get fork block header: %w", err)
		}
		s.forkBlockID = flowgo.HashToID(blockResp.Block.Id)
		return nil
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not get forked block height: %w", err)
	}

	// use the current latest block from the rpc host if no height was provided
	if s.forkHeight == 0 {
		resp, err := s.accessClient.GetLatestBlockHeader(ctx, &access.GetLatestBlockHeaderRequest{IsSealed: true})
		if err != nil {
			return fmt.Errorf("could not get last block height: %w", err)
		}
		s.forkHeight = resp.Block.Height - blockBuffer
	}

	// Fetch the fork block header to get the block ID for cache key
	blockResp, err := s.accessClient.GetBlockHeaderByHeight(ctx, &access.GetBlockHeaderByHeightRequest{Height: s.forkHeight})
	if err != nil {
		return fmt.Errorf("could not get fork block header: %w", err)
	}
	s.forkBlockID = flowgo.HashToID(blockResp.Block.Id)

	s.logger.Info().
		Uint64("forkHeight", s.forkHeight).
		Str("forkBlockID", s.forkBlockID.String()).
		Str("host", s.host).
		Str("chainId", s.chainID.String()).
		Msg("Using fork height")

	// store the initial fork height. any future queries for data on the rpc host will be fixed
	// to this height.
	err = s.StoreForkedBlockHeight(ctx, s.forkHeight)
	if err != nil {
		return fmt.Errorf("could not set start block height: %w", err)
	}

	// initialize the local latest height.
	err = s.SetBlockHeight(s.forkHeight)
	if err != nil {
		return fmt.Errorf("could not set start block height: %w", err)
	}

	return nil
}

func (s *Store) BlockByID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.Block, error) {
	var height uint64
	block, err := s.DefaultStore.BlockByID(ctx, blockID)
	if err == nil {
		height = block.Height
	} else if errors.Is(err, storage.ErrNotFound) {
		heightRes, err := s.accessClient.GetBlockHeaderByID(ctx, &access.GetBlockHeaderByIDRequest{Id: blockID[:]})
		if err != nil {
			return nil, err
		}
		height = heightRes.Block.Height
	} else {
		return nil, err
	}

	return s.BlockByHeight(ctx, height)
}

func (s *Store) LatestBlock(ctx context.Context) (flowgo.Block, error) {
	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil {
		return flowgo.Block{}, fmt.Errorf("could not get local latest block: %w", err)
	}

	block, err := s.BlockByHeight(ctx, latestBlockHeight)
	if err != nil {
		return flowgo.Block{}, err
	}

	return *block, nil
}

func (s *Store) BlockByHeight(ctx context.Context, height uint64) (*flowgo.Block, error) {
	block, err := s.DefaultStore.BlockByHeight(ctx, height)
	if err == nil {
		return block, nil
	}

	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get local latest block: %w", err)
	}
	if height > latestBlockHeight {
		return nil, &types.BlockNotFoundByHeightError{Height: height}
	}

	blockRes, err := s.accessClient.GetBlockHeaderByHeight(ctx, &access.GetBlockHeaderByHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}

	header, err := convert.MessageToBlockHeader(blockRes.GetBlock())
	if err != nil {
		return nil, err
	}

	payload := flowgo.NewEmptyPayload()
	return &flowgo.Block{
		Payload:    *payload,
		HeaderBody: header.HeaderBody,
	}, nil
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	return snapshot.NewReadFuncStorageSnapshot(func(id flowgo.RegisterID) (flowgo.RegisterValue, error) {
		lookupHeight := blockHeight

		// CRITICAL: Check overlay (SQLite) FIRST - local mutations take precedence!
		// This ensures that any state modified by transactions is returned before cached fork data
		value, err := s.DefaultStore.GetBytesAtVersion(
			ctx,
			s.Storage(storage.LedgerStoreName),
			[]byte(id.String()),
			lookupHeight,
		)
		if err == nil && value != nil {
			return value, nil
		}

		// Check Badger fork cache SECOND - only if not found in overlay
		if s.cache != nil {
			cacheKey := []byte(id.String())
			value, err := s.cache.Get(ctx, cacheKey)
			if err == nil && value != nil {
				return value, nil
			}
		}

		// FVM expects an empty byte array if the value is not found
		value = []byte{}

		// Fetch from network
		// for consistency, always use data at the forked height for future blocks
		if lookupHeight > s.forkHeight {
			lookupHeight = s.forkHeight
		}

		registerID := convert.RegisterIDToMessage(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
		response, err := s.executionClient.GetRegisterValues(ctx, &executiondata.GetRegisterValuesRequest{
			BlockHeight: lookupHeight,
			RegisterIds: []*entities.RegisterID{registerID},
		})

		if err != nil {
			if status.Code(err) != codes.NotFound {
				return nil, err
			}
		}

		if response != nil && len(response.Values) > 0 {
			value = response.Values[0]
		}

		// Store in fork cache (mutually exclusive: Badger OR base storage fallback)
		if s.cache != nil {
			// Primary: Badger cache for fork registers (always separate from overlay)
			if err := s.cache.Set(ctx, []byte(id.String()), value); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to write to fork cache")
			}
		} else {
			// Fallback: If Badger failed to initialize, write to base storage
			// Currently this is always SQLite (enforced in server.go)
			// This ensures fork cache still works even if Badger is unavailable
			err = s.DataSetter.SetBytesWithVersion(
				ctx,
				s.Storage(storage.LedgerStoreName),
				[]byte(id.String()),
				value,
				lookupHeight)
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to write to storage cache")
			}
		}

		return value, nil
	}), nil
}
func (s *Store) Stop() {
	if s.cache != nil {
		if err := s.cache.Close(); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to close fork cache")
		}
	}
	_ = s.grpcConn.Close()
}
