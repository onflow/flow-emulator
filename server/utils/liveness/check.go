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
	"sync"
	"time"
)

// Check is a heartbeat style liveness reporter.
//
// It is not guaranteed to be safe to CheckIn across multiple goroutines.
// IsLive must be safe to be called concurrently with CheckIn.
type Check interface {
	CheckIn()
	IsLive(time.Duration) bool
}

// internalCheck implements a Check.
type internalCheck struct {
	lock             sync.RWMutex
	lastCheckIn      time.Time
	defaultTolerance time.Duration
}

// CheckIn adds a heartbeat at the current time.
func (c *internalCheck) CheckIn() {
	c.lock.Lock()
	c.lastCheckIn = time.Now()
	c.lock.Unlock()
}

// IsLive checks if we are still live against the given the tolerance between hearbeats.
//
// If tolerance is 0, the default tolerance is used.
func (c *internalCheck) IsLive(tolerance time.Duration) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if tolerance == 0 {
		tolerance = c.defaultTolerance
	}

	return c.lastCheckIn.Add(tolerance).After(time.Now())
}
