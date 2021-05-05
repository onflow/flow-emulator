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
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/psiemens/graceland"
	"github.com/sirupsen/logrus"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-emulator/storage"
)

// EmulatorServer is a local server that runs a Flow Emulator instance.
//
// The server wraps an emulated blockchain instance with the Access API gRPC handlers.
type EmulatorServer struct {
	logger   *logrus.Logger
	config   *Config
	backend  *backend.Backend
	group    *graceland.Group
	liveness graceland.Routine
	storage  graceland.Routine
	grpc     graceland.Routine
	http     graceland.Routine
	blocks   graceland.Routine
}

const (
	defaultGRPCPort               = 3569
	defaultHTTPPort               = 8080
	defaultLivenessCheckTolerance = time.Second
	defaultDBGCInterval           = time.Minute * 5
	defaultDBGCRatio              = 0.5
)

var (
	defaultHTTPHeaders = []HTTPHeader{
		{
			Key:   "Access-Control-Allow-Origin",
			Value: "*",
		},
		{
			Key:   "Access-Control-Allow-Methods",
			Value: "POST, GET, OPTIONS, PUT, DELETE",
		},
		{
			Key:   "Access-Control-Allow-Headers",
			Value: "*",
		},
	}
)

// Config is the configuration for an emulator server.
type Config struct {
	GRPCPort               int
	GRPCDebug              bool
	HTTPPort               int
	HTTPHeaders            []HTTPHeader
	BlockTime              time.Duration
	ServicePublicKey       crypto.PublicKey
	ServiceKeySigAlgo      crypto.SignatureAlgorithm
	ServiceKeyHashAlgo     crypto.HashAlgorithm
	GenesisTokenSupply     cadence.UFix64
	TransactionExpiry      uint
	StorageLimitEnabled    bool
	TransactionFeesEnabled bool
	TransactionMaxGasLimit uint64
	ScriptGasLimit         uint64
	Persist                bool
	// DBPath is the path to the Badger database on disk.
	DBPath string
	// DBGCInterval is the time interval at which to garbage collect the Badger value log.
	DBGCInterval time.Duration
	// DBGCDiscardRatio is the ratio of space to reclaim during a Badger garbage collection run.
	DBGCDiscardRatio float64
	// LivenessCheckTolerance is the time interval in which the server must respond to liveness probes.
	LivenessCheckTolerance time.Duration
}

// NewEmulatorServer creates a new instance of a Flow Emulator server.
func NewEmulatorServer(logger *logrus.Logger, conf *Config) *EmulatorServer {
	conf = sanitizeConfig(conf)

	storage, err := configureStorage(logger, conf)
	if err != nil {
		logger.WithError(err).Error("❗  Failed to configure storage")
		return nil
	}

	blockchain, err := configureBlockchain(conf, storage.Store())
	if err != nil {
		logger.WithError(err).Error("❗  Failed to configure emulated blockchain")
		return nil
	}

	chain := blockchain.GetChain()

	contracts := logrus.Fields{
		"FlowServiceAccount": chain.ServiceAddress().HexWithPrefix(),
		"FlowToken":          fvm.FlowTokenAddress(chain).HexWithPrefix(),
		"FungibleToken":      fvm.FungibleTokenAddress(chain).HexWithPrefix(),
		"FlowFees":           fvm.FlowFeesAddress(chain).HexWithPrefix(),
		"FlowStorageFees":    chain.ServiceAddress().HexWithPrefix(),
	}
	logger.WithFields(contracts).Infof("📜  Flow contracts")

	backend := configureBackend(logger, conf, blockchain)

	livenessTicker := NewLivenessTicker(conf.LivenessCheckTolerance)
	grpcServer := NewGRPCServer(logger, backend, conf.GRPCPort, conf.GRPCDebug)
	httpServer := NewHTTPServer(grpcServer, livenessTicker, conf.HTTPPort, conf.HTTPHeaders)

	server := &EmulatorServer{
		logger:   logger,
		config:   conf,
		backend:  backend,
		storage:  storage,
		liveness: livenessTicker,
		grpc:     grpcServer,
		http:     httpServer,
	}

	// only create blocks ticker if block time > 0
	if conf.BlockTime > 0 {
		server.blocks = NewBlocksTicker(backend, conf.BlockTime)
	}

	return server
}

// Start starts the Flow Emulator server.
func (s *EmulatorServer) Start() {
	s.Stop()

	s.group = graceland.NewGroup()

	s.logger.
		WithField("port", s.config.GRPCPort).
		Infof("🌱  Starting gRPC server on port %d", s.config.GRPCPort)

	s.logger.
		WithField("port", s.config.HTTPPort).
		Infof("🌱  Starting HTTP server on port %d", s.config.HTTPPort)

	// only start blocks ticker if it exists
	if s.blocks != nil {
		s.group.Add(s.blocks)
	}

	s.group.Add(s.liveness)
	s.group.Add(s.grpc)
	s.group.Add(s.http)

	// routines are shut down in insertion order, so database is added last
	s.group.Add(s.storage)

	err := s.group.Start()
	if err != nil {
		s.logger.WithError(err).Error("❗  Server error")
	}

	s.Stop()
}

func (s *EmulatorServer) Stop() {
	if s.group == nil {
		return
	}

	s.group.Stop()

	s.logger.Info("🛑  Server stopped")
}

func configureStorage(logger *logrus.Logger, conf *Config) (storage Storage, err error) {
	if conf.Persist {
		return NewBadgerStorage(logger, conf.DBPath, conf.DBGCInterval, conf.DBGCDiscardRatio)
	}

	return NewMemoryStorage(), nil
}

func configureBlockchain(conf *Config, store storage.Store) (*emulator.Blockchain, error) {
	options := []emulator.Option{
		emulator.WithStore(store),
		emulator.WithGenesisTokenSupply(conf.GenesisTokenSupply),
		emulator.WithTransactionMaxGasLimit(conf.TransactionMaxGasLimit),
		emulator.WithScriptGasLimit(conf.ScriptGasLimit),
		emulator.WithTransactionExpiry(conf.TransactionExpiry),
		emulator.WithStorageLimitEnabled(conf.StorageLimitEnabled),
		emulator.WithTransactionFeesEnabled(conf.TransactionFeesEnabled),
	}

	if conf.ServicePublicKey != nil {
		options = append(
			options,
			emulator.WithServicePublicKey(conf.ServicePublicKey, conf.ServiceKeySigAlgo, conf.ServiceKeyHashAlgo),
		)
	}

	blockchain, err := emulator.NewBlockchain(options...)
	if err != nil {
		return nil, err
	}

	return blockchain, nil
}

func configureBackend(logger *logrus.Logger, conf *Config, blockchain *emulator.Blockchain) *backend.Backend {
	b := backend.New(logger, blockchain)

	if conf.BlockTime == 0 {
		b.EnableAutoMine()
	}

	return b
}

func sanitizeConfig(conf *Config) *Config {
	if conf.GRPCPort == 0 {
		conf.GRPCPort = defaultGRPCPort
	}

	if conf.HTTPPort == 0 {
		conf.HTTPPort = defaultHTTPPort
	}

	if conf.HTTPHeaders == nil {
		conf.HTTPHeaders = defaultHTTPHeaders
	}

	if conf.DBGCInterval == 0 {
		conf.DBGCInterval = defaultDBGCInterval
	}

	if conf.DBGCDiscardRatio == 0 {
		conf.DBGCDiscardRatio = defaultDBGCRatio
	}

	if conf.LivenessCheckTolerance == 0 {
		conf.LivenessCheckTolerance = defaultLivenessCheckTolerance
	}

	return conf
}
