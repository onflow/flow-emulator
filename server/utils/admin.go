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
	"context"
	"errors"
	"fmt"
	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/server/access"
	"net"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

const (
	LivenessPath    = "/live"
	MetricsPath     = "/metrics"
	EmulatorApiPath = "/emulator/"
)

type HTTPHeader struct {
	Key   string
	Value string
}

type HTTPServer struct {
	logger     *zerolog.Logger
	host       string
	port       int
	httpServer *http.Server
	listener   net.Listener
}

func NewAdminServer(
	logger *zerolog.Logger,
	emulator emulator.Emulator,
	adapter *adapters.AccessAdapter,
	grpcServer *access.GRPCServer,
	liveness *LivenessTicker,
	host string,
	port int,
	headers []HTTPHeader,
) *HTTPServer {
	wrappedServer := grpcweb.WrapServer(
		grpcServer.Server(),
		// TODO: is this needed?
		grpcweb.WithOriginFunc(func(origin string) bool { return true }),
	)

	mux := http.NewServeMux()

	// register metrics handler
	mux.Handle(MetricsPath, promhttp.Handler())

	// register liveness handler
	mux.Handle(LivenessPath, liveness.Handler())

	// register gRPC HTTP proxy
	mux.Handle("/", wrappedHandler(wrappedServer, headers))

	// register API handler
	mux.Handle(EmulatorApiPath, NewEmulatorAPIServer(emulator, adapter))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", host, port),
		Handler: mux,
	}

	return &HTTPServer{
		logger:     logger,
		host:       host,
		port:       port,
		httpServer: httpServer,
	}
}

func (h *HTTPServer) Listen() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", h.host, h.port))
	if err != nil {
		return err
	}

	h.listener = lis
	return nil
}

func (h *HTTPServer) Start() error {
	if h.listener == nil {
		if err := h.Listen(); err != nil {
			return err
		}
	}

	h.logger.Info().
		Int("port", h.port).
		Msgf("✅  Started admin server on port %d", h.port)

	err := h.httpServer.Serve(h.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (h *HTTPServer) Stop() {
	_ = h.httpServer.Shutdown(context.Background())
}

func wrappedHandler(wrappedServer *grpcweb.WrappedGrpcServer, headers []HTTPHeader) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		setResponseHeaders(&res, headers)

		if (*req).Method == "OPTIONS" {
			return
		}

		wrappedServer.ServeHTTP(res, req)
	}
}

func setResponseHeaders(w *http.ResponseWriter, headers []HTTPHeader) {
	for _, header := range headers {
		(*w).Header().Set(header.Key, header.Value)
	}
}
