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

package emulator

import (
	"github.com/onflow/flow-go/access"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/types"
)

// Emulator defines the method set of an emulated blockchain.
type Emulator interface {
	EnableAutoMine()
	DisableAutoMine()

	Ping() error
	GetNetworkParameters() access.NetworkParameters

	GetLatestBlock() (*flowgo.Block, flowgo.BlockStatus, error)
	GetBlockByHeight(height uint64) (*flowgo.Block, flowgo.BlockStatus, error)
	GetBlockByID(id flowgo.Identifier) (*flowgo.Block, flowgo.BlockStatus, error)

	GetCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error)

	GetTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error)
	GetTransactionResult(txID flowgo.Identifier) (*access.TransactionResult, error)
	GetTransactionResultByIndex(blockID flowgo.Identifier, index uint32) (*access.TransactionResult, error)
	GetTransactionsByBlockID(blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error)
	GetTransactionResultsByBlockID(blockID flowgo.Identifier) ([]*access.TransactionResult, error)

	GetAccount(address flowgo.Address) (*flowgo.Account, error)
	GetAccountAtBlockHeight(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error)

	GetEventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error)

	GetEventsForBlockIDs(eventType string, blockIDs []flowgo.Identifier) ([]flowgo.BlockEvents, error)
	GetEventsForHeightRange(eventType string, startHeight, endHeight uint64) ([]flowgo.BlockEvents, error)

	ExecuteScript(script []byte, arguments [][]byte) (*types.ScriptResult, error)
	ExecuteScriptAtBlockHeight(script []byte, arguments [][]byte, blockHeight uint64) (*types.ScriptResult, error)
	ExecuteScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*types.ScriptResult, error)

	GetAccountStorage(address flowgo.Address) (*AccountStorage, error)

	SendTransaction(tx *flowgo.TransactionBody) error

	AddTransaction(tx flowgo.TransactionBody) error
	ExecuteAndCommitBlock() (*flowgo.Block, []*types.TransactionResult, error)
	ExecuteNextTransaction() (*types.TransactionResult, error)
	ExecuteBlock() ([]*types.TransactionResult, error)
	CommitBlock() (*flowgo.Block, error)
}
