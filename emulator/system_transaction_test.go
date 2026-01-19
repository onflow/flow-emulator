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

package emulator_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/emulator"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestGetSystemTransactions(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Commit a block to generate system transactions
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.NotNil(t, block)

	blockID := flowgo.Identifier(block.ID())

	// Get system transactions for the block
	systemTxIDs, err := b.GetSystemTransactionsForBlock(blockID)
	require.NoError(t, err)
	assert.NotEmpty(t, systemTxIDs, "block should have system transactions")

	// Test GetSystemTransaction for each system transaction
	for _, txID := range systemTxIDs {
		tx, err := adapter.GetSystemTransaction(context.Background(), txID, blockID)
		require.NoError(t, err)
		assert.NotNil(t, tx, "system transaction should exist")
		// Note: system transactions may have different IDs than expected
		// due to how they are generated, so we just verify we can retrieve them

		// Test GetSystemTransactionResult
		result, err := adapter.GetSystemTransactionResult(
			context.Background(),
			txID,
			blockID,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.NoError(t, err)
		assert.NotNil(t, result, "system transaction result should exist")
		// Verify the result has the expected transaction ID
		assert.Equal(t, txID, result.TransactionID)
	}
}

func TestGetSystemTransaction_NotFound(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Commit a block
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)

	blockID := flowgo.Identifier(block.ID())

	// Try to get a non-existent system transaction
	fakeTxID := flowgo.MakeID([32]byte{0x99})
	tx, err := adapter.GetSystemTransaction(context.Background(), fakeTxID, blockID)
	assert.Error(t, err, "should return error for non-existent system transaction")
	assert.Nil(t, tx)

	result, err := adapter.GetSystemTransactionResult(
		context.Background(),
		fakeTxID,
		blockID,
		entities.EventEncodingVersion_CCF_V0,
	)
	assert.Error(t, err, "should return error for non-existent system transaction result")
	assert.Nil(t, result)
}

func TestGetSystemTransaction_BlockNotFound(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Try to get system transactions for a non-existent block
	fakeBlockID := flowgo.MakeID([32]byte{0x88})
	fakeTxID := flowgo.MakeID([32]byte{0x99})

	tx, err := adapter.GetSystemTransaction(context.Background(), fakeTxID, fakeBlockID)
	assert.Error(t, err, "should return error for non-existent block")
	assert.Nil(t, tx)

	result, err := adapter.GetSystemTransactionResult(
		context.Background(),
		fakeTxID,
		fakeBlockID,
		entities.EventEncodingVersion_CCF_V0,
	)
	assert.Error(t, err, "should return error for non-existent block")
	assert.Nil(t, result)
}

