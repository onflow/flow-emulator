// Package emulator provides an emulated version of the Flow blockchain that can be used
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
	"github.com/onflow/flow-go-sdk"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
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

	"github.com/onflow/flow-emulator/convert"
	sdkconvert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/util"
	"github.com/onflow/flow-emulator/types"
)

// Blockchain emulates the functionality of the Flow blockchain.
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

type ServiceKey struct {
	Index          int
	Address        sdk.Address
	SequenceNumber uint64
	PrivateKey     sdkcrypto.PrivateKey
	PublicKey      sdkcrypto.PublicKey
	HashAlgo       sdkcrypto.HashAlgorithm
	SigAlgo        sdkcrypto.SignatureAlgorithm
	Weight         int
}

func (s ServiceKey) Signer() (sdkcrypto.Signer, error) {
	return sdkcrypto.NewInMemorySigner(s.PrivateKey, s.HashAlgo)
}

func (s ServiceKey) AccountKey() *sdk.AccountKey {

	var publicKey sdkcrypto.PublicKey
	if s.PublicKey != nil {
		publicKey = s.PublicKey
	}

	if s.PrivateKey != nil {
		publicKey = s.PrivateKey.PublicKey()
	}

	return &sdk.AccountKey{
		Index:          s.Index,
		PublicKey:      publicKey,
		SigAlgo:        s.SigAlgo,
		HashAlgo:       s.HashAlgo,
		Weight:         s.Weight,
		SequenceNumber: s.SequenceNumber,
	}
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

// config is a set of configuration options for an emulated blockchain.
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
	CoverageReportingEnabled     bool
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
	serviceKey.Address = sdk.Address(conf.GetChainID().Chain().ServiceAddress())
	serviceKey.Weight = sdk.AccountKeyWeightThreshold

	return serviceKey
}

const defaultGenesisTokenSupply = "1000000000.0"
const defaultScriptGasLimit = 100000
const defaultTransactionMaxGasLimit = flowgo.DefaultMaxTransactionGasLimit

// defaultConfig is the default configuration for an emulated blockchain.
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
		CoverageReportingEnabled:     false,
	}
}()

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

// WithCoverageReportingEnabled enables/disables Cadence code coverage reporting.
//
// If set to false, the emulator will not collect/expose coverage information for Cadence code.
//
// The default is false.
func WithCoverageReportingEnabled(enabled bool) Option {
	return func(c *config) {
		c.CoverageReportingEnabled = enabled
	}
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

	b.pendingBlock = newPendingBlock(latestBlock, latestLedger)
	b.transactionValidator = configureTransactionValidator(b.conf, blocks)

	return nil
}

// NewBlockchain instantiates a new emulated blockchain with the provided options.
func NewBlockchain(opts ...Option) (*Blockchain, error) {

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
	return b, nil

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

func (h CadenceHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
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
		Debugger:                 blockchain.debugger,
		AccountLinkingEnabled:    true,
		AttachmentsEnabled:       true,
		CoverageReportingEnabled: conf.CoverageReportingEnabled,
	}
	coverageReportedRuntime := &CoverageReportedRuntime{
		Runtime:        runtime.NewInterpreterRuntime(config),
		CoverageReport: runtime.NewCoverageReport(),
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
	genesisExecutionSnapshot, err := bootstrapLedger(
		vm,
		ctx,
		store.LedgerByHeight(context.Background(), 0),
		conf,
	)
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
	ledger := store.LedgerByHeight(context.Background(), 0)

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
	latestLedger := store.LedgerByHeight(
		context.Background(),
		latestBlock.Header.Height)

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

// ServiceKey returns the service private key for this blockchain.
func (b *Blockchain) ServiceKey() ServiceKey {
	serviceAccount, err := b.getAccount(sdkconvert.SDKAddressToFlow(b.serviceKey.Address))
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
		return nil, &StorageError{err}
	}

	return &block, nil
}

// GetBlockByID gets a block by ID.
func (b *Blockchain) GetBlockByID(id sdk.Identifier) (*flowgo.Block, error) {
	block, err := b.storage.BlockByID(context.Background(), sdkconvert.SDKIdentifierToFlow(id))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &BlockNotFoundByIDError{ID: id}
		}

		return nil, &StorageError{err}
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
			return nil, &BlockNotFoundByHeightError{Height: height}
		}
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) GetChain() flowgo.Chain {
	return b.vmCtx.Chain
}

