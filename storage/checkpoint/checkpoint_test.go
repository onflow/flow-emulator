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

package checkpoint_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/storage/checkpoint"
)

func Test_Checkpoint_Storage(t *testing.T) {

	dir := t.TempDir()

	chainID := flowsdk.Emulator
	log := zerolog.New(zerolog.NewTestWriter(t))

	stateHash := createCheckpoint(t, log, dir, chainID)

	store, err := checkpoint.New(log, dir, hex.EncodeToString(stateHash[:]), flowgo.ChainID(chainID))
	require.NoError(t, err)
	b, _ := emulator.New(
		emulator.WithStore(store),
		emulator.WithChainID(flowgo.ChainID(chainID)),
		emulator.WithStorageLimitEnabled(false),
		emulator.WithServicePrivateKey(
			unittest.ServiceAccountPrivateKey.PrivateKey,
			unittest.ServiceAccountPrivateKey.SignAlgo,
			unittest.ServiceAccountPrivateKey.HashAlgo,
		),
	)

	// check that all contracts are there
	sc := systemcontracts.SystemContractsForChain(flowgo.ChainID(chainID))
	for _, contract := range sc.All() {
		scriptCode := fmt.Sprintf(`
			access(all) fun main() {
				getAccount(0x%s).contracts.get(name: "%s") ?? panic("contract is not deployed")
	    	}`, contract.Address, contract.Name)

		scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
		require.NoError(t, err)
		require.NoError(t, scriptResult.Error)
	}

	// run some transaction

	adapter := adapters.NewSDKAdapter(&log, b)
	contracts := []templates.Contract{
		{
			Name:   "Counting",
			Source: counterScript,
		},
	}

	counterAddress, err := adapter.CreateAccount(
		context.Background(),
		nil,
		contracts,
	)
	require.NoError(t, err)

	addTwoScript := fmt.Sprintf(
		`
            import 0x%s

            transaction {
                prepare(signer: auth(Storage, Capabilities) &Account) {
                    var counter = signer.storage.borrow<&Counting.Counter>(from: /storage/counter)
                    if counter == nil {
                        signer.storage.save(<-Counting.createCounter(), to: /storage/counter)
                        counter = signer.storage.borrow<&Counting.Counter>(from: /storage/counter)

                        // Also publish this for others to borrow.
                        let cap = signer.capabilities.storage.issue<&Counting.Counter>(/storage/counter)
                        signer.capabilities.publish(cap, at: /public/counter)
                    }
                    counter?.add(2)
                }
            }
        `,
		counterAddress,
	)

	tx1 := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Submit tx1
	err = adapter.SendTransaction(context.Background(), *tx1)
	require.NoError(t, err)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	require.NoError(t, err)
	require.NoError(t, result.Error)

	_, err = b.CommitBlock()
	require.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err := adapter.GetTransactionResult(context.Background(), tx1.ID())
	require.NoError(t, err)
	require.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)
}

func createCheckpoint(t *testing.T, log zerolog.Logger, dir string, chainID flowsdk.ChainID) flowgo.StateCommitment {
	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	diskWal, err := wal.NewDiskWAL(
		log,
		nil,
		metrics.NewNoopCollector(),
		dir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	require.NoError(t, err)

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		metrics.NewNoopCollector(),
		log,
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)

	compactor, err := complete.NewCompactor(
		led,
		diskWal,
		unittest.Logger(),
		complete.DefaultCacheSize,
		checkpointDistance,
		checkpointsToKeep,
		atomic.NewBool(false),
		metrics.NewNoopCollector(),
	)
	require.NoError(t, err)

	<-compactor.Ready()

	// WAL segments are 32kB, so here we generate 2 keys 16kB each, times `size`
	// so we should get at least `size` segments

	opts := []fvm.Option{
		// default chain is Testnet
		fvm.WithChain(flowgo.ChainID(chainID).Chain()),
		fvm.WithEntropyProvider(testutil.EntropyProviderFixture(nil)),
	}

	ctx := fvm.NewContext(opts...)

	vm := fvm.NewVirtualMachine()

	snapshotTree := snapshot.NewSnapshotTree(nil)

	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithSetupEVMEnabled(true),
		fvm.WithSetupVMBridgeEnabled(true),
	}

	result, _, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		snapshotTree)
	require.NoError(t, err)

	keys, values := executionState.RegisterEntriesToKeysValues(result.UpdatedRegisters())
	update, err := ledger.NewUpdate(led.InitialState(), keys, values)
	require.NoError(t, err)

	newState, _, err := led.Set(update)
	require.NoError(t, err)

	<-led.Done()
	<-compactor.Done()

	return flowgo.StateCommitment(newState)
}

const counterScript = `

  access(all) contract Counting {

      access(all) event CountIncremented(count: Int)

      access(all) resource Counter {
          access(all) var count: Int

          init() {
              self.count = 0
          }

          access(all) fun add(_ count: Int) {
              self.count = self.count + count
              emit CountIncremented(count: self.count)
          }
      }

      access(all) fun createCounter(): @Counter {
          return <-create Counter()
      }
  }
`
