package backend_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client/convert"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-emulator/mocks"
	"github.com/dapperlabs/flow-emulator/server/backend"
	"github.com/dapperlabs/flow-emulator/server/handler"
	"github.com/dapperlabs/flow-emulator/types"
)

func TestBackendAutoMine(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	api := mocks.NewMockBlockchainAPI(mockCtrl)

	backend := backend.New(logrus.New(), api)
	handler := handler.New(backend)

	// enable automine flag
	backend.EnableAutoMine()
	// disable automine flag at the end of the test
	defer backend.DisableAutoMine()

	var capturedTx flow.Transaction

	api.EXPECT().
		AddTransaction(gomock.Any()).
		DoAndReturn(func(tx flow.Transaction) error {
			capturedTx = tx
			return nil
		}).
		Times(1)

	// expect transaction to be executed immediately
	api.EXPECT().
		ExecuteAndCommitBlock().
		DoAndReturn(func() (*flow.Block, []*types.TransactionResult, error) {
			return &flow.Block{BlockHeader: flow.BlockHeader{}, BlockPayload: flow.BlockPayload{}},
				make([]*types.TransactionResult, 0), nil
		}).
		Times(1)

	tx := test.TransactionGenerator().New()

	txMsg, err := convert.TransactionToMessage(*tx)
	require.NoError(t, err)

	requestTx := access.SendTransactionRequest{
		Transaction: txMsg,
	}

	response, err := handler.SendTransaction(context.Background(), &requestTx)
	assert.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, capturedTx.ID(), capturedTx.ID())
	assert.Equal(t, capturedTx.ID(), flow.HashToID(response.GetId()))
}
