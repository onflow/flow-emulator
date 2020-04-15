package convert

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow/protobuf/go/flow/entities"
)

func MessageToCollection(m *entities.Collection) flow.LightCollection {
	transactionIDs := make([]flow.Identifier, len(m.GetTransactionIds()))
	for i, transactionID := range m.GetTransactionIds() {
		transactionIDs[i] = flow.HashToID(transactionID)
	}

	return flow.LightCollection{
		Transactions: transactionIDs,
	}
}

func CollectionToMessage(c flow.LightCollection) *entities.Collection {
	transactionIDs := make([][]byte, len(c.Transactions))

	for i, transactionID := range c.Transactions {
		transactionIDs[i] = transactionID[:]
	}

	return &entities.Collection{
		TransactionIds: transactionIDs,
	}
}
