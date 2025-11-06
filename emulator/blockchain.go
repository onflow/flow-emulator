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

// Package emulator provides an emulated version of the Flow emulator that can be used
// for development purposes.
//
// This package can be used as a library or as a standalone application.
//
// When used as a library, this package provides tools to write programmatic tests for
// Flow applications.
//
// When used as a standalone application, this package implements the Flow Access API
// and is fully-compatible with Flow gRPC client libraries.
package emulator

import (
	"context"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	flowsdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	fvmcrypto "github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	accessmodel "github.com/onflow/flow-go/model/access"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/util"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils"
)

// systemChunkTransactionTemplate looks for the RandomBeaconHistory
// heartbeat resource on the service account and calls it.
//
//go:embed templates/systemChunkTransactionTemplate.cdc
var systemChunkTransactionTemplate string

var _ Emulator = &Blockchain{}

// New instantiates a new emulated emulator with the provided options.
func New(opts ...Option) (*Blockchain, error) {
	// apply options to the default config
	conf := defaultConfig
	for _, opt := range opts {
		opt(&conf)
	}

	b := &Blockchain{
		storage:                conf.GetStore(),
		broadcaster:            engine.NewBroadcaster(),
		serviceKey:             conf.GetServiceKey(),
		debugger:               nil,
		activeDebuggingSession: false,
		conf:                   conf,
		clock:                  NewSystemClock(),
		sourceFileMap:          make(map[common.Location]string),
		entropyProvider:        &blockHashEntropyProvider{},
		computationReport: &ComputationReport{
			Scripts:      make(map[string]ProcedureReport),
			Transactions: make(map[string]ProcedureReport),
		},
	}
	err := b.ReloadBlockchain()
	if err != nil {
		return nil, err
	}
	if len(conf.Contracts) > 0 {
		err := DeployContracts(b, conf.Contracts)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Option is a function applying a change to the emulator config.
type Option func(*config)

// WithLogger sets the fvm logger
func WithLogger(
	logger zerolog.Logger,
) Option {
	return func(c *config) {
		c.Logger = logger
	}
}

// WithServerLogger sets the logger
func WithServerLogger(
	logger zerolog.Logger,
) Option {
	return func(c *config) {
		c.ServerLogger = logger
	}
}

// WithServicePublicKey sets the service key from a public key.
func WithServicePublicKey(
	servicePublicKey sdkcrypto.PublicKey,
	sigAlgo sdkcrypto.SignatureAlgorithm,
	hashAlgo sdkcrypto.HashAlgorithm,
) Option {
	return func(c *config) {
		c.ServiceKey = ServiceKey{
			PublicKey: servicePublicKey,
			SigAlgo:   sigAlgo,
			HashAlgo:  hashAlgo,
		}
	}
}

// WithServicePrivateKey sets the service key from private key.
func WithServicePrivateKey(
	privateKey sdkcrypto.PrivateKey,
	sigAlgo sdkcrypto.SignatureAlgorithm,
	hashAlgo sdkcrypto.HashAlgorithm,
) Option {
	return func(c *config) {
		c.ServiceKey = ServiceKey{
			PrivateKey: privateKey,
			PublicKey:  privateKey.PublicKey(),
			HashAlgo:   hashAlgo,
			SigAlgo:    sigAlgo,
		}
	}
}

// WithStore sets the persistent storage provider.
func WithStore(store storage.Store) Option {
	return func(c *config) {
		c.Store = store
	}
}

// WithSimpleAddresses enables simple addresses, which are sequential starting with 0x01.
func WithSimpleAddresses() Option {
	return func(c *config) {
		c.SimpleAddresses = true
	}
}

// WithGenesisTokenSupply sets the genesis token supply.
func WithGenesisTokenSupply(supply cadence.UFix64) Option {
	return func(c *config) {
		c.GenesisTokenSupply = supply
	}
}

// WithTransactionMaxGasLimit sets the maximum gas limit for transactions.
//
// Individual transactions will still be bounded by the limit they declare.
// This function sets the maximum limit that any transaction can declare.
//
// This limit does not affect script executions. Use WithScriptGasLimit
// to set the gas limit for script executions.
func WithTransactionMaxGasLimit(maxLimit uint64) Option {
	return func(c *config) {
		c.TransactionMaxGasLimit = maxLimit
	}
}

// WithScriptGasLimit sets the gas limit for scripts.
//
// This limit does not affect transactions, which declare their own limit.
// Use WithTransactionMaxGasLimit to set the maximum gas limit for transactions.
func WithScriptGasLimit(limit uint64) Option {
	return func(c *config) {
		c.ScriptGasLimit = limit
	}
}

// WithTransactionExpiry sets the transaction expiry measured in blocks.
//
// If set to zero, transaction expiry is disabled and the reference block ID field
// is not required.
func WithTransactionExpiry(expiry uint) Option {
	return func(c *config) {
		c.TransactionExpiry = expiry
	}
}

// WithStorageLimitEnabled enables/disables limiting account storage used to their storage capacity.
//
// If set to false, accounts can store any amount of data,
// otherwise they can only store as much as their storage capacity.
// The default is true.
func WithStorageLimitEnabled(enabled bool) Option {
	return func(c *config) {
		c.StorageLimitEnabled = enabled
	}
}

// WithMinimumStorageReservation sets the minimum account balance.
//
// The cost of creating new accounts is also set to this value.
// The default is taken from fvm.DefaultMinimumStorageReservation
func WithMinimumStorageReservation(minimumStorageReservation cadence.UFix64) Option {
	return func(c *config) {
		c.MinimumStorageReservation = minimumStorageReservation
	}
}

// WithStorageMBPerFLOW sets the cost of a megabyte of storage in FLOW
//
// the default is taken from fvm.DefaultStorageMBPerFLOW
func WithStorageMBPerFLOW(storageMBPerFLOW cadence.UFix64) Option {
	return func(c *config) {
		c.StorageMBPerFLOW = storageMBPerFLOW
	}
}

// WithTransactionFeesEnabled enables/disables transaction fees.
//
// If set to false transactions don't cost any flow.
// The default is false.
func WithTransactionFeesEnabled(enabled bool) Option {
	return func(c *config) {
		c.TransactionFeesEnabled = enabled
	}
}

// WithExecutionEffortWeights sets the execution effort weights.
// default is the Mainnet values.
func WithExecutionEffortWeights(weights meter.ExecutionEffortWeights) Option {
	return func(c *config) {
		c.ExecutionEffortWeights = weights
	}
}

// WithContractRemovalEnabled restricts/allows removal of already deployed contracts.
//
// The default is provided by on-chain value.
func WithContractRemovalEnabled(enabled bool) Option {
	return func(c *config) {
		c.ContractRemovalEnabled = enabled
	}
}

// WithTransactionValidationEnabled enables/disables transaction validation.
//
// If set to false, the emulator will not verify transaction signatures or validate sequence numbers.
//
// The default is true.
func WithTransactionValidationEnabled(enabled bool) Option {
	return func(c *config) {
		c.TransactionValidationEnabled = enabled
	}
}

// WithChainID sets chain type for address generation
// The default is emulator.
func WithChainID(chainID flowgo.ChainID) Option {
	return func(c *config) {
		c.ChainID = chainID
	}
}

// WithCoverageReport injects a CoverageReport to collect coverage information.
//
// The default is nil.
func WithCoverageReport(coverageReport *runtime.CoverageReport) Option {
	return func(c *config) {
		c.CoverageReport = coverageReport
	}
}

func WithComputationReporting(enabled bool) Option {
	return func(c *config) {
		c.ComputationReportingEnabled = enabled
	}
}

func WithScheduledTransactions(enabled bool) Option {
	return func(c *config) {
		c.ScheduledTransactionsEnabled = enabled
	}
}

// WithSetupEVMEnabled enables/disables the EVM setup.
func WithSetupEVMEnabled(enabled bool) Option {
	return func(c *config) {
		c.SetupEVMEnabled = enabled
	}
}

// WithSetupVMBridgeEnabled enables/disables the VM bridge setup.
func WithSetupVMBridgeEnabled(enabled bool) Option {
	return func(c *config) {
		c.SetupVMBridgeEnabled = enabled
	}
}

// Contracts allows users to deploy the given contracts.
// Some default common contracts are pre-configured in the `CommonContracts`
// global variable. It includes contracts such as ExampleNFT but could
// contain more contracts in the future.
// The default value is []ContractDescription{}.
func Contracts(contracts []ContractDescription) Option {
	return func(c *config) {
		c.Contracts = contracts
	}
}

// Blockchain emulates the functionality of the Flow emulator.
type Blockchain struct {
	// committed chain state: blocks, transactions, registers, events
	storage     storage.Store
	broadcaster *engine.Broadcaster

	// mutex protecting pending block
	mu sync.RWMutex

	// pending block containing block info, register state, pending transactions
	pendingBlock *pendingBlock
	clock        Clock

	// used to execute transactions and scripts
	vm    *fvm.VirtualMachine
	vmCtx fvm.Context

	transactionValidator *validator.TransactionValidator

	serviceKey ServiceKey

	debugger               *interpreter.Debugger
	activeDebuggingSession bool
	currentCode            string
	currentScriptID        string

	conf config

	runtime runtime.Runtime

	sourceFileMap map[common.Location]string

	entropyProvider   *blockHashEntropyProvider
	computationReport *ComputationReport
}

// config is a set of configuration options for an emulated emulator.
type config struct {
	ServiceKey                   ServiceKey
	Store                        storage.Store
	SimpleAddresses              bool
	GenesisTokenSupply           cadence.UFix64
	TransactionMaxGasLimit       uint64
	ScriptGasLimit               uint64
	TransactionExpiry            uint
	StorageLimitEnabled          bool
	TransactionFeesEnabled       bool
	ExecutionEffortWeights       meter.ExecutionEffortWeights
	ContractRemovalEnabled       bool
	MinimumStorageReservation    cadence.UFix64
	StorageMBPerFLOW             cadence.UFix64
	Logger                       zerolog.Logger
	ServerLogger                 zerolog.Logger
	TransactionValidationEnabled bool
	ChainID                      flowgo.ChainID
	CoverageReport               *runtime.CoverageReport
	AutoMine                     bool
	Contracts                    []ContractDescription
	ComputationReportingEnabled  bool
	ScheduledTransactionsEnabled bool
	SetupEVMEnabled              bool
	SetupVMBridgeEnabled         bool
}

func (conf config) GetStore() storage.Store {
	if conf.Store == nil {
		store, err := util.CreateDefaultStorage()
		if err != nil {
			panic("Cannot initialize memory storage")
		}
		conf.Store = store
	}
	return conf.Store
}

func (conf config) GetChainID() flowgo.ChainID {
	if conf.SimpleAddresses {
		return flowgo.MonotonicEmulator
	}

	return conf.ChainID
}

func (conf config) GetServiceKey() ServiceKey {
	// set up service key
	serviceKey := conf.ServiceKey
	serviceKey.Address = flowsdk.Address(conf.GetChainID().Chain().ServiceAddress())
	serviceKey.Weight = flowsdk.AccountKeyWeightThreshold
	return serviceKey
}

const defaultGenesisTokenSupply = "1000000000.0"

const defaultScriptGasLimit = 100000

const defaultTransactionMaxGasLimit = flowgo.DefaultMaxTransactionGasLimit

// defaultConfig is the default configuration for an emulated emulator.
var defaultConfig = func() config {
	genesisTokenSupply, err := cadence.NewUFix64(defaultGenesisTokenSupply)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse default genesis token supply: %s", err.Error()))
	}

	return config{
		ServiceKey:                   DefaultServiceKey(),
		Store:                        nil,
		SimpleAddresses:              false,
		GenesisTokenSupply:           genesisTokenSupply,
		ScriptGasLimit:               defaultScriptGasLimit,
		TransactionMaxGasLimit:       defaultTransactionMaxGasLimit,
		MinimumStorageReservation:    fvm.DefaultMinimumStorageReservation,
		StorageMBPerFLOW:             fvm.DefaultStorageMBPerFLOW,
		TransactionExpiry:            0, // TODO: replace with sensible default
		StorageLimitEnabled:          true,
		Logger:                       zerolog.Nop(),
		ServerLogger:                 zerolog.Nop(),
		TransactionValidationEnabled: true,
		ChainID:                      flowgo.Emulator,
		CoverageReport:               nil,
		AutoMine:                     false,
		ComputationReportingEnabled:  false,
		ScheduledTransactionsEnabled: true,
		SetupEVMEnabled:              true,
		SetupVMBridgeEnabled:         true,
	}
}()

func (b *Blockchain) Broadcaster() *engine.Broadcaster {
	return b.broadcaster
}

func (b *Blockchain) ReloadBlockchain() error {
	var err error

	blocks := newBlocks(b)

	b.vm, b.vmCtx, err = configureFVM(b, b.conf, blocks)
	if err != nil {
		return err
	}

	latestBlock, latestLedger, err := configureLedger(
		b.conf,
		b.storage,
		b.vm,
		b.vmCtx)
	if err != nil {
		return err
	}

	b.pendingBlock = newPendingBlock(latestBlock, latestLedger, b.clock)
	b.transactionValidator, err = configureTransactionValidator(b.conf, blocks)
	if err != nil {
		return err
	}

	return nil
}

func (b *Blockchain) EnableAutoMine() {
	b.conf.AutoMine = true
}

func (b *Blockchain) DisableAutoMine() {
	b.conf.AutoMine = false
}

func (b *Blockchain) Ping() error {
	return nil
}

func (b *Blockchain) Runtime() runtime.Runtime {
	return b.runtime
}

func (b *Blockchain) GetChain() flowgo.Chain {
	return b.vmCtx.Chain
}

func (b *Blockchain) GetNetworkParameters() accessmodel.NetworkParameters {
	return accessmodel.NetworkParameters{
		ChainID: b.GetChain().ChainID(),
	}
}

func (b *Blockchain) rollbackProvider() (storage.RollbackProvider, error) {
	rollbackProvider, isRollbackProvider := b.storage.(storage.RollbackProvider)
	if !isRollbackProvider {
		return nil, fmt.Errorf("storage doesn't support rollback")
	}
	return rollbackProvider, nil
}

func (b *Blockchain) RollbackToBlockHeight(height uint64) error {
	rollbackProvider, err := b.rollbackProvider()
	if err != nil {
		return err
	}

	err = rollbackProvider.RollbackToBlockHeight(height)
	if err != nil {
		return err
	}

	return b.ReloadBlockchain()
}

func (b *Blockchain) snapshotProvider() (storage.SnapshotProvider, error) {
	snapshotProvider, isSnapshotProvider := b.storage.(storage.SnapshotProvider)
	if !isSnapshotProvider || !snapshotProvider.SupportSnapshotsWithCurrentConfig() {
		return nil, fmt.Errorf("storage doesn't support snapshots")
	}
	return snapshotProvider, nil
}

func (b *Blockchain) Snapshots() ([]string, error) {
	snapshotProvider, err := b.snapshotProvider()
	if err != nil {
		return []string{}, err
	}
	return snapshotProvider.Snapshots()
}

func (b *Blockchain) CreateSnapshot(name string) error {
	snapshotProvider, err := b.snapshotProvider()
	if err != nil {
		return err
	}
	err = snapshotProvider.CreateSnapshot(name)
	if err != nil {
		return err
	}
	return b.ReloadBlockchain()
}

func (b *Blockchain) LoadSnapshot(name string) error {
	snapshotProvider, err := b.snapshotProvider()
	if err != nil {
		return err
	}
	err = snapshotProvider.LoadSnapshot(name)
	if err != nil {
		return err
	}
	return b.ReloadBlockchain()
}

type CadenceHook struct {
	MainLogger *zerolog.Logger
}

func (h CadenceHook) Run(_ *zerolog.Event, level zerolog.Level, msg string) {
	const logPrefix = "Cadence log:"
	if level != zerolog.NoLevel && strings.HasPrefix(msg, logPrefix) {
		h.MainLogger.Info().Msg(
			strings.Replace(msg,
				logPrefix,
				aurora.Colorize("LOG:", aurora.BlueFg|aurora.BoldFm).String(),
				1))
	}
}

// `blockHashEntropyProvider implements `environment.EntropyProvider`
// which provides a source of entropy to fvm context (required for Cadence's randomness),
// by using the latest block hash.
type blockHashEntropyProvider struct {
	LatestBlock flowgo.Identifier
}

func (gen *blockHashEntropyProvider) RandomSource() ([]byte, error) {
	return gen.LatestBlock[:], nil
}

// make sure `blockHashEntropyProvider implements `environment.EntropyProvider`
var _ environment.EntropyProvider = &blockHashEntropyProvider{}

func configureFVM(blockchain *Blockchain, conf config, blocks *blocks) (*fvm.VirtualMachine, fvm.Context, error) {
	vm := fvm.NewVirtualMachine()

	cadenceLogger := conf.Logger.Hook(CadenceHook{MainLogger: &conf.ServerLogger}).Level(zerolog.DebugLevel)

	runtimeConfig := runtime.Config{
		Debugger:       blockchain.debugger,
		CoverageReport: conf.CoverageReport,
	}
	rt := runtime.NewRuntime(runtimeConfig)
	customRuntimePool := reusableRuntime.NewCustomReusableCadenceRuntimePool(
		1,
		runtimeConfig,
		func(config runtime.Config) runtime.Runtime {
			return rt
		},
	)

	fvmOptions := []fvm.Option{
		fvm.WithLogger(cadenceLogger),
		fvm.WithChain(conf.GetChainID().Chain()),
		fvm.WithBlocks(blocks),
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(!conf.ContractRemovalEnabled),
		fvm.WithComputationLimit(conf.ScriptGasLimit),
		fvm.WithCadenceLogging(true),
		fvm.WithAccountStorageLimit(conf.StorageLimitEnabled),
		fvm.WithTransactionFeesEnabled(conf.TransactionFeesEnabled),
		fvm.WithReusableCadenceRuntimePool(customRuntimePool),
		fvm.WithEntropyProvider(blockchain.entropyProvider),
		fvm.WithEVMEnabled(true),
		fvm.WithScheduleCallbacksEnabled(conf.ScheduledTransactionsEnabled),
	}

	if !conf.TransactionValidationEnabled {
		fvmOptions = append(
			fvmOptions,
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false))
	}

	ctx := fvm.NewContext(
		fvmOptions...,
	)

	blockchain.runtime = rt

	return vm, ctx, nil
}

