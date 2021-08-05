/*
 * Flow Emulator
 *
 * Copyright 2019-2021 Dapper Labs, Inc.
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

	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.Headers = &headers{}

type headers struct {
	blockchain *Blockchain
}

func (h headers) IndexByChunkID(_, _ flowgo.Identifier) error {
	panic("should not be called")
}

func (h headers) BatchIndexByChunkID(_, _ flowgo.Identifier, batch storage.BatchStorage) error {
	panic("should not be called")
}

func (h headers) IDByChunkID(_ flowgo.Identifier) (flowgo.Identifier, error) {
	panic("should not be called")
}

func newHeaders(b *Blockchain) *headers {
	return &headers{b}
}

func (h headers) Store(_ *flowgo.Header) error {
	return errors.New("not implemented")
}

func (h headers) ByBlockID(blockID flowgo.Identifier) (*flowgo.Header, error) {
	block, err := h.blockchain.storage.BlockByID(blockID)
	if err != nil {
		return nil, err
	}
	return block.Header, nil
}

func (h headers) ByHeight(height uint64) (*flowgo.Header, error) {
	block, err := h.blockchain.storage.BlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return block.Header, nil
}

func (h headers) ByParentID(_ flowgo.Identifier) ([]*flowgo.Header, error) {
	return nil, errors.New("not implemented")
}
