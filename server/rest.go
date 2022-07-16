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
	"context"
	"fmt"
	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/rs/zerolog"
	"net"
	"net/http"
	"os"
)

type RestServer struct {
	server   *http.Server
	listener net.Listener
}

func (r *RestServer) Start() error {
	err := r.server.Serve(r.listener)
	if err != nil {
		return err
	}
	return nil
}

func (r *RestServer) Stop() {
	_ = r.server.Shutdown(context.Background())
}

func NewRestServer(be *backend.Backend, blockchain *emulator.Blockchain, port int, debug bool) (*RestServer, error) {
	logger := zerolog.Logger{}
	if debug {
		logger = zerolog.New(os.Stdout)
	}

	srv, err := rest.NewServer(backend.NewAdapter(be), "127.0.0.1:3333", logger, blockchain.GetChain())
	if err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	return &RestServer{
		server:   srv,
		listener: l,
	}, nil
}
