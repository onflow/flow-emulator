package emulator_test

import (
	"fmt"
	flowgo "github.com/onflow/flow-go/model/flow"
	"testing"

	"github.com/onflow/flow-emulator/emulator"
	"github.com/stretchr/testify/require"
)

func TestCommonContractsDeployment(t *testing.T) {

	t.Parallel()

	//only test monotonic and emulator ( mainnet / testnet is used for remote debugging )
	chains := []flowgo.Chain{
		flowgo.Emulator.Chain(),
		flowgo.MonotonicEmulator.Chain(),
	}

	contracts := []string{
		"FUSD",
		"MetadataViews",
		"ExampleNFT",
		"NFTStorefrontV2",
		"NFTStorefront",
		"NonFungibleToken",
	}
	for _, chain := range chains {

		b, err := emulator.New(
			emulator.Contracts(emulator.NewCommonContracts(chain)),
			emulator.WithChainID(chain.ChainID()),
		)
		require.NoError(t, err)

		serviceAccount := b.ServiceKey().Address.Hex()

		for _, contract := range contracts {

			scriptCode := fmt.Sprintf(`
			pub fun main() {
				getAccount(0x%s).contracts.get(name: "%s") ?? panic("contract is not deployed")
	    	}`, serviceAccount, contract)

			scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
			require.NoError(t, err)
			require.NoError(t, scriptResult.Error)

		}
	}
}
