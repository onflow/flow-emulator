/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-emulator/storage"
	"golang.org/x/exp/slices"
)

type BlockResponse struct {
	Height  int    `json:"height"`
	BlockId string `json:"blockId"`
	Context string `json:"context,omitempty"`
}

type EmulatorAPIServer struct {
	router  *mux.Router
	server  *EmulatorServer
	backend *backend.Backend
	storage *Storage
}

func NewEmulatorAPIServer(server *EmulatorServer, backend *backend.Backend, storage *Storage) *EmulatorAPIServer {
	router := mux.NewRouter().StrictSlash(true)
	r := &EmulatorAPIServer{router: router,
		server:  server,
		backend: backend,
		storage: storage,
	}

	router.HandleFunc("/emulator/newBlock", r.CommitBlock)

	router.HandleFunc("/emulator/snapshots", r.SnapshotCreate).Methods("POST")
	router.HandleFunc("/emulator/snapshots", r.SnapshotList).Methods("GET")
	router.HandleFunc("/emulator/snapshots/{name}", r.SnapshotJump).Methods("PUT")

	return r
}

func (m EmulatorAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.ServeHTTP(w, r)
}

func (m EmulatorAPIServer) CommitBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m.backend.CommitBlock()

	header, _, err := m.backend.GetLatestBlockHeader(r.Context(), true)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	blockResponse := &BlockResponse{
		Height:  int(header.Height),
		BlockId: header.ID().String(),
	}

	err = json.NewEncoder(w).Encode(blockResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

}
func (m EmulatorAPIServer) SnapshotList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	snapshotProvider, isSnapshotProvider := (*m.storage).Store().(storage.SnapshotProvider)
	if !isSnapshotProvider || !snapshotProvider.SupportSnapshotsWithCurrentConfig() {
		m.server.logger.Error().Msg("State management is not available with current storage backend")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	snapshots, err := snapshotProvider.Snapshots()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bytes, err := json.Marshal(snapshots)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes)

}

func (m EmulatorAPIServer) reloadBlockchainFromSnapshot(name string, snapshotProvider storage.Store, w http.ResponseWriter) {
	blockchain, err := configureBlockchain(m.server.config, snapshotProvider)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m.backend.SetEmulator(blockchain)
	block, err := blockchain.GetLatestBlock()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	blockResponse := &BlockResponse{
		Height:  int(block.Header.Height),
		BlockId: block.Header.ID().String(),
		Context: name,
	}

	bytes, err := json.Marshal(blockResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes)

}

func (m EmulatorAPIServer) SnapshotJump(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	name := vars["name"]

	snapshotProvider, isSnapshotProvider := (*m.storage).Store().(storage.SnapshotProvider)
	if !isSnapshotProvider || !snapshotProvider.SupportSnapshotsWithCurrentConfig() {
		m.server.logger.Error().Msg("State management is not available with current storage backend")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	snapshots, err := snapshotProvider.Snapshots()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !slices.Contains(snapshots, name) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = snapshotProvider.JumpToSnapshot(name, false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m.reloadBlockchainFromSnapshot(name, (*m.storage).Store(), w)

}

func (m EmulatorAPIServer) SnapshotCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.FormValue("name")

	if name == "" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	snapshotProvider, isSnapshotProvider := (*m.storage).Store().(storage.SnapshotProvider)
	if !isSnapshotProvider || !snapshotProvider.SupportSnapshotsWithCurrentConfig() {
		m.server.logger.Error().Msg("State management is not available with current storage backend")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	snapshots, err := snapshotProvider.Snapshots()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if slices.Contains(snapshots, name) {
		w.WriteHeader(http.StatusConflict)
		return
	}
	err = snapshotProvider.JumpToSnapshot(name, true)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m.reloadBlockchainFromSnapshot(name, (*m.storage).Store(), w)

}
