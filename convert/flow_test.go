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

package convert

import (
	"testing"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
)

func TestSDKAccountToFlowAndBack(t *testing.T) {

	t.Parallel()

	contract := []byte("access(all) contract Test {}")
	var keys []*sdk.AccountKey

	keys = append(keys, &sdk.AccountKey{
		Index:          0,
		PublicKey:      test.AccountKeyGenerator().New().PublicKey,
		SigAlgo:        crypto.ECDSA_P256,
		HashAlgo:       crypto.SHA3_256,
		Weight:         1000,
		SequenceNumber: 2,
		Revoked:        true,
	}, &sdk.AccountKey{
		Index:          1,
		PublicKey:      test.AccountKeyGenerator().New().PublicKey,
		SigAlgo:        crypto.ECDSA_P256,
		HashAlgo:       crypto.SHA3_256,
		Weight:         500,
		SequenceNumber: 0,
		Revoked:        false,
	})

	acc := &sdk.Account{
		Address: sdk.HexToAddress("0x1"),
		Balance: 10,
		Code:    contract,
		Keys:    keys,
		Contracts: map[string][]byte{
			"Test": contract,
		},
	}

	flowAcc, err := SDKAccountToFlow(acc)

	assert.NoError(t, err)
	assert.Equal(t, len(flowAcc.Keys), len(acc.Keys))
	assert.Equal(t, flowAcc.Address.Hex(), acc.Address.Hex())
	assert.Equal(t, flowAcc.Contracts, acc.Contracts)
	assert.Equal(t, flowAcc.Balance, acc.Balance)

	for i, k := range acc.Keys {
		assert.Equal(t, k.Revoked, flowAcc.Keys[i].Revoked)
		assert.Equal(t, k.Weight, flowAcc.Keys[i].Weight)
		assert.Equal(t, k.SequenceNumber, flowAcc.Keys[i].SeqNumber)
		assert.Equal(t, k.HashAlgo, flowAcc.Keys[i].HashAlgo)
		assert.Equal(t, k.SigAlgo, flowAcc.Keys[i].SignAlgo)
	}

	sdkAccount, err := FlowAccountToSDK(*flowAcc)
	assert.NoError(t, err)
	assert.Equal(t, len(flowAcc.Keys), len(sdkAccount.Keys))
	assert.Equal(t, flowAcc.Address.Hex(), sdkAccount.Address.Hex())
	assert.Equal(t, flowAcc.Contracts, sdkAccount.Contracts)
	assert.Equal(t, flowAcc.Balance, sdkAccount.Balance)

	for i, k := range sdkAccount.Keys {
		assert.Equal(t, k.Revoked, flowAcc.Keys[i].Revoked)
		assert.Equal(t, k.Weight, flowAcc.Keys[i].Weight)
		assert.Equal(t, k.SequenceNumber, flowAcc.Keys[i].SeqNumber)
		assert.Equal(t, k.HashAlgo, flowAcc.Keys[i].HashAlgo)
		assert.Equal(t, k.SigAlgo, flowAcc.Keys[i].SignAlgo)
	}
}
