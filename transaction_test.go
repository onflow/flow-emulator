package emulator_test

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow-go/fvm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
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
func TestSubmitTransaction_Invalid(t *testing.T) {

	t.Run("Empty transaction", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		t.Skip("TODO: transaction validation")

		// Create empty transaction (no required fields)
		tx := flow.NewTransaction()

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing script", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		// Create transaction with no Script field
		tx := flow.NewTransaction().
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing script", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		// Create transaction with invalid Script field
		tx := flow.NewTransaction().
			SetScript([]byte("this script cannot be parsed")).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.InvalidTransactionScriptError{}, err)
	})

	t.Run("Missing gas limit", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// Create transaction with no GasLimit field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Missing payer account", func(t *testing.T) {
		t.Skip("TODO: transaction validation")

		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// Create transaction with no PayerAccount field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetGasLimit(emulator.MaxGasLimit)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing proposal key", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// Create transaction with no PayerAccount field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(emulator.MaxGasLimit)

		tx.ProposalKey = flow.ProposalKey{}

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Invalid sequence number", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		invalidSequenceNumber := b.ServiceKey().SequenceNumber + 2137
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetPayer(b.ServiceKey().Address).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, invalidSequenceNumber).
			SetGasLimit(emulator.MaxGasLimit).
			AddAuthorizer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		spew.Dump(result)

		require.Error(t, result.Error)

		assert.IsType(t, &types.FlowError{}, result.Error)
		assert.IsType(t, &fvm.InvalidProposalKeySequenceNumberError{}, result.Error.(*types.FlowError).FlowError)
		assert.Equal(t, invalidSequenceNumber, result.Error.(*types.FlowError).FlowError.(*fvm.InvalidProposalKeySequenceNumberError).ProvidedSeqNumber)
	})

	const expiry = 10

	t.Run("Missing reference block ID", func(t *testing.T) {
		b, err := emulator.NewBlockchain(
			emulator.WithTransactionExpiry(expiry),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Expired transaction", func(t *testing.T) {
		b, err := emulator.NewBlockchain(
			emulator.WithTransactionExpiry(expiry),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		expiredBlock, err := b.GetLatestBlock()
		require.NoError(t, err)

		// commit blocks until expiry window is exceeded
		for i := 0; i < expiry+1; i++ {
			_, _, err := b.ExecuteAndCommitBlock()
			require.NoError(t, err)
		}

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetReferenceBlockID(flow.Identifier(expiredBlock.ID())).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.ExpiredTransactionError{}, err)
	})
}

func TestSubmitTransaction_Duplicate(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
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

func TestSubmitTransaction_Reverted(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	tx := flow.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
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

func TestSubmitTransaction_Authorizers(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	accountKeys := test.AccountKeyGenerator()

	accountKeyB, signerB := accountKeys.NewWithSigner()
	accountKeyB.SetWeight(flow.AccountKeyWeightThreshold)

	accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, nil)
	assert.NoError(t, err)

	t.Run("Extra authorizers", func(t *testing.T) {
		// script only supports one account
		script := []byte(`
		  transaction {
		    prepare(signer: AuthAccount) {}
		  }
		`)

		// create transaction with two authorizing accounts
		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assert.True(t, result.Reverted())

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})

	t.Run("Insufficient authorizers", func(t *testing.T) {
		// script requires two accounts
		script := []byte(`
		  transaction {
		    prepare(signerA: AuthAccount, signerB: AuthAccount) {}
		  }
		`)

		// create transaction with two accounts
		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
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

func TestSubmitTransaction_EnvelopeSignature(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	t.Run("Missing envelope signature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(addressA).
			AddAuthorizer(b.ServiceKey().Address)

		err = tx.SignPayload(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvm.MissingSignatureError{}, result.Error)
	})

	t.Run("Invalid account", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		invalidAddress := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.NewTransaction().
			SetScript([]byte(`transaction { execute { } }`)).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(invalidAddress)

		err = tx.SignPayload(invalidAddress, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvm.InvalidSignaturePublicKeyDoesNotExistError{}, result.Error)
	})

	t.Run("Invalid key", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// use key that does not exist on service account
		invalidKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256,
			[]byte("invalid key invalid key invalid key invalid key invalid key invalid key"))
		invalidSigner := crypto.NewNaiveSigner(invalidKey, crypto.SHA3_256)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, invalidSigner)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvm.InvalidSignatureVerificationError{}, result.Error)
	})

	t.Run("Key weights", func(t *testing.T) {
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
			SetGasLimit(emulator.MaxGasLimit).
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

		t.Run("Insufficient key weight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			unittest.AssertFVMErrorType(t, &fvm.MissingSignatureError{}, result.Error)
		})

		t.Run("Sufficient key weight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			assertTransactionSucceeded(t, result)
		})
	})
}

