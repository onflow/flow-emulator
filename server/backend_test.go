package server_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/dapperlabs/flow-go-sdk/client/convert"
	"github.com/dapperlabs/flow-go-sdk/test"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/cadence"
	encoding "github.com/dapperlabs/cadence/encoding/json"
	"github.com/dapperlabs/flow-go-sdk"
	access "github.com/dapperlabs/flow/protobuf/go/flow/access"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/mocks"
	"github.com/dapperlabs/flow-emulator/server"
	"github.com/dapperlabs/flow-emulator/types"
)

func TestPing(t *testing.T) {
	ctx := context.Background()
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)
	backend := server.NewBackend(logrus.New(), b)

	_, err = backend.Ping(ctx, &access.PingRequest{})
	assert.NoError(t, err)
}

func TestBackend(t *testing.T) {
	ids := test.IdentifierGenerator()
	results := test.TransactionResultGenerator()

	// wrapper which manages mock lifecycle
	withMocks := func(sut func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI)) func(t *testing.T) {
		return func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			api := mocks.NewMockBlockchainAPI(mockCtrl)

			backend := server.NewBackend(logrus.New(), api)

			sut(t, backend, api)
		}
	}

	t.Run("ExecuteScriptAtLatestBlock", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		scriptResponse := cadence.NewInt(rand.Int())
		executionScriptRequest := access.ExecuteScriptAtLatestBlockRequest{
			Script: sampleScriptText,
		}
		latestBlock := &types.Block{Height: rand.Uint64()}

		api.EXPECT().
			GetLatestBlock().
			Return(latestBlock, nil).
			Times(1)

		api.EXPECT().
			ExecuteScriptAtBlock(sampleScriptText, latestBlock.Height).
			Return(emulator.ScriptResult{
				Value: scriptResponse,
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtLatestBlock(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := encoding.Decode(response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, scriptResponse, value)
	}))

	t.Run("ExecuteScriptAtBlockHeight", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		scriptResponse := cadence.NewInt(rand.Int())
		executionScriptRequest := access.ExecuteScriptAtBlockHeightRequest{
			Script:      sampleScriptText,
			BlockHeight: rand.Uint64(),
		}

		api.EXPECT().
			ExecuteScriptAtBlock(sampleScriptText, executionScriptRequest.BlockHeight).
			Return(emulator.ScriptResult{
				Value: scriptResponse,
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtBlockHeight(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := encoding.Decode(response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, scriptResponse, value)
	}))

	t.Run("ExecuteScriptAtBlockID", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		scriptResponse := cadence.NewInt(rand.Int())
		randomBlock := &types.Block{Height: rand.Uint64()}

		executionScriptRequest := access.ExecuteScriptAtBlockIDRequest{
			Script:  sampleScriptText,
			BlockId: randomBlock.ID().Bytes(),
		}

		api.EXPECT().
			GetBlockByID(randomBlock.ID()).
			Return(randomBlock, nil).
			Times(1)

		api.EXPECT().
			ExecuteScriptAtBlock(sampleScriptText, randomBlock.Height).
			Return(emulator.ScriptResult{
				Value: scriptResponse,
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtBlockID(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := encoding.Decode(response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, scriptResponse, value)
	}))

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		account := flow.Account{
			Address: flow.RootAddress,
			Balance: 10,
			Code:    []byte("pub fun main() {}"),
			Keys:    []flow.AccountKey{},
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

		assert.Equal(t, account.Address, flow.BytesToAddress(response.Account.Address))
		assert.Equal(t, account.Balance, response.Account.Balance)
	}))

	t.Run("GetEvents fails with wrong block heights", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		api.EXPECT().
			GetEventsByHeight(gomock.Any(), gomock.Any()).
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
		startBlock := types.Block{
			Height: 10,
		}

		api.EXPECT().
			GetBlockByHeight(startBlock.Height).
			Return(&startBlock, nil).
			Times(1)

		api.EXPECT().
			GetEventsByHeight(startBlock.Height, gomock.Any()).
			Return(nil, errors.New("dummy")).
			Times(1)

		_, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{StartHeight: 10, EndHeight: 11})

		if assert.Error(t, err) {
			grpcError, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, grpcError.Code(), codes.Internal)
		}
	}))

	t.Run("GetEvents", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		// TODO: fix event tests

		// eventType := "SomeEvents"
		//
		// eventsToReturn := []flow.Event{
		// 	unittest.EventFixture(func(e *flow.Event) {
		// 		e.EventIndex = 0
		// 	}),
		// 	unittest.EventFixture(func(e *flow.Event) {
		// 		e.EventIndex = 1
		// 	}),
		// }
		//
		// api.EXPECT().
		// 	GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
		// 	Return(eventsToReturn, nil).
		// 	Times(1)
		//
		// var startBlock uint64 = 21
		// var endBlock uint64 = 37
		//
		// response, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{
		// 	Type:        eventType,
		// 	StartHeight: startBlock,
		// 	EndHeight:   endBlock,
		// })
		//
		// assert.NoError(t, err)
		// assert.NotNil(t, response)
		//
		// resEvents := response.GetEvents()
		//
		// assert.Len(t, resEvents, 2)
		// assert.EqualValues(t, 0, resEvents[0].GetEventIndex())
		// assert.EqualValues(t, 1, resEvents[1].GetEventIndex())
	}))

	t.Run("GetLatestBlockHeader", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		parentID := types.Block{Height: rand.Uint64()}.ID()
		latestBlock := &types.Block{
			Height:   rand.Uint64(),
			ParentID: parentID,
		}

		api.EXPECT().
			GetLatestBlock().
			Return(latestBlock, nil).
			Times(1)

		getBlockHeaderRequest := access.GetLatestBlockHeaderRequest{}

		response, err := backend.GetLatestBlockHeader(context.Background(), &getBlockHeaderRequest)
		assert.NoError(t, err)

		blockResponse := response.GetBlock()
		assert.Equal(t, latestBlock.Height, blockResponse.GetHeight())
		assert.Equal(t, latestBlock.ID(), flow.HashToID(blockResponse.GetId()))
		assert.Equal(t, latestBlock.ParentID, flow.HashToID(blockResponse.GetParentId()))
	}))

	t.Run("GetBlockHeaderAtBlockHeight", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		parentID := types.Block{Height: rand.Uint64()}.ID()
		requestedBlock := &types.Block{
			Height:   rand.Uint64(),
			ParentID: parentID,
		}

		api.EXPECT().
			GetBlockByHeight(requestedBlock.Height).
			Return(requestedBlock, nil).
			Times(1)

		getBlockHeaderRequest := access.GetBlockHeaderByHeightRequest{
			Height: requestedBlock.Height,
		}
		response, err := backend.GetBlockHeaderByHeight(context.Background(), &getBlockHeaderRequest)
		assert.NoError(t, err)

		blockResponse := response.GetBlock()
		assert.Equal(t, requestedBlock.Height, blockResponse.GetHeight())
		assert.Equal(t, requestedBlock.ID(), flow.HashToID(blockResponse.GetId()))
		assert.Equal(t, requestedBlock.ParentID, flow.HashToID(blockResponse.GetParentId()))
	}))

	t.Run("GetBlockHeaderAtBlockID", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		parentID := types.Block{Height: rand.Uint64()}.ID()
		requestedBlock := &types.Block{
			Height:   rand.Uint64(),
			ParentID: parentID,
		}

		api.EXPECT().
			GetBlockByID(requestedBlock.ID()).
			Return(requestedBlock, nil).
			Times(1)

		getBlockHeaderRequest := access.GetBlockHeaderByIDRequest{
			Id: requestedBlock.ID().Bytes(),
		}
		response, err := backend.GetBlockHeaderByID(context.Background(), &getBlockHeaderRequest)
		assert.NoError(t, err)

		blockResponse := response.GetBlock()
		assert.Equal(t, requestedBlock.Height, blockResponse.GetHeight())
		assert.Equal(t, requestedBlock.ID(), flow.HashToID(blockResponse.GetId()))
		assert.Equal(t, requestedBlock.ParentID, flow.HashToID(blockResponse.GetParentId()))
	}))

	t.Run("GetTransaction tx does not exists", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		bytes := []byte{2, 2, 2, 2}

		api.EXPECT().
			GetTransaction(gomock.Any()).
			Return(nil, emulator.ErrTransactionNotFound).
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

	t.Run("GetTransactionResult", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		txID := ids.New()
		expectedResult := results.New()

		api.EXPECT().
			GetTransactionResult(txID).
			Return(&expectedResult, nil).
			Times(1)

		response, err := backend.GetTransactionResult(context.Background(), &access.GetTransactionRequest{
			Id: txID.Bytes(),
		})

		assert.NoError(t, err)

		result, err := convert.MessageToTransactionResult(response)
		require.NoError(t, err)

		assert.Equal(t, expectedResult, result)
	}))

	// t.Run("GetTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

	// 	txEventType := "TxEvent"

	// 	tx := unittest.TransactionFixture(func(t *flow.Transaction) {
	// 		t.Status = flow.TransactionStatusSealed
	// 	})

	// 	txHash := tx.ID()

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

		tx := test.TransactionGenerator().New()
		txMsg := convert.TransactionToMessage(*tx)

		requestTx := access.SendTransactionRequest{
			Transaction: txMsg,
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, capturedTx.ID(), capturedTx.ID())
		assert.Equal(t, capturedTx.ID(), flow.HashToID(response.GetId()))
	}))

	t.Run("SendTransaction which errors while processing", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		api.EXPECT().
			AddTransaction(gomock.Any()).
			Return(&emulator.ErrInvalidSignaturePublicKey{}).
			Times(1)

		tx := test.TransactionGenerator().New()

		// remove payer to make transaction invalid
		tx.Payer = flow.ZeroAddress

		txMsg := convert.TransactionToMessage(*tx)

		requestTx := access.SendTransactionRequest{
			Transaction: txMsg,
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

		tx := test.TransactionGenerator().New()
		txMsg := convert.TransactionToMessage(*tx)

		requestTx := access.SendTransactionRequest{
			Transaction: txMsg,
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, capturedTx.ID(), capturedTx.ID())
		assert.Equal(t, capturedTx.ID(), flow.HashToID(response.GetId()))
	}))
}
