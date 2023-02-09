package server

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/go-dap"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	sdk "github.com/onflow/flow-go-sdk"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/server/backend"
)

type ScopeIdentifier uint

const (
	ScopeIdentifierLocal   ScopeIdentifier = 10000
	ScopeIdentifierGlobal  ScopeIdentifier = 10001
	ScopeIdentifierStorage ScopeIdentifier = 10002
)

type debugSession struct {
	logger                *zerolog.Logger
	backend               *backend.Backend
	readWriter            *bufio.ReadWriter
	variables             map[int]any
	variableHandleCounter int
	// sendQueue is used to capture messages from multiple request
	// processing goroutines while writing them to the client connection
	// from a single goroutine via sendFromQueue.
	//
	// We must keep track of the multiple channel senders with a wait group
	// to make sure we do not close this channel prematurely.
	//
	// Closing this channel will signal the sendFromQueue goroutine that it can exit.
	sendQueue chan dap.Message
	sendWg    sync.WaitGroup
	// debugger is the current
	debugger          *interpreter.Debugger
	stop              *interpreter.Stop
	code              string
	scriptLocation    common.StringLocation
	scriptID          string
	stopOnEntry       bool
	configurationDone bool
	launchRequested   bool
	launched          bool
	targetDepth       int
}

// sendFromQueue is to be run in a separate goroutine to listen
// on a channel for messages to send back to the client.
// It will return once the channel is closed.
func (ds *debugSession) sendFromQueue() {
	for message := range ds.sendQueue {
		ds.logger.Trace().Msgf("DAP response: %#+v", message)

		_ = dap.WriteProtocolMessage(ds.readWriter.Writer, message)
		_ = ds.readWriter.Flush()
	}
}

func (ds *debugSession) handleRequest() error {
	request, err := dap.ReadProtocolMessage(ds.readWriter.Reader)
	if err != nil {
		return err
	}

	ds.logger.Trace().Msgf("DAP request: %#+v", request)

	ds.sendWg.Add(1)
	go func() {
		ds.dispatchRequest(request)
		ds.sendWg.Done()
	}()

	return nil
}

func (ds *debugSession) send(message dap.Message) {
	ds.sendQueue <- message
}

