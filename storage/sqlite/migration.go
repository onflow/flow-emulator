package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/storage"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func Migrate(url string) error {
	store, err := New(url)
	if err != nil {
		return err
	}

	payloads, payloadInfo, accounts, err := payloadsAndAccountsFromSnapshot(store.db)
	if err != nil {
		return err
	}

	rwf := &ReportWriterFactory{}
	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}
	logger := newConsoleLogger()

	payloads, err = migrateLinkValues(rwf, logger, capabilityIDs, accounts, payloads)
	if err != nil {
		return err
	}

	payloads, err = migrateCadenceValues(rwf, logger, capabilityIDs, accounts, payloads)
	if err != nil {
		return err
	}

	return writePayloadsToSnapshot(store, payloads, payloadInfo)
}

func migrateLinkValues(
	rwf *ReportWriterFactory,
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
	rwf *ReportWriterFactory,
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

func newConsoleLogger() zerolog.Logger {
	writer := zerolog.ConsoleWriter{
		Out: os.Stdout,
	}

	return zerolog.New(writer).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)
}

func writePayloadsToSnapshot(
	store *Store,
	payloads []*ledger.Payload,
	payloadInfoSet map[flow.RegisterID]payloadMetaInfo,
) error {

	const storeName = storage.LedgerStoreName

	for _, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return err
		}

		registerId, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return err
		}

		registerIdBytes := []byte(registerId.String())

		value := payload.Value()

		payloadInfo, ok := payloadInfoSet[registerId]
		if ok {
			_, err = store.db.Exec(
				fmt.Sprintf(
					"INSERT INTO %s (key, version, value, height) VALUES (?, ?, ?, ?) ON CONFLICT(key, version, height) DO UPDATE SET value=excluded.value",
					storeName,
				),
				hex.EncodeToString(registerIdBytes),
				payloadInfo.version,
				hex.EncodeToString(value),
				payloadInfo.height,
			)
		} else {
			// if this is a new payload, use the current block height
			err = store.SetBytes(
				nil,
				storeName,
				registerIdBytes,
				value,
			)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func payloadsAndAccountsFromSnapshot(db *sql.DB) (
	[]*ledger.Payload,
	map[flow.RegisterID]payloadMetaInfo,
	[]common.Address,
	error,
) {
	rows, err := db.Query("SELECT key, value, version, height FROM ledger")
	if err != nil {
		return nil, nil, nil, err
	}

	var payloads []*ledger.Payload
	var accounts []common.Address
	accountsSet := make(map[common.Address]struct{})

	payloadSet := make(map[flow.RegisterID]payloadMetaInfo)

	for rows.Next() {
		var hexKey, hexValue string
		var height, version int

		err := rows.Scan(&hexKey, &hexValue, &height, &version)
		if err != nil {
			return nil, nil, nil, err
		}

		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, nil, nil, err
		}

		value, err := hex.DecodeString(hexValue)
		if err != nil {
			return nil, nil, nil, err
		}

		registerId, address := registerIDKeyFromString(string(key))

		if _, contains := accountsSet[address]; !contains {
			accountsSet[address] = struct{}{}
			accounts = append(accounts, address)
		}

		ledgerKey := convert.RegisterIDToLedgerKey(registerId)

		payload := ledger.NewPayload(
			ledgerKey,
			value,
		)

		payloads = append(payloads, payload)
		payloadSet[registerId] = payloadMetaInfo{
			height:  height,
			version: version,
		}
	}

	return payloads, payloadSet, accounts, nil
}

// registerIDKeyFromString is the inverse of `flow.RegisterID.String()` method.
func registerIDKeyFromString(s string) (flow.RegisterID, common.Address) {
	parts := strings.SplitN(s, "/", 2)

	owner := parts[0]
	key := parts[1]

	address, err := common.HexToAddress(owner)
	if err != nil {
		panic(err)
	}

	var decodedKey string

	switch key[0] {
	case '$':
		b := make([]byte, 9)
		b[0] = '$'

		int64Value, err := strconv.ParseInt(key[1:], 10, 64)
		if err != nil {
			panic(err)
		}

		binary.BigEndian.PutUint64(b[1:], uint64(int64Value))

		decodedKey = string(b)
	case '#':
		decoded, err := hex.DecodeString(key[1:])
		if err != nil {
			panic(err)
		}
		decodedKey = string(decoded)
	default:
		panic("Invalid register key")
	}

	return flow.RegisterID{
			Owner: string(address.Bytes()),
			Key:   decodedKey,
		},
		address
}

type payloadMetaInfo struct {
	height, version int
}

type ReportWriterFactory struct{}

func (_m *ReportWriterFactory) ReportWriter(_ string) reporters.ReportWriter {
	return &NOOPWriter{}
}

type NOOPWriter struct{}

var _ reporters.ReportWriter = &NOOPWriter{}

func (r *NOOPWriter) Write(_ any) {
	// NO-OP
}

func (r *NOOPWriter) Close() {}
