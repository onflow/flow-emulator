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

package server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-emulator/convert"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestNoPersistence(t *testing.T) {
	logger := zerolog.Nop()

	dbPath := "test_no_persistence"

	_ = os.RemoveAll(dbPath)
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()

	conf := &Config{DBPath: dbPath}
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	require.NotNil(t, server)
	_, err := os.Stat(conf.DBPath)
	require.True(t, os.IsNotExist(err), "DB should not exist")
}

func TestPersistenceWithPersistFlag(t *testing.T) {
	logger := zerolog.Nop()

	dbPath := "test_persistence"

	_ = os.RemoveAll(dbPath)
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()

	conf := &Config{Persist: true, DBPath: dbPath}
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	require.NotNil(t, server)
	_, err := os.Stat(conf.DBPath)
	require.NoError(t, err, "DB should exist")
}

func TestPersistenceWithSnapshotFlag(t *testing.T) {
	logger := zerolog.Nop()

	dbPath := "test_persistence_with_snapshot"

	_ = os.RemoveAll(dbPath)
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()

	conf := &Config{Snapshot: true, DBPath: dbPath}
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	require.NotNil(t, server)
	_, err := os.Stat(conf.DBPath)
	require.True(t, os.IsNotExist(err), "DB should not exist")
}

func TestExecuteScript(t *testing.T) {

	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, &Config{})
	go server.Start()
	defer server.Stop()

	require.NotNil(t, server)

	const code = `
      access(all) fun main(): String {
	      return "Hello"
      }
    `
	adapter := server.AccessAdapter()
	result, err := adapter.ExecuteScriptAtLatestBlock(context.Background(), []byte(code), nil)
	require.NoError(t, err)

	require.JSONEq(t, `{"type":"String","value":"Hello"}`, string(result))

}

func TestExecuteScriptImportingContracts(t *testing.T) {
	conf := &Config{
		WithContracts: true,
	}

	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()
	require.NotNil(t, server)
	serviceAccount := server.Emulator().ServiceKey().Address.Hex()

	code := fmt.Sprintf(
		`
	      import ExampleNFT, NFTStorefront from 0x%s

          access(all) fun main() {
		      let collection <- ExampleNFT.createEmptyCollection()
		      destroy collection

		      NFTStorefront
		  }
        `,
		serviceAccount,
	)

	_, err := server.Emulator().ExecuteScript([]byte(code), nil)
	require.NoError(t, err)

}

func TestCustomChainID(t *testing.T) {

	conf := &Config{
		WithContracts: true,
		ChainID:       "flow-sandboxnet",
	}
	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	serviceAccount := server.Emulator().ServiceKey().Address.Hex()

	require.Equal(t, "f4527793ee68aede", serviceAccount)
}