func (ds *debugSession) dispatchRequest(request dap.Message) {
	switch request := request.(type) {

	case *dap.InitializeRequest:

		// TODO:  only allow one debug session at a time
		ds.debugger = interpreter.NewDebugger()

		ds.send(&dap.InitializeResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.Capabilities{
				SupportsConfigurationDoneRequest: true,
			},
		})

		ds.send(&dap.InitializedEvent{
			Event: newDAPEvent("initialized"),
		})

	case *dap.SetBreakpointsRequest:
		path := request.Arguments.Source.Path

		if path == ds.scriptLocation.String() {
			path = ds.scriptID
		}

		location, err := pathLocation(path)
		if err != nil {
			ds.send(newDAPErrorResponse(
				request.Seq,
				request.Command,
				dap.ErrorMessage{
					Format: "cannot add breakpoints for path: {path}",
					Variables: map[string]string{
						"path": path,
					},
				},
			))
			break
		}

		ds.debugger.ClearBreakpointsForLocation(location)

		requestBreakpoints := request.Arguments.Breakpoints

		responseBreakpoints := make([]dap.Breakpoint, 0, len(requestBreakpoints))

		for _, requestBreakpoint := range requestBreakpoints {
			ds.debugger.AddBreakpoint(location, uint(requestBreakpoint.Line))

			responseBreakpoints = append(
				responseBreakpoints,
				dap.Breakpoint{
					Source: &dap.Source{
						Path: request.Arguments.Source.Path,
					},
					// TODO:
					Verified: true,
				},
			)
		}

		ds.send(&dap.SetBreakpointsResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.SetBreakpointsResponseBody{
				Breakpoints: responseBreakpoints,
			},
		})

	case *dap.DisconnectRequest:
		ds.debugger.Continue()
		ds.backend.GetEmulator().EndDebugging()
		ds.configurationDone = false
		ds.launchRequested = false
		ds.send(&dap.DisconnectResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.AttachRequest:
		ds.targetDepth = 1
		ds.stopOnEntry = true
		ds.debugger = interpreter.NewDebugger()
		ds.run()

		ds.send(&dap.LaunchResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.LaunchRequest:
		// TODO: only allow one program at a time
		ds.debugger = interpreter.NewDebugger()
		ds.targetDepth = 1

		var args map[string]any
		_ = json.Unmarshal(request.Arguments, &args)

		programArg, ok := args["program"]
		if !ok {
			ds.send(newDAPErrorResponse(
				request.Seq,
				request.Command,
				dap.ErrorMessage{
					Format:   "Missing program",
					ShowUser: true,
				},
			))
			break
		}
		b, _ := os.ReadFile(programArg.(string))
		ds.code = string(b)
		stopOnEntryArg := args["stopOnEntry"]
		ds.stopOnEntry, _ = stopOnEntryArg.(bool)

		scriptID := sdk.Identifier(flowgo.MakeIDFromFingerPrint([]byte(ds.code)))

		ds.scriptID = hex.EncodeToString(scriptID[:])
		ds.scriptLocation = common.StringLocation(programArg.(string))

		ds.launchRequested = true

		ds.send(&dap.LaunchResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

		if ds.configurationDone && !ds.launched {
			ds.run()
		}

	case *dap.ConfigurationDoneRequest:
		ds.configurationDone = true

		if ds.launchRequested && !ds.launched {
			ds.run()
		}

		ds.send(&dap.ConfigurationDoneResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.ThreadsRequest:
		ds.send(&dap.ThreadsResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.ThreadsResponseBody{
				Threads: []dap.Thread{
					{
						Id:   1,
						Name: "Emulator",
					},
				},
			},
		})

	case *dap.PauseRequest:
		ds.debugger.RequestPause()

		ds.send(&dap.PauseResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.NextRequest:

		currentDepth := len(ds.stop.Interpreter.CallStack())
		ds.targetDepth = currentDepth

		ds.step()

		ds.send(&dap.StepOutResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.StepInRequest:
		ds.targetDepth = -1
		ds.step()

		ds.send(&dap.StepInResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.StepOutRequest:
		currentDepth := len(ds.stop.Interpreter.CallStack())
		ds.targetDepth = currentDepth - 1

		ds.step()
		ds.send(&dap.StepOutResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
		})

	case *dap.StackTraceRequest:
		// TODO: reply with error if ds.stop == nil

		stackFrames := ds.stackFrames()

		ds.send(&dap.StackTraceResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.StackTraceResponseBody{
				StackFrames: stackFrames,
			},
		})

	case *dap.SourceRequest:
		path := request.Arguments.Source.Path

		code := ds.pathCode(path)

		if code == "" {
			ds.send(newDAPErrorResponse(
				request.Seq,
				request.Command,
				dap.ErrorMessage{
					Format: "unknown source: {path}",
					Variables: map[string]string{
						"path": path,
					},
				},
			))
		} else {
			ds.send(&dap.SourceResponse{
				Response: newDAPSuccessResponse(request.GetRequest()),
				Body: dap.SourceResponseBody{
					Content: code,
				},
			})
		}

	case *dap.ContinueRequest:
		ds.stop = nil
		ds.debugger.Continue()

		ds.send(&dap.ContinueResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.ContinueResponseBody{
				AllThreadsContinued: true,
			},
		})

		// NOTE: ContinuedEvent not expected:
		//   Please note: a debug adapter is not expected to send this event in response
		//   to a request that implies that execution continues, e.g. ‘launch’ or ‘continue’.
		//   It is only necessary to send a ‘continued’ event if there was no previous request that implied this.

	case *dap.EvaluateRequest:
		// TODO: reply with error if ds.stop == nil

		variableName := request.Arguments.Expression

		activation := ds.debugger.CurrentActivation(ds.stop.Interpreter)

		variable := activation.Find(variableName)
		if variable != nil {
			value := variable.GetValue()
			ds.send(&dap.EvaluateResponse{
				Response: newDAPSuccessResponse(request.GetRequest()),
				Body: dap.EvaluateResponseBody{
					Result: value.String(),
				},
			})
		}

		ds.send(newDAPErrorResponse(
			request.Seq,
			request.Command,
			dap.ErrorMessage{
				Format: "unknown variable: {name}",
				Variables: map[string]string{
					"name": variableName,
				},
			},
		))

	case *dap.ScopesRequest:
		// TODO: return more fine-grained scopes

		ds.send(&dap.ScopesResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.ScopesResponseBody{
				Scopes: []dap.Scope{
					{
						Name:               "Variables",
						PresentationHint:   "locals",
						VariablesReference: int(ScopeIdentifierLocal),
					},
					{
						Name:               "Storage",
						PresentationHint:   "registers",
						VariablesReference: int(ScopeIdentifierStorage),
					},
				},
			},
		})

	case *dap.VariablesRequest:
		// TODO: reply with error if ds.stop == nil
		if ds.stop == nil {
			ds.send(newDAPErrorResponse(request.GetRequest().GetSeq(), "", dap.ErrorMessage{}))

		}

		variableRequested := request.Arguments.VariablesReference
		responseVariables := make([]dap.Variable, 0)

		inter := ds.stop.Interpreter

		switch ScopeIdentifier(variableRequested) {

		case ScopeIdentifierLocal:
			//reset variables
			ds.variableHandleCounter = 0
			ds.variables = make(map[int]any, 0)

			activation := ds.debugger.CurrentActivation(inter)
			location := ds.stop.Interpreter.Location
			functionValues := activation.FunctionValues()

			for name, variable := range functionValues {

				// TODO: generalize exclusion of built-ins / standard library definitions

				if location.String() == ds.scriptID && name == "self" {
					continue
				}
				if name == "BLS" || name == "RLP" {
					continue
				}

				value := variable.GetValue()

				cadenceValue, err := runtime.ExportValue(value, inter, interpreter.EmptyLocationRange)
				if err != nil {
					//	panic(err)
					continue
				}
				variable := ds.convertValueToDAPVariable(name, cadenceValue)
				responseVariables = append(
					responseVariables,
					variable,
				)
			}

		case ScopeIdentifierStorage:
			var index int = 1
			for {
				account, err := ds.backend.GetEmulator().GetAccountByIndex(uint(index))
				if err != nil { //end of accounts
					break
				}

				variable := dap.Variable{
					Name:  account.Address.String(),
					Value: "FlowAccount",
					Type:  "FlowAccount",
					PresentationHint: dap.VariablePresentationHint{
						Kind:       "class",
						Visibility: "public",
					},
					VariablesReference: ds.storeVariable(account),
				}
				index++
				responseVariables = append(
					responseVariables,
					variable,
				)
			}

		default:
			valueRequested := ds.variables[variableRequested]
			switch value := valueRequested.(type) {

			case *sdk.Account:
				storage := inter.SharedState.Config.Storage.GetStorageMap(
					common.Address(value.Address),
					common.PathDomainStorage.Identifier(),
					false,
				)
				responseVariables = ds.convertStorageMapToDAPVariables(inter, storage)

			case interpreter.Value:

			case cadence.Value:
				responseVariables = ds.convertCadenceValueMembersToDAPVariables(value)
			}

		}

		ds.send(&dap.VariablesResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.VariablesResponseBody{
				Variables: responseVariables,
			},
		})
	}
}

