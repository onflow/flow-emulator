package unittest

import (
	flowgo "github.com/dapperlabs/flow-go/model/flow"
	flowUnittest "github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/dapperlabs/flow-go/utils/unittest/generator"

	"github.com/dapperlabs/flow-emulator/types"
)

func TransactionFixture(n ...func(t *flowgo.TransactionBody)) flowgo.TransactionBody {

	tx := flowUnittest.TransactionBodyFixture()

	for _, f := range n {
		f(&tx)
	}

	return tx
}

func StorableTransactionResultFixture() types.StorableTransactionResult {
	events := generator.EventGenerator()

	return types.StorableTransactionResult{
		ErrorCode:    42,
		ErrorMessage: "foo",
		Logs:         []string{"a", "b", "c"},
		Events:       []flowgo.Event{events.New(), events.New()},
	}
}
