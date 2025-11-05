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

package emulator

import (
	"fmt"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	flowsdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	accessmodel "github.com/onflow/flow-go/model/access"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/types"
)

type ServiceKey struct {
	Index          uint32
	Address        flowsdk.Address
	SequenceNumber uint64
	PrivateKey     sdkcrypto.PrivateKey
	PublicKey      sdkcrypto.PublicKey
	HashAlgo       sdkcrypto.HashAlgorithm
	SigAlgo        sdkcrypto.SignatureAlgorithm
	Weight         int
}

const defaultServiceKeyPrivateKeySeed = "elephant ears space cowboy octopus rodeo potato cannon pineapple"
const DefaultServiceKeySigAlgo = sdkcrypto.ECDSA_P256
const DefaultServiceKeyHashAlgo = sdkcrypto.SHA3_256

func DefaultServiceKey() ServiceKey {
	return GenerateDefaultServiceKey(DefaultServiceKeySigAlgo, DefaultServiceKeyHashAlgo)
}

func GenerateDefaultServiceKey(
	sigAlgo sdkcrypto.SignatureAlgorithm,
	hashAlgo sdkcrypto.HashAlgorithm,
) ServiceKey {
	privateKey, err := sdkcrypto.GeneratePrivateKey(
		sigAlgo,
		[]byte(defaultServiceKeyPrivateKeySeed),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate default service key: %s", err.Error()))
	}

	return ServiceKey{
		PrivateKey: privateKey,
		SigAlgo:    sigAlgo,
		HashAlgo:   hashAlgo,
	}
}

func (s ServiceKey) Signer() (sdkcrypto.Signer, error) {
	return sdkcrypto.NewInMemorySigner(s.PrivateKey, s.HashAlgo)
}

func (s ServiceKey) AccountKey() *flowsdk.AccountKey {

	var publicKey sdkcrypto.PublicKey
	if s.PublicKey != nil {
		publicKey = s.PublicKey
	}

	if s.PrivateKey != nil {
		publicKey = s.PrivateKey.PublicKey()
	}

	return &flowsdk.AccountKey{
		Index:          s.Index,
		PublicKey:      publicKey,
		SigAlgo:        s.SigAlgo,
		HashAlgo:       s.HashAlgo,
		Weight:         s.Weight,
		SequenceNumber: s.SequenceNumber,
	}
}

type CoverageReportCapable interface {
	CoverageReport() *runtime.CoverageReport
	ResetCoverageReport()
}

type ComputationReportCapable interface {
	ComputationReport() *ComputationReport
}

type DebuggingCapable interface {
	StartDebugger() *interpreter.Debugger
	EndDebugging()
	// Deprecated: Needed for the debugger right now, do NOT use for other purposes.
	// TODO: refactor
	GetAccountUnsafe(address flowgo.Address) (*flowgo.Account, error)
}

type SnapshotCapable interface {
	Snapshots() ([]string, error)
	CreateSnapshot(name string) error
	LoadSnapshot(name string) error
}

type RollbackCapable interface {
	RollbackToBlockHeight(height uint64) error
}

type AccessProvider interface {
	Ping() error
	GetNetworkParameters() accessmodel.NetworkParameters

	GetLatestBlock() (*flowgo.Block, error)
	GetBlockByID(id flowgo.Identifier) (*flowgo.Block, error)
	GetBlockByHeight(height uint64) (*flowgo.Block, error)

	GetCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error)
	GetFullCollectionByID(colID flowgo.Identifier) (*flowgo.Collection, error)

	GetTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error)
	GetTransactionResult(txID flowgo.Identifier) (*accessmodel.TransactionResult, error)
	GetTransactionsByBlockID(blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error)
	GetTransactionResultsByBlockID(blockID flowgo.Identifier) ([]*accessmodel.TransactionResult, error)

	// System transaction methods (matches flow-go access.TransactionsAPI)
	GetSystemTransaction(txID flowgo.Identifier, blockID flowgo.Identifier) (*flowgo.TransactionBody, error)
	GetSystemTransactionResult(txID flowgo.Identifier, blockID flowgo.Identifier) (*accessmodel.TransactionResult, error)

	// Scheduled transaction methods (matches flow-go access.TransactionsAPI)
	GetScheduledTransaction(scheduledTxID uint64) (*flowgo.TransactionBody, error)
	GetScheduledTransactionResult(scheduledTxID uint64) (*accessmodel.TransactionResult, error)

	GetAccount(address flowgo.Address) (*flowgo.Account, error)
	GetAccountAtBlockHeight(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error)
	GetAccountByIndex(uint) (*flowgo.Account, error)

	GetEventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error)
	GetEventsForBlockIDs(eventType string, blockIDs []flowgo.Identifier) ([]flowgo.BlockEvents, error)
	GetEventsForHeightRange(eventType string, startHeight, endHeight uint64) ([]flowgo.BlockEvents, error)

	ExecuteScript(script []byte, arguments [][]byte) (*types.ScriptResult, error)
	ExecuteScriptAtBlockHeight(script []byte, arguments [][]byte, blockHeight uint64) (*types.ScriptResult, error)
	ExecuteScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*types.ScriptResult, error)

	SendTransaction(tx *flowgo.TransactionBody) error
	AddTransaction(tx flowgo.TransactionBody) error
}

type AutoMineCapable interface {
	EnableAutoMine()
	DisableAutoMine()
}

type ExecutionCapable interface {
	ExecuteAndCommitBlock() (*flowgo.Block, []*types.TransactionResult, error)
	ExecuteNextTransaction() (*types.TransactionResult, error)
	ExecuteBlock() ([]*types.TransactionResult, error)
	CommitBlock() (*flowgo.Block, error)
}

type LogProvider interface {
	GetLogs(flowgo.Identifier) ([]string, error)
}

type SourceMapCapable interface {
	GetSourceFile(location common.Location) string
}

// Emulator defines the method set of an emulated emulator.
type Emulator interface {
	ServiceKey() ServiceKey

	AccessProvider

	CoverageReportCapable
	ComputationReportCapable
	DebuggingCapable
	SnapshotCapable
	RollbackCapable
	AutoMineCapable
	ExecutionCapable
	LogProvider
	SourceMapCapable
}
