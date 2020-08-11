package main

import (
	"github.com/onflow/flow-go-sdk/crypto"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/cmd/emulator/start"
)

func defaultServiceKey(bool) (crypto.PrivateKey, crypto.SignatureAlgorithm, crypto.HashAlgorithm) {
	serviceKey := emulator.DefaultServiceKey()
	return *serviceKey.PrivateKey, serviceKey.SigAlgo, serviceKey.HashAlgo
}

func main() {
	if err := start.Cmd(defaultServiceKey).Execute(); err != nil {
		start.Exit(1, err.Error())
	}
}
