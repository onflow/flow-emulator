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

package badger

import (
	"bytes"
	"fmt"
	"strings"

	flowgo "github.com/onflow/flow-go/model/flow"
)

const (
	blockKeyPrefix             = "block_by_height"
	blockIDIndexKeyPrefix      = "block_id_to_height"
	collectionKeyPrefix        = "collection_by_id"
	transactionKeyPrefix       = "transaction_by_id"
	transactionResultKeyPrefix = "transaction_result_by_id"
	ledgerKeyPrefix            = "ledger_by_block_height" // TODO remove
	eventKeyPrefix             = "event_by_block_height"
	ledgerChangelogKeyPrefix   = "ledger_changelog_by_register_id"
	ledgerValueKeyPrefix       = "ledger_value_by_block_height_register_id"
)

// The following *Key functions return keys to use when reading/writing values
// to Badger. The key name describes how the value is indexed (eg. by block
// height or by ID).
//
// Keys for which numeric ordering is defined, (eg. block height), have the
// numeric component of the key left-padded with zeros (%032d) so that
// lexicographic ordering matches numeric ordering.

func latestBlockKey() []byte {
	return []byte("latest_block_height")
}

func blockKey(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", blockKeyPrefix, blockHeight))
}

func blockIDIndexKey(blockID flowgo.Identifier) []byte {
	return []byte(fmt.Sprintf("%s-%x", blockIDIndexKeyPrefix, blockID))
}

func collectionKey(colID flowgo.Identifier) []byte {
	return []byte(fmt.Sprintf("%s-%x", collectionKeyPrefix, colID))
}

func transactionKey(txID flowgo.Identifier) []byte {
	return []byte(fmt.Sprintf("%s-%x", transactionKeyPrefix, txID))
}

func transactionResultKey(txID flowgo.Identifier) []byte {
	return []byte(fmt.Sprintf("%s-%x", transactionResultKeyPrefix, txID))
}

func eventKey(blockHeight uint64, txIndex, eventIndex uint32, eventType flowgo.EventType) []byte {
	return []byte(fmt.Sprintf(
		"%s-%032d-%032d-%032d-%s",
		eventKeyPrefix,
		blockHeight,
		txIndex,
		eventIndex,
		eventType,
	))
}

func eventKeyBlockPrefix(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf(
		"%s-%032d",
		eventKeyPrefix,
		blockHeight,
	))
}

func eventKeyHasType(key []byte, eventType []byte) bool {
	// event type is at the end of the key, so we can simply compare suffixes
	return bytes.HasSuffix(key, eventType)
}

// TODO remove this
func ledgerKey(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", ledgerKeyPrefix, blockHeight))
}

func ledgerChangelogKey(registerID flowgo.RegisterID) []byte {
	return []byte(fmt.Sprintf("%s-%s-%s",
		ledgerChangelogKeyPrefix,
		registerID.Owner,
		registerID.Key))
}

func ledgerValueKey(registerID flowgo.RegisterID, blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s-%s-%032d", ledgerValueKeyPrefix, registerID.String(), blockHeight))
}

// registerIDFromLedgerChangelogKey recovers the register ID from a ledger
// changelog key.
func registerIDFromLedgerChangelogKey(key []byte) (flowgo.RegisterID, error) {
	registerString := strings.TrimPrefix(string(key), ledgerChangelogKeyPrefix+"-")

	c := strings.Count(registerString, "-")
	if c == 3 { // old format
		parts := strings.SplitN(registerString, "-", 3)
		if len(parts) < 3 {
			return flowgo.RegisterID{}, fmt.Errorf("failed to parse register ID from %s", string(key))
		}
		return flowgo.RegisterID{
			Owner: parts[0],
			Key:   parts[2], // skip the controller
		}, nil
	}
	// else is the new format
	parts := strings.SplitN(registerString, "-", 2)
	if len(parts) < 2 {
		return flowgo.RegisterID{}, fmt.Errorf("failed to parse register ID from %s", string(key))
	}

	return flowgo.RegisterID{
		Owner: parts[0],
		Key:   parts[1],
	}, nil
}