func (ds *debugSession) convertValueToDAPVariable(name string, value cadence.Value) dap.Variable {
	referenceHandle := 0
	switch value.(type) {
	case cadence.Dictionary, cadence.Array, cadence.Struct, cadence.Resource:
		referenceHandle = ds.storeVariable(value)
	}
	return dap.Variable{
		Name:  name,
		Value: value.String(),
		Type:  value.Type().ID(),
		PresentationHint: dap.VariablePresentationHint{
			Kind:       "property",
			Visibility: "private",
		},
		VariablesReference: referenceHandle,
	}
}

func (ds *debugSession) storeVariable(value any) int {
	ds.variableHandleCounter++
	ds.variables[ds.variableHandleCounter] = value
	return ds.variableHandleCounter
}

func (ds *debugSession) convertCadenceValueMembersToDAPVariables(cadenceValue cadence.Value) []dap.Variable {
	members := make([]dap.Variable, 0)

	switch value := cadenceValue.(type) {
	case cadence.Resource:
		for i, field := range value.ResourceType.Fields {
			variable := ds.convertValueToDAPVariable(field.Identifier, value.Fields[i])
			members = append(members, variable)
		}
	case cadence.Struct:
		for i, field := range value.StructType.Fields {
			variable := ds.convertValueToDAPVariable(field.Identifier, value.Fields[i])
			members = append(members, variable)
		}

	case cadence.Array:
		for i, element := range value.Values {
			variable := ds.convertValueToDAPVariable(fmt.Sprintf("[%d]", i), element)
			members = append(members, variable)
		}

	case cadence.Dictionary:
		for _, pair := range value.Pairs {
			variable := ds.convertValueToDAPVariable(pair.Key.String(), pair.Value)
			members = append(members, variable)
		}
	}
	return members
}