func (b *Blockchain) GetCollection(colID sdk.Identifier) (*sdk.Collection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	col, err := b.storage.CollectionByID(context.Background(), sdkconvert.SDKIdentifierToFlow(colID))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &CollectionNotFoundError{ID: colID}
		}
		return nil, &StorageError{err}
	}

	sdkCol := sdkconvert.FlowLightCollectionToSDK(col)

	return &sdkCol, nil
}

// GetTransaction gets an existing transaction by ID.
//
// The function first looks in the pending block, then the current blockchain state.
func (b *Blockchain) GetTransaction(id sdk.Identifier) (*sdk.Transaction, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	txID := sdkconvert.SDKIdentifierToFlow(id)

	pendingTx := b.pendingBlock.GetTransaction(txID)
	if pendingTx != nil {
		pendingSDKTx := sdkconvert.FlowTransactionToSDK(*pendingTx)
		return &pendingSDKTx, nil
	}

	tx, err := b.storage.TransactionByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &TransactionNotFoundError{ID: txID}
		}
		return nil, &StorageError{err}
	}

	sdkTx := sdkconvert.FlowTransactionToSDK(tx)
	return &sdkTx, nil
}

func (b *Blockchain) GetTransactionResult(ID sdk.Identifier) (*sdk.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	txID := sdkconvert.SDKIdentifierToFlow(ID)

	if b.pendingBlock.ContainsTransaction(txID) {
		return &sdk.TransactionResult{
			Status: sdk.TransactionStatusPending,
		}, nil
	}

	storedResult, err := b.storage.TransactionResultByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return &sdk.TransactionResult{
				Status: sdk.TransactionStatusUnknown,
			}, nil
		}
		return nil, &StorageError{err}
	}

	var errResult error

	if storedResult.ErrorCode != 0 {
		errResult = &ExecutionError{
			Code:    storedResult.ErrorCode,
			Message: storedResult.ErrorMessage,
		}
	}

	sdkEvents, err := sdkconvert.FlowEventsToSDK(storedResult.Events)
	if err != nil {
		return nil, err
	}

	result := sdk.TransactionResult{
		Status: sdk.TransactionStatusSealed,
		Error:  errResult,
		Events: sdkEvents,
	}

	return &result, nil
}

// GetAccountByIndex returns the account for the given address.
func (b *Blockchain) GetAccountByIndex(index uint) (*sdk.Account, error) {

	generator := flow.NewAddressGenerator(sdk.ChainID(b.vmCtx.Chain.ChainID()))

	generator.SetIndex(index)

	flowAddress := sdkconvert.SDKAddressToFlow(generator.Address())

	account, err := b.getAccount(flowAddress)
	if err != nil {
		return nil, err
	}

	sdkAccount, err := sdkconvert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}

	return &sdkAccount, nil
}

// Deprecated: Needed for the debugger right now, do NOT use for other purposes.
// TODO: refactor
func (b *Blockchain) GetAccountUnsafe(address sdk.Address) (*sdk.Account, error) {

	flowAddress := sdkconvert.SDKAddressToFlow(address)

	account, err := b.getAccount(flowAddress)
	if err != nil {
		return nil, err
	}

	sdkAccount, err := sdkconvert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}

	return &sdkAccount, nil
}

// GetAccount returns the account for the given address.
func (b *Blockchain) GetAccount(address sdk.Address) (*sdk.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	flowAddress := sdkconvert.SDKAddressToFlow(address)

	account, err := b.getAccount(flowAddress)
	if err != nil {
		return nil, err
	}

	sdkAccount, err := sdkconvert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}

	return &sdkAccount, nil
}

// getAccount returns the account for the given address.
func (b *Blockchain) getAccount(address flowgo.Address) (*flowgo.Account, error) {
	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Header.Height)
}

// GetAccountAtBlock returns the account for the given address at specified block height.
func (b *Blockchain) GetAccountAtBlock(address sdk.Address, blockHeight uint64) (*sdk.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	flowAddress := sdkconvert.SDKAddressToFlow(address)

	account, err := b.getAccountAtBlock(flowAddress, blockHeight)
	if err != nil {
		return nil, err
	}

	sdkAccount, err := sdkconvert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}

	return &sdkAccount, nil
}

