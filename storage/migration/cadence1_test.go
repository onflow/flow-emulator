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
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/util"

	"github.com/onflow/flow-emulator/storage/sqlite"
)

var flowTokenAddress = func() common.Address {
	address, _ := common.HexToAddress("0ae53cb6e3f42a79")
	return address
}()

const testAccountAddress = "01cf0e2f2f715450"

func TestCadence1Migration(t *testing.T) {
	const emulatorStateFile = "test-data/emulator_state_cadence_v0.42.6"

	// Work on a temp copy of the state,
	// since the migration will be updating the state.
	tempEmulatorState, err := os.CreateTemp("test-data", "temp_emulator_state")
	require.NoError(t, err)

	tempEmulatorStatePath := tempEmulatorState.Name()

	defer tempEmulatorState.Close()
	defer os.Remove(tempEmulatorStatePath)

	content, err := os.ReadFile(emulatorStateFile)
	require.NoError(t, err)

	_, err = tempEmulatorState.Write(content)
	require.NoError(t, err)

	// Migrate

	store, err := sqlite.New(tempEmulatorStatePath)
	require.NoError(t, err)

	logWriter := &writer{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	// Then migrate the values.
	rwf := &NOOPReportWriterFactory{}
	err = MigrateCadence1(store, nil, rwf, logger)
	require.NoError(t, err)

	// Reload the payloads from the emulator state (store)
	// and check whether the registers have migrated properly.
	checkMigratedPayloads(t, store)

	err = store.Close()
	require.NoError(t, err)

	require.Empty(t, logWriter.logs)
}

func checkMigratedPayloads(t *testing.T, store *sqlite.Store) {
	address, err := common.HexToAddress(testAccountAddress)
	if err != nil {
		panic(err)
	}

	payloads, _, _, err := util.PayloadsAndAccountsFromEmulatorSnapshot(store.DB())
	require.NoError(t, err)

	migratorRuntime, err := migrations.NewMigratorRuntime(
		address,
		payloads,
		util.RuntimeInterfaceConfig{},
	)
	require.NoError(t, err)

	storageMap := migratorRuntime.Storage.GetStorageMap(address, common.PathDomainStorage.Identifier(), false)
	require.NotNil(t, storageMap)

	//require.Equal(t, 12, int(storageMap.Count()))

	iterator := storageMap.Iterator(migratorRuntime.Interpreter)

	fullyEntitledAccountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.FullyEntitledAccountReferenceType)
	accountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.AccountReferenceType)

	var values []interpreter.Value
	for key, value := iterator.Next(); key != nil; key, value = iterator.Next() {
		values = append(values, value)
	}

	testContractLocation := common.NewAddressLocation(
		nil,
		address,
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

	//entitlementAuthorization := func() interpreter.EntitlementSetAuthorization {
	//	return interpreter.NewEntitlementSetAuthorization(
	//		nil,
	//		func() (entitlements []common.TypeID) {
	//			return []common.TypeID{
	//				testContractLocation.TypeID(nil, "Test.E"),
	//			}
	//		},
	//		1,
	//		sema.Conjunction,
	//	)
	//}

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
					Value: interpreter.NewUnmeteredUInt64Value(1369094286720630784),
					Name:  "uuid",
				},
			},
			address,
		),

		interpreter.NewUnmeteredSomeValueNonCopying(
			interpreter.NewUnmeteredCapabilityValue(
				interpreter.NewUnmeteredUInt64Value(2),
				interpreter.NewAddressValue(nil, address),
				interpreter.NewReferenceStaticType(nil, interpreter.UnauthorizedAccess, rResourceType),
			),
		),

		// TODO: Untyped Capability is not migrated?
		//interpreter.NewUnmeteredCapabilityValue(
		//	interpreter.NewUnmeteredUInt64Value(2),
		//	interpreter.NewAddressValue(nil, address),
		//	interpreter.NewReferenceStaticType(nil, interpreter.UnauthorizedAccess, rResourceType),
		//),

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
					interpreter.UnauthorizedAccess,
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
					interpreter.UnauthorizedAccess,
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
					Value: interpreter.NewUnmeteredUInt64Value(12321848580485677058),
					Name:  "uuid",
				},
			},
			address,
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

type writer struct {
	logs []string
}

var _ io.Writer = &writer{}

func (w *writer) Write(p []byte) (n int, err error) {
	w.logs = append(w.logs, string(p))
	return len(p), nil
}
