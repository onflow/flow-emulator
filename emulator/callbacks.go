package emulator

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed templates/executeCallbackTransaction.cdc
var executeCallbackScript []byte

//go:embed templates/processCallbackTransaction.cdc
var processCallbackScript []byte

func processCallbackTransaction(
	serviceAddress flow.Address,
	parentID flow.Identifier,
) flow.TransactionBody {
	script := replaceSchedulerAddress(processCallbackScript, serviceAddress)

	return *flow.NewTransactionBody().
		SetScript(script).
		SetComputeLimit(defaultTransactionMaxGasLimit).
		SetPayer(serviceAddress).
		SetReferenceBlockID(parentID)
}

func executeCallbackTransactions(
	scheduleEvent []flowsdk.Event,
	serviceAddress flow.Address,
	parentID flow.Identifier,
) ([]flow.TransactionBody, error) {
	var transactions []flow.TransactionBody
	script := replaceSchedulerAddress(executeCallbackScript, serviceAddress)

	for _, e := range scheduleEvent {
		limit, id, err := parseSchedulerProcessedEvent(e, serviceAddress)
		if err != nil {
			return nil, err
		}

		tx := flow.NewTransactionBody().
			SetScript(script).
			AddArgument(id).
			SetPayer(serviceAddress).
			SetReferenceBlockID(parentID).
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
func parseSchedulerProcessedEvent(event flowsdk.Event, serviceAddress flow.Address) (uint64, []byte, error) {
	callbackScheduledEvent := fmt.Sprintf(
		"A.%s.UnsafeCallbackScheduler.CallbackProcessed",
		serviceAddress.String(),
	)
	const (
		IDField        = "id"
		executionField = "executionEffort"
	)

	if event.Type != callbackScheduledEvent {
		return 0, nil, fmt.Errorf("invalid event type, got: %s, expected: %s", event.Type, callbackScheduledEvent)
	}

	id, ok := event.Value.SearchFieldByName(IDField).(cadence.UInt64)
	if !ok {
		return 0, nil, fmt.Errorf("invalid ID value type: %v", id)
	}

	encodedID, err := jsoncdc.Encode(id)
	if err != nil {
		return 0, nil, err
	}

	effort, ok := event.Value.SearchFieldByName(executionField).(cadence.UInt64)
	if !ok {
		return 0, nil, fmt.Errorf("invalid effort value type: %v", effort)
	}
	computeLimit := binary.BigEndian.Uint64(effort.ToBigEndianBytes())

	return computeLimit, encodedID, nil
}

func replaceSchedulerAddress(script []byte, serviceAddress flow.Address) []byte {
	s := strings.ReplaceAll(
		string(script),
		`import "UnsafeCallbackScheduler"`,
		fmt.Sprintf("import UnsafeCallbackScheduler from %s", serviceAddress.HexWithPrefix()),
	)

	return []byte(s)
}
