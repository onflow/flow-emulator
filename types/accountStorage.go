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
package types

import (
	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

type StorageItem map[string]cadence.Value

func (s StorageItem) Get(key string) cadence.Value {
	return s[key]
}

// NewAccountStorage creates an instance of the storage that holds the values for each storage path.
func NewAccountStorage(
	account *flow.Account,
	address sdk.Address,
	private StorageItem,
	public StorageItem,
	storage StorageItem,
) (*AccountStorage, error) {
	return &AccountStorage{
		Account: account,
		Address: address,
		Private: private,
		Public:  public,
		Storage: storage,
	}, nil
}

type AccountStorage struct {
	Address sdk.Address
	Private StorageItem
	Public  StorageItem
	Storage StorageItem
	Account *flow.Account
}
