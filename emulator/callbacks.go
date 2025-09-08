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

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

const (
	contractName               = "FlowCallbackScheduler"
	callbackProcessedEventName = "Processed"
)

// todo: replace all the functions bellow with flow-go implementation once it's done
// issue: https://github.com/onflow/flow-emulator/issues/829

func processCallbackTransaction(
	serviceAddress flow.Address,
	parentID flow.Identifier,
) flow.TransactionBody {
	env := templates.Environment{
		FlowCallbackSchedulerAddress: serviceAddress.HexWithPrefix(),
	}

	script := templates.GenerateProcessCallbackScript(env)

	txBuilder := flow.NewTransactionBodyBuilder().
		SetScript(script).
		SetComputeLimit(defaultTransactionMaxGasLimit).
		SetPayer(serviceAddress).
		SetReferenceBlockID(parentID)

	tx, err := txBuilder.Build()
	if err != nil {
		panic(err)
	}

	return *tx
}

func executeCallbackTransactions(
	processedEvents []flowsdk.Event,
	serviceAddress flow.Address,
	parentID flow.Identifier,
) ([]flow.TransactionBody, error) {
	var transactions []flow.TransactionBody
	env := templates.Environment{
		FlowCallbackSchedulerAddress: serviceAddress.HexWithPrefix(),
	}

	script := templates.GenerateExecuteCallbackScript(env)

	for _, e := range processedEvents {
		id, _, limit, _, err := parseSchedulerProcessedEvent(e, serviceAddress)
		if err != nil {
			return nil, err
		}

		txBuilder := flow.NewTransactionBodyBuilder().
			SetScript(script).
			AddArgument(id).
			SetPayer(serviceAddress).
			SetReferenceBlockID(parentID).
			SetComputeLimit(limit)

		tx, err := txBuilder.Build()
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, *tx)
	}

	return transactions, nil
}

// parseSchedulerProcessedEvent parses flow event that is emitted during scheduler
// marking the callback as processed.
// Returns:
// - ID of the callback encoded as bytes
// - The priority of the callback
// - execution effort
// - The address of the account that owns the callback
// - error in case the event type is not correct
func parseSchedulerProcessedEvent(event flowsdk.Event, serviceAddress flow.Address) ([]byte, uint8, uint64, cadence.Address, error) {
	contractLocation := common.AddressLocation{
		Address: common.Address(serviceAddress),
		Name:    contractName,
	}
	callbackProcessedEvent := contractLocation.TypeID(nil, fmt.Sprintf("%s.%s", contractName, callbackProcessedEventName))

	const (
		IDField        = "id"
		priorityField  = "priority"
		executionField = "executionEffort"
		ownerField     = "callbackOwner"
	)

	if event.Type != string(callbackProcessedEvent) {
		return nil, 0, 0, cadence.BytesToAddress([]byte{}), fmt.Errorf("invalid event type, got: %s, expected: %s", event.Type, callbackProcessedEvent)
	}

	id, ok := event.Value.SearchFieldByName(IDField).(cadence.UInt64)
	if !ok {
		return nil, 0, 0, cadence.BytesToAddress([]byte{}), fmt.Errorf("invalid ID value type: %v", id)
	}

	encodedID, err := jsoncdc.Encode(id)
	if err != nil {
		return nil, 0, 0, cadence.BytesToAddress([]byte{}), err
	}

	priorityRaw, ok := event.Value.SearchFieldByName(priorityField).(cadence.UInt8)
	if !ok {
		return nil, 0, 0, cadence.BytesToAddress([]byte{}), fmt.Errorf("invalid priority value type: %v", priorityRaw)
	}
	priority := uint8(priorityRaw)

	effortRaw, ok := event.Value.SearchFieldByName(executionField).(cadence.UInt64)
	if !ok {
		return nil, 0, 0, cadence.BytesToAddress([]byte{}), fmt.Errorf("invalid effort value type: %v", effortRaw)
	}
	executionEffort := uint64(effortRaw)

	owner, ok := event.Value.SearchFieldByName(ownerField).(cadence.Address)
	if !ok {
		return nil, 0, 0, cadence.BytesToAddress([]byte{}), fmt.Errorf("invalid owner value type: %v", owner)
	}

	return encodedID, priority, executionEffort, owner, nil
}
