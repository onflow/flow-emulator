/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
)

type CoverageReportedRuntime struct {
	runtime.Runtime
	runtime.Environment
	*runtime.CoverageReport

	fvmEnv reusableRuntime.Environment
}

func (crt *CoverageReportedRuntime) SetFvmEnvironment(
	fvmEnv reusableRuntime.Environment,
) {
	crt.fvmEnv = fvmEnv
}

func (crt CoverageReportedRuntime) NewScriptExecutor(
	script runtime.Script,
	context runtime.Context,
) runtime.Executor {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.NewScriptExecutor(script, context)
}

func (crt CoverageReportedRuntime) NewTransactionExecutor(
	script runtime.Script,
	context runtime.Context,
) runtime.Executor {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.NewTransactionExecutor(script, context)
}

func (crt CoverageReportedRuntime) NewContractFunctionExecutor(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) runtime.Executor {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.NewContractFunctionExecutor(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		context,
	)
}

func (crt CoverageReportedRuntime) SetDebugger(
	debugger *interpreter.Debugger,
) {
	crt.Runtime.SetDebugger(debugger)
}

func (crt CoverageReportedRuntime) ExecuteScript(
	script runtime.Script,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.ExecuteScript(script, context)
}

func (crt CoverageReportedRuntime) ExecuteTransaction(
	script runtime.Script,
	context runtime.Context,
) error {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.ExecuteTransaction(script, context)
}

func (crt CoverageReportedRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		context,
	)
}

func (crt CoverageReportedRuntime) ParseAndCheckProgram(
	source []byte,
	context runtime.Context,
) (
	*interpreter.Program,
	error,
) {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.ParseAndCheckProgram(source, context)
}

func (crt *CoverageReportedRuntime) SetCoverageReport(
	coverageReport *runtime.CoverageReport,
) {
	crt.CoverageReport = coverageReport
}

func (crt CoverageReportedRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.ReadStored(address, path, context)
}

func (crt CoverageReportedRuntime) ReadLinked(
	address common.Address,
	path cadence.Path,
	context runtime.Context,
) (
	cadence.Value,
	error,
) {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.ReadLinked(address, path, context)
}

func (crt CoverageReportedRuntime) Storage(
	context runtime.Context,
) (
	*runtime.Storage,
	*interpreter.Interpreter,
	error,
) {
	context.CoverageReport = crt.CoverageReport
	return crt.Runtime.Storage(context)
}
