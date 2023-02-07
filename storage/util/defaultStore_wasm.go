//go:build wasm
// +build wasm

package util

import (
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/memstore"
	"github.com/onflow/flow-emulator/storage/redis"
)

func CreateDefaultStorage() (storage.Store, error) {
	return memstore.New(), nil
}

func NewSqliteStorage(url string) (storage.Store, error) {
	return memstore.New(), nil
}

func NewRedisStorage(url string) (storage.Store, error) {
	return redis.New(url)
}
