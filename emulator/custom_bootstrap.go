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
	"math"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
)

type CustomBootStrapExecutor struct {
	ctx           fvm.Context
	txnState      storage.TransactionPreparer
	baseExecutor  fvm.ProcedureExecutor
	executionFunc func(ctx fvm.Context, executor *CustomBootStrapExecutor) error
}

func NewCustomBootStrapExecutor(ctx fvm.Context, txnState storage.TransactionPreparer, baseExecutor fvm.ProcedureExecutor, executionFunc func(ctx fvm.Context, executor *CustomBootStrapExecutor) error) *CustomBootStrapExecutor {
	return &CustomBootStrapExecutor{
		ctx:           ctx,
		baseExecutor:  baseExecutor,
		txnState:      txnState,
		executionFunc: executionFunc,
	}
}

func (c *CustomBootStrapExecutor) InvokeMetaTransaction(
	parentCtx fvm.Context,
	tx *fvm.TransactionProcedure,
) (
	errors.CodedError,
	error,
) {
	ctx := fvm.NewContextFromParent(parentCtx,
		fvm.WithAccountStorageLimit(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),

		fvm.WithMemoryAndInteractionLimitsDisabled(),
		fvm.WithComputationLimit(math.MaxUint64),
	)

	executor := tx.NewExecutor(ctx, c.txnState)
	err := fvm.Run(executor)

	return executor.Output().Err, err
}

func (c *CustomBootStrapExecutor) Preprocess() error {
	err := c.baseExecutor.Preprocess()
	if err != nil {
		return err
	}
	return nil
}

func (c *CustomBootStrapExecutor) Execute() error {
	err := c.baseExecutor.Execute()
	if err != nil {
		return err
	}
	return c.executionFunc(c.ctx, c)
}

func (c *CustomBootStrapExecutor) Cleanup() {

}

func (c *CustomBootStrapExecutor) Output() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{}
}

func (c *CustomBootstrap) NewExecutor(ctx fvm.Context, txnState storage.TransactionPreparer) fvm.ProcedureExecutor {
	return NewCustomBootStrapExecutor(ctx, txnState, c.baseBootstrap.NewExecutor(ctx, txnState), c.executionFunc)
}

func (c *CustomBootstrap) ComputationLimit(ctx fvm.Context) uint64 {
	return math.MaxUint64
}

func (c *CustomBootstrap) MemoryLimit(ctx fvm.Context) uint64 {
	return math.MaxUint64
}

func (c *CustomBootstrap) ShouldDisableMemoryAndInteractionLimits(ctx fvm.Context) bool {
	return true
}

func (c *CustomBootstrap) Type() fvm.ProcedureType {
	return fvm.BootstrapProcedureType
}

func (c *CustomBootstrap) ExecutionTime() logical.Time {
	return 0
}

type CustomBootstrap struct {
	baseBootstrap fvm.Procedure
	executionFunc func(ctx fvm.Context, executor *CustomBootStrapExecutor) error
}

func NewCustomBootstrap(baseBootstrap fvm.Procedure, executionFunc func(ctx fvm.Context, executor *CustomBootStrapExecutor) error) *CustomBootstrap {
	return &CustomBootstrap{
		baseBootstrap: baseBootstrap,
		executionFunc: executionFunc,
	}
}

var _ fvm.Procedure = &CustomBootstrap{}
var _ fvm.ProcedureExecutor = &CustomBootStrapExecutor{}
