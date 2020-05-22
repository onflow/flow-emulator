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
	"errors"
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	model "github.com/dapperlabs/flow-go/model/flow"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	sdk "github.com/onflow/flow-go-sdk"
	sdkCrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go-sdk/templates"

	"github.com/dapperlabs/flow-emulator/convert"
	sdkConvert "github.com/dapperlabs/flow-emulator/convert/sdk"
	"github.com/dapperlabs/flow-emulator/storage"
	"github.com/dapperlabs/flow-emulator/storage/memstore"
	"github.com/dapperlabs/flow-emulator/types"
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
	virtualMachine virtualmachine.VirtualMachine

	rootKey RootKey
}

// BlockchainAPI defines the method set of an emulated blockchain.
type BlockchainAPI interface {
	AddTransaction(tx sdk.Transaction) error
	ExecuteNextTransaction() (*types.TransactionResult, error)
	ExecuteBlock() ([]*types.TransactionResult, error)
	CommitBlock() (*sdk.Block, error)
	ExecuteAndCommitBlock() (*sdk.Block, []*types.TransactionResult, error)
	GetLatestBlock() (*sdk.Block, error)
	GetBlockByID(id sdk.Identifier) (*sdk.Block, error)
	GetBlockByHeight(height uint64) (*sdk.Block, error)
	GetCollection(colID sdk.Identifier) (*sdk.Collection, error)
	GetTransaction(txID sdk.Identifier) (*sdk.Transaction, error)
	GetTransactionResult(txID sdk.Identifier) (*sdk.TransactionResult, error)
	GetAccount(address sdk.Address) (*sdk.Account, error)
	GetAccountAtBlock(address sdk.Address, blockHeight uint64) (*sdk.Account, error)
	GetEventsByHeight(blockHeight uint64, eventType string) ([]sdk.Event, error)
	ExecuteScript(script []byte) (*types.ScriptResult, error)
	ExecuteScriptAtBlock(script []byte, blockHeight uint64) (*types.ScriptResult, error)
	RootAccountAddress() sdk.Address
	RootKey() RootKey
}

var _ BlockchainAPI = &Blockchain{}

type RootKey struct {
	ID             int
	Address        sdk.Address
	SequenceNumber uint64
	PrivateKey     *sdkCrypto.PrivateKey
	PublicKey      *sdkCrypto.PublicKey
	HashAlgo       sdkCrypto.HashAlgorithm
	SignAlgo       sdkCrypto.SignatureAlgorithm
	Weight         int
}

func (r RootKey) Signer() sdkCrypto.Signer {
	return sdkCrypto.NewInMemorySigner(*r.PrivateKey, r.HashAlgo)
}

func (r RootKey) AccountKey() *sdk.AccountKey {

	var publicKey sdkCrypto.PublicKey
	if r.PublicKey != nil {
		publicKey = *r.PublicKey
	}

	if r.PrivateKey != nil {
		publicKey = r.PrivateKey.PublicKey()
	}

	return &sdk.AccountKey{
		ID:             r.ID,
		PublicKey:      publicKey,
		SigAlgo:        r.SignAlgo,
		HashAlgo:       r.HashAlgo,
		Weight:         r.Weight,
		SequenceNumber: r.SequenceNumber,
	}
}

// MaxGasLimit is the maximum gas limit supported by the emulated blockchain.
//
// TODO: replace with safe limit
const MaxGasLimit = 999999999

// config is a set of configuration options for an emulated blockchain.
type config struct {
	RootKey RootKey
	Store   storage.Store
}

// defaultConfig is the default configuration for an emulated blockchain.
// NOTE: Instantiated in init function
var defaultConfig config

// Option is a function applying a change to the emulator config.
type Option func(*config)

// WithRootPublicKey sets the root key from a public key.
func WithRootPublicKey(
	rootPublicKey sdkCrypto.PublicKey,
	sigAlgo sdkCrypto.SignatureAlgorithm,
	hashAlgo sdkCrypto.HashAlgorithm,
) Option {
	return func(c *config) {
		c.RootKey = RootKey{
			PublicKey: &rootPublicKey,
			SignAlgo:  sigAlgo,
			HashAlgo:  hashAlgo,
		}
	}
}

