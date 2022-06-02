package emulator

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	sdk "github.com/onflow/flow-go-sdk"
)

type StorageItem map[string]interpreter.Value

func (s StorageItem) Get(key string) interpreter.Value {
	return s[key]
}

type AccountStorage struct {
	Address sdk.Address
	Private StorageItem
	Public  StorageItem
	Storage StorageItem
}

func NewAccountStorage(address sdk.Address, storage *runtime.Storage) (*AccountStorage, error) {
	addr, err := common.BytesToAddress(address.Bytes())
	if err != nil {
		return nil, err
	}

	extractStorage := func(path common.PathDomain) StorageItem {
		storageMap := storage.GetStorageMap(addr, path.Identifier(), false)
		if storageMap == nil {
			return nil
		}

		iterator := storageMap.Iterator(nil)
		values := make(StorageItem)
		k, v := iterator.Next()
		for v != nil {
			values[k] = v
			k, v = iterator.Next()
		}
		return values
	}

	return &AccountStorage{
		Address: address,
		Private: extractStorage(common.PathDomainPrivate),
		Public:  extractStorage(common.PathDomainPublic),
		Storage: extractStorage(common.PathDomainStorage),
	}, nil
}
