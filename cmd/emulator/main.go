package main

import (
	"github.com/onflow/flow-go-sdk/crypto"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/cmd/emulator/start"
)

func defaultServiceKey(
	init bool,
	sigAlgo crypto.SignatureAlgorithm,
	hashAlgo crypto.HashAlgorithm,
) (crypto.PrivateKey, crypto.SignatureAlgorithm, crypto.HashAlgorithm) {
	if sigAlgo == crypto.UnknownSignatureAlgorithm {
		sigAlgo = emulator.DefaultServiceKeySigAlgo
	}

	if hashAlgo == crypto.UnknownHashAlgorithm {
		hashAlgo = emulator.DefaultServiceKeyHashAlgo
	}

	serviceKey := emulator.GenerateDefaultServiceKey(sigAlgo, hashAlgo)
	return *serviceKey.PrivateKey, serviceKey.SigAlgo, serviceKey.HashAlgo
}

func main() {
	if err := start.Cmd(defaultServiceKey).Execute(); err != nil {
		start.Exit(1, err.Error())
	}
}
