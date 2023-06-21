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

	for _, chain := range chains {
		contracts := emulator.NewCommonContracts(chain)

		b, err := emulator.New(
			emulator.Contracts(contracts),
			emulator.WithChainID(chain.ChainID()),
		)
		require.NoError(t, err)

		for _, contract := range contracts {

			require.Equal(t, contract.Address.Hex(), chain.ServiceAddress().Hex())

			scriptCode := fmt.Sprintf(`
			pub fun main() {
				getAccount(0x%s).contracts.get(name: "%s") ?? panic("contract is not deployed")
	    	}`, contract.Address, contract.Name)

			scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
			require.NoError(t, err)
			require.NoError(t, scriptResult.Error)

		}
	}
}
