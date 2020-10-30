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
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
)

type StorableTransactionResult struct {
	ErrorCode    int
	ErrorMessage string
	Logs         []string
	Events       []flowgo.Event
}

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TransactionID flow.Identifier
	Error         error
	Logs          []string
	Events        []flow.Event
}

// Succeeded returns true if the transaction executed without errors.
func (r TransactionResult) Succeeded() bool {
	return r.Error == nil
}

// Reverted returns true if the transaction executed with errors.
func (r TransactionResult) Reverted() bool {
	return !r.Succeeded()
}

// TODO - this class should be part of SDK for consistency

// A ScriptResult is the result of executing a script.
type ScriptResult struct {
	ScriptID flow.Identifier
	Value    cadence.Value
	Error    error
	Logs     []string
	Events   []flow.Event
}

// Succeeded returns true if the script executed without errors.
func (r ScriptResult) Succeeded() bool {
	return r.Error == nil
}

// Reverted returns true if the script executed with errors.
func (r ScriptResult) Reverted() bool {
	return !r.Succeeded()
}
