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
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSchedulerProcessedEvent(t *testing.T) {
	serviceAddress := flow.HexToAddress("0x01")
	contractLocation := common.AddressLocation{
		Address: common.Address(serviceAddress),
		Name:    contractName,
	}
	eventName := fmt.Sprintf("A.%s.%s.%s", serviceAddress.String(), contractName, callbackProcessedEvent)

	expectedEventType := string(contractLocation.TypeID(nil, contractName+".CallbackProcessed"))
	eventType := cadence.NewEventType(
		nil,
		eventName,
		[]cadence.Field{
			{Identifier: "ID", Type: cadence.UInt64Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
		},
		nil,
	)

	tests := []struct {
		name           string
		event          flowsdk.Event
		serviceAddress flow.Address
		expectedLimit  uint64
		expectedID     []byte
		expectError    bool
		errorContains  string
	}{
		{
			name: "valid event with ID=1 and effort=1000",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(1),    // ID
					cadence.NewUInt64(1000), // executionEffort
				}).WithType(eventType),
			},
			serviceAddress: serviceAddress,
			expectedLimit:  1000,
			expectedID:     mustEncodeJSON(cadence.NewUInt64(1)),
			expectError:    false,
		},
		{
			name: "valid event with ID=42 and effort=5000",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(42),   // ID
					cadence.NewUInt64(5000), // executionEffort
				}).WithType(eventType),
			},
			serviceAddress: serviceAddress,
			expectedLimit:  5000,
			expectedID:     mustEncodeJSON(cadence.NewUInt64(42)),
			expectError:    false,
		},
		{
			name: "valid event with ID=100 and effort=0",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(100), // ID
					cadence.NewUInt64(0),   // executionEffort
				}).WithType(eventType),
			},
			serviceAddress: serviceAddress,
			expectedLimit:  0,
			expectedID:     mustEncodeJSON(cadence.NewUInt64(100)),
			expectError:    false,
		},
		{
			name: "invalid event type",
			event: flowsdk.Event{
				Type: "A.0000000000000001.SomeOtherContract.SomeOtherEvent",
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(1),    // ID
					cadence.NewUInt64(1000), // executionEffort
				}).WithType(eventType),
			},
			serviceAddress: serviceAddress,
			expectError:    true,
			errorContains:  "invalid event type",
		},
		{
			name: "missing ID field",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(1000), // executionEffort
				}).WithType(cadence.NewEventType(
					nil,
					eventName,
					[]cadence.Field{
						{Identifier: "executionEffort", Type: cadence.UInt64Type},
					},
					nil,
				)),
			},
			serviceAddress: serviceAddress,
			expectError:    true,
			errorContains:  "invalid ID value type",
		},
		{
			name: "missing executionEffort field",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(1), // ID
				}).WithType(cadence.NewEventType(
					nil,
					eventName,
					[]cadence.Field{
						{Identifier: "ID", Type: cadence.UInt64Type},
					},
					nil,
				)),
			},
			serviceAddress: serviceAddress,
			expectError:    true,
			errorContains:  "invalid effort value type",
		},
		{
			name: "wrong ID field type",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.String("not-a-number"), // ID (wrong type)
					cadence.NewUInt64(1000),        // executionEffort
				}).WithType(cadence.NewEventType(
					nil,
					eventName,
					[]cadence.Field{
						{Identifier: "ID", Type: cadence.StringType},
						{Identifier: "executionEffort", Type: cadence.UInt64Type},
					},
					nil,
				)),
			},
			serviceAddress: serviceAddress,
			expectError:    true,
			errorContains:  "invalid ID value type",
		},
		{
			name: "wrong executionEffort field type",
			event: flowsdk.Event{
				Type: expectedEventType,
				Value: cadence.NewEvent([]cadence.Value{
					cadence.NewUInt64(1),           // ID
					cadence.String("not-a-number"), // executionEffort (wrong type)
				}).WithType(cadence.NewEventType(
					nil,
					eventName,
					[]cadence.Field{
						{Identifier: "ID", Type: cadence.UInt64Type},
						{Identifier: "executionEffort", Type: cadence.StringType},
					},
					nil,
				)),
			},
			serviceAddress: serviceAddress,
			expectError:    true,
			errorContains:  "invalid effort value type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, id, err := parseSchedulerProcessedEvent(tt.event, tt.serviceAddress)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedLimit, limit)
			assert.Equal(t, tt.expectedID, id)
		})
	}
}

// mustEncodeJSON is a helper function to encode cadence values to JSON for testing
func mustEncodeJSON(value cadence.Value) []byte {
	encoded, err := jsoncdc.Encode(value)
	if err != nil {
		panic(fmt.Sprintf("failed to encode JSON: %v", err))
	}
	return encoded
}
