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
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
)

type CoverageReportedRuntime struct {
	runtime.Runtime
	runtime.Environment
	*runtime.CoverageReport
}

func (crr CoverageReportedRuntime) NewScriptExecutor(
	script runtime.Script,
	context runtime.Context,
) runtime.Executor {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.NewScriptExecutor(script, context)
}

func (crr CoverageReportedRuntime) ExecuteScript(
	script runtime.Script,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.ExecuteScript(script, context)
}

func (crr CoverageReportedRuntime) NewTransactionExecutor(
	script runtime.Script,
	context runtime.Context,
) runtime.Executor {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.NewTransactionExecutor(script, context)
}

func (crr CoverageReportedRuntime) ExecuteTransaction(
	script runtime.Script,
	context runtime.Context,
) error {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.ExecuteTransaction(script, context)
}

func (crr CoverageReportedRuntime) NewContractFunctionExecutor(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) runtime.Executor {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.NewContractFunctionExecutor(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		context,
	)
}

func (crr CoverageReportedRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		context,
	)
}

func (crr CoverageReportedRuntime) ParseAndCheckProgram(
	source []byte,
	context runtime.Context,
) (
	*interpreter.Program,
	error,
) {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.ParseAndCheckProgram(source, context)
}

func (crr CoverageReportedRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.ReadStored(address, path, context)
}

func (crr CoverageReportedRuntime) Storage(
	context runtime.Context,
) (
	*runtime.Storage,
	*interpreter.Interpreter,
	error,
) {
	context.CoverageReport = crr.CoverageReport
	return crr.Runtime.Storage(context)
}
