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
	"math"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/storage/sqlite"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/model/flow"
)

func MigrateCadence1(
	store *sqlite.Store,
	stagedContracts []migrations.StagedContract,
	rwf reporters.ReportWriterFactory,
	logger zerolog.Logger,
) error {
	payloads, payloadInfo, _, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	if err != nil {
		return err
	}

	// TODO: >1 breaks atree storage map iteration
	//   and requires LinkValueMigration.LinkValueMigration to be thread-safe
	const nWorker = 1

	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = migrations.EVMContractChangeNone

	const burnerContractChange = migrations.BurnerContractChangeDeploy

	const maxAccountSize = math.MaxUint64

	cadence1Migrations := migrations.NewCadence1Migrations(
		logger,
		rwf,
		nWorker,
		flow.Emulator,
		false,
		false,
		evmContractChange,
		burnerContractChange,
		stagedContracts,
		false,
		maxAccountSize,
	)

	for _, migration := range cadence1Migrations {
		payloads, err = migration.Migrate(payloads)
		if err != nil {
			return err
		}
	}

	return WritePayloadsToSnapshot(store, payloads, payloadInfo)
}
