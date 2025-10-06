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

package storage

import (
	"testing"

	"github.com/onflow/flow-go-sdk/test"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils/unittest"
)

func TestEncodeTransaction(t *testing.T) {

	t.Parallel()

	tx := unittest.TransactionFixture()
	data, err := encodeTransaction(tx)
	require.Nil(t, err)

	var decodedTx flowgo.TransactionBody
	err = decodeTransaction(&decodedTx, data)
	require.Nil(t, err)

	assert.Equal(t, tx.ID(), decodedTx.ID())
}

func TestEncodeTransactionResult(t *testing.T) {

	t.Parallel()

	test := func(eventEncodingVersion entities.EventEncodingVersion) {

		t.Run(eventEncodingVersion.String(), func(t *testing.T) {
			t.Parallel()

			result := unittest.StorableTransactionResultFixture(eventEncodingVersion)

			data, err := encodeTransactionResult(result)
			require.Nil(t, err)

			var decodedResult types.StorableTransactionResult
			err = decodeTransactionResult(&decodedResult, data)
			require.Nil(t, err)

			assert.Equal(t, result, decodedResult)
		})
	}

	test(entities.EventEncodingVersion_CCF_V0)
	test(entities.EventEncodingVersion_JSON_CDC_V0)
}

func TestEncodeBlock(t *testing.T) {

	t.Parallel()

	ids := test.IdentifierGenerator()

	block, err := flowgo.NewBlock(flowgo.UntrustedBlock{
		HeaderBody: flowgo.HeaderBody{
			Height:             1234,
			ParentID:           flowgo.Identifier(ids.New()),
			ChainID:            flowgo.Emulator,
			Timestamp:          uint64(flowgo.GenesisTime.UnixMilli()),
			View:               1,
			ParentVoterIndices: []uint8{0},
			ParentVoterSigData: []byte{0},
			ProposerID:         flowgo.Identifier(ids.New()),
		},
		Payload: flowgo.Payload{
			ProtocolStateID: flowgo.Identifier(ids.New()),
			Guarantees: []*flowgo.CollectionGuarantee{
				{
					CollectionID: flowgo.Identifier(ids.New()),
				},
			},
		},
	})
	require.Nil(t, err)

	data, err := encodeBlock(*block)
	require.Nil(t, err)

	var decodedBlock flowgo.Block
	err = decodeBlock(&decodedBlock, data)
	require.Nil(t, err)

	assert.Equal(t, block.ID(), decodedBlock.ID())
	assert.Equal(t, *block.ToHeader(), *decodedBlock.ToHeader())
	assert.Equal(t, block.Payload, decodedBlock.Payload)
}

func TestEncodeGenesisBlock(t *testing.T) {

	t.Parallel()

	block := Genesis(flowgo.Emulator)

	data, err := encodeBlock(*block)
	require.Nil(t, err)

	var decodedBlock flowgo.Block
	err = decodeBlock(&decodedBlock, data)
	require.Nil(t, err)

	assert.Equal(t, block.ID(), decodedBlock.ID())
	assert.Equal(t, *block.ToHeader(), *decodedBlock.ToHeader())
	assert.Equal(t, block.Payload, decodedBlock.Payload)
}

func TestEncodeEvents(t *testing.T) {

	t.Parallel()

	test := func(eventEncodingVersion entities.EventEncodingVersion) {

		t.Run(eventEncodingVersion.String(), func(t *testing.T) {
			t.Parallel()

			event1, _ := convert.SDKEventToFlow(test.EventGenerator(eventEncodingVersion).New())
			event2, _ := convert.SDKEventToFlow(test.EventGenerator(eventEncodingVersion).New())

			events := []flowgo.Event{
				event1,
				event2,
			}

			data, err := encodeEvents(events)
			require.Nil(t, err)

			var decodedEvents []flowgo.Event
			err = decodeEvents(&decodedEvents, data)
			require.Nil(t, err)
			assert.Equal(t, events, decodedEvents)
		})
	}

	test(entities.EventEncodingVersion_CCF_V0)
	test(entities.EventEncodingVersion_JSON_CDC_V0)
}

// Helper (TODO: @jribbink delete later)
func Genesis(chainID flowgo.ChainID) *flowgo.Block {
	// create the headerBody
	headerBody := flowgo.HeaderBody{
		ChainID:   chainID,
		ParentID:  flowgo.ZeroID,
		Height:    0,
		Timestamp: uint64(flowgo.GenesisTime.UnixMilli()),
		View:      0,
	}

	// combine to block
	block := &flowgo.Block{
		HeaderBody: headerBody,
		Payload:    *flowgo.NewEmptyPayload(),
	}

	return block
}
