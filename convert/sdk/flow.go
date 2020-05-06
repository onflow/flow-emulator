package sdk

import (
	"fmt"

	flowCrypto "github.com/dapperlabs/flow-go/crypto"
	flowHash "github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"
	sdkCryptio "github.com/onflow/flow-go-sdk/crypto"
)

func SDKIdentifierToFlow(sdkIdentifier sdk.Identifier) flow.Identifier {
	return flow.Identifier(sdkIdentifier)
}

func FlowIdentifierToSDK(flowIdentifier flow.Identifier) sdk.Identifier {
	return sdk.Identifier(flowIdentifier)
}

func FlowIdentifiersToSDK(flowIdentifiers []flow.Identifier) []sdk.Identifier {
	ret := make([]sdk.Identifier, len(flowIdentifiers))
	for i, flowIdentifier := range flowIdentifiers {
		ret[i] = FlowIdentifierToSDK(flowIdentifier)
	}
	return ret
}

func SDKProposalKeyToFlow(sdkProposalKey sdk.ProposalKey) flow.ProposalKey {
	return flow.ProposalKey{
		Address:        SDKAddressToFlow(sdkProposalKey.Address),
		KeyID:          uint64(sdkProposalKey.KeyID),
		SequenceNumber: sdkProposalKey.SequenceNumber,
	}
}

func FlowProposalKeyToSDK(flowProposalKey flow.ProposalKey) sdk.ProposalKey {
	return sdk.ProposalKey{
		Address:        FlowAddressToSDK(flowProposalKey.Address),
		KeyID:          int(flowProposalKey.KeyID),
		SequenceNumber: flowProposalKey.SequenceNumber,
	}
}

func SDKAddressToFlow(sdkAddress sdk.Address) flow.Address {
	return flow.Address(sdkAddress)
}

func FlowAddressToSDK(flowAddress flow.Address) sdk.Address {
	return sdk.Address(flowAddress)
}

func SDKAddressesToFlow(sdkAddresses []sdk.Address) []flow.Address {
	ret := make([]flow.Address, len(sdkAddresses))
	for i, sdkAddress := range sdkAddresses {
		ret[i] = SDKAddressToFlow(sdkAddress)
	}
	return ret
}

func FlowAddressesToSDK(flowAddresses []flow.Address) []sdk.Address {
	ret := make([]sdk.Address, len(flowAddresses))
	for i, flowAddress := range flowAddresses {
		ret[i] = FlowAddressToSDK(flowAddress)
	}
	return ret
}

func SDKTransactionSignatureToFlow(sdkTransactionSignature sdk.TransactionSignature) flow.TransactionSignature {
	return flow.TransactionSignature{
		Address:     SDKAddressToFlow(sdkTransactionSignature.Address),
		SignerIndex: sdkTransactionSignature.SignerIndex,
		KeyID:       uint64(sdkTransactionSignature.KeyID),
		Signature:   sdkTransactionSignature.Signature,
	}
}

func FlowTransactionSignatureToSDK(flowTransactionSignature flow.TransactionSignature) sdk.TransactionSignature {
	return sdk.TransactionSignature{
		Address:     FlowAddressToSDK(flowTransactionSignature.Address),
		SignerIndex: flowTransactionSignature.SignerIndex,
		KeyID:       int(flowTransactionSignature.KeyID),
		Signature:   flowTransactionSignature.Signature,
	}
}

func SDKTransactionSignaturesToFlow(sdkTransactionSignatures []sdk.TransactionSignature) []flow.TransactionSignature {
	ret := make([]flow.TransactionSignature, len(sdkTransactionSignatures))
	for i, sdkTransactionSignature := range sdkTransactionSignatures {
		ret[i] = SDKTransactionSignatureToFlow(sdkTransactionSignature)
	}
	return ret
}

