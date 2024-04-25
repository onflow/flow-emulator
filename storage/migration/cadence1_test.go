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
	_ "embed"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/onflow/flow-go/cmd/util/ledger/util/snapshot"
	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/storage/sqlite"
)

var flowTokenAddress = func() common.Address {
	address, _ := common.HexToAddress("0ae53cb6e3f42a79")
	return address
}()

var testAccountAddress = func() common.Address {
	address, _ := common.HexToAddress("01cf0e2f2f715450")
	return address
}()

//go:embed test-data/test_contract_upgraded.cdc
var testContractUpdated string

//go:embed test-data/test_contract_old.cdc
var testContractOld string

const emulatorStateFile = "test-data/emulator_state_cadence_v0.42.6"

func createEmulatorStateTempCopy(t *testing.T) string {
	tempEmulatorState, err := os.CreateTemp("test-data", "temp_emulator_state")
	require.NoError(t, err)
	defer func() {
		err = tempEmulatorState.Close()
		require.NoError(t, err)
	}()

	tempEmulatorStatePath := tempEmulatorState.Name()

	content, err := os.ReadFile(emulatorStateFile)
	require.NoError(t, err)

	_, err = tempEmulatorState.Write(content)
	require.NoError(t, err)
	return tempEmulatorStatePath
}