func configureLedger(
	conf config,
	store storage.Store,
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
) (
	*flowgo.Block,
	snapshot.StorageSnapshot,
	error,
) {
	latestBlock, err := store.LatestBlock(context.Background())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// storage is empty, bootstrap new ledger state
			return configureNewLedger(conf, store, vm, ctx)
		}

		// internal storage error, fail fast
		return nil, nil, err
	}

	// storage contains data, load state from storage
	return configureExistingLedger(&latestBlock, store)
}

func configureNewLedger(
	conf config,
	store storage.Store,
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
) (
	*flowgo.Block,
	snapshot.StorageSnapshot,
	error,
) {
	ledger, err := store.LedgerByHeight(context.Background(), 0)
	if err != nil {
		return nil, nil, err
	}

	genesisExecutionSnapshot, output, err := bootstrapLedger(vm, ctx, ledger, conf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to bootstrap execution state: %w", err)
	}

	// commit the genesis block to storage
	genesis := Genesis(conf.GetChainID())
	// The `output.Events` slice contains only EVM-related events emitted
	// during the VM bridge bootstrap. This is needed for the proper
	// setup of the EVM Gateway, since missing events will cause state
	// mismatch, preventing the service to run.
	err = store.CommitBlock(
		context.Background(),
		*genesis,
		nil,
		nil,
		nil,
		genesisExecutionSnapshot,
		output.Events,
		nil,
	)
	if err != nil {
		return nil, nil, err
	}

	// get empty ledger view
	ledger, err = store.LedgerByHeight(context.Background(), 0)
	if err != nil {
		return nil, nil, err
	}

	return genesis, ledger, nil
}

