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
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/util"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils"
	flowsdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm"
	fvmcrypto "github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

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
		serviceKey:             conf.GetServiceKey(),
		debugger:               nil,
		activeDebuggingSession: false,
		conf:                   conf,
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

// Contracts allows users to deploy the given contracts.
// Some default common contracts are pre-configured in the `CommonContracts`
// global variable. It includes contracts such as:
// NonFungibleToken, FUSD, MetadataViews, NFTStorefront, NFTStorefrontV2, ExampleNFT
// The default value is []ContractDescription{}.
func Contracts(contracts []ContractDescription) Option {
	return func(c *config) {
		c.Contracts = contracts
	}
}

// Blockchain emulates the functionality of the Flow emulator.
type Blockchain struct {
	// committed chain state: blocks, transactions, registers, events
	storage storage.Store

	// mutex protecting pending block
	mu sync.RWMutex

	// pending block containing block info, register state, pending transactions
	pendingBlock *pendingBlock

	// used to execute transactions and scripts
	vm    *fvm.VirtualMachine
	vmCtx fvm.Context

	transactionValidator *access.TransactionValidator

	serviceKey ServiceKey

	debugger               *interpreter.Debugger
	activeDebuggingSession bool
	currentCode            string
	currentScriptID        string

	conf config

	coverageReportedRuntime *CoverageReportedRuntime
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
	}
}()

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

	b.pendingBlock = newPendingBlock(latestBlock, latestLedger)
	b.transactionValidator = configureTransactionValidator(b.conf, blocks)

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

func (b *Blockchain) GetChain() flowgo.Chain {
	return b.vmCtx.Chain
}

