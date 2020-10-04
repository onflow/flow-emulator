package memstore

import (
	"sync"
	"testing"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemstore(t *testing.T) {
	const blockHeight = 0
	const key = "foo"
	value := []byte("bar")

	store := New()

	err := store.UnsafeInsertLedgerDelta(
		blockHeight,
		delta.Delta{
			Data: map[string]flowgo.RegisterValue{
				string(state.RegisterID("", "", key)): value,
			},
		},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			view := store.LedgerViewByHeight(blockHeight)
			actualValue, err := view.Get("", "", key)
			require.NoError(t, err)

			require.NoError(t, err)
			assert.Equal(t, value, actualValue)
		}()
	}

	wg.Wait()
}
