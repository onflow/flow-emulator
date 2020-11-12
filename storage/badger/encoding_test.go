package badger

import (
	"testing"

	"github.com/onflow/flow-go-sdk/test"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	convert "github.com/dapperlabs/flow-emulator/convert/sdk"
	"github.com/dapperlabs/flow-emulator/types"
	"github.com/dapperlabs/flow-emulator/utils/unittest"
)

func TestEncodeTransaction(t *testing.T) {
	tx := unittest.TransactionFixture()
	data, err := encodeTransaction(tx)
	require.Nil(t, err)

	var decodedTx flowgo.TransactionBody
	err = decodeTransaction(&decodedTx, data)
	require.Nil(t, err)

	assert.Equal(t, tx.ID(), decodedTx.ID())
}

func TestEncodeTransactionResult(t *testing.T) {
	result := unittest.StorableTransactionResultFixture()

	data, err := encodeTransactionResult(result)
	require.Nil(t, err)

	var decodedResult types.StorableTransactionResult
	err = decodeTransactionResult(&decodedResult, data)
	require.Nil(t, err)

	assert.Equal(t, result, decodedResult)
}

func TestEncodeBlock(t *testing.T) {
	ids := test.IdentifierGenerator()

	block := flowgo.Block{
		Header: &flowgo.Header{
			Height:   1234,
			ParentID: flowgo.Identifier(ids.New()),
		},
		Payload: &flowgo.Payload{
			Guarantees: []*flowgo.CollectionGuarantee{
				{
					CollectionID: flowgo.Identifier(ids.New()),
				},
			},
		},
	}

	data, err := encodeBlock(block)
	require.Nil(t, err)

	var decodedBlock flowgo.Block
	err = decodeBlock(&decodedBlock, data)
	require.Nil(t, err)

	assert.Equal(t, block.ID(), decodedBlock.ID())
	assert.Equal(t, *block.Header, *decodedBlock.Header)
	assert.Equal(t, *block.Payload, *decodedBlock.Payload)
}
func TestEncodeGenesisBlock(t *testing.T) {
	block := flowgo.Genesis(flowgo.Emulator)

	data, err := encodeBlock(*block)
	require.Nil(t, err)

	var decodedBlock flowgo.Block
	err = decodeBlock(&decodedBlock, data)
	require.Nil(t, err)

	assert.Equal(t, block.ID(), decodedBlock.ID())
	assert.Equal(t, *block.Header, *decodedBlock.Header)
	assert.Equal(t, *block.Payload, *decodedBlock.Payload)
}

func TestEncodeEvent(t *testing.T) {
	event, _ := convert.SDKEventToFlow(test.EventGenerator().New())

	data, err := encodeEvent(event)
	require.Nil(t, err)

	var decodedEvent flowgo.Event
	err = decodeEvent(&decodedEvent, data)
	require.Nil(t, err)
	assert.Equal(t, event, decodedEvent)
}

func TestEncodeChangelist(t *testing.T) {
	var clist changelist
	clist.add(1)

	data, err := encodeChangelist(clist)
	require.NoError(t, err)

	var decodedClist changelist
	err = decodeChangelist(&decodedClist, data)
	require.NoError(t, err)
	assert.Equal(t, clist, decodedClist)
}
