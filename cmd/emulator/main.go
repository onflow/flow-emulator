package main

import (
	"github.com/onflow/flow-go-sdk/crypto"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/cmd/emulator/start"
)

const DefaultServicePrivateKeySeed = emulator.DefaultServicePrivateKeySeed

func defaultServiceKey(bool) (crypto.PrivateKey, crypto.SignatureAlgorithm, crypto.HashAlgorithm) {
	servicePrivateKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte(DefaultServicePrivateKeySeed))

	serviceKeySigAlgo := servicePrivateKey.Algorithm()
	serviceKeyHashAlgo := crypto.SHA3_256

	return servicePrivateKey, serviceKeySigAlgo, serviceKeyHashAlgo
}

func main() {
	if err := start.Cmd(defaultServiceKey).Execute(); err != nil {
		start.Exit(1, err.Error())
	}
}
