package emulator_test

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/fvm"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/utils/unittest"
)

const testContract = "pub contract Test {}"

func TestCreateAccount(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	t.Run("Simple addresses", func(t *testing.T) {
		b, err := emulator.NewBlockchain(emulator.WithSimpleAddresses())
		require.NoError(t, err)

		accountKey := accountKeys.New()

		tx := templates.CreateAccount([]*flow.AccountKey{accountKey}, nil, b.ServiceKey().Address)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := lastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Equal(t, "0000000000000005", account.Address.Hex())
		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Empty(t, account.Code)
	})

	t.Run("Single public keys", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()

		tx := templates.CreateAccount([]*flow.AccountKey{accountKey}, nil, b.ServiceKey().Address)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := lastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Empty(t, account.Code)
	})

	t.Run("Multiple public keys", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKeyA := accountKeys.New()
		accountKeyB := accountKeys.New()

		tx := templates.CreateAccount(
			[]*flow.AccountKey{accountKeyA, accountKeyB},
			nil,
			b.ServiceKey().Address,
		)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := lastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Equal(t, accountKeyB.PublicKey.Encode(), account.Keys[1].PublicKey.Encode())
		assert.Empty(t, account.Code)
	})

	t.Run("Public keys and code", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKeyA := accountKeys.New()
		accountKeyB := accountKeys.New()

		code := []byte(testContract)

		tx := templates.CreateAccount(
			[]*flow.AccountKey{accountKeyA, accountKeyB},
			code,
			b.ServiceKey().Address,
		)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := lastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Equal(t, accountKeyB.PublicKey.Encode(), account.Keys[1].PublicKey.Encode())
		assert.Equal(t, code, account.Code)
	})

	t.Run("Code and no keys", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		code := []byte(testContract)

		tx := templates.CreateAccount(
			nil,
			code,
			b.ServiceKey().Address,
		)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := lastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		assert.Empty(t, account.Keys)
		assert.Equal(t, code, account.Code)
	})

	t.Run("Event emitted", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()

		code := []byte(testContract)

		tx := templates.CreateAccount(
			[]*flow.AccountKey{accountKey},
			code,
			b.ServiceKey().Address,
		)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		events, err := b.GetEventsByHeight(block.Header.Height, flow.EventAccountCreated)
		require.NoError(t, err)
		require.Len(t, events, 1)

		accountEvent := flow.AccountCreatedEvent(events[0])

		account, err := b.GetAccount(accountEvent.Address())
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey, account.Keys[0].PublicKey)
		assert.Equal(t, code, account.Code)
	})

	t.Run("Invalid hash algorithm", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()
		accountKey.SetHashAlgo(crypto.SHA3_384) // SHA3_384 is invalid for ECDSA_P256

		tx := templates.CreateAccount(
			[]*flow.AccountKey{accountKey},
			nil,
			b.ServiceKey().Address,
		)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())
	})

	t.Run("Invalid code", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		code := []byte("not a valid script")

		tx := templates.CreateAccount(
			nil,
			code,
			b.ServiceKey().Address,
		)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())
	})
}

func TestAddAccountKey(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	t.Run("Valid key", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		newAccountKey, newSigner := accountKeys.NewWithSigner()

		tx1 := templates.AddAccountKey(b.ServiceKey().Address, newAccountKey)

		tx1.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx1)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		script := []byte("transaction { execute {} }")

		var newKeyID = 1 // new key with have ID 1
		var newKeySequenceNum uint64 = 0

		tx2 := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, newKeyID, newKeySequenceNum).
			SetPayer(b.ServiceKey().Address)

		err = tx2.SignEnvelope(b.ServiceKey().Address, newKeyID, newSigner)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx2)
		require.NoError(t, err)

		result, err = b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})

	t.Run("Invalid hash algorithm", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()
		accountKey.SetHashAlgo(crypto.SHA3_384) // SHA3_384 is invalid for ECDSA_P256

		tx := templates.AddAccountKey(b.ServiceKey().Address, accountKey)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())
	})
}

