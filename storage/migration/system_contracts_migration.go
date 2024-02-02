/*
 * Flow Emulator
 *
 * Copyright 2024 Dapper Labs, Inc.
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

package migration

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/storage/sqlite"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const chain = flow.Emulator

func MigrateSystemContracts(store *sqlite.Store) error {
	payloads, payloadInfo, accounts, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	if err != nil {
		return err
	}

	logger := NewConsoleLogger()

	payloads, err = migrateSystemContracts(logger, accounts, payloads)
	if err != nil {
		return err
	}

	return WritePayloadsToSnapshot(store, payloads, payloadInfo)
}

func migrateSystemContracts(
	logger zerolog.Logger,
	accounts []common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	migration := migrations.ChangeContractCodeMigration{}

	systemContractChanges := migrations.SystemContractChanges(chain)

	for _, contractChange := range systemContractChanges {
		migration.RegisterContractChange(
			contractChange.Address,
			contractChange.ContractName,
			contractChange.NewContractCode,
		)
	}

	err := migration.InitMigration(logger, nil, 0)
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		ctx := context.Background()
		payloads, err = migration.MigrateAccount(ctx, account, payloads)
		if err != nil {
			return nil, err
		}
	}

	// Add new contracts
	newlyIntroducedContracts := migrations.NewlyAddedSystemContracts(chain)
	payloads = append(payloads, newlyIntroducedContracts...)

	return payloads, nil
}