// WithStore sets the persistent storage provider.
func WithStore(store storage.Store) Option {
	return func(c *config) {
		c.Store = store
	}
}

// NewBlockchain instantiates a new emulated blockchain with the provided options.
func NewBlockchain(opts ...Option) (*Blockchain, error) {
	var pendingBlock *pendingBlock

	// apply options to the default config
	config := defaultConfig
	for _, opt := range opts {
		opt(&config)
	}

	// if no store is specified, use a memstore
	// NOTE: we don't initialize this in defaultConfig because otherwise the same
	// memstore is shared between Blockchain instances
	if config.Store == nil {
		config.Store = memstore.New()
	}
	store := config.Store

	// set up root key
	rootKey := config.RootKey
	rootKey.Address = sdk.RootAddress
	rootKey.Weight = sdk.AccountKeyWeightThreshold

	latestBlock, err := store.LatestBlock()
	if err == nil && latestBlock.Header.Height > 0 {
		// storage contains data, load state from storage
		latestLedgerView := store.LedgerViewByHeight(latestBlock.Header.Height)

		// restore pending block header from store information
		pendingBlock = newPendingBlock(&latestBlock, latestLedgerView)
	} else if err != nil && !errors.Is(err, storage.ErrNotFound) {
		// internal storage error, fail fast
		return nil, err
	} else {
		genesisLedgerView := store.LedgerViewByHeight(0)

		// storage is empty, create the root account
		_, err := createAccount(genesisLedgerView, rootKey.AccountKey())
		if err != nil {
			return nil, fmt.Errorf("error while creating root account: %w", err)
		}

		// commit the genesis block to storage
		genesis := model.Genesis(nil)

		err = store.CommitBlock(
			*genesis,
			nil,
			nil,
			nil,
			genesisLedgerView.Delta(),
			nil,
		)
		if err != nil {
			return nil, err
		}

		// get empty ledger view
		ledgerView := store.LedgerViewByHeight(0)

		// create pending block from genesis
		pendingBlock = newPendingBlock(genesis, ledgerView)
	}

	b := &Blockchain{
		storage:      config.Store,
		pendingBlock: pendingBlock,
		rootKey:      rootKey,
	}

	interpreterRuntime := runtime.NewInterpreterRuntime()

	b.virtualMachine, err = virtualmachine.New(interpreterRuntime)

	if err != nil {
		return nil, fmt.Errorf("cannot create virual machine: %w", err)
	}

	return b, nil
}

// RootAccountAddress returns the root account address for this blockchain.
func (b *Blockchain) RootAccountAddress() sdk.Address {
	return b.rootKey.Address
}

// RootKey returns the root private key for this blockchain.
func (b *Blockchain) RootKey() RootKey {
	rootAccount, err := b.getAccount(sdkConvert.SDKAddressToFlow(b.rootKey.Address))
	if err != nil {
		return b.rootKey
	}

	if len(rootAccount.Keys) > 0 {
		b.rootKey.ID = 0
		b.rootKey.SequenceNumber = rootAccount.Keys[0].SeqNumber
		b.rootKey.Weight = rootAccount.Keys[0].Weight
	}

	return b.rootKey
}

// PendingBlockID returns the ID of the pending block.
func (b *Blockchain) PendingBlockID() model.Identifier {
	return b.pendingBlock.ID()
}

func (b *Blockchain) getLatestBlock() (model.Block, error) {
	block, err := b.storage.LatestBlock()
	if err != nil {
		return model.Block{}, &StorageError{err}
	}

	return block, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *Blockchain) GetLatestBlock() (*sdk.Block, error) {
	block, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}

	sdkBlock := sdkConvert.FlowBlockToSDK(block)

	return &sdkBlock, nil
}

// GetBlockByID gets a block by ID.
func (b *Blockchain) GetBlockByID(id sdk.Identifier) (*sdk.Block, error) {
	block, err := b.storage.BlockByID(sdkConvert.SDKIdentifierToFlow(id))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &BlockNotFoundByIDError{ID: id}
		}
		return nil, &StorageError{err}
	}

	sdkBlock := sdkConvert.FlowBlockToSDK(block)

	return &sdkBlock, nil
}

