package emulator_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
)

const counterScript = `

  pub contract Counting {

      pub event CountIncremented(count: Int)

      pub resource Counter {
          pub var count: Int

          init() {
              self.count = 0
          }

          pub fun add(_ count: Int) {
              self.count = self.count + count
              emit CountIncremented(count: self.count)
          }
      }

      pub fun createCounter(): @Counter {
          return <-create Counter()
      }
  }
`

// generateAddTwoToCounterScript generates a script that increments a counter.
// If no counter exists, it is created.
func generateAddTwoToCounterScript(counterAddress flow.Address) string {
	return fmt.Sprintf(
		`
            import 0x%s

            transaction {
                prepare(signer: AuthAccount) {
                    var counter = signer.borrow<&Counting.Counter>(from: /storage/counter)
                    if counter == nil {
                        signer.save(<-Counting.createCounter(), to: /storage/counter)
                        signer.link<&Counting.Counter>(/public/counter, target: /storage/counter)
                        counter = signer.borrow<&Counting.Counter>(from: /storage/counter)
                    }
                    counter?.add(2)
                }
            }
        `,
		counterAddress,
	)
}

func deployAndGenerateAddTwoScript(t *testing.T, b *emulator.Blockchain) (string, flow.Address) {
	counterAddress, err := b.CreateAccount(nil, []byte(counterScript))
	require.NoError(t, err)

	return generateAddTwoToCounterScript(counterAddress), counterAddress
}

func generateGetCounterCountScript(counterAddress flow.Address, accountAddress flow.Address) string {
	return fmt.Sprintf(
		`
            import 0x%s

            pub fun main(): Int {
                return getAccount(0x%s).getCapability(/public/counter)!.borrow<&Counting.Counter>()?.count ?? 0
            }
        `,
		counterAddress,
		accountAddress,
	)
}

func assertTransactionSucceeded(t *testing.T, result *emulator.TransactionResult) {
	if !assert.True(t, result.Succeeded()) {
		t.Error(result.Error)
	}
}
