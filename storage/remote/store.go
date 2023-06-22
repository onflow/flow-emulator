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
	"github.com/onflow/flow-archive/api/archive"
	"github.com/onflow/flow-archive/codec/zbor"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	flowgo "github.com/onflow/flow-go/model/flow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-emulator/storage"
)

type Store struct {
	storage.DefaultStore
	forkedStore   storage.Store
	archiveClient archive.APIClient
	grpcConn      *grpc.ClientConn
	archiveHost   string
	forkHeight    uint64
}

func (s *Store) SetBlockHeight(height uint64) error {

	if height < s.forkHeight {
		s.forkHeight = height
	}

	return s.forkedStore.SetBlockHeight(height)
}

func (s *Store) RollbackToBlockHeight(height uint64) error {
	rollbackProvider, isRollbackProvider := s.forkedStore.(storage.RollbackProvider)
	if !isRollbackProvider {
		return fmt.Errorf("storage provider does not support rollback")
	}
	err := rollbackProvider.RollbackToBlockHeight(height)
	if err != nil {
		return err
	}
	return s.SetBlockHeight(height)
}

func (s *Store) Snapshots() ([]string, error) {
	snapshotProvider, isSnapshotShotProvider := s.forkedStore.(storage.SnapshotProvider)
	if !isSnapshotShotProvider {
		return []string{}, fmt.Errorf("storage provider does not support snapshots")
	}
	return snapshotProvider.Snapshots()
}

func (s *Store) CreateSnapshot(snapshotName string) error {
	snapshotProvider, isSnapshotShotProvider := s.forkedStore.(storage.SnapshotProvider)
	if !isSnapshotShotProvider {
		return fmt.Errorf("storage provider does not support snapshots")
	}
	return snapshotProvider.CreateSnapshot(snapshotName)
}

func (s *Store) LoadSnapshot(snapshotName string) error {
	snapshotProvider, isSnapshotShotProvider := s.forkedStore.(storage.SnapshotProvider)
	if !isSnapshotShotProvider {
		return fmt.Errorf("storage provider does not support snapshots")
	}
	return snapshotProvider.LoadSnapshot(snapshotName)
}

func (s *Store) SupportSnapshotsWithCurrentConfig() bool {
	snapshotProvider, isSnapshotShotProvider := s.forkedStore.(storage.SnapshotProvider)
	if !isSnapshotShotProvider {
		return false
	}
	return snapshotProvider.SupportSnapshotsWithCurrentConfig()
}

type Option func(*Store)

var _ storage.SnapshotProvider = &Store{}
var _ storage.RollbackProvider = &Store{}

// WithChainID sets a chain ID and is used to determine which archive node to use.
func WithChainID(ID flowgo.ChainID) Option {
	return func(store *Store) {
		archiveHosts := map[flowgo.ChainID]string{
			flowgo.Mainnet: "archive.mainnet.nodes.onflow.org:9000",
			flowgo.Testnet: "archive.testnet.nodes.onflow.org:9000",
		}

		store.archiveHost = archiveHosts[ID]
	}
}

// WithHost sets archive node archiveHost.
func WithHost(host string) Option {
	return func(store *Store) {
		store.archiveHost = host
	}
}

// WithForkHeight sets height to fork chain.
func WithForkHeight(height uint64) Option {
	return func(store *Store) {
		store.forkHeight = height
	}
}

// WithClient can set an archive node archiveClient
//
// This is mostly use for testing.
func WithClient(client archive.APIClient) Option {
	return func(store *Store) {
		store.archiveClient = client
	}
}

func New(provider storage.Store, options ...Option) (*Store, error) {
	store := &Store{
		forkedStore: provider,
		forkHeight:  0,
	}

	for _, opt := range options {
		opt(store)
	}

	if store.archiveClient == nil {
		if store.archiveHost == "" {
			return nil, fmt.Errorf("archive node archiveHost must be provided")
		}

		conn, err := grpc.Dial(
			store.archiveHost,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("could not connect to archive node: %w", err)
		}

		store.grpcConn = conn
		store.archiveClient = archive.NewAPIClient(conn)
	}

	if store.forkHeight == 0 {
		// check persist
		lastHeight, err := store.forkedStore.LatestBlockHeight(context.Background())
		if err != nil {
			// if it's not set yet, get the block height at fork point
			heightRes, err := store.archiveClient.GetLast(context.Background(), &archive.GetLastRequest{})
			if err != nil {
				return nil, err
			}
			store.forkHeight = heightRes.Height
		} else {
			store.forkHeight = lastHeight
		}
		err = store.SetBlockHeight(store.forkHeight)
		if err != nil {
			return nil, err
		}
	}

	store.DataGetter = store.forkedStore.(storage.DataGetter)
	store.DataSetter = store.forkedStore.(storage.DataSetter)
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

func (s *Store) BlockByID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.Block, error) {
	block, err := s.forkedStore.BlockByID(ctx, blockID)
	if err == nil {
		return block, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	//not found on fork
	heightRes, err := s.archiveClient.GetHeightForBlock(ctx, &archive.GetHeightForBlockRequest{BlockID: blockID[:]})
	if err != nil {
		return nil, err
	}

	if heightRes.Height > s.forkHeight {
		//return 'not found' for blocks after fork
		return nil, storage.ErrNotFound
	}
	return s.BlockByHeight(ctx, heightRes.Height)
}

func (s *Store) LatestBlock(ctx context.Context) (block flowgo.Block, err error) {
	block, err = s.forkedStore.LatestBlock(ctx)
	if err == nil {
		return block, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return flowgo.Block{}, err
	}
	//if no block on fork, ask archive
	aBlock, err := s.BlockByHeight(ctx, s.forkHeight)
	if err != nil {
		return flowgo.Block{}, err
	}
	return *aBlock, nil
}

func (s *Store) BlockByHeight(ctx context.Context, height uint64) (*flowgo.Block, error) {

	if height <= s.forkHeight {
		//get block from archive node
		blockRes, err := s.archiveClient.GetHeader(ctx, &archive.GetHeaderRequest{Height: height})
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

	//get block from local fork
	block, err := s.forkedStore.BlockByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	return block, nil

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

		ledgerKey := convert.RegisterIDToLedgerKey(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
		ledgerPath, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)
		if err != nil {
			return nil, err
		}

		// if we don't have it, get it from the archive node
		response, err := s.archiveClient.GetRegisterValues(ctx, &archive.GetRegisterValuesRequest{
			Height: blockHeight,
			Paths:  [][]byte{ledgerPath[:]},
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
