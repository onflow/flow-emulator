package convert

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/ptypes"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow/protobuf/go/flow/legacy/access"
	"github.com/onflow/flow/protobuf/go/flow/legacy/entities"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")

func AccountToMessage(a flow.Account) *entities.Account {
	accountKeys := make([]*entities.AccountKey, len(a.Keys))
	for i, key := range a.Keys {
		accountKeys[i] = AccountKeyToMessage(key)
	}

	return &entities.Account{
		Address: a.Address.Bytes(),
		Balance: a.Balance,
		Code:    a.Code,
		Keys:    accountKeys,
	}
}

func AccountKeyToMessage(a *flow.AccountKey) *entities.AccountKey {
	return &entities.AccountKey{
		Index:          uint32(a.ID),
		PublicKey:      a.PublicKey.Encode(),
		SignAlgo:       uint32(a.SigAlgo),
		HashAlgo:       uint32(a.HashAlgo),
		Weight:         uint32(a.Weight),
		SequenceNumber: uint32(a.SequenceNumber),
	}
}

func BlockToMessage(b flow.Block) (*entities.Block, error) {
	t, err := ptypes.TimestampProto(b.BlockHeader.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("convert: failed to convert block timestamp to message: %w", err)
	}

	return &entities.Block{
		Id:                   b.BlockHeader.ID.Bytes(),
		ParentId:             b.BlockHeader.ParentID.Bytes(),
		Height:               b.BlockHeader.Height,
		Timestamp:            t,
		CollectionGuarantees: CollectionGuaranteesToMessages(b.BlockPayload.CollectionGuarantees),
	}, nil
}

func BlockHeaderToMessage(b flow.BlockHeader) (*entities.BlockHeader, error) {
	t, err := ptypes.TimestampProto(b.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("convert: failed to convert message to block timestamp: %w", err)
	}

	return &entities.BlockHeader{
		Id:        b.ID.Bytes(),
		ParentId:  b.ParentID.Bytes(),
		Height:    b.Height,
		Timestamp: t,
	}, nil
}

func CadenceValueToMessage(value cadence.Value) ([]byte, error) {
	b, err := jsoncdc.Encode(value)
	if err != nil {
		return nil, fmt.Errorf("convert: %w", err)
	}

	return b, nil
}

func CollectionToMessage(c flow.Collection) *entities.Collection {
	transactionIDMessages := make([][]byte, len(c.TransactionIDs))
	for i, transactionID := range c.TransactionIDs {
		transactionIDMessages[i] = transactionID.Bytes()
	}

	return &entities.Collection{
		TransactionIds: transactionIDMessages,
	}
}

func CollectionGuaranteeToMessage(g flow.CollectionGuarantee) *entities.CollectionGuarantee {
	return &entities.CollectionGuarantee{
		CollectionId: g.CollectionID.Bytes(),
	}
}

func MessageToCollectionGuarantee(m *entities.CollectionGuarantee) (flow.CollectionGuarantee, error) {
	if m == nil {
		return flow.CollectionGuarantee{}, ErrEmptyMessage
	}

	return flow.CollectionGuarantee{
		CollectionID: flow.HashToID(m.CollectionId),
	}, nil
}

func CollectionGuaranteesToMessages(l []*flow.CollectionGuarantee) []*entities.CollectionGuarantee {
	results := make([]*entities.CollectionGuarantee, len(l))
	for i, item := range l {
		results[i] = CollectionGuaranteeToMessage(*item)
	}
	return results
}

func MessagesToCollectionGuarantees(l []*entities.CollectionGuarantee) ([]*flow.CollectionGuarantee, error) {
	results := make([]*flow.CollectionGuarantee, len(l))
	for i, item := range l {
		temp, err := MessageToCollectionGuarantee(item)
		if err != nil {
			return nil, err
		}
		results[i] = &temp
	}
	return results, nil
}

func EventToMessage(e flow.Event) (*entities.Event, error) {
	payload, err := CadenceValueToMessage(e.Value)
	if err != nil {
		return nil, err
	}

	return &entities.Event{
		Type:             e.Type,
		TransactionId:    e.TransactionID[:],
		TransactionIndex: uint32(e.TransactionIndex),
		EventIndex:       uint32(e.EventIndex),
		Payload:          payload,
	}, nil
}

