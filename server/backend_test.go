package server_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/cadence"
	encoding "github.com/dapperlabs/cadence/encoding/xdr"
	"github.com/dapperlabs/flow-go-sdk"
	access "github.com/dapperlabs/flow/protobuf/go/flow/access"
	"github.com/dapperlabs/flow/protobuf/go/flow/entities"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/mocks"
	"github.com/dapperlabs/flow-emulator/server"
	"github.com/dapperlabs/flow-emulator/types"
	"github.com/dapperlabs/flow-emulator/utils/unittest"
)

func TestPing(t *testing.T) {
	ctx := context.Background()
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)
	server := server.NewBackend(logrus.New(), b)

	_, err = server.Ping(ctx, &access.PingRequest{})
	assert.NoError(t, err)
}

func TestBackend(t *testing.T) {

	//wrapper which manages mock lifecycle
	withMocks := func(sut func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI)) func(t *testing.T) {
		return func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			api := mocks.NewMockBlockchainAPI(mockCtrl)

			backend := server.NewBackend(logrus.New(), api)

			sut(t, backend, api)
		}
	}

	t.Run("ExecuteScript", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		executionScriptRequest := access.ExecuteScriptAtLatestBlockRequest{
			Script: sampleScriptText,
		}

		api.EXPECT().
			ExecuteScript(sampleScriptText).
			Return(emulator.ScriptResult{
				Value: cadence.NewInt(2137),
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtLatestBlock(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := encoding.Decode(cadence.IntType{}, response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, cadence.NewInt(2137), value)
	}))

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		account := flow.Account{
			Address: flow.RootAddress,
			Balance: 10,
			Code:    []byte("pub fun main() {}"),
			Keys:    []flow.AccountPublicKey{},
		}

		api.EXPECT().
			GetAccount(account.Address).
			Return(&account, nil).
			Times(1)

		request := access.GetAccountRequest{
			Address: account.Address.Bytes(),
		}
		response, err := backend.GetAccount(context.Background(), &request)

		assert.NoError(t, err)

		assert.Equal(t, account.Address.Bytes(), response.Account.Address)
		assert.Equal(t, account.Balance, response.Account.Balance)
	}))

	t.Run("GetEvents fails with wrong block numbers", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		api.EXPECT().
			GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]flow.Event{}, nil).
			Times(0)

		response, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{
			Type:        "SomeEvents",
			StartHeight: 37,
			EndHeight:   21,
		})

		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
	}))

	t.Run("GetEvents fails if blockchain returns error", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		api.EXPECT().
			GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("dummy")).
			Times(1)

		_, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{})

		if assert.Error(t, err) {
			grpcError, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, grpcError.Code(), codes.Internal)
		}
	}))

	t.Run("GetEvents", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		eventType := "SomeEvents"

		eventsToReturn := []flow.Event{
			unittest.EventFixture(func(e *flow.Event) {
				e.Index = 0
			}),
			unittest.EventFixture(func(e *flow.Event) {
				e.Index = 1
			}),
		}

		api.EXPECT().
			GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(eventsToReturn, nil).
			Times(1)

		var startBlock uint64 = 21
		var endBlock uint64 = 37

		response, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{
			Type:        eventType,
			StartHeight: startBlock,
			EndHeight:   endBlock,
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		resEvents := response.GetEvents()

		assert.Len(t, resEvents, 2)
		assert.EqualValues(t, 0, resEvents[0].GetIndex())
		assert.EqualValues(t, 1, resEvents[1].GetIndex())
	}))

	t.Run("GetLatestBlockHeader", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		block := types.Block{
			Number:            11,
			PreviousBlockHash: nil,
			TransactionHashes: nil,
		}

		api.EXPECT().
			GetLatestBlock().
			Return(&block, nil).
			Times(1)

		response, err := backend.GetLatestBlockHeader(context.Background(), &access.GetLatestBlockHeaderRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, block.Number, response.Block.Height)
	}))

	t.Run("GetTransaction tx does not exists", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		bytes := []byte{2, 2, 2, 2}

		api.EXPECT().
			GetTransaction(gomock.Any()).
			Return(nil, &emulator.ErrTransactionNotFound{}).
			Times(1)

		response, err := backend.GetTransaction(context.Background(), &access.GetTransactionRequest{
			Id: bytes,
		})

		assert.Error(t, err)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.NotFound, grpcError.Code())

		assert.Nil(t, response)
	}))

	// t.Run("GetTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

	// 	txEventType := "TxEvent"

	// 	tx := unittest.TransactionFixture(func(t *flow.Transaction) {
	// 		t.Status = flow.TransactionSealed
	// 	})

	// 	txHash := tx.Hash()

	// 	txEvents := []flow.Event{
	// 		unittest.EventFixture(func(e *flow.Event) {
	// 			e.TxHash = txHash
	// 			e.Type = txEventType
	// 		}),
	// 	}

	// 	tx.Events = txEvents

	// 	api.EXPECT().
	// 		GetTransaction(gomock.Eq(txHash)).
	// 		Return(&tx, nil).
	// 		Times(1)

	// 	response, err := backend.GetTransaction(context.Background(), &access.GetTransactionRequest{
	// 		Id: txHash,
	// 	})

	// 	assert.NoError(t, err)
	// 	assert.NotNil(t, response)

	// 	assert.Equal(t, tx.Nonce, response.Transaction.)

	// 	resEvents := response.GetEvents()

	// 	require.Len(t, resEvents, 1)

	// 	event := convert.MessageToEvent(resEvents[0])

	// 	assert.Equal(t, txHash, event.TxHash)
	// 	assert.Equal(t, txEventType, event.Type)
	// }))

	t.Run("Ping", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		response, err := backend.Ping(context.Background(), &access.PingRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)
	}))

	t.Run("SendTransaction with nil tx", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		response, err := backend.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: nil,
		})

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

	t.Run("SendTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		var capturedTx flow.Transaction

		api.EXPECT().
			AddTransaction(gomock.Any()).
			DoAndReturn(func(tx flow.Transaction) error {
				capturedTx = tx
				return nil
			}).
			Times(1)

		requestTx := access.SendTransactionRequest{
			Transaction: &entities.Transaction{
				Script:           nil,
				ReferenceBlockId: nil,
				PayerAccount:     nil,
				ScriptAccounts: [][]byte{
					nil,
					{1, 2, 3, 4},
				},
				Signatures: []*entities.AccountSignature{
					{
						Account:   []byte{2, 2, 2, 2},
						Signature: []byte{4, 4, 4, 4},
					}, nil,
				},
				Status: 0,
			},
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Len(t, capturedTx.ScriptAccounts, 2)
		assert.Len(t, capturedTx.Signatures, 2)

		assert.True(t, capturedTx.Hash().Equal(response.GetId()))
	}))

	t.Run("SendTransaction which errors while processing", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		api.EXPECT().
			AddTransaction(gomock.Any()).
			Return(&emulator.ErrInvalidSignaturePublicKey{}).
			Times(1)

		requestTx := access.SendTransactionRequest{
			Transaction: &entities.Transaction{
				Script:           nil,
				ReferenceBlockId: nil,
				PayerAccount:     nil,
				ScriptAccounts:   nil,
				Signatures:       nil,
				Status:           0,
			},
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

	t.Run("SendTransaction with AutoMine enabled", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

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
			DoAndReturn(func() (*types.Block, []emulator.TransactionResult, error) {
				return &types.Block{}, make([]emulator.TransactionResult, 0), nil
			}).
			Times(1)

		requestTx := access.SendTransactionRequest{
			Transaction: &entities.Transaction{
				Script:           nil,
				ReferenceBlockId: nil,
				PayerAccount:     nil,
				ScriptAccounts: [][]byte{
					nil,
					{1, 2, 3, 4},
				},
				Signatures: []*entities.AccountSignature{
					{
						Account:   []byte{2, 2, 2, 2},
						Signature: []byte{4, 4, 4, 4},
					}, nil,
				},
				Status: 0,
			},
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Len(t, capturedTx.ScriptAccounts, 2)
		assert.Len(t, capturedTx.Signatures, 2)

		assert.True(t, capturedTx.Hash().Equal(response.GetId()))
	}))
}
