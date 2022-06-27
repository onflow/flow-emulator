package emulator_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
	convert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/onflow/flow-emulator/storage/badger"
)

func TestInitialization(t *testing.T) {

	t.Parallel()

	t.Run("should inject initial state when initialized with empty store", func(t *testing.T) {

		t.Parallel()

		dir, err := ioutil.TempDir("", "badger-test")
		require.Nil(t, err)
		defer os.RemoveAll(dir)
		store, err := badger.New(badger.WithPath(dir))
		require.Nil(t, err)
		defer store.Close()

		b, _ := emulator.NewBlockchain(emulator.WithStore(store))

		serviceAcct, err := b.GetAccount(flow.ServiceAddress(flow.Emulator))
		require.NoError(t, err)

		assert.NotNil(t, serviceAcct)

		latestBlock, err := b.GetLatestBlock()
		require.NoError(t, err)

		assert.EqualValues(t, 0, latestBlock.Header.Height)
		assert.Equal(t,
			flowgo.Genesis(flowgo.Emulator).ID(),
			latestBlock.ID(),
		)
	})

	t.Run("should restore state when initialized with non-empty store", func(t *testing.T) {

		t.Parallel()

		dir, err := ioutil.TempDir("", "badger-test")
		require.Nil(t, err)
		defer os.RemoveAll(dir)
		store, err := badger.New(badger.WithPath(dir))
		require.Nil(t, err)
		defer store.Close()

		b, _ := emulator.NewBlockchain(emulator.WithStore(store), emulator.WithStorageLimitEnabled(false))

		contracts := []templates.Contract{
			{
				Name:   "Counting",
				Source: counterScript,
			},
		}

		counterAddress, err := b.CreateAccount(nil, contracts)
		require.NoError(t, err)

		// Submit a transaction adds some ledger state and event state
		script := fmt.Sprintf(
			`
                import 0x%s

                transaction {

                  prepare(acct: AuthAccount) {

                    let counter <- Counting.createCounter()
                    counter.add(1)

                    acct.save(<-counter, to: /storage/counter)

                    acct.link<&Counting.Counter>(/public/counter, target: /storage/counter)
                  }
                }
            `,
			counterAddress,
		)

		tx := flow.NewTransaction().
			SetScript([]byte(script)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		require.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		assert.NoError(t, err)
		require.NotNil(t, block)

		minedTx, err := b.GetTransaction(tx.ID())
		require.NoError(t, err)

		minedEvents, err := b.GetEventsByHeight(block.Header.Height, "")
		require.NoError(t, err)

		// Create a new blockchain with the same store
		b, _ = emulator.NewBlockchain(emulator.WithStore(store))

		t.Run("should be able to read blocks", func(t *testing.T) {
			latestBlock, err := b.GetLatestBlock()
			require.NoError(t, err)

			assert.Equal(t, block.ID(), latestBlock.ID())

			blockByHeight, err := b.GetBlockByHeight(block.Header.Height)
			require.NoError(t, err)

			assert.Equal(t, block.ID(), blockByHeight.ID())

			blockByHash, err := b.GetBlockByID(convert.FlowIdentifierToSDK(block.ID()))
			require.NoError(t, err)

			assert.Equal(t, block.ID(), blockByHash.ID())
		})

		t.Run("should be able to read transactions", func(t *testing.T) {
			txByHash, err := b.GetTransaction(tx.ID())
			require.NoError(t, err)

			assert.Equal(t, minedTx, txByHash)
		})

		t.Run("should be able to read events", func(t *testing.T) {
			gotEvents, err := b.GetEventsByHeight(block.Header.Height, "")
			require.NoError(t, err)

			assert.Equal(t, minedEvents, gotEvents)
		})

		t.Run("should be able to read ledger state", func(t *testing.T) {
			readScript := fmt.Sprintf(
				`
                  import 0x%s

                  pub fun main(): Int {
                      return getAccount(0x%s).getCapability(/public/counter)!.borrow<&Counting.Counter>()?.count ?? 0
                  }
                `,
				counterAddress,
				b.ServiceKey().Address,
			)

			result, err := b.ExecuteScript([]byte(readScript), nil)
			require.NoError(t, err)

			assert.Equal(t, cadence.NewInt(1), result.Value)
		})
	})
}