func MessageToIdentifier(b []byte) flow.Identifier {
	return flow.BytesToID(b)
}

func MessagesToIdentifiers(l [][]byte) []flow.Identifier {
	results := make([]flow.Identifier, len(l))
	for i, item := range l {
		results[i] = MessageToIdentifier(item)
	}
	return results
}

func TransactionToMessage(t flow.Transaction) (*entities.Transaction, error) {
	proposalKeyMessage := &entities.Transaction_ProposalKey{
		Address:        t.ProposalKey.Address.Bytes(),
		KeyId:          uint32(t.ProposalKey.KeyID),
		SequenceNumber: t.ProposalKey.SequenceNumber,
	}

	authMessages := make([][]byte, len(t.Authorizers))
	for i, auth := range t.Authorizers {
		authMessages[i] = auth.Bytes()
	}

	payloadSigMessages := make([]*entities.Transaction_Signature, len(t.PayloadSignatures))

	for i, sig := range t.PayloadSignatures {
		payloadSigMessages[i] = &entities.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyID),
			Signature: sig.Signature,
		}
	}

	envelopeSigMessages := make([]*entities.Transaction_Signature, len(t.EnvelopeSignatures))

	for i, sig := range t.EnvelopeSignatures {
		envelopeSigMessages[i] = &entities.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyID),
			Signature: sig.Signature,
		}
	}

	return &entities.Transaction{
		Script:             t.Script,
		Arguments:          t.Arguments,
		ReferenceBlockId:   t.ReferenceBlockID.Bytes(),
		GasLimit:           t.GasLimit,
		ProposalKey:        proposalKeyMessage,
		Payer:              t.Payer.Bytes(),
		Authorizers:        authMessages,
		PayloadSignatures:  payloadSigMessages,
		EnvelopeSignatures: envelopeSigMessages,
	}, nil
}

func MessageToTransaction(m *entities.Transaction) (flow.Transaction, error) {
	if m == nil {
		return flow.Transaction{}, ErrEmptyMessage
	}

	t := flow.NewTransaction()

	t.SetScript(m.GetScript())
	t.SetReferenceBlockID(flow.HashToID(m.GetReferenceBlockId()))
	t.SetGasLimit(m.GetGasLimit())

	for _, arg := range m.GetArguments() {
		t.AddRawArgument(arg)
	}

	proposalKey := m.GetProposalKey()
	if proposalKey != nil {
		proposalAddress := flow.BytesToAddress(proposalKey.GetAddress())
		t.SetProposalKey(proposalAddress, int(proposalKey.GetKeyId()), proposalKey.GetSequenceNumber())
	}

	payer := m.GetPayer()
	if payer != nil {
		t.SetPayer(
			flow.BytesToAddress(payer),
		)
	}

	for _, authorizer := range m.GetAuthorizers() {
		t.AddAuthorizer(
			flow.BytesToAddress(authorizer),
		)
	}

	for _, sig := range m.GetPayloadSignatures() {
		addr := flow.BytesToAddress(sig.GetAddress())
		t.AddPayloadSignature(addr, int(sig.GetKeyId()), sig.GetSignature())
	}

	for _, sig := range m.GetEnvelopeSignatures() {
		addr := flow.BytesToAddress(sig.GetAddress())
		t.AddEnvelopeSignature(addr, int(sig.GetKeyId()), sig.GetSignature())
	}

	return *t, nil
}

func TransactionResultToMessage(result flow.TransactionResult) (*access.TransactionResultResponse, error) {
	eventMessages := make([]*entities.Event, len(result.Events))

	for i, event := range result.Events {
		eventMsg, err := EventToMessage(event)
		if err != nil {
			return nil, err
		}

		eventMessages[i] = eventMsg
	}

	statusCode := 0
	errorMsg := ""

	if result.Error != nil {
		statusCode = 1
		errorMsg = result.Error.Error()
	}

	return &access.TransactionResultResponse{
		Status:       entities.TransactionStatus(result.Status),
		StatusCode:   uint32(statusCode),
		ErrorMessage: errorMsg,
		Events:       eventMessages,
	}, nil
}