func FlowTransactionSignaturesToSDK(flowTransactionSignatures []flow.TransactionSignature) []sdk.TransactionSignature {
	ret := make([]sdk.TransactionSignature, len(flowTransactionSignatures))
	for i, flowTransactionSignature := range flowTransactionSignatures {
		ret[i] = FlowTransactionSignatureToSDK(flowTransactionSignature)
	}
	return ret
}

func SDKTransactionToFlow(sdkTx sdk.Transaction) flow.TransactionBody {
	return flow.TransactionBody{
		ReferenceBlockID:   SDKIdentifierToFlow(sdkTx.ReferenceBlockID),
		Script:             sdkTx.Script,
		GasLimit:           sdkTx.GasLimit,
		ProposalKey:        SDKProposalKeyToFlow(sdkTx.ProposalKey),
		Payer:              SDKAddressToFlow(sdkTx.Payer),
		Authorizers:        SDKAddressesToFlow(sdkTx.Authorizers),
		PayloadSignatures:  SDKTransactionSignaturesToFlow(sdkTx.PayloadSignatures),
		EnvelopeSignatures: SDKTransactionSignaturesToFlow(sdkTx.EnvelopeSignatures),
	}
}

func FlowTransactionToSDK(flowTx flow.TransactionBody) sdk.Transaction {
	return sdk.Transaction{
		ReferenceBlockID:   FlowIdentifierToSDK(flowTx.ReferenceBlockID),
		Script:             flowTx.Script,
		GasLimit:           flowTx.GasLimit,
		ProposalKey:        FlowProposalKeyToSDK(flowTx.ProposalKey),
		Payer:              FlowAddressToSDK(flowTx.Payer),
		Authorizers:        FlowAddressesToSDK(flowTx.Authorizers),
		PayloadSignatures:  FlowTransactionSignaturesToSDK(flowTx.PayloadSignatures),
		EnvelopeSignatures: FlowTransactionSignaturesToSDK(flowTx.EnvelopeSignatures),
	}
}

