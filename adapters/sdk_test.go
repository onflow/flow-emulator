/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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

package adapters

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/stdlib"
	flowsdk "github.com/onflow/flow-go-sdk"
	accessmodel "github.com/onflow/flow-go/model/access"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/emulator/mocks"
	"github.com/onflow/flow-emulator/types"
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

	t.Parallel()

	t.Run("Ping", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {
		emu.EXPECT().
			Ping().
			Times(1)

		err := adapter.Ping(context.Background())
		assert.NoError(t, err)
	}))

	// Ensure CreateAccount returns address when AccountCreated event is in previous result
	// (simulating scheduled transactions adding a trailing transaction result).
	t.Run("CreateAccount_fallback_to_previous_result_when_scheduled_transactions", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {
		ctx := context.Background()

		svcAddr := flowsdk.HexToAddress("0x01")
		serviceKey := emulator.DefaultServiceKey()
		serviceKey.Address = svcAddr
		emu.EXPECT().ServiceKey().Return(serviceKey).Times(1)

		latestBlock := &flowgo.Block{HeaderBody: flowgo.HeaderBody{Height: 10, ChainID: flowgo.Emulator, Timestamp: uint64(time.Now().UnixMilli())}}
		emu.EXPECT().GetLatestBlock().Return(latestBlock, nil).Times(1)

		// Capture txID to set on the matching TransactionResult
		var txID flowsdk.Identifier
		emu.EXPECT().SendTransaction(gomock.Any()).DoAndReturn(func(tx *flowgo.TransactionBody) error {
			txID = flowsdk.Identifier(tx.ID())
			return nil
		}).Times(1)

		// Build two results: prev (n-2) contains AccountCreated, last (n-1) is a scheduler tx without it
		newAccount := flowsdk.HexToAddress("0x02")
		createdEventType := flowsdk.EventAccountCreated
		createdEvent := flowsdk.Event{
			Type: createdEventType,
			Value: cadence.NewEvent([]cadence.Value{
				cadence.NewAddress(newAccount),
			}).WithType(cadence.NewEventType(nil, createdEventType, []cadence.Field{
				{Identifier: stdlib.AccountEventAddressParameter.Identifier, Type: cadence.AddressType},
			}, nil)),
		}

		// ExecuteAndCommitBlock returns results where len==2, with last having no AccountCreated
		emu.EXPECT().ExecuteAndCommitBlock().DoAndReturn(func() (*flowgo.Block, []*types.TransactionResult, error) {
			prevResult := &types.TransactionResult{
				TransactionID:   txID,
				ComputationUsed: 0,
				MemoryEstimate:  0,
				Error:           nil,
				Logs:            nil,
				Events:          []flowsdk.Event{createdEvent},
			}
			lastResult := &types.TransactionResult{
				ComputationUsed: 0,
				MemoryEstimate:  0,
				Error:           nil,
				Logs:            nil,
				Events:          []flowsdk.Event{},
			}
			return latestBlock, []*types.TransactionResult{prevResult, lastResult}, nil
		}).Times(1)
		emu.EXPECT().CommitBlock().Return(latestBlock, nil).Times(1)

		addr, err := adapter.CreateAccount(ctx, nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, newAccount, addr)
	}))

	// Ensure CreateAccount can scan multiple previous results (more than one trailing scheduler tx)
	t.Run("CreateAccount_fallback_scans_multiple_previous_results", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {
		ctx := context.Background()

		svcAddr := flowsdk.HexToAddress("0x01")
		serviceKey := emulator.DefaultServiceKey()
		serviceKey.Address = svcAddr
		emu.EXPECT().ServiceKey().Return(serviceKey).Times(1)

		latestBlock := &flowgo.Block{HeaderBody: flowgo.HeaderBody{Height: 11, ChainID: flowgo.Emulator, Timestamp: uint64(time.Now().UnixMilli())}}
		emu.EXPECT().GetLatestBlock().Return(latestBlock, nil).Times(1)

		var txID flowsdk.Identifier
		emu.EXPECT().SendTransaction(gomock.Any()).DoAndReturn(func(tx *flowgo.TransactionBody) error {
			txID = flowsdk.Identifier(tx.ID())
			return nil
		}).Times(1)

		newAccount := flowsdk.HexToAddress("0x03")
		createdEventType := flowsdk.EventAccountCreated
		createdEvent := flowsdk.Event{
			Type: createdEventType,
			Value: cadence.NewEvent([]cadence.Value{
				cadence.NewAddress(newAccount),
			}).WithType(cadence.NewEventType(nil, createdEventType, []cadence.Field{
				{Identifier: stdlib.AccountEventAddressParameter.Identifier, Type: cadence.AddressType},
			}, nil)),
		}

		emu.EXPECT().ExecuteAndCommitBlock().DoAndReturn(func() (*flowgo.Block, []*types.TransactionResult, error) {
			resultWithAccount := &types.TransactionResult{TransactionID: txID, Events: []flowsdk.Event{createdEvent}}
			trailing1 := &types.TransactionResult{Events: []flowsdk.Event{}}
			trailing2 := &types.TransactionResult{Events: []flowsdk.Event{}}
			return latestBlock, []*types.TransactionResult{resultWithAccount, trailing2, trailing1}, nil
		}).Times(1)
		emu.EXPECT().CommitBlock().Return(latestBlock, nil).Times(1)

		addr, err := adapter.CreateAccount(ctx, nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, newAccount, addr)
	}))

	t.Run("GetLatestBlockHeader", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
				Height:    42,
				ChainID:   flowgo.Emulator,
				Timestamp: uint64(timestamp.UnixMilli()),
			},
		}

		block := flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{
				ID:        flowsdk.Identifier{0xef, 0x7a, 0x85, 0xb0, 0xe1, 0xa9, 0x5b, 0xb5, 0xd2, 0x86, 0xe8, 0x9a, 0x55, 0x30, 0x7a, 0x95, 0x64, 0x1d, 0x8, 0x89, 0x53, 0x74, 0xa2, 0x2d, 0x0, 0xc7, 0xe2, 0xee, 0x1f, 0x85, 0x9, 0x7},
				Height:    42,
				Timestamp: timestamp,
			},
		}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetLatestBlockHeader(context.Background(), true)
		// check all values match except timestamp (explicitly  not checked in assert)
		assert.Equal(t, &block.BlockHeader, result)
		assert.Equal(t, block.Timestamp.Equal(result.Timestamp), true)
		assert.Equal(t, flowsdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetLatestBlockHeader(context.Background(), true)
		assert.Nil(t, result)
		assert.Equal(t, flowsdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockHeaderByHeight", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
				Height:    42,
				ChainID:   flowgo.Emulator,
				Timestamp: uint64(timestamp.UnixMilli()),
			},
		}

		block := flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{
				ID:        flowsdk.Identifier{0xef, 0x7a, 0x85, 0xb0, 0xe1, 0xa9, 0x5b, 0xb5, 0xd2, 0x86, 0xe8, 0x9a, 0x55, 0x30, 0x7a, 0x95, 0x64, 0x1d, 0x8, 0x89, 0x53, 0x74, 0xa2, 0x2d, 0x0, 0xc7, 0xe2, 0xee, 0x1f, 0x85, 0x9, 0x7},
				Height:    42,
				Timestamp: timestamp,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockHeaderByHeight(context.Background(), 42)
		assert.Equal(t, &block.BlockHeader, result)
		assert.Equal(t, flowsdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockHeaderByHeight(context.Background(), 42)
		assert.Nil(t, result)
		assert.Equal(t, flowsdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockHeaderByID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		id := flowsdk.Identifier{}
		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
				Height:    42,
				ChainID:   flowgo.Emulator,
				Timestamp: uint64(timestamp.UnixMilli()),
			},
		}

		block := flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{
				ID:        flowsdk.Identifier{0xef, 0x7a, 0x85, 0xb0, 0xe1, 0xa9, 0x5b, 0xb5, 0xd2, 0x86, 0xe8, 0x9a, 0x55, 0x30, 0x7a, 0x95, 0x64, 0x1d, 0x8, 0x89, 0x53, 0x74, 0xa2, 0x2d, 0x0, 0xc7, 0xe2, 0xee, 0x1f, 0x85, 0x9, 0x7},
				Height:    42,
				Timestamp: timestamp,
			},
		}

		//success
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockHeaderByID(context.Background(), id)
		assert.Equal(t, &block.BlockHeader, result)
		assert.Equal(t, flowsdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockHeaderByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Equal(t, flowsdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetLatestBlock", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
				Height:    42,
				ChainID:   flowgo.Emulator,
				Timestamp: uint64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()),
			},
			Payload: flowgo.Payload{
				Guarantees: []*flowgo.CollectionGuarantee{
					{
						CollectionID: flowgo.MustHexStringToIdentifier("db94e7ef4c9e758f27f96777c61b5cca10528e9db5e7dfd3b44ffceb26b284c0"),
					},
				},
				Seals: []*flowgo.Seal{
					{
						BlockID:  flowgo.MustHexStringToIdentifier("890581b4ee0666d2a90b7e9212aaa37535f7bcec76f571c3402bc4bc58ee2918"),
						ResultID: flowgo.MustHexStringToIdentifier("a7990b0bab754a68844de3698bb2d2c7966acb9ef65fd5a3a5be53a93a764edf"),
					},
				},
			},
		}

		block := flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{
				ID:        flowsdk.Identifier{0xc7, 0xd7, 0x53, 0x11, 0xee, 0x39, 0x6b, 0x54, 0x1a, 0x0c, 0x2e, 0x55, 0xcf, 0x80, 0xab, 0xce, 0x09, 0x2d, 0x86, 0x11, 0xb7, 0xce, 0xa4, 0xc9, 0x79, 0x13, 0x7e, 0x73, 0x1f, 0x52, 0x3b, 0x1a},
				Height:    42,
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			BlockPayload: flowsdk.BlockPayload{
				CollectionGuarantees: []*flowsdk.CollectionGuarantee{
					{
						CollectionID: flowsdk.HexToID("db94e7ef4c9e758f27f96777c61b5cca10528e9db5e7dfd3b44ffceb26b284c0"),
					},
				},
				Seals: []*flowsdk.BlockSeal{
					{
						BlockID:  flowsdk.HexToID("890581b4ee0666d2a90b7e9212aaa37535f7bcec76f571c3402bc4bc58ee2918"),
						ResultId: flowsdk.HexToID("a7990b0bab754a68844de3698bb2d2c7966acb9ef65fd5a3a5be53a93a764edf"),
					},
				},
			},
		}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetLatestBlock(context.Background(), true)
		assert.Equal(t, &block, result)
		assert.Equal(t, flowsdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetLatestBlock(context.Background(), true)
		assert.Nil(t, result)
		assert.Equal(t, flowsdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockByHeight", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
				Height:    42,
				ChainID:   flowgo.Emulator,
				Timestamp: uint64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()),
			},
			Payload: flowgo.Payload{
				Guarantees: []*flowgo.CollectionGuarantee{
					{
						CollectionID: flowgo.MustHexStringToIdentifier("db94e7ef4c9e758f27f96777c61b5cca10528e9db5e7dfd3b44ffceb26b284c0"),
					},
				},
				Seals: []*flowgo.Seal{
					{
						BlockID:  flowgo.MustHexStringToIdentifier("890581b4ee0666d2a90b7e9212aaa37535f7bcec76f571c3402bc4bc58ee2918"),
						ResultID: flowgo.MustHexStringToIdentifier("a7990b0bab754a68844de3698bb2d2c7966acb9ef65fd5a3a5be53a93a764edf"),
					},
				},
			},
		}

		block := flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{
				ID:        flowsdk.Identifier{0xc7, 0xd7, 0x53, 0x11, 0xee, 0x39, 0x6b, 0x54, 0x1a, 0x0c, 0x2e, 0x55, 0xcf, 0x80, 0xab, 0xce, 0x09, 0x2d, 0x86, 0x11, 0xb7, 0xce, 0xa4, 0xc9, 0x79, 0x13, 0x7e, 0x73, 0x1f, 0x52, 0x3b, 0x1a},
				Height:    42,
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			BlockPayload: flowsdk.BlockPayload{
				CollectionGuarantees: []*flowsdk.CollectionGuarantee{
					{
						CollectionID: flowsdk.HexToID("db94e7ef4c9e758f27f96777c61b5cca10528e9db5e7dfd3b44ffceb26b284c0"),
					},
				},
				Seals: []*flowsdk.BlockSeal{
					{
						BlockID:  flowsdk.HexToID("890581b4ee0666d2a90b7e9212aaa37535f7bcec76f571c3402bc4bc58ee2918"),
						ResultId: flowsdk.HexToID("a7990b0bab754a68844de3698bb2d2c7966acb9ef65fd5a3a5be53a93a764edf"),
					},
				},
			},
		}

		//success
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockByHeight(context.Background(), 42)
		assert.Equal(t, &block, result)
		assert.Equal(t, flowsdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockByHeight(context.Background(), 42)
		assert.Nil(t, result)
		assert.Equal(t, flowsdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockByID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		id := flowsdk.Identifier{}
		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
				Height:    42,
				ChainID:   flowgo.Emulator,
				Timestamp: uint64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()),
			},
			Payload: flowgo.Payload{
				Guarantees: []*flowgo.CollectionGuarantee{
					{
						CollectionID: flowgo.MustHexStringToIdentifier("db94e7ef4c9e758f27f96777c61b5cca10528e9db5e7dfd3b44ffceb26b284c0"),
					},
				},
				Seals: []*flowgo.Seal{
					{
						BlockID:  flowgo.MustHexStringToIdentifier("890581b4ee0666d2a90b7e9212aaa37535f7bcec76f571c3402bc4bc58ee2918"),
						ResultID: flowgo.MustHexStringToIdentifier("a7990b0bab754a68844de3698bb2d2c7966acb9ef65fd5a3a5be53a93a764edf"),
					},
				},
			},
		}

		block := flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{
				ID:        flowsdk.Identifier{0xc7, 0xd7, 0x53, 0x11, 0xee, 0x39, 0x6b, 0x54, 0x1a, 0x0c, 0x2e, 0x55, 0xcf, 0x80, 0xab, 0xce, 0x09, 0x2d, 0x86, 0x11, 0xb7, 0xce, 0xa4, 0xc9, 0x79, 0x13, 0x7e, 0x73, 0x1f, 0x52, 0x3b, 0x1a},
				Height:    42,
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			BlockPayload: flowsdk.BlockPayload{
				CollectionGuarantees: []*flowsdk.CollectionGuarantee{
					{
						CollectionID: flowsdk.HexToID("db94e7ef4c9e758f27f96777c61b5cca10528e9db5e7dfd3b44ffceb26b284c0"),
					},
				},
				Seals: []*flowsdk.BlockSeal{
					{
						BlockID:  flowsdk.HexToID("890581b4ee0666d2a90b7e9212aaa37535f7bcec76f571c3402bc4bc58ee2918"),
						ResultId: flowsdk.HexToID("a7990b0bab754a68844de3698bb2d2c7966acb9ef65fd5a3a5be53a93a764edf"),
					},
				},
			},
		}

		//success
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(flowBlock, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockByID(context.Background(), id)
		assert.Equal(t, &block, result)
		assert.Equal(t, flowsdk.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(convert.SDKIdentifierToFlow(id)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Equal(t, flowsdk.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetCollectionByID", sdkTest(func(t *testing.T, adapter *SDKAdapter, emu *mocks.MockEmulator) {

		id := flowsdk.Identifier{}
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

		id := flowsdk.Identifier{}
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

		txID := flowsdk.Identifier{}

		expected := flowsdk.TransactionResult{
			Events: []flowsdk.Event{},
		}
		flowResult := accessmodel.TransactionResult{}

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

		address := flowsdk.Address{}
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

		address := flowsdk.Address{}
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

		address := flowsdk.Address{}
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
			HeaderBody: flowgo.HeaderBody{
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

		id := flowsdk.Identifier{}
		stringValue, _ := cadence.NewString("42")
		emulatorResult := types.ScriptResult{Value: stringValue}
		expected, _ := convertScriptResult(&emulatorResult, nil)

		flowBlock := &flowgo.Block{
			HeaderBody: flowgo.HeaderBody{
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

		expected := []*flowsdk.BlockEvents{}
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
		blockIDs := []flowsdk.Identifier{flowsdk.Identifier{}}

		expected := []*flowsdk.BlockEvents{}
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

		blockID := flowsdk.Identifier{}

		flowTransactions := []*flowgo.TransactionBody{}

		expected := []*flowsdk.Transaction{}

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

		blockID := flowsdk.Identifier{}

		flowTransactionResults := []*accessmodel.TransactionResult{}
		expected := []*flowsdk.TransactionResult{}

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

		transaction := flowsdk.Transaction{}
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
