package sqlite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	file, err := os.CreateTemp("", "test.sqlite")
	require.NoError(t, err)

	store, err := New(file.Name())
	require.NoError(t, err)

	err = store.Close()
	assert.NoError(t, err)

	_, err = New("/invalidLocation")
	assert.NotContains(
		t,
		err.Error(),
		"unable to open database file: out of memory",
		"should not attempt to open the database file if the location is invalid",
	)
	assert.ErrorContains(
		t,
		err,
		"no such file or directory",
		"should return an error indicating the location is invalid",
	)
}
