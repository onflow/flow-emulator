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

package access

import (
	"fmt"
	"github.com/onflow/flow-emulator/adapters"
	mockModule "github.com/onflow/flow-go/module/mock"
	"net"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/onflow/flow-go/access"
	legacyaccess "github.com/onflow/flow-go/access/legacy"
	"github.com/onflow/flow-go/model/flow"
	flowgo "github.com/onflow/flow-go/model/flow"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type mockHeaderCache struct {
}

func (mockHeaderCache) Get() *flowgo.Header {
	return &flowgo.Header{}
}

type GRPCServer struct {
	logger     *zerolog.Logger
	host       string
	port       int
	grpcServer *grpc.Server
	listener   net.Listener
}

func NewGRPCServer(logger *zerolog.Logger, adapter *adapters.AccessAdapter, chain flow.Chain, host string, port int, debug bool) *GRPCServer {
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpcprometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpcprometheus.UnaryServerInterceptor),
	)

	//TODO: bluesign: clean this up
	me := new(mockModule.Local)
	me.On("NodeID").Return(flowgo.ZeroID)

	legacyaccessproto.RegisterAccessAPIServer(grpcServer, legacyaccess.NewHandler(adapter, chain))
	accessproto.RegisterAccessAPIServer(grpcServer, access.NewHandler(adapter, chain, mockHeaderCache{}, me))

	grpcprometheus.Register(grpcServer)

	if debug {
		reflection.Register(grpcServer)
	}

	return &GRPCServer{
		logger:     logger,
		host:       host,
		port:       port,
		grpcServer: grpcServer,
	}
}

func (g *GRPCServer) Server() *grpc.Server {
	return g.grpcServer
}

func (g *GRPCServer) Listen() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", g.host, g.port))
	if err != nil {
		return err
	}
	g.listener = lis
	return nil
}

func (g *GRPCServer) Start() error {
	if g.listener == nil {
		if err := g.Listen(); err != nil {
			return err
		}
	}

	g.logger.Info().Int("port", g.port).Msgf("✅  Started gRPC server on port %d", g.port)

	err := g.grpcServer.Serve(g.listener)
	if err != nil {
		return err
	}

	return nil
}

func (g *GRPCServer) Stop() {
	g.grpcServer.GracefulStop()
}
