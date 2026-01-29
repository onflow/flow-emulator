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

package unittest

import (
	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/crypto"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-emulator/convert"

	"github.com/onflow/flow-emulator/types"
)

func TransactionFixture() flowgo.TransactionBody {
	return *convert.SDKTransactionToFlow(*test.TransactionGenerator().New())
}

func SignatureFixtureForTransactions() crypto.Signature {
	sigLen := crypto.SignatureLenECDSAP256
	sig := make([]byte, sigLen)

	// make sure the ECDSA signature passes the format check
	sig[sigLen/2] = 0
	sig[0] = 0
	sig[sigLen/2-1] |= 1
	sig[sigLen-1] |= 1
	return sig
} 

func StorableTransactionResultFixture(eventEncodingVersion entities.EventEncodingVersion) types.StorableTransactionResult {
	events := test.EventGenerator(eventEncodingVersion)

	eventA, _ := convert.SDKEventToFlow(events.New())
	eventB, _ := convert.SDKEventToFlow(events.New())

	return types.StorableTransactionResult{
		ErrorCode:    42,
		ErrorMessage: "foo",
		Logs:         []string{"a", "b", "c"},
		Events: []flowgo.Event{
			eventA,
			eventB,
		},
	}
}

func FullCollectionFixture(n int) flowgo.Collection {
	transactions := make([]*flowgo.TransactionBody, n)
	for i := 0; i < n; i++ {
		tx := TransactionFixture()
		transactions[i] = &tx
	}

	return flowgo.Collection{
		Transactions: transactions,
	}
}