func (b *Blockchain) GetNetworkParameters() access.NetworkParameters {
	return access.NetworkParameters{
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

func configureFVM(blockchain *Blockchain, conf config, blocks *blocks) (*fvm.VirtualMachine, fvm.Context, error) {
	vm := fvm.NewVirtualMachine()

	cadenceLogger := conf.Logger.Hook(CadenceHook{MainLogger: &conf.ServerLogger}).Level(zerolog.DebugLevel)

	config := runtime.Config{
		Debugger:                     blockchain.debugger,
		AccountLinkingEnabled:        true,
		AttachmentsEnabled:           true,
		CapabilityControllersEnabled: true,
		CoverageReport:               conf.CoverageReport,
	}
	coverageReportedRuntime := &CoverageReportedRuntime{
		Runtime:        runtime.NewInterpreterRuntime(config),
		CoverageReport: conf.CoverageReport,
		Environment:    runtime.NewBaseInterpreterEnvironment(config),
	}
	customRuntimePool := reusableRuntime.NewCustomReusableCadenceRuntimePool(
		1,
		config,
		func(config runtime.Config) runtime.Runtime {
			return coverageReportedRuntime
		},
	)

	fvmOptions := []fvm.Option{
		fvm.WithLogger(cadenceLogger),
		fvm.WithChain(conf.GetChainID().Chain()),
		fvm.WithBlocks(blocks),
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(!conf.ContractRemovalEnabled),
		fvm.WithGasLimit(conf.ScriptGasLimit),
		fvm.WithCadenceLogging(true),
		fvm.WithAccountStorageLimit(conf.StorageLimitEnabled),
		fvm.WithTransactionFeesEnabled(conf.TransactionFeesEnabled),
		fvm.WithReusableCadenceRuntimePool(customRuntimePool),
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

	blockchain.coverageReportedRuntime = coverageReportedRuntime

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

	genesisExecutionSnapshot, err := bootstrapLedger(vm, ctx, ledger, conf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to bootstrap execution state: %w", err)
	}

	// commit the genesis block to storage
	genesis := flowgo.Genesis(conf.GetChainID())

	err = store.CommitBlock(
		context.Background(),
		*genesis,
		nil,
		nil,
		nil,
		genesisExecutionSnapshot,
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
		latestBlock.Header.Height,
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

	executionSnapshot, _, err := vm.Run(ctx, bootstrap, ledger)
	if err != nil {
		return nil, err
	}

	return executionSnapshot, nil
}

func configureBootstrapProcedure(conf config, flowAccountKey flowgo.AccountPublicKey, supply cadence.UFix64) *fvm.BootstrapProcedure {
	options := make([]fvm.BootstrapProcedureOption, 0)
	options = append(options,
		fvm.WithInitialTokenSupply(supply),
		fvm.WithRestrictedAccountCreationEnabled(false),
	)
	if conf.StorageLimitEnabled {
		options = append(options,
			fvm.WithAccountCreationFee(conf.MinimumStorageReservation),
			fvm.WithMinimumStorageReservation(conf.MinimumStorageReservation),
			fvm.WithStorageMBPerFLOW(conf.StorageMBPerFLOW),
		)
	}
	if conf.TransactionFeesEnabled {
		// This enables variable transaction fees AND execution effort metering
		// as described in Variable Transaction Fees: Execution Effort FLIP: https://github.com/onflow/flow/pull/753)
		// TODO: In the future this should be an injectable parameter. For now this is hard coded
		// as this is the first iteration of variable execution fees.
		options = append(options,
			fvm.WithTransactionFee(fvm.BootstrapProcedureFeeParameters{
				SurgeFactor:         cadence.UFix64(100_000_000), // 1.0
				InclusionEffortCost: cadence.UFix64(100),         // 1E-6
				ExecutionEffortCost: cadence.UFix64(499_000_000), // 4.99
			}),
			fvm.WithExecutionEffortWeights(map[common.ComputationKind]uint64{
				common.ComputationKindStatement:          1569,
				common.ComputationKindLoop:               1569,
				common.ComputationKindFunctionInvocation: 1569,
				environment.ComputationKindGetValue:      808,
				environment.ComputationKindCreateAccount: 2837670,
				environment.ComputationKindSetValue:      765,
			}),
		)
	}
	return fvm.Bootstrap(
		flowAccountKey,
		options...,
	)
}

func configureTransactionValidator(conf config, blocks *blocks) *access.TransactionValidator {
	return access.NewTransactionValidator(
		blocks,
		conf.GetChainID().Chain(),
		access.TransactionValidationOptions{
			Expiry:                       conf.TransactionExpiry,
			ExpiryBuffer:                 0,
			AllowEmptyReferenceBlockID:   conf.TransactionExpiry == 0,
			AllowUnknownReferenceBlockID: false,
			MaxGasLimit:                  conf.TransactionMaxGasLimit,
			CheckScriptsParse:            true,
			MaxTransactionByteSize:       flowgo.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flowgo.DefaultMaxCollectionByteSize,
		},
	)
}

func (b *Blockchain) newFVMContextFromHeader(header *flowgo.Header) fvm.Context {
	return fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithBlockHeader(header),
	)
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
	return b.pendingBlock.Block().Header.Timestamp
}

// GetLatestBlock gets the latest sealed block.
func (b *Blockchain) GetLatestBlock() (*flowgo.Block, error) {
	block, err := b.storage.LatestBlock(context.Background())
	if err != nil {
		return nil, err
	}

	return &block, nil
}

// GetBlockByID gets a block by ID.
func (b *Blockchain) GetBlockByID(id flowgo.Identifier) (*flowgo.Block, error) {
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

	col, err := b.storage.CollectionByID(context.Background(), colID)
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

func (b *Blockchain) GetTransactionResult(txID flowgo.Identifier) (*access.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.pendingBlock.ContainsTransaction(txID) {
		return &access.TransactionResult{
			Status: flowgo.TransactionStatusPending,
		}, nil
	}

	storedResult, err := b.storage.TransactionResultByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return &access.TransactionResult{
				Status: flowgo.TransactionStatusUnknown,
			}, nil
		}
		return nil, err
	}

	result := access.TransactionResult{
		Status:        flowgo.TransactionStatusSealed,
		StatusCode:    uint(storedResult.ErrorCode),
		ErrorMessage:  storedResult.ErrorMessage,
		Events:        storedResult.Events,
		TransactionID: txID,
		BlockHeight:   storedResult.BlockHeight,
		BlockID:       storedResult.BlockID,
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
	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Header.Height)
}

// GetAccount returns the account for the given address.
func (b *Blockchain) GetAccount(address flowgo.Address) (*flowgo.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getAccount(address)
}

// getAccount returns the account for the given address.
func (b *Blockchain) getAccount(address flowgo.Address) (*flowgo.Account, error) {
	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Header.Height)
}

// GetAccountAtBlockHeight  returns the account for the given address at specified block height.
func (b *Blockchain) GetAccountAtBlockHeight(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	account, err := b.getAccountAtBlock(address, blockHeight)
	if err != nil {
		return nil, err
	}

	return account, nil
}

// GetAccountAtBlock returns the account for the given address at specified block height.
func (b *Blockchain) getAccountAtBlock(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {
	ledger, err := b.storage.LedgerByHeight(context.Background(), blockHeight)
	if err != nil {
		return nil, err
	}

	account, err := b.vm.GetAccount(b.vmCtx, address, ledger)
	if fvmerrors.IsAccountNotFoundError(err) {
		return nil, &types.AccountNotFoundError{Address: address}
	}

	return account, nil
}

func (b *Blockchain) GetEventsForBlockIDs(eventType string, blockIDs []flowgo.Identifier) (result []flowgo.BlockEvents, err error) {
	for _, blockID := range blockIDs {
		block, err := b.storage.BlockByID(context.Background(), blockID)
		if err != nil {
			break
		}
		events, err := b.storage.EventsByHeight(context.Background(), block.Header.Height, eventType)
		if err != nil {
			break
		}
		result = append(result, flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Header.Height,
			BlockTimestamp: block.Header.Timestamp,
			Events:         events,
		})
	}

	return result, err
}

func (b *Blockchain) GetEventsForHeightRange(eventType string, startHeight, endHeight uint64) (result []flowgo.BlockEvents, err error) {

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
			BlockHeight:    block.Header.Height,
			BlockTimestamp: block.Header.Timestamp,
			Events:         events,
		})
	}

	return result, err
}