func (ds *debugSession) convertInterpreterValueToDAPVariables(
	inter *interpreter.Interpreter,
	value interpreter.Value,
) []dap.Variable {
	cadenceValue, err := runtime.ExportValue(value, inter, interpreter.EmptyLocationRange)
	if err != nil {
		panic(err)
	}
	return ds.convertCadenceValueMembersToDAPVariables(cadenceValue)
}

func (ds *debugSession) convertStorageMapToDAPVariables(
	inter *interpreter.Interpreter,
	value *interpreter.StorageMap,
) []dap.Variable {

	members := make([]dap.Variable, value.Count())

	it := value.Iterator(inter)
	for {
		key, value := it.Next()
		if key == "" {
			break
		}

		cadenceValue, err := runtime.ExportValue(value, inter, interpreter.EmptyLocationRange)
		if err != nil {
			panic(err)
		}

		variable := ds.convertValueToDAPVariable(key, cadenceValue)
		members = append(members, variable)
	}

	return members
}

func (ds *debugSession) stackFrames() []dap.StackFrame {
	invocations := ds.stop.Interpreter.CallStack()

	stackFrames := make([]dap.StackFrame, 0, len(invocations))

	location := ds.stop.Interpreter.Location
	astRange := ast.NewRangeFromPositioned(nil, ds.stop.Statement)

	startPos := astRange.StartPosition()
	endPos := astRange.EndPosition(nil)

	locationString := locationPath(location)
	if location.String() == ds.scriptID {
		locationString = ds.scriptLocation.String()
	}
	stackFrames = append(
		stackFrames,
		dap.StackFrame{
			Source: dap.Source{
				Path: locationString,
			},
			Line:      startPos.Line,
			Column:    startPos.Column + 1,
			EndLine:   endPos.Line,
			EndColumn: endPos.Column + 2,
		},
	)

	for i := len(invocations) - 1; i >= 0; i-- {
		invocation := invocations[i]

		locationRange := invocation.LocationRange

		location := locationRange.Location
		if location == nil {
			continue
		}

		startPos := locationRange.StartPosition()
		endPos := locationRange.EndPosition(nil)

		locationString := locationPath(location)
		if location.String() == ds.scriptID {
			locationString = ds.scriptLocation.String()
		}
		stackFrames = append(
			stackFrames,
			dap.StackFrame{
				Source: dap.Source{
					Path: locationString,
				},
				Line:      startPos.Line,
				Column:    startPos.Column + 1,
				EndLine:   endPos.Line,
				EndColumn: endPos.Column + 2,
			},
		)
	}
	return stackFrames
}

