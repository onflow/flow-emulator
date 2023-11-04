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

	"github.com/onflow/flow-archive/api/archive"
	"github.com/onflow/flow-archive/codec/zbor"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/sqlite"
	"github.com/onflow/flow-emulator/types"
)

type Store struct {
	*sqlite.Store
	client           archive.APIClient
	grpcConn         *grpc.ClientConn
	host             string
	startBlockHeight uint64
}

type Option func(*Store)

// WithChainID sets a chain ID and is used to determine which archive node to use.
func WithChainID(ID flowgo.ChainID) Option {
	return func(store *Store) {
		archiveHosts := map[flowgo.ChainID]string{
			flowgo.Mainnet: "archive.mainnet.nodes.onflow.org:9000",
			flowgo.Testnet: "archive.testnet.nodes.onflow.org:9000",
		}

		store.host = archiveHosts[ID]
	}
}

// WithHost sets archive node host.
func WithHost(host string) Option {
	return func(store *Store) {
		store.host = host
	}
}

// WithClient can set an archive node client
//
// This is mostly use for testing.
func WithClient(client archive.APIClient) Option {
	return func(store *Store) {
		store.client = client
	}
}

// WithStartBlockHeight sets the start height for the store.
func WithStartBlockHeight(height uint64) Option {
	return func(store *Store) {
		store.startBlockHeight = height
	}
}

func New(provider *sqlite.Store, options ...Option) (*Store, error) {
	store := &Store{
		Store: provider,
	}

	for _, opt := range options {
		opt(store)
	}

	if store.client == nil {
		if store.host == "" {
			return nil, fmt.Errorf("archive node host must be provided")
		}

		conn, err := grpc.Dial(
			store.host,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("could not connect to archive node: %w", err)
		}

		store.grpcConn = conn
		store.client = archive.NewAPIClient(conn)
	}

	if err := store.initializeStartBlock(); err != nil {
		return nil, err
	}

	store.DataGetter = store
	store.DataSetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

func (s *Store) initializeStartBlock() error {
	if s.startBlockHeight > 0 {
		// only set the start block height if it's not already set
		// it may be set if restarting from a persistent store
		_, err := s.Store.LatestBlockHeight(context.Background())
		if errors.Is(err, storage.ErrNotFound) {
			err = s.Store.SetBlockHeight(s.startBlockHeight)
			if err != nil {
				return fmt.Errorf("could not set start block height: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("error getting latest block height: %w", err)
		}
		return nil
	}

	resp, err := s.client.GetLast(context.Background(), &archive.GetLastRequest{})
	if err != nil {
		return fmt.Errorf("could not get last block height: %w", err)
	}
	err = s.Store.SetBlockHeight(resp.Height)
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
		heightRes, err := s.client.GetHeightForBlock(ctx, &archive.GetHeightForBlockRequest{BlockID: blockID[:]})
		if err != nil {
			return nil, err
		}
		height = heightRes.Height
	} else {
		return nil, err
	}

	return s.BlockByHeight(ctx, height)
}

func (s *Store) LatestBlock(ctx context.Context) (flowgo.Block, error) {
	// try to resume from the last local block
	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return flowgo.Block{}, err
	}

	// if it's not set yet, get the latest block available from the archive node
	if latestBlockHeight == 0 {
		heightRes, err := s.client.GetLast(ctx, &archive.GetLastRequest{})
		if err != nil {
			return flowgo.Block{}, err
		}
		latestBlockHeight = heightRes.Height
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

	if latestBlockHeight, err := s.LatestBlockHeight(ctx); err == nil && height > latestBlockHeight {
		return nil, &types.BlockNotFoundByHeightError{Height: height}
	}

	blockRes, err := s.client.GetHeader(ctx, &archive.GetHeaderRequest{Height: height})
	if err != nil {
		return nil, err
	}

	var header flowgo.Header
	err = zbor.NewCodec().Unmarshal(blockRes.Data, &header)
	if err != nil {
		return nil, err
	}

	payload := flowgo.EmptyPayload()
	return &flowgo.Block{
		Payload: &payload,
		Header:  &header,
	}, nil
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	// err := s.SetBlockHeight(blockHeight)
	// if err != nil {
	// 	return nil, err
	// }

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

		log.Printf("fetching register: owner: %x, key: %x", []byte(id.Owner), []byte(id.Key))

		// if we don't have it, get it from the archive node
		registerID := flowgo.RegisterID{Key: id.Key, Owner: id.Owner}
		response, err := s.client.GetRegisterValues(ctx, &archive.GetRegisterValuesRequest{
			Height:    blockHeight,
			Registers: [][]byte{registerID.Bytes()},
		})
		if err != nil {
			return nil, err
		}

		if len(response.Values) == 0 {
			return nil, fmt.Errorf("not found value for register id %s", id.String())
		}

		value = response.Values[0]

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
