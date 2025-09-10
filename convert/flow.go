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
	"fmt"

	"github.com/onflow/flow-emulator/types"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	flowsdk "github.com/onflow/flow-go-sdk"
	accessmodel "github.com/onflow/flow-go/model/access"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func SDKIdentifierToFlow(sdkIdentifier flowsdk.Identifier) flowgo.Identifier {
	return flowgo.Identifier(sdkIdentifier)
}

func SDKIdentifiersToFlow(sdkIdentifiers []flowsdk.Identifier) []flowgo.Identifier {
	ret := make([]flowgo.Identifier, len(sdkIdentifiers))
	for i, sdkIdentifier := range sdkIdentifiers {
		ret[i] = SDKIdentifierToFlow(sdkIdentifier)
	}
	return ret
}

func FlowIdentifierToSDK(flowIdentifier flowgo.Identifier) flowsdk.Identifier {
	return flowsdk.Identifier(flowIdentifier)
}

func FlowIdentifiersToSDK(flowIdentifiers []flowgo.Identifier) []flowsdk.Identifier {
	ret := make([]flowsdk.Identifier, len(flowIdentifiers))
	for i, flowIdentifier := range flowIdentifiers {
		ret[i] = FlowIdentifierToSDK(flowIdentifier)
	}
	return ret
}

func SDKProposalKeyToFlow(sdkProposalKey flowsdk.ProposalKey) flowgo.ProposalKey {
	return flowgo.ProposalKey{
		Address:        SDKAddressToFlow(sdkProposalKey.Address),
		KeyIndex:       sdkProposalKey.KeyIndex,
		SequenceNumber: sdkProposalKey.SequenceNumber,
	}
}

func FlowProposalKeyToSDK(flowProposalKey flowgo.ProposalKey) flowsdk.ProposalKey {
	return flowsdk.ProposalKey{
		Address:        FlowAddressToSDK(flowProposalKey.Address),
		KeyIndex:       flowProposalKey.KeyIndex,
		SequenceNumber: flowProposalKey.SequenceNumber,
	}
}

func SDKAddressToFlow(sdkAddress flowsdk.Address) flowgo.Address {
	return flowgo.Address(sdkAddress)
}

func FlowAddressToSDK(flowAddress flowgo.Address) flowsdk.Address {
	return flowsdk.Address(flowAddress)
}

func SDKAddressesToFlow(sdkAddresses []flowsdk.Address) []flowgo.Address {
	ret := make([]flowgo.Address, len(sdkAddresses))
	for i, sdkAddress := range sdkAddresses {
		ret[i] = SDKAddressToFlow(sdkAddress)
	}
	return ret
}

func FlowAddressesToSDK(flowAddresses []flowgo.Address) []flowsdk.Address {
	ret := make([]flowsdk.Address, len(flowAddresses))
	for i, flowAddress := range flowAddresses {
		ret[i] = FlowAddressToSDK(flowAddress)
	}
	return ret
}

func SDKTransactionSignatureToFlow(sdkTransactionSignature flowsdk.TransactionSignature) flowgo.TransactionSignature {
	return flowgo.TransactionSignature{
		Address:     SDKAddressToFlow(sdkTransactionSignature.Address),
		SignerIndex: sdkTransactionSignature.SignerIndex,
		KeyIndex:    sdkTransactionSignature.KeyIndex,
		Signature:   sdkTransactionSignature.Signature,
	}
}

func FlowTransactionSignatureToSDK(flowTransactionSignature flowgo.TransactionSignature) flowsdk.TransactionSignature {
	return flowsdk.TransactionSignature{
		Address:     FlowAddressToSDK(flowTransactionSignature.Address),
		SignerIndex: flowTransactionSignature.SignerIndex,
		KeyIndex:    flowTransactionSignature.KeyIndex,
		Signature:   flowTransactionSignature.Signature,
	}
}

