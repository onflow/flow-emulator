package main

import (
	"github.com/onflow/flow-go-sdk/crypto"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/cmd/emulator/start"
)

const DefaultRootPrivateKeySeed = emulator.DefaultRootPrivateKeySeed

func defaultRootKey() (crypto.PrivateKey, crypto.SignatureAlgorithm, crypto.HashAlgorithm) {
	rootPrivateKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte(DefaultRootPrivateKeySeed))

	rootKeySigAlgo := rootPrivateKey.Algorithm()
	rootKeyHashAlgo := crypto.SHA3_256

	return rootPrivateKey, rootKeySigAlgo, rootKeyHashAlgo
}

func main() {
	if err := start.Cmd(defaultRootKey).Execute(); err != nil {
		start.Exit(1, err.Error())
	}
}
