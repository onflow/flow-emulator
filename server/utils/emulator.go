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

package utils

import (
	"encoding/json"
	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
	"net/http"
	"strconv"

	fvmerrors "github.com/onflow/flow-go/fvm/errors"

	"github.com/gorilla/mux"

	"golang.org/x/exp/slices"
)

type BlockResponse struct {
	Height  int    `json:"height"`
	BlockId string `json:"blockId"`
	Context string `json:"context,omitempty"`
}

type EmulatorAPIServer struct {
	router   *mux.Router
	emulator emulator.Emulator
	adapter  *adapters.AccessAdapter
}

func NewEmulatorAPIServer(emulator emulator.Emulator, adapter *adapters.AccessAdapter) *EmulatorAPIServer {
	router := mux.NewRouter().StrictSlash(true)
	r := &EmulatorAPIServer{router: router,
		emulator: emulator,
		adapter:  adapter,
	}

	router.HandleFunc("/emulator/newBlock", r.CommitBlock)

	router.HandleFunc("/emulator/rollback", r.Rollback).Methods("POST")

	router.HandleFunc("/emulator/snapshots", r.SnapshotCreate).Methods("POST")
	router.HandleFunc("/emulator/snapshots", r.SnapshotList).Methods("GET")
	router.HandleFunc("/emulator/snapshots/{name}", r.SnapshotJump).Methods("PUT")

	router.HandleFunc("/emulator/storages/{address}", r.Storage)

	router.HandleFunc("/emulator/config", r.Config)

	router.HandleFunc("/emulator/codeCoverage", r.CodeCoverage).Methods("GET")
	router.HandleFunc("/emulator/codeCoverage/reset", r.ResetCodeCoverage).Methods("PUT")

	return r
}

func (m EmulatorAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.ServeHTTP(w, r)
}

func (m EmulatorAPIServer) Config(w http.ResponseWriter, _ *http.Request) {
	type ConfigInfo struct {
		ServiceKey string `json:"service_key"`
	}

	c := ConfigInfo{
		ServiceKey: m.emulator.ServiceKey().PublicKey.String(),
	}

	s, _ := json.MarshalIndent(c, "", "\t")
	_, _ = w.Write(s)
}

func (m EmulatorAPIServer) CommitBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := m.emulator.CommitBlock()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	block, err := m.emulator.GetLatestBlock()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	blockResponse := &BlockResponse{
		Height:  int(block.Header.Height),
		BlockId: block.Header.ID().String(),
	}

	err = json.NewEncoder(w).Encode(blockResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

}

func (m EmulatorAPIServer) Rollback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.FormValue("height") == "" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	height, err := strconv.ParseUint(r.FormValue("height"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = m.emulator.RollbackToBlockHeight(height)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

}

func (m EmulatorAPIServer) SnapshotList(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	snapshots, err := m.emulator.Snapshots()
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

func (m EmulatorAPIServer) latestBlockResponse(name string, w http.ResponseWriter) {

	block, err := m.emulator.GetLatestBlock()
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

	snapshots, err := m.emulator.Snapshots()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !slices.Contains(snapshots, name) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = m.emulator.LoadSnapshot(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m.latestBlockResponse(name, w)
}

func (m EmulatorAPIServer) SnapshotCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.FormValue("name")

	if name == "" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	snapshots, err := m.emulator.Snapshots()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if slices.Contains(snapshots, name) {
		w.WriteHeader(http.StatusConflict)
		return
	}

	err = m.emulator.CreateSnapshot(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	m.latestBlockResponse(name, w)
}

func (m EmulatorAPIServer) Storage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	address := vars["address"]

	addr := flowgo.HexToAddress(address)

	accountStorage, err := m.emulator.GetAccountStorage(addr)
	if err != nil {
		if fvmerrors.IsAccountNotFoundError(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(accountStorage)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (m EmulatorAPIServer) CodeCoverage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(m.emulator.CoverageReport())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (m EmulatorAPIServer) ResetCodeCoverage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m.emulator.ResetCoverageReport()
	w.WriteHeader(http.StatusOK)
}
