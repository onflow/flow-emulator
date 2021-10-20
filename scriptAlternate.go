package emulator

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

type ScriptInvocator struct {
	storage *storage.Store
}

func NewScriptInvocator(s storage.Store) ScriptInvocator {
	return ScriptInvocator{
		storage: &s,
	}
}

func valueDeclarations(s storage.Store) []runtime.ValueDeclaration {
	var predeclaredValues []runtime.ValueDeclaration

	saveSnapshot := runtime.ValueDeclaration{
		Name: "saveSnapshot",
		Type: &sema.FunctionType{
			Parameters: []*sema.Parameter{
				{
					Label:          sema.ArgumentLabelNotRequired,
					Identifier:     "name",
					TypeAnnotation: sema.NewTypeAnnotation(sema.StringType),
				},
			},
			ReturnTypeAnnotation: &sema.TypeAnnotation{
				Type: sema.VoidType,
			},
		},
		Kind:           common.DeclarationKindFunction,
		IsConstant:     true,
		ArgumentLabels: nil,
		Value: interpreter.NewHostFunctionValue(
			func(invocation interpreter.Invocation) interpreter.Value {

				name, ok := invocation.Arguments[0].(*interpreter.StringValue)
				if !ok {
					panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
						"first argument of saveSnapshot must be an String"))
				}

				/*s.SetTag(name.Str, &object.Signature{
					Name:  "Flow Emulator",
					Email: "emulator@onflow.org",
					When:  time.Now(),
				})*/

				fmt.Printf("snap called %s\n", name)

				return interpreter.VoidValue{}
			},
		),
	}

	predeclaredValues = append(predeclaredValues, saveSnapshot)

	return predeclaredValues
}

func (i ScriptInvocator) Process(
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	proc *fvm.ScriptProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	env := fvm.NewScriptEnvironment(ctx, vm, sth, programs)
	location := common.ScriptLocation(proc.ID[:])

	value, err := vm.Runtime.ExecuteScript(
		runtime.Script{
			Source:    proc.Script,
			Arguments: proc.Arguments,
		},
		runtime.Context{
			Interface:         env,
			Location:          location,
			PredeclaredValues: valueDeclarations(*i.storage),
		},
	)

	if err != nil {
		return errors.HandleRuntimeError(err)
	}

	proc.Value = value
	proc.Logs = env.Logs()
	proc.Events = env.Events()
	proc.GasUsed = env.GetComputationUsed()
	return nil
}