func configureExistingLedger(
	latestBlock *flowgo.Block,
	store storage.Store,
) (
	*flowgo.Block,
	snapshot.StorageSnapshot,
	error,
) {
	latestLedger, err := store.LedgerByHeight(
		context.Background(),
		latestBlock.Height,
	)
	if err != nil {
		return nil, nil, err
	}

	return latestBlock, latestLedger, nil
}

func bootstrapLedger(
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	ledger snapshot.StorageSnapshot,
	conf config,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	accountKey := conf.GetServiceKey().AccountKey()
	publicKey, _ := crypto.DecodePublicKey(
		accountKey.SigAlgo,
		accountKey.PublicKey.Encode(),
	)

	ctx = fvm.NewContextFromParent(
		ctx,
		fvm.WithAccountStorageLimit(false),
	)

	flowAccountKey := flowgo.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  accountKey.SigAlgo,
		HashAlgo:  accountKey.HashAlgo,
		Weight:    fvm.AccountKeyWeightThreshold,
	}

	bootstrap := configureBootstrapProcedure(conf, flowAccountKey, conf.GenesisTokenSupply)

	executionSnapshot, output, err := vm.Run(ctx, bootstrap, ledger)
	if err != nil {
		return nil, fvm.ProcedureOutput{}, err
	}

	if output.Err != nil {
		return nil, fvm.ProcedureOutput{}, output.Err
	}

	return executionSnapshot, output, nil
}

func configureBootstrapProcedure(conf config, flowAccountKey flowgo.AccountPublicKey, supply cadence.UFix64) *fvm.BootstrapProcedure {
	options := make([]fvm.BootstrapProcedureOption, 0)
	options = append(options,
		fvm.WithInitialTokenSupply(supply),
		fvm.WithRestrictedAccountCreationEnabled(false),
		// This enables variable transaction fees AND execution effort metering
		// as described in Variable Transaction Fees:
		// Execution Effort FLIP: https://github.com/onflow/flow/pull/753)
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithExecutionMemoryLimit(math.MaxUint32),
		fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		fvm.WithExecutionEffortWeights(environment.MainnetExecutionEffortWeights),
		fvm.WithSetupVMBridgeEnabled(cadence.NewBool(conf.SetupVMBridgeEnabled)),
		fvm.WithSetupEVMEnabled(cadence.NewBool(conf.SetupEVMEnabled)),
	)

	if conf.ExecutionEffortWeights != nil {
		options = append(options,
			fvm.WithExecutionEffortWeights(conf.ExecutionEffortWeights),
		)
	}
	if conf.StorageLimitEnabled {
		options = append(options,
			fvm.WithAccountCreationFee(conf.MinimumStorageReservation),
			fvm.WithMinimumStorageReservation(conf.MinimumStorageReservation),
			fvm.WithStorageMBPerFLOW(conf.StorageMBPerFLOW),
		)
	}

	return fvm.Bootstrap(
		flowAccountKey,
		options...,
	)
}