// GetBlockByHeight gets a block by height.
func (b *Blockchain) GetBlockByHeight(height uint64) (*sdk.Block, error) {
	block, err := b.getBlockByHeight(height)
	if err != nil {
		return nil, err
	}

	sdkBlock := sdkConvert.FlowBlockToSDK(block)

	return &sdkBlock, nil
}

func (b *Blockchain) getBlockByHeight(height uint64) (model.Block, error) {
	block, err := b.storage.BlockByHeight(height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return model.Block{}, &BlockNotFoundByHeightError{Height: height}
		}
		return model.Block{}, err
	}

	return block, nil
}

func (b *Blockchain) GetCollection(colID sdk.Identifier) (*sdk.Collection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	col, err := b.storage.CollectionByID(sdkConvert.SDKIdentifierToFlow(colID))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &CollectionNotFoundError{ID: colID}
		}
		return nil, &StorageError{err}
	}

	sdkCol := sdkConvert.FlowLightCollectionToSDK(col)

	return &sdkCol, nil
}

// GetTransaction gets an existing transaction by ID.
//
// The function first looks in the pending block, then the current blockchain state.
func (b *Blockchain) GetTransaction(ID sdk.Identifier) (*sdk.Transaction, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	txID := sdkConvert.SDKIdentifierToFlow(ID)

	pendingTx := b.pendingBlock.GetTransaction(txID)
	if pendingTx != nil {
		pendingSDKTx := sdkConvert.FlowTransactionToSDK(*pendingTx)
		return &pendingSDKTx, nil
	}

	tx, err := b.storage.TransactionByID(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &TransactionNotFoundError{ID: txID}
		}
		return nil, &StorageError{err}
	}

	sdkTx := sdkConvert.FlowTransactionToSDK(tx)

	return &sdkTx, nil
}

func (b *Blockchain) GetTransactionResult(ID sdk.Identifier) (*sdk.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	txID := sdkConvert.SDKIdentifierToFlow(ID)

	if b.pendingBlock.ContainsTransaction(txID) {
		return &sdk.TransactionResult{
			Status: sdk.TransactionStatusPending,
		}, nil
	}

	storedResult, err := b.storage.TransactionResultByID(txID)
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

	sdkEvents, err := sdkConvert.FlowEventsToSDK(storedResult.Events)
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

// GetAccount returns the account for the given address.
func (b *Blockchain) GetAccount(address sdk.Address) (*sdk.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	flowAddress := sdkConvert.SDKAddressToFlow(address)

	account, err := b.getAccount(flowAddress)
	if err != nil {
		return nil, err
	}

	sdkAccount, err := sdkConvert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}

	return &sdkAccount, err
}

// getAccount returns the account for the given address.
func (b *Blockchain) getAccount(address model.Address) (*model.Account, error) {
	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	ledgerAccess := virtualmachine.LedgerDAL{b.storage.LedgerViewByHeight(latestBlock.Height)}

	acct := ledgerAccess.GetAccount(address)

	if acct == nil {
		return nil, &AccountNotFoundError{Address: address}
	}

	return acct, nil
}

// TODO: Implement
func (b *Blockchain) GetAccountAtBlock(address sdk.Address, blockHeight uint64) (*sdk.Account, error) {
	panic("not implemented")
}

