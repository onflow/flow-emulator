package debugger

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/onflow/flow-emulator/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"

	"github.com/google/go-dap"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	sdk "github.com/onflow/flow-go-sdk"
)

type ScopeIdentifier uint

const (
	ScopeIdentifierLocal   ScopeIdentifier = 10000
	ScopeIdentifierGlobal  ScopeIdentifier = 10001
	ScopeIdentifierStorage ScopeIdentifier = 10002
)

type session struct {
	logger                *zerolog.Logger
	emulator              emulator.Emulator
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
func (s *session) sendFromQueue() {
	for message := range s.sendQueue {
		s.logger.Trace().Msgf("DAP response: %#+v", message)

		_ = dap.WriteProtocolMessage(s.readWriter.Writer, message)
		_ = s.readWriter.Flush()
	}
}

func (s *session) handleRequest() error {
	request, err := dap.ReadProtocolMessage(s.readWriter.Reader)
	if err != nil {
		return err
	}

	s.logger.Trace().Msgf("DAP request: %#+v", request)

	s.sendWg.Add(1)
	go func() {
		s.dispatchRequest(request)
		s.sendWg.Done()
	}()

	return nil
}

func (s *session) send(message dap.Message) {
	s.sendQueue <- message
}

func (s *session) dispatchRequest(request dap.Message) {
	switch request := request.(type) {

	case *dap.InitializeRequest:
		s.handleInitialize(request)

	case *dap.SetBreakpointsRequest:
		s.handleSetBreakpointsRequest(request)

	case *dap.DisconnectRequest:
		s.handleDisconnectRequest(request)

	case *dap.AttachRequest:
		s.handleAttachRequest(request)

	case *dap.LaunchRequest:
		s.handleLaunchRequest(request)

	case *dap.ConfigurationDoneRequest:
		s.handleConfigurationDoneRequest(request)

	case *dap.ThreadsRequest:
		s.handleThreadsRequest(request)

	case *dap.PauseRequest:
		s.handlePauseRequest(request)

	case *dap.NextRequest:
		s.handleNextRequest(request)

	case *dap.StepInRequest:
		s.handleStepInRequest(request)

	case *dap.StepOutRequest:
		s.handleStepOutRequest(request)

	case *dap.StackTraceRequest:
		s.handleStackTraceRequest(request)

	case *dap.SourceRequest:
		s.handleSourceRequest(request)

	case *dap.ContinueRequest:
		s.handleContinueRequest(request)

	case *dap.EvaluateRequest:
		s.handleEvaluateRequest(request)

	case *dap.ScopesRequest:
		s.handleScopesRequest(request)

	case *dap.VariablesRequest:
		s.handleVariablesRequest(request)
	}
}

func (s *session) handleVariablesRequest(request *dap.VariablesRequest) {
	if s.stop == nil {
		s.send(newDAPErrorResponse(request.Request, dap.ErrorMessage{
			Format: "invalid request",
		}))
		return
	}

	variableRequested := request.Arguments.VariablesReference
	responseVariables := make([]dap.Variable, 0)

	inter := s.stop.Interpreter

	switch ScopeIdentifier(variableRequested) {

	case ScopeIdentifierLocal:
		//reset variables
		s.variableHandleCounter = 0
		s.variables = make(map[int]any, 0)

		activation := s.debugger.CurrentActivation(inter)
		location := s.stop.Interpreter.Location
		functionValues := activation.FunctionValues()

		for name, variable := range functionValues {

			// TODO: generalize exclusion of built-ins / standard library definitions

			if location.String() == s.scriptID && name == "self" {
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
			variable := s.convertValueToDAPVariable(name, cadenceValue)
			responseVariables = append(
				responseVariables,
				variable,
			)
		}

	case ScopeIdentifierStorage:
		index := 1
		for {
			account, err := s.emulator.GetAccountByIndex(uint(index))
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
				VariablesReference: s.storeVariable(account),
			}
			index++
			responseVariables = append(
				responseVariables,
				variable,
			)
		}

	default:
		valueRequested := s.variables[variableRequested]
		switch value := valueRequested.(type) {

		case *sdk.Account:
			storage := inter.SharedState.Config.Storage.GetStorageMap(
				common.Address(value.Address),
				common.PathDomainStorage.Identifier(),
				false,
			)
			responseVariables = s.convertStorageMapToDAPVariables(inter, storage)

		case interpreter.Value:

		case cadence.Value:
			responseVariables = s.convertCadenceValueMembersToDAPVariables(value)
		}

	}

	s.send(&dap.VariablesResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
		Body: dap.VariablesResponseBody{
			Variables: responseVariables,
		},
	})
}

