/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package emulator

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed templates/executeCallbackTransaction.cdc
var executeCallbackScript []byte

//go:embed templates/processCallbackTransaction.cdc
var processCallbackScript []byte

const (
	contractName           = "UnsafeCallbackScheduler"
	callbackProcessedEvent = "CallbackProcessed"
)

// todo: replace all the functions bellow with flow-go implementation once it's done
// issue: https://github.com/onflow/flow-emulator/issues/829

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
	contractLocation := common.AddressLocation{
		Address: common.Address(serviceAddress),
		Name:    contractName,
	}
	callbackScheduledEvent := contractLocation.TypeID(nil, fmt.Sprintf("%s.%s", contractName, callbackProcessedEvent))

	const (
		IDField        = "ID"
		executionField = "executionEffort"
	)

	if event.Type != string(callbackScheduledEvent) {
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
	computeLimit := uint64(effort)

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
