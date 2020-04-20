package emulator_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
)

const testContract = "pub contract Test {}"

func TestCreateAccount(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	t.Run("SingleKey", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()

		createAccountScript, err := templates.CreateAccount([]*flow.AccountKey{accountKey}, nil)
		require.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey, account.Keys[0].PublicKey)
		assert.Empty(t, account.Code)
	})

	t.Run("MultipleKeys", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKeyA := accountKeys.New()
		accountKeyB := accountKeys.New()

		createAccountScript, err := templates.CreateAccount([]*flow.AccountKey{accountKeyA, accountKeyB}, nil)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA.PublicKey, account.Keys[0].PublicKey)
		assert.Equal(t, accountKeyB.PublicKey, account.Keys[1].PublicKey)
		assert.Empty(t, account.Code)
	})

	t.Run("KeysAndCode", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKeyA := accountKeys.New()
		accountKeyB := accountKeys.New()

		code := []byte(testContract)

		createAccountScript, err := templates.CreateAccount([]*flow.AccountKey{accountKeyA, accountKeyB}, code)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		fmt.Println("ROOT KEY", b.RootKey())

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA.PublicKey, account.Keys[0].PublicKey)
		assert.Equal(t, accountKeyB.PublicKey, account.Keys[1].PublicKey)
		assert.Equal(t, code, account.Code)
	})

	t.Run("CodeAndNoKeys", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		code := []byte(testContract)

		createAccountScript, err := templates.CreateAccount(nil, code)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		assert.Empty(t, account.Keys)
		assert.Equal(t, code, account.Code)
	})

	t.Run("EventEmitted", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()

		code := []byte(testContract)

		createAccountScript, err := templates.CreateAccount([]*flow.AccountKey{accountKey}, code)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		events, err := b.GetEventsByHeight(block.Height, flow.EventAccountCreated)
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

	t.Run("InvalidKeyHashingAlgorithm", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		lastAccount := b.LastCreatedAccount()

		accountKey := accountKeys.New()
		accountKey.SetHashAlgo(crypto.SHA3_384) // SHA3_384 is invalid for ECDSA_P256

		createAccountScript, err := templates.CreateAccount([]*flow.AccountKey{accountKey}, nil)
		require.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		newAccount := b.LastCreatedAccount()

		assert.Equal(t, lastAccount, newAccount)
	})

	t.Run("InvalidCode", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		lastAccount := b.LastCreatedAccount()

		code := []byte("not a valid script")

		createAccountScript, err := templates.CreateAccount(nil, code)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(createAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		newAccount := b.LastCreatedAccount()

		assert.Equal(t, lastAccount, newAccount)
	})
}

func TestAddAccountKey(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	t.Run("ValidKey", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		newAccountKey, newSigner := accountKeys.NewWithSigner()

		addKeyScript, err := templates.AddAccountKey(newAccountKey)
		assert.NoError(t, err)

		tx1 := flow.NewTransaction().
			SetScript(addKeyScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx1.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
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
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, newKeyID, newKeySequenceNum).
			SetPayer(b.RootKey().Address)

		err = tx2.SignEnvelope(b.RootKey().Address, newKeyID, newSigner)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx2)
		require.NoError(t, err)

		result, err = b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})

	t.Run("InvalidKeyHashingAlgorithm", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKey := accountKeys.New()
		accountKey.SetHashAlgo(crypto.SHA3_384) // SHA3_384 is invalid for ECDSA_P256

		addKeyScript, err := templates.AddAccountKey(accountKey)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript(addKeyScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
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

	addKeyScript, err := templates.AddAccountKey(newAccountKey)
	assert.NoError(t, err)

	// create transaction that adds public key to account keys
	tx1 := flow.NewTransaction().
		SetScript(addKeyScript).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	// sign with root key
	err = tx1.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// submit tx1 (should succeed)
	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err := b.GetAccount(b.RootKey().Address)
	assert.NoError(t, err)

	assert.Len(t, account.Keys, 2)

	// create transaction that removes root key
	tx2 := flow.NewTransaction().
		SetScript(templates.RemoveAccountKey(0)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	// sign with root key
	err = tx2.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// submit tx2 (should succeed)
	err = b.AddTransaction(*tx2)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = b.GetAccount(b.RootKey().Address)
	assert.NoError(t, err)

	assert.Len(t, account.Keys, 1)

	// create transaction that removes remaining account key
	tx3 := flow.NewTransaction().
		SetScript(templates.RemoveAccountKey(0)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	// sign with root key (that has been removed)
	err = tx3.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// submit tx3 (should fail)
	err = b.AddTransaction(*tx3)
	assert.IsType(t, &emulator.InvalidSignaturePublicKeyError{}, err)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = b.GetAccount(b.RootKey().Address)
	assert.NoError(t, err)

	assert.Len(t, account.Keys, 1)

	// create transaction that removes remaining account key
	tx4 := flow.NewTransaction().
		SetScript(templates.RemoveAccountKey(0)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	// sign with remaining account key
	err = tx4.SignEnvelope(b.RootKey().Address, 0, newSigner)
	assert.NoError(t, err)

	// submit tx4 (should succeed)
	err = b.AddTransaction(*tx4)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = b.GetAccount(b.RootKey().Address)
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

	t.Run("ValidSignature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, codeA)
		require.NoError(t, err)

		account, err := b.GetAccount(accountAddressB)
		require.NoError(t, err)

		assert.Equal(t, codeA, account.Code)

		tx := flow.NewTransaction().
			SetScript(templates.UpdateAccountCode(codeB)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
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

	t.Run("InvalidSignature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, codeA)
		require.NoError(t, err)

		account, err := b.GetAccount(accountAddressB)
		require.NoError(t, err)

		assert.Equal(t, codeA, account.Code)

		tx := flow.NewTransaction().
			SetScript(templates.UpdateAccountCode(codeB)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.MissingSignatureError{}, err)

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

	assert.Equal(t, flow.HexToAddress("02"), address)

	script := []byte(`
		// address imports can omit leading zeros
		import 0x02

		transaction {
		  execute {
			let answer = Computer.answer()
			if answer != 42 {
				panic("?!")
			}
		  }
		}
	`)

	tx := flow.NewTransaction().
		SetScript(script).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address)

	err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)
}
