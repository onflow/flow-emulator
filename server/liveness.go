/*
 * Flow Emulator
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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

package server

import (
	"net/http"
	"time"

	"github.com/onflow/flow-emulator/server/liveness"
)

type LivenessTicker struct {
	collector *liveness.CheckCollector
	ticker    *time.Ticker
	done      chan bool
}

func NewLivenessTicker(tolerance time.Duration) *LivenessTicker {
	return &LivenessTicker{
		collector: liveness.NewCheckCollector(tolerance),
		ticker:    time.NewTicker(tolerance / 2),
		done:      make(chan bool, 1),
	}
}

func (l *LivenessTicker) Start() error {
	check := l.collector.NewCheck()

	for {
		select {
		case <-l.ticker.C:
			check.CheckIn()
		case <-l.done:
			return nil
		}
	}
}

func (l *LivenessTicker) Stop() {
	l.done <- true
}

func (l *LivenessTicker) Handler() http.Handler {
	return l.collector
}
