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
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
)

type RestServer struct {
	logger   *logrus.Logger
	host     string
	port     int
	server   *http.Server
	listener net.Listener
}

func (r *RestServer) Start() error {
	r.logger.
		WithField("port", r.port).
		Infof("✅  Started REST API server on port %d", r.port)

	err := r.server.Serve(r.listener)
	if err != nil {
		return err
	}
	return nil
}

func (r *RestServer) Stop() {
	_ = r.server.Shutdown(context.Background())
}

func NewRestServer(logger *logrus.Logger, be *backend.Backend, chain flow.Chain, host string, port int, debug bool) (*RestServer, error) {

	debug_logger := zerolog.Logger{}
	if debug {
		debug_logger = zerolog.New(os.Stdout)
	}

	srv, err := rest.NewServer(backend.NewAdapter(be), "127.0.0.1:3333", debug_logger, chain)
	if err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	return &RestServer{
		logger:   logger,
		host:     host,
		port:     port,
		server:   srv,
		listener: l,
	}, nil
}
