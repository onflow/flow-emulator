package emulator_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/storage/badger"
	"github.com/dapperlabs/flow-emulator/types"
)

func TestInitialization(t *testing.T) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	require.Nil(t, err)
	defer store.Close()

	t.Run("should inject initial state when initialized with empty store", func(t *testing.T) {
		b, _ := emulator.NewBlockchain(emulator.WithStore(store))

		rootAcct, err := b.GetAccount(flow.RootAddress)
		require.NoError(t, err)

		assert.NotNil(t, rootAcct)

		latestBlock, err := b.GetLatestBlock()
		require.NoError(t, err)

		assert.EqualValues(t, 0, latestBlock.Height)
		assert.Equal(t, types.GenesisBlock().ID(), latestBlock.ID())
	})

	t.Run("should restore state when initialized with non-empty store", func(t *testing.T) {
		b, _ := emulator.NewBlockchain(emulator.WithStore(store))

		counterAddress, err := b.CreateAccount(nil, []byte(counterScript))
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
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
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

		minedEvents, err := b.GetEventsByHeight(block.Height, "")

		// Create a new blockchain with the same store
		b, _ = emulator.NewBlockchain(emulator.WithStore(store))

		t.Run("should be able to read blocks", func(t *testing.T) {
			latestBlock, err := b.GetLatestBlock()
			require.NoError(t, err)

			assert.Equal(t, block.ID(), latestBlock.ID())

			blockByHeight, err := b.GetBlockByHeight(block.Height)
			require.NoError(t, err)

			assert.Equal(t, block.ID(), blockByHeight.ID())

			blockByHash, err := b.GetBlockByID(block.ID())
			require.NoError(t, err)

			assert.Equal(t, block.ID(), blockByHash.ID())
		})

		t.Run("should be able to read transactions", func(t *testing.T) {
			txByHash, err := b.GetTransaction(tx.ID())
			require.NoError(t, err)

			assert.Equal(t, minedTx, txByHash)
		})

		t.Run("should be able to read events", func(t *testing.T) {
			gotEvents, err := b.GetEventsByHeight(block.Height, "")
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
				b.RootKey().Address,
			)

			result, err := b.ExecuteScript([]byte(readScript))
			require.NoError(t, err)

			assert.Equal(t, cadence.NewInt(1), result.Value)
		})
	})
}
