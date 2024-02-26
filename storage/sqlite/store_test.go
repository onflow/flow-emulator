/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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
