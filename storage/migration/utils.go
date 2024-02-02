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
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/sqlite"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func WritePayloadsToSnapshot(
	store *sqlite.Store,
	payloads []*ledger.Payload,
	payloadInfoSet map[flow.RegisterID]util.PayloadMetaInfo,
) error {

	const storeName = storage.LedgerStoreName

	ctx := context.TODO()

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
			// Insert the values back with the existing height and version.
			err = store.SetBytesWithVersionAndHeight(
				ctx,
				storeName,
				registerIdBytes,
				value,
				payloadInfo.Version,
				payloadInfo.Height,
			)
		} else {
			// If this is a new payload, use the current block height, and default version.
			err = store.SetBytes(
				ctx,
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

func NewConsoleLogger() zerolog.Logger {
	writer := zerolog.ConsoleWriter{
		Out: os.Stdout,
	}

	return zerolog.New(writer).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)
}

type NOOPReportWriterFactory struct{}

func (*NOOPReportWriterFactory) ReportWriter(_ string) reporters.ReportWriter {
	return &NOOPWriter{}
}

type NOOPWriter struct{}

var _ reporters.ReportWriter = &NOOPWriter{}

func (*NOOPWriter) Write(_ any) {
	// NO-OP
}

func (r *NOOPWriter) Close() {}