func migrateEmulatorState(t *testing.T, store *sqlite.Store) {

	logWriter := &writer{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	rwf := &NOOPReportWriterFactory{}
	stagedContracts := stagedContracts()

	err := MigrateCadence1(
		store,
		t.TempDir(),
		migrations.EVMContractChangeNone,
		migrations.BurnerContractChangeDeploy,
		stagedContracts,
		rwf,
		logger,
	)
	require.NoError(t, err)
	require.Empty(t, logWriter.logs)
}

func TestCadence1Migration(t *testing.T) {

	// Work on a temp copy of the state,
	// since the migration will be updating the state.
	tempEmulatorStatePath := createEmulatorStateTempCopy(t)
	defer func() {
		err := os.Remove(tempEmulatorStatePath)
		require.NoError(t, err)
	}()

	store, err := sqlite.New(tempEmulatorStatePath)
	require.NoError(t, err)
	defer func() {
		err := store.Close()
		require.NoError(t, err)
	}()

	// Check payloads before migration
	checkPayloadsBeforeMigration(t, store)

	// Migrate
	migrateEmulatorState(t, store)

	// Re-load the store from the file.
	migratedStore, err := sqlite.New(tempEmulatorStatePath)
	require.NoError(t, err)
	defer func() {
		err = migratedStore.Close()
		require.NoError(t, err)
	}()

	// Reload the payloads from the emulator state (store)
	// and check whether the registers have migrated properly.
	checkMigratedPayloads(t, migratedStore)
}

func checkPayloadsBeforeMigration(t *testing.T, store *sqlite.Store) {
	payloads, _, _, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	require.NoError(t, err)

	for _, payload := range payloads {
		key, err := payload.Key()
		require.NoError(t, err)

		// Check if the old contract uses old syntax,
		// that are not supported in Cadence 1.0.
		if string(key.KeyParts[1].Value) == "code.Test" {
			assert.Equal(t, testContractOld, string(payload.Value()))
			return
		}
	}
}

func checkMigratedPayloads(t *testing.T, store *sqlite.Store) {

	payloads, _, _, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	require.NoError(t, err)

	migratorRuntime, err := migrations.NewMigratorRuntime(
		payloads,
		flowgo.Emulator,
		migrations.MigratorRuntimeConfig{},
		snapshot.LargeChangeSetOrReadonlySnapshot,
	)
	require.NoError(t, err)

	storageMap := migratorRuntime.Storage.GetStorageMap(testAccountAddress, common.PathDomainStorage.Identifier(), false)
	require.NotNil(t, storageMap)

	require.Equal(t, 12, int(storageMap.Count()))

	iterator := storageMap.Iterator(migratorRuntime.Interpreter)

	fullyEntitledAccountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.FullyEntitledAccountReferenceType)
	accountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.AccountReferenceType)

	var values []interpreter.Value
	for key, value := iterator.Next(); key != nil; key, value = iterator.Next() {
		values = append(values, value)
	}

	testContractLocation := common.NewAddressLocation(
		nil,
		testAccountAddress,
		"Test",
	)

	flowTokenLocation := common.NewAddressLocation(
		nil,
		flowTokenAddress,
		"FlowToken",
	)

	fooInterfaceType := interpreter.NewInterfaceStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.Foo",
	)

	barInterfaceType := interpreter.NewInterfaceStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.Bar",
	)

	bazInterfaceType := interpreter.NewInterfaceStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.Baz",
	)

	rResourceType := interpreter.NewCompositeStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.R",
	)

	entitlementAuthorization := func() interpreter.EntitlementSetAuthorization {
		return interpreter.NewEntitlementSetAuthorization(
			nil,
			func() (entitlements []common.TypeID) {
				return []common.TypeID{
					testContractLocation.TypeID(nil, "Test.E"),
				}
			},
			1,
			sema.Conjunction,
		)
	}

	expectedValues := []interpreter.Value{
		// Both string values should be in the normalized form.
		interpreter.NewUnmeteredStringValue("Caf\u00E9"),
		interpreter.NewUnmeteredStringValue("Caf\u00E9"),

		interpreter.NewUnmeteredTypeValue(fullyEntitledAccountReferenceType),

		interpreter.NewDictionaryValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeString,
				interpreter.PrimitiveStaticTypeInt,
			),
			interpreter.NewUnmeteredStringValue("Caf\u00E9"),
			interpreter.NewUnmeteredIntValueFromInt64(1),
			interpreter.NewUnmeteredStringValue("H\u00E9llo"),
			interpreter.NewUnmeteredIntValueFromInt64(2),
		),

		interpreter.NewDictionaryValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeInt,
			),
			interpreter.NewUnmeteredTypeValue(
				&interpreter.IntersectionStaticType{
					Types: []*interpreter.InterfaceStaticType{
						fooInterfaceType,
						barInterfaceType,
					},
					LegacyType: interpreter.PrimitiveStaticTypeAnyStruct,
				},
			),
			interpreter.NewUnmeteredIntValueFromInt64(1),
			interpreter.NewUnmeteredTypeValue(
				&interpreter.IntersectionStaticType{
					Types: []*interpreter.InterfaceStaticType{
						fooInterfaceType,
						barInterfaceType,
						bazInterfaceType,
					},
					LegacyType: interpreter.PrimitiveStaticTypeAnyStruct,
				},
			),
			interpreter.NewUnmeteredIntValueFromInt64(2),
		),

		interpreter.NewCompositeValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			testContractLocation,
			"Test.R",
			common.CompositeKindResource,
			[]interpreter.CompositeField{
				{
					Value: interpreter.NewUnmeteredUInt64Value(11457157452030541824),
					Name:  "uuid",
				},
			},
			testAccountAddress,
		),

		interpreter.NewUnmeteredSomeValueNonCopying(
			interpreter.NewUnmeteredCapabilityValue(
				interpreter.NewUnmeteredUInt64Value(2),
				interpreter.NewAddressValue(nil, testAccountAddress),
				interpreter.NewReferenceStaticType(nil, entitlementAuthorization(), rResourceType),
			),
		),

		interpreter.NewUnmeteredCapabilityValue(
			interpreter.NewUnmeteredUInt64Value(2),
			interpreter.NewAddressValue(nil, testAccountAddress),
			interpreter.NewReferenceStaticType(nil, entitlementAuthorization(), rResourceType),
		),

		interpreter.NewDictionaryValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeInt,
			),
			interpreter.NewUnmeteredTypeValue(fullyEntitledAccountReferenceType),
			interpreter.NewUnmeteredIntValueFromInt64(1),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Capabilities),
			interpreter.NewUnmeteredIntValueFromInt64(2),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_AccountCapabilities),
			interpreter.NewUnmeteredIntValueFromInt64(3),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_StorageCapabilities),
			interpreter.NewUnmeteredIntValueFromInt64(4),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Contracts),
			interpreter.NewUnmeteredIntValueFromInt64(5),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Keys),
			interpreter.NewUnmeteredIntValueFromInt64(6),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Inbox),
			interpreter.NewUnmeteredIntValueFromInt64(7),
			interpreter.NewUnmeteredTypeValue(accountReferenceType),
			interpreter.NewUnmeteredIntValueFromInt64(8),
			interpreter.NewUnmeteredTypeValue(interpreter.AccountKeyStaticType),
			interpreter.NewUnmeteredIntValueFromInt64(9),
		),

		interpreter.NewDictionaryValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeString,
			),
			interpreter.NewUnmeteredTypeValue(
				interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			),
			interpreter.NewUnmeteredStringValue("non_auth_ref"),
		),
		interpreter.NewDictionaryValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeString,
			),
			interpreter.NewUnmeteredTypeValue(
				interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			),
			interpreter.NewUnmeteredStringValue("auth_ref"),
		),

		interpreter.NewCompositeValue(
			migratorRuntime.Interpreter,
			interpreter.EmptyLocationRange,
			flowTokenLocation,
			"FlowToken.Vault",
			common.CompositeKindResource,
			[]interpreter.CompositeField{
				{
					Value: interpreter.NewUnmeteredUFix64Value(0.001 * sema.Fix64Factor),
					Name:  "balance",
				},
				{
					Value: interpreter.NewUnmeteredUInt64Value(8791026472627208194),
					Name:  "uuid",
				},
			},
			testAccountAddress,
		),
	}

	require.Equal(t, len(expectedValues), len(values))

	// Order is non-deterministic, so do a greedy compare.
	for _, value := range values {
		found := false
		actualValue := value.(interpreter.EquatableValue)
		for i, expectedValue := range expectedValues {
			if actualValue.Equal(migratorRuntime.Interpreter, interpreter.EmptyLocationRange, expectedValue) {
				expectedValues = append(expectedValues[:i], expectedValues[i+1:]...)
				found = true
				break
			}

		}
		if !found {
			assert.Fail(t, fmt.Sprintf("extra item in actual values: %s", actualValue))
		}
	}

	if len(expectedValues) != 0 {
		assert.Fail(t, fmt.Sprintf("%d extra item(s) in expected values: %s", len(expectedValues), expectedValues))
	}
}

