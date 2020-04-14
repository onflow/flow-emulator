package badger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Keys that include a block height should be ordered lexicographically, as
// this is how Badger sorts when iterating over keys.
// More information here: https://github.com/dgraph-io/badger/issues/317
func TestKeyOrdering(t *testing.T) {
	// create a list of heights in increasing order, this test will check the
	// corresponding keys are also in increasing lexicographic order
	nums := []uint64{0, 1, 2, 3, 10, 29, 50, 99, 100, 1000, 1234, 100000000, 19825983621301235}

	t.Run("block key", func(t *testing.T) {
		var keys [][]byte
		for _, num := range nums {
			keys = append(keys, blockKey(num))
		}
		for i := 0; i < len(keys)-1; i++ {
			// lower index keys should be considered less
			assert.Equal(t, -1, bytes.Compare(keys[i], keys[i+1]))
		}
	})

	t.Run("registers key", func(t *testing.T) {
		var keys [][]byte
		for _, num := range nums {
			keys = append(keys, ledgerKey(num))
		}
		for i := 0; i < len(keys)-1; i++ {
			// lower index keys should be considered less
			assert.Equal(t, -1, bytes.Compare(keys[i], keys[i+1]))
		}
	})

	t.Run("event key", func(t *testing.T) {
		var keys [][]byte
		for _, num := range nums {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					keys = append(keys, eventKey(num, i, j, "foo"))
				}
			}
		}

		for i := 0; i < len(keys)-1; i++ {
			// lower index keys should be considered less
			assert.Equal(t, -1, bytes.Compare(keys[i], keys[i+1]))
		}
	})
}
