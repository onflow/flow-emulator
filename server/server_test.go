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

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/convert"
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

	serviceAccount := server.Emulator().ServiceKey().Address.Hex()

	require.Equal(t, "f4527793ee68aede", serviceAccount)
}

func TestScheduledCallback_IncrementsCounter(t *testing.T) {
	logger := zerolog.Nop()
	conf := &Config{
		ScheduledTransactionsEnabled: true,
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

	              access(all) view fun getViews(): [Type] {
	                  return []
	              }

	              access(all) fun resolveView(_ view: Type): AnyStruct? {
	                  return nil
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
	{
		latestBlock := mustGetLatestBlock(t, server)
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
		require.GreaterOrEqual(t, len(results), 1)
	}

	// 2) Create handler and schedule a callback to increment count
	createAndSchedule := fmt.Sprintf(
		`
	      import FlowTransactionScheduler from 0x%s
	      import ScheduledHandler from 0x%s

	      transaction {
	          prepare(acct: auth(SaveValue, LinkValue) &Account) {
	              // create the handler resource and publish capability
	              let handler <- ScheduledHandler.createHandler()
	              acct.save(<-handler, to: /storage/counterHandler)
	              acct.link<&{FlowTransactionScheduler.TransactionHandler}>(
	                  /public/counterHandler,
	                  target: /storage/counterHandler
	              )
	              let cap = acct.getCapability<&{FlowTransactionScheduler.TransactionHandler}>(/public/counterHandler)

	              // schedule with sufficient effort and explicit types
	              FlowTransactionScheduler.schedule(cap: cap, priority: UInt8(1), executionEffort: UInt64(100000))
	          }
	      }
	    `,
		serviceHex,
		serviceHex,
	)

	{
		latestBlock := mustGetLatestBlock(t, server)
		tx := flowsdk.NewTransaction().
			SetScript([]byte(createAndSchedule)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAddress, server.Emulator().ServiceKey().Index, server.Emulator().ServiceKey().SequenceNumber).
			SetPayer(serviceAddress)
		tx.SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID()))

		signer, err := server.Emulator().ServiceKey().Signer()
		require.NoError(t, err)
		require.NoError(t, tx.SignEnvelope(serviceAddress, server.Emulator().ServiceKey().Index, signer))

		require.NoError(t, server.Emulator().SendTransaction(convert.SDKTransactionToFlow(*tx)))
		_, _, err = server.Emulator().ExecuteAndCommitBlock()
		require.NoError(t, err)
		// Trigger next block so scheduler can process in a subsequent block
		_, _, err = server.Emulator().ExecuteAndCommitBlock()
		require.NoError(t, err)
	}

	// 3) Verify count incremented by scheduled execution
	verifyScript := fmt.Sprintf(
		`
	      import ScheduledHandler from %s
	      access(all) fun main(): Int { return ScheduledHandler.getCount() }
	    `,
		serviceHexWithPrefix,
	)
	res, err := server.Emulator().ExecuteScript([]byte(verifyScript), nil)
	require.NoError(t, err)
	require.NoError(t, res.Error)
	require.Equal(t, cadence.NewInt(1), res.Value)
}

// mustGetLatestBlock is a tiny helper for brevity
func mustGetLatestBlock(t *testing.T, s *EmulatorServer) *flowgo.Block {
	latestBlock, err := s.Emulator().GetLatestBlock()
	require.NoError(t, err)
	return latestBlock
}
