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
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/storage/sqlite"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
)

func MigrateCadence1(
	store *sqlite.Store,
	rwf reporters.ReportWriterFactory,
	logger zerolog.Logger,
) error {
	payloads, payloadInfo, _, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	if err != nil {
		return err
	}

	// If there are no payloads, there is nothing to migrate.
	if len(payloads) == 0 {
		return nil
	}

	// TODO: >1 breaks atree storage map iteration
	//   and requires LinkValueMigration.LinkValueMigration to be thread-safe
	const nWorker = 1

	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = migrations.EVMContractChangeNone

	// TODO:
	var stagedContracts []migrations.StagedContract

	cadence1Migrations := migrations.NewCadence1Migrations(
		logger,
		rwf,
		nWorker,
		flow.Emulator,
		evmContractChange,
		stagedContracts,
	)

	for _, migration := range cadence1Migrations {
		payloads, err = migration(payloads)
		if err != nil {
			return err
		}
	}

	return WritePayloadsToSnapshot(store, payloads, payloadInfo)
}
