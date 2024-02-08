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
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/storage/sqlite"
)

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
	err = MigrateCadence1(store, rwf, logger)
	require.NoError(t, err)

	require.Empty(t, logWriter.logs)
}

type writer struct {
	logs []string
}

var _ io.Writer = &writer{}

func (w *writer) Write(p []byte) (n int, err error) {
	w.logs = append(w.logs, string(p))
	return len(p), nil
}