// GetAccountAtBlock returns the account for the given address at specified block height.
func (b *Blockchain) getAccountAtBlock(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {

	account, err := b.vm.GetAccount(
		b.vmCtx,
		address,
		b.storage.LedgerByHeight(context.Background(), blockHeight),
	)

	if fvmerrors.IsAccountNotFoundError(err) {
		return nil, &AccountNotFoundError{Address: address}
	}

	return account, nil
}

// GetEventsByHeight returns the events in the block at the given height, optionally filtered by type.
func (b *Blockchain) GetEventsByHeight(blockHeight uint64, eventType string) ([]sdk.Event, error) {
	flowEvents, err := b.storage.EventsByHeight(context.Background(), blockHeight, eventType)
	if err != nil {
		return nil, err
	}

	sdkEvents, err := sdkconvert.FlowEventsToSDK(flowEvents)
	if err != nil {
		return nil, fmt.Errorf("could not convert events: %w", err)
	}

	return sdkEvents, err
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) AddTransaction(tx sdk.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.addTransaction(tx)
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) addTransaction(sdkTx sdk.Transaction) error {

	tx := sdkconvert.SDKTransactionToFlow(sdkTx)

	// If index > 0, pending block has begun execution (cannot add more transactions)
	if b.pendingBlock.ExecutionStarted() {
		return &PendingBlockMidExecutionError{BlockID: b.pendingBlock.ID()}
	}

	if b.pendingBlock.ContainsTransaction(tx.ID()) {
		return &DuplicateTransactionError{TxID: tx.ID()}
	}

	_, err := b.storage.TransactionByID(context.Background(), tx.ID())
	if err == nil {
		// Found the transaction, this is a duplicate
		return &DuplicateTransactionError{TxID: tx.ID()}
	} else if !errors.Is(err, storage.ErrNotFound) {
		// Error in the storage provider
		return fmt.Errorf("failed to check storage for transaction %w", err)
	}

	err = b.transactionValidator.Validate(tx)
	if err != nil {
		return convertAccessError(err)
	}

	// add transaction to pending block
	b.pendingBlock.AddTransaction(*tx)

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
		return results, &PendingBlockTransactionsExhaustedError{
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
		return nil, &PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.ID(),
		}
	}

	txnBody := b.pendingBlock.NextTransaction()
	txnId := txnBody.ID()

	b.currentCode = string(txnBody.Script)
	b.currentScriptID = txnId.String()
	if b.debugger != nil {
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
		return nil, &PendingBlockCommitBeforeExecutionError{BlockID: b.pendingBlock.ID()}
	}

	// pending block cannot be committed before execution completes
	if b.pendingBlock.ExecutionStarted() && !b.pendingBlock.ExecutionComplete() {
		return nil, &PendingBlockMidExecutionError{BlockID: b.pendingBlock.ID()}
	}

	block := b.pendingBlock.Block()
	collections := b.pendingBlock.Collections()
	transactions := b.pendingBlock.Transactions()
	transactionResults, err := convertToSealedResults(b.pendingBlock.TransactionResults())
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

	ledger := b.storage.LedgerByHeight(
		context.Background(),
		block.Header.Height)

	// reset pending block using current block and ledger state
	b.pendingBlock = newPendingBlock(block, ledger)

	return block, nil
}

