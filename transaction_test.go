package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
		AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

	err = tx1.SignContainer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Submit tx1
	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	// TODO: fix once GetTransactionStatus is implemented
	// tx1 status becomes TransactionFinalized
	// tx2, err := b.GetTransaction(tx1.ID())
	// assert.NoError(t, err)
	// assert.Equal(t, flow.TransactionFinalized, tx2.Status)
}

// TODO: Add test case for missing ReferenceBlockID
func TestSubmitInvalidTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	t.Run("EmptyTransaction", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		// Create empty transaction (no required fields)
		tx := flow.NewTransaction()

		err := tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
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
			SetPayer(b.RootAccountAddress(), 0)

		err := tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
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
			SetPayer(b.RootAccountAddress(), 0)

		err := tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
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
			SetGasLimit(10)

		err := tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
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

	accountAddress := b.RootAccountAddress()

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetPayer(accountAddress, 0).
		AddAuthorizer(accountAddress, 0)

	err = tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
	assert.NoError(t, err)

	// Submit tx
	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

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
		SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
		AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

	err = tx.SignContainer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Submit invalid tx1
	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Reverted())

	// TODO: fix once GetTransactionStatus is implemented
	// tx1 status becomes TransactionReverted
	// tx1, err := b.GetTransaction(tx.ID())
	// assert.NoError(t, err)
	// assert.Equal(t, flow.TransactionReverted, tx1.Status)
}

func TestSubmitTransactionScriptAccounts(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	privateKeyA := b.RootKey()

	privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
		[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
	publicKeyB := privateKeyB.ToAccountKey()
	publicKeyB.Weight = keys.PublicKeyWeightThreshold

	accountAddressA := b.RootAccountAddress()
	accountAddressB, err := b.CreateAccount([]flow.AccountKey{publicKeyB}, nil, getNonce())
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
			SetPayer(accountAddressA, 0).
			AddAuthorizer(accountAddressA, 0).
			AddAuthorizer(accountAddressB, 0)

		err = tx.SignPayload(accountAddressB, 0, privateKeyB.Signer())
		assert.NoError(t, err)

		err = tx.SignContainer(accountAddressA, 0, privateKeyA.Signer())
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
			SetPayer(accountAddressA, 0).
			AddAuthorizer(accountAddressA, 0)

		err = tx.SignContainer(accountAddressA, 0, privateKeyA.Signer())
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
			SetPayer(addressA, 0).
			AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

		err = tx.SignContainer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
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
			SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
			AddAuthorizer(invalidAddress, b.RootKey().ToAccountKey().ID)

		err = tx.SignPayload(invalidAddress, b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		err = tx.SignContainer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
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
			SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
			AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

		err = tx.SignContainer(b.RootAccountAddress(), 0, invalidKey.Signer())
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

		accountAddressA, err := b.CreateAccount([]flow.AccountKey{publicKeyA, publicKeyB}, nil, getNonce())
		assert.NoError(t, err)

		script := []byte(`
		  transaction {
		    prepare(signer: AuthAccount) {}
		  }
		`)

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(10).
			SetPayer(accountAddressA, 0).
			AddAuthorizer(accountAddressA, 0)

		t.Run("InsufficientKeyWeight", func(t *testing.T) {
			err = tx.SignContainer(accountAddressA, 1, privateKeyB.Signer())
			assert.NoError(t, err)

			err = b.AddTransaction(*tx)
			assert.IsType(t, err, &emulator.ErrMissingSignature{})
		})

		t.Run("SufficientKeyWeight", func(t *testing.T) {
			err = tx.SignContainer(accountAddressA, 0, privateKeyA.Signer())
			assert.NoError(t, err)

			err = tx.SignContainer(accountAddressA, 1, privateKeyB.Signer())
			assert.NoError(t, err)

			err = b.AddTransaction(*tx)
			assert.NoError(t, err)

			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)
			assert.True(t, result.Succeeded())
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
			SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
			AddAuthorizer(addressA, 0)

		err = tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		privateKeyA := b.RootKey()

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
		publicKeyB := privateKeyB.ToAccountKey()
		publicKeyB.Weight = keys.PublicKeyWeightThreshold

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]flow.AccountKey{publicKeyB}, nil, getNonce())
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
			SetPayer(accountAddressA, 0).
			AddAuthorizer(accountAddressA, 0).
			AddAuthorizer(accountAddressB, 0)

		err = tx.SignPayload(accountAddressB, 0, privateKeyB.Signer())
		assert.NoError(t, err)

		err = tx.SignContainer(accountAddressA, 0, privateKeyA.Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		assert.Contains(t,
			result.Logs,
			interpreter.NewAddressValueFromBytes(accountAddressA.Bytes()).String(),
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

	// TODO: fix once GetTransactionStatus is implemented
	// addTwoScript, counterAddress := deployAndGenerateAddTwoScript(t, b)
	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	accountAddress := b.RootAccountAddress()

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetPayer(accountAddress, 0).
		AddAuthorizer(accountAddress, 0)

	err = tx.SignContainer(b.RootAccountAddress(), 0, b.RootKey().Signer())
	assert.NoError(t, err)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	t.Run("InvalidHash", func(t *testing.T) {
		_, err := b.GetTransaction(flow.ZeroID)
		assert.IsType(t, &emulator.ErrTransactionNotFound{}, err)
	})

	t.Run("ValidHash", func(t *testing.T) {
		// TODO: fix once GetTransactionStatus is implemented
		// resTx, err := b.GetTransaction(tx.ID())
		// require.NoError(t, err)
		// assert.Equal(t, resTx.Status, flow.TransactionFinalized)
		//
		// require.Len(t, resTx.Events, 1)
		// actualEvent := resTx.Events[0]
		//
		// decodedEvent := actualEvent.Value
		//
		// location := runtime.AddressLocation(counterAddress.Bytes())
		// eventType := fmt.Sprintf("%s.Counting.CountIncremented", location.ID())
		//
		// assert.Equal(t, tx.ID(), actualEvent.TransactionID)
		// assert.Equal(t, eventType, actualEvent.Type)
		// assert.Equal(t, uint(0), actualEvent.EventIndex)
		// assert.Equal(t, cadence.NewInt(2), decodedEvent.Fields[0])
	})
}
