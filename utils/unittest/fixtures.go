package unittest

import (
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/dapperlabs/flow-emulator/types"
)

func TransactionFixture(n ...func(t *flow.Transaction)) flow.Transaction {
	tx := test.TransactionGenerator().New()

	if len(n) > 0 {
		n[0](tx)
	}

	return *tx
}

func StorableTransactionResultFixture() types.StorableTransactionResult {
	events := test.EventGenerator()

	return types.StorableTransactionResult{
		ErrorCode:    42,
		ErrorMessage: "foo",
		Logs:         []string{"a", "b", "c"},
		Events:       []flow.Event{events.New(), events.New()},
	}
}