func configureTransactionValidator(conf config, blocks *blocks) (*validator.TransactionValidator, error) {
	return validator.NewTransactionValidator(
		blocks,
		conf.GetChainID().Chain(),
		metrics.NewNoopCollector(),
		validator.TransactionValidationOptions{
			Expiry:                       conf.TransactionExpiry,
			ExpiryBuffer:                 0,
			AllowEmptyReferenceBlockID:   conf.TransactionExpiry == 0,
			AllowUnknownReferenceBlockID: false,
			MaxGasLimit:                  conf.TransactionMaxGasLimit,
			CheckScriptsParse:            true,
			MaxTransactionByteSize:       flowgo.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flowgo.DefaultMaxCollectionByteSize,
			CheckPayerBalanceMode:        validator.Disabled,
		},
		nil,
	)
}

func (b *Blockchain) setFVMContextFromHeader(header *flowgo.Header) fvm.Context {
	b.vmCtx = fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithBlockHeader(header),
	)

	return b.vmCtx
}

func (b *Blockchain) CurrentScript() (string, string) {
	return b.currentScriptID, b.currentCode
}

// ServiceKey returns the service private key for this emulator.
func (b *Blockchain) ServiceKey() ServiceKey {
	serviceAccount, err := b.getAccount(flowgo.Address(b.serviceKey.Address))
	if err != nil {
		return b.serviceKey
	}

	if len(serviceAccount.Keys) > 0 {
		b.serviceKey.Index = 0
		b.serviceKey.SequenceNumber = serviceAccount.Keys[0].SeqNumber
		b.serviceKey.Weight = serviceAccount.Keys[0].Weight
	}

	return b.serviceKey
}

// PendingBlockID returns the ID of the pending block.
func (b *Blockchain) PendingBlockID() flowgo.Identifier {
	return b.pendingBlock.ID()
}

// PendingBlockView returns the view of the pending block.
func (b *Blockchain) PendingBlockView() uint64 {
	return b.pendingBlock.view
}

// PendingBlockTimestamp returns the Timestamp of the pending block.
func (b *Blockchain) PendingBlockTimestamp() time.Time {
	return time.UnixMilli(int64(b.pendingBlock.Block().Timestamp)).UTC()
}

// GetLatestBlock gets the latest sealed block.
func (b *Blockchain) GetLatestBlock() (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getLatestBlock()
}

func (b *Blockchain) getLatestBlock() (*flowgo.Block, error) {
	block, err := b.storage.LatestBlock(context.Background())
	if err != nil {
		return nil, err
	}

	return &block, nil
}

// GetBlockByID gets a block by ID.
func (b *Blockchain) GetBlockByID(id flowgo.Identifier) (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getBlockByID(id)
}

func (b *Blockchain) getBlockByID(id flowgo.Identifier) (*flowgo.Block, error) {
	block, err := b.storage.BlockByID(context.Background(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.BlockNotFoundByIDError{ID: id}
		}

		return nil, err
	}

	return block, nil
}

// GetBlockByHeight gets a block by height.
func (b *Blockchain) GetBlockByHeight(height uint64) (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, err := b.getBlockByHeight(height)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) getBlockByHeight(height uint64) (*flowgo.Block, error) {
	block, err := b.storage.BlockByHeight(context.Background(), height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.BlockNotFoundByHeightError{Height: height}
		}
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) GetCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getCollectionByID(colID)
}

func (b *Blockchain) getCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error) {
	col, err := b.storage.CollectionByID(context.Background(), colID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.CollectionNotFoundError{ID: colID}
		}
		return nil, err
	}

	return &col, nil
}

func (b *Blockchain) GetFullCollectionByID(colID flowgo.Identifier) (*flowgo.Collection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getFullCollectionByID(colID)
}

func (b *Blockchain) getFullCollectionByID(colID flowgo.Identifier) (*flowgo.Collection, error) {
	col, err := b.storage.FullCollectionByID(context.Background(), colID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.CollectionNotFoundError{ID: colID}
		}
		return nil, err
	}

	return &col, nil
}

// GetTransaction gets an existing transaction by ID.
//
// The function first looks in the pending block, then the current emulator state.
func (b *Blockchain) GetTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getTransaction(txID)
}

func (b *Blockchain) getTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error) {
	pendingTx := b.pendingBlock.GetTransaction(txID)
	if pendingTx != nil {
		return pendingTx, nil
	}

	tx, err := b.storage.TransactionByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: txID}
		}
		return nil, err
	}

	return &tx, nil
}

func (b *Blockchain) GetTransactionResult(txID flowgo.Identifier) (*accessmodel.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.getTransactionResult(txID)
}

func (b *Blockchain) getTransactionResult(txID flowgo.Identifier) (*accessmodel.TransactionResult, error) {
	if b.pendingBlock.ContainsTransaction(txID) {
		return &accessmodel.TransactionResult{
			Status: flowgo.TransactionStatusPending,
		}, nil
	}

	storedResult, err := b.storage.TransactionResultByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return &accessmodel.TransactionResult{
				Status: flowgo.TransactionStatusUnknown,
			}, nil
		}
		return nil, err
	}

	statusCode := 0
	if storedResult.ErrorCode > 0 {
		statusCode = 1
	}
	result := accessmodel.TransactionResult{
		Status:        flowgo.TransactionStatusSealed,
		StatusCode:    uint(statusCode),
		ErrorMessage:  storedResult.ErrorMessage,
		Events:        storedResult.Events,
		TransactionID: txID,
		BlockHeight:   storedResult.BlockHeight,
		BlockID:       storedResult.BlockID,
		CollectionID:  storedResult.CollectionID,
	}

	return &result, nil
}

// GetAccountByIndex returns the account for the given address.
func (b *Blockchain) GetAccountByIndex(index uint) (*flowgo.Account, error) {
	generator := flowsdk.NewAddressGenerator(flowsdk.ChainID(b.vmCtx.Chain.ChainID()))

	generator.SetIndex(index)

	account, err := b.GetAccountUnsafe(convert.SDKAddressToFlow(generator.Address()))
	if err != nil {
		return nil, err
	}

	return account, nil
}

// Deprecated: Needed for the debugger right now, do NOT use for other purposes.
// TODO: refactor
func (b *Blockchain) GetAccountUnsafe(address flowgo.Address) (*flowgo.Account, error) {
	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Height)
}

// GetAccount returns the account for the given address.
func (b *Blockchain) GetAccount(address flowgo.Address) (*flowgo.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getAccount(address)
}

// getAccount returns the account for the given address.
func (b *Blockchain) getAccount(address flowgo.Address) (*flowgo.Account, error) {
	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Height)
}

// GetAccountAtBlockHeight  returns the account for the given address at specified block height.
func (b *Blockchain) GetAccountAtBlockHeight(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getAccountAtBlock(address, blockHeight)
}

// GetAccountAtBlock returns the account for the given address at specified block height.
func (b *Blockchain) getAccountAtBlock(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {
	ledger, err := b.storage.LedgerByHeight(context.Background(), blockHeight)
	if err != nil {
		return nil, err
	}

	account, err := fvm.GetAccount(b.vmCtx, address, ledger)
	if fvmerrors.IsAccountNotFoundError(err) {
		return nil, &types.AccountNotFoundError{Address: address}
	}

	return account, err
}

func (b *Blockchain) GetEventsForBlockIDs(eventType string, blockIDs []flowgo.Identifier) (result []flowgo.BlockEvents, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, blockID := range blockIDs {
		block, err := b.storage.BlockByID(context.Background(), blockID)
		if err != nil {
			break
		}
		events, err := b.storage.EventsByHeight(context.Background(), block.Height, eventType)
		if err != nil {
			break
		}
		result = append(result, flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Height,
			BlockTimestamp: time.UnixMilli(int64(block.Timestamp)).UTC(),
			Events:         events,
		})
	}

	return result, err
}