func TestSubmitTransaction_PayloadSignatures(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	t.Run("Missing payload signature", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(addressA)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvm.MissingSignatureError{}, result.Error)
	})

	t.Run("Multiple payload signers", func(t *testing.T) {
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
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		assert.Contains(t,
			result.Logs,
			interpreter.NewAddressValueFromBytes(b.ServiceKey().Address.Bytes()).String(),
		)

		assert.Contains(t,
			result.Logs,
			interpreter.NewAddressValueFromBytes(accountAddressB.Bytes()).String(),
		)
	})
}

func TestSubmitTransaction_Arguments(t *testing.T) {
	addresses := test.AddressGenerator()

	fix64Value, _ := cadence.NewFix64("123456.00000")
	uFix64Value, _ := cadence.NewUFix64("123456.00000")

	var tests = []struct {
		argType cadence.Type
		arg     cadence.Value
	}{
		{
			cadence.BoolType{},
			cadence.NewBool(true),
		},
		{
			cadence.StringType{},
			cadence.NewString("foo"),
		},
		{
			cadence.AddressType{},
			cadence.NewAddress(addresses.New()),
		},
		{
			cadence.IntType{},
			cadence.NewInt(42),
		},
		{
			cadence.Int8Type{},
			cadence.NewInt8(42),
		},
		{
			cadence.Int16Type{},
			cadence.NewInt16(42),
		},
		{
			cadence.Int32Type{},
			cadence.NewInt32(42),
		},
		{
			cadence.Int64Type{},
			cadence.NewInt64(42),
		},
		{
			cadence.Int128Type{},
			cadence.NewInt128(42),
		},
		{
			cadence.Int256Type{},
			cadence.NewInt256(42),
		},
		{
			cadence.UIntType{},
			cadence.NewUInt(42),
		},
		{
			cadence.UInt8Type{},
			cadence.NewUInt8(42),
		},
		{
			cadence.UInt16Type{},
			cadence.NewUInt16(42),
		},
		{
			cadence.UInt32Type{},
			cadence.NewUInt32(42),
		},
		{
			cadence.UInt64Type{},
			cadence.NewUInt64(42),
		},
		{
			cadence.UInt128Type{},
			cadence.NewUInt128(42),
		},
		{
			cadence.UInt256Type{},
			cadence.NewUInt256(42),
		},
		{
			cadence.Word8Type{},
			cadence.NewWord8(42),
		},
		{
			cadence.Word16Type{},
			cadence.NewWord16(42),
		},
		{
			cadence.Word32Type{},
			cadence.NewWord32(42),
		},
		{
			cadence.Word64Type{},
			cadence.NewWord64(42),
		},
		{
			cadence.Fix64Type{},
			fix64Value,
		},
		{
			cadence.UFix64Type{},
			uFix64Value,
		},
		{
			cadence.ConstantSizedArrayType{
				Size:        3,
				ElementType: cadence.IntType{},
			},
			cadence.NewArray([]cadence.Value{
				cadence.NewInt(1),
				cadence.NewInt(2),
				cadence.NewInt(3),
			}),
		},
		{
			cadence.DictionaryType{
				KeyType:     cadence.StringType{},
				ElementType: cadence.IntType{},
			},
			cadence.NewDictionary([]cadence.KeyValuePair{
				{
					Key:   cadence.NewString("a"),
					Value: cadence.NewInt(1),
				},
				{
					Key:   cadence.NewString("b"),
					Value: cadence.NewInt(2),
				},
				{
					Key:   cadence.NewString("c"),
					Value: cadence.NewInt(3),
				},
			}),
		},
	}

	var script = func(argType cadence.Type) []byte {
		return []byte(fmt.Sprintf(`
            transaction(x: %s) {
              execute {
                log(x)
              }
            }
		`, argType.ID()))
	}

	for _, tt := range tests {
		t.Run(tt.argType.ID(), func(t *testing.T) {
			t.Parallel()

			b, err := emulator.NewBlockchain()
			require.NoError(t, err)

			tx := flow.NewTransaction().
				SetScript(script(tt.argType)).
				SetGasLimit(emulator.MaxGasLimit).
				SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
				SetPayer(b.ServiceKey().Address)

			err = tx.AddArgument(tt.arg)
			assert.NoError(t, err)

			err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
			assert.NoError(t, err)

			err = b.AddTransaction(*tx)
			assert.NoError(t, err)

			result, err := b.ExecuteNextTransaction()
			require.NoError(t, err)
			assertTransactionSucceeded(t, result)

			assert.Len(t, result.Logs, 1)
		})
	}

	t.Run("Log", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		script := []byte(`
          transaction(x: Int) {
            execute {
              log(x * 6)
            }
          }
		`)

		x := 7

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.AddArgument(cadence.NewInt(x))
		assert.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		require.Len(t, result.Logs, 1)
		assert.Equal(t, "42", result.Logs[0])
	})
}