func SDKTransactionSignaturesToFlow(sdkTransactionSignatures []flowsdk.TransactionSignature) []flowgo.TransactionSignature {
	ret := make([]flowgo.TransactionSignature, len(sdkTransactionSignatures))
	for i, sdkTransactionSignature := range sdkTransactionSignatures {
		ret[i] = SDKTransactionSignatureToFlow(sdkTransactionSignature)
	}
	return ret
}

func FlowTransactionSignaturesToSDK(flowTransactionSignatures []flowgo.TransactionSignature) []flowsdk.TransactionSignature {
	ret := make([]flowsdk.TransactionSignature, len(flowTransactionSignatures))
	for i, flowTransactionSignature := range flowTransactionSignatures {
		ret[i] = FlowTransactionSignatureToSDK(flowTransactionSignature)
	}
	return ret
}

func SDKTransactionToFlow(sdkTx flowsdk.Transaction) *flowgo.TransactionBody {
	return &flowgo.TransactionBody{
		ReferenceBlockID:   SDKIdentifierToFlow(sdkTx.ReferenceBlockID),
		Script:             sdkTx.Script,
		Arguments:          sdkTx.Arguments,
		GasLimit:           sdkTx.GasLimit,
		ProposalKey:        SDKProposalKeyToFlow(sdkTx.ProposalKey),
		Payer:              SDKAddressToFlow(sdkTx.Payer),
		Authorizers:        SDKAddressesToFlow(sdkTx.Authorizers),
		PayloadSignatures:  SDKTransactionSignaturesToFlow(sdkTx.PayloadSignatures),
		EnvelopeSignatures: SDKTransactionSignaturesToFlow(sdkTx.EnvelopeSignatures),
	}
}

func FlowTransactionToSDK(flowTx flowgo.TransactionBody) flowsdk.Transaction {
	transaction := flowsdk.Transaction{
		ReferenceBlockID:   FlowIdentifierToSDK(flowTx.ReferenceBlockID),
		Script:             flowTx.Script,
		Arguments:          flowTx.Arguments,
		GasLimit:           flowTx.GasLimit,
		ProposalKey:        FlowProposalKeyToSDK(flowTx.ProposalKey),
		Payer:              FlowAddressToSDK(flowTx.Payer),
		Authorizers:        FlowAddressesToSDK(flowTx.Authorizers),
		PayloadSignatures:  FlowTransactionSignaturesToSDK(flowTx.PayloadSignatures),
		EnvelopeSignatures: FlowTransactionSignaturesToSDK(flowTx.EnvelopeSignatures),
	}
	return transaction
}

func FlowTransactionResultToSDK(result *accessmodel.TransactionResult) (*flowsdk.TransactionResult, error) {

	events, err := FlowEventsToSDK(result.Events)
	if err != nil {
		return nil, err
	}

	if result.ErrorMessage != "" {
		err = &types.ExecutionError{Code: int(result.StatusCode), Message: result.ErrorMessage}
	}

	sdkResult := &flowsdk.TransactionResult{
		Status:        flowsdk.TransactionStatus(result.Status),
		Error:         err,
		Events:        events,
		TransactionID: flowsdk.Identifier(result.TransactionID),
		BlockHeight:   result.BlockHeight,
		BlockID:       flowsdk.Identifier(result.BlockID),
	}

	return sdkResult, nil
}

func SDKEventToFlow(event flowsdk.Event) (flowgo.Event, error) {
	payload, err := ccf.EventsEncMode.Encode(event.Value)
	if err != nil {
		return flowgo.Event{}, err
	}

	return flowgo.Event{
		Type:             flowgo.EventType(event.Type),
		TransactionID:    SDKIdentifierToFlow(event.TransactionID),
		TransactionIndex: uint32(event.TransactionIndex),
		EventIndex:       uint32(event.EventIndex),
		Payload:          payload,
	}, nil
}

