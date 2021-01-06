/*
 * Flow Emulator
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/fvm"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/storage"
)

var _ fvm.Blocks = &blocks{}
var _ access.Blocks = &blocks{}

type blocks struct {
	blockchain *Blockchain
}

func newBlocks(b *Blockchain) *blocks {
	return &blocks{b}
}

func (b *blocks) ByHeightFrom(height uint64, header *flowgo.Header) (*flowgo.Header, error) {
	if header == nil {
		byHeight, err := b.blockchain.storage.BlockByHeight(height)
		if err != nil {
			return nil, err
		}
		return byHeight.Header, nil
	}

	if header.Height == height {
		return header, nil
	}

	if height > header.Height {
		return nil, fmt.Errorf("requested height (%d) larger than given header's height (%d)", height, header.Height)
	}

	id := header.ParentID

	// travel chain back
	for {
		// recent block should be in cache so this is supposed to be fast
		parent, err := b.blockchain.storage.BlockByID(id)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve block parent: %w", err)
		}
		if parent.Header.Height == height {
			return parent.Header, nil
		}

		_, err = b.blockchain.storage.BlockByHeight(parent.Header.Height)
		// if height isn't finalized, move to parent
		if err != nil && errors.Is(err, storage.ErrNotFound) {
			id = parent.Header.ParentID
			continue
		}
		// any other error bubbles up
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve block parent: %w", err)
		}
		//if parent is finalized block, we can just use finalized chain
		// to get desired height

		block, err := b.blockchain.storage.BlockByHeight(height)
		if err != nil {
			return nil, err
		}
		return block.Header, nil
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
