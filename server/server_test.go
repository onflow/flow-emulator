/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestNoPersistence(t *testing.T) {
	logger := zerolog.Nop()

	dbPath := "test_no_persistence"

	_ = os.RemoveAll(dbPath)
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()

	conf := &Config{DBPath: dbPath}
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	require.NotNil(t, server)
	_, err := os.Stat(conf.DBPath)
	require.True(t, os.IsNotExist(err), "DB should not exist")
}

func TestPersistenceWithPersistFlag(t *testing.T) {
	logger := zerolog.Nop()

	dbPath := "test_persistence"

	_ = os.RemoveAll(dbPath)
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()

	conf := &Config{Persist: true, DBPath: dbPath}
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	require.NotNil(t, server)
	_, err := os.Stat(conf.DBPath)
	require.NoError(t, err, "DB should exist")
}

func TestPersistenceWithSnapshotFlag(t *testing.T) {
	logger := zerolog.Nop()

	dbPath := "test_persistence_with_snapshot"

	_ = os.RemoveAll(dbPath)
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()

	conf := &Config{Snapshot: true, DBPath: dbPath}
	server := NewEmulatorServer(&logger, conf)
	defer server.Stop()

	require.NotNil(t, server)
	_, err := os.Stat(conf.DBPath)
	require.True(t, os.IsNotExist(err), "DB should not exist")
}

func TestExecuteScript(t *testing.T) {

	logger := zerolog.Nop()
	server := NewEmulatorServer(&logger, &Config{})
	go server.Start()
	defer server.Stop()

	require.NotNil(t, server)

	const code = `
      access(all) fun main(): String {
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

          access(all) fun main() {
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
