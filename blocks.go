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

package emulator

import (
	"errors"

	"github.com/onflow/flow-go/access"

	"github.com/onflow/flow-go/fvm"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/storage"
)

var _ fvm.Blocks = &blocks{}
var _ access.Blocks = &blocks{}

type blocks struct {
	blockchain *Blockchain
	headers    *headers
}

func newBlocks(b *Blockchain) *blocks {
	return &blocks{
		blockchain: b,
		headers:    newHeaders(b),
	}
}

func (b *blocks) HeaderByID(id flowgo.Identifier) (*flowgo.Header, error) {
	block, err := b.blockchain.storage.BlockByID(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return block.Header, nil
}

func (b *blocks) FinalizedHeader() (*flowgo.Header, error) {
	block, err := b.blockchain.storage.LatestBlock()
	if err != nil {
		return nil, err
	}

	return block.Header, nil
}

func (b *blocks) ByHeightFrom(height uint64, header *flowgo.Header) (*flowgo.Header, error) {
	return fvm.NewBlockFinder(b.headers).
		ByHeightFrom(height, header)
}