func (ds *debugSession) pathCode(path string) string {
	basename := strings.TrimSuffix(path, ".cdc")
	backendEmulator := ds.backend.GetEmulator()

	runningScriptID, runningCode := backendEmulator.(*emulator.Blockchain).CurrentScript()
	if basename == runningScriptID {
		return runningCode
	}

	location, err := pathLocation(path)
	if err != nil {
		return ""
	}

	if location == ds.scriptLocation {
		return ds.code
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		var account *sdk.Account
		address := sdk.Address(addressLocation.Address)
		// nolint:staticcheck
		account, err = backendEmulator.GetAccountUnsafe(address)
		if err != nil {
			return ""
		}

		contract, ok := account.Contracts[addressLocation.Name]
		if !ok {
			return ""
		}

		return string(contract)
	}

	return ""
}

func (ds *debugSession) step() {
	ds.debugger.RequestPause()
	ds.debugger.Continue()
}

func (ds *debugSession) run() context.CancelFunc {
	ds.backend.GetEmulator().SetDebugger(ds.debugger)
	if ds.stopOnEntry {
		ds.debugger.RequestPause()
	}
	ds.variableHandleCounter = 0
	ds.variables = make(map[int]any, 0)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case stop := <-ds.debugger.Stops():
				ds.stop = &stop
				depth := len(ds.stop.Interpreter.CallStack())

				//TODO: check stop reason breakpoint

				if ds.targetDepth == -1 || depth <= ds.targetDepth {
					ds.send(&dap.StoppedEvent{
						Event: newDAPEvent("stopped"),
						Body: dap.StoppedEventBody{
							Reason:            "pause",
							AllThreadsStopped: true,
							ThreadId:          1,
						},
					})
				} else {
					ds.step()
				}

			}
		}
	}()

	if ds.code != "" {
		go func() {
			// TODO: add support for arguments
			// TODO: add support for transactions. requires automine

			result, err := ds.backend.ExecuteScriptAtLatestBlock(context.Background(), []byte(ds.code), nil)
			cancel()

			var outputBody dap.OutputEventBody
			if err != nil {
				outputBody = dap.OutputEventBody{
					Category: "stderr",
					Output:   err.Error(),
				}
			} else {
				outputBody = dap.OutputEventBody{
					Category: "stdout",
					Output:   string(result),
				}
			}

			ds.send(&dap.OutputEvent{
				Event: newDAPEvent("output"),
				Body:  outputBody,
			})

			var exitCode int
			if err != nil {
				exitCode = 1
			}

			ds.send(&dap.ExitedEvent{
				Event: newDAPEvent("exited"),
				Body: dap.ExitedEventBody{
					ExitCode: exitCode,
				},
			})

			ds.send(&dap.TerminatedEvent{
				Event: newDAPEvent("terminated"),
			})

			ds.backend.GetEmulator().EndDebugging()

		}()

	}
	ds.launched = true
	return cancel
}

func newDAPEvent(event string) dap.Event {
	return dap.Event{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  0,
			Type: "event",
		},
		Event: event,
	}
}

func newDAPResponse(requestSeq int, command string, success bool) dap.Response {
	return dap.Response{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  0,
			Type: "response",
		},
		Command:    command,
		RequestSeq: requestSeq,
		Success:    success,
	}
}

func newDAPSuccessResponse(request *dap.Request) dap.Response {
	return newDAPResponse(request.Seq, request.Command, true)
}

func newDAPErrorResponse(requestSeq int, command string, message dap.ErrorMessage) *dap.ErrorResponse {
	return &dap.ErrorResponse{
		Response: newDAPResponse(requestSeq, command, false),
		Body: dap.ErrorResponseBody{
			Error: message,
		},
	}
}

func locationPath(location common.Location) string {
	return fmt.Sprintf("%s.cdc", location.String())
}

func pathLocation(path string) (common.Location, error) {

	basename := strings.TrimSuffix(path, ".cdc")

	if strings.Count(basename, ".") == 0 {
		location, _, err := common.DecodeTypeID(nil, "s."+basename)
		if err == nil && location != nil {
			return location, nil
		}

	}
	if strings.Count(basename, ".") < 3 {
		basename += "._"
	}
	basename = "A." + basename
	location, _, err := common.DecodeTypeID(nil, basename)
	if err != nil {
		return nil, err
	}
	return location, nil
}