// GetEventsByHeight returns the events in the block at the given height, optionally filtered by type.
func (b *Blockchain) GetEventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error) {
	return b.storage.EventsByHeight(context.Background(), blockHeight, eventType)
}

// SendTransaction submits a transaction to the network.
func (b *Blockchain) SendTransaction(flowTx *flowgo.TransactionBody) error {
	err := b.AddTransaction(*flowTx)
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

	err = b.transactionValidator.Validate(&tx)
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

	// empty blocks do not require execution, treat as a no-op
	if b.pendingBlock.Empty() {
		return results, nil
	}

	header := b.pendingBlock.Block().Header
	blockContext := b.newFVMContextFromHeader(header)

	// cannot execute a block that has already executed
	if b.pendingBlock.ExecutionComplete() {
		return results, &types.PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.ID(),
		}
	}

	// continue executing transactions until execution is complete
	for !b.pendingBlock.ExecutionComplete() {
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

	header := b.pendingBlock.Block().Header
	blockContext := b.newFVMContextFromHeader(header)
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
	txnId := txnBody.ID()

	b.currentCode = string(txnBody.Script)
	b.currentScriptID = txnId.String()
	if b.activeDebuggingSession {
		b.debugger.RequestPause()
	}

	// use the computer to execute the next transaction
	output, err := b.pendingBlock.ExecuteNextTransaction(b.vm, ctx)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	tr, err := convert.VMTransactionResultToEmulator(txnId, output)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	// if transaction error exist try to further debug what was the problem
	if tr.Error != nil {
		tr.Debug = b.debugSignatureError(tr.Error, txnBody)
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
	transactions := b.pendingBlock.Transactions()
	transactionResults, err := convertToSealedResults(b.pendingBlock.TransactionResults(), b.pendingBlock.ID(), b.pendingBlock.height)
	if err != nil {
		return nil, err
	}
	executionSnapshot := b.pendingBlock.Finalize()
	events := b.pendingBlock.Events()

	// commit the pending block to storage
	err = b.storage.CommitBlock(
		context.Background(),
		*block,
		collections,
		transactions,
		transactionResults,
		executionSnapshot,
		events)
	if err != nil {
		return nil, err
	}

	ledger, err := b.storage.LedgerByHeight(
		context.Background(),
		block.Header.Height,
	)
	if err != nil {
		return nil, err
	}

	// reset pending block using current block and ledger state
	b.pendingBlock = newPendingBlock(block, ledger)

	return block, nil
}

func (b *Blockchain) GetAccountStorage(
	address flowgo.Address,
) (
	*types.AccountStorage,
	error,
) {
	view := b.pendingBlock.ledgerState.NewChild()

	env := environment.NewScriptEnvironmentFromStorageSnapshot(
		b.vmCtx.EnvironmentParams,
		view)

	r := b.vmCtx.Borrow(env)
	defer b.vmCtx.Return(r)
	ctx := runtime.Context{
		Interface: env,
	}

	store, inter, err := r.Storage(ctx)
	if err != nil {
		return nil, err
	}

	account, err := b.vm.GetAccount(
		b.vmCtx,
		flowgo.BytesToAddress(address.Bytes()),
		view)
	if err != nil {
		return nil, err
	}

	extractStorage := func(path common.PathDomain) (types.StorageItem, error) {
		storageMap := store.GetStorageMap(
			common.MustBytesToAddress(address.Bytes()),
			path.Identifier(),
			false,
		)
		if storageMap == nil {
			return nil, nil
		}

		iterator := storageMap.Iterator(nil)
		values := make(types.StorageItem)

		for k, v := iterator.Next(); v != nil; k, v = iterator.Next() {
			exportedValue, err := runtime.ExportValue(v, inter, interpreter.EmptyLocationRange)
			if err != nil {
				// just skip errored value
				continue
			}

			if k, ok := k.(interpreter.StringAtreeValue); ok {
				values[string(k)] = exportedValue
			}
		}
		return values, nil
	}

	privateStorageItems, err := extractStorage(common.PathDomainPrivate)
	if err != nil {
		return nil, err
	}

	publicStorageItems, err := extractStorage(common.PathDomainPublic)
	if err != nil {
		return nil, err
	}

	storageStorageItems, err := extractStorage(common.PathDomainStorage)
	if err != nil {
		return nil, err
	}

	return types.NewAccountStorage(
		account,
		flowsdk.Address(address),
		privateStorageItems,
		publicStorageItems,
		storageStorageItems,
	)
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
	b.conf.ServerLogger.Info().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msgf("📦 Block #%d committed", block.Header.Height)

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
		latestBlock.Header.Height,
	)
	if err != nil {
		return err
	}

	// reset pending block using latest committed block and ledger state
	b.pendingBlock = newPendingBlock(&latestBlock, latestLedger)

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *Blockchain) ExecuteScript(
	script []byte,
	arguments [][]byte,
) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	return b.ExecuteScriptAtBlockHeight(script, arguments, latestBlock.Header.Height)
}

