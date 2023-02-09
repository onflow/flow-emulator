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
)

type ConfigInfo struct {
	ServiceKey string `json:"service_key"`
}

var _ http.Handler = &ConfigInfo{}

func NewConfigInfo(emulatorServer *EmulatorServer) *ConfigInfo {
	return &ConfigInfo{
		ServiceKey: emulatorServer.blockchain.ServiceKey().PrivateKey.String(),
	}
}

func (c *ConfigInfo) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s, _ := json.MarshalIndent(c, "", "\t")
	_, _ = w.Write(s)
}
