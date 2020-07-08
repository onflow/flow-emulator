package server_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/dapperlabs/flow-go/fvm"
	"github.com/golang/mock/gomock"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client/convert"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
		latestBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64()}}

		api.EXPECT().
			GetLatestBlock().
			Return(latestBlock, nil).
			Times(1)

		api.EXPECT().
			ExecuteScriptAtBlock(sampleScriptText, nil, latestBlock.Height).
			Return(&types.ScriptResult{
				Value: scriptResponse,
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtLatestBlock(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := jsoncdc.Decode(response.GetValue())
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
			ExecuteScriptAtBlock(sampleScriptText, nil, executionScriptRequest.BlockHeight).
			Return(&types.ScriptResult{
				Value: scriptResponse,
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtBlockHeight(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := jsoncdc.Decode(response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, scriptResponse, value)
	}))

	t.Run("ExecuteScriptAtBlockID", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		scriptResponse := cadence.NewInt(rand.Int())
		randomBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64()}}

		executionScriptRequest := access.ExecuteScriptAtBlockIDRequest{
			Script:  sampleScriptText,
			BlockId: randomBlock.ID.Bytes(),
		}

		api.EXPECT().
			GetBlockByID(randomBlock.ID).
			Return(randomBlock, nil).
			Times(1)

		api.EXPECT().
			ExecuteScriptAtBlock(sampleScriptText, nil, randomBlock.Height).
			Return(&types.ScriptResult{
				Value: scriptResponse,
				Error: nil,
			}, nil).
			Times(1)

		response, err := backend.ExecuteScriptAtBlockID(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := jsoncdc.Decode(response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, scriptResponse, value)
	}))

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		keys := test.AccountKeyGenerator()

		account := flow.Account{
			Address: flow.ServiceAddress(flow.Emulator),
			Balance: 10,
			Code:    []byte("pub fun main() {}"),
			Keys: []*flow.AccountKey{
				keys.New(),
				keys.New(),
			},
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

		assert.Len(t, response.Account.Keys, len(account.Keys))
		for i, key := range response.Account.Keys {
			assert.EqualValues(t, account.Keys[i].ID, key.Index)
		}
	}))

	t.Run("GetEventsForHeightRange fails with invalid block heights", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		latestBlock := flow.Block{BlockHeader: flow.BlockHeader{Height: 21}}

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

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

	t.Run("GetEventsForHeightRange fails if blockchain returns error", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		startBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: 10}}

		latestBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: 11}}

		api.EXPECT().
			GetLatestBlock().
			Return(latestBlock, nil)

		api.EXPECT().
			GetBlockByHeight(startBlock.Height).
			Return(startBlock, nil).
			Times(1)

		api.EXPECT().
			GetEventsByHeight(startBlock.Height, gomock.Any()).
			Return(nil, errors.New("dummy")).
			Times(1)

		_, err := backend.GetEventsForHeightRange(
			context.Background(),
			&access.GetEventsForHeightRangeRequest{StartHeight: 10, EndHeight: 11},
		)

		if assert.Error(t, err) {
			grpcError, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, grpcError.Code(), codes.Internal)
		}
	}))

	t.Run("GetEventsForHeightRange", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		events := test.EventGenerator()

		eventsToReturn := []flow.Event{
			events.New(),
			events.New(),
		}

		eventType := "SomeEvents"

		blocks := []flow.Block{
			flow.Block{BlockHeader: flow.BlockHeader{Height: 1}},
			flow.Block{BlockHeader: flow.BlockHeader{Height: 2}},
		}

		var (
			latestBlock = blocks[1]
			startHeight = blocks[0].Height
			endHeight   = blocks[1].Height
		)

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

		api.EXPECT().
			GetBlockByHeight(blocks[0].Height).
			Return(&blocks[0], nil)

		api.EXPECT().
			GetBlockByHeight(blocks[1].Height).
			Return(&blocks[1], nil)

		api.EXPECT().
			GetEventsByHeight(blocks[0].Height, eventType).
			Return(eventsToReturn, nil)

		api.EXPECT().
			GetEventsByHeight(blocks[1].Height, eventType).
			Return(eventsToReturn, nil)

		response, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{
			Type:        eventType,
			StartHeight: startHeight,
			EndHeight:   endHeight,
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		blockResults := response.GetResults()

		require.Len(t, blocks, 2)

		for i, block := range blockResults {
			assert.Len(t, block.GetEvents(), 2)
			assert.Equal(t, block.GetBlockId(), blocks[i].ID.Bytes())
			assert.Equal(t, block.GetBlockHeight(), blocks[i].Height)
		}
	}))

	t.Run("GetEventsForHeightRange succeeds if end height is higher than latest", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		events := test.EventGenerator()

		eventsToReturn := []flow.Event{
			events.New(),
			events.New(),
		}

		eventType := "SomeEvents"

		blocks := []flow.Block{
			flow.Block{BlockHeader: flow.BlockHeader{Height: 1}},
			flow.Block{BlockHeader: flow.BlockHeader{Height: 2}},
		}

		var (
			latestBlock = blocks[1]
			startHeight = blocks[0].Height
			// end height is higher than latest block height
			endHeight uint64 = 10
		)

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

		api.EXPECT().
			GetBlockByHeight(blocks[0].Height).
			Return(&blocks[0], nil)

		api.EXPECT().
			GetBlockByHeight(blocks[1].Height).
			Return(&blocks[1], nil)

		api.EXPECT().
			GetEventsByHeight(blocks[0].Height, eventType).
			Return(eventsToReturn, nil)

		api.EXPECT().
			GetEventsByHeight(blocks[1].Height, eventType).
			Return(eventsToReturn, nil)

		response, err := backend.GetEventsForHeightRange(context.Background(), &access.GetEventsForHeightRangeRequest{
			Type:        eventType,
			StartHeight: startHeight,
			EndHeight:   endHeight,
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		blockResults := response.GetResults()

		require.Len(t, blocks, 2)

		for i, block := range blockResults {
			assert.Len(t, block.GetEvents(), 2)
			assert.Equal(t, block.GetBlockId(), blocks[i].ID.Bytes())
			assert.Equal(t, block.GetBlockHeight(), blocks[i].Height)
		}
	}))

	t.Run("GetLatestBlockHeader", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		parentID := flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64()}}.ID
		latestBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64(), ParentID: parentID}}

		api.EXPECT().
			GetLatestBlock().
			Return(latestBlock, nil).
			Times(1)

		getBlockHeaderRequest := access.GetLatestBlockHeaderRequest{}

		response, err := backend.GetLatestBlockHeader(context.Background(), &getBlockHeaderRequest)
		assert.NoError(t, err)

		blockResponse := response.GetBlock()
		assert.Equal(t, latestBlock.Height, blockResponse.GetHeight())
		assert.Equal(t, latestBlock.ID, flow.HashToID(blockResponse.GetId()))
		assert.Equal(t, latestBlock.ParentID, flow.HashToID(blockResponse.GetParentId()))
	}))

	t.Run("GetBlockHeaderAtBlockHeight", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		parentID := flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64()}}.ID
		requestedBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64(), ParentID: parentID}}

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
		assert.Equal(t, requestedBlock.ID, flow.HashToID(blockResponse.GetId()))
		assert.Equal(t, requestedBlock.ParentID, flow.HashToID(blockResponse.GetParentId()))
	}))

	t.Run("GetBlockHeaderAtBlockID", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {
		parentID := flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64()}}.ID
		requestedBlock := &flow.Block{BlockHeader: flow.BlockHeader{Height: rand.Uint64(), ParentID: parentID}}

		api.EXPECT().
			GetBlockByID(requestedBlock.ID).
			Return(requestedBlock, nil).
			Times(1)

		getBlockHeaderRequest := access.GetBlockHeaderByIDRequest{
			Id: requestedBlock.ID.Bytes(),
		}
		response, err := backend.GetBlockHeaderByID(context.Background(), &getBlockHeaderRequest)
		assert.NoError(t, err)

		blockResponse := response.GetBlock()
		assert.Equal(t, requestedBlock.Height, blockResponse.GetHeight())
		assert.Equal(t, requestedBlock.ID, flow.HashToID(blockResponse.GetId()))
		assert.Equal(t, requestedBlock.ParentID, flow.HashToID(blockResponse.GetParentId()))
	}))

	t.Run("GetTransaction tx does not exists", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		id := []byte{2, 2, 2, 2}

		api.EXPECT().
			GetTransaction(gomock.Any()).
			Return(nil, &emulator.TransactionNotFoundError{}).
			Times(1)

		response, err := backend.GetTransaction(context.Background(), &access.GetTransactionRequest{
			Id: id,
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
		txMsg, err := convert.TransactionToMessage(*tx)
		require.NoError(t, err)

		requestTx := access.SendTransactionRequest{
			Transaction: txMsg,
		}

		response, err := backend.SendTransaction(context.Background(), &requestTx)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, capturedTx.ID(), capturedTx.ID())
		assert.Equal(t, capturedTx.ID(), flow.HashToID(response.GetId()))
	}))

	t.Run("SendTransaction which errors while processing", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockBlockchainAPI) {

		api.EXPECT().
			AddTransaction(gomock.Any()).
			Return(&types.FlowError{FlowError: &fvm.InvalidSignaturePublicKeyError{}}).
			Times(1)

		tx := test.TransactionGenerator().New()

		// remove payer to make transaction invalid
		tx.Payer = flow.Address{}

		txMsg, err := convert.TransactionToMessage(*tx)
		require.NoError(t, err)

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
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, capturedTx.ID(), capturedTx.ID())
		assert.Equal(t, capturedTx.ID(), flow.HashToID(response.GetId()))
	}))
}