func (b *Blockchain) ExecuteScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	requestedBlock, err := b.storage.BlockByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	requestedLedgerSnapshot, err := b.storage.LedgerByHeight(
		context.Background(),
		requestedBlock.Header.Height,
	)
	if err != nil {
		return nil, err
	}

	header := requestedBlock.Header
	blockContext := b.newFVMContextFromHeader(header)

	scriptProc := fvm.Script(script).WithArguments(arguments...)
	b.currentCode = string(script)
	b.currentScriptID = scriptProc.ID.String()
	if b.activeDebuggingSession {
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

	return &types.ScriptResult{
		ScriptID:        scriptID,
		Value:           convertedValue,
		Error:           scriptError,
		Logs:            output.Logs,
		Events:          events,
		ComputationUsed: output.ComputationUsed,
		MemoryEstimate:  output.MemoryEstimate,
	}, nil
}

func (b *Blockchain) ExecuteScriptAtBlockHeight(
	script []byte,
	arguments [][]byte,
	blockHeight uint64,
) (*types.ScriptResult, error) {
	b.mu.RLock()
	requestedBlock, err := b.getBlockByHeight(blockHeight)
	b.mu.RUnlock()

	if err != nil {
		return nil, err
	}
	return b.ExecuteScriptAtBlockID(script, arguments, requestedBlock.Header.ID())
}

func convertToSealedResults(
	results map[flowgo.Identifier]IndexedTransactionResult,
	blockID flowgo.Identifier,
	blockHeight uint64,
) (map[flowgo.Identifier]*types.StorableTransactionResult, error) {

	output := make(map[flowgo.Identifier]*types.StorableTransactionResult)

	for id, result := range results {
		temp, err := convert.ToStorableResult(result.ProcedureOutput, blockID, blockHeight)
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
	return b.coverageReportedRuntime.CoverageReport
}

func (b *Blockchain) ResetCoverageReport() {
	b.coverageReportedRuntime.Reset()
}

func (b *Blockchain) GetTransactionsByBlockID(blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	//TODO implement me
	panic("implement me")
}

func (b *Blockchain) GetTransactionResultsByBlockID(blockID flowgo.Identifier) ([]*access.TransactionResult, error) {
	//TODO implement me
	panic("implement me")
}
