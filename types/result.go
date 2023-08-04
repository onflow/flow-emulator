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

package types

import (
	"fmt"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/crypto/hash"
	flowgo "github.com/onflow/flow-go/model/flow"
)

type StorableTransactionResult struct {
	ErrorCode    int
	ErrorMessage string
	Logs         []string
	Events       []flowgo.Event
	BlockID      flowgo.Identifier
	BlockHeight  uint64
}

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TransactionID   flowsdk.Identifier
	ComputationUsed uint64
	MemoryEstimate  uint64
	Error           error
	Logs            []string
	Events          []flowsdk.Event
	Debug           *TransactionResultDebug
}

// Succeeded returns true if the transaction executed without errors.
func (r TransactionResult) Succeeded() bool {
	return r.Error == nil
}

// Reverted returns true if the transaction executed with errors.
func (r TransactionResult) Reverted() bool {
	return !r.Succeeded()
}

// TransactionResultDebug provides details about unsuccessful transaction execution
type TransactionResultDebug struct {
	Message string
	Meta    map[string]any
}

// NewTransactionInvalidHashAlgo creates debug details for transactions with invalid hashing algorithm
func NewTransactionInvalidHashAlgo(
	key flowgo.AccountPublicKey,
	address flowgo.Address,
	invalidAlgo hash.HashingAlgorithm,
) *TransactionResultDebug {
	return &TransactionResultDebug{
		Message: fmt.Sprintf(
			"invalid hashing algorithm signature: public key %d on account %s does not have a valid signature: key requires %s hashing algorithm, but %s was used",
			key.Index, address, key.HashAlgo, invalidAlgo,
		),
		Meta: nil,
	}
}

// NewTransactionInvalidSignature creates more debug details for transactions with invalid signature
func NewTransactionInvalidSignature(
	tx *flowgo.TransactionBody,
) *TransactionResultDebug {
	return &TransactionResultDebug{
		Message: "",
		Meta: map[string]any{
			"payer":            tx.Payer.String(),
			"proposer":         tx.ProposalKey.Address.String(),
			"proposerKeyIndex": fmt.Sprintf("%d", tx.ProposalKey.KeyIndex),
			"authorizers":      fmt.Sprintf("%v", tx.Authorizers),
			"gasLimit":         fmt.Sprintf("%d", tx.GasLimit),
		},
	}
}

// TODO - this class should be part of SDK for consistency

// A ScriptResult is the result of executing a script.
type ScriptResult struct {
	ScriptID        flowsdk.Identifier
	Value           cadence.Value
	Error           error
	Logs            []string
	Events          []flowsdk.Event
	ComputationUsed uint64
	MemoryEstimate  uint64
}

// Succeeded returns true if the script executed without errors.
func (r ScriptResult) Succeeded() bool {
	return r.Error == nil
}

// Reverted returns true if the script executed with errors.
func (r ScriptResult) Reverted() bool {
	return !r.Succeeded()
}
