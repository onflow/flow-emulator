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
	"sync"
	"time"
)

const (
	// DefaultTolerance is the default time (in seconds) allowed between heartbeats.
	DefaultTolerance = time.Second * 30

	// ToleranceHeader is the HTTP header name used to override a collector's configured tolerance.
	ToleranceHeader = "X-Liveness-Tolerance"
)

// CheckCollector produces multiple checks and returns
// live only if all descendant checks are live.
//
// Each child check may only be used by one goroutine,
// the CheckCollector may be used by multiple routines at once to
// produce checks.
type CheckCollector struct {
	lock             sync.RWMutex
	defaultTolerance time.Duration
	checks           []Check
}

// NewCheckCollector creates a threadsafe Collector.
func NewCheckCollector(tolerance time.Duration) *CheckCollector {
	if tolerance == 0 {
		tolerance = DefaultTolerance
	}

	return &CheckCollector{
		defaultTolerance: tolerance,
		checks:           make([]Check, 0, 1),
	}
}

// NewCheck returns a Check which is safe for use on a single routine.
func (c *CheckCollector) NewCheck() Check {
	c.lock.Lock()
	check := &internalCheck{
		defaultTolerance: c.defaultTolerance,
		lastCheckIn:      time.Now(),
	}
	c.checks = append(c.checks, check)
	c.lock.Unlock()
	return check
}

// Register adds a check to a collector
func (c *CheckCollector) Register(ck Check) {
	c.lock.Lock()
	c.checks = append(c.checks, ck)
	c.lock.Unlock()
}

func (c *CheckCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tolerance := c.defaultTolerance
	var err error

	if toleranceStr := r.Header.Get(ToleranceHeader); toleranceStr != "" {
		tolerance, err = time.ParseDuration(toleranceStr)
		if err != nil {
			http.Error(w, "Invalid tolerance: "+toleranceStr, http.StatusBadRequest)
			return
		}
	}

	if !c.IsLive(tolerance) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// IsLive checks if we are still live against the given the tolerance between hearbeats.
//
// If tolerance is 0, the default tolerance is used.
func (c *CheckCollector) IsLive(tolerance time.Duration) bool {
	if tolerance == 0 {
		tolerance = c.defaultTolerance
	}

	c.lock.RLock()
	defer c.lock.RUnlock()

	for i := range c.checks {
		if !c.checks[i].IsLive(tolerance) {
			return false
		}
	}

	return true
}
