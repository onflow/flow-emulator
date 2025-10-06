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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndLoadSnapshots(t *testing.T) {

	t.Parallel()

	blockchain1, adapter1 := setupTransactionTests(t)
	blockchain2, adapter2 := setupTransactionTests(t)

	err := blockchain1.CreateSnapshot("BlockchainCreated")
	require.NoError(t, err)

	err = blockchain2.CreateSnapshot("BlockchainCreated")
	require.NoError(t, err)

	_, _ = DeployAndGenerateAddTwoScript(t, adapter1)
	_, _ = DeployAndGenerateAddTwoScript(t, adapter2)

	err = blockchain1.CreateSnapshot("ContractDeployed")
	require.NoError(t, err)

	err = blockchain2.CreateSnapshot("ContractDeployed")
	require.NoError(t, err)

	block1, err := blockchain1.GetLatestBlock()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), block1.Height)

	block2, err := blockchain2.GetLatestBlock()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), block2.Height)

	err = blockchain1.LoadSnapshot("BlockchainCreated")
	require.NoError(t, err)

	err = blockchain2.LoadSnapshot("BlockchainCreated")
	require.NoError(t, err)

	block1, err = blockchain1.GetLatestBlock()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), block1.Height)

	block2, err = blockchain2.GetLatestBlock()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), block2.Height)
}
