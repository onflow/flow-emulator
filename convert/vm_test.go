/*
 * Flow Emulator
 *
 * Copyright 2019-2021 Dapper Labs, Inc.
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

package convert_test

import (
	"testing"

	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow-go/fvm"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-emulator/convert"
	sdkConvert "github.com/onflow/flow-emulator/convert/sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TestVm(t *testing.T) {

	t.Parallel()

	t.Run("should be able to convert", func(t *testing.T) {

		t.Parallel()

		idGenerator := test.IdentifierGenerator()

		eventGenerator := test.EventGenerator()
		event1, err := sdkConvert.SDKEventToFlow(eventGenerator.New())
		assert.NoError(t, err)

		event2, err := sdkConvert.SDKEventToFlow(eventGenerator.New())
		assert.NoError(t, err)

		tp := &fvm.TransactionProcedure{
			ID:              flowgo.Identifier(idGenerator.New()),
			Logs:            []string{"TestLog1", "TestLog2"},
			Events:          []flowgo.Event{event1, event2},
			ComputationUsed: 5,
			Err:             nil,
		}

		tr, err := convert.VMTransactionResultToEmulator(tp)
		assert.NoError(t, err)

		assert.Equal(t, tp.ID, flowgo.Identifier(tr.TransactionID))
		assert.Equal(t, tp.Logs, tr.Logs)

		flowEvents, err := sdkConvert.FlowEventsToSDK(tp.Events)
		assert.NoError(t, err)
		assert.Equal(t, flowEvents, tr.Events)

		assert.Equal(t, tp.ComputationUsed, tr.ComputationUsed)
		assert.Equal(t, tp.Err, tr.Error)
	})
}
