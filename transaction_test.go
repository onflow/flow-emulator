package emulator_test

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.IsType(t, err, &emulator.InvalidTransactionError{})
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
		assert.IsType(t, err, &emulator.InvalidTransactionError{})
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
		assert.IsType(t, err, &emulator.InvalidTransactionError{})
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
		assert.IsType(t, err, &emulator.InvalidTransactionError{})
	})

	t.Run("MissingProposalKey", func(t *testing.T) {

		// Create transaction with no PayerAccount field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10)

		tx.ProposalKey = flow.ProposalKey{}

		err := tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.InvalidTransactionError{})
	})

	t.Run("WrongSequenceKey", func(t *testing.T) {

		invalidSequenceNumber := b.RootKey().SequenceNumber + 2137
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetPayer(b.RootKey().Address).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, invalidSequenceNumber).
			SetGasLimit(10).
			AddAuthorizer(b.RootKey().Address)

		err := tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		spew.Dump(result)

		require.Error(t, result.Error)

		assert.IsType(t, &virtualmachine.InvalidProposalSequenceNumberError{}, result.Error)
		assert.Equal(t, invalidSequenceNumber, result.Error.(*virtualmachine.InvalidProposalSequenceNumberError).ProvidedSeqNumber)
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
	assert.IsType(t, err, &emulator.DuplicateTransactionError{})
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

	accountKeys := test.AccountKeyGenerator()

	accountKeyB, signerB := accountKeys.NewWithSigner()
	accountKeyB.SetWeight(flow.AccountKeyWeightThreshold)

	accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, nil)
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

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
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
	accountKeys := test.AccountKeyGenerator()

	t.Run("MissingPayerSignature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(addressA).
			AddAuthorizer(b.RootKey().Address)

		err = tx.SignPayload(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.IsType(t, result.Error, &virtualmachine.MissingSignatureError{})
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
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.IsType(t, result.Error, &virtualmachine.InvalidSignatureAccountError{})
	})

	t.Run("InvalidKeyPair", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// use key-pair that does not exist on root account
		invalidKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256,
			[]byte("invalid key invalid key invalid key invalid key invalid key invalid key"))
		invalidSigner := crypto.NewNaiveSigner(invalidKey, crypto.SHA3_256)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, invalidSigner)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.IsType(t, result.Error, &virtualmachine.InvalidSignaturePublicKeyError{})
	})

	t.Run("KeyWeights", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKeyA, signerA := accountKeys.NewWithSigner()
		accountKeyA.SetWeight(flow.AccountKeyWeightThreshold / 2)

		accountKeyB, signerB := accountKeys.NewWithSigner()
		accountKeyB.SetWeight(flow.AccountKeyWeightThreshold / 2)

		accountAddressA, err := b.CreateAccount([]*flow.AccountKey{accountKeyA, accountKeyB}, nil)
		assert.NoError(t, err)

		script := []byte(`
		  transaction {
		    prepare(signer: AuthAccount) {}
		  }
		`)

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(10).
			SetProposalKey(accountAddressA, 1, 0).
			SetPayer(accountAddressA).
			AddAuthorizer(accountAddressA)

		// Insufficient keys
		err = tx.SignEnvelope(accountAddressA, 1, signerB)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		// Sufficient keys
		err = tx.SignEnvelope(accountAddressA, 0, signerA)
		assert.NoError(t, err)

		err = tx.SignEnvelope(accountAddressA, 1, signerB)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		t.Run("InsufficientKeyWeight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			assert.IsType(t, &virtualmachine.MissingSignatureError{}, result.Error)
		})

		t.Run("SufficientKeyWeight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			assertTransactionSucceeded(t, result)
		})
	})
}

func TestSubmitTransactionScriptSignatures(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

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
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.IsType(t, result.Error, &virtualmachine.MissingSignatureError{})
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountKeyB, signerB := accountKeys.NewWithSigner()
		accountKeyB.SetWeight(flow.AccountKeyWeightThreshold)

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, nil)
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

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
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
		if assert.Error(t, err) {
			assert.IsType(t, &emulator.TransactionNotFoundError{}, err)
		}
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
	assert.Equal(t, 0, event.EventIndex)
	assert.Equal(t, cadence.NewInt(2), event.Value.Fields[0])
}
