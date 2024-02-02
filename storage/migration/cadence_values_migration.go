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
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
)

func MigrateCadenceValues(
	store *sqlite.Store,
	rwf reporters.ReportWriterFactory,
	logger zerolog.Logger,
) error {
	payloads, payloadInfo, accounts, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	if err != nil {
		return err
	}

	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}

	payloads, err = migrateLinkValues(rwf, logger, capabilityIDs, accounts, payloads)
	if err != nil {
		return err
	}

	payloads, err = migrateCadenceValues(rwf, logger, capabilityIDs, accounts, payloads)
	if err != nil {
		return err
	}

	return WritePayloadsToSnapshot(store, payloads, payloadInfo)
}

func migrateLinkValues(
	rwf reporters.ReportWriterFactory,
	logger zerolog.Logger,
	capabilityIDs map[interpreter.AddressPath]interpreter.UInt64Value,
	accounts []common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	linkValueMigration := migrations.NewCadenceLinkValueMigrator(rwf, capabilityIDs)

	err := linkValueMigration.InitMigration(logger, nil, 0)
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		ctx := context.Background()
		payloads, err = linkValueMigration.MigrateAccount(ctx, account, payloads)
		if err != nil {
			return nil, err
		}
	}
	return payloads, nil
}

func migrateCadenceValues(
	rwf reporters.ReportWriterFactory,
	logger zerolog.Logger,
	capabilityIDs map[interpreter.AddressPath]interpreter.UInt64Value,
	accounts []common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	valueMigration := migrations.NewCadenceValueMigrator(
		rwf,
		capabilityIDs,
		func(staticType *interpreter.CompositeStaticType) interpreter.StaticType {
			return nil
		},
		func(staticType *interpreter.InterfaceStaticType) interpreter.StaticType {
			return nil
		},
	)

	err := valueMigration.InitMigration(logger, nil, 0)
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		ctx := context.Background()
		payloads, err = valueMigration.MigrateAccount(ctx, account, payloads)
		if err != nil {
			return nil, err
		}
	}

	return payloads, nil
}
