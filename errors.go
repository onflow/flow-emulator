package emulator

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go/crypto"
)

type NotFoundError interface {
	isNotFoundError()
}

type BlockNotFoundError interface {
	isBlockNotFoundError()
}

// BlockNotFoundByHeightError indicates that a block could not be found at the specified height.
type BlockNotFoundByHeightError struct {
	Height uint64
}

func (e *BlockNotFoundByHeightError) isNotFoundError()      {}
func (e *BlockNotFoundByHeightError) isBlockNotFoundError() {}

func (e *BlockNotFoundByHeightError) Error() string {
	return fmt.Sprintf("could not find block at height %d", e.Height)
}

// BlockNotFoundByIDError indicates that a block with the specified ID could not be found.
type BlockNotFoundByIDError struct {
	ID flow.Identifier
}

func (e *BlockNotFoundByIDError) isNotFoundError()      {}
func (e *BlockNotFoundByIDError) isBlockNotFoundError() {}

func (e *BlockNotFoundByIDError) Error() string {
	return fmt.Sprintf("could not find block with ID %s", e.ID)
}

// CollectionNotFoundError indicates that a collection could not be found.
type CollectionNotFoundError struct {
	ID flow.Identifier
}

func (e *CollectionNotFoundError) isNotFoundError() {}

func (e *CollectionNotFoundError) Error() string {
	return fmt.Sprintf("could not find collection with ID %s", e.ID)
}

// TransactionNotFoundError indicates that a transaction could not be found.
type TransactionNotFoundError struct {
	ID flow.Identifier
}

func (e *TransactionNotFoundError) isNotFoundError() {}

func (e *TransactionNotFoundError) Error() string {
	return fmt.Sprintf("could not find transaction with ID %s", e.ID)
}

// AccountNotFoundError indicates that an account could not be found.
type AccountNotFoundError struct {
	Address flow.Address
}

func (e *AccountNotFoundError) isNotFoundError() {}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf("could not find account with address %s", e.Address)
}

// DuplicateTransactionError indicates that a transaction has already been submitted.
type DuplicateTransactionError struct {
	TxID flow.Identifier
}

func (e *DuplicateTransactionError) Error() string {
	return fmt.Sprintf("Transaction with ID %s has already been submitted", e.TxID)
}

// MissingSignatureError indicates that a transaction is missing a required signature.
type MissingSignatureError struct {
	Account flow.Address
}

func (e *MissingSignatureError) Error() string {
	return fmt.Sprintf("Account %s does not have sufficient signatures", e.Account)
}

// InvalidSignaturePublicKeyError indicates that signature uses an invalid public key.
type InvalidSignaturePublicKeyError struct {
	Account flow.Address
	KeyID   int
}

func (e *InvalidSignaturePublicKeyError) Error() string {
	return fmt.Sprintf("invalid signature for key %d on account %s", e.KeyID, e.Account)
}

// InvalidSignatureAccountError indicates that a signature references a nonexistent account.
type InvalidSignatureAccountError struct {
	Address flow.Address
}

func (e *InvalidSignatureAccountError) Error() string {
	return fmt.Sprintf("Account with address %s does not exist", e.Address)
}

// InvalidTransactionError indicates that a submitted transaction is invalid (missing required fields).
type InvalidTransactionError struct {
	TxID          flow.Identifier
	MissingFields []string
}

func (e *InvalidTransactionError) Error() string {
	return fmt.Sprintf(
		"Transaction with ID %s is invalid (missing required fields): %s",
		e.TxID,
		strings.Join(e.MissingFields, ", "),
	)
}

// InvalidStateVersionError indicates that a state version hash provided is invalid.
type InvalidStateVersionError struct {
	Version crypto.Hash
}

func (e *InvalidStateVersionError) Error() string {
	return fmt.Sprintf("Execution state with version hash %x is invalid", e.Version)
}

// PendingBlockCommitBeforeExecutionError indicates that the current pending block has not been executed (cannot commit).
type PendingBlockCommitBeforeExecutionError struct {
	BlockID flow.Identifier
}

func (e *PendingBlockCommitBeforeExecutionError) Error() string {
	return fmt.Sprintf("Pending block with ID %s cannot be commited before execution", e.BlockID)
}

// PendingBlockMidExecutionError indicates that the current pending block is mid-execution.
type PendingBlockMidExecutionError struct {
	BlockID flow.Identifier
}

func (e *PendingBlockMidExecutionError) Error() string {
	return fmt.Sprintf("Pending block with ID %s is currently being executed", e.BlockID)
}

// PendingBlockTransactionsExhaustedError indicates that the current pending block has finished executing (no more transactions to execute).
type PendingBlockTransactionsExhaustedError struct {
	BlockID flow.Identifier
}

func (e *PendingBlockTransactionsExhaustedError) Error() string {
	return fmt.Sprintf("Pending block with ID %s contains no more transactions to execute", e.BlockID)
}

// StorageError indicates that an error occurred in the storage provider.
type StorageError struct {
	inner error
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("storage failure: %v", e.inner)
}

func (e *StorageError) Unwrap() error {
	return e.inner
}
