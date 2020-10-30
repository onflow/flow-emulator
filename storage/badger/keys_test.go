/*
 * Flow Emulator
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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
					keys = append(keys, eventKey(num, uint32(i), uint32(j), "foo"))
				}
			}
		}

		for i := 0; i < len(keys)-1; i++ {
			// lower index keys should be considered less
			assert.Equal(t, -1, bytes.Compare(keys[i], keys[i+1]))
		}
	})
}
