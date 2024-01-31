package sqlite

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateMigration(t *testing.T) {
	path := "test-data/emulator.sqlite"

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	err = Migrate(db)
	require.NoError(t, err)
}
