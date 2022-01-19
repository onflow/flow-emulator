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
	"time"

	"github.com/onflow/flow-emulator/server/backend"
)

type BlocksTicker struct {
	backend *backend.Backend
	ticker  *time.Ticker
	done    chan bool
}

func NewBlocksTicker(
	backend *backend.Backend,
	blockTime time.Duration,
) *BlocksTicker {
	return &BlocksTicker{
		backend: backend,
		ticker:  time.NewTicker(blockTime),
		done:    make(chan bool, 1),
	}
}

func (t *BlocksTicker) Start() error {
	for {
		select {
		case <-t.ticker.C:
			t.backend.CommitBlock()
		case <-t.done:
			return nil
		}
	}
}

func (t *BlocksTicker) Stop() {
	t.done <- true
}
