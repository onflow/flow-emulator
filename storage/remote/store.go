package remote

import (
	"context"
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/flow-archive/api/archive"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/sqlite"
	exeState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Store struct {
	*sqlite.Store
	client archive.APIClient
}

func New() (*Store, error) {
	conn, err := grpc.Dial(
		"archive.mainnet.nodes.onflow.org:9000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not connect to archive node")
	}

	memorySql, err := sqlite.New(":memory:")
	if err != nil {
		return nil, err
	}

	store := &Store{
		client: archive.NewAPIClient(conn),
		Store:  memorySql,
	}

	store.DataGetter = store
	store.DataSetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

func (s *Store) LatestBlock(ctx context.Context) (flowgo.Block, error) {
	block := flowgo.Block{
		Header: &flowgo.Header{},
	}

	heightRes, err := s.client.GetLast(ctx, &archive.GetLastRequest{})
	if err != nil {
		return block, err
	}

	blockRes, err := s.client.GetHeader(ctx, &archive.GetHeaderRequest{Height: heightRes.Height})
	if err != nil {
		return block, err
	}

	var header flowgo.Header
	err = cbor.Unmarshal(blockRes.Data, &header)
	if err != nil {
		return block, err
	}

	block.Header = &header
	return block, nil
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) state.StorageSnapshot {
	_ = s.SetBlockHeight(blockHeight)

	return snapshot.NewReadFuncStorageSnapshot(func(id flowgo.RegisterID) (flowgo.RegisterValue, error) {
		// first try to see if we have local stored ledger
		value, err := s.DefaultStore.GetBytesAtVersion(ctx, "ledger", []byte(id.String()), blockHeight)
		if !errors.Is(err, storage.ErrNotFound) {
			if err != nil {
				return nil, err
			}
			return value, nil
		}

		ledgerKey := exeState.RegisterIDToKey(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
		ledgerPath, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)

		response, err := s.client.GetRegisterValues(ctx, &archive.GetRegisterValuesRequest{
			Height:    blockHeight,
			Registers: [][]byte{ledgerPath[:]},
		})
		if err != nil {
			return nil, err
		}

		if len(response.Values) == 0 {
			return nil, fmt.Errorf("not found value for register id %s", id.String())
		}

		return response.Values[0], nil
	})
}