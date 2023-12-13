/*
 * Flow Emulator
 *
 * Copyright Dapper Labs, Inc.
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
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/emulator"
)

func TestRandomSourceHistoryLowestHeight(t *testing.T) {

	t.Parallel()

	chain := flowgo.Emulator.Chain()
	contracts := emulator.NewCommonContracts(flowgo.Emulator.Chain())

	b, err := emulator.New(
		emulator.Contracts(contracts),
		emulator.WithChainID(chain.ChainID()),
	)
	require.NoError(t, err)

	serviceAddress := chain.ServiceAddress().Hex()

	scriptCode := fmt.Sprintf(`
        import RandomBeaconHistory from 0x%s

        access(all)
        fun main(): UInt64 {
            return RandomBeaconHistory.getLowestHeight()
        }
	`, serviceAddress)

	scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
	require.NoError(t, err)
	require.NoError(t, scriptResult.Error)

	assert.Equal(t, cadence.UInt64(1), scriptResult.Value)
}

func TestRandomSourceHistoryAtBlockHeight(t *testing.T) {

	t.Parallel()

	chain := flowgo.Emulator.Chain()
	contracts := emulator.NewCommonContracts(flowgo.Emulator.Chain())

	b, err := emulator.New(
		emulator.Contracts(contracts),
		emulator.WithChainID(chain.ChainID()),
	)
	require.NoError(t, err)

	_, err = b.CommitBlock()
	require.NoError(t, err)

	_, err = b.CommitBlock()
	require.NoError(t, err)

	serviceAddress := chain.ServiceAddress().Hex()

	scriptCode := fmt.Sprintf(`
        import RandomBeaconHistory from 0x%s

        access(all)
        fun main(): Bool {
            let sorAt1 = RandomBeaconHistory.sourceOfRandomness(atBlockHeight: 1)
            let sorAt2 = RandomBeaconHistory.sourceOfRandomness(atBlockHeight: 2)

            assert(sorAt1.blockHeight == 1)
            assert(sorAt2.blockHeight == 2)
            assert(sorAt1.value != sorAt2.value)

            return true
        }
	`, serviceAddress)

	scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
	require.NoError(t, err)
	require.NoError(t, scriptResult.Error)

	assert.Equal(t, cadence.Bool(true), scriptResult.Value)
}