func (s *session) handleScopesRequest(request *dap.ScopesRequest) {
	// TODO: return more fine-grained scopes

	s.send(&dap.ScopesResponse{
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
}

func (s *session) handleEvaluateRequest(request *dap.EvaluateRequest) {
	if s.stop == nil {
		s.send(newDAPErrorResponse(request.Request, dap.ErrorMessage{
			Format: "invalid request",
		}))
		return
	}

	variableName := request.Arguments.Expression

	activation := s.debugger.CurrentActivation(s.stop.Interpreter)

	variable := activation.Find(variableName)
	if variable == nil {
		s.send(newDAPErrorResponse(
			request.Request,
			dap.ErrorMessage{
				Format: "unknown variable: {name}",
				Variables: map[string]string{
					"name": variableName,
				},
			},
		))
		return
	}

	value := variable.GetValue()
	s.send(&dap.EvaluateResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
		Body: dap.EvaluateResponseBody{
			Result: value.String(),
		},
	})
}

func (s *session) handleContinueRequest(request *dap.ContinueRequest) {
	s.stop = nil
	s.debugger.Continue()

	s.send(&dap.ContinueResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
		Body: dap.ContinueResponseBody{
			AllThreadsContinued: true,
		},
	})

	// NOTE: ContinuedEvent not expected:
	//   Please note: a debug adapter is not expected to send this event in response
	//   to a request that implies that execution continues, e.g. ‘launch’ or ‘continue’.
	//   It is only necessary to send a ‘continued’ event if there was no previous request that implied this.

}

func (s *session) handleSourceRequest(request *dap.SourceRequest) {
	path := request.Arguments.Source.Path

	code := s.pathCode(path)

	if code == "" {
		s.send(newDAPErrorResponse(
			request.Request,
			dap.ErrorMessage{
				Format: "unknown source: {path}",
				Variables: map[string]string{
					"path": path,
				},
			},
		))
	} else {
		s.send(&dap.SourceResponse{
			Response: newDAPSuccessResponse(request.GetRequest()),
			Body: dap.SourceResponseBody{
				Content: code,
			},
		})
	}
}

func (s *session) handleStackTraceRequest(request *dap.StackTraceRequest) {
	if s.stop == nil {
		s.send(newDAPErrorResponse(request.Request, dap.ErrorMessage{
			Format: "invalid request",
		}))
		return
	}

	stackFrames := s.stackFrames()

	s.send(&dap.StackTraceResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
		Body: dap.StackTraceResponseBody{
			StackFrames: stackFrames,
		},
	})
}

