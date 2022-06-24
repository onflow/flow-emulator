package emulator_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	convert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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

	t.Parallel()

	t.Run("Empty transaction", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		// Create empty transaction (no required fields)
		tx := flow.NewTransaction()

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing script", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		// Create transaction with no Script field
		tx := flow.NewTransaction().
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing script", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		// Create transaction with invalid Script field
		tx := flow.NewTransaction().
			SetScript([]byte("this script cannot be parsed")).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.InvalidTransactionScriptError{}, err)
	})

	t.Run("Missing gas limit", func(t *testing.T) {

		t.Parallel()

		t.Skip("TODO: transaction validation")

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// Create transaction with no GasLimit field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Missing payer account", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// Create transaction with no PayerAccount field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing proposal key", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// Create transaction with no PayerAccount field
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit)

		tx.ProposalKey = flow.ProposalKey{}

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Invalid sequence number", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		invalidSequenceNumber := b.ServiceKey().SequenceNumber + 2137
		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetPayer(b.ServiceKey().Address).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, invalidSequenceNumber).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)

		require.Error(t, result.Error)

		assert.IsType(t, &types.FlowError{}, result.Error)
		assert.IsType(t, &fvmerrors.InvalidProposalSeqNumberError{}, result.Error.(*types.FlowError).FlowError)
		assert.Equal(t, invalidSequenceNumber, result.Error.(*types.FlowError).FlowError.(*fvmerrors.InvalidProposalSeqNumberError).ProvidedSeqNumber())
	})

	const expiry = 10

	t.Run("Missing reference block ID", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithTransactionExpiry(expiry),
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Expired transaction", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithTransactionExpiry(expiry),
			emulator.WithStorageLimitEnabled(false),
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
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.ExpiredTransactionError{}, err)
	})

	t.Run("Invalid hash algorithm proposer", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		invalidSigner, err := crypto.NewNaiveSigner(b.ServiceKey().PrivateKey, crypto.SHA2_256)
		require.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, invalidSigner)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		pk, _ := convert.SDKAccountKeyToFlow(b.ServiceKey().AccountKey())
		assert.Equal(t, types.NewTransactionInvalidHashAlgo(
			pk, convert.SDKAddressToFlow(b.ServiceKey().Address), crypto.SHA2_256,
		), result.Debug)
	})

	t.Run("Invalid hash algorithm authorizer", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		pk, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("invalid key invalid key invalid key invalid key invalid key invalid key"))
		assert.NoError(t, err)

		accountKeyB := (&flow.AccountKey{}).FromPrivateKey(pk)
		accountKeyB.HashAlgo = crypto.SHA3_256
		accountKeyB.Weight = flow.AccountKeyWeightThreshold

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, nil)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript([]byte(`
			  transaction {
				prepare(signer: AuthAccount) {}
			  }
			`)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(accountAddressB)

		invalidSigner, err := crypto.NewNaiveSigner(pk, crypto.SHA2_256)
		require.NoError(t, err)

		err = tx.SignPayload(accountAddressB, 0, invalidSigner)
		require.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		key := flowgo.AccountPublicKey{
			Index:     0,
			PublicKey: nil,
			SignAlgo:  0,
			HashAlgo:  crypto.SHA3_256,
			SeqNumber: 0,
			Weight:    0,
			Revoked:   false,
		}
		assert.Equal(t, types.NewTransactionInvalidHashAlgo(
			key, convert.SDKAddressToFlow(accountAddressB), crypto.SHA2_256,
		), result.Debug)
	})

	t.Run("Invalid signature for provided data", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		tx.SetGasLimit(100) // change data after signing

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		debug := types.NewTransactionInvalidSignature(&flowgo.TransactionBody{
			ReferenceBlockID: flowgo.Identifier{},
			Script:           nil,
			Arguments:        nil,
			GasLimit:         flowgo.DefaultMaxTransactionGasLimit,
			ProposalKey: flowgo.ProposalKey{
				Address:        convert.SDKAddressToFlow(b.ServiceKey().Address),
				KeyIndex:       uint64(b.ServiceKey().Index),
				SequenceNumber: b.ServiceKey().SequenceNumber,
			},
			Payer:              convert.SDKAddressToFlow(b.ServiceKey().Address),
			Authorizers:        convert.SDKAddressesToFlow([]flow.Address{b.ServiceKey().Address}),
			PayloadSignatures:  nil,
			EnvelopeSignatures: nil,
		})

		assert.NotNil(t, result.Error)
		assert.IsType(t, result.Debug, debug)
	})
}

