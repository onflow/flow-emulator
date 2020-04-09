package unittest

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go-sdk/test"
	"github.com/dapperlabs/flow-go/crypto"

	"github.com/dapperlabs/flow-go-sdk"
)

const PublicKeyFixtureCount = 2

func PublicKeyFixtures() [PublicKeyFixtureCount]crypto.PublicKey {
	encodedKeys := [PublicKeyFixtureCount]string{
		"3059301306072a8648ce3d020106082a8648ce3d0301070342000472b074a452d0a764a1da34318f44cb16740df1cfab1e6b50e5e4145dc06e5d151c9c25244f123e53c9b6fe237504a37e7779900aad53ca26e3b57c5c3d7030c4",
		"3059301306072a8648ce3d020106082a8648ce3d03010703420004d4423e4ca70ed9fb9bb9ce771e9393e0c3a1b66f3019ed89ab410cdf8f73d5a8ca06cc093766c1a46069cf83fce2a294d3322d55bb86ac9cb5aa805c7dd8d715",
	}

	keys := [PublicKeyFixtureCount]crypto.PublicKey{}

	for i, hexKey := range encodedKeys {
		bytesKey, _ := hex.DecodeString(hexKey)
		publicKey, _ := crypto.DecodePublicKey(crypto.ECDSA_P256, bytesKey)
		keys[i] = publicKey
	}

	return keys
}

func TransactionFixture(n ...func(t *flow.Transaction)) flow.Transaction {
	tx := test.TransactionGenerator().New()

	if len(n) > 0 {
		n[0](tx)
	}

	return *tx
}

func TransactionResultFixture() flow.TransactionResult {
	return flow.TransactionResult{
		Status: flow.TransactionPending,
		Events: []flow.Event{
			EventFixture(func(e *flow.Event) {}),
		},
	}
}

func EventFixture(n ...func(e *flow.Event)) flow.Event {
	event := flow.Event{
		Type: "Transfer",
		// TODO: create proper fixture
		// Values: map[string]interface{}{
		// 	"to":   flow.ZeroAddress,
		// 	"from": flow.ZeroAddress,
		// 	"id":   1,
		// },
	}

	if len(n) >= 1 {
		n[0](&event)
	}

	return event
}