func (s *session) handleStepOutRequest(request *dap.StepOutRequest) {
	currentDepth := len(s.stop.Interpreter.CallStack())
	s.targetDepth = currentDepth - 1

	s.step()

	s.send(&dap.StepOutResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handleStepInRequest(request *dap.StepInRequest) {
	s.targetDepth = -1

	s.step()

	s.send(&dap.StepInResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handleNextRequest(request *dap.NextRequest) {
	currentDepth := len(s.stop.Interpreter.CallStack())
	s.targetDepth = currentDepth

	s.step()

	s.send(&dap.NextResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handlePauseRequest(request *dap.PauseRequest) {
	s.debugger.RequestPause()

	s.send(&dap.PauseResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handleThreadsRequest(request *dap.ThreadsRequest) {
	s.send(&dap.ThreadsResponse{
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
}

func (s *session) handleConfigurationDoneRequest(request *dap.ConfigurationDoneRequest) {
	s.configurationDone = true

	if s.launchRequested && !s.launched {
		s.run()
	}

	s.send(&dap.ConfigurationDoneResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handleLaunchRequest(request *dap.LaunchRequest) {
	// TODO: only allow one program at a time
	s.debugger = s.emulator.StartDebugger()
	s.targetDepth = 1

	var args map[string]any
	_ = json.Unmarshal(request.Arguments, &args)

	programArg, ok := args["program"]
	if !ok {
		s.send(newDAPErrorResponse(
			request.Request,
			dap.ErrorMessage{
				Format:   "Missing program",
				ShowUser: true,
			},
		))
		return
	}

	b, _ := os.ReadFile(programArg.(string))
	s.code = string(b)
	stopOnEntryArg := args["stopOnEntry"]
	s.stopOnEntry, _ = stopOnEntryArg.(bool)

	scriptID := sdk.Identifier(flowgo.MakeIDFromFingerPrint([]byte(s.code)))

	s.scriptID = hex.EncodeToString(scriptID[:])
	s.scriptLocation = common.StringLocation(programArg.(string))

	s.launchRequested = true

	s.send(&dap.LaunchResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})

	if s.configurationDone && !s.launched {
		s.run()
	}
}

func (s *session) handleAttachRequest(request *dap.AttachRequest) {
	s.targetDepth = 1
	s.stopOnEntry = false
	s.debugger = s.emulator.StartDebugger()

	s.run()

	s.send(&dap.LaunchResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handleDisconnectRequest(request *dap.DisconnectRequest) {
	s.debugger.Continue()
	s.emulator.EndDebugging()
	s.configurationDone = false
	s.launchRequested = false
	s.send(&dap.DisconnectResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
	})
}

func (s *session) handleSetBreakpointsRequest(request *dap.SetBreakpointsRequest) {
	path := request.Arguments.Source.Path

	if path == s.scriptLocation.String() {
		path = s.scriptID
	}

	location, err := pathLocation(path)
	if err != nil {
		s.send(newDAPErrorResponse(
			request.Request,
			dap.ErrorMessage{
				Format: "cannot add breakpoints for path: {path}",
				Variables: map[string]string{
					"path": path,
				},
			},
		))
		return
	}

	s.debugger.ClearBreakpointsForLocation(location)

	requestBreakpoints := request.Arguments.Breakpoints

	responseBreakpoints := make([]dap.Breakpoint, 0, len(requestBreakpoints))

	for _, requestBreakpoint := range requestBreakpoints {
		s.debugger.AddBreakpoint(location, uint(requestBreakpoint.Line))

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

	s.send(&dap.SetBreakpointsResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
		Body: dap.SetBreakpointsResponseBody{
			Breakpoints: responseBreakpoints,
		},
	})
}

func (s *session) handleInitialize(request *dap.InitializeRequest) {
	// TODO: only allow one debug session at a time
	s.debugger = s.emulator.StartDebugger()

	s.send(&dap.InitializeResponse{
		Response: newDAPSuccessResponse(request.GetRequest()),
		Body: dap.Capabilities{
			SupportsConfigurationDoneRequest: true,
		},
	})

	s.send(&dap.InitializedEvent{
		Event: newDAPEvent("initialized"),
	})
}

func (s *session) convertValueToDAPVariable(name string, value cadence.Value) dap.Variable {
	referenceHandle := 0
	switch value.(type) {
	case cadence.Dictionary, cadence.Array, cadence.Struct, cadence.Resource:
		referenceHandle = s.storeVariable(value)
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

func (s *session) storeVariable(value any) int {
	s.variableHandleCounter++
	s.variables[s.variableHandleCounter] = value
	return s.variableHandleCounter
}

func (s *session) convertCadenceValueMembersToDAPVariables(cadenceValue cadence.Value) []dap.Variable {
	members := make([]dap.Variable, 0)

	switch value := cadenceValue.(type) {
	case cadence.Resource:
		for i, field := range value.ResourceType.Fields {
			variable := s.convertValueToDAPVariable(field.Identifier, value.Fields[i])
			members = append(members, variable)
		}
	case cadence.Struct:
		for i, field := range value.StructType.Fields {
			variable := s.convertValueToDAPVariable(field.Identifier, value.Fields[i])
			members = append(members, variable)
		}

	case cadence.Array:
		for i, element := range value.Values {
			variable := s.convertValueToDAPVariable(fmt.Sprintf("[%d]", i), element)
			members = append(members, variable)
		}

	case cadence.Dictionary:
		for _, pair := range value.Pairs {
			variable := s.convertValueToDAPVariable(pair.Key.String(), pair.Value)
			members = append(members, variable)
		}
	}
	return members
}

func (s *session) convertInterpreterValueToDAPVariables(
	inter *interpreter.Interpreter,
	value interpreter.Value,
) []dap.Variable {
	cadenceValue, err := runtime.ExportValue(value, inter, interpreter.EmptyLocationRange)
	if err != nil {
		panic(err)
	}
	return s.convertCadenceValueMembersToDAPVariables(cadenceValue)
}

func (s *session) convertStorageMapToDAPVariables(
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

		variable := s.convertValueToDAPVariable(key, cadenceValue)
		members = append(members, variable)
	}

	return members
}

func (s *session) stackFrames() []dap.StackFrame {
	invocations := s.stop.Interpreter.CallStack()

	stackFrames := make([]dap.StackFrame, 0, len(invocations))

	stackFrame := s.newStackFrame(
		s.stop.Statement,
		s.stop.Interpreter.Location,
	)

	stackFrames = append(
		stackFrames,
		stackFrame,
	)

	for i := len(invocations) - 1; i >= 0; i-- {
		invocation := invocations[i]

		locationRange := invocation.LocationRange

		location := locationRange.Location
		if location == nil {
			continue
		}

		stackFrame := s.newStackFrame(locationRange, location)

		stackFrames = append(
			stackFrames,
			stackFrame,
		)
	}

	return stackFrames
}

func (s *session) newStackFrame(positioned ast.HasPosition, location common.Location) dap.StackFrame {
	startPos := positioned.StartPosition()
	endPos := positioned.EndPosition(nil)

	locationString := locationPath(location)
	if location.String() == s.scriptID {
		locationString = s.scriptLocation.String()
	}

	return dap.StackFrame{
		Source: dap.Source{
			Path: locationString,
		},
		Line:      startPos.Line,
		Column:    startPos.Column + 1,
		EndLine:   endPos.Line,
		EndColumn: endPos.Column + 2,
	}
}

func (s *session) pathCode(path string) string {
	basename := strings.TrimSuffix(path, ".cdc")
	backendEmulator := s.emulator

	runningScriptID, runningCode := backendEmulator.(*emulator.Blockchain).CurrentScript()
	if basename == runningScriptID {
		return runningCode
	}

	location, err := pathLocation(path)
	if err != nil {
		return ""
	}

	if location == s.scriptLocation {
		return s.code
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		var account *flowgo.Account
		address := flowgo.Address(addressLocation.Address)
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

func (s *session) step() {
	s.debugger.RequestPause()
	s.debugger.Continue()
}

func (s *session) run() context.CancelFunc {
	s.debugger = s.emulator.StartDebugger()

	if s.stopOnEntry {
		s.debugger.RequestPause()
	}
	s.variableHandleCounter = 0
	s.variables = make(map[int]any, 0)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case stop := <-s.debugger.Stops():
				s.stop = &stop
				depth := len(s.stop.Interpreter.CallStack())

				//TODO: check stop reason breakpoint

				if s.targetDepth == -1 || depth <= s.targetDepth {
					s.send(&dap.StoppedEvent{
						Event: newDAPEvent("stopped"),
						Body: dap.StoppedEventBody{
							Reason:            "pause",
							AllThreadsStopped: true,
							ThreadId:          1,
						},
					})
				} else {
					s.step()
				}

			}
		}
	}()

	if s.code != "" {
		go func() {
			// TODO: add support for arguments
			// TODO: add support for transactions. requires automine

			result, err := s.emulator.ExecuteScript([]byte(s.code), nil)
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
					Output:   result.Value.String(),
				}
			}

			s.send(&dap.OutputEvent{
				Event: newDAPEvent("output"),
				Body:  outputBody,
			})

			var exitCode int
			if err != nil {
				exitCode = 1
			}

			s.send(&dap.ExitedEvent{
				Event: newDAPEvent("exited"),
				Body: dap.ExitedEventBody{
					ExitCode: exitCode,
				},
			})

			s.send(&dap.TerminatedEvent{
				Event: newDAPEvent("terminated"),
			})

			s.emulator.EndDebugging()

		}()

	}
	s.launched = true
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

func newDAPErrorResponse(request dap.Request, message dap.ErrorMessage) *dap.ErrorResponse {
	return &dap.ErrorResponse{
		Response: newDAPResponse(request.Seq, request.Command, false),
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