func TestSubmitTransaction_Duplicate(t *testing.T) {

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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

	t.Parallel()

	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	tx := flow.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
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
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

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
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

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

	t.Parallel()

	t.Run("Missing envelope signature", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignPayload(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvmerrors.AccountAuthorizationError{}, result.Error)
	})

	t.Run("Invalid account", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addresses := flow.NewAddressGenerator(flow.Emulator)
		for {
			_, err := b.GetAccount(addresses.NextAddress())
			if err != nil {
				break
			}
		}

		nonExistentAccountAddress := addresses.Address()

		tx := flow.NewTransaction().
			SetScript([]byte(`transaction { execute { } }`)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(nonExistentAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignPayload(nonExistentAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		var sigErr *fvmerrors.InvalidProposalSignatureError
		assert.True(t, errors.As(result.Error, &sigErr))
	})

	t.Run("Invalid key", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// use key that does not exist on service account
		invalidKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256,
			[]byte("invalid key invalid key invalid key invalid key invalid key invalid key"))
		invalidSigner, err := crypto.NewNaiveSigner(invalidKey, crypto.SHA3_256)
		require.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, invalidSigner)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		var sigErr *fvmerrors.InvalidProposalSignatureError
		assert.True(t, errors.As(result.Error, &sigErr))
	})

	t.Run("Key weights", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		accountKeys := test.AccountKeyGenerator()

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
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(accountAddressA, 1, 0).
			SetPayer(accountAddressA).
			AddAuthorizer(accountAddressA)

		// Insufficient keys
		err = tx.SignEnvelope(accountAddressA, 1, signerB)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		// Add key so we have sufficient keys
		err = tx.SignEnvelope(accountAddressA, 0, signerA)
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		t.Run("Insufficient key weight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			unittest.AssertFVMErrorType(t, &fvmerrors.AccountAuthorizationError{}, result.Error)
		})

		t.Run("Sufficient key weight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			assertTransactionSucceeded(t, result)
		})
	})
}

func TestSubmitTransaction_PayloadSignatures(t *testing.T) {

	t.Parallel()

	t.Run("Missing payload signature", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		// create a new account,
		// authorizer must be different from payer

		accountKeys := test.AccountKeyGenerator()

		accountKeyB, _ := accountKeys.NewWithSigner()
		accountKeyB.SetWeight(flow.AccountKeyWeightThreshold)

		accountAddressB, err := b.CreateAccount([]*flow.AccountKey{accountKeyB}, nil)
		assert.NoError(t, err)

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(accountAddressB)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		unittest.AssertFVMErrorType(t, &fvmerrors.AccountAuthorizationError{}, result.Error)
	})

	t.Run("Multiple payload signers", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		accountKeys := test.AccountKeyGenerator()

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
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		assert.Contains(t,
			result.Logs,
			interpreter.NewUnmeteredAddressValueFromBytes(b.ServiceKey().Address.Bytes()).String(),
		)

		assert.Contains(t,
			result.Logs,
			interpreter.NewUnmeteredAddressValueFromBytes(accountAddressB.Bytes()).String(),
		)
	})
}

func TestSubmitTransaction_Arguments(t *testing.T) {

	t.Parallel()

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
			cadence.String("foo"),
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
					Key:   cadence.String("a"),
					Value: cadence.NewInt(1),
				},
				{
					Key:   cadence.String("b"),
					Value: cadence.NewInt(2),
				},
				{
					Key:   cadence.String("c"),
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
				SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
				SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
				SetPayer(b.ServiceKey().Address)

			err = tx.AddArgument(tt.arg)
			assert.NoError(t, err)

			signer, err := b.ServiceKey().Signer()
			require.NoError(t, err)

			err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
			require.NoError(t, err)

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
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.AddArgument(cadence.NewInt(x))
		assert.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assertTransactionSucceeded(t, result)

		require.Len(t, result.Logs, 1)
		assert.Equal(t, "42", result.Logs[0])
	})
}

func TestSubmitTransaction_ProposerSequence(t *testing.T) {

	t.Parallel()

	t.Run("Valid transaction increases sequence number", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		script := []byte(`
		  transaction {
		    prepare(signer: AuthAccount) {}
		  }
		`)
		prevSeq := b.ServiceKey().SequenceNumber

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		tx1Result, err := b.GetTransactionResult(tx.ID())
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusSealed, tx1Result.Status)

		assert.Equal(t, prevSeq+1, b.ServiceKey().SequenceNumber)
	})

	t.Run("Reverted transaction increases sequence number", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.NewBlockchain(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		prevSeq := b.ServiceKey().SequenceNumber
		script := []byte(`
		  transaction {
			prepare(signer: AuthAccount) {} 
			execute { panic("revert!") }
		  }
		`)

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		_, err = b.ExecuteNextTransaction()
		assert.NoError(t, err)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		tx1Result, err := b.GetTransactionResult(tx.ID())
		assert.NoError(t, err)
		assert.Equal(t, prevSeq+1, b.ServiceKey().SequenceNumber)
		assert.Equal(t, flow.TransactionStatusSealed, tx1Result.Status)
		assert.Len(t, tx1Result.Events, 0)
		assert.IsType(t, &emulator.ExecutionError{}, tx1Result.Error)
	})
}