func stagedContracts() []migrations.StagedContract {
	return []migrations.StagedContract{
		{
			Contract: migrations.Contract{
				Name: "Test",
				Code: []byte(testContractUpdated),
			},
			Address: testAccountAddress,
		},
	}
}

type writer struct {
	logs []string
}

var _ io.Writer = &writer{}

func (w *writer) Write(p []byte) (n int, err error) {
	w.logs = append(w.logs, string(p))
	return len(p), nil
}

func TestLoadMigratedValuesInTransaction(t *testing.T) {

	t.Parallel()

	// Create a temporary snapshot file to work on.
	tempEmulatorStatePath := createEmulatorStateTempCopy(t)
	defer func() {
		err := os.Remove(tempEmulatorStatePath)
		require.NoError(t, err)
	}()

	store, err := sqlite.New(tempEmulatorStatePath)
	require.NoError(t, err)
	defer func() {
		err := store.Close()
		require.NoError(t, err)
	}()

	// Check payloads before migration
	checkPayloadsBeforeMigration(t, store)

	// Migrate
	migrateEmulatorState(t, store)

	// Re-load the store from the file.
	migratedStore, err := sqlite.New(tempEmulatorStatePath)
	require.NoError(t, err)
	defer func() {
		err = migratedStore.Close()
		require.NoError(t, err)
	}()

	// Execute a transaction against the migrated storage.

	blockchain, err := emulator.New(
		emulator.WithStore(migratedStore),
		emulator.WithTransactionValidationEnabled(false),
		emulator.WithTransactionFeesEnabled(false),
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	txCode := `
        import Test from 0x01cf0e2f2f715450

        transaction {

            prepare(acct: auth(Storage) &Account) {
                var string1 = acct.storage.load<String>(from: /storage/string_value_1)
                log(string1)

                var string2 = acct.storage.load<String>(from: /storage/string_value_2)
                log(string2)

                var typeValue = acct.storage.load<Type>(from: /storage/type_value)
                log(typeValue)

                var dict1 = acct.storage.load<{String: Int}>(from: /storage/dictionary_with_string_keys)
                log(dict1)

                var dict2 = acct.storage.load<{Type: Int}>(from: /storage/dictionary_with_restricted_typed_keys)
                log(dict2)

                destroy acct.storage.load<@Test.R>(from: /storage/r)

                var cap1 = acct.storage.load<Capability<&Test.R>?>(from: /storage/capability)
                log(cap1)

                var cap2 = acct.storage.load<Capability<auth(Test.E) &Test.R>>(from: /storage/untyped_capability)
                log(cap2)

                var dict3 = acct.storage.load<{Type: String}>(from: /storage/dictionary_with_reference_typed_key)
                log(dict3)

                var dict4 = acct.storage.load<{Type: String}>(from: /storage/dictionary_with_auth_reference_typed_key)
                log(dict4)
            }

            execute {}
        }
    `

	tx := flowsdk.NewTransaction().
		SetScript([]byte(txCode)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(blockchain.ServiceKey().Address, blockchain.ServiceKey().Index, blockchain.ServiceKey().SequenceNumber).
		SetPayer(blockchain.ServiceKey().Address).
		AddAuthorizer(flowsdk.Address(testAccountAddress))

	signer, err := blockchain.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(blockchain.ServiceKey().Address, blockchain.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Submit tx
	blockchainLogger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&blockchainLogger, blockchain)
	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	// Execute tx
	result, err := blockchain.ExecuteNextTransaction()
	require.NoError(t, err)

	if !assert.True(t, result.Succeeded()) {
		t.Error(result.Error)
	}

	require.Equal(t,
		[]string{
			`"Caf\u{e9}"`,
			`"Caf\u{e9}"`,
			`Type<auth(Storage, Contracts, Keys, Inbox, Capabilities) &Account>()`,
			`{"H\u{e9}llo": 2, "Caf\u{e9}": 1}`,
			`{Type<{A.01cf0e2f2f715450.Test.Bar, A.01cf0e2f2f715450.Test.Foo}>(): 1, Type<{A.01cf0e2f2f715450.Test.Foo, A.01cf0e2f2f715450.Test.Bar, A.01cf0e2f2f715450.Test.Baz}>(): 2}`,
			`Capability<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(address: 0x01cf0e2f2f715450, id: 2)`,
			`Capability<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(address: 0x01cf0e2f2f715450, id: 2)`,
			`{Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(): "non_auth_ref"}`,
			`{Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(): "auth_ref"}`,
		},
		result.Logs,
	)

	_, err = blockchain.CommitBlock()
	require.NoError(t, err)

	// tx status becomes TransactionStatusSealed
	tx1Result, err := adapter.GetTransactionResult(context.Background(), tx.ID())
	require.NoError(t, err)
	require.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)
}