func (b *Blockchain) GetEventsForHeightRange(eventType string, startHeight, endHeight uint64) (result []flowgo.BlockEvents, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for blockHeight := startHeight; blockHeight <= endHeight; blockHeight++ {
		block, err := b.storage.BlockByHeight(context.Background(), blockHeight)
		if err != nil {
			break
		}

		events, err := b.storage.EventsByHeight(context.Background(), blockHeight, eventType)
		if err != nil {
			break
		}

		result = append(result, flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Height,
			BlockTimestamp: time.UnixMilli(int64(block.Timestamp)).UTC(),
			Events:         events,
		})
	}

	return result, err
}

// GetEventsByHeight returns the events in the block at the given height, optionally filtered by type.
func (b *Blockchain) GetEventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.storage.EventsByHeight(context.Background(), blockHeight, eventType)
}

// SendTransaction submits a transaction to the network.
func (b *Blockchain) SendTransaction(flowTx *flowgo.TransactionBody) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	err := b.addTransaction(*flowTx)
	if err != nil {
		return err
	}

	if b.conf.AutoMine {
		_, _, err := b.executeAndCommitBlock()
		if err != nil {
			return err
		}
	}

	return nil
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) AddTransaction(tx flowgo.TransactionBody) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.addTransaction(tx)
}

func (b *Blockchain) addTransaction(tx flowgo.TransactionBody) error {
	// If index > 0, pending block has begun execution (cannot add more transactions)
	if b.pendingBlock.ExecutionStarted() {
		return &types.PendingBlockMidExecutionError{BlockID: b.pendingBlock.ID()}
	}

	if b.pendingBlock.ContainsTransaction(tx.ID()) {
		return &types.DuplicateTransactionError{TxID: tx.ID()}
	}

	_, err := b.storage.TransactionByID(context.Background(), tx.ID())
	if err == nil {
		// Found the transaction, this is a duplicate
		return &types.DuplicateTransactionError{TxID: tx.ID()}
	} else if !errors.Is(err, storage.ErrNotFound) {
		// Error in the storage provider
		return fmt.Errorf("failed to check storage for transaction %w", err)
	}

	err = b.transactionValidator.Validate(context.Background(), &tx)
	if err != nil {
		return types.ConvertAccessError(err)
	}

	// add transaction to pending block
	b.pendingBlock.AddTransaction(tx)

	return nil
}

// ExecuteBlock executes the remaining transactions in pending block.
func (b *Blockchain) ExecuteBlock() ([]*types.TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeBlock()
}

func (b *Blockchain) executeBlock() ([]*types.TransactionResult, error) {
	results := make([]*types.TransactionResult, 0)

	header := b.pendingBlock.Block().ToHeader()
	blockContext := b.setFVMContextFromHeader(header)

	// cannot execute a block that has already executed (only relevant for non-empty blocks)
	if b.pendingBlock.ExecutionComplete() && !b.pendingBlock.Empty() {
		return results, &types.PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.ID(),
		}
	}

	// continue executing transactions until execution is complete (skip for empty blocks)
	for !b.pendingBlock.ExecutionComplete() && !b.pendingBlock.Empty() {
		result, err := b.executeNextTransaction(blockContext)
		if err != nil {
			return results, err
		}

		results = append(results, result)
	}

	return results, nil
}

// ExecuteNextTransaction executes the next indexed transaction in pending block.
func (b *Blockchain) ExecuteNextTransaction() (*types.TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	block := b.pendingBlock.Block()

	// Use a trusted header derived from the block to avoid nil header cases
	blockContext := b.setFVMContextFromHeader(block.ToHeader())
	return b.executeNextTransaction(blockContext)
}

// executeNextTransaction is a helper function for ExecuteBlock and ExecuteNextTransaction that
// executes the next transaction in the pending block.
func (b *Blockchain) executeNextTransaction(ctx fvm.Context) (*types.TransactionResult, error) {
	// check if there are remaining txs to be executed
	if b.pendingBlock.ExecutionComplete() {
		return nil, &types.PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.ID(),
		}
	}

	txnBody := b.pendingBlock.NextTransaction()
	txnID := txnBody.ID()

	b.currentCode = string(txnBody.Script)
	b.currentScriptID = txnID.String()

	pragmas := ExtractPragmas(b.currentCode)

	if b.activeDebuggingSession && pragmas.Contains(PragmaDebug) {
		b.debugger.RequestPause()
	}

	// use the computer to execute the next transaction
	output, err := b.pendingBlock.ExecuteNextTransaction(b.vm, ctx)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	tr, err := convert.VMTransactionResultToEmulator(txnID, output)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	// if transaction error exist try to further debug what was the problem
	if tr.Error != nil {
		tr.Debug = b.debugSignatureError(tr.Error, txnBody)
	}

	// add to source map if any pragma
	if pragmas.Contains(PragmaSourceFile) {
		location := common.NewTransactionLocation(nil, tr.TransactionID.Bytes())
		sourceFile := pragmas.FilterByName(PragmaSourceFile).First().Argument()
		b.sourceFileMap[location] = sourceFile
	}

	if b.conf.ComputationReportingEnabled {
		location := common.NewTransactionLocation(nil, tr.TransactionID.Bytes())
		arguments := make([]string, 0)
		for _, argument := range txnBody.Arguments {
			arguments = append(arguments, string(argument))
		}
		b.computationReport.ReportTransaction(
			tr,
			b.sourceFileMap[location],
			b.currentCode,
			arguments,
			output.ComputationIntensities,
		)
	}

	return tr, nil
}

