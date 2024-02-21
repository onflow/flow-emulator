/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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
	"log"

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
}

type Option func(*Store)

// WithHost sets access node host.
func WithHost(host string, chainID flowgo.ChainID) Option {
	return func(store *Store) {
		store.host = host
		store.chainID = chainID
	}
}

// WithClient can set an archive node client
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

// WithStartBlockHeight sets the start height for the store.
func WithStartBlockHeight(height uint64) Option {
	return func(store *Store) {
		store.forkHeight = height
	}
}

func New(provider *sqlite.Store, options ...Option) (*Store, error) {
	store := &Store{
		Store: provider,
	}

	for _, opt := range options {
		opt(store)
	}

	if store.executionClient == nil {
		if store.host == "" {
			return nil, fmt.Errorf("archive node host must be provided")
		}

		conn, err := grpc.Dial(
			store.host,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*1024)),
		)
		if err != nil {
			return nil, fmt.Errorf("could not connect to archive node: %w", err)
		}

		store.grpcConn = conn
		store.executionClient = executiondata.NewExecutionDataAPIClient(conn)
		store.accessClient = access.NewAccessAPIClient(conn)
	}

	params, err := store.accessClient.GetNetworkParameters(context.Background(), &access.GetNetworkParametersRequest{})
	if err != nil {
		return nil, fmt.Errorf("could not get network parameters: %w", err)
	}

	if params.ChainId != store.chainID.String() {
		return nil, fmt.Errorf("chain ID of access node does not match chain ID provided in config: %s != %s", params.ChainId, store.chainID)
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
	forkHeight, err := s.Store.ForkedBlockHeight(ctx)
	if err == nil && forkHeight > 0 {
		s.forkHeight = forkHeight
		return nil
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not get forked block height: %w", err)
	}

	// use the current latest block from the archive node if no height was provided
	if s.forkHeight == 0 {
		resp, err := s.accessClient.GetLatestBlockHeader(ctx, &access.GetLatestBlockHeaderRequest{IsSealed: true})
		if err != nil {
			return fmt.Errorf("could not get last block height: %w", err)
		}
		s.forkHeight = resp.Block.Height
	}

	log.Printf("fork height: %d", s.forkHeight)

	// store the initial fork height. any future queries for data on the archive node will be fixed
	// to this height.
	err = s.Store.StoreForkedBlockHeight(ctx, s.forkHeight)
	if err != nil {
		return fmt.Errorf("could not set start block height: %w", err)
	}

	// initialize the local latest height.
	err = s.Store.SetBlockHeight(s.forkHeight)
	if err != nil {
		return fmt.Errorf("could not set start block height: %w", err)
	}

	return nil
}

func (s *Store) BlockByID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.Block, error) {
	var height uint64
	block, err := s.DefaultStore.BlockByID(ctx, blockID)
	if err == nil {
		height = block.Header.Height
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

	payload := flowgo.EmptyPayload()
	return &flowgo.Block{
		Payload: &payload,
		Header:  header,
	}, nil
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	return snapshot.NewReadFuncStorageSnapshot(func(id flowgo.RegisterID) (flowgo.RegisterValue, error) {
		// first try to see if we have local stored ledger
		value, err := s.DefaultStore.GetBytesAtVersion(
			ctx,
			s.KeyGenerator.Storage(storage.LedgerStoreName),
			[]byte(id.String()),
			blockHeight,
		)
		if err == nil && value != nil {
			return value, nil
		}

		// FVM expectes an empty byte array if the value is not found
		value = []byte{}

		// if we don't have it, get it from the archive node
		// for consistency, always use data at the forked height for future blocks
		if blockHeight > s.forkHeight {
			blockHeight = s.forkHeight
		}

		log.Printf("fetching register: block: %d, owner: %x, key: %x (%s)", blockHeight, []byte(id.Owner), []byte(id.Key), []byte(id.Key))

		registerID := convert.RegisterIDToMessage(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
		response, err := s.executionClient.GetRegisterValues(ctx, &executiondata.GetRegisterValuesRequest{
			BlockHeight: blockHeight,
			RegisterIds: []*entities.RegisterID{registerID},
		})
		if err != nil {
			log.Printf("could not get register value: %v", err)
			if status.Code(err) != codes.NotFound {
				return nil, err
			}
		}

		if response == nil || len(response.Values) == 0 {
			log.Printf("not found value for register id %s", id.String())
			// return nil, fmt.Errorf("not found value for register id %s", id.String())
			// return []byte{}, nil
		} else {
			value = response.Values[0]
		}

		// log.Printf("register value: %x", value)

		// cache the value for future use
		err = s.DataSetter.SetBytesWithVersion(
			ctx,
			s.KeyGenerator.Storage(storage.LedgerStoreName),
			[]byte(id.String()),
			value,
			blockHeight)
		if err != nil {
			return nil, fmt.Errorf("could not cache ledger value: %w", err)
		}

		return value, nil
	}), nil
}

func (s *Store) Stop() {
	_ = s.grpcConn.Close()
}
