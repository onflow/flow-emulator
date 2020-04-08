package examples

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/cadence/runtime/cmd"
	"github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/dapperlabs/flow-go-sdk/templates"

	emulator "github.com/dapperlabs/flow-emulator"
)

// ReadFile reads a file from the file system
func ReadFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// GetNonce returns a nonce value that is guaranteed to be unique.
var GetNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()

// RandomPrivateKey returns a randomly generated private key
func RandomPrivateKey() flow.AccountPrivateKey {
	seed := make([]byte, 40)
	rand.Read(seed)

	privateKey, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

// NewEmulator returns a emulator object for testing
func NewEmulator() *emulator.Blockchain {
	b, err := emulator.NewBlockchain()
	if err != nil {
		panic(err)
	}
	return b
}

func RootAccount() (flow.Address, flow.AccountPrivateKey) {
	privateKeyHex := "f87db87930770201010420c2e6c8cb9e8c9b9a7afe1df8ae431e68317ff7a9f42f8982b7877a9da76b28a7a00a06082a8648ce3d030107a14403420004c2c482bf01344a085af036f9413dd17a0d98a5b6fb4915c3ad4c3cb574e03ea5e2d47608093a26081c165722621bf9d8ff4b880cac0e7c586af3d86c0818a4af0203"
	privateKey := keys.MustDecodePrivateKeyHex(privateKeyHex)

	// root account always has address 0x01
	addr := flow.HexToAddress("01")

	return addr, privateKey
}

// SignAndSubmit signs a transaction with an array of signers and adds their signatures to the transaction
// Then submits the transaction to the emulator. If the private keys don't match up with the addresses,
// the transaction will not succeed.
// shouldRevert parameter indicates whether the transaction should fail or not
// This function asserts the correct result and commits the block if it passed
func SignAndSubmit(
	t *testing.T,
	b *emulator.Blockchain,
	tx *flow.Transaction,
	signingKeys []flow.AccountPrivateKey,
	signingAddresses []flow.Address,
	shouldRevert bool,
) {
	// sign transaction with each signer
	for i, address := range signingAddresses {
		signingKey := signingKeys[i]
		err := tx.SignContainer(address, signingKey.ToAccountKey().ID, signingKey.Signer())
		assert.NoError(t, err)
	}

	// submit the signed transaction
	err := b.AddTransaction(*tx)
	require.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	require.NoError(t, err)

	if shouldRevert {
		assert.True(t, result.Reverted())
	} else {
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
			cmd.PrettyPrintError(result.Error, "", map[string]string{"": ""})
		}
	}

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func CreateAccount() (flow.AccountPrivateKey, flow.Address) {
	privateKey := RandomPrivateKey()
	publicKey := privateKey.ToAccountKey()
	publicKey.Weight = keys.PublicKeyWeightThreshold

	addr := createAccount(
		[]flow.AccountKey{publicKey},
		nil,
	)

	return privateKey, addr
}

func DeployContract(code []byte) flow.Address {
	return createAccount(nil, code)
}

func createAccount(publicKeys []flow.AccountKey, code []byte) flow.Address {
	ctx := context.Background()
	flowClient, err := client.New("127.0.0.1:3569")
	Handle(err)

	rootAcctAddr, rootAcctKey := RootAccount()

	createAccountScript, err := templates.CreateAccount(publicKeys, code)
	Handle(err)

	createAccountTx := flow.NewTransaction().
		SetScript(createAccountScript).
		SetGasLimit(10).
		SetPayer(rootAcctAddr, 0)

	err = createAccountTx.SignContainer(rootAcctAddr, rootAcctKey.ToAccountKey().ID, rootAcctKey.Signer())
	Handle(err)

	err = flowClient.SendTransaction(ctx, *createAccountTx)
	Handle(err)

	// TODO: replace once GetTransactionStatus is implemented
	// tx := WaitForSeal(ctx, flowClient, createAccountTx.ID())
	// accountCreatedEvent := flow.AccountCreatedEvent(tx.Events[0])
	//
	// return accountCreatedEvent.Address()

	return flow.Address{}
}

func Handle(err error) {
	if err != nil {
		fmt.Println("err:", err.Error())
		panic(err)
	}
}

func WaitForSeal(ctx context.Context, c *client.Client, id flow.Identifier) *flow.Transaction {
	tx, err := c.GetTransaction(ctx, id)
	Handle(err)

	fmt.Printf("Waiting for transaction %x to be sealed...\n", id)

	// TODO: replace once GetTransactionStatus is implemented
	// for tx.Status != flow.TransactionSealed {
	// 	time.Sleep(time.Second)
	// 	fmt.Print(".")
	// 	tx, err = c.GetTransaction(ctx, hash)
	// 	Handle(err)
	// }

	fmt.Println()
	fmt.Printf("Transaction %x sealed\n", id)

	return tx
}

// setupUsersTokens sets up two accounts with 30 Fungible Tokens each
// and a NFT collection with 1 NFT each
func setupUsersTokens(
	t *testing.T,
	b *emulator.Blockchain,
	tokenAddr flow.Address,
	nftAddr flow.Address,
	signingKeys []flow.AccountPrivateKey,
	signingAddresses []flow.Address,
) {
	// add array of signers to transaction
	for i := 0; i < len(signingAddresses); i++ {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateTokenScript(tokenAddr, 30)).
			SetGasLimit(20).
			SetPayer(b.RootKey().Address, b.RootKey().ID).
			AddAuthorizer(signingAddresses[i], 0)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey(), signingKeys[i]}, []flow.Address{b.RootAccountAddress(), signingAddresses[i]}, false)

		// then deploy a NFT to the accounts
		tx = flow.NewTransaction().
			SetScript(GenerateCreateNFTScript(nftAddr, i+1)).
			SetGasLimit(20).
			SetPayer(b.RootKey().Address, b.RootKey().ID).
			AddAuthorizer(signingAddresses[i], 0)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey(), signingKeys[i]}, []flow.Address{b.RootAccountAddress(), signingAddresses[i]}, false)
	}
}
