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

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-emulator/storage/badger"
)

type EmulatorApiServer struct {
	router  *mux.Router
	server  *EmulatorServer
	backend *backend.Backend
	storage *Storage
}

func NewEmulatorApiServer(server *EmulatorServer, backend *backend.Backend, storage *Storage) *EmulatorApiServer {
	router := mux.NewRouter().StrictSlash(true)
	r := &EmulatorApiServer{router: router,
		server:  server,
		backend: backend,
		storage: storage,
	}

	//router.HandleFunc("/api/timeMachine", r.Config)
	//router.HandleFunc("/api/storage", r.Config)
	//router.HandleFunc("/api/storage/{account}", r.Config)
	//router.HandleFunc("/api/commitBlock", r.CommitBlock)
	router.HandleFunc("/api/snapshot/{name}", r.Snapshot)

	return r
}

func (m EmulatorApiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.ServeHTTP(w, r)
}

func (m EmulatorApiServer) CommitBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m.backend.CommitBlock()
	w.WriteHeader(http.StatusOK)
}

func (m EmulatorApiServer) Snapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	name := vars["name"]

	switch (*m.storage).Store().(type) {
	case *badger.Store:
		badgerStore := (*m.storage).Store().(*badger.Store)
		badgerStore.JumpToContext(name)

		blockchain, err := configureBlockchain(m.server.config, badgerStore)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		m.backend.SetEmulator(blockchain)

		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusInternalServerError)
	}

}

func (m EmulatorApiServer) Config(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