func TestScheduledCallback_IncrementsCounter(t *testing.T) {
	logger := zerolog.Nop()
	conf := &Config{
		WithContracts:                true,
		ScheduledTransactionsEnabled: true,
		ChainID:                      "flow-emulator",
	}

	server := NewEmulatorServer(&logger, conf)
	require.NotNil(t, server)
	defer server.Stop()

	serviceAddress := server.Emulator().ServiceKey().Address
	serviceHex := serviceAddress.Hex()
	serviceHexWithPrefix := serviceAddress.HexWithPrefix()

	// 1) Deploy a handler contract implementing the scheduler's handler interface.
	// The handler increments a contract-level counter from its returned Transaction.
	contractCode := fmt.Sprintf(
		`
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

			init() {
				self.count = 0
			}
		}
	`,
		serviceHex,
	)

	// Deploy the handler contract to the service account
	latestBlock, _ := server.Emulator().GetLatestBlock()
	tx := templates.AddAccountContract(serviceAddress, templates.Contract{
		Name:   "ScheduledHandler",
		Source: contractCode,
	})
	tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID())).
		SetProposalKey(serviceAddress, server.Emulator().ServiceKey().Index, server.Emulator().ServiceKey().SequenceNumber).
		SetPayer(serviceAddress)

	signer, err := server.Emulator().ServiceKey().Signer()
	require.NoError(t, err)
	require.NoError(t, tx.SignEnvelope(serviceAddress, server.Emulator().ServiceKey().Index, signer))

	require.NoError(t, server.Emulator().SendTransaction(convert.SDKTransactionToFlow(*tx)))
	_, results, err := server.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)

	// 2) Create handler and schedule a callback to increment count
	createAndSchedule := fmt.Sprintf(
		`
		import FlowTransactionScheduler from 0x%s
		import ScheduledHandler from 0x%s
		import FungibleToken from 0xee82856bf20e2aa6
		import FlowToken from 0x0ae53cb6e3f42a79

		transaction {
			prepare(acct: auth(Storage, Capabilities, FungibleToken.Withdraw) &Account) {
				// create the handler resource and publish capability (Cadence 1.0 APIs)
				let handler <- ScheduledHandler.createHandler()
				acct.storage.save(<-handler, to: /storage/counterHandler)
				let issued: Capability<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}> =
					acct.capabilities.storage.issue<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>(/storage/counterHandler)

				// seed FLOW for fees (mint to service account)
				let adminRef = acct.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
					?? panic("missing FlowToken admin")
				let minter <- adminRef.createNewMinter(allowedAmount: 10.0)
				let minted <- minter.mintTokens(amount: 1.0)
				let receiver = acct.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
					?? panic("missing FlowToken receiver")
				receiver.deposit(from: <-minted)
				destroy minter

				// compute fees via estimate
                let estimate = FlowTransactionScheduler.estimate(
					data: nil,
                    timestamp: getCurrentBlock().timestamp + 1.0,
					priority: FlowTransactionScheduler.Priority.High,
					executionEffort: UInt64(5000)
				)
				let feeAmount: UFix64 = estimate.flowFee ?? 0.001
				// withdraw fees to pay for scheduling
				let vaultRef = acct.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("missing FlowToken vault")
				let fees <- (vaultRef.withdraw(amount: feeAmount) as! @FlowToken.Vault)

				// schedule with sufficient effort and priority; timestamp must be in the future
                destroy <- FlowTransactionScheduler.schedule(
					handlerCap: issued,
					data: nil,
                    timestamp: getCurrentBlock().timestamp + 1.0,
					priority: FlowTransactionScheduler.Priority.High,
					executionEffort: UInt64(5000),
					fees: <-fees
				)
			}
		}
	`,
		serviceHex,
		serviceHex,
	)

	latestBlock, _ = server.Emulator().GetLatestBlock()
	tx = flowsdk.NewTransaction().
		SetScript([]byte(createAndSchedule)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAddress, server.Emulator().ServiceKey().Index, server.Emulator().ServiceKey().SequenceNumber).
		SetPayer(serviceAddress).
		AddAuthorizer(serviceAddress)
	tx.SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID()))

	signer, err = server.Emulator().ServiceKey().Signer()
	require.NoError(t, err)
	require.NoError(t, tx.SignEnvelope(serviceAddress, server.Emulator().ServiceKey().Index, signer))

	require.NoError(t, server.Emulator().SendTransaction(convert.SDKTransactionToFlow(*tx)))
	_, results, err = server.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)

	// ensure scheduled timestamp is in the past relative to next commit
	time.Sleep(4000 * time.Millisecond)

	for i, r := range results {
		r.Succeeded()
		txBody, _ := server.emulator.GetTransaction(flowgo.Identifier(flowgo.MakeIDFromFingerPrint(r.TransactionID.Bytes())))
		t.Logf("schedule block tx %d txBody: %v", i, txBody)
		if r.Error != nil {
			t.Fatalf("schedule block tx %d failed: %v", i, r.Error)
		}
	}

	// Sleep-commit-sleep-commit to ensure pending block timestamp advances past the scheduled time
	time.Sleep(1500 * time.Millisecond)
	_, _, err = server.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)
	time.Sleep(1500 * time.Millisecond)
	_, _, err = server.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)

	// 3) Verify count incremented by scheduled execution
	verifyScript := fmt.Sprintf(
		`
		import ScheduledHandler from %s
		access(all) fun main(): Int { return ScheduledHandler.getCount() }
	`,
		serviceHexWithPrefix,
	)

	// Verify the counter incremented after scheduler processing block
	res, err := server.Emulator().ExecuteScript([]byte(verifyScript), nil)
	require.NoError(t, err)
	require.NoError(t, res.Error)
	require.Equal(t, cadence.NewInt(1), res.Value)
}
