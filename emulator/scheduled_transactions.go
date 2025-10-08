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
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/flow-go/fvm/blueprints"
	flowgo "github.com/onflow/flow-go/model/flow"
)

// parseScheduledIDs parses ID of the scheduled transactions from the events
func parseScheduledIDs(env templates.Environment, events flowgo.EventsList) ([]string, error) {
	const (
		processedCallbackIDFieldName     = "id"
		processedCallbackEffortFieldName = "executionEffort"
	)

	ids := make([]string, 0)

	for _, event := range events {
		if blueprints.PendingExecutionEventType(env) != event.Type {
			continue
		}

		eventData, err := ccf.Decode(nil, event.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}

		cadenceEvent, ok := eventData.(cadence.Event)
		if !ok {
			return nil, fmt.Errorf("event data is not a cadence event")
		}

		idValue := cadence.SearchFieldByName(
			cadenceEvent,
			processedCallbackIDFieldName,
		)

		id, ok := idValue.(cadence.UInt64)
		if !ok {
			return nil, fmt.Errorf("id is not uint64")
		}

		ids = append(ids, fmt.Sprintf("%d", id))
	}

	return ids, nil
}
