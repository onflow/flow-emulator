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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_BasicOperations(t *testing.T) {
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

	cache, err := New("flow-mainnet", "abc123def456", config, &logger) // Will be truncated to 12 chars
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	ctx := context.Background()

	// Test Set and Get
	key := []byte("test_register_id")
	value := []byte("test_register_value")

	err = cache.Set(ctx, key, value)
	require.NoError(t, err)

	retrieved, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)

	// Test Get non-existent key
	missing, err := cache.Get(ctx, []byte("missing_key"))
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestCache_Disabled(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := Config{
		Enabled: false,
		BaseDir: tmpDir,
	}

	cache, err := New("flow-mainnet", "abc123def456", config, &logger) // Will be truncated to 12 chars
	require.NoError(t, err)
	assert.Nil(t, cache)
}

func TestCache_SeparateByNetworkAndHeight(t *testing.T) {
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

	// Create cache for mainnet with blockID1
	cache1, err := New("flow-mainnet", "block111", config, &logger)
	require.NoError(t, err)
	require.NotNil(t, cache1)
	defer cache1.Close()

	// Create cache for testnet with same blockID
	cache2, err := New("flow-testnet", "block111", config, &logger)
	require.NoError(t, err)
	require.NotNil(t, cache2)
	defer cache2.Close()

	// Create cache for mainnet with different blockID
	cache3, err := New("flow-mainnet", "block222", config, &logger)
	require.NoError(t, err)
	require.NotNil(t, cache3)
	defer cache3.Close()

	// Verify separate directories created
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// Check directory names
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	assert.True(t, names["flow-mainnet-block111"])
	assert.True(t, names["flow-testnet-block111"])
	assert.True(t, names["flow-mainnet-block222"])
}

func TestCache_Pruning(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := Config{
		Enabled:      true,
		BaseDir:      tmpDir,
		MaxSize:      DefaultMaxSizeBytes,
		TotalMaxSize: DefaultTotalMaxSize,
		TTL:          1 * time.Second, // Short TTL for testing
		MaxEntries:   2,                // Only keep 2 caches
	}

	// Create 3 caches
	cache1, err := New("flow-mainnet", "block100", config, &logger)
	require.NoError(t, err)
	cache1.Close()

	time.Sleep(100 * time.Millisecond)

	cache2, err := New("flow-mainnet", "block200", config, &logger)
	require.NoError(t, err)
	cache2.Close()

	time.Sleep(100 * time.Millisecond)

	cache3, err := New("flow-mainnet", "block300", config, &logger)
	require.NoError(t, err)
	cache3.Close()

	// Pruning happens on next cache creation, so we still have 3
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 3, len(entries))

	// Wait for TTL to expire
	time.Sleep(2 * time.Second)

	// Create new cache, should prune old ones
	cache4, err := New("flow-mainnet", "block400", config, &logger)
	require.NoError(t, err)
	defer cache4.Close()

	entries, err = os.ReadDir(tmpDir)
	require.NoError(t, err)
	// Should only have the newest cache
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "flow-mainnet-block400", entries[0].Name())
}

func TestCache_Stats(t *testing.T) {
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

	cache, err := New("flow-mainnet", "abc123def456", config, &logger) // Will be truncated to 12 chars
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	stats := cache.GetStats()
	assert.True(t, stats["enabled"].(bool))
	assert.Contains(t, stats["path"].(string), "flow-mainnet-abc123def456")
	assert.GreaterOrEqual(t, stats["size_mb"].(int64), int64(0))
}

func TestCache_DefaultConfig(t *testing.T) {
	// Test without env vars
	config := GetDefaultConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, DefaultCacheDir, config.BaseDir)
	assert.Equal(t, int64(DefaultMaxSizeBytes), config.MaxSize)

	// Test with env vars
	os.Setenv("FLOW_CACHE_DIR", "/custom/path")
	os.Setenv("FLOW_NO_CACHE", "1")
	defer os.Unsetenv("FLOW_CACHE_DIR")
	defer os.Unsetenv("FLOW_NO_CACHE")

	config = GetDefaultConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, "/custom/path", config.BaseDir)
}

func TestCache_Persistence(t *testing.T) {
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

	ctx := context.Background()
	key := []byte("persistent_key")
	value := []byte("persistent_value")

	// Create cache and write data
	cache1, err := New("flow-mainnet", "abc123def456", config, &logger)
	require.NoError(t, err)
	require.NotNil(t, cache1)

	err = cache1.Set(ctx, key, value)
	require.NoError(t, err)

	cache1.Close()

	// Reopen cache and verify data persisted
	cache2, err := New("flow-mainnet", "abc123def456", config, &logger)
	require.NoError(t, err)
	require.NotNil(t, cache2)
	defer cache2.Close()

	retrieved, err := cache2.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)
}

func TestCache_CreateDirFailure(t *testing.T) {
	logger := zerolog.Nop()

	// Use a path that can't be created
	config := Config{
		Enabled: true,
		BaseDir: "/root/impossible/path",
	}

	// Should return nil cache without error (graceful degradation)
	cache, err := New("flow-mainnet", "abc123def456", config, &logger) // Will be truncated to 12 chars
	assert.NoError(t, err)
	assert.Nil(t, cache)
}

func TestPruneCache_EmptyDir(t *testing.T) {
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

	// Should not error on empty directory
	err := pruneCache(tmpDir, config, &logger)
	assert.NoError(t, err)
}

func TestCache_ConcurrentAccess(t *testing.T) {
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

	cache, err := New("flow-mainnet", "abc123def456", config, &logger) // Will be truncated to 12 chars
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	ctx := context.Background()

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := []byte(filepath.Join("key", string(rune(n))))
			value := []byte(filepath.Join("value", string(rune(n))))
			cache.Set(ctx, key, value)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all writes succeeded
	for i := 0; i < 10; i++ {
		key := []byte(filepath.Join("key", string(rune(i))))
		value, err := cache.Get(ctx, key)
		assert.NoError(t, err)
		assert.NotNil(t, value)
	}
}
