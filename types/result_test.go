/*
 * Flow Emulator
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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

package types_test

import (
	"errors"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-emulator/types"
)

func TestResult(t *testing.T) {

	t.Parallel()

	t.Run("should return correct boolean", func(t *testing.T) {

		t.Parallel()

		idGenerator := test.IdentifierGenerator()

		trSucceed := &types.TransactionResult{
			TransactionID:   idGenerator.New(),
			ComputationUsed: 20,
			Error:           nil,
			Logs:            []string{},
			Events:          []flow.Event{},
		}
		assert.True(t, trSucceed.Succeeded())
		assert.False(t, trSucceed.Reverted())

		trReverted := &types.TransactionResult{
			TransactionID:   idGenerator.New(),
			ComputationUsed: 20,
			Error:           errors.New("transaction execution error"),
			Logs:            []string{},
			Events:          []flow.Event{},
		}
		assert.True(t, trReverted.Reverted())
		assert.False(t, trReverted.Succeeded())

		srSucceed := &types.ScriptResult{
			ScriptID: idGenerator.New(),
			Value:    cadence.Value(cadence.NewInt(1)),
			Error:    nil,
			Logs:     []string{},
			Events:   []flow.Event{},
		}
		assert.True(t, srSucceed.Succeeded())
		assert.False(t, srSucceed.Reverted())

		srReverted := &types.ScriptResult{
			ScriptID: idGenerator.New(),
			Value:    cadence.Value(cadence.NewInt(1)),
			Error:    errors.New("transaction execution error"),
			Logs:     []string{},
			Events:   []flow.Event{},
		}
		assert.True(t, srReverted.Reverted())
		assert.False(t, srReverted.Succeeded())
	})
}
