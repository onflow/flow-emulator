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

	"github.com/onflow/flow-emulator/types"
)

func ToStorableResult(tp *fvm.TransactionProcedure) (types.StorableTransactionResult, error) {
	var errorCode int
	var errorMessage string

	if tp.Err != nil {
		errorCode = int(tp.Err.Code())
		errorMessage = tp.Err.Error()
	}

	return types.StorableTransactionResult{
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		Logs:         tp.Logs,
		Events:       tp.Events,
	}, nil
}
