//go:build !wasm
// +build !wasm

package util

import (
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/redis"
	"github.com/onflow/flow-emulator/storage/sqlite"
)

func CreateDefaultStorage() (storage.Store, error) {
	return sqlite.New(":memory:")
}

func NewSqliteStorage(url string) (storage.Store, error) {
	return sqlite.New(url)
}

func NewRedisStorage(url string) (storage.Store, error) {
	return redis.New(url)
}
