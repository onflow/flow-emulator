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

package convert

import (
	"github.com/onflow/flow-go/fvm"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"

	sdkConvert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/onflow/flow-emulator/types"
)

func VMTransactionResultToEmulator(tp *fvm.TransactionProcedure) (*types.TransactionResult, error) {
	txID := sdkConvert.FlowIdentifierToSDK(tp.ID)

	sdkEvents, err := sdkConvert.FlowEventsToSDK(tp.Events)
	if err != nil {
		return nil, err
	}

	return &types.TransactionResult{
		TransactionID:   txID,
		ComputationUsed: tp.ComputationUsed,
		Error:           VMErrorToEmulator(tp.Err),
		Logs:            tp.Logs,
		Events:          sdkEvents,
	}, nil
}

func VMErrorToEmulator(vmError fvmerrors.Error) error {
	if vmError == nil {
		return nil
	}

	return &types.FlowError{FlowError: vmError}
}