// GetEventsByHeight returns the events in the block at the given height, optionally filtered by type.
func (b *Blockchain) GetEventsByHeight(blockHeight uint64, eventType string) ([]sdk.Event, error) {
	flowEvents, err := b.storage.EventsByHeight(blockHeight, eventType)
	if err != nil {
		return nil, err
	}

	sdkEvents, err := sdkConvert.FlowEventsToSDK(flowEvents)
	if err != nil {
		return nil, fmt.Errorf("could not convert events: %w", err)
	}

	return sdkEvents, err
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) AddTransaction(sdkTx sdk.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.addTransaction(sdkTx)
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) addTransaction(sdkTx sdk.Transaction) error {

	tx := sdkConvert.SDKTransactionToFlow(sdkTx)

	// If Index > 0, pending block has begun execution (cannot add anymore txs)
	if b.pendingBlock.ExecutionStarted() {
		return &PendingBlockMidExecutionError{BlockID: b.pendingBlock.ID()}
	}

	if b.pendingBlock.ContainsTransaction(tx.ID()) {
		return &DuplicateTransactionError{TxID: tx.ID()}
	}

	_, err := b.storage.TransactionByID(tx.ID())
	if err == nil {
		// Found the transaction, this is a dupe
		return &DuplicateTransactionError{TxID: tx.ID()}
	} else if !errors.Is(err, storage.ErrNotFound) {
		// Error in the storage provider
		return fmt.Errorf("failed to check storage for transaction %w", err)
	}

	if tx.ProposalKey == (model.ProposalKey{}) {
		return &InvalidTransactionError{TxID: tx.ID(), MissingFields: []string{"proposal_key"}}
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
	blockContext := b.virtualMachine.NewBlockContext(header)

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
	blockContext := b.virtualMachine.NewBlockContext(header)

	return b.executeNextTransaction(blockContext)
}

// executeNextTransaction is a helper function for ExecuteBlock and ExecuteNextTransaction that
// executes the next transaction in the pending block.
func (b *Blockchain) executeNextTransaction(blockContext virtualmachine.BlockContext) (*types.TransactionResult, error) {
	// check if there are remaining txs to be executed
	if b.pendingBlock.ExecutionComplete() {
		return nil, &PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.ID(),
		}
	}

	// use the computer to execute the next transaction
	result, err := b.pendingBlock.ExecuteNextTransaction(
		func(
			ledgerView *delta.View,
			tx *model.TransactionBody,
		) (*virtualmachine.TransactionResult, error) {
			return blockContext.ExecuteTransaction(ledgerView, tx)
		},
	)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	tr := convert.VMTransactionResultToEmulator(*result, b.pendingBlock.index)

	return &tr, nil
}

// CommitBlock seals the current pending block and saves it to storage.
//
// This function clears the pending transaction pool and resets the pending block.
func (b *Blockchain) CommitBlock() (*sdk.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	flowBlock, err := b.commitBlock()
	if err != nil {
		return nil, err
	}

	sdkBlock := sdkConvert.FlowBlockToSDK(*flowBlock)

	return &sdkBlock, err
}

func (b *Blockchain) commitBlock() (*model.Block, error) {
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
	delta := b.pendingBlock.LedgerDelta()
	events := b.pendingBlock.Events()

	// commit the pending block to storage
	err = b.storage.CommitBlock(*block, collections, transactions, transactionResults, delta, events)
	if err != nil {
		return nil, err
	}

	ledgerView := b.storage.LedgerViewByHeight(block.Header.Height)

	// reset pending block using current block and ledger state
	b.pendingBlock = newPendingBlock(block, ledgerView)

	return block, nil
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) ExecuteAndCommitBlock() (*sdk.Block, []*types.TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeAndCommitBlock()
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) executeAndCommitBlock() (*sdk.Block, []*types.TransactionResult, error) {

	results, err := b.executeBlock()
	if err != nil {
		return nil, nil, err
	}

	block, err := b.commitBlock()
	if err != nil {
		return nil, results, err
	}

	sdkBlock := sdkConvert.FlowBlockToSDK(*block)
	return &sdkBlock, results, nil
}

// ResetPendingBlock clears the transactions in pending block.
func (b *Blockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return err
	}

	latestLedgerView := b.storage.LedgerViewByHeight(latestBlock.Header.Height)

	// reset pending block using latest committed block and ledger state
	b.pendingBlock = newPendingBlock(&latestBlock, latestLedgerView)

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *Blockchain) ExecuteScript(script []byte) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	return b.ExecuteScriptAtBlock(script, latestBlock.Height)
}

