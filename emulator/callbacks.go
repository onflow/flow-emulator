package emulator

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed templates/executeCallbackTransaction.cdc
var executeCallbackScript []byte

//go:embed templates/scheduleCallbackTransaction.cdc
var scheduleCallbackScript []byte

func scheduleCallbackTransaction() flow.TransactionBody {
	return *flow.NewTransactionBody().
		SetScript(scheduleCallbackScript)
}

func executeCallbackTransactions(scheduleEvent []flowsdk.Event) ([]flow.TransactionBody, error) {
	var transactions []flow.TransactionBody

	for _, e := range scheduleEvent {
		limit, id, err := parseSchedulerProcessedEvent(e)
		if err != nil {
			return nil, err
		}

		tx := flow.NewTransactionBody().
			SetScript(executeCallbackScript).
			AddArgument(id).
			SetComputeLimit(limit)

		transactions = append(transactions, *tx)
	}

	return transactions, nil
}

// parseSchedulerProcessedEvent parses flow event that is emitted during scheduler
// marking the callback as scheduled.
// Returns:
// - execution effort
// - ID of the event encoded as bytes
// - error in case the event type is not correct
func parseSchedulerProcessedEvent(event flowsdk.Event) (uint64, []byte, error) {
	if event.Type != "A.Scheduler.CallbackProcessed" {
		return 0, nil, fmt.Errorf("invalid event type: %s", event.Type)
	}

	id := event.Value.SearchFieldByName("ID")
	encodedID, err := jsoncdc.Encode(id)
	if err != nil {
		return 0, nil, err
	}

	effort, ok := event.Value.SearchFieldByName("computationEffort").(cadence.UInt64)
	if !ok {
		return 0, nil, fmt.Errorf("invalid effort value type: %v", effort)
	}
	computeLimit := binary.BigEndian.Uint64(effort.ToBigEndianBytes())

	return computeLimit, encodedID, nil
}
