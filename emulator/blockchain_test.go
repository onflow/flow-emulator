/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/types"
)

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

// generateAddTwoToCounterScript generates a script that increments a counter.
// If no counter exists, it is created.
func GenerateAddTwoToCounterScript(counterAddress flowsdk.Address) string {
	return fmt.Sprintf(
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
}

func DeployAndGenerateAddTwoScript(t *testing.T, adapter *adapters.SDKAdapter) (string, flowsdk.Address) {

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

	return GenerateAddTwoToCounterScript(counterAddress), counterAddress
}

func GenerateGetCounterCountScript(counterAddress flowsdk.Address, accountAddress flowsdk.Address) string {
	return fmt.Sprintf(
		`
            import 0x%s

            access(all) fun main(): Int {
                return getAccount(0x%s).capabilities.borrow<&Counting.Counter>(/public/counter)?.count ?? 0
            }
        `,
		counterAddress,
		accountAddress,
	)
}

func AssertTransactionSucceeded(t *testing.T, result *types.TransactionResult) {
	if !assert.True(t, result.Succeeded()) {
		t.Error(result.Error)
	}
}

func LastCreatedAccount(b *emulator.Blockchain, result *types.TransactionResult) (*flowsdk.Account, error) {
	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	address, err := LastCreatedAccountAddress(result)
	if err != nil {
		return nil, err
	}

	return adapter.GetAccount(context.Background(), address)
}

func LastCreatedAccountAddress(result *types.TransactionResult) (flowsdk.Address, error) {
	for _, event := range result.Events {
		if event.Type == flowsdk.EventAccountCreated {
			return flowsdk.Address(event.Value.Fields[0].(cadence.Address)), nil
		}
	}

	return flowsdk.Address{}, fmt.Errorf("no account created in this result")
}