// CommitBlock seals the current pending block and saves it to storage.
//
// This function clears the pending transaction pool and resets the pending block.
func (b *Blockchain) CommitBlock() (*flowgo.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	block, err := b.commitBlock()
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) commitBlock() (*flowgo.Block, error) {
	// pending block cannot be committed before execution starts (unless empty)
	if !b.pendingBlock.ExecutionStarted() && !b.pendingBlock.Empty() {
		return nil, &types.PendingBlockCommitBeforeExecutionError{BlockID: b.pendingBlock.ID()}
	}

	// pending block cannot be committed before execution completes
	if b.pendingBlock.ExecutionStarted() && !b.pendingBlock.ExecutionComplete() {
		return nil, &types.PendingBlockMidExecutionError{BlockID: b.pendingBlock.ID()}
	}

	block := b.pendingBlock.Block()
	collections := b.pendingBlock.Collections()

	collectionID := flowgo.ZeroID

	if len(collections) > 0 {
		collectionID = collections[0].ID()
	}
	transactions := b.pendingBlock.Transactions()
	transactionResults, err := convertToSealedResults(b.pendingBlock.TransactionResults(), b.pendingBlock.ID(), b.pendingBlock.height, collectionID)
	if err != nil {
		return nil, err
	}

	// execute scheduled transactions out-of-band before the system chunk
	// (does not change collections or payload)
	blockContext := fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithBlockHeader(block.ToHeader()),
	)
	systemTransactions := storage.SystemTransactions{
		BlockID:      b.pendingBlock.ID(),
		Transactions: []flowgo.Identifier{},
	}
	if b.conf.ScheduledTransactionsEnabled {
		tx, results, scheduledTxIDs, err := b.executeScheduledTransactions(blockContext)
		if err != nil {
			return nil, err
		}

		convertedResults, err := convertToSealedResults(results, b.pendingBlock.ID(), b.pendingBlock.height, flowgo.ZeroID)
		if err != nil {
			return nil, err
		}
		maps.Copy(transactionResults, convertedResults)
		for id, t := range tx {
			transactions[id] = t
			systemTransactions.Transactions = append(systemTransactions.Transactions, id)
		}

		// Store scheduled transaction ID mappings
		systemTransactions.ScheduledTransactionIDs = scheduledTxIDs
	}

	// lastly we execute the system chunk transaction
	chunkBody, itr, err := b.executeSystemChunkTransaction()
	if err != nil {
		return nil, err
	}

	systemTxID := chunkBody.ID()
	systemTxStorableResult, err := convert.ToStorableResult(itr.ProcedureOutput, b.pendingBlock.ID(), b.pendingBlock.height, flowgo.ZeroID)
	if err != nil {
		return nil, err
	}
	transactions[systemTxID] = chunkBody
	transactionResults[systemTxID] = &systemTxStorableResult
	systemTransactions.Transactions = append(systemTransactions.Transactions, systemTxID)

	executionSnapshot := b.pendingBlock.Finalize()
	events := b.pendingBlock.Events()

	// should we try to just store a collection with same as block ID
	err = b.storage.CommitBlock(
		context.Background(),
		*block,
		collections,
		transactions,
		transactionResults,
		executionSnapshot,
		events,
		&systemTransactions)
	if err != nil {
		return nil, err
	}

	// Index scheduled transactions globally (scheduledTxID → blockID)
	for scheduledTxID := range systemTransactions.ScheduledTransactionIDs {
		err = b.storage.IndexScheduledTransactionID(context.Background(), scheduledTxID, block.ID())
		if err != nil {
			return nil, fmt.Errorf("failed to index scheduled transaction %d: %w", scheduledTxID, err)
		}
	}

	ledger, err := b.storage.LedgerByHeight(
		context.Background(),
		block.Height,
	)
	if err != nil {
		return nil, err
	}

	// notify listeners on new block
	b.broadcaster.Publish()

	// reset pending block using current block and ledger state
	b.pendingBlock = newPendingBlock(block, ledger, b.clock)
	b.entropyProvider.LatestBlock = block.ID()

	return block, nil
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) ExecuteAndCommitBlock() (*flowgo.Block, []*types.TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeAndCommitBlock()
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) executeAndCommitBlock() (*flowgo.Block, []*types.TransactionResult, error) {
	results, err := b.executeBlock()
	if err != nil {
		return nil, nil, err
	}

	block, err := b.commitBlock()
	if err != nil {
		return nil, results, err
	}

	for _, result := range results {
		utils.PrintTransactionResult(&b.conf.ServerLogger, result)
	}

	blockID := block.ID()
	b.conf.ServerLogger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msgf("📦 Block #%d committed", block.Height)

	return block, results, nil
}

// ResetPendingBlock clears the transactions in pending block.
func (b *Blockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	latestBlock, err := b.storage.LatestBlock(context.Background())
	if err != nil {
		return err
	}

	latestLedger, err := b.storage.LedgerByHeight(
		context.Background(),
		latestBlock.Height,
	)
	if err != nil {
		return err
	}

	// reset pending block using latest committed block and ledger state
	b.pendingBlock = newPendingBlock(&latestBlock, latestLedger, b.clock)

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *Blockchain) ExecuteScript(
	script []byte,
	arguments [][]byte,
) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}

	return b.executeScriptAtBlockID(script, arguments, latestBlock.ID())
}

func (b *Blockchain) ExecuteScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.executeScriptAtBlockID(script, arguments, id)
}

func (b *Blockchain) executeScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*types.ScriptResult, error) {
	requestedBlock, err := b.storage.BlockByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	requestedLedgerSnapshot, err := b.storage.LedgerByHeight(
		context.Background(),
		requestedBlock.Height,
	)
	if err != nil {
		return nil, err
	}

	blockContext := fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithBlockHeader(requestedBlock.ToHeader()),
	)

	scriptProc := fvm.Script(script).WithArguments(arguments...)
	b.currentCode = string(script)
	b.currentScriptID = scriptProc.ID.String()

	pragmas := ExtractPragmas(b.currentCode)

	if b.activeDebuggingSession && pragmas.Contains(PragmaDebug) {
		b.debugger.RequestPause()
	}
	_, output, err := b.vm.Run(
		blockContext,
		scriptProc,
		requestedLedgerSnapshot)
	if err != nil {
		return nil, err
	}

	scriptID := flowsdk.Identifier(flowgo.MakeIDFromFingerPrint(script))

	events, err := convert.FlowEventsToSDK(output.Events)
	if err != nil {
		return nil, err
	}

	var scriptError error = nil
	var convertedValue cadence.Value = nil

	if output.Err == nil {
		convertedValue = output.Value
	} else {
		scriptError = convert.VMErrorToEmulator(output.Err)
	}

	// add to source map if any pragma
	if pragmas.Contains(PragmaSourceFile) {
		location := common.NewScriptLocation(nil, scriptID.Bytes())
		sourceFile := pragmas.FilterByName(PragmaSourceFile).First().Argument()
		b.sourceFileMap[location] = sourceFile
	}

	scriptResult := &types.ScriptResult{
		ScriptID:        scriptID,
		Value:           convertedValue,
		Error:           scriptError,
		Logs:            output.Logs,
		Events:          events,
		ComputationUsed: output.ComputationUsed,
		MemoryEstimate:  output.MemoryEstimate,
	}

	if b.conf.ComputationReportingEnabled {
		location := common.NewScriptLocation(nil, scriptID.Bytes())
		scriptArguments := make([]string, 0)
		for _, argument := range arguments {
			scriptArguments = append(scriptArguments, string(argument))
		}
		b.computationReport.ReportScript(
			scriptResult,
			b.sourceFileMap[location],
			b.currentCode,
			scriptArguments,
			output.ComputationIntensities,
		)
	}

	return scriptResult, nil
}

func (b *Blockchain) ExecuteScriptAtBlockHeight(
	script []byte,
	arguments [][]byte,
	blockHeight uint64,
) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	requestedBlock, err := b.getBlockByHeight(blockHeight)
	if err != nil {
		return nil, err
	}

	return b.executeScriptAtBlockID(script, arguments, requestedBlock.ID())
}

func convertToSealedResults(
	results map[flowgo.Identifier]IndexedTransactionResult,
	blockID flowgo.Identifier,
	blockHeight uint64,
	collectionID flowgo.Identifier,
) (map[flowgo.Identifier]*types.StorableTransactionResult, error) {
	output := make(map[flowgo.Identifier]*types.StorableTransactionResult)

	for id, result := range results {
		temp, err := convert.ToStorableResult(result.ProcedureOutput, blockID, blockHeight, collectionID)
		if err != nil {
			return nil, err
		}
		output[id] = &temp
	}

	return output, nil
}

// debugSignatureError tries to unwrap error to the root and test for invalid hashing algorithms
func (b *Blockchain) debugSignatureError(err error, tx *flowgo.TransactionBody) *types.TransactionResultDebug {
	if fvmerrors.HasErrorCode(err, fvmerrors.ErrCodeInvalidEnvelopeSignatureError) {
		for _, sig := range tx.EnvelopeSignatures {
			debug := b.testAlternativeHashAlgo(sig, tx.EnvelopeMessage())
			if debug != nil {
				return debug
			}
		}
	}
	if fvmerrors.HasErrorCode(err, fvmerrors.ErrCodeInvalidPayloadSignatureError) {
		for _, sig := range tx.PayloadSignatures {
			debug := b.testAlternativeHashAlgo(sig, tx.PayloadMessage())
			if debug != nil {
				return debug
			}
		}
	}

	return types.NewTransactionInvalidSignature(tx)
}