func TestRemoveAccountKey(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	accountKeys := test.AccountKeyGenerator()

	newAccountKey, newSigner := accountKeys.NewWithSigner()

	// create transaction that adds public key to account keys
	tx1 := templates.AddAccountKey(b.ServiceKey().Address, newAccountKey)
	assert.NoError(t, err)

	// create transaction that adds public key to account keys
	tx1.SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	// sign with service key
	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
	assert.NoError(t, err)

	// submit tx1 (should succeed)
	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err := b.GetAccount(b.ServiceKey().Address)
	assert.NoError(t, err)

	assert.Len(t, account.Keys, 2)

	// create transaction that removes service key
	tx2 := templates.RemoveAccountKey(b.ServiceKey().Address, 0)

	tx2.SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	// sign with service key
	err = tx2.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
	assert.NoError(t, err)

	// submit tx2 (should succeed)
	err = b.AddTransaction(*tx2)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = b.GetAccount(b.ServiceKey().Address)
	assert.NoError(t, err)

	assert.Len(t, account.Keys, 1)

	// create transaction that removes remaining account key
	tx3 := templates.RemoveAccountKey(b.ServiceKey().Address, 0)

	tx3.SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	// sign with service key (that has been removed)
	err = tx3.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
	assert.NoError(t, err)

	// submit tx3 (should fail)
	err = b.AddTransaction(*tx3)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)

	unittest.AssertFVMErrorType(t, &fvm.InvalidSignaturePublicKeyError{}, result.Error)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = b.GetAccount(b.ServiceKey().Address)
	assert.NoError(t, err)

	assert.Len(t, account.Keys, 1)

	// create transaction that removes remaining account key
	tx4 := templates.RemoveAccountKey(b.ServiceKey().Address, 0)

	tx4.SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	// sign with remaining account key
	err = tx4.SignEnvelope(b.ServiceKey().Address, 0, newSigner)
	assert.NoError(t, err)

	// submit tx4 (should succeed)
	err = b.AddTransaction(*tx4)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = b.GetAccount(b.ServiceKey().Address)
	assert.NoError(t, err)

	// no more keys left on account
	assert.Empty(t, account.Keys)
}

func TestUpdateAccountCode(t *testing.T) {
	codeA := []byte(`
      pub contract Test {
          pub fun a(): Int {
              return 1
          }
      }
    `)
	codeB := []byte(`
      pub contract Test {
          pub fun b(): Int {
              return 2
          }
      }
    `)

	accountKeys := test.AccountKeyGenerator()

	accountKeyB, signerB := accountKeys.NewWithSigner()

	t.Run("Valid signature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, codeA)
		require.NoError(t, err)

		account, err := b.GetAccount(accountAddressB)
		require.NoError(t, err)

		assert.Equal(t, codeA, account.Code)

		tx := templates.UpdateAccountCode(accountAddressB, codeB)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err = b.GetAccount(accountAddressB)
		assert.NoError(t, err)

		assert.Equal(t, codeB, account.Code)
	})

	t.Run("Invalid signature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, codeA)
		require.NoError(t, err)

		account, err := b.GetAccount(accountAddressB)
		require.NoError(t, err)

		assert.Equal(t, codeA, account.Code)

		tx := templates.UpdateAccountCode(accountAddressB, codeB)

		tx.SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvm.MissingSignatureError{}, result.Error)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err = b.GetAccount(accountAddressB)
		assert.NoError(t, err)

		// code should not be updated
		assert.Equal(t, codeA, account.Code)
	})
}

func TestImportAccountCode(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	accountScript := []byte(`
      pub contract Computer {
          pub fun answer(): Int {
              return 42
          }
      }
	`)

	address, err := b.CreateAccount(nil, accountScript)
	assert.NoError(t, err)

	script := []byte(fmt.Sprintf(`
		// address imports can omit leading zeros
		import 0x%s

		transaction {
		  execute {
			let answer = Computer.answer()
			if answer != 42 {
				panic("?!")
			}
		  }
		}
	`, address))

	tx := flow.NewTransaction().
		SetScript(script).
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
	assert.NoError(t, err)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)
}