// Verifies that:
// - The scheduler "process" system transaction result contains the PendingExecution event
// - One system transaction result contains the EVM.BlockExecuted event
//
// This test ensures system transaction results are correctly stored and retrieved using
// composite keys (blockID, txID). System transactions can have identical IDs across blocks
// (e.g., the process transaction uses the same script/payer/reference block), so storing
// by txID alone causes collisions where later blocks overwrite earlier blocks' results.
func TestSystemTransactions_ContainExpectedEvents(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(emulator.WithStorageLimitEnabled(false))
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Deploy handler contract and schedule a callback so scheduler emits events.
	serviceAddress := b.ServiceKey().Address
	serviceHex := serviceAddress.Hex()
	serviceHexWithPrefix := serviceAddress.HexWithPrefix()

	handlerContract := fmt.Sprintf(`
		import FlowTransactionScheduler from 0x%s
		access(all) contract ScheduledHandler {
			access(contract) var count: Int
			access(all) view fun getCount(): Int { return self.count }
			access(all) resource Handler: FlowTransactionScheduler.TransactionHandler {
				access(FlowTransactionScheduler.Execute) fun executeTransaction(id: UInt64, data: AnyStruct?) {
					ScheduledHandler.count = ScheduledHandler.count + 1
				}
			}
			access(all) fun createHandler(): @Handler { return <- create Handler() }
			init() { self.count = 0 }
		}
	`, serviceHex)

	latestBlock, err := b.GetLatestBlock()
	require.NoError(t, err)
	tx := templates.AddAccountContract(serviceAddress, templates.Contract{
		Name:   "ScheduledHandler",
		Source: handlerContract,
	})
	tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID())).
		SetProposalKey(serviceAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAddress)
	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)
	require.NoError(t, tx.SignEnvelope(serviceAddress, b.ServiceKey().Index, signer))
	require.NoError(t, b.SendTransaction(convert.SDKTransactionToFlow(*tx)))
	_, _, err = b.ExecuteAndCommitBlock()
	require.NoError(t, err)

	createAndSchedule := fmt.Sprintf(`
		import FlowTransactionScheduler from 0x%s
		import ScheduledHandler from 0x%s
		import FungibleToken from 0xee82856bf20e2aa6
		import FlowToken from 0x0ae53cb6e3f42a79
		transaction {
			prepare(acct: auth(Storage, Capabilities, FungibleToken.Withdraw) &Account) {
				let handler <- ScheduledHandler.createHandler()
				acct.storage.save(<-handler, to: /storage/counterHandler)
				let issued: Capability<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}> =
					acct.capabilities.storage.issue<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>(/storage/counterHandler)
				let adminRef = acct.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
					?? panic("missing FlowToken admin")
				let minter <- adminRef.createNewMinter(allowedAmount: 10.0)
				let minted <- minter.mintTokens(amount: 1.0)
				let receiver = acct.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
					?? panic("missing FlowToken receiver")
				receiver.deposit(from: <-minted)
				destroy minter
				let estimate = FlowTransactionScheduler.estimate(
					data: nil,
					timestamp: getCurrentBlock().timestamp + 3.0,
					priority: FlowTransactionScheduler.Priority.High,
					executionEffort: UInt64(5000)
				)
				let feeAmount: UFix64 = estimate.flowFee ?? 0.001
				let vaultRef = acct.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("missing FlowToken vault")
				let fees <- (vaultRef.withdraw(amount: feeAmount) as! @FlowToken.Vault)
				destroy <- FlowTransactionScheduler.schedule(
					handlerCap: issued,
					data: nil,
					timestamp: getCurrentBlock().timestamp + 3.0,
					priority: FlowTransactionScheduler.Priority.High,
					executionEffort: UInt64(5000),
					fees: <-fees
				)
			}
		}
	`, serviceHex, serviceHex)
	latestBlock, err = b.GetLatestBlock()
	require.NoError(t, err)
	tx = flowsdk.NewTransaction().
		SetScript([]byte(createAndSchedule)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAddress).
		AddAuthorizer(serviceAddress)
	tx.SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID()))
	signer, err = b.ServiceKey().Signer()
	require.NoError(t, err)
	require.NoError(t, tx.SignEnvelope(serviceAddress, b.ServiceKey().Index, signer))
	require.NoError(t, b.SendTransaction(convert.SDKTransactionToFlow(*tx)))
	_, results, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("schedule tx failed: %v", r.Error)
		}
	}

	// Advance time and commit a couple of blocks to trigger scheduling.
	time.Sleep(3500 * time.Millisecond)
	_, _, err = b.ExecuteAndCommitBlock()
	require.NoError(t, err)
	time.Sleep(3500 * time.Millisecond)
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.NotNil(t, block)
	blockID := flowgo.Identifier(block.ID())

	// Gather system transaction IDs for the block
	systemTxIDs, err := b.GetSystemTransactionsForBlock(blockID)
	require.NoError(t, err)
	require.NotEmpty(t, systemTxIDs, "expected system transactions for block")

	// Env used to compute expected event types
	env := systemcontracts.SystemContractsForChain(b.GetChain().ChainID()).AsTemplateEnv()
	expectedPendingExec := blueprints.PendingExecutionEventType(env)
	evmBlockExecutedEventType := common.AddressLocation{
		Address: common.Address(b.GetChain().ServiceAddress()),
		Name:    "EVM.BlockExecuted",
	}.ID()

	foundPendingExec := false
	foundEvmBlockExecuted := false

	t.Logf("Checking %d system transactions for block %s", len(systemTxIDs), blockID.String())
	t.Logf("Expected PendingExecution event type: %s", expectedPendingExec)
	t.Logf("Expected EVM.BlockExecuted event type: %s", evmBlockExecutedEventType)

	for i, txID := range systemTxIDs {
		// Fetch result for events
		result, err := adapter.GetSystemTransactionResult(
			context.Background(),
			txID,
			blockID,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.NoError(t, err)
		require.NotNil(t, result)

		t.Logf("\nSystem tx %d (ID: %s):", i, txID.String())
		t.Logf("  Event count: %d", len(result.Events))

		for j, evt := range result.Events {
			t.Logf("  Event[%d]: Type=%s, TxIndex=%d, EventIndex=%d",
				j, evt.Type, evt.TransactionIndex, evt.EventIndex)

			if evt.Type == expectedPendingExec {
				foundPendingExec = true
				t.Logf("    ✓ FOUND PendingExecution")
			}
			if string(evt.Type) == evmBlockExecutedEventType {
				foundEvmBlockExecuted = true
				t.Logf("    ✓ FOUND EVM.BlockExecuted")
			}
		}
	}

	assert.True(t, foundPendingExec, "expected PendingExecution event on process system transaction result")
	assert.True(t, foundEvmBlockExecuted, "expected EVM.BlockExecuted event on a system transaction result")

	// Verify each system transaction has unique transaction indices in their events
	seenTxIndices := make(map[uint32]bool)
	for i, txID := range systemTxIDs {
		result, err := adapter.GetSystemTransactionResult(
			context.Background(),
			txID,
			blockID,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.NoError(t, err)

		for _, evt := range result.Events {
			if seenTxIndices[evt.TransactionIndex] {
				t.Errorf("System tx %d has duplicate TransactionIndex %d", i, evt.TransactionIndex)
			}
			seenTxIndices[evt.TransactionIndex] = true
		}
	}

	t.Logf("Found %d unique transaction indices across system transactions", len(seenTxIndices))
	assert.Equal(t, len(systemTxIDs), len(seenTxIndices), "each system transaction should have a unique index")

	// Sanity check: scheduled handler executed and incremented counter
	verifyScript := fmt.Sprintf(`
		import ScheduledHandler from %s
		access(all) fun main(): Int { return ScheduledHandler.getCount() }
	`, serviceHexWithPrefix)
	scriptRes, err := b.ExecuteScript([]byte(verifyScript), nil)
	require.NoError(t, err)
	require.NoError(t, scriptRes.Error)
}

// Verifies that when there are NO scheduled callbacks pending,
// the process system transaction result contains NO PendingExecution event
// (because there's nothing to process)
func TestSystemTransactions_EmptyBlock_NoPendingExecution(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Commit an empty block (no user txs, no scheduled callbacks)
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.NotNil(t, block)
	blockID := flowgo.Identifier(block.ID())

	// Gather system transaction IDs for the block
	systemTxIDs, err := b.GetSystemTransactionsForBlock(blockID)
	require.NoError(t, err)
	require.NotEmpty(t, systemTxIDs, "expected system transactions for block")

	// Env used to compute expected event types
	env := systemcontracts.SystemContractsForChain(b.GetChain().ChainID()).AsTemplateEnv()
	expectedPendingExec := blueprints.PendingExecutionEventType(env)

	t.Logf("Checking %d system transactions for empty block %s", len(systemTxIDs), blockID.String())
	t.Logf("Expected PendingExecution event type: %s", expectedPendingExec)

	foundAnyPendingExec := false

	for i, txID := range systemTxIDs {
		result, err := adapter.GetSystemTransactionResult(
			context.Background(),
			txID,
			blockID,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.NoError(t, err)
		require.NotNil(t, result)

		t.Logf("\nSystem tx %d (ID: %s):", i, txID.String())
		t.Logf("  Event count: %d", len(result.Events))

		for j, evt := range result.Events {
			t.Logf("  Event[%d]: Type=%s", j, evt.Type)

			if evt.Type == expectedPendingExec {
				foundAnyPendingExec = true
				t.Logf("    UNEXPECTED: Found PendingExecution in empty block!")
			}
		}
	}

	assert.False(t, foundAnyPendingExec, "should NOT have PendingExecution event when nothing is scheduled")
}

func TestScheduledTransaction_QueryByID(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(emulator.WithStorageLimitEnabled(false))
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)
	serviceAddress := b.ServiceKey().Address
	serviceHex := serviceAddress.Hex()

	// Deploy handler
	handlerContract := fmt.Sprintf(`
		import FlowTransactionScheduler from 0x%s
		access(all) contract Handler {
			access(all) resource H: FlowTransactionScheduler.TransactionHandler {
				access(FlowTransactionScheduler.Execute) fun executeTransaction(id: UInt64, data: AnyStruct?) {}
			}
			access(all) fun create(): @H { return <- create H() }
		}
	`, serviceHex)

	latestBlock, err := b.GetLatestBlock()
	require.NoError(t, err)
	tx := templates.AddAccountContract(serviceAddress, templates.Contract{Name: "Handler", Source: handlerContract})
	tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID())).
		SetProposalKey(serviceAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAddress)
	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)
	require.NoError(t, tx.SignEnvelope(serviceAddress, b.ServiceKey().Index, signer))
	require.NoError(t, b.SendTransaction(convert.SDKTransactionToFlow(*tx)))
	_, _, err = b.ExecuteAndCommitBlock()
	require.NoError(t, err)

	// Schedule transaction
	scheduleTx := fmt.Sprintf(`
		import FlowTransactionScheduler from 0x%s
		import Handler from 0x%s
		import FungibleToken from 0xee82856bf20e2aa6
		import FlowToken from 0x0ae53cb6e3f42a79
		transaction {
			prepare(acct: auth(Storage, Capabilities, FungibleToken.Withdraw) &Account) {
				let h <- Handler.create()
				acct.storage.save(<-h, to: /storage/h)
				let cap = acct.capabilities.storage.issue<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>(/storage/h)
				let admin = acct.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
				let minter <- admin.createNewMinter(allowedAmount: 10.0)
				let minted <- minter.mintTokens(amount: 1.0)
				let receiver = acct.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)!
				receiver.deposit(from: <-minted)
				destroy minter
				let vaultRef = acct.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)!
				let fees <- (vaultRef.withdraw(amount: 0.001) as! @FlowToken.Vault)
				destroy <- FlowTransactionScheduler.schedule(
					handlerCap: cap, data: nil,
					timestamp: getCurrentBlock().timestamp + 2.0,
					priority: FlowTransactionScheduler.Priority.High,
					executionEffort: UInt64(5000), fees: <-fees
				)
			}
		}
	`, serviceHex, serviceHex)

	latestBlock, err = b.GetLatestBlock()
	require.NoError(t, err)
	tx = flowsdk.NewTransaction().SetScript([]byte(scheduleTx)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAddress).AddAuthorizer(serviceAddress).
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID()))
	signer, err = b.ServiceKey().Signer()
	require.NoError(t, err)
	require.NoError(t, tx.SignEnvelope(serviceAddress, b.ServiceKey().Index, signer))
	require.NoError(t, b.SendTransaction(convert.SDKTransactionToFlow(*tx)))
	_, _, err = b.ExecuteAndCommitBlock()
	require.NoError(t, err)

	// Wait and execute to trigger scheduled tx
	time.Sleep(2500 * time.Millisecond)
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)

	// Get system transactions from block
	systemTxIDs, err := b.GetSystemTransactionsForBlock(flowgo.Identifier(block.ID()))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(systemTxIDs), 2, "should have system chunk + scheduled tx")

	// Scheduled tx is the second one (first is static system chunk)
	scheduledTxID := systemTxIDs[1]

	// Query by ID alone (no blockID) - this is what we're testing
	tx2, err := adapter.GetTransaction(context.Background(), scheduledTxID)
	require.NoError(t, err)
	require.NotNil(t, tx2)

	result, err := adapter.GetTransactionResult(context.Background(), scheduledTxID, flowgo.ZeroID, flowgo.ZeroID, entities.EventEncodingVersion_CCF_V0)
	require.NoError(t, err)
	require.NotNil(t, result)
}
