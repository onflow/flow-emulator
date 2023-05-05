package remote

import (
	"context"
	"github.com/onflow/flow-archive/api/archive"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Store struct {
	*storage.DefaultStore
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

	return &Store{
		client: archive.NewAPIClient(conn),
	}, nil
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) state.StorageSnapshot {
	return nil
}
