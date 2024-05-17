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
	"runtime"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/storage/sqlite"
)

func MigrateCadence1(
	store *sqlite.Store,
	outputDir string,
	evmContractChange migrations.EVMContractChange,
	burnerContractChange migrations.BurnerContractChange,
	stagedContracts []migrations.StagedContract,
	rwf reporters.ReportWriterFactory,
	logger zerolog.Logger,

) error {
	payloads, payloadInfo, _, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	if err != nil {
		return err
	}

	nWorker := runtime.NumCPU()

	cadence1Migrations := migrations.NewCadence1Migrations(
		logger,
		outputDir,
		rwf,
		migrations.Options{
			NWorker:                           nWorker,
			DiffMigrations:                    false,
			LogVerboseDiff:                    false,
			CheckStorageHealthBeforeMigration: false,
			ChainID:                           flow.Emulator,
			EVMContractChange:                 evmContractChange,
			BurnerContractChange:              burnerContractChange,
			StagedContracts:                   stagedContracts,
			Prune:                             false,
			MaxAccountSize:                    0,
		},
	)

	byAccountRegisters, err := registers.NewByAccountFromPayloads(payloads)
	if err != nil {
		return err
	}

	for _, migration := range cadence1Migrations {
		logger.Info().Str("migration", migration.Name).Msg("running migration")
		err = migration.Migrate(byAccountRegisters)
		if err != nil {
			return err
		}
	}

	newPayloads := byAccountRegisters.DestructIntoPayloads(nWorker)

	return WritePayloadsToSnapshot(store, newPayloads, payloadInfo)
}
