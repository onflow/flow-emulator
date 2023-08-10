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

package liveness

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_BasicCheck(t *testing.T) {

	t.Parallel()

	mc := NewCheckCollector(time.Millisecond * 20)
	if !mc.IsLive(0) {
		t.Errorf("Multicheck with no checks should always pass")
	}

	c1 := mc.NewCheck()
	if !mc.IsLive(0) {
		t.Errorf("Just made check should pass")
	}

	time.Sleep(time.Millisecond * 30)
	if mc.IsLive(0) {
		t.Errorf("Multi check should have failed")
	}

	c1.CheckIn()
	if !mc.IsLive(0) {
		t.Errorf("Checker should passed after checkin")
	}

	c2 := mc.NewCheck()
	if !mc.IsLive(0) {
		t.Errorf("Just made check 2 should pass")
	}

	time.Sleep(time.Millisecond * 30)
	c1.CheckIn()
	// don't checkIn c2

	if mc.IsLive(0) {
		t.Errorf("Multi check should have failed by c2")
	}

	c2.CheckIn()
	if !mc.IsLive(0) {
		t.Errorf("Check 2 should pass after checkin")
	}
}

func Test_CheckHTTP(t *testing.T) {

	t.Parallel()

	c := NewCheckCollector(time.Millisecond * 20)
	r := httptest.NewRequest(http.MethodGet, "/live", nil)
	wr := httptest.NewRecorder()

	c1 := c.NewCheck()
	_ = c.NewCheck() // never check-in c2

	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusOK {
		t.Errorf("Check should have passed")
	}

	time.Sleep(time.Millisecond * 30)
	c1.CheckIn()

	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusServiceUnavailable {
		t.Errorf("Check should not have passed")
	}
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
	if wr.Code != http.StatusOK {
		t.Errorf("Check should have passed")
	}

	time.Sleep(time.Millisecond * 60)

	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusOK {
		t.Errorf("Check should still have passed")
	}

	r.Header.Del(ToleranceHeader)
	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusServiceUnavailable {
		t.Errorf("Check should not have passed")
	}
}