func FlowEventToSDK(flowEvent flow.Event) (sdk.Event, error) {

	cadenceValue, err := json.Decode(flowEvent.Payload)
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

func FlowEventsToSDK(flowEvents []flow.Event) ([]sdk.Event, error) {
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

func FlowSignAlgoToSDK(signAlgo flowCrypto.SigningAlgorithm) sdkCryptio.SignatureAlgorithm {
	return sdkCryptio.StringToSignatureAlgorithm(signAlgo.String())
}

func SDKSignAlgoToFlow(signAlgo sdkCryptio.SignatureAlgorithm) flowCrypto.SigningAlgorithm {
	return flowCrypto.StringToSignatureAlgorithm(signAlgo.String())
}

func FlowHashAlgoToSDK(hashAlgo flowHash.HashingAlgorithm) sdkCryptio.HashAlgorithm {
	return sdkCryptio.StringToHashAlgorithm(hashAlgo.String())
}

func SDKHashAlgoToFlow(hashAlgo sdkCryptio.HashAlgorithm) flowHash.HashingAlgorithm {
	return flowHash.StringToHashAlgorithm(hashAlgo.String())
}

func FlowAccountPublicKeyToSDK(flowPublicKey flow.AccountPublicKey, id int) (sdk.AccountKey, error) {
	// TODO - Looks like SDK contains copy-paste of code from flow-go
	// Once crypto become its own separate library, this can possibly be simplified or not needed
	encodedPublicKey := flowPublicKey.PublicKey.Encode()

	sdkSignAlgo := FlowSignAlgoToSDK(flowPublicKey.SignAlgo)

	sdkPublicKey, err := sdkCryptio.DecodePublicKey(sdkSignAlgo, encodedPublicKey)
	if err != nil {
		return sdk.AccountKey{}, err
	}

	sdkHashAlgo := FlowHashAlgoToSDK(flowPublicKey.HashAlgo)

	return sdk.AccountKey{
		ID:             id,
		PublicKey:      sdkPublicKey,
		SigAlgo:        sdkSignAlgo,
		HashAlgo:       sdkHashAlgo,
		Weight:         flowPublicKey.Weight,
		SequenceNumber: flowPublicKey.SeqNumber,
	}, nil
}

func SDKAccountPublicKeyToFlow(sdkPublicKey sdk.AccountKey) (flow.AccountPublicKey, error) {
	encodedPublicKey := sdkPublicKey.PublicKey.Encode()

	flowSignAlgo := SDKSignAlgoToFlow(sdkPublicKey.SigAlgo)

	flowPublicKey, err := flowCrypto.DecodePublicKey(flowSignAlgo, encodedPublicKey)
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	flowHashAlgo := SDKHashAlgoToFlow(sdkPublicKey.HashAlgo)

	return flow.AccountPublicKey{
		PublicKey: flowPublicKey,
		SignAlgo:  flowSignAlgo,
		HashAlgo:  flowHashAlgo,
		Weight:    sdkPublicKey.Weight,
		SeqNumber: sdkPublicKey.SequenceNumber,
	}, nil
}

func FlowAccountPublicKeysToSDK(flowPublicKeys []flow.AccountPublicKey) ([]*sdk.AccountKey, error) {
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

func FlowAccountToSDK(flowAccount flow.Account) (sdk.Account, error) {
	sdkPublicKeys, err := FlowAccountPublicKeysToSDK(flowAccount.Keys)
	if err != nil {
		return sdk.Account{}, err
	}
	return sdk.Account{
		Address: FlowAddressToSDK(flowAccount.Address),
		Balance: flowAccount.Balance,
		Code:    flowAccount.Code,
		Keys:    sdkPublicKeys,
	}, nil
}

func FlowHeaderToSDK(flowHeader *flow.Header) sdk.BlockHeader {
	return sdk.BlockHeader{
		ID:       FlowIdentifierToSDK(flowHeader.ID()),
		ParentID: FlowIdentifierToSDK(flowHeader.ParentID),
		Height:   flowHeader.Height,
	}
}

func FlowCollectionGuaranteeToSDK(flowGuarantee flow.CollectionGuarantee) sdk.CollectionGuarantee {
	return sdk.CollectionGuarantee{
		CollectionID: FlowIdentifierToSDK(flowGuarantee.CollectionID),
	}
}

func FlowCollectionGuaranteesToSDK(flowGuarantees []*flow.CollectionGuarantee) []*sdk.CollectionGuarantee {
	ret := make([]*sdk.CollectionGuarantee, len(flowGuarantees))
	for i, flowGuarantee := range flowGuarantees {
		sdkGuarantee := FlowCollectionGuaranteeToSDK(*flowGuarantee)
		ret[i] = &sdkGuarantee
	}
	return ret
}

func FlowSealToSDK(flowSeal flow.Seal) sdk.BlockSeal {
	return sdk.BlockSeal{
		//TODO
	}
}

func FlowSealsToSDK(flowSeals []*flow.Seal) []*sdk.BlockSeal {
	ret := make([]*sdk.BlockSeal, len(flowSeals))
	for i, flowSeal := range flowSeals {
		sdkSeal := FlowSealToSDK(*flowSeal)
		ret[i] = &sdkSeal
	}
	return ret
}

func FlowPayloadToSDK(flowPayload *flow.Payload) sdk.BlockPayload {
	return sdk.BlockPayload{
		CollectionGuarantees: FlowCollectionGuaranteesToSDK(flowPayload.Guarantees),
		Seals:                FlowSealsToSDK(flowPayload.Seals),
	}
}

func FlowBlockToSDK(flowBlock flow.Block) sdk.Block {
	return sdk.Block{
		BlockHeader:  FlowHeaderToSDK(flowBlock.Header),
		BlockPayload: FlowPayloadToSDK(flowBlock.Payload),
	}
}

func FlowLightCollectionToSDK(flowCollection flow.LightCollection) sdk.Collection {
	return sdk.Collection{
		TransactionIDs: FlowIdentifiersToSDK(flowCollection.Transactions),
	}
}
