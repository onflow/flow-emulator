/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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
package emulator

import (
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
			exportedValue, err := runtime.ExportValue(v, inter, interpreter.EmptyLocationRange)
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
