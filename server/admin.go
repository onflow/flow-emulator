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
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/onflow/flow-emulator/server/backend"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
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
	logger     *logrus.Logger
	host       string
	port       int
	httpServer *http.Server
}

func NewAdminServer(
	logger *logrus.Logger,
	emulatorServer *EmulatorServer,
	backend *backend.Backend,
	storage *Storage,
	grpcServer *GRPCServer,
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
	mux.Handle(EmulatorApiPath, NewEmulatorAPIServer(emulatorServer, backend, storage))

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

func (h *HTTPServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", h.host, h.port))
	if err != nil {
		return err
	}

	h.logger.
		WithField("port", h.port).
		Infof("✅  Started admin server on port %d", h.port)

	err = h.httpServer.Serve(lis)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (h *HTTPServer) Stop() {
	_ = h.httpServer.Shutdown(context.Background())
}

func (h *HTTPServer) Server() *http.Server {
	return h.httpServer
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
