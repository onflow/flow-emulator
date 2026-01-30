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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
)

const (
	DefaultCacheDir     = ".flow-fork-cache"
	DefaultMaxSizeBytes = 500 * 1024 * 1024      // 500 MB per cache
	DefaultTotalMaxSize = 2 * 1024 * 1024 * 1024 // 2 GB total
	DefaultTTL          = 30 * 24 * time.Hour    // 30 days
	DefaultMaxEntries   = 10                     // 10 caches max
)

type Config struct {
	Enabled      bool
	BaseDir      string
	MaxSize      int64
	TotalMaxSize int64
	TTL          time.Duration
	MaxEntries   int
}

type Cache struct {
	db     *badger.DB
	config Config
	logger *zerolog.Logger
	path   string
}

// New creates a new Badger cache for fork register data
func New(network string, blockID string, config Config, logger *zerolog.Logger) (*Cache, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Create base directory
	if err := os.MkdirAll(config.BaseDir, 0755); err != nil {
		logger.Warn().Err(err).Msg("Failed to create cache directory, fork cache disabled")
		return nil, nil
	}

	// Prune old caches before opening new one
	if err := pruneCache(config.BaseDir, config, logger); err != nil {
		logger.Warn().Err(err).Msg("Failed to prune cache")
	}

	// Create cache subdirectory using blockID instead of height
	// Format: network-blockID (e.g., flow-mainnet-abc123def456)
	// Truncate blockID to 12 chars (like GitHub commits) for readability
	truncatedID := blockID
	if len(blockID) > 12 {
		truncatedID = blockID[:12]
	}
	subdir := fmt.Sprintf("%s-%s", network, truncatedID)
	cachePath := filepath.Join(config.BaseDir, subdir)

	if err := os.MkdirAll(cachePath, 0755); err != nil {
		logger.Warn().Err(err).Msg("Failed to create cache subdirectory")
		return nil, nil
	}

	// Open Badger DB
	opts := badger.DefaultOptions(cachePath)
	opts.Logger = nil // Disable badger's verbose logging

	db, err := badger.Open(opts)
	if err != nil {
		logger.Warn().Err(err).Str("path", cachePath).Msg("Failed to open cache, continuing without cache")
		return nil, nil
	}

	cache := &Cache{
		db:     db,
		config: config,
		logger: logger,
		path:   cachePath,
	}

	// Update access time
	updateAccessTime(cachePath)

	logger.Info().
		Str("path", cachePath).
		Msg("Fork register cache enabled")

	return cache, nil
}

// Get retrieves a value from the cache
func (c *Cache) Get(ctx context.Context, key []byte) ([]byte, error) {
	if c == nil || c.db == nil {
		return nil, fmt.Errorf("cache not initialized")
	}

	var value []byte
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		value, err = item.ValueCopy(nil)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return nil, nil
	}

	return value, err
}

// Set stores a value in the cache
func (c *Cache) Set(ctx context.Context, key []byte, value []byte) error {
	if c == nil || c.db == nil {
		return nil // Silently skip if cache disabled
	}

	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Close closes the cache
func (c *Cache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// GetStats returns cache statistics
func (c *Cache) GetStats() map[string]interface{} {
	if c == nil || c.db == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	lsm, vlog := c.db.Size()

	return map[string]interface{}{
		"enabled":    true,
		"path":       c.path,
		"size_mb":    (lsm + vlog) / (1024 * 1024),
		"lsm_bytes":  lsm,
		"vlog_bytes": vlog,
	}
}

// IsEnabled returns whether caching is enabled
func IsEnabled() bool {
	// Check if explicitly disabled
	if os.Getenv("FLOW_NO_CACHE") != "" {
		return false
	}
	return true
}

// GetDefaultConfig returns default cache configuration
func GetDefaultConfig() Config {
	baseDir := DefaultCacheDir
	if dir := os.Getenv("FLOW_CACHE_DIR"); dir != "" {
		baseDir = dir
	}

	return Config{
		Enabled:      IsEnabled(),
		BaseDir:      baseDir,
		MaxSize:      DefaultMaxSizeBytes,
		TotalMaxSize: DefaultTotalMaxSize,
		TTL:          DefaultTTL,
		MaxEntries:   DefaultMaxEntries,
	}
}

// updateAccessTime updates the access time for a cache directory
func updateAccessTime(cachePath string) {
	now := time.Now()
	os.Chtimes(cachePath, now, now)
}

// getDirSize returns the size of a directory in bytes
func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// getCacheInfo returns information about a cache directory
type cacheInfo struct {
	path       string
	accessTime time.Time
	size       int64
}

// getCaches returns all cache directories with their metadata
func getCaches(baseDir string) ([]cacheInfo, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var caches []cacheInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(baseDir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		caches = append(caches, cacheInfo{
			path:       path,
			accessTime: info.ModTime(),
			size:       getDirSize(path),
		})
	}

	return caches, nil
}

// pruneCache removes old caches based on configured limits
func pruneCache(baseDir string, config Config, logger *zerolog.Logger) error {
	caches, err := getCaches(baseDir)
	if err != nil {
		return err
	}

	if len(caches) == 0 {
		return nil
	}

	// Sort by access time (oldest first)
	for i := 0; i < len(caches); i++ {
		for j := i + 1; j < len(caches); j++ {
			if caches[i].accessTime.After(caches[j].accessTime) {
				caches[i], caches[j] = caches[j], caches[i]
			}
		}
	}

	// Prune by age
	cutoff := time.Now().Add(-config.TTL)
	for i := 0; i < len(caches); {
		if caches[i].accessTime.Before(cutoff) {
			logger.Info().
				Str("path", filepath.Base(caches[i].path)).
				Msg("Pruning old cache")
			os.RemoveAll(caches[i].path)
			caches = append(caches[:i], caches[i+1:]...)
		} else {
			i++
		}
	}

	// Prune by count (keep newest)
	if len(caches) > config.MaxEntries {
		for i := 0; i < len(caches)-config.MaxEntries; i++ {
			logger.Info().
				Str("path", filepath.Base(caches[i].path)).
				Msg("Pruning excess cache")
			os.RemoveAll(caches[i].path)
		}
		caches = caches[len(caches)-config.MaxEntries:]
	}

	// Prune by total size (oldest first)
	var totalSize int64
	for _, cache := range caches {
		totalSize += cache.size
	}

	for i := 0; i < len(caches) && totalSize > config.TotalMaxSize; i++ {
		logger.Info().
			Str("path", filepath.Base(caches[i].path)).
			Int64("freed_mb", caches[i].size/(1024*1024)).
			Msg("Pruning cache for size limit")
		os.RemoveAll(caches[i].path)
		totalSize -= caches[i].size
	}

	return nil
}
