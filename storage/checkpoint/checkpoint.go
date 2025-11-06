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

package checkpoint

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/memstore"
)

// Store is just a memstore, but the starting state is loaded from a checkpoint folder
// any new blocks exist in memory only and are not persisted to disk.
type Store struct {
	// Store is a memstore
	// Theoretically this could also be a persistent store
	*memstore.Store
}

// New returns a new Store implementation.
func New(
	log zerolog.Logger,
	path string,
	stateCommitment string,
	chainID flowgo.ChainID,
) (*Store, error) {
	var err error
	stateCommitmentBytes, err := hex.DecodeString(stateCommitment)
	if err != nil {
		return nil, fmt.Errorf("invalid state commitment hex: %w", err)
	}
	state, err := flowgo.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid state commitment: %w", err)
	}

	store := memstore.New()
	snap, err := loadSnapshotFromCheckpoint(log, path, state)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot from checkpoint: %w", err)
	}

	// pretend this state was the genesis state
	genesis := Genesis(chainID)
	err = store.CommitBlock(
		context.Background(),
		*genesis,
		nil,
		nil,
		nil,
		snap,
		nil,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to commit genesis block: %s", err)
	}

	return &Store{
		Store: store,
	}, nil
}

func loadSnapshotFromCheckpoint(
	log zerolog.Logger,
	dir string,
	targetHash flowgo.StateCommitment,
) (*snapshot.ExecutionSnapshot, error) {
	log.Info().Msg("init WAL")

	diskWal, err := wal.NewDiskWAL(
		log,
		nil,
		metrics.NewNoopCollector(),
		dir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create disk WAL: %w", err)
	}

	log.Info().Msg("init ledger")

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	log.Info().Msg("init compactor")

	compactor, err := complete.NewCompactor(led, diskWal, log, complete.DefaultCacheSize, checkpointDistance, checkpointsToKeep, atomic.NewBool(false), &metrics.NoopCollector{})
	if err != nil {
		return nil, fmt.Errorf("cannot create compactor: %w", err)
	}

	log.Info().Msgf("waiting for compactor to load checkpoint and WAL")

	<-compactor.Ready()

	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	trie, err := led.FindTrieByStateCommit(targetHash)
	if err != nil {
		return nil, fmt.Errorf("cannot find trie by state commitment: %w", err)
	}
	payloads := trie.AllPayloads()

	writeSet := make(map[flowgo.RegisterID]flowgo.RegisterValue, len(payloads))
	for _, p := range payloads {
		id, value, err := convert.PayloadToRegister(p)
		if err != nil {
			return nil, fmt.Errorf("cannot convert payload to register: %w", err)
		}

		writeSet[id] = value
	}

	log.Info().Msg("snapshot loaded")

	// garbage collector should clean up the WAL and the checkpoint

	// only the write set is needed for the emulator
	return &snapshot.ExecutionSnapshot{
		WriteSet: writeSet,
	}, err
}

var _ storage.Store = &Store{}

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
