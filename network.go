package emulator

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-go-sdk"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"google.golang.org/grpc"
)

type Network struct {
	flowClient     *client.Client
	signer         crypto.Signer
	serviceAccount *sdk.Account
	serviceKey     ServiceKey
	txQueue        []sdk.Transaction
}

// NewNetwork returns a client that conforms to BlockchainAPI
func NewNetwork(flowAccessAddress, privateKeyHex string) (*Network, error) {
	flowClient, err := client.New(flowAccessAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	// Service address is the first generated account address
	addr := sdk.ServiceAddress(sdk.Testnet)

	acc, err := flowClient.GetAccount(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	accountKey := acc.Keys[0]

	privateKey, err := crypto.DecodePrivateKeyHex(accountKey.SigAlgo, privateKeyHex)
	if err != nil {
		return nil, err
	}
	signer := crypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)

	serviceKey := ServiceKey{
		Index:          accountKey.Index,
		Address:        addr,
		SequenceNumber: accountKey.SequenceNumber,
		PrivateKey:     &privateKey,
		PublicKey:      &accountKey.PublicKey,
		HashAlgo:       accountKey.HashAlgo,
		SigAlgo:        accountKey.SigAlgo,
		Weight:         accountKey.Weight,
	}

	return &Network{
		flowClient:     flowClient,
		signer:         signer,
		serviceAccount: acc,
		serviceKey:     serviceKey,
		txQueue:        []sdk.Transaction{},
	}, nil
}

func (n *Network) AddTransaction(tx sdk.Transaction) error {
	n.txQueue = append(n.txQueue, tx)
	return nil
}
func (n *Network) ExecuteNextTransaction() (*types.TransactionResult, error) {
	ctx := context.Background()
	tx := n.txQueue[0]
	n.txQueue = n.txQueue[1:]

	err := n.flowClient.SendTransaction(ctx, tx)
	if err != nil {
		return nil, err
	}
	txResp, err := n.WaitForSeal(ctx, tx.ID())
	if err != nil {
		return nil, err
	}
	// If service account was the proposer, we have to manage the sequence number here
	if txResp.Error == nil && // TODO: remove once https://github.com/dapperlabs/flow-go/issues/4107 is done
		tx.ProposalKey.Address == n.serviceKey.Address &&
		tx.ProposalKey.KeyIndex == n.serviceKey.Index {
		n.serviceKey.SequenceNumber++
	}

	return &types.TransactionResult{
		TransactionID: tx.ID(),
		Error:         txResp.Error,
		Events:        txResp.Events,
	}, nil
}
func (n *Network) CreateAccount(publicKeys []*flow.AccountKey, code []byte) (flow.Address, error) {
	ctx := context.Background()
	addr := flow.Address{}

	for _, key := range publicKeys {
		// Reset IDs and Sequence Numbers
		key.Index = 0
		key.SequenceNumber = 0
	}

	finalizedBlock, err := n.flowClient.GetLatestBlockHeader(ctx, false)
	if err != nil {
		return addr, err
	}

	accountTx := templates.CreateAccount(publicKeys, code, n.serviceKey.Address).
		SetReferenceBlockID(finalizedBlock.ID).
		SetProposalKey(n.serviceKey.Address, n.serviceKey.Index, n.serviceKey.SequenceNumber).
		SetPayer(n.serviceKey.Address)

	err = accountTx.SignEnvelope(
		n.serviceKey.Address,
		n.serviceKey.Index,
		n.signer,
	)
	if err != nil {
		return addr, err
	}
	err = n.flowClient.SendTransaction(ctx, *accountTx)
	if err != nil {
		return addr, err
	}
	accountTxResp, err := n.WaitForSeal(ctx, accountTx.ID())
	if accountTxResp.Error != nil {
		return addr, accountTxResp.Error
	} else if err != nil {
		return addr, err
	}

	// Successful Tx, increment sequence number
	n.serviceKey.SequenceNumber++

	for _, event := range accountTxResp.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			addr = accountCreatedEvent.Address()
		}
	}
	return addr, nil
}
func (n *Network) ExecuteBlock() ([]*types.TransactionResult, error) {
	panic("not implemented")
}
func (n *Network) CommitBlock() (*flowgo.Block, error) {
	return n.GetLatestBlock()
}
func (n *Network) ExecuteAndCommitBlock() (*flowgo.Block, []*types.TransactionResult, error) {
	panic("not implemented")
}
func (n *Network) GetLatestBlock() (*flowgo.Block, error) {
	ctx := context.Background()
	block, err := n.flowClient.GetLatestBlock(ctx, true)
	if err != nil {
		return nil, err
	}
	return fromSDKBlock(block), nil
}
func (n *Network) GetLatestBlockID() (sdk.Identifier, error) {
	ctx := context.Background()
	block, err := n.flowClient.GetLatestBlock(ctx, true)
	if err != nil {
		return sdk.Identifier{}, err
	}
	return block.ID, nil
}
func (n *Network) GetBlockByID(id sdk.Identifier) (*flowgo.Block, error) {
	ctx := context.Background()
	block, err := n.flowClient.GetBlockByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return fromSDKBlock(block), nil
}
func (n *Network) GetBlockByHeight(height uint64) (*flowgo.Block, error) {
	ctx := context.Background()
	block, err := n.flowClient.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	return fromSDKBlock(block), nil
}
func (n *Network) GetCollection(colID sdk.Identifier) (*sdk.Collection, error) {
	panic("not implemented")
}
func (n *Network) GetTransaction(txID sdk.Identifier) (*sdk.Transaction, error) {
	panic("not implemented")
}
func (n *Network) GetTransactionResult(txID sdk.Identifier) (*sdk.TransactionResult, error) {
	return n.flowClient.GetTransactionResult(context.Background(), txID)
}
func (n *Network) GetAccount(address sdk.Address) (*sdk.Account, error) {
	panic("not implemented")
}
func (n *Network) GetAccountAtBlock(address sdk.Address, blockHeight uint64) (*sdk.Account, error) {
	panic("not implemented")
}
func (n *Network) GetEventsByHeight(blockHeight uint64, eventType string) ([]sdk.Event, error) {
	// panic("not implemented")
	ctx := context.Background()
	blockEvents, err := n.flowClient.GetEventsForHeightRange(ctx, client.EventRangeQuery{
		Type:        eventType,
		StartHeight: blockHeight,
		EndHeight:   blockHeight,
	})
	if err != nil {
		return nil, err
	}
	return blockEvents[0].Events, nil

}
func (n *Network) ExecuteScript(script []byte, args [][]byte) (*types.ScriptResult, error) {
	ctx := context.Background()
	arguments := []cadence.Value{}
	for _, arg := range args {
		val, err := jsoncdc.Decode(arg)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, val)
	}
	res, err := n.flowClient.ExecuteScriptAtLatestBlock(ctx, script, arguments)
	return &types.ScriptResult{
		Value: res,
		Error: err,
	}, nil
}
func (n *Network) ExecuteScriptAtBlock(script []byte, args [][]byte, blockHeight uint64) (*types.ScriptResult, error) {
	panic("not implemented")
}
func (n *Network) ServiceKey() ServiceKey {
	return n.serviceKey
}

func (n *Network) WaitForSeal(ctx context.Context, id flow.Identifier) (*flow.TransactionResult, error) {
	result, err := n.flowClient.GetTransactionResult(ctx, id)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Waiting for transaction %s to be sealed...\n", id)

	for result.Status != flow.TransactionStatusSealed {
		time.Sleep(time.Second)
		fmt.Print(".")
		result, err = n.flowClient.GetTransactionResult(ctx, id)
		if err != nil {
			return nil, err
		}
	}

	fmt.Println()
	fmt.Printf("Transaction %s sealed\n", id)

	return result, nil
}

func fromSDKBlock(block *sdk.Block) *flowgo.Block {
	// TODO: Add payload body
	return &flowgo.Block{
		Header: &flowgo.Header{
			ParentID:  flowgo.Identifier(block.ParentID),
			Height:    block.Height,
			Timestamp: block.Timestamp,
		},
	}
}
