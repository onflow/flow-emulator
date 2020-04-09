package emulator

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go/crypto"
)

// ErrBlockNotFound indicates that a block specified by ID or height cannot be found.
type ErrBlockNotFound struct {
	BlockID  flow.Identifier
	BlockNum uint64
}

func (e *ErrBlockNotFound) Error() string {
	if e.BlockNum == 0 {
		return fmt.Sprintf("Block with ID %x cannot be found", e.BlockID)
	}

	return fmt.Sprintf("Block number %d cannot be found", e.BlockNum)
}

// ErrTransactionNotFound indicates that a transaction specified by ID cannot be found.
type ErrTransactionNotFound struct {
	TxID flow.Identifier
}

func (e *ErrTransactionNotFound) Error() string {
	return fmt.Sprintf("Transaction with ID %x cannot be found", e.TxID)
}

// ErrAccountNotFound indicates that an account specified by address cannot be found.
type ErrAccountNotFound struct {
	Address flow.Address
}

func (e *ErrAccountNotFound) Error() string {
	return fmt.Sprintf("Account with address %s cannot be found", e.Address)
}

// ErrDuplicateTransaction indicates that a transaction has already been submitted.
type ErrDuplicateTransaction struct {
	TxID flow.Identifier
}

func (e *ErrDuplicateTransaction) Error() string {
	return fmt.Sprintf("Transaction with ID %x has already been submitted", e.TxID)
}

// ErrMissingSignature indicates that a transaction is missing a required signature.
type ErrMissingSignature struct {
	Account flow.Address
}

func (e *ErrMissingSignature) Error() string {
	return fmt.Sprintf("Account %s does not have sufficient signatures", e.Account)
}

// ErrInvalidSignaturePublicKey indicates that signature uses an invalid public key.
type ErrInvalidSignaturePublicKey struct {
	Account flow.Address
}

func (e *ErrInvalidSignaturePublicKey) Error() string {
	return fmt.Sprintf("Public key used for signing does not exist on account %s", e.Account)
}

// ErrInvalidSignatureAccount indicates that a signature references a nonexistent account.
type ErrInvalidSignatureAccount struct {
	Address flow.Address
}

func (e *ErrInvalidSignatureAccount) Error() string {
	return fmt.Sprintf("Account with address %s does not exist", e.Address)
}

// ErrInvalidTransaction indicates that a submitted transaction is invalid (missing required fields).
type ErrInvalidTransaction struct {
	TxID          flow.Identifier
	MissingFields []string
}

func (e *ErrInvalidTransaction) Error() string {
	return fmt.Sprintf(
		"Transaction with ID %x is invalid (missing required fields): %s",
		e.TxID,
		strings.Join(e.MissingFields, ", "),
	)
}

// ErrInvalidStateVersion indicates that a state version hash provided is invalid.
type ErrInvalidStateVersion struct {
	Version crypto.Hash
}

func (e *ErrInvalidStateVersion) Error() string {
	return fmt.Sprintf("World State with version hash %x is invalid", e.Version)
}

// ErrPendingBlockCommitBeforeExecution indicates that the current pending block has not been executed (cannot commit).
type ErrPendingBlockCommitBeforeExecution struct {
	BlockID flow.Identifier
}

func (e *ErrPendingBlockCommitBeforeExecution) Error() string {
	return fmt.Sprintf("Pending block with ID %x cannot be commited before execution", e.BlockID)
}

// ErrPendingBlockMidExecution indicates that the current pending block is mid-execution.
type ErrPendingBlockMidExecution struct {
	BlockID flow.Identifier
}

func (e *ErrPendingBlockMidExecution) Error() string {
	return fmt.Sprintf("Pending block with ID %x is currently being executed", e.BlockID)
}

// ErrPendingBlockTransactionsExhausted indicates that the current pending block has finished executing (no more transactions to execute).
type ErrPendingBlockTransactionsExhausted struct {
	BlockID flow.Identifier
}

func (e *ErrPendingBlockTransactionsExhausted) Error() string {
	return fmt.Sprintf("Pending block with ID %x contains no more transactions to execute", e.BlockID)
}

// ErrStorage indicates that an error occurred in the storage provider.
type ErrStorage struct {
	inner error
}

func (e *ErrStorage) Error() string {
	return fmt.Sprintf("storage failure: %v", e.inner)
}

func (e *ErrStorage) Unwrap() error {
	return e.inner
}
