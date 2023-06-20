package emulator_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonContractsDeployment(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.Contracts(emulator.CommonContracts),
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
			"\"A.f8d6e0586b0a20c7.FUSD\"",
			"\"A.f8d6e0586b0a20c7.NonFungibleToken\"",
			"\"A.f8d6e0586b0a20c7.MetadataViews\"",
			"\"A.f8d6e0586b0a20c7.ExampleNFT\"",
			"\"A.f8d6e0586b0a20c7.NFTStorefrontV2\"",
			"\"A.f8d6e0586b0a20c7.NFTStorefront\"",
		},
		scriptResult.Logs,
	)
	assert.Equal(t, cadence.NewBool(true), scriptResult.Value)
}
