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

package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Account struct {
	Type    string   `json:"type"`
	Address string   `json:"address"`
	Scopes  []string `json:"scopes"`
	KeyId   int      `json:"keyId"`
	Label   string   `json:"label"`
}

type WalletConfig struct {
	Address    string `json:"address"`
	KeyId      int    `json:"keyId"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	AccessNode string `json:"accessNode"`
	UseAPI     bool   `json:"useAPI"`
	Suffix     string `json:"suffix"`
}

type WalletApiServer struct {
	config *WalletConfig
}

func NewWalletApiServer(config WalletConfig) *WalletApiServer {
	return &WalletApiServer{
		config: &config,
	}
}

func (m WalletApiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	if upath == "/api/config" {
		m.Config(w, r)
	}

}

func (m WalletApiServer) Config(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(m.config)
	if err != nil {
		w.WriteHeader(500)
	}
}