// testAlternativeHashAlgo tries to verify the signature with alternative hashing algorithm and if
// the signature is verified returns more verbose error
func (b *Blockchain) testAlternativeHashAlgo(sig flowgo.TransactionSignature, msg []byte) *types.TransactionResultDebug {
	acc, err := b.getAccount(sig.Address)
	if err != nil {
		return nil
	}

	key := acc.Keys[sig.KeyIndex]

	for _, algo := range []hash.HashingAlgorithm{sdkcrypto.SHA2_256, sdkcrypto.SHA3_256} {
		if key.HashAlgo == algo {
			continue // skip valid hash algo
		}

		h, _ := fvmcrypto.NewPrefixedHashing(algo, flowgo.TransactionTagString)
		valid, _ := key.PublicKey.Verify(sig.Signature, msg, h)
		if valid {
			return types.NewTransactionInvalidHashAlgo(key, acc.Address, algo)
		}
	}

	return nil
}

func (b *Blockchain) StartDebugger() *interpreter.Debugger {
	b.activeDebuggingSession = true
	return b.debugger
}

func (b *Blockchain) EndDebugging() {
	b.activeDebuggingSession = false
}

func (b *Blockchain) CoverageReport() *runtime.CoverageReport {
	return b.conf.CoverageReport
}

func (b *Blockchain) ComputationReport() *ComputationReport {
	return b.computationReport
}

func (b *Blockchain) ResetCoverageReport() {
	b.conf.CoverageReport.Reset()
}

func (b *Blockchain) GetTransactionsByBlockID(blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, err := b.getBlockByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockID, err)
	}

	var transactions []*flowgo.TransactionBody
	for i, guarantee := range block.Payload.Guarantees {
		c, err := b.getCollectionByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get collection [%d] %s: %w", i, guarantee.CollectionID, err)
		}

		for j, txID := range c.Transactions {
			tx, err := b.getTransaction(txID)
			if err != nil {
				return nil, fmt.Errorf("failed to get transaction [%d] %s: %w", j, txID, err)
			}
			transactions = append(transactions, tx)
		}
	}
	st, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get system transactions %w", err)
	}
	for j, txID := range st.Transactions {
		tx, err := b.getTransaction(txID)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction [%d] %s: %w", j, txID, err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

func (b *Blockchain) GetTransactionResultsByBlockID(blockID flowgo.Identifier) ([]*accessmodel.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, err := b.getBlockByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockID, err)
	}

	var results []*accessmodel.TransactionResult
	for i, guarantee := range block.Payload.Guarantees {
		c, err := b.getCollectionByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get collection [%d] %s: %w", i, guarantee.CollectionID, err)
		}

		for j, txID := range c.Transactions {
			result, err := b.getTransactionResult(txID)
			if err != nil {
				return nil, fmt.Errorf("failed to get transaction result [%d] %s: %w", j, txID, err)
			}
			results = append(results, result)
		}
	}

	st, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get system transactions %w", err)
	}
	for j, txID := range st.Transactions {
		result, err := b.getTransactionResult(txID)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction result [%d] %s: %w", j, txID, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetSystemTransactionsForBlock returns the list of system transaction IDs for a given block.
func (b *Blockchain) GetSystemTransactionsForBlock(blockID flowgo.Identifier) ([]flowgo.Identifier, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// No system transactions for this block (e.g., older blocks before this feature)
			return []flowgo.Identifier{}, nil
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	return systemTxs.Transactions, nil
}

// GetSystemTransaction returns a system transaction by its ID and block ID.
// System transactions include scheduled transactions and the system chunk transaction.
func (b *Blockchain) GetSystemTransaction(txID flowgo.Identifier, blockID flowgo.Identifier) (*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// First, verify this is actually a system transaction for the given block
	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: txID}
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	// Check if the transaction is in the system transactions list
	found := false
	for _, id := range systemTxs.Transactions {
		if id == txID {
			found = true
			break
		}
	}

	if !found {
		return nil, &types.TransactionNotFoundError{ID: txID}
	}

	// Retrieve the transaction body
	return b.getTransaction(txID)
}

// GetSystemTransactionResult returns the result of a system transaction by its ID and block ID.
func (b *Blockchain) GetSystemTransactionResult(txID flowgo.Identifier, blockID flowgo.Identifier) (*accessmodel.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// First, verify this is actually a system transaction for the given block
	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: txID}
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	// Check if the transaction is in the system transactions list
	found := false
	for _, id := range systemTxs.Transactions {
		if id == txID {
			found = true
			break
		}
	}

	if !found {
		return nil, &types.TransactionNotFoundError{ID: txID}
	}

	// Retrieve the transaction result
	return b.getTransactionResult(txID)
}

// GetScheduledTransactionByBlockID returns a scheduled transaction by its scheduled transaction ID and block ID.
// This is a helper method that requires knowing which block contains the scheduled transaction.
func (b *Blockchain) GetScheduledTransactionByBlockID(scheduledTxID uint64, blockID flowgo.Identifier) (*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Get the system transactions for the block
	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	// Look up the transaction ID from the scheduled transaction ID
	txID, found := systemTxs.ScheduledTransactionIDs[scheduledTxID]
	if !found {
		return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
	}

	// Retrieve the transaction body
	return b.getTransaction(txID)
}

// GetScheduledTransaction returns a scheduled transaction by its scheduled transaction ID (uint64).
// Uses the global index to look up which block contains the transaction.
func (b *Blockchain) GetScheduledTransaction(scheduledTxID uint64) (*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Look up the block ID from the global index
	blockID, err := b.storage.BlockIDByScheduledTransactionID(context.Background(), scheduledTxID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
		}
		return nil, fmt.Errorf("failed to find block for scheduled transaction %d: %w", scheduledTxID, err)
	}

	// Get the system transactions for the block
	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	// Look up the transaction ID from the scheduled transaction ID
	txID, found := systemTxs.ScheduledTransactionIDs[scheduledTxID]
	if !found {
		return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
	}

	// Retrieve the transaction body
	return b.getTransaction(txID)
}

// GetScheduledTransactionResultByBlockID returns the result of a scheduled transaction by its scheduled transaction ID and block ID.
// This is a helper method that requires knowing which block contains the scheduled transaction.
func (b *Blockchain) GetScheduledTransactionResultByBlockID(scheduledTxID uint64, blockID flowgo.Identifier) (*accessmodel.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Get the system transactions for the block
	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	// Look up the transaction ID from the scheduled transaction ID
	txID, found := systemTxs.ScheduledTransactionIDs[scheduledTxID]
	if !found {
		return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
	}

	// Retrieve the transaction result
	return b.getTransactionResult(txID)
}

// GetScheduledTransactionResult returns the result of a scheduled transaction by its scheduled transaction ID.
// Uses the global index to look up which block contains the transaction.
func (b *Blockchain) GetScheduledTransactionResult(scheduledTxID uint64) (*accessmodel.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Look up the block ID from the global index
	blockID, err := b.storage.BlockIDByScheduledTransactionID(context.Background(), scheduledTxID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
		}
		return nil, fmt.Errorf("failed to find block for scheduled transaction %d: %w", scheduledTxID, err)
	}

	// Get the system transactions for the block
	systemTxs, err := b.storage.SystemTransactionsForBlockID(context.Background(), blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
		}
		return nil, fmt.Errorf("failed to get system transactions for block %s: %w", blockID, err)
	}

	// Look up the transaction ID from the scheduled transaction ID
	txID, found := systemTxs.ScheduledTransactionIDs[scheduledTxID]
	if !found {
		return nil, &types.TransactionNotFoundError{ID: flowgo.ZeroID}
	}

	// Retrieve the transaction result
	return b.getTransactionResult(txID)
}

func (b *Blockchain) GetLogs(identifier flowgo.Identifier) ([]string, error) {
	txResult, err := b.storage.TransactionResultByID(context.Background(), identifier)
	if err != nil {
		return nil, err
	}
	return txResult.Logs, nil
}

// SetClock sets the given clock on blockchain's pending block.
// After this block is committed, the block timestamp will
// contain the value of clock.Now().
func (b *Blockchain) SetClock(clock Clock) {
	b.clock = clock
	b.pendingBlock.SetClock(clock)
}

