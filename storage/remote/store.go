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
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
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
	"github.com/onflow/flow-emulator/storage/sqlite"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils"
)

// Configuration
const (
	blockBuffer = 10 // Buffer to allow for block propagation

	// globalCacheSize is the number of register entries held in the global
	// in-memory LRU cache. Fork-mode data at the pinned height is immutable,
	// so entries never go stale. 50k entries ≈ 200-500 MB depending on value
	// sizes, which is a reasonable trade-off for eliminating repeated gRPC and
	// SQLite lookups.
	globalCacheSize = 50_000

	// rpcTimeout is the per-call deadline for a single GetRegisterValues RPC.
	// If the access node is slow or overloaded we'd rather skip a register
	// (FVM treats missing registers as empty) than block the entire emulator
	// for minutes via the retry interceptor.
	rpcTimeout = 10 * time.Second

	// batchSize is the maximum number of register IDs sent in one
	// GetRegisterValues RPC. The access-node API supports arrays, but the
	// original emulator only ever sent 1 at a time.
	batchSize = 50
)

type Store struct {
	*sqlite.Store
	executionClient executiondata.ExecutionDataAPIClient
	accessClient    access.AccessAPIClient
	grpcConn        *grpc.ClientConn
	host            string
	chainID         flowgo.ChainID
	forkHeight      uint64
	logger          *zerolog.Logger

	// globalCache is a process-wide LRU cache for register values at the fork
	// height. Because fork-height data is immutable, entries never need
	// invalidation. This eliminates redundant SQLite reads and gRPC calls
	// across snapshots / blocks.
	globalCache *lru.Cache[string, flowgo.RegisterValue]
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
	gc, err := lru.New[string, flowgo.RegisterValue](globalCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create global register cache: %w", err)
	}

	store := &Store{
		Store:       provider,
		logger:      logger,
		globalCache: gc,
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

	s.logger.Info().
		Uint64("forkHeight", s.forkHeight).
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

// getRegisterSingle fetches a single register from the remote node with a
// per-call timeout. If the call times out or the access node returns NotFound,
// the function returns an empty byte slice (FVM treats missing registers as
// empty) instead of propagating the error.
func (s *Store) getRegisterSingle(
	ctx context.Context,
	id flowgo.RegisterID,
	lookupHeight uint64,
) (flowgo.RegisterValue, error) {
	callCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	registerID := convert.RegisterIDToMessage(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
	response, err := s.executionClient.GetRegisterValues(callCtx, &executiondata.GetRegisterValuesRequest{
		BlockHeight: lookupHeight,
		RegisterIds: []*entities.RegisterID{registerID},
	})

	if err != nil {
		code := status.Code(err)
		if code == codes.NotFound || code == codes.DeadlineExceeded || code == codes.InvalidArgument {
			s.logger.Warn().
				Str("register", id.String()).
				Err(err).
				Msg("register fetch failed, returning empty")
			return []byte{}, nil
		}
		return nil, err
	}

	if response != nil && len(response.Values) > 0 {
		return response.Values[0], nil
	}
	return []byte{}, nil
}

// getRegisterBatch fetches multiple registers in a single RPC call.
// Returns a map from register key string → value. On timeout, returns
// whatever partial results are available (empty map on total failure).
func (s *Store) getRegisterBatch(
	ctx context.Context,
	ids []flowgo.RegisterID,
	lookupHeight uint64,
) map[string]flowgo.RegisterValue {
	callCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	regIDs := make([]*entities.RegisterID, len(ids))
	for i, id := range ids {
		regIDs[i] = convert.RegisterIDToMessage(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
	}

	response, err := s.executionClient.GetRegisterValues(callCtx, &executiondata.GetRegisterValuesRequest{
		BlockHeight: lookupHeight,
		RegisterIds: regIDs,
	})

	result := make(map[string]flowgo.RegisterValue, len(ids))
	if err != nil {
		s.logger.Warn().Err(err).Int("count", len(ids)).Msg("batch register fetch failed")
		return result
	}

	if response != nil {
		for i, val := range response.Values {
			if i < len(ids) {
				result[ids[i].String()] = val
			}
		}
	}
	return result
}

// resolveRegister looks up a single register through the 3-tier cache hierarchy:
// 1. Global in-memory LRU cache (fastest)
// 2. Local SQLite store (persistent across restarts)
// 3. Remote gRPC call to access node (slowest)
//
// Results from tier 3 are written back to tiers 1 and 2.
func (s *Store) resolveRegister(
	ctx context.Context,
	id flowgo.RegisterID,
	blockHeight uint64,
) (flowgo.RegisterValue, error) {
	key := id.String()

	// Fork-height state is immutable; anything above fork height can be mutated
	// by user transactions and must always prefer local storage. Only use the
	// process-wide global cache when the lookup height is immutable data.
	useGlobalCache := blockHeight <= s.forkHeight

	// Tier 1: global in-memory cache (immutable fork data only)
	if useGlobalCache {
		if val, ok := s.globalCache.Get(key); ok {
			return val, nil
		}
	}

	// Tier 2: local SQLite (contains all post-fork writes). Always check this
	// first for mutable heights so we don't return stale fork-height data from
	// the global cache.
	value, err := s.DefaultStore.GetBytesAtVersion(
		ctx,
		s.Storage(storage.LedgerStoreName),
		[]byte(key),
		blockHeight,
	)
	if err == nil && value != nil {
		if useGlobalCache {
			s.globalCache.Add(key, value)
		}
		return value, nil
	}

	// Tier 3: remote gRPC
	lookupHeight := blockHeight
	if lookupHeight > s.forkHeight {
		lookupHeight = s.forkHeight
	}

	value, err = s.getRegisterSingle(ctx, id, lookupHeight)
	if err != nil {
		return nil, err
	}

	// Write back to tier 1 + tier 2
	if lookupHeight == s.forkHeight {
		s.globalCache.Add(key, value)
	}
	_ = s.DataSetter.SetBytesWithVersion(
		ctx,
		s.Storage(storage.LedgerStoreName),
		[]byte(key),
		value,
		lookupHeight,
	)

	return value, nil
}

// prefetchRegistersForOwner fetches all common register keys for a given account
// owner in a single batched RPC call and populates both the global cache and
// SQLite. This dramatically reduces the number of sequential gRPC round-trips
// when the FVM first touches a new account.
func (s *Store) prefetchRegistersForOwner(ctx context.Context, owner string, lookupHeight uint64) {
	// Common register keys that FVM reads when touching an account.
	// These are the low-level Atree storage register patterns.
	commonKeys := []string{
		"public_key_count",
		"public_key_0",
		"public_key_1",
		"public_key_2",
		"contract_names",
		"exists",
		"storage_used",
		"storage_index",
		// Atree slab storage registers (account root slabs)
		"$\x00\x00\x00\x00\x00\x00\x00\x01",
		"$\x00\x00\x00\x00\x00\x00\x00\x02",
		"$\x00\x00\x00\x00\x00\x00\x00\x03",
		"$\x00\x00\x00\x00\x00\x00\x00\x04",
		"$\x00\x00\x00\x00\x00\x00\x00\x05",
		"$\x00\x00\x00\x00\x00\x00\x00\x06",
		"$\x00\x00\x00\x00\x00\x00\x00\x07",
		"$\x00\x00\x00\x00\x00\x00\x00\x08",
	}

	// Build register IDs, skipping any already in the global cache.
	var toFetch []flowgo.RegisterID
	for _, key := range commonKeys {
		rid := flowgo.RegisterID{Owner: owner, Key: key}
		if _, ok := s.globalCache.Get(rid.String()); !ok {
			toFetch = append(toFetch, rid)
		}
	}

	if len(toFetch) == 0 {
		return
	}

	// Batch fetch
	results := s.getRegisterBatch(ctx, toFetch, lookupHeight)

	// Populate caches
	for _, rid := range toFetch {
		key := rid.String()
		val, ok := results[key]
		if !ok {
			val = []byte{}
		}
		s.globalCache.Add(key, val)
		_ = s.DataSetter.SetBytesWithVersion(
			ctx,
			s.Storage(storage.LedgerStoreName),
			[]byte(key),
			val,
			lookupHeight,
		)
	}

	s.logger.Debug().
		Str("owner", fmt.Sprintf("%x", owner)).
		Int("fetched", len(toFetch)).
		Int("cached", len(results)).
		Msg("prefetched registers for owner")
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	// Track which owners we've already prefetched in this snapshot to avoid
	// redundant batch calls within a single block execution.
	prefetched := &sync.Map{}

	return snapshot.NewReadFuncStorageSnapshot(func(id flowgo.RegisterID) (flowgo.RegisterValue, error) {
		// Trigger prefetch for this owner if we haven't yet.
		// This runs once per unique owner per snapshot and populates the global
		// cache with common registers, so subsequent reads for the same owner
		// hit tier-1 cache.
		if id.Owner != "" {
			if _, loaded := prefetched.LoadOrStore(id.Owner, true); !loaded {
				lookupHeight := blockHeight
				if lookupHeight > s.forkHeight {
					lookupHeight = s.forkHeight
				}
				s.prefetchRegistersForOwner(ctx, id.Owner, lookupHeight)
			}
		}

		return s.resolveRegister(ctx, id, blockHeight)
	}), nil
}

func (s *Store) Stop() {
	_ = s.grpcConn.Close()
}
