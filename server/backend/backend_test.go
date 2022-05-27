/*
 * Flow Emulator
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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

package backend_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-emulator/server/backend/mocks"
	"github.com/onflow/flow-emulator/types"
)

func backendTest(f func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator)) func(t *testing.T) {
	return func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		emu := mocks.NewMockEmulator(mockCtrl)
		back := backend.New(logrus.New(), emu)

		f(t, back, emu)
	}
}

func TestBackend(t *testing.T) {

	t.Parallel()

	ids := test.IdentifierGenerator()
	results := test.TransactionResultGenerator()

	t.Run("Ping", backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
		err := backend.Ping(context.Background())
		assert.NoError(t, err)
	}))

	t.Run(
		"ExecuteScriptAtLatestBlock",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			script := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")

			expectedValue := cadence.NewInt(rand.Int())

			latestBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil).
				Times(1)

			emu.EXPECT().
				ExecuteScriptAtBlock(script, nil, latestBlock.Header.Height).
				Return(&types.ScriptResult{
					Value: expectedValue,
					Error: nil,
				}, nil).
				Times(1)

			value, err := backend.ExecuteScriptAtLatestBlock(context.Background(), script, nil)
			assert.NoError(t, err)

			actualValue, err := jsoncdc.Decode(nil, value)
			assert.NoError(t, err)

			assert.Equal(t, expectedValue, actualValue)
		}),
	)

	t.Run(
		"ExecuteScriptAtLatestBlock fails with error",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			script := []byte("I will fail you!")
			scriptErr := fmt.Errorf("failure description")

			latestBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil).
				Times(1)

			emu.EXPECT().
				ExecuteScriptAtBlock(script, nil, latestBlock.Header.Height).
				Return(&types.ScriptResult{
					Value: nil,
					Error: scriptErr,
				}, nil).
				Times(1)

			value, err := backend.ExecuteScriptAtLatestBlock(context.Background(), script, nil)
			assert.Error(t, err)
			assert.Nil(t, value)
			assert.Equal(t, err.Error(), scriptErr.Error())
		}),
	)

	t.Run(
		"ExecuteScriptAtBlockHeight",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			script := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
			blockHeight := rand.Uint64()

			expectedValue := cadence.NewInt(rand.Int())

			emu.EXPECT().
				ExecuteScriptAtBlock(script, nil, blockHeight).
				Return(&types.ScriptResult{
					Value: expectedValue,
					Error: nil,
				}, nil).
				Times(1)

			value, err := backend.ExecuteScriptAtBlockHeight(context.Background(), blockHeight, script, nil)
			assert.NoError(t, err)

			actualValue, err := jsoncdc.Decode(nil, value)
			assert.NoError(t, err)

			assert.Equal(t, expectedValue, actualValue)
		}),
	)

	t.Run(
		"ExecuteScriptAtBlockID",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			script := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
			expectedValue := cadence.NewInt(rand.Int())

			randomBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}

			emu.EXPECT().
				GetBlockByID(flow.Identifier(randomBlock.ID())).
				Return(&randomBlock, nil).
				Times(1)

			emu.EXPECT().
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

			actualValue, err := jsoncdc.Decode(nil, value)
			assert.NoError(t, err)

			assert.Equal(t, expectedValue, actualValue)
		}),
	)

	t.Run(
		"GetAccountAtLatestBlock",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			expectedAccount := test.AccountGenerator().New()

			emu.EXPECT().
				GetAccount(expectedAccount.Address).
				Return(expectedAccount, nil).
				Times(1)

			actualAccount, err := backend.GetAccountAtLatestBlock(context.Background(), expectedAccount.Address)
			require.NoError(t, err)

			assert.Equal(t, *expectedAccount, *actualAccount)
		}),
	)

	t.Run(
		"GetEventsForHeightRange fails with invalid block heights",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			latestBlock := flowgo.Block{Header: &flowgo.Header{Height: 21}}

			eventType := "SomeEvent"
			startHeight := uint64(37)
			endHeight := uint64(21)

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil)

			emu.EXPECT().
				GetEventsByHeight(gomock.Any(), gomock.Any()).
				Return([]flow.Event{}, nil).
				Times(0)

			_, err := backend.GetEventsForHeightRange(context.Background(), eventType, startHeight, endHeight)
			require.Error(t, err)

			grpcError, ok := status.FromError(err)
			require.True(t, ok)

			assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
		}),
	)

	t.Run(
		"GetEventsForHeightRange fails if blockchain returns error",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			startBlock := flowgo.Block{Header: &flowgo.Header{Height: 10}}
			latestBlock := flowgo.Block{Header: &flowgo.Header{Height: 11}}

			eventType := "SomeEvent"

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil)

			emu.EXPECT().
				GetBlockByHeight(startBlock.Header.Height).
				Return(&startBlock, nil).
				Times(1)

			emu.EXPECT().
				GetEventsByHeight(startBlock.Header.Height, gomock.Any()).
				Return(nil, errors.New("dummy")).
				Times(1)

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
		}),
	)

	t.Run(
		"GetEventsForHeightRange",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			events := test.EventGenerator()

			eventsToReturn := []flow.Event{
				events.New(),
				events.New(),
			}

			blocks := []flowgo.Block{
				{Header: &flowgo.Header{Height: 1}},
				{Header: &flowgo.Header{Height: 2}},
			}

			var (
				eventType   = "SomeEvent"
				latestBlock = blocks[1]
				startHeight = blocks[0].Header.Height
				endHeight   = blocks[1].Header.Height
			)

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil)

			emu.EXPECT().
				GetBlockByHeight(blocks[0].Header.Height).
				Return(&blocks[0], nil)

			emu.EXPECT().
				GetBlockByHeight(blocks[1].Header.Height).
				Return(&blocks[1], nil)

			emu.EXPECT().
				GetEventsByHeight(blocks[0].Header.Height, eventType).
				Return(eventsToReturn, nil)

			emu.EXPECT().
				GetEventsByHeight(blocks[1].Header.Height, eventType).
				Return(eventsToReturn, nil)

			results, err := backend.GetEventsForHeightRange(context.Background(),
				eventType,
				startHeight,
				endHeight,
			)
			require.NoError(t, err)

			assert.NotNil(t, results)

			require.Len(t, results, 2)

			for i, block := range results {
				assert.Len(t, block.Events, 2)
				assert.Equal(t, block.BlockID, blocks[i].ID())
				assert.Equal(t, block.BlockHeight, blocks[i].Header.Height)
			}
		}),
	)

	t.Run(
		"GetEventsForHeightRange succeeds if end height is higher than latest",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
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

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil)

			emu.EXPECT().
				GetBlockByHeight(blocks[0].Header.Height).
				Return(&blocks[0], nil)

			emu.EXPECT().
				GetBlockByHeight(blocks[1].Header.Height).
				Return(&blocks[1], nil)

			emu.EXPECT().
				GetEventsByHeight(blocks[0].Header.Height, eventType).
				Return(eventsToReturn, nil)

			emu.EXPECT().
				GetEventsByHeight(blocks[1].Header.Height, eventType).
				Return(eventsToReturn, nil)

			results, err := backend.GetEventsForHeightRange(context.Background(),
				eventType,
				startHeight,
				endHeight,
			)
			require.NoError(t, err)

			assert.NotNil(t, results)

			require.Len(t, results, 2)

			for i, block := range results {
				assert.Len(t, block.Events, 2)
				assert.Equal(t, block.BlockID, blocks[i].ID())
				assert.Equal(t, block.BlockHeight, blocks[i].Header.Height)
			}
		}),
	)

	t.Run(
		"GetEventsForHeightRange fails with empty eventType",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {

			eventType := " "
			height := uint64(1)

			_, err := backend.GetEventsForHeightRange(context.Background(), eventType, height, height)
			require.Error(t, err)

			grpcError, ok := status.FromError(err)
			require.True(t, ok)

			assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
		}),
	)

	t.Run(
		"GetLatestBlockHeader",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			parentID := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}.ID()
			latestBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64(), ParentID: parentID}}

			emu.EXPECT().
				GetLatestBlock().
				Return(&latestBlock, nil).
				Times(1)

			header, err := backend.GetLatestBlockHeader(context.Background(), false)
			assert.NoError(t, err)

			assert.Equal(t, latestBlock.Header.Height, header.Height)
			assert.Equal(t, latestBlock.ID(), header.ID())
			assert.Equal(t, latestBlock.Header.ParentID, header.ParentID)
		}),
	)

	t.Run(
		"GetBlockHeaderAtBlockHeight",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			parentID := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}.ID()
			requestedBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64(), ParentID: parentID}}

			emu.EXPECT().
				GetBlockByHeight(requestedBlock.Header.Height).
				Return(&requestedBlock, nil).
				Times(1)

			header, err := backend.GetBlockHeaderByHeight(context.Background(), requestedBlock.Header.Height)
			assert.NoError(t, err)

			assert.Equal(t, requestedBlock.Header.Height, header.Height)
			assert.Equal(t, requestedBlock.ID(), header.ID())
			assert.Equal(t, requestedBlock.Header.ParentID, header.ParentID)
		}),
	)

	t.Run(
		"GetBlockHeaderAtBlockID",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			parentID := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64()}}.ID()
			requestedBlock := flowgo.Block{Header: &flowgo.Header{Height: rand.Uint64(), ParentID: parentID}}

			emu.EXPECT().
				GetBlockByID(flow.Identifier(requestedBlock.ID())).
				Return(&requestedBlock, nil).
				Times(1)

			header, err := backend.GetBlockHeaderByID(context.Background(), flow.Identifier(requestedBlock.ID()))
			assert.NoError(t, err)

			assert.Equal(t, requestedBlock.Header.Height, header.Height)
			assert.Equal(t, requestedBlock.ID(), header.ID())
			assert.Equal(t, requestedBlock.Header.ParentID, header.ParentID)
		}),
	)

	t.Run(
		"GetTransaction tx does not exists",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			txID := ids.New()

			emu.EXPECT().
				GetTransaction(txID).
				Return(nil, &emulator.TransactionNotFoundError{}).
				Times(1)

			tx, err := backend.GetTransaction(context.Background(), txID)
			require.Error(t, err)

			grpcError, ok := status.FromError(err)
			require.True(t, ok)

			assert.Equal(t, codes.NotFound, grpcError.Code())

			assert.Nil(t, tx)
		}),
	)

	t.Run(
		"GetTransactionResult",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
			txID := ids.New()
			expectedTx := results.New()

			emu.EXPECT().
				GetTransactionResult(txID).
				Return(&expectedTx, nil).
				Times(1)

			actualTx, err := backend.GetTransactionResult(context.Background(), txID)
			require.NoError(t, err)

			assert.Equal(t, expectedTx, *actualTx)
		}),
	)

	t.Run("SendTransaction", backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {

		var actualTx flow.Transaction

		emu.EXPECT().
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

	t.Run(
		"SendTransaction which errors while processing",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {

			emu.EXPECT().
				AddTransaction(gomock.Any()).
				Return(&types.FlowError{FlowError: &fvmerrors.AccountAuthorizationError{}}).
				Times(1)

			expectedTx := test.TransactionGenerator().New()

			// remove payer to make transaction invalid
			expectedTx.Payer = flow.Address{}

			err := backend.SendTransaction(context.Background(), *expectedTx)
			require.Error(t, err)

			grpcError, ok := status.FromError(err)
			require.True(t, ok)

			assert.Equal(t, codes.InvalidArgument, grpcError.Code())
		}),
	)

	t.Run(
		"GetEventsForBlockIDs",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {
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

			emu.EXPECT().
				GetBlockByID(flow.Identifier(blocks[0].ID())).
				Return(&blocks[0], nil)

			emu.EXPECT().
				GetBlockByID(flow.Identifier(blocks[1].ID())).
				Return(&blocks[1], nil)

			emu.EXPECT().
				GetEventsByHeight(blocks[0].Header.Height, eventType).
				Return(eventsToReturn, nil)

			emu.EXPECT().
				GetEventsByHeight(blocks[1].Header.Height, eventType).
				Return(eventsToReturn, nil)

			blockIDs := make([]flow.Identifier, len(blocks))
			for i, b := range blocks {
				blockIDs[i] = flow.Identifier(b.ID())
			}

			results, err := backend.GetEventsForBlockIDs(context.Background(),
				eventType,
				blockIDs,
			)
			require.NoError(t, err)

			assert.NotNil(t, results)

			require.Len(t, results, 2)

			for i, block := range results {
				assert.Len(t, block.Events, 2)
				assert.Equal(t, block.BlockID, blocks[i].ID())
				assert.Equal(t, block.BlockHeight, blocks[i].Header.Height)
			}
		}),
	)

	t.Run(
		"GetEventsForBlockIDs fails with empty eventType",
		backendTest(func(t *testing.T, backend *backend.Backend, emu *mocks.MockEmulator) {

			eventType := " "

			blocks := []flowgo.Block{
				{Header: &flowgo.Header{Height: 1}},
				{Header: &flowgo.Header{Height: 2}},
			}

			blockIDs := make([]flow.Identifier, len(blocks))
			for i, b := range blocks {
				blockIDs[i] = flow.Identifier(b.ID())
			}

			_, err := backend.GetEventsForBlockIDs(context.Background(),
				eventType,
				blockIDs,
			)
			require.Error(t, err)

			grpcError, ok := status.FromError(err)
			require.True(t, ok)

			assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
		}),
	)
}

func TestBackendAutoMine(t *testing.T) {

	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	emu := mocks.NewMockEmulator(mockCtrl)

	backend := backend.New(logrus.New(), emu)

	// enable automine flag
	backend.EnableAutoMine()
	// disable automine flag at the end of the test
	defer backend.DisableAutoMine()

	var actualTx flow.Transaction

	emu.EXPECT().
		AddTransaction(gomock.Any()).
		DoAndReturn(func(tx flow.Transaction) error {
			actualTx = tx
			return nil
		}).
		Times(1)

	// expect transaction to be executed immediately
	emu.EXPECT().
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
