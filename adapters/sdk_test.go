package adapters

import (
	"context"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/emulator/mocks"
	"github.com/onflow/flow-emulator/types"
	flowgosdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/access"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func sdkTest(f func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator)) func(t *testing.T) {
	return func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		emu := mocks.NewMockEmulator(mockCtrl)
		logger := zerolog.Nop()
		back := NewSDKAdapter(&logger, emu)

		f(t, back, emu)
	}
}

func TestSDK(t *testing.T) {

	t.Run("Ping", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {
		emu.EXPECT().
			Ping().
			Times(1)

		err := adapter.Ping(context.Background())
		assert.NoError(t, err)
	}))

	t.Run("GetLatestBlockHeader", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		block := flowgosdk.Block{
			BlockHeader: flowgosdk.BlockHeader{
				ID:     flowgosdk.Identifier{0x8c, 0x3c, 0xf9, 0x36, 0xbf, 0x2d, 0x3, 0x8d, 0x21, 0x71, 0xb4, 0x80, 0x1f, 0xba, 0x30, 0x36, 0x3c, 0xd5, 0x76, 0xc3, 0x21, 0xb4, 0x3d, 0xbd, 0xa2, 0x69, 0xa1, 0xe2, 0x7c, 0x6f, 0x58, 0x28},
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetLatestBlockHeader(context.Background(), true)
		assert.Equal(t, &block.BlockHeader, result)
		assert.Equal(t, flowgosdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetLatestBlockHeader(context.Background(), true)
		assert.Nil(t, result)
		assert.Equal(t, flowgosdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockHeaderByHeight", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		block := flowgosdk.Block{
			BlockHeader: flowgosdk.BlockHeader{
				ID:     flowgosdk.Identifier{0x8c, 0x3c, 0xf9, 0x36, 0xbf, 0x2d, 0x3, 0x8d, 0x21, 0x71, 0xb4, 0x80, 0x1f, 0xba, 0x30, 0x36, 0x3c, 0xd5, 0x76, 0xc3, 0x21, 0xb4, 0x3d, 0xbd, 0xa2, 0x69, 0xa1, 0xe2, 0x7c, 0x6f, 0x58, 0x28},
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockHeaderByHeight(context.Background(), 42)
		assert.Equal(t, &block.BlockHeader, result)
		assert.Equal(t, flowgosdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockHeaderByHeight(context.Background(), 42)
		assert.Nil(t, result)
		assert.Equal(t, flowgosdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockHeaderByID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		id := flowgosdk.Identifier{}
		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		block := flowgosdk.Block{
			BlockHeader: flowgosdk.BlockHeader{
				ID:     flowgosdk.Identifier{0x8c, 0x3c, 0xf9, 0x36, 0xbf, 0x2d, 0x3, 0x8d, 0x21, 0x71, 0xb4, 0x80, 0x1f, 0xba, 0x30, 0x36, 0x3c, 0xd5, 0x76, 0xc3, 0x21, 0xb4, 0x3d, 0xbd, 0xa2, 0x69, 0xa1, 0xe2, 0x7c, 0x6f, 0x58, 0x28},
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockHeaderByID(context.Background(), id)
		assert.Equal(t, &block.BlockHeader, result)
		assert.Equal(t, flowgosdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockHeaderByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Equal(t, flowgosdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetLatestBlock", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		block := flowgosdk.Block{
			BlockHeader: flowgosdk.BlockHeader{
				ID:     flowgosdk.Identifier{0x8c, 0x3c, 0xf9, 0x36, 0xbf, 0x2d, 0x3, 0x8d, 0x21, 0x71, 0xb4, 0x80, 0x1f, 0xba, 0x30, 0x36, 0x3c, 0xd5, 0x76, 0xc3, 0x21, 0xb4, 0x3d, 0xbd, 0xa2, 0x69, 0xa1, 0xe2, 0x7c, 0x6f, 0x58, 0x28},
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetLatestBlock(context.Background(), true)
		assert.Equal(t, &block, result)
		assert.Equal(t, flowgosdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetLatestBlock(context.Background(), true)
		assert.Nil(t, result)
		assert.Equal(t, flowgosdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockByHeight", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		block := flowgosdk.Block{
			BlockHeader: flowgosdk.BlockHeader{
				ID:     flowgosdk.Identifier{0x8c, 0x3c, 0xf9, 0x36, 0xbf, 0x2d, 0x3, 0x8d, 0x21, 0x71, 0xb4, 0x80, 0x1f, 0xba, 0x30, 0x36, 0x3c, 0xd5, 0x76, 0xc3, 0x21, 0xb4, 0x3d, 0xbd, 0xa2, 0x69, 0xa1, 0xe2, 0x7c, 0x6f, 0x58, 0x28},
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockByHeight(context.Background(), 42)
		assert.Equal(t, &block, result)
		assert.Equal(t, flowgosdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockByHeight(context.Background(), 42)
		assert.Nil(t, result)
		assert.Equal(t, flowgosdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockByID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		id := flowgosdk.Identifier{}
		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		block := flowgosdk.Block{
			BlockHeader: flowgosdk.BlockHeader{
				ID:     flowgosdk.Identifier{0x8c, 0x3c, 0xf9, 0x36, 0xbf, 0x2d, 0x3, 0x8d, 0x21, 0x71, 0xb4, 0x80, 0x1f, 0xba, 0x30, 0x36, 0x3c, 0xd5, 0x76, 0xc3, 0x21, 0xb4, 0x3d, 0xbd, 0xa2, 0x69, 0xa1, 0xe2, 0x7c, 0x6f, 0x58, 0x28},
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockByID(context.Background(), id)
		assert.Equal(t, &block, result)
		assert.Equal(t, flowgosdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Equal(t, flowgosdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetCollectionByID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		id := flowgosdk.Identifier{}
		flowCollection := flowgo.LightCollection{}
		collection := convert.FlowLightCollectionToSDK(flowCollection)

		//success
		emu.EXPECT().
			GetCollectionByID(convert.SDKIdentifierToFlow(id)).
			Return(&flowCollection, nil).
			Times(1)

		result, err := adapter.GetCollectionByID(context.Background(), id)
		assert.Equal(t, collection, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetCollectionByID(convert.SDKIdentifierToFlow(id)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetCollectionByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransaction", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		id := flowgosdk.Identifier{}
		transaction := flowgo.TransactionBody{}
		expected := convert.FlowTransactionToSDK(transaction)

		//success
		emu.EXPECT().
			GetTransaction(convert.SDKIdentifierToFlow(id)).
			Return(&transaction, nil).
			Times(1)

		result, err := adapter.GetTransaction(context.Background(), id)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransaction(convert.SDKIdentifierToFlow(id)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransaction(context.Background(), id)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionResult", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		txID := flowgosdk.Identifier{}

		expected := flowgosdk.TransactionResult{
			Events: []flowgosdk.Event{},
		}
		flowResult := access.TransactionResult{}

		//success
		emu.EXPECT().
			GetTransactionResult(convert.SDKIdentifierToFlow(txID)).
			Return(&flowResult, nil).
			Times(1)

		result, err := adapter.GetTransactionResult(context.Background(), txID)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionResult(convert.SDKIdentifierToFlow(txID)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionResult(context.Background(), txID)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetAccount", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		address := flowgosdk.Address{}
		account := flowgo.Account{}

		expected, err := convert.FlowAccountToSDK(account)
		assert.NoError(t, err)

		//success
		emu.EXPECT().
			GetAccount(convert.SDKAddressToFlow(address)).
			Return(&account, nil).
			Times(1)

		result, err := adapter.GetAccount(context.Background(), address)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetAccount(convert.SDKAddressToFlow(address)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetAccount(context.Background(), address)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetAccountAtLatestBlock", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		address := flowgosdk.Address{}
		account := flowgo.Account{}

		expected, err := convert.FlowAccountToSDK(account)
		assert.NoError(t, err)

		//success
		emu.EXPECT().
			GetAccount(convert.SDKAddressToFlow(address)).
			Return(&account, nil).
			Times(1)

		result, err := adapter.GetAccountAtLatestBlock(context.Background(), address)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetAccount(convert.SDKAddressToFlow(address)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetAccountAtLatestBlock(context.Background(), address)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetAccountAtBlockHeight", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		height := uint64(42)

		address := flowgosdk.Address{}
		account := flowgo.Account{}

		expected, err := convert.FlowAccountToSDK(account)
		assert.NoError(t, err)

		//success
		emu.EXPECT().
			GetAccountAtBlockHeight(convert.SDKAddressToFlow(address), height).
			Return(&account, nil).
			Times(1)

		result, err := adapter.GetAccountAtBlockHeight(context.Background(), address, height)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetAccountAtBlockHeight(convert.SDKAddressToFlow(address), height).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetAccountAtBlockHeight(context.Background(), address, height)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("ExecuteScriptAtLatestBlock", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		script := []byte("some cadence code here")
		var arguments [][]byte

		stringValue, _ := cadence.NewString("42")
		emulatorResult := types.ScriptResult{Value: stringValue}
		expected, _ := convertScriptResult(&emulatorResult, nil)

		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(flowBlock, nil).
			Times(1)

		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, uint64(42)).
			Return(&emulatorResult, nil).
			Times(1)

		result, err := adapter.ExecuteScriptAtLatestBlock(context.Background(), script, arguments)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(flowBlock, nil).
			Times(1)

		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.ExecuteScriptAtLatestBlock(context.Background(), script, arguments)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("ExecuteScriptAtBlockHeight", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		script := []byte("some cadence code here")
		var arguments [][]byte

		height := uint64(42)
		stringValue, _ := cadence.NewString("42")
		emulatorResult := types.ScriptResult{Value: stringValue}
		expected, _ := convertScriptResult(&emulatorResult, nil)

		//success
		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, height).
			Return(&emulatorResult, nil).
			Times(1)

		result, err := adapter.ExecuteScriptAtBlockHeight(context.Background(), height, script, arguments)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, height).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.ExecuteScriptAtBlockHeight(context.Background(), height, script, arguments)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("ExecuteScriptAtBlockID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		script := []byte("some cadence code here")
		var arguments [][]byte

		id := flowgosdk.Identifier{}
		stringValue, _ := cadence.NewString("42")
		emulatorResult := types.ScriptResult{Value: stringValue}
		expected, _ := convertScriptResult(&emulatorResult, nil)

		flowBlock := &flowgo.Block{
			Header: &flowgo.Header{
				Height: 42,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(flowBlock, nil).
			Times(1)

		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, uint64(42)).
			Return(&emulatorResult, nil).
			Times(1)

		result, err := adapter.ExecuteScriptAtBlockID(context.Background(), id, script, arguments)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(flowBlock, nil).
			Times(1)

		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.ExecuteScriptAtBlockID(context.Background(), id, script, arguments)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetEventsForHeightRange", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		eventType := "testEvent"
		startHeight := uint64(0)
		endHeight := uint64(42)

		expected := []*flowgosdk.BlockEvents{}
		flowEvents := []flowgo.BlockEvents{}

		//success
		emu.EXPECT().
			GetEventsForHeightRange(eventType, startHeight, endHeight).
			Return(flowEvents, nil).
			Times(1)

		result, err := adapter.GetEventsForHeightRange(context.Background(), eventType, startHeight, endHeight)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetEventsForHeightRange(eventType, startHeight, endHeight).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetEventsForHeightRange(context.Background(), eventType, startHeight, endHeight)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetEventsForBlockIDs", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		eventType := "testEvent"
		blockIDs := []flowgosdk.Identifier{flowgosdk.Identifier{}}

		expected := []*flowgosdk.BlockEvents{}
		flowEvents := []flowgo.BlockEvents{}

		//success
		emu.EXPECT().
			GetEventsForBlockIDs(eventType, convert.SDKIdentifiersToFlow(blockIDs)).
			Return(flowEvents, nil).
			Times(1)

		result, err := adapter.GetEventsForBlockIDs(context.Background(), eventType, blockIDs)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetEventsForBlockIDs(eventType, convert.SDKIdentifiersToFlow(blockIDs)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetEventsForBlockIDs(context.Background(), eventType, blockIDs)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionsByBlockID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		blockID := flowgosdk.Identifier{}

		flowTransactions := []*flowgo.TransactionBody{}

		expected := []*flowgosdk.Transaction{}

		//success
		emu.EXPECT().
			GetTransactionsByBlockID(convert.SDKIdentifierToFlow(blockID)).
			Return(flowTransactions, nil).
			Times(1)

		result, err := adapter.GetTransactionsByBlockID(context.Background(), blockID)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionsByBlockID(convert.SDKIdentifierToFlow(blockID)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionsByBlockID(context.Background(), blockID)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionResultsByBlockID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		blockID := flowgosdk.Identifier{}

		flowTransactionResults := []*access.TransactionResult{}
		expected := []*flowgosdk.TransactionResult{}

		//success
		emu.EXPECT().
			GetTransactionResultsByBlockID(convert.SDKIdentifierToFlow(blockID)).
			Return(flowTransactionResults, nil).
			Times(1)

		result, err := adapter.GetTransactionResultsByBlockID(context.Background(), blockID)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionResultsByBlockID(convert.SDKIdentifierToFlow(blockID)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionResultsByBlockID(context.Background(), blockID)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("SendTransaction", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		transaction := flowgosdk.Transaction{}
		flowTransaction := convert.SDKTransactionToFlow(transaction)

		//success
		emu.EXPECT().
			SendTransaction(flowTransaction).
			Return(nil).
			Times(1)

		err := adapter.SendTransaction(context.Background(), transaction)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			SendTransaction(flowTransaction).
			Return(fmt.Errorf("some error")).
			Times(1)

		err = adapter.SendTransaction(context.Background(), transaction)
		assert.Error(t, err)

	}))

}
