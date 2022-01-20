/*
 * Flow Emulator
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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

package main

import (
	"github.com/onflow/flow-go-sdk/crypto"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/cmd/emulator/start"
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
	return serviceKey.PrivateKey, serviceKey.SigAlgo, serviceKey.HashAlgo
}

func main() {
	if err := start.Cmd(defaultServiceKey).Execute(); err != nil {
		start.Exit(1, err.Error())
	}
}