func (b *Blockchain) ExecuteScriptAtBlock(script []byte, blockHeight uint64) (*types.ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	requestedBlock, err := b.getBlockByHeight(blockHeight)
	if err != nil {
		return nil, err
	}

	requestedLedgerView := b.storage.LedgerViewByHeight(requestedBlock.Header.Height)

	header := requestedBlock.Header

	result, err := b.virtualMachine.NewBlockContext(header).ExecuteScript(requestedLedgerView, script)

	if err != nil {
		return nil, err
	}

	hasher := hash.NewSHA3_256()
	scriptID := sdk.HashToID(hasher.ComputeHash(script))

	events := sdkConvert.RuntimeEventsToSDK(result.Events, scriptID, 0)

	var scriptError error = nil
	var convertedValue cadence.Value = nil

	if result.Error == nil {
		convertedValue = result.Value
	} else {
		scriptError = convert.VMErrorToEmulator(result.Error)
	}

	return &types.ScriptResult{
		ScriptID: scriptID,
		Value:    convertedValue,
		Error:    scriptError,
		Logs:     result.Logs,
		Events:   events,
	}, nil
}

// LastCreatedAccount returns the last account that was created in the blockchain.
func (b *Blockchain) LastCreatedAccount() *model.Account {

	ledgerAccess := virtualmachine.LedgerDAL{Ledger: b.pendingBlock.ledgerView}

	account := ledgerAccess.GetAccount(ledgerAccess.GetLatestAccount())
	return account
}

// CreateAccount submits a transaction to create a new account with the given
// account keys and code. The transaction is paid by the root account.
func (b *Blockchain) CreateAccount(publicKeys []*sdk.AccountKey, code []byte) (sdk.Address, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	createAccountScript, err := templates.CreateAccount(publicKeys, code)

	if err != nil {
		return sdk.Address{}, err
	}

	rootKey := b.RootKey()
	rootKeyAddress := rootKey.Address

	fmt.Printf("rootKey = %x\n", rootKey.PrivateKey.Encode())

	tx := sdk.NewTransaction().
		SetScript(createAccountScript).
		SetGasLimit(MaxGasLimit).
		SetProposalKey(rootKeyAddress, rootKey.ID, rootKey.SequenceNumber).
		SetPayer(rootKeyAddress)

	err = tx.SignEnvelope(rootKeyAddress, rootKey.ID, rootKey.Signer())
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

	return sdkConvert.FlowAddressToSDK(b.LastCreatedAccount().Address), nil
}

// createAccount creates an account with the given private key and injects it
// into the given state, bypassing the need for a transaction.
func createAccount(ledgerView *delta.View, accountKey *sdk.AccountKey) (sdk.Account, error) {
	flowAccountKey, err := sdkConvert.SDKAccountPublicKeyToFlow(*accountKey)
	if err != nil {
		return sdk.Account{}, err
	}

	ledgerAccess := virtualmachine.LedgerDAL{Ledger: ledgerView}
	accountAddress, err := ledgerAccess.CreateAccountInLedger([]model.AccountPublicKey{flowAccountKey})

	if err != nil {
		return sdk.Account{}, err
	}

	account := ledgerAccess.GetAccount(accountAddress)

	sdkAccount, err := sdkConvert.FlowAccountToSDK(*account)
	if err != nil {
		return sdk.Account{}, err
	}

	return sdkAccount, nil
}

func convertToSealedResults(
	results map[model.Identifier]IndexedTransactionResult,
) (map[model.Identifier]*types.StorableTransactionResult, error) {

	output := make(map[model.Identifier]*types.StorableTransactionResult)

	for id, result := range results {
		temp, err := convert.ToStorableResult(result.TransactionResult, result.Index)
		if err != nil {
			return nil, err
		}
		output[id] = &temp
	}

	return output, nil
}

const DefaultRootPrivateKeySeed = "elephant ears space cowboy octopus rodeo potato cannon pineapple"

func init() {
	// Initialize default emulator options
	privateKey, err := sdkCrypto.GeneratePrivateKey(sdkCrypto.ECDSA_P256, []byte(DefaultRootPrivateKeySeed))
	if err != nil {
		panic("Failed to generate default root key: " + err.Error())
	}

	defaultConfig.RootKey = RootKey{
		PrivateKey: &privateKey,
		SignAlgo:   privateKey.Algorithm(),
		HashAlgo:   sdkCrypto.SHA3_256,
	}
}
