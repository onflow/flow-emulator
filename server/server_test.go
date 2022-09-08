package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestExecuteScript(t *testing.T) {

	t.Parallel()

	server := NewEmulatorServer(logrus.New(), &Config{})

	const code = `
      pub fun main(): String {
	      return "Hello"
      }
    `
	result, err := server.backend.ExecuteScriptAtLatestBlock(context.Background(), []byte(code), nil)
	require.NoError(t, err)

	require.JSONEq(t, `{"type":"String","value":"Hello"}`, string(result))

}

func TestExecuteScriptImportingContracts(t *testing.T) {

	t.Parallel()

	conf := &Config{
		WithContracts: true,
	}

	server := NewEmulatorServer(logrus.New(), conf)

	serviceAccount := server.blockchain.ServiceKey().Address.Hex()

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

	_, err := server.backend.ExecuteScriptAtLatestBlock(context.Background(), []byte(code), nil)
	require.NoError(t, err)

}

func TestCustomChainID(t *testing.T) {

	t.Parallel()

	conf := &Config{
		WithContracts: true,
		ChainID:       "flow-mainnet",
	}

	server := NewEmulatorServer(logrus.New(), conf)

	serviceAccount := server.blockchain.ServiceKey().Address.Hex()

	require.Equal(t, serviceAccount, "e467b9dd11fa00df")
}
