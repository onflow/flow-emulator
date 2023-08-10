/*
 * Flow Emulator
 *
 * Copyright Dapper Labs, Inc.
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

package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestExecuteScript(t *testing.T) {

	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, &Config{})
	go server.Start()
	defer server.Stop()

	require.NotNil(t, server)

	const code = `
      pub fun main(): String {
	      return "Hello"
      }
    `
	adapter := server.AccessAdapter()
	result, err := adapter.ExecuteScriptAtLatestBlock(context.Background(), []byte(code), nil)
	require.NoError(t, err)

	require.JSONEq(t, `{"type":"String","value":"Hello"}`, string(result))

}

func TestExecuteScriptImportingContracts(t *testing.T) {
	conf := &Config{
		WithContracts: true,
	}

	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, conf)
	require.NotNil(t, server)
	serviceAccount := server.Emulator().ServiceKey().Address.Hex()

	code := fmt.Sprintf(
		`
	      import ExampleNFT, NFTStorefront from 0x%s

          pub fun main() {
		      let collection <- ExampleNFT.createEmptyCollection()
		      destroy collection

		      NFTStorefront
		  }
        `,
		serviceAccount,
	)

	_, err := server.Emulator().ExecuteScript([]byte(code), nil)
	require.NoError(t, err)

}

func TestCustomChainID(t *testing.T) {

	conf := &Config{
		WithContracts: true,
		ChainID:       "flow-sandboxnet",
	}
	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, conf)

	serviceAccount := server.Emulator().ServiceKey().Address.Hex()

	require.Equal(t, "f4527793ee68aede", serviceAccount)
}
