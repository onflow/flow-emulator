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

package convert

import (
	"github.com/onflow/flow-go/fvm"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/types"
)

func ToStorableResult(
	output fvm.ProcedureOutput,
	blockID flowgo.Identifier,
	blockHeight uint64,
) (
	types.StorableTransactionResult,
	error,
) {
	var errorCode int
	var errorMessage string

	if output.Err != nil {
		errorCode = int(output.Err.Code())
		errorMessage = output.Err.Error()
	}

	return types.StorableTransactionResult{
		BlockID:      blockID,
		BlockHeight:  blockHeight,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		Logs:         output.Logs,
		Events:       output.Events,
	}, nil
}
