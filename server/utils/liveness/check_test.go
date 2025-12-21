/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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

package liveness

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_BasicCheck(t *testing.T) {

	t.Parallel()

	mc := NewCheckCollector(time.Millisecond * 20)
	assert.True(t, mc.IsLive(0))

	c1 := mc.NewCheck()
	assert.True(t, mc.IsLive(0))

	time.Sleep(time.Millisecond * 30)
	assert.False(t, mc.IsLive(0))

	c1.CheckIn()
	assert.True(t, mc.IsLive(0))

	c2 := mc.NewCheck()
	assert.True(t, mc.IsLive(0))

	time.Sleep(time.Millisecond * 30)
	c1.CheckIn()
	// don't checkIn c2

	assert.False(t, mc.IsLive(0))

	c2.CheckIn()
	assert.True(t, mc.IsLive(0))
}

func Test_CheckHTTP(t *testing.T) {

	t.Parallel()

	c := NewCheckCollector(time.Millisecond * 20)
	r := httptest.NewRequest(http.MethodGet, "/live", nil)
	wr := httptest.NewRecorder()

	c1 := c.NewCheck()
	_ = c.NewCheck() // never check-in c2

	c.ServeHTTP(wr, r)
	assert.Equal(t, http.StatusOK, wr.Code)

	time.Sleep(time.Millisecond * 30)
	c1.CheckIn()

	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	assert.Equal(t, http.StatusServiceUnavailable, wr.Code)
}

func Test_CheckHTTPOverride(t *testing.T) {

	t.Parallel()

	c := NewCheckCollector(time.Millisecond * 20)
	r := httptest.NewRequest(http.MethodGet, "/live", nil)
	r.Header.Add(ToleranceHeader, "30s")
	wr := httptest.NewRecorder()

	c1 := c.NewCheck()

	c1.CheckIn()

	c.ServeHTTP(wr, r)
	assert.Equal(t, http.StatusOK, wr.Code)

	time.Sleep(time.Millisecond * 60)

	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	assert.Equal(t, http.StatusOK, wr.Code)

	r.Header.Del(ToleranceHeader)
	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	assert.Equal(t, http.StatusServiceUnavailable, wr.Code)
}