func FlowEventToSDK(flowEvent flowgo.Event) (flowsdk.Event, error) {
	cadenceValue, err := ccf.EventsDecMode.Decode(nil, flowEvent.Payload)
	if err != nil {
		return flowsdk.Event{}, err
	}

	cadenceEvent, ok := cadenceValue.(cadence.Event)
	if !ok {
		return flowsdk.Event{}, fmt.Errorf("cadence value not of type event: %s", cadenceValue)
	}

	return flowsdk.Event{
		Type:             string(flowEvent.Type),
		TransactionID:    FlowIdentifierToSDK(flowEvent.TransactionID),
		TransactionIndex: int(flowEvent.TransactionIndex),
		EventIndex:       int(flowEvent.EventIndex),
		Value:            cadenceEvent,
	}, nil
}

func FlowEventsToSDK(flowEvents []flowgo.Event) ([]flowsdk.Event, error) {
	ret := make([]flowsdk.Event, len(flowEvents))
	var err error
	for i, flowEvent := range flowEvents {
		ret[i], err = FlowEventToSDK(flowEvent)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func FlowAccountPublicKeyToSDK(flowPublicKey flowgo.AccountPublicKey, index uint32) (flowsdk.AccountKey, error) {

	return flowsdk.AccountKey{
		Index:          index,
		PublicKey:      flowPublicKey.PublicKey,
		SigAlgo:        flowPublicKey.SignAlgo,
		HashAlgo:       flowPublicKey.HashAlgo,
		Weight:         flowPublicKey.Weight,
		SequenceNumber: flowPublicKey.SeqNumber,
		Revoked:        flowPublicKey.Revoked,
	}, nil
}

func SDKAccountKeyToFlow(key *flowsdk.AccountKey) (flowgo.AccountPublicKey, error) {

	return flowgo.AccountPublicKey{
		Index:     key.Index,
		PublicKey: key.PublicKey,
		SignAlgo:  key.SigAlgo,
		HashAlgo:  key.HashAlgo,
		Weight:    key.Weight,
		SeqNumber: key.SequenceNumber,
		Revoked:   key.Revoked,
	}, nil
}

func SDKAccountKeysToFlow(keys []*flowsdk.AccountKey) ([]flowgo.AccountPublicKey, error) {
	accountKeys := make([]flowgo.AccountPublicKey, len(keys))

	for i, key := range keys {
		accountKey, err := SDKAccountKeyToFlow(key)
		if err != nil {
			return nil, err
		}

		accountKeys[i] = accountKey
	}

	return accountKeys, nil
}

func FlowAccountPublicKeysToSDK(flowPublicKeys []flowgo.AccountPublicKey) ([]*flowsdk.AccountKey, error) {
	ret := make([]*flowsdk.AccountKey, len(flowPublicKeys))
	for i, flowPublicKey := range flowPublicKeys {
		v, err := FlowAccountPublicKeyToSDK(flowPublicKey, uint32(i))
		if err != nil {
			return nil, err
		}

		ret[i] = &v
	}
	return ret, nil
}

func FlowAccountToSDK(flowAccount flowgo.Account) (*flowsdk.Account, error) {
	sdkPublicKeys, err := FlowAccountPublicKeysToSDK(flowAccount.Keys)
	if err != nil {
		return &flowsdk.Account{}, err
	}

	return &flowsdk.Account{
		Address:   FlowAddressToSDK(flowAccount.Address),
		Balance:   flowAccount.Balance,
		Code:      nil,
		Keys:      sdkPublicKeys,
		Contracts: flowAccount.Contracts,
	}, nil
}

func SDKAccountToFlow(account *flowsdk.Account) (*flowgo.Account, error) {
	keys, err := SDKAccountKeysToFlow(account.Keys)
	if err != nil {
		return nil, err
	}

	return &flowgo.Account{
		Address:   SDKAddressToFlow(account.Address),
		Balance:   account.Balance,
		Keys:      keys,
		Contracts: account.Contracts,
	}, nil
}

func FlowLightCollectionToSDK(flowCollection flowgo.LightCollection) flowsdk.Collection {
	return flowsdk.Collection{
		TransactionIDs: FlowIdentifiersToSDK(flowCollection.Transactions),
	}
}