// NewScriptEnvironment returns an environment.Environment by
// using as a storage snapshot the blockchain's ledger state.
// Useful for tools that use the emulator's blockchain as a library.
func (b *Blockchain) NewScriptEnvironment() environment.Environment {
	return environment.NewScriptEnvironmentFromStorageSnapshot(
		b.vmCtx.EnvironmentParams,
		b.pendingBlock.ledgerState.NewChild(),
	)
}

func (b *Blockchain) GetSourceFile(location common.Location) string {
	value, exists := b.sourceFileMap[location]
	if exists {
		return value
	}

	addressLocation, isAddressLocation := location.(common.AddressLocation)
	if !isAddressLocation {
		return location.ID()
	}

	env := b.NewScriptEnvironment()
	r := b.vmCtx.Borrow(env)
	defer b.vmCtx.Return(r)

	code, err := r.TxRuntimeEnv.GetAccountContractCode(addressLocation)
	if err != nil {
		return location.ID()
	}
	pragmas := ExtractPragmas(string(code))
	if pragmas.Contains(PragmaSourceFile) {
		return pragmas.FilterByName(PragmaSourceFile).First().Argument()
	}

	return location.ID()
}

func (b *Blockchain) systemChunkTransaction() (*flowgo.TransactionBody, error) {
	serviceAddress := b.GetChain().ServiceAddress()

	script := templates.ReplaceAddresses(
		systemChunkTransactionTemplate,
		templates.Environment{
			RandomBeaconHistoryAddress: serviceAddress.Hex(),
		},
	)

	// TODO: move this to `templates.Environment` struct
	script = strings.ReplaceAll(
		script,
		`import EVM from "EVM"`,
		fmt.Sprintf(
			"import EVM from %s",
			serviceAddress.HexWithPrefix(),
		),
	)

	txBuilder := flowgo.NewTransactionBodyBuilder().
		SetScript([]byte(script)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		AddAuthorizer(serviceAddress).
		SetPayer(serviceAddress).
		SetReferenceBlockID(b.pendingBlock.parentID)

	tx, err := txBuilder.Build()
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (b *Blockchain) executeSystemChunkTransaction() (*flowgo.TransactionBody, *IndexedTransactionResult, error) {
	txn, err := b.systemChunkTransaction()
	if err != nil {
		return nil, nil, err
	}
	ctx := fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithLogger(zerolog.Nop()),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithRandomSourceHistoryCallAllowed(true),
		fvm.WithBlockHeader(b.pendingBlock.Block().ToHeader()),
	)

	executionSnapshot, output, err := b.vm.Run(
		ctx,
		fvm.Transaction(txn, uint32(len(b.pendingBlock.Transactions()))),
		b.pendingBlock.ledgerState,
	)
	if err != nil {
		return nil, nil, err
	}

	if output.Err != nil {
		return nil, nil, output.Err
	}

	b.pendingBlock.events = append(b.pendingBlock.events, output.Events...)

	err = b.pendingBlock.ledgerState.Merge(executionSnapshot)
	if err != nil {
		return nil, nil, err
	}

	itr := &IndexedTransactionResult{
		ProcedureOutput: output,
		Index:           0,
	}
	return txn, itr, nil
}

func (b *Blockchain) executeScheduledTransactions(blockContext fvm.Context) (map[flowgo.Identifier]*flowgo.TransactionBody, map[flowgo.Identifier]IndexedTransactionResult, map[uint64]flowgo.Identifier, error) {
	systemTransactions := map[flowgo.Identifier]*flowgo.TransactionBody{}
	systemTransactionResults := map[flowgo.Identifier]IndexedTransactionResult{}
	scheduledTxIDMap := map[uint64]flowgo.Identifier{} // maps scheduled tx ID to transaction ID

	// disable checks for signatures and keys since we are executing a system transaction
	ctx := fvm.NewContextFromParent(
		blockContext,
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
	)

	// process scheduled transactions out-of-band (do not alter collections)
	processTx, err := blueprints.ProcessCallbacksTransaction(b.GetChain())
	if err != nil {
		return nil, nil, nil, err
	}

	// Use the real transaction ID from the process transaction
	processID := processTx.ID()
	systemTransactions[processID] = processTx

	executionSnapshot, output, err := b.vm.Run(
		ctx,
		fvm.Transaction(processTx, uint32(len(b.pendingBlock.Transactions()))),
		b.pendingBlock.ledgerState,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	systemTransactionResults[processID] = IndexedTransactionResult{
		ProcedureOutput: output,
		Index:           0,
	}

	// record events and state changes
	b.pendingBlock.events = append(b.pendingBlock.events, output.Events...)
	if err := b.pendingBlock.ledgerState.Merge(executionSnapshot); err != nil {
		return nil, nil, nil, err
	}

	executeTxs, err := blueprints.ExecuteCallbacksTransactions(b.GetChain(), output.Events)
	if err != nil {
		return nil, nil, nil, err
	}

	env := systemcontracts.SystemContractsForChain(b.GetChain().ChainID()).AsTemplateEnv()
	scheduledIDs, err := parseScheduledIDs(env, output.Events)
	if err != nil {
		return nil, nil, nil, err
	}

	// execute scheduled transactions out-of-band
	for idx, tx := range executeTxs {
		id := tx.ID()
		execSnapshot, execOutput, err := b.vm.Run(
			ctx,
			fvm.Transaction(tx, uint32(len(b.pendingBlock.Transactions()))),
			b.pendingBlock.ledgerState,
		)
		if err != nil {
			return nil, nil, nil, err
		}

		systemTransactionResults[id] = IndexedTransactionResult{
			ProcedureOutput: execOutput,
			Index:           0,
		}
		systemTransactions[id] = tx

		// Map the scheduled transaction ID (uint64) to the transaction ID (Identifier)
		if idx < len(scheduledIDs) {
			scheduledIDStr := scheduledIDs[idx]
			// Parse the scheduled ID as uint64
			var scheduledTxID uint64
			if _, err := fmt.Sscanf(scheduledIDStr, "%d", &scheduledTxID); err == nil {
				scheduledTxIDMap[scheduledTxID] = id
			}
		}

		// Print scheduled transaction result (labeled), including app-level scheduled tx id
		schedResult, err := convert.VMTransactionResultToEmulator(tx.ID(), execOutput)
		if err != nil {
			return nil, nil, nil, err
		}
		appScheduledID := ""
		if idx < len(scheduledIDs) {
			appScheduledID = scheduledIDs[idx]
		}
		utils.PrintScheduledTransactionResult(&b.conf.ServerLogger, schedResult, appScheduledID)

		b.pendingBlock.events = append(b.pendingBlock.events, execOutput.Events...)
		if err := b.pendingBlock.ledgerState.Merge(execSnapshot); err != nil {
			return nil, nil, nil, err
		}
	}

	return systemTransactions, systemTransactionResults, scheduledTxIDMap, nil
}

func (b *Blockchain) GetRegisterValues(registerIDs flowgo.RegisterIDs, height uint64) (values []flowgo.RegisterValue, err error) {
	ledger, err := b.storage.LedgerByHeight(context.Background(), height)
	if err != nil {
		return nil, err
	}
	for _, registerID := range registerIDs {
		value, err := ledger.Get(registerID)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// Helper (TODO: @jribbink delete later)
func Genesis(chainID flowgo.ChainID) *flowgo.Block {
	// create the headerBody
	headerBody := flowgo.HeaderBody{
		ChainID:   chainID,
		ParentID:  flowgo.ZeroID,
		Height:    0,
		Timestamp: uint64(flowgo.GenesisTime.UnixMilli()),
		View:      0,
	}

	// combine to block
	block := &flowgo.Block{
		HeaderBody: headerBody,
		Payload:    *flowgo.NewEmptyPayload(),
	}

	return block
}
