package badger

import (
	"fmt"

	"github.com/dapperlabs/flow-go-sdk"
)

const (
	blockKeyPrefix           = "block_by_height"
	blockIDIndexKeyPrefix    = "block_id_to_height"
	transactionKeyPrefix     = "transaction_by_id"
	ledgerKeyPrefix          = "ledger_by_block_height" // TODO remove
	eventsKeyPrefix          = "events_by_block_height"
	ledgerChangelogKeyPrefix = "ledger_changelog_by_register_id"
	ledgerValueKeyPrefix     = "ledger_value_by_block_height_register_id"
)

// The following *Key functions return keys to use when reading/writing values
// to Badger. The key name describes how the value is indexed (eg. by block
// height or by ID).
//
// Keys for which numeric ordering is defined, (eg. block height), have the
// numeric component of the key left-padded with zeros (%032d) so that
// lexicographic ordering matches numeric ordering.

func blockKey(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", blockKeyPrefix, blockHeight))
}

func blockIDIndexKey(blockID flow.Identifier) []byte {
	return []byte(fmt.Sprintf("%s-%x", blockIDIndexKeyPrefix, blockID))
}

func latestBlockKey() []byte {
	return []byte("latest_block_height")
}

func transactionKey(txID flow.Identifier) []byte {
	return []byte(fmt.Sprintf("%s-%x", transactionKeyPrefix, txID))
}

func eventsKey(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", eventsKeyPrefix, blockHeight))
}

// TODO remove this
func ledgerKey(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", ledgerKeyPrefix, blockHeight))
}

func ledgerChangelogKey(registerID string) []byte {
	return []byte(fmt.Sprintf("%s-%s", ledgerChangelogKeyPrefix, registerID))
}

func ledgerValueKey(registerID string, blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%s-%032d", ledgerValueKeyPrefix, registerID, blockHeight))
}

// blockHeightFromEventsKey recovers the block height from an event key.
func blockHeightFromEventsKey(key []byte) uint64 {
	var blockHeight uint64
	_, _ = fmt.Sscanf(string(key), eventsKeyPrefix+"-%032d", &blockHeight)
	return blockHeight
}

// registerIDFromLedgerChangelogKey recovers the register ID from a ledger
// changelog key.
func registerIDFromLedgerChangelogKey(key []byte) string {
	var registerID string
	_, _ = fmt.Sscanf(string(key), ledgerChangelogKeyPrefix+"-%s", &registerID)
	return registerID
}