func TestGetTransaction(t *testing.T) {

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	addTwoScript, counterAddress := deployAndGenerateAddTwoScript(t, b)

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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

	addr, _ := common.BytesToAddress(counterAddress.Bytes())
	location := common.AddressLocation{
		Address: addr,
		Name:    "Counting",
	}
	eventType := location.TypeID(nil, "Counting.CountIncremented")

	assert.Equal(t, tx.ID(), event.TransactionID)
	assert.Equal(t, string(eventType), event.Type)
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

	t.Parallel()

	accountKeys := test.AccountKeyGenerator()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	accountKey, accountSigner := accountKeys.NewWithSigner()

	contracts := []templates.Contract{
		{
			Name:   "HelloWorld",
			Source: helloWorldContract,
		},
	}

	createAccountTx, err := templates.CreateAccount(
		[]*flow.AccountKey{accountKey},
		contracts,
		b.ServiceKey().Address,
	)
	require.NoError(t, err)

	createAccountTx.SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = createAccountTx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvent := flow.AccountCreatedEvent(event)
		newAccountAddress = accountCreatedEvent.Address()
		break
	}

	if newAccountAddress == flow.EmptyAddress {
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
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
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

	t.Parallel()

	accountKeys := test.AccountKeyGenerator()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	accountKey, accountSigner := accountKeys.NewWithSigner()
	_ = accountSigner

	contracts := []templates.Contract{
		{
			Name:   "HelloWorld",
			Source: `pub contract HelloWorld {}`,
		},
	}

	createAccountTx, err := templates.CreateAccount(
		[]*flow.AccountKey{accountKey},
		contracts,
		b.ServiceKey().Address,
	)
	assert.NoError(t, err)

	createAccountTx.
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = createAccountTx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvent := flow.AccountCreatedEvent(event)
		newAccountAddress = accountCreatedEvent.Address()
		break
	}

	if newAccountAddress == flow.EmptyAddress {
		assert.Fail(t, "missing account created event")
	}

	t.Logf("new account address: 0x%s", newAccountAddress.Hex())

	account, err := b.GetAccount(newAccountAddress)
	assert.NoError(t, err)

	accountKey = account.Keys[0]

	updateAccountCodeTx := templates.UpdateAccountContract(
		newAccountAddress,
		templates.Contract{
			Name:   "HelloWorld",
			Source: helloWorldContract,
		},
	)
	updateAccountCodeTx.
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
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
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
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

func TestInfiniteTransaction(t *testing.T) {

	t.Parallel()

	const limit = 1000

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
		emulator.WithTransactionMaxGasLimit(limit),
	)
	require.NoError(t, err)

	const code = `
      pub fun test() {
          test()
      }

      transaction {
          execute {
              test()
          }
      }
    `

	// Create a new account

	accountKeys := test.AccountKeyGenerator()
	accountKey, signer := accountKeys.NewWithSigner()
	accountAddress, err := b.CreateAccount([]*flow.AccountKey{accountKey}, nil)
	assert.NoError(t, err)

	// Sign the transaction using the new account.
	// Do not test using the service account,
	// as the computation limit is disabled for it

	tx := flow.NewTransaction().
		SetScript([]byte(code)).
		SetGasLimit(limit).
		SetProposalKey(accountAddress, 0, 0).
		SetPayer(accountAddress)

	err = tx.SignEnvelope(accountAddress, 0, signer)
	assert.NoError(t, err)

	// Submit tx
	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	// Execute tx
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)

	require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
}

func TestSubmitTransactionWithCustomLogger(t *testing.T) {

	t.Parallel()
	var memlog bytes.Buffer
	memlogWrite := io.Writer(&memlog)
	logger := zerolog.New(memlogWrite).Level(zerolog.DebugLevel)

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
		emulator.WithLogger(logger),
	)
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

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

	var meter Meter
	scanner := bufio.NewScanner(&memlog)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "transaction execution data") {
			err = json.Unmarshal([]byte(txt), &meter)
		}
	}

	assert.NoError(t, err)
	assert.Equal(t, `{LedgerInteractionUsed:9373 ComputationUsed:18 MemoryUsed:0 ComputationIntensities:map[Statement:10 Loop:1 FunctionInvocation:8 CreateCompositeValue:2 TransferCompositeValue:3 CreateArrayValue:4 TransferArrayValue:446 CreateDictionaryValue:1 ComputationKind(2005):1 ComputationKind(2006):1 ComputationKind(2007):1 ComputationKind(2008):1 ComputationKind(2011):2 ComputationKind(2015):2 ComputationKind(2017):2 ComputationKind(2020):4760 ComputationKind(2022):1 ComputationKind(2025):3 ComputationKind(2026):1621 ComputationKind(2027):1] MemoryIntensities:map[]}`, fmt.Sprintf("%+v", meter))

}

type Meter struct {
	LedgerInteractionUsed  int                           `json:"ledgerInteractionUsed"`
	ComputationUsed        int                           `json:"computationUsed"`
	MemoryUsed             int                           `json:"memoryUsed"`
	ComputationIntensities MeteredComputationIntensities `json:"computationIntensities"`
	MemoryIntensities      MeteredMemoryIntensities      `json:"memoryIntensities"`
}

type MeteredComputationIntensities map[common.ComputationKind]uint
type MeteredMemoryIntensities map[common.MemoryKind]uint
