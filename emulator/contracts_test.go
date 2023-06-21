package emulator_test

import (
	"fmt"
	flowgo "github.com/onflow/flow-go/model/flow"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonContractsDeployment(t *testing.T) {

	t.Parallel()

	//only test monotonic and emulator ( mainnet / testnet is used for remote debugging )
	chains := []flowgo.Chain{
		flowgo.MonotonicEmulator.Chain(),
		flowgo.Emulator.Chain(),
	}

	for _, chain := range chains {

		b, err := emulator.New(
			emulator.Contracts(emulator.NewCommonContracts(chain)),
			emulator.WithChainID(chain.ChainID()),
		)
		require.NoError(t, err)

		serviceAccount := b.ServiceKey().Address.Hex()
		scriptCode := fmt.Sprintf(`
	    import
	        FUSD,
	        NonFungibleToken,
	        MetadataViews,
	        ExampleNFT,
	        NFTStorefrontV2,
	        NFTStorefront
	    from 0x%s

	    pub fun main(): Bool {
	        log(Type<FUSD>().identifier)
	        log(Type<NonFungibleToken>().identifier)
			log(Type<MetadataViews>().identifier)
	        log(Type<ExampleNFT>().identifier)
	        log(Type<NFTStorefrontV2>().identifier)
	        log(Type<NFTStorefront>().identifier)
	        return true
	    }
	`, serviceAccount)

		scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
		require.NoError(t, err)
		assert.ElementsMatch(
			t,
			[]string{
				fmt.Sprintf("\"A.%s.FUSD\"", serviceAccount),
				fmt.Sprintf("\"A.%s.NonFungibleToken\"", serviceAccount),
				fmt.Sprintf("\"A.%s.MetadataViews\"", serviceAccount),
				fmt.Sprintf("\"A.%s.ExampleNFT\"", serviceAccount),
				fmt.Sprintf("\"A.%s.NFTStorefrontV2\"", serviceAccount),
				fmt.Sprintf("\"A.%s.NFTStorefront\"", serviceAccount),
			},
			scriptResult.Logs,
		)
		assert.Equal(t, cadence.NewBool(true), scriptResult.Value)
	}
}