func TestGetTransaction(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
	assert.NoError(t, err)

	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	t.Run("Nonexistent", func(t *testing.T) {
		_, err := b.GetTransaction(flow.EmptyID)
		if assert.Error(t, err) {
			assert.IsType(t, &emulator.TransactionNotFoundError{}, err)
		}
	})

	t.Run("Existent", func(t *testing.T) {
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
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
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

	location := runtime.AddressContractLocation{
		AddressLocation: runtime.AddressLocation(counterAddress.Bytes()),
		Name:            "Counting",
	}
	eventType := fmt.Sprintf("%s.Counting.CountIncremented", location.ID())

	assert.Equal(t, tx.ID(), event.TransactionID)
	assert.Equal(t, eventType, event.Type)
	assert.Equal(t, 0, event.EventIndex)
	assert.Equal(t, cadence.NewInt(2), event.Value.Fields[0])
}

const helloWorldContract = `
    pub contract HelloWorld {

        pub fun hello(): String {
            return "Hello, World!"
        }
    }
`

const callHelloTxTemplate = `
    import HelloWorld from 0x%s
    transaction {
        prepare() {
            assert(HelloWorld.hello() == "Hello, World!")
        }
    }
`

func TestHelloWorld_NewAccount(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	accountKey, accountSigner := accountKeys.NewWithSigner()

	contracts := []templates.Contract{
		{
			Name:   "HelloWorld",
			Source: helloWorldContract,
		},
	}

	createAccountTx := templates.CreateAccount(
		[]*flow.AccountKey{accountKey},
		contracts,
		b.ServiceKey().Address,
	)

	createAccountTx.SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	err = createAccountTx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
	assert.NoError(t, err)

	err = b.AddTransaction(*createAccountTx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// createAccountTx status becomes TransactionStatusSealed
	createAccountTxResult, err := b.GetTransactionResult(createAccountTx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, createAccountTxResult.Status)

	var newAccountAddress flow.Address
	for _, event := range createAccountTxResult.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			newAccountAddress = accountCreatedEvent.Address()
			break
		}

		assert.Fail(t, "missing account created event")
	}

	t.Logf("new account address: 0x%s", newAccountAddress.Hex())

	account, err := b.GetAccount(newAccountAddress)
	assert.NoError(t, err)

	assert.Equal(t, newAccountAddress, account.Address)

	// call hello world code

	accountKey = account.Keys[0]

	callHelloCode := []byte(fmt.Sprintf(callHelloTxTemplate, newAccountAddress.Hex()))
	callHelloTx := flow.NewTransaction().
		SetScript(callHelloCode).
		SetProposalKey(newAccountAddress, accountKey.Index, accountKey.SequenceNumber).
		SetPayer(newAccountAddress)

	err = callHelloTx.SignEnvelope(newAccountAddress, accountKey.Index, accountSigner)
	assert.NoError(t, err)

	err = b.AddTransaction(*callHelloTx)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestHelloWorld_UpdateAccount(t *testing.T) {
	accountKeys := test.AccountKeyGenerator()

	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	accountKey, accountSigner := accountKeys.NewWithSigner()
	_ = accountSigner

	contracts := []templates.Contract{
		{
			Name: "HelloWorld",
			Source: `pub contract HelloWorld {}`,
		},
	}

	createAccountTx := templates.CreateAccount(
		[]*flow.AccountKey{accountKey},
		contracts,
		b.ServiceKey().Address,
	)

	createAccountTx.
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	err = createAccountTx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
	assert.NoError(t, err)

	err = b.AddTransaction(*createAccountTx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// createAccountTx status becomes TransactionStatusSealed
	createAccountTxResult, err := b.GetTransactionResult(createAccountTx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, createAccountTxResult.Status)

	var newAccountAddress flow.Address
	for _, event := range createAccountTxResult.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			newAccountAddress = accountCreatedEvent.Address()
			break
		}

		assert.Fail(t, "missing account created event")
	}

	t.Logf("new account address: 0x%s", newAccountAddress.Hex())

	account, err := b.GetAccount(newAccountAddress)
	assert.NoError(t, err)

	accountKey = account.Keys[0]

	updateAccountCodeTx := templates.UpdateAccountContract(
		newAccountAddress,
		"HelloWorld",
		[]byte(helloWorldContract),
	)
	updateAccountCodeTx.
		SetGasLimit(emulator.MaxGasLimit).
		SetProposalKey(newAccountAddress, accountKey.Index, accountKey.SequenceNumber).
		SetPayer(newAccountAddress)

	err = updateAccountCodeTx.SignEnvelope(newAccountAddress, accountKey.Index, accountSigner)
	assert.NoError(t, err)

	err = b.AddTransaction(*updateAccountCodeTx)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// call hello world code

	accountKey.SequenceNumber++

	callHelloCode := []byte(fmt.Sprintf(callHelloTxTemplate, newAccountAddress.Hex()))
	callHelloTx := flow.NewTransaction().
		SetScript(callHelloCode).
		SetProposalKey(newAccountAddress, accountKey.Index, accountKey.SequenceNumber).
		SetPayer(newAccountAddress)

	err = callHelloTx.SignEnvelope(newAccountAddress, accountKey.Index, accountSigner)
	assert.NoError(t, err)

	err = b.AddTransaction(*callHelloTx)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}
