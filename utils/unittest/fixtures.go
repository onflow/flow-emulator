package unittest

import (
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go-sdk/test"

	convert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/onflow/flow-emulator/types"
)

func TransactionFixture() flowgo.TransactionBody {
	return *convert.SDKTransactionToFlow(*test.TransactionGenerator().New())
}

func StorableTransactionResultFixture() types.StorableTransactionResult {
	events := test.EventGenerator()

	eventA, _ := convert.SDKEventToFlow(events.New())
	eventB, _ := convert.SDKEventToFlow(events.New())

	return types.StorableTransactionResult{
		ErrorCode:    42,
		ErrorMessage: "foo",
		Logs:         []string{"a", "b", "c"},
		Events: []flowgo.Event{
			eventA,
			eventB,
		},
	}
}
