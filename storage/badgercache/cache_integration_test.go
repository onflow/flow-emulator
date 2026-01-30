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

package badgercache

import (
	"os"
	"path/filepath"
	"testing"

	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCache_ChainIDAndBlockIDFormat verifies cache directories use correct chainID and blockID format
func TestCache_ChainIDAndBlockIDFormat(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := Config{
		Enabled:      true,
		BaseDir:      tmpDir,
		MaxSize:      DefaultMaxSizeBytes,
		TotalMaxSize: DefaultTotalMaxSize,
		TTL:          DefaultTTL,
		MaxEntries:   DefaultMaxEntries,
	}

	testCases := []struct {
		chainID     flowgo.ChainID
		blockID     string
		expectedDir string
	}{
		{
			chainID:     flowgo.Mainnet,
			blockID:     "abc123def456789012345678901234567890abcdef1234567890123456789abcd",
			expectedDir: "flow-mainnet-abc123def456",
		},
		{
			chainID:     flowgo.Testnet,
			blockID:     "9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba",
			expectedDir: "flow-testnet-9876543210fe",
		},
		{
			chainID:     flowgo.Emulator,
			blockID:     "deadbeef00001111222233334444555566667777888899990000aaaabbbbcccc",
			expectedDir: "flow-emulator-deadbeef0000",
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.chainID)+"-"+tc.blockID, func(t *testing.T) {
			cache, err := New(tc.chainID.String(), tc.blockID, config, &logger)
			require.NoError(t, err)
			require.NotNil(t, cache)
			defer cache.Close()

			// Verify directory created with correct name
			expectedPath := filepath.Join(tmpDir, tc.expectedDir)
			_, err = os.Stat(expectedPath)
			assert.NoError(t, err, "Cache directory should exist: %s", expectedPath)

			// Verify cache path is correct
			assert.Contains(t, cache.path, tc.expectedDir)
		})
	}
}
