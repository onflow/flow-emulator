package emulator

import (
	"encoding/json"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

type StorageItem map[string]cadence.Value

func (s StorageItem) Get(key string) cadence.Value {
	return s[key]
}

func (s StorageItem) MarshalJSON() ([]byte, error) {
	serialized := make(map[string]interface{})
	for k, v := range s {
		if link, ok := v.(cadence.Link); ok {
			serialized[k] = map[string]string{
				"type":   link.Type().ID(),
				"target": link.TargetPath.String(),
			}
		} else {
			serialized[k] = v.String()
		}
	}
	return json.Marshal(serialized)
}

// NewAccountStorage creates an instance of the storage that holds the values for each storage path.
func NewAccountStorage(
	address sdk.Address,
	account *flow.Account,
	storage *runtime.Storage,
	inter *interpreter.Interpreter,
) (*AccountStorage, error) {
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
			exportedValue, err := runtime.ExportValue(v, inter)
			if err != nil {
				continue // just skip errored value
			}

			values[k] = exportedValue
			k, v = iterator.Next()
		}
		return values
	}

	return &AccountStorage{
		Account: account,
		Address: address,
		Private: extractStorage(common.PathDomainPrivate),
		Public:  extractStorage(common.PathDomainPublic),
		Storage: extractStorage(common.PathDomainStorage),
	}, nil
}

type AccountStorage struct {
	Address sdk.Address
	Private StorageItem
	Public  StorageItem
	Storage StorageItem
	Account *flow.Account
}
