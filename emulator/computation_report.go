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
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
)

type ProcedureReport struct {
	Path            string `json:"path"`
	ComputationUsed uint64 `json:"computation"`
	// To get the computation from the intensities map, see:
	// https://github.com/onflow/flow-go/blob/master/fvm/meter/computation_meter.go#L32-L39
	Intensities    map[string]uint `json:"intensities"`
	MemoryEstimate uint64          `json:"memory"`
	Code           string          `json:"source"`
	Arguments      []string        `json:"arguments"`
}

type ComputationReport struct {
	Scripts      map[string]ProcedureReport `json:"scripts"`
	Transactions map[string]ProcedureReport `json:"transactions"`
}

func (cr *ComputationReport) ReportScript(
	scriptResult *types.ScriptResult,
	path string,
	code string,
	arguments []string,
	intensities meter.MeteredComputationIntensities,
) {
	scriptReport := ProcedureReport{
		Path:            path,
		ComputationUsed: scriptResult.ComputationUsed,
		Intensities:     transformIntensities(intensities),
		MemoryEstimate:  scriptResult.MemoryEstimate,
		Code:            code,
		Arguments:       arguments,
	}
	cr.Scripts[scriptResult.ScriptID.String()] = scriptReport
}

func (cr *ComputationReport) ReportTransaction(
	txResult *types.TransactionResult,
	path string,
	code string,
	arguments []string,
	intensities meter.MeteredComputationIntensities,
) {
	txReport := ProcedureReport{
		Path:            path,
		ComputationUsed: txResult.ComputationUsed,
		Intensities:     transformIntensities(intensities),
		MemoryEstimate:  txResult.MemoryEstimate,
		Code:            code,
		Arguments:       arguments,
	}
	cr.Transactions[txResult.TransactionID.String()] = txReport
}

// transformIntensities maps a numeric common.ComputationKind value
// to a human-friendly string value.
func transformIntensities(
	intensities meter.MeteredComputationIntensities,
) map[string]uint {
	result := make(map[string]uint)

	for kind, value := range intensities {
		switch kind {
		case common.ComputationKindStatement:
			result["Statement"] = value
		case common.ComputationKindLoop:
			result["Loop"] = value
		case common.ComputationKindFunctionInvocation:
			result["FunctionInvocation"] = value
		case common.ComputationKindCreateCompositeValue:
			result["CreateCompositeValue"] = value
		case common.ComputationKindTransferCompositeValue:
			result["TransferCompositeValue"] = value
		case common.ComputationKindDestroyCompositeValue:
			result["DestroyCompositeValue"] = value
		case common.ComputationKindCreateArrayValue:
			result["CreateArrayValue"] = value
		case common.ComputationKindTransferArrayValue:
			result["TransferArrayValue"] = value
		case common.ComputationKindDestroyArrayValue:
			result["DestroyArrayValue"] = value
		case common.ComputationKindCreateDictionaryValue:
			result["CreateDictionaryValue"] = value
		case common.ComputationKindTransferDictionaryValue:
			result["TransferDictionaryValue"] = value
		case common.ComputationKindDestroyDictionaryValue:
			result["DestroyDictionaryValue"] = value
		case common.ComputationKindEncodeValue:
			result["EncodeValue"] = value
		case common.ComputationKindSTDLIBPanic:
			result["STDLIBPanic"] = value
		case common.ComputationKindSTDLIBAssert:
			result["STDLIBAssert"] = value
		case common.ComputationKindSTDLIBRevertibleRandom:
			result["STDLIBRevertibleRandom"] = value
		case common.ComputationKindSTDLIBRLPDecodeString:
			result["STDLIBRLPDecodeString"] = value
		case common.ComputationKindSTDLIBRLPDecodeList:
			result["STDLIBRLPDecodeList"] = value
		case environment.ComputationKindHash:
			result["Hash"] = value
		case environment.ComputationKindVerifySignature:
			result["VerifySignature"] = value
		case environment.ComputationKindAddAccountKey:
			result["AddAccountKey"] = value
		case environment.ComputationKindAddEncodedAccountKey:
			result["AddEncodedAccountKey"] = value
		case environment.ComputationKindAllocateStorageIndex:
			result["AllocateStorageIndex"] = value
		case environment.ComputationKindCreateAccount:
			result["CreateAccount"] = value
		case environment.ComputationKindEmitEvent:
			result["EmitEvent"] = value
		case environment.ComputationKindGenerateUUID:
			result["GenerateUUID"] = value
		case environment.ComputationKindGetAccountAvailableBalance:
			result["GetAccountAvailableBalance"] = value
		case environment.ComputationKindGetAccountBalance:
			result["GetAccountBalance"] = value
		case environment.ComputationKindGetAccountContractCode:
			result["GetAccountContractCode"] = value
		case environment.ComputationKindGetAccountContractNames:
			result["GetAccountContractNames"] = value
		case environment.ComputationKindGetAccountKey:
			result["GetAccountKey"] = value
		case environment.ComputationKindGetBlockAtHeight:
			result["GetBlockAtHeight"] = value
		case environment.ComputationKindGetCode:
			result["GetCode"] = value
		case environment.ComputationKindGetCurrentBlockHeight:
			result["GetCurrentBlockHeight"] = value
		case environment.ComputationKindGetStorageCapacity:
			result["GetStorageCapacity"] = value
		case environment.ComputationKindGetStorageUsed:
			result["GetStorageUsed"] = value
		case environment.ComputationKindGetValue:
			result["GetValue"] = value
		case environment.ComputationKindRemoveAccountContractCode:
			result["RemoveAccountContractCode"] = value
		case environment.ComputationKindResolveLocation:
			result["ResolveLocation"] = value
		case environment.ComputationKindRevokeAccountKey:
			result["RevokeAccountKey"] = value
		case environment.ComputationKindSetValue:
			result["SetValue"] = value
		case environment.ComputationKindUpdateAccountContractCode:
			result["UpdateAccountContractCode"] = value
		case environment.ComputationKindValidatePublicKey:
			result["ValidatePublicKey"] = value
		case environment.ComputationKindValueExists:
			result["ValueExists"] = value
		case environment.ComputationKindAccountKeysCount:
			result["AccountKeysCount"] = value
		case environment.ComputationKindBLSVerifyPOP:
			result["BLSVerifyPOP"] = value
		case environment.ComputationKindBLSAggregateSignatures:
			result["BLSAggregateSignatures"] = value
		case environment.ComputationKindBLSAggregatePublicKeys:
			result["BLSAggregatePublicKeys"] = value
		case environment.ComputationKindGetOrLoadProgram:
			result["GetOrLoadProgram"] = value
		case environment.ComputationKindGenerateAccountLocalID:
			result["GenerateAccountLocalID"] = value
		case environment.ComputationKindGetRandomSourceHistory:
			result["GetRandomSourceHistory"] = value
		case environment.ComputationKindEVMGasUsage:
			result["EVMGasUsage"] = value
		case environment.ComputationKindRLPEncoding:
			result["RLPEncoding"] = value
		case environment.ComputationKindRLPDecoding:
			result["RLPDecoding"] = value
		case environment.ComputationKindEncodeEvent:
			result["EncodeEvent"] = value
		case environment.ComputationKindEVMEncodeABI:
			result["EVMEncodeABI"] = value
		case environment.ComputationKindEVMDecodeABI:
			result["EVMDecodeABI"] = value
		default:
			result[kind.String()] = value
		}
	}

	return result
}
