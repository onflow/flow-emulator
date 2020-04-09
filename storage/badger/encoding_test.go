package badger

import (
	"testing"

	"github.com/dapperlabs/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go-sdk"

	"github.com/dapperlabs/flow-emulator/types"
	"github.com/dapperlabs/flow-emulator/utils/unittest"
)

func TestEncodeTransaction(t *testing.T) {
	tx := unittest.TransactionFixture()
	data, err := encodeTransaction(tx)
	require.Nil(t, err)

	var decodedTx flow.Transaction
	err = decodeTransaction(&decodedTx, data)
	require.Nil(t, err)

	assert.Equal(t, tx.ID(), decodedTx.ID())
}

func TestEncodeBlock(t *testing.T) {
	ids := test.IdentifierGenerator()

	block := types.Block{
		Height:         1234,
		ParentID:       ids.New(),
		TransactionIDs: []flow.Identifier{ids.New()},
	}
	data, err := encodeBlock(block)
	require.Nil(t, err)

	var decodedBlock types.Block
	err = decodeBlock(&decodedBlock, data)
	require.Nil(t, err)

	assert.Equal(t, block.Height, decodedBlock.Height)
	assert.Equal(t, block.ParentID, decodedBlock.ParentID)
	assert.Equal(t, block.TransactionIDs, decodedBlock.TransactionIDs)
}

func TestEncodeEventList(t *testing.T) {
	eventList := []flow.Event{unittest.EventFixture(func(e *flow.Event) {})}
	data, err := encodeEvents(eventList)
	require.Nil(t, err)

	var decodedEventList []flow.Event
	err = decodeEvents(&decodedEventList, data)
	require.Nil(t, err)
	assert.Equal(t, eventList, decodedEventList)
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
