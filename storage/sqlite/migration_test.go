package sqlite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateMigration(t *testing.T) {
	const emulatorStateFile = "test-data/emulator.sqlite"

	// Work on a temp copy of the state,
	// since the migration will be updating the state.
	tempEmulatorState, err := os.CreateTemp("test-data", "tmp-emulator.state")
	require.NoError(t, err)

	tempEmulatorStatePath := tempEmulatorState.Name()

	defer func() {
		err := tempEmulatorState.Close()
		require.NoError(t, err)
	}()

	defer func() {
		err := os.Remove(tempEmulatorStatePath)
		require.NoError(t, err)
	}()

	content, err := os.ReadFile(emulatorStateFile)
	require.NoError(t, err)

	_, err = tempEmulatorState.Write(content)
	require.NoError(t, err)

	// Migrate

	err = Migrate(tempEmulatorStatePath)
	require.NoError(t, err)

}
