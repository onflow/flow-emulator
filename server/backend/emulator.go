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

package backend

import (
	"github.com/onflow/cadence/runtime/interpreter"
	emulator "github.com/onflow/flow-emulator"
	sdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/types"
)

// Emulator defines the method set of an emulated blockchain.
type Emulator interface {
	SetDebugger(*interpreter.Debugger)
	EndDebugging()

	AddTransaction(tx sdk.Transaction) error
	ExecuteNextTransaction() (*types.TransactionResult, error)
	ExecuteBlock() ([]*types.TransactionResult, error)
	CommitBlock() (*flowgo.Block, error)
	ExecuteAndCommitBlock() (*flowgo.Block, []*types.TransactionResult, error)
	GetLatestBlock() (*flowgo.Block, error)
	GetBlockByID(id sdk.Identifier) (*flowgo.Block, error)
	GetBlockByHeight(height uint64) (*flowgo.Block, error)
	GetCollection(colID sdk.Identifier) (*sdk.Collection, error)
	GetTransaction(txID sdk.Identifier) (*sdk.Transaction, error)
	GetTransactionResult(txID sdk.Identifier) (*sdk.TransactionResult, error)
	// Deprecated: Needed for the debugger right now, do NOT use for other purposes.
	// TODO: refactor
	GetAccountUnsafe(address sdk.Address) (*sdk.Account, error)
	GetAccount(address sdk.Address) (*sdk.Account, error)
	GetAccountByIndex(uint) (*sdk.Account, error)

	GetAccountAtBlock(address sdk.Address, blockHeight uint64) (*sdk.Account, error)
	GetEventsByHeight(blockHeight uint64, eventType string) ([]sdk.Event, error)
	ExecuteScript(script []byte, arguments [][]byte) (*types.ScriptResult, error)
	ExecuteScriptAtBlock(script []byte, arguments [][]byte, blockHeight uint64) (*types.ScriptResult, error)
	GetAccountStorage(address sdk.Address) (*emulator.AccountStorage, error)

	Snapshots() ([]string, error)
	CreateSnapshot(name string) error
	LoadSnapshot(name string) error

	RollbackToBlockHeight(height uint64) error
}
