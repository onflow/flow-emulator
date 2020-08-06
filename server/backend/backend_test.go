package backend_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/dapperlabs/flow-go/fvm"
	flowgo "github.com/dapperlabs/flow-go/model/flow"
	"github.com/golang/mock/gomock"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/mocks"
	"github.com/dapperlabs/flow-emulator/server/backend"
	"github.com/dapperlabs/flow-emulator/types"
)

func backendTest(f func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI)) func(t *testing.T) {
	return func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		api := mocks.NewMockBlockchainAPI(mockCtrl)

		backend := backend.New(logrus.New(), api)

		f(t, backend, api)
	}
}

func TestBackend(t *testing.T) {
	ids := test.IdentifierGenerator()
	results := test.TransactionResultGenerator()

	t.Run("ExecuteScriptAtLatestBlock", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		script := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")

		expectedValue := cadence.NewInt(rand.Int())

		latestBlock := &flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}

		api.EXPECT().
			GetLatestBlock().
			Return(latestBlock, nil).
			Times(1)

		api.EXPECT().
			ExecuteScriptAtBlock(script, nil, latestBlock.Header.Height).
			Return(&types.ScriptResult{
				Value: expectedValue,
				Error: nil,
			}, nil).
			Times(1)

		value, err := backend.ExecuteScriptAtLatestBlock(context.Background(), script, nil)
		assert.NoError(t, err)

		actualValue, err := jsoncdc.Decode(value)
		assert.NoError(t, err)

		assert.Equal(t, expectedValue, actualValue)
	}))

	t.Run("ExecuteScriptAtBlockHeight", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		script := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		blockHeight := rand.Uint64()

		expectedValue := cadence.NewInt(rand.Int())

		api.EXPECT().
			ExecuteScriptAtBlock(script, nil, blockHeight).
			Return(&types.ScriptResult{
				Value: expectedValue,
				Error: nil,
			}, nil).
			Times(1)

		value, err := backend.ExecuteScriptAtBlockHeight(context.Background(), blockHeight, script, nil)
		assert.NoError(t, err)

		actualValue, err := jsoncdc.Decode(value)
		assert.NoError(t, err)

		assert.Equal(t, expectedValue, actualValue)
	}))

	t.Run("ExecuteScriptAtBlockID", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		script := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		expectedValue := cadence.NewInt(rand.Int())

		randomBlock := &flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}

		api.EXPECT().
			GetBlockByID(flow.Identifier(randomBlock.ID())).
			Return(randomBlock, nil).
			Times(1)

		api.EXPECT().
			ExecuteScriptAtBlock(script, nil, randomBlock.Header.Height).
			Return(&types.ScriptResult{
				Value: expectedValue,
				Error: nil,
			}, nil).
			Times(1)

		value, err := backend.ExecuteScriptAtBlockID(
			context.Background(),
			flow.Identifier(randomBlock.ID()),
			script,
			nil,
		)
		assert.NoError(t, err)

		actualValue, err := jsoncdc.Decode(value)
		assert.NoError(t, err)

		assert.Equal(t, expectedValue, actualValue)
	}))

	t.Run("GetAccountAtLatestBlock", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		keys := test.AccountKeyGenerator()

		expectedAccount := flow.Account{
			Address: flow.ServiceAddress(flow.Emulator),
			Balance: 10,
			Code:    []byte("pub fun main() {}"),
			Keys: []*flow.AccountKey{
				keys.New(),
				keys.New(),
			},
		}

		api.EXPECT().
			GetAccount(expectedAccount.Address).
			Return(&expectedAccount, nil).
			Times(1)

		actualAccount, err := backend.GetAccountAtLatestBlock(context.Background(), expectedAccount.Address)
		require.NoError(t, err)

		assert.Equal(t, expectedAccount, *actualAccount)
	}))

	t.Run("GetEventsForHeightRange fails with invalid block heights", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		latestBlock := flowgo.Block{Header: &flowgo.Header{Height: 21}}

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

		api.EXPECT().
			GetEventsByHeight(gomock.Any(), gomock.Any()).
			Return([]flow.Event{}, nil).
			Times(0)

		eventType := "SomeEvent"
		startHeight := uint64(37)
		endHeight := uint64(21)

		_, err := backend.GetEventsForHeightRange(context.Background(), eventType, startHeight, endHeight)
		require.Error(t, err)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
	}))

	t.Run("GetEventsForHeightRange fails if blockchain returns error", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		startBlock := flowgo.Block{Header: &flowgo.Header{Height: 10}}
		latestBlock := flowgo.Block{Header: &flowgo.Header{Height: 11}}

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

		api.EXPECT().
			GetBlockByHeight(startBlock.Header.Height).
			Return(&startBlock, nil).
			Times(1)

		api.EXPECT().
			GetEventsByHeight(startBlock.Header.Height, gomock.Any()).
			Return(nil, errors.New("dummy")).
			Times(1)

		eventType := "SomeEvent"

		_, err := backend.GetEventsForHeightRange(
			context.Background(),
			eventType,
			startBlock.Header.Height,
			latestBlock.Header.Height,
		)
		require.Error(t, err)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, grpcError.Code(), codes.Internal)
	}))

	t.Run("GetEventsForHeightRange", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		events := test.EventGenerator()

		eventsToReturn := []flow.Event{
			events.New(),
			events.New(),
		}

		eventType := "SomeEvent"

		blocks := []flowgo.Block{
			{Header: &flowgo.Header{Height: 1}},
			{Header: &flowgo.Header{Height: 2}},
		}

		var (
			latestBlock = blocks[1]
			startHeight = blocks[0].Header.Height
			endHeight   = blocks[1].Header.Height
		)

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

		api.EXPECT().
			GetBlockByHeight(blocks[0].Header.Height).
			Return(&blocks[0], nil)

		api.EXPECT().
			GetBlockByHeight(blocks[1].Header.Height).
			Return(&blocks[1], nil)

		api.EXPECT().
			GetEventsByHeight(blocks[0].Header.Height, eventType).
			Return(eventsToReturn, nil)

		api.EXPECT().
			GetEventsByHeight(blocks[1].Header.Height, eventType).
			Return(eventsToReturn, nil)

		results, err := backend.GetEventsForHeightRange(context.Background(),
			eventType,
			startHeight,
			endHeight,
		)
		require.NoError(t, err)

		assert.NotNil(t, results)

		require.Len(t, blocks, 2)

		for i, block := range results {
			assert.Len(t, block.Events, 2)
			assert.Equal(t, block.BlockID, blocks[i].ID())
			assert.Equal(t, block.BlockHeight, blocks[i].Header.Height)
		}
	}))

	t.Run("GetEventsForHeightRange succeeds if end height is higher than latest", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		events := test.EventGenerator()

		eventsToReturn := []flow.Event{
			events.New(),
			events.New(),
		}

		eventType := "SomeEvents"

		blocks := []flowgo.Block{
			{Header: &flowgo.Header{Height: 1}},
			{Header: &flowgo.Header{Height: 2}},
		}

		var (
			latestBlock = blocks[1]
			startHeight = blocks[0].Header.Height
			// end height is higher than latest block height
			endHeight uint64 = 10
		)

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil)

		api.EXPECT().
			GetBlockByHeight(blocks[0].Header.Height).
			Return(&blocks[0], nil)

		api.EXPECT().
			GetBlockByHeight(blocks[1].Header.Height).
			Return(&blocks[1], nil)

		api.EXPECT().
			GetEventsByHeight(blocks[0].Header.Height, eventType).
			Return(eventsToReturn, nil)

		api.EXPECT().
			GetEventsByHeight(blocks[1].Header.Height, eventType).
			Return(eventsToReturn, nil)

		results, err := backend.GetEventsForHeightRange(context.Background(),
			eventType,
			startHeight,
			endHeight,
		)
		require.NoError(t, err)

		assert.NotNil(t, results)

		require.Len(t, blocks, 2)

		for i, block := range results {
			assert.Len(t, block.Events, 2)
			assert.Equal(t, block.BlockID, blocks[i].ID())
			assert.Equal(t, block.BlockHeight, blocks[i].Header.Height)
		}
	}))

	t.Run("GetLatestBlockHeader", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		parentID := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}.ID()
		latestBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64(), ParentID: parentID}}

		api.EXPECT().
			GetLatestBlock().
			Return(&latestBlock, nil).
			Times(1)

		header, err := backend.GetLatestBlockHeader(context.Background(), false)
		assert.NoError(t, err)

		assert.Equal(t, latestBlock.Header.Height, header.Height)
		assert.Equal(t, latestBlock.ID(), header.ID())
		assert.Equal(t, latestBlock.Header.ParentID, header.ParentID)
	}))

	t.Run("GetBlockHeaderAtBlockHeight", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		parentID := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}.ID()
		requestedBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64(), ParentID: parentID}}

		api.EXPECT().
			GetBlockByHeight(requestedBlock.Header.Height).
			Return(&requestedBlock, nil).
			Times(1)

		header, err := backend.GetBlockHeaderByHeight(context.Background(), requestedBlock.Header.Height)
		assert.NoError(t, err)

		assert.Equal(t, requestedBlock.Header.Height, header.Height)
		assert.Equal(t, requestedBlock.ID(), header.ID())
		assert.Equal(t, requestedBlock.Header.ParentID, header.ParentID)
	}))

	t.Run("GetBlockHeaderAtBlockID", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		parentID := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}.ID()
		requestedBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64(), ParentID: parentID}}

		api.EXPECT().
			GetBlockByID(flow.Identifier(requestedBlock.ID())).
			Return(&requestedBlock, nil).
			Times(1)

		header, err := backend.GetBlockHeaderByID(context.Background(), flow.Identifier(requestedBlock.ID()))
		assert.NoError(t, err)

		assert.Equal(t, requestedBlock.Header.Height, header.Height)
		assert.Equal(t, requestedBlock.ID(), header.ID())
		assert.Equal(t, requestedBlock.Header.ParentID, header.ParentID)
	}))

	t.Run("GetTransaction tx does not exists", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		txID := ids.New()

		api.EXPECT().
			GetTransaction(txID).
			Return(nil, &emulator.TransactionNotFoundError{}).
			Times(1)

		tx, err := backend.GetTransaction(context.Background(), txID)
		require.Error(t, err)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.NotFound, grpcError.Code())

		assert.Nil(t, tx)
	}))

	t.Run("GetTransactionResult", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		txID := ids.New()
		expectedTx := results.New()

		api.EXPECT().
			GetTransactionResult(txID).
			Return(&expectedTx, nil).
			Times(1)

		actualTx, err := backend.GetTransactionResult(context.Background(), txID)
		require.NoError(t, err)

		assert.Equal(t, expectedTx, *actualTx)
	}))

	// t.Run("GetTransaction", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {

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

	t.Run("Ping", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {
		err := backend.Ping(context.Background())
		assert.NoError(t, err)
	}))

	t.Run("SendTransaction", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {

		var actualTx flow.Transaction

		api.EXPECT().
			AddTransaction(gomock.Any()).
			DoAndReturn(func(tx flow.Transaction) error {
				actualTx = tx
				return nil
			}).
			Times(1)

		expectedTx := test.TransactionGenerator().New()

		err := backend.SendTransaction(context.Background(), *expectedTx)
		require.NoError(t, err)

		assert.Equal(t, *expectedTx, actualTx)
	}))

	t.Run("SendTransaction which errors while processing", backendTest(func(t *testing.T, backend *backend.Backend, api *mocks.MockBlockchainAPI) {

		api.EXPECT().
			AddTransaction(gomock.Any()).
			Return(&types.FlowError{FlowError: &fvm.InvalidSignaturePublicKeyError{}}).
			Times(1)

		expectedTx := test.TransactionGenerator().New()

		// remove payer to make transaction invalid
		expectedTx.Payer = flow.Address{}

		err := backend.SendTransaction(context.Background(), *expectedTx)
		require.Error(t, err)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())
	}))
}

func TestBackendAutoMine(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	api := mocks.NewMockBlockchainAPI(mockCtrl)

	backend := backend.New(logrus.New(), api)

	// enable automine flag
	backend.EnableAutoMine()
	// disable automine flag at the end of the test
	defer backend.DisableAutoMine()

	var actualTx flow.Transaction

	api.EXPECT().
		AddTransaction(gomock.Any()).
		DoAndReturn(func(tx flow.Transaction) error {
			actualTx = tx
			return nil
		}).
		Times(1)

	// expect transaction to be executed immediately
	api.EXPECT().
		ExecuteAndCommitBlock().
		DoAndReturn(func() (*flowgo.Block, []*types.TransactionResult, error) {
			return &flowgo.Block{Header: &flowgo.Header{}, Payload: &flowgo.Payload{}},
				make([]*types.TransactionResult, 0), nil
		}).
		Times(1)

	expectedTx := test.TransactionGenerator().New()

	err := backend.SendTransaction(context.Background(), *expectedTx)
	require.NoError(t, err)

	assert.Equal(t, *expectedTx, actualTx)
}
