/*
 * Flow Emulator
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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

	"github.com/onflow/flow-go/crypto/hash"

	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

type FlowError struct {
	FlowError fvmerrors.Error
}

func (f *FlowError) Error() string {
	return f.FlowError.Error()
}

func (f *FlowError) Unwrap() error {
	return f.FlowError
}

func NewTransactionSignatureError(err *FlowError, tx *flow.TransactionBody) *TransactionSignatureError {
	return &TransactionSignatureError{
		err:         err,
		transaction: tx,
	}
}

type TransactionSignatureError struct {
	err         *FlowError
	transaction *flow.TransactionBody
}

func (t *TransactionSignatureError) Error() string {
	return t.err.Error()
}

func (t *TransactionSignatureError) Unwrap() error {
	return t.err
}

func (t *TransactionSignatureError) Transaction() *flow.TransactionBody {
	return t.transaction
}

func NewSignatureHashingError(
	index int,
	address flow.Address,
	usedAlgo hash.HashingAlgorithm,
	requiredAlgo hash.HashingAlgorithm,
) *TransactionHashingError {
	return &TransactionHashingError{
		index,
		address,
		usedAlgo,
		requiredAlgo,
	}
}

type TransactionHashingError struct {
	index        int
	address      flow.Address
	usedAlgo     hash.HashingAlgorithm
	requiredAlgo hash.HashingAlgorithm
}

func (t *TransactionHashingError) Error() string {
	return fmt.Sprintf(
		"invalid hashing algorithm signature: public key %d on account %s does not have a valid signature: key requires %s hashing algorithm, but %s was used",
		t.index,
		t.address.Hex(),
		t.requiredAlgo,
		t.usedAlgo,
	)
}