func (b *Blockchain) GetAccountStorage(
	address sdk.Address,
) (
	*AccountStorage,
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

	extractStorage := func(path common.PathDomain) StorageItem {
		storageMap := store.GetStorageMap(
			common.MustBytesToAddress(address.Bytes()),
			path.Identifier(),
			false)
		if storageMap == nil {
			return nil
		}

		iterator := storageMap.Iterator(nil)
		values := make(StorageItem)

		for k, v := iterator.Next(); v != nil; k, v = iterator.Next() {
			exportedValue, err := runtime.ExportValue(v, inter, interpreter.EmptyLocationRange)
			if err != nil {
				// just skip errored value
				continue
			}
			values[k] = exportedValue
		}
		return values
	}

	return NewAccountStorage(
		account,
		address,
		extractStorage(common.PathDomainPrivate),
		extractStorage(common.PathDomainPublic),
		extractStorage(common.PathDomainStorage),
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

	return block, results, nil
}

// ResetPendingBlock clears the transactions in pending block.
func (b *Blockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	latestBlock, err := b.storage.LatestBlock(context.Background())
	if err != nil {
		return &StorageError{err}
	}

	latestLedger := b.storage.LedgerByHeight(
		context.Background(),
		latestBlock.Header.Height)

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

	return b.ExecuteScriptAtBlock(script, arguments, latestBlock.Header.Height)
}

func (b *Blockchain) ExecuteScriptAtBlock(
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

	requestedLedgerSnapshot := b.storage.LedgerByHeight(
		context.Background(),
		requestedBlock.Header.Height)

	header := requestedBlock.Header
	blockContext := b.newFVMContextFromHeader(header)

	scriptProc := fvm.Script(script).WithArguments(arguments...)
	b.currentCode = string(script)
	b.currentScriptID = scriptProc.ID.String()
	if b.debugger != nil {
		b.debugger.RequestPause()
	}
	_, output, err := b.vm.Run(
		blockContext,
		scriptProc,
		requestedLedgerSnapshot)
	if err != nil {
		return nil, err
	}

	scriptID := sdk.Identifier(flowgo.MakeIDFromFingerPrint(script))

	events, err := sdkconvert.FlowEventsToSDK(output.Events)
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
	}, nil
}

// CreateAccount submits a transaction to create a new account with the given
// account keys and contracts. The transaction is paid by the service account.
func (b *Blockchain) CreateAccount(publicKeys []*sdk.AccountKey, contracts []templates.Contract) (sdk.Address, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	serviceKey := b.ServiceKey()
	serviceAddress := serviceKey.Address

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return sdk.Address{}, err
	}

	tx, err := templates.CreateAccount(publicKeys, contracts, serviceAddress)
	if err != nil {
		return sdk.Address{}, err
	}

	tx.SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(sdk.Identifier(latestBlock.ID())).
		SetProposalKey(serviceAddress, serviceKey.Index, serviceKey.SequenceNumber).
		SetPayer(serviceAddress)

	signer, err := serviceKey.Signer()
	if err != nil {
		return sdk.Address{}, err
	}

	err = tx.SignEnvelope(serviceAddress, serviceKey.Index, signer)
	if err != nil {
		return sdk.Address{}, err
	}

	err = b.addTransaction(*tx)
	if err != nil {
		return sdk.Address{}, err
	}

	_, results, err := b.executeAndCommitBlock()
	if err != nil {
		return sdk.Address{}, err
	}

	lastResult := results[len(results)-1]

	_, err = b.commitBlock()
	if err != nil {
		return sdk.Address{}, err
	}

	if !lastResult.Succeeded() {
		return sdk.Address{}, lastResult.Error
	}

	var address sdk.Address

	for _, event := range lastResult.Events {
		if event.Type == sdk.EventAccountCreated {
			address = sdk.Address(event.Value.Fields[0].(cadence.Address))
			break
		}
	}

	if address == (sdk.Address{}) {
		return sdk.Address{}, fmt.Errorf("failed to find AccountCreated event")
	}

	return address, nil
}

func convertToSealedResults(
	results map[flowgo.Identifier]IndexedTransactionResult,
) (map[flowgo.Identifier]*types.StorableTransactionResult, error) {

	output := make(map[flowgo.Identifier]*types.StorableTransactionResult)

	for id, result := range results {
		temp, err := convert.ToStorableResult(result.ProcedureOutput)
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

func (b *Blockchain) SetDebugger(debugger *interpreter.Debugger) {
	b.debugger = debugger
}

func (b *Blockchain) EndDebugging() {
	b.SetDebugger(nil)
}

func (b *Blockchain) CoverageReport() *runtime.CoverageReport {
	return b.coverageReportedRuntime.CoverageReport
}

func (b *Blockchain) SetCoverageReport(coverageReport *runtime.CoverageReport) {
	b.coverageReportedRuntime.CoverageReport = coverageReport
}

func (b *Blockchain) ResetCoverageReport() {
	b.coverageReportedRuntime.Reset()
}
