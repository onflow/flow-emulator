/*
 * Flow Emulator
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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

package sdk

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/access"
	flowcrypto "github.com/onflow/flow-go/crypto"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func SDKIdentifierToFlow(sdkIdentifier sdk.Identifier) flowgo.Identifier {
	return flowgo.Identifier(sdkIdentifier)
}

func SDKIdentifiersToFlow(sdkIdentifiers []sdk.Identifier) []flowgo.Identifier {
	ret := make([]flowgo.Identifier, len(sdkIdentifiers))
	for i, sdkIdentifier := range sdkIdentifiers {
		ret[i] = SDKIdentifierToFlow(sdkIdentifier)
	}
	return ret
}

func FlowIdentifierToSDK(flowIdentifier flowgo.Identifier) sdk.Identifier {
	return sdk.Identifier(flowIdentifier)
}

func FlowIdentifiersToSDK(flowIdentifiers []flowgo.Identifier) []sdk.Identifier {
	ret := make([]sdk.Identifier, len(flowIdentifiers))
	for i, flowIdentifier := range flowIdentifiers {
		ret[i] = FlowIdentifierToSDK(flowIdentifier)
	}
	return ret
}

func SDKProposalKeyToFlow(sdkProposalKey sdk.ProposalKey) flowgo.ProposalKey {
	return flowgo.ProposalKey{
		Address:        SDKAddressToFlow(sdkProposalKey.Address),
		KeyIndex:       uint64(sdkProposalKey.KeyIndex),
		SequenceNumber: sdkProposalKey.SequenceNumber,
	}
}

func FlowProposalKeyToSDK(flowProposalKey flowgo.ProposalKey) sdk.ProposalKey {
	return sdk.ProposalKey{
		Address:        FlowAddressToSDK(flowProposalKey.Address),
		KeyIndex:       int(flowProposalKey.KeyIndex),
		SequenceNumber: flowProposalKey.SequenceNumber,
	}
}

func SDKAddressToFlow(sdkAddress sdk.Address) flowgo.Address {
	return flowgo.Address(sdkAddress)
}

func FlowAddressToSDK(flowAddress flowgo.Address) sdk.Address {
	return sdk.Address(flowAddress)
}

func SDKAddressesToFlow(sdkAddresses []sdk.Address) []flowgo.Address {
	ret := make([]flowgo.Address, len(sdkAddresses))
	for i, sdkAddress := range sdkAddresses {
		ret[i] = SDKAddressToFlow(sdkAddress)
	}
	return ret
}

func FlowAddressesToSDK(flowAddresses []flowgo.Address) []sdk.Address {
	ret := make([]sdk.Address, len(flowAddresses))
	for i, flowAddress := range flowAddresses {
		ret[i] = FlowAddressToSDK(flowAddress)
	}
	return ret
}

func SDKTransactionSignatureToFlow(sdkTransactionSignature sdk.TransactionSignature) flowgo.TransactionSignature {
	return flowgo.TransactionSignature{
		Address:     SDKAddressToFlow(sdkTransactionSignature.Address),
		SignerIndex: sdkTransactionSignature.SignerIndex,
		KeyIndex:    uint64(sdkTransactionSignature.KeyIndex),
		Signature:   sdkTransactionSignature.Signature,
	}
}

func FlowTransactionSignatureToSDK(flowTransactionSignature flowgo.TransactionSignature) sdk.TransactionSignature {
	return sdk.TransactionSignature{
		Address:     FlowAddressToSDK(flowTransactionSignature.Address),
		SignerIndex: flowTransactionSignature.SignerIndex,
		KeyIndex:    int(flowTransactionSignature.KeyIndex),
		Signature:   flowTransactionSignature.Signature,
	}
}

func SDKTransactionSignaturesToFlow(sdkTransactionSignatures []sdk.TransactionSignature) []flowgo.TransactionSignature {
	ret := make([]flowgo.TransactionSignature, len(sdkTransactionSignatures))
	for i, sdkTransactionSignature := range sdkTransactionSignatures {
		ret[i] = SDKTransactionSignatureToFlow(sdkTransactionSignature)
	}
	return ret
}

func FlowTransactionSignaturesToSDK(flowTransactionSignatures []flowgo.TransactionSignature) []sdk.TransactionSignature {
	ret := make([]sdk.TransactionSignature, len(flowTransactionSignatures))
	for i, flowTransactionSignature := range flowTransactionSignatures {
		ret[i] = FlowTransactionSignatureToSDK(flowTransactionSignature)
	}
	return ret
}

func SDKTransactionToFlow(sdkTx sdk.Transaction) *flowgo.TransactionBody {
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

func FlowTransactionToSDK(flowTx flowgo.TransactionBody) sdk.Transaction {
	return sdk.Transaction{
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
}

func SDKTransactionResultToFlow(result *sdk.TransactionResult) (*access.TransactionResult, error) {
	statusCode := uint(0)
	errorMessage := ""

	if result.Error != nil {
		statusCode = 1
		errorMessage = result.Error.Error()
	}

	events, err := SDKEventsToFlow(result.Events)
	if err != nil {
		return nil, err
	}

	return &access.TransactionResult{
		Status:       flowgo.TransactionStatus(result.Status),
		StatusCode:   statusCode,
		Events:       events,
		ErrorMessage: errorMessage,
	}, nil
}

func SDKCollectionToFlow(col *sdk.Collection) *flowgo.LightCollection {
	return &flowgo.LightCollection{
		Transactions: SDKIdentifiersToFlow(col.TransactionIDs),
	}
}

func SDKEventToFlow(event sdk.Event) (flowgo.Event, error) {
	payload, err := jsoncdc.Encode(event.Value)
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

func SDKEventsToFlow(events []sdk.Event) ([]flowgo.Event, error) {
	flowEvents := make([]flowgo.Event, len(events))

	for i, event := range events {
		flowEvent, err := SDKEventToFlow(event)
		if err != nil {
			return nil, err
		}

		flowEvents[i] = flowEvent
	}

	return flowEvents, nil
}

func FlowEventToSDK(flowEvent flowgo.Event) (sdk.Event, error) {
	cadenceValue, err := jsoncdc.Decode(flowEvent.Payload)
	if err != nil {
		return sdk.Event{}, err
	}

	cadenceEvent, ok := cadenceValue.(cadence.Event)
	if !ok {
		return sdk.Event{}, fmt.Errorf("cadence value not of type event: %s", cadenceValue)
	}

	return sdk.Event{
		Type:             string(flowEvent.Type),
		TransactionID:    FlowIdentifierToSDK(flowEvent.TransactionID),
		TransactionIndex: int(flowEvent.TransactionIndex),
		EventIndex:       int(flowEvent.EventIndex),
		Value:            cadenceEvent,
	}, nil
}

func FlowEventsToSDK(flowEvents []flowgo.Event) ([]sdk.Event, error) {
	ret := make([]sdk.Event, len(flowEvents))
	var err error
	for i, flowEvent := range flowEvents {
		ret[i], err = FlowEventToSDK(flowEvent)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func FlowAccountPublicKeyToSDK(flowPublicKey flowgo.AccountPublicKey, index int) (sdk.AccountKey, error) {
	// TODO - Looks like SDK contains copy-paste of code from flow-go
	// Once crypto become its own separate library, this can possibly be simplified or not needed
	encodedPublicKey := flowPublicKey.PublicKey.Encode()

	sdkSignAlgo := flowPublicKey.SignAlgo

	sdkPublicKey, err := sdkcrypto.DecodePublicKey(sdkSignAlgo, encodedPublicKey)
	if err != nil {
		return sdk.AccountKey{}, err
	}

	sdkHashAlgo := flowPublicKey.HashAlgo

	return sdk.AccountKey{
		Index:          index,
		PublicKey:      sdkPublicKey,
		SigAlgo:        sdkSignAlgo,
		HashAlgo:       sdkHashAlgo,
		Weight:         flowPublicKey.Weight,
		SequenceNumber: flowPublicKey.SeqNumber,
		Revoked:        flowPublicKey.Revoked,
	}, nil
}

func SDKAccountKeyToFlow(key *sdk.AccountKey) (flowgo.AccountPublicKey, error) {
	encodedPublicKey := key.PublicKey.Encode()

	flowSignAlgo := key.SigAlgo

	flowPublicKey, err := flowcrypto.DecodePublicKey(flowSignAlgo, encodedPublicKey)
	if err != nil {
		return flowgo.AccountPublicKey{}, err
	}

	flowhashAlgo := key.HashAlgo

	return flowgo.AccountPublicKey{
		Index:     key.Index,
		PublicKey: flowPublicKey,
		SignAlgo:  flowSignAlgo,
		HashAlgo:  flowhashAlgo,
		Weight:    key.Weight,
		SeqNumber: key.SequenceNumber,
	}, nil
}

func SDKAccountKeysToFlow(keys []*sdk.AccountKey) ([]flowgo.AccountPublicKey, error) {
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

func FlowAccountPublicKeysToSDK(flowPublicKeys []flowgo.AccountPublicKey) ([]*sdk.AccountKey, error) {
	ret := make([]*sdk.AccountKey, len(flowPublicKeys))
	for i, flowPublicKey := range flowPublicKeys {
		v, err := FlowAccountPublicKeyToSDK(flowPublicKey, i)
		if err != nil {
			return nil, err
		}

		ret[i] = &v
	}
	return ret, nil
}

func FlowAccountToSDK(flowAccount flowgo.Account) (sdk.Account, error) {
	sdkPublicKeys, err := FlowAccountPublicKeysToSDK(flowAccount.Keys)
	if err != nil {
		return sdk.Account{}, err
	}

	return sdk.Account{
		Address:   FlowAddressToSDK(flowAccount.Address),
		Balance:   flowAccount.Balance,
		Code:      nil,
		Keys:      sdkPublicKeys,
		Contracts: flowAccount.Contracts,
	}, nil
}

func SDKAccountToFlow(account *sdk.Account) (*flowgo.Account, error) {
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

func FlowCollectionGuaranteeToSDK(flowGuarantee flowgo.CollectionGuarantee) sdk.CollectionGuarantee {
	return sdk.CollectionGuarantee{
		CollectionID: FlowIdentifierToSDK(flowGuarantee.CollectionID),
	}
}

func FlowCollectionGuaranteesToSDK(flowGuarantees []*flowgo.CollectionGuarantee) []*sdk.CollectionGuarantee {
	ret := make([]*sdk.CollectionGuarantee, len(flowGuarantees))
	for i, flowGuarantee := range flowGuarantees {
		sdkGuarantee := FlowCollectionGuaranteeToSDK(*flowGuarantee)
		ret[i] = &sdkGuarantee
	}
	return ret
}

func FlowSealToSDK(flowSeal flowgo.Seal) sdk.BlockSeal {
	return sdk.BlockSeal{
		// TODO
	}
}

func FlowSealsToSDK(flowSeals []*flowgo.Seal) []*sdk.BlockSeal {
	ret := make([]*sdk.BlockSeal, len(flowSeals))
	for i, flowSeal := range flowSeals {
		sdkSeal := FlowSealToSDK(*flowSeal)
		ret[i] = &sdkSeal
	}
	return ret
}

func FlowPayloadToSDK(flowPayload *flowgo.Payload) sdk.BlockPayload {
	return sdk.BlockPayload{
		CollectionGuarantees: FlowCollectionGuaranteesToSDK(flowPayload.Guarantees),
		Seals:                FlowSealsToSDK(flowPayload.Seals),
	}
}

func FlowLightCollectionToSDK(flowCollection flowgo.LightCollection) sdk.Collection {
	return sdk.Collection{
		TransactionIDs: FlowIdentifiersToSDK(flowCollection.Transactions),
	}
}
