package emulator_test

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/cadence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/cadence/runtime"
	"github.com/dapperlabs/cadence/runtime/interpreter"
	"github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/keys"

	emulator "github.com/dapperlabs/flow-emulator"
)

func TestSubmitTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	err = tx1.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Submit tx1
	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err := b.GetTransactionResult(tx1.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, tx1Result.Status)
}

// TODO: Add test case for missing ReferenceBlockID
// TODO: Add test case for missing ProposalKey
func TestSubmitInvalidTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	t.Run("EmptyTransaction", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		// Create empty transaction (no required fields)
		tx := flow.NewTransaction()

		err := tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingScript", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		// Create transaction with no Script field
		tx := flow.NewTransaction().
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err := tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingGasLimit", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		// Create transaction with no GasLimit field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address)

		err := tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingPayerAccount", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		// Create transaction with no PayerAccount field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetGasLimit(10)

		err := tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Submit tx
	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// Submit same tx again (errors)
	err = b.AddTransaction(*tx)
	assert.IsType(t, err, &emulator.ErrDuplicateTransaction{})
}

func TestSubmitTransactionReverted(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	tx := flow.NewTransaction().
		SetScript([]byte("invalid script")).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Submit invalid tx1
	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Reverted())

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err := b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, tx1Result.Status)
	assert.Error(t, tx1Result.Error)
}

func TestSubmitTransactionScriptAccounts(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
		[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
	publicKeyB := privateKeyB.ToAccountKey()
	publicKeyB.Weight = keys.PublicKeyWeightThreshold

	accountAddressB, err := b.CreateAccount([]flow.AccountKey{publicKeyB}, nil)
	assert.NoError(t, err)

	t.Run("TooManyAccountsForScript", func(t *testing.T) {
		// script only supports one account
		script := []byte(`
		  transaction {
		    prepare(signer: AuthAccount) {}
		  }
		`)

		// create transaction with two authorizing accounts
		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, publicKeyB.ID, privateKeyB.Signer())
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})

	t.Run("NotEnoughAccountsForScript", func(t *testing.T) {
		// script requires two accounts
		script := []byte(`
		  transaction {
		    prepare(signerA: AuthAccount, signerB: AuthAccount) {}
		  }
		`)

		// create transaction with two accounts
		tx := flow.NewTransaction().
			SetScript(script).
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

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})
}

func TestSubmitTransactionPayerSignature(t *testing.T) {
	t.Run("MissingPayerSignature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(addressA, 0, 0).
			SetPayer(addressA).
			AddAuthorizer(b.RootKey().Address)

		err = tx.SignPayload(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("InvalidAccount", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		invalidAddress := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.NewTransaction().
			SetScript([]byte(`transaction { execute { } }`)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(invalidAddress)

		err = tx.SignPayload(invalidAddress, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrInvalidSignatureAccount{})
	})

	t.Run("InvalidKeyPair", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// use key-pair that does not exist on root account
		invalidKey, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("invalid key elephant ears space cowboy octopus rodeo potato cannon"))

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, invalidKey.Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrInvalidSignaturePublicKey{})
	})

	t.Run("KeyWeights", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		privateKeyA, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
		publicKeyA := privateKeyA.ToAccountKey()
		publicKeyA.Weight = keys.PublicKeyWeightThreshold / 2

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("space cowboy elephant ears octopus rodeo potato cannon pineapple"))
		publicKeyB := privateKeyB.ToAccountKey()
		publicKeyB.Weight = keys.PublicKeyWeightThreshold / 2

		accountAddressA, err := b.CreateAccount([]flow.AccountKey{publicKeyA, publicKeyB}, nil)
		assert.NoError(t, err)

		script := []byte(`
		  transaction {
		    prepare(signer: AuthAccount) {}
		  }
		`)

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(10).
			SetProposalKey(accountAddressA, 0, 0).
			SetPayer(accountAddressA).
			AddAuthorizer(accountAddressA)

		t.Run("InsufficientKeyWeight", func(t *testing.T) {
			err = tx.SignEnvelope(accountAddressA, 1, privateKeyB.Signer())
			assert.NoError(t, err)

			err = b.AddTransaction(*tx)
			assert.IsType(t, err, &emulator.ErrMissingSignature{})
		})

		t.Run("SufficientKeyWeight", func(t *testing.T) {
			err = tx.SignEnvelope(accountAddressA, 0, privateKeyA.Signer())
			assert.NoError(t, err)

			err = tx.SignEnvelope(accountAddressA, 1, privateKeyB.Signer())
			assert.NoError(t, err)

			err = b.AddTransaction(*tx)
			assert.NoError(t, err)

			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)
			assertTransactionSucceeded(t, result)
		})
	})
}

func TestSubmitTransactionScriptSignatures(t *testing.T) {
	t.Run("MissingScriptSignature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(addressA)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
		publicKeyB := privateKeyB.ToAccountKey()
		publicKeyB.Weight = keys.PublicKeyWeightThreshold

		accountAddressB, err := b.CreateAccount([]flow.AccountKey{publicKeyB}, nil)
		assert.NoError(t, err)

		multipleAccountScript := []byte(`
		  transaction {
		    prepare(signerA: AuthAccount, signerB: AuthAccount) {
		      log(signerA.address)
			  log(signerB.address)
		    }
		  }
		`)

		tx := flow.NewTransaction().
			SetScript(multipleAccountScript).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, privateKeyB.Signer())
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		assert.Contains(t,
			result.Logs,
			interpreter.NewAddressValueFromBytes(b.RootKey().Address.Bytes()).String(),
		)

		assert.Contains(t,
			result.Logs,
			interpreter.NewAddressValueFromBytes(accountAddressB.Bytes()).String(),
		)
	})
}

func TestGetTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
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

	t.Run("Non-existent", func(t *testing.T) {
		_, err := b.GetTransaction(flow.ZeroID)
		assert.IsType(t, &emulator.ErrTransactionNotFound{}, err)
	})

	t.Run("Exists", func(t *testing.T) {
		tx2, err := b.GetTransaction(tx1.ID())
		require.NoError(t, err)

		assert.Equal(t, tx1.ID(), tx2.ID())
	})
}

func TestGetTransactionResult(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, counterAddress := deployAndGenerateAddTwoScript(t, b)

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	result, err := b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusUnknown, result.Status)
	require.Empty(t, result.Events)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err = b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusPending, result.Status)
	require.Empty(t, result.Events)

	_, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)

	result, err = b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusPending, result.Status)
	require.Empty(t, result.Events)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	result, err = b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, result.Status)

	require.Len(t, result.Events, 1)

	event := result.Events[0]

	location := runtime.AddressLocation(counterAddress.Bytes())
	eventType := fmt.Sprintf("%s.Counting.CountIncremented", location.ID())

	assert.Equal(t, tx.ID(), event.TransactionID)
	assert.Equal(t, eventType, event.Type)
	assert.Equal(t, uint(0), event.Index)
	assert.Equal(t, cadence.NewInt(2), event.Value.Fields[0])
}
