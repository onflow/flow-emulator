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
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/psiemens/graceland"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/server/access"
	"github.com/onflow/flow-emulator/server/debugger"
	"github.com/onflow/flow-emulator/server/utils"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/sqlite"
	"github.com/onflow/flow-emulator/storage/util"
)

// EmulatorServer is a local server that runs a Flow Emulator instance.
//
// The server wraps an emulated emulator instance with the Access API gRPC handlers.
type EmulatorServer struct {
	logger        *zerolog.Logger
	config        *Config
	emulator      emulator.Emulator
	accessAdapter *adapters.AccessAdapter
	group         *graceland.Group
	liveness      graceland.Routine
	storage       graceland.Routine
	grpc          *access.GRPCServer
	rest          *access.RestServer
	admin         *utils.HTTPServer
	blocks        graceland.Routine
	debugger      graceland.Routine
}

const (
	defaultGRPCPort               = 3569
	defaultRESTPort               = 8888
	defaultAdminPort              = 8080
	defaultLivenessCheckTolerance = time.Second
	defaultDBGCInterval           = time.Minute * 5
	defaultDBGCRatio              = 0.5
)

var (
	defaultHTTPHeaders = []utils.HTTPHeader{
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
	GRPCPort                  int
	GRPCDebug                 bool
	AdminPort                 int
	DebuggerPort              int
	RESTPort                  int
	RESTDebug                 bool
	HTTPHeaders               []utils.HTTPHeader
	BlockTime                 time.Duration
	ServicePublicKey          crypto.PublicKey
	ServicePrivateKey         crypto.PrivateKey
	ServiceKeySigAlgo         crypto.SignatureAlgorithm
	ServiceKeyHashAlgo        crypto.HashAlgorithm
	GenesisTokenSupply        cadence.UFix64
	TransactionExpiry         uint
	StorageLimitEnabled       bool
	MinimumStorageReservation cadence.UFix64
	StorageMBPerFLOW          cadence.UFix64
	TransactionFeesEnabled    bool
	EVMEnabled                bool
	TransactionMaxGasLimit    uint64
	ScriptGasLimit            uint64
	Persist                   bool
	Snapshot                  bool
	// ContractRemovalEnabled configures possible removal of contracts.
	ContractRemovalEnabled bool
	// DBPath is the path to the Badger database on disk.
	DBPath string
	// DBGCInterval is the time interval at which to garbage collect the Badger value log.
	DBGCInterval time.Duration
	// DBGCDiscardRatio is the ratio of space to reclaim during a Badger garbage collection run.
	DBGCDiscardRatio float64
	// LivenessCheckTolerance is the time interval in which the server must respond to liveness probes.
	LivenessCheckTolerance time.Duration
	// Whether to deploy some extra Flow contracts when emulator starts
	WithContracts bool
	// Enable simple monotonically increasing address format (e.g. 0x1, 0x2, etc)
	SimpleAddressesEnabled    bool
	SkipTransactionValidation bool
	// Host listen on for the emulator servers (REST/GRPC/Admin)
	Host string
	//Chain to emulation
	ChainID flowgo.ChainID
	//Redis URL for redis storage backend
	RedisURL string
	//Sqlite URL for sqlite storage backend
	SqliteURL string
	// CoverageReportingEnabled enables/disables Cadence code coverage reporting.
	CoverageReportingEnabled bool
	// StartBlockHeight is the height at which to start the emulator.
	StartBlockHeight uint64
}

type listener interface {
	Listen() error
}

// NewEmulatorServer creates a new instance of a Flow Emulator server.
func NewEmulatorServer(logger *zerolog.Logger, conf *Config) *EmulatorServer {
	conf = sanitizeConfig(conf)

	store, err := configureStorage(conf)
	if err != nil {
		logger.Error().Err(err).Msg("❗  Failed to configure storage")
		return nil
	}

	emulatedBlockchain, err := configureBlockchain(logger, conf, store)
	if err != nil {
		logger.Err(err).Msg("❗  Failed to configure emulated emulator")
		return nil
	}

	chain := emulatedBlockchain.GetChain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	contracts := sc.All()
	// sort contracts to always have the same order
	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].Name < contracts[j].Name
	})
	for _, contract := range contracts {
		logger.
			Info().
			Fields(map[string]any{contract.Name: contract.Address.HexWithPrefix()}).
			Msg("📜 Flow contract")
	}

	if conf.WithContracts {
		commonContracts := emulator.NewCommonContracts(chain)
		err := emulator.DeployContracts(emulatedBlockchain, commonContracts)
		if err != nil {
			logger.Error().Err(err).Msg("❗  Failed to deploy contracts")
		}

		for _, contract := range commonContracts {
			logger.Info().Fields(
				map[string]any{
					contract.Name: fmt.Sprintf("0x%s", contract.Address.Hex()),
				},
			).Msg(contract.Description)
		}
	}

	accessAdapter := adapters.NewAccessAdapter(logger, emulatedBlockchain)
	livenessTicker := utils.NewLivenessTicker(conf.LivenessCheckTolerance)
	grpcServer := access.NewGRPCServer(logger, accessAdapter, chain, conf.Host, conf.GRPCPort, conf.GRPCDebug)
	restServer, err := access.NewRestServer(logger, emulatedBlockchain, accessAdapter, chain, conf.Host, conf.RESTPort, conf.RESTDebug)
	if err != nil {
		logger.Error().Err(err).Msg("❗  Failed to startup REST API")
		return nil
	}

	server := &EmulatorServer{
		logger:        logger,
		config:        conf,
		storage:       store,
		liveness:      livenessTicker,
		grpc:          grpcServer,
		rest:          restServer,
		admin:         nil,
		emulator:      emulatedBlockchain,
		accessAdapter: accessAdapter,
		debugger:      debugger.New(logger, emulatedBlockchain, conf.DebuggerPort),
	}

	server.admin = utils.NewAdminServer(logger, emulatedBlockchain, accessAdapter, grpcServer, livenessTicker, conf.Host, conf.AdminPort, conf.HTTPHeaders)

	// only create blocks ticker if block time > 0
	if conf.BlockTime > 0 {
		server.blocks = emulator.NewBlocksTicker(emulatedBlockchain, conf.BlockTime)
		emulatedBlockchain.DisableAutoMine()
	} else {
		emulatedBlockchain.EnableAutoMine()
	}

	return server
}

// Listen starts listening for incoming connections.
//
// After this non-blocking function executes we can treat the
// emulator server as ready.
func (s *EmulatorServer) Listen() error {
	for _, lis := range []listener{s.grpc, s.rest, s.admin} {
		err := lis.Listen()
		if err != nil { // fail quick
			return err
		}
	}

	return nil
}

// Start starts the Flow Emulator server.
//
// This is a blocking call that listens and starts the emulator server.
func (s *EmulatorServer) Start() {
	s.Stop()

	s.group = graceland.NewGroup()
	// only start blocks ticker if it exists
	if s.blocks != nil {
		s.group.Add(s.blocks)
	}
	s.group.Add(s.liveness)

	s.logger.Info().
		Int("port", s.config.GRPCPort).
		Msgf("🌱 Starting gRPC server on port %d", s.config.GRPCPort)
	s.group.Add(s.grpc)

	s.logger.Info().
		Int("port", s.config.RESTPort).
		Msgf("🌱 Starting REST API on port %d", s.config.RESTPort)
	s.group.Add(s.rest)

	s.logger.Info().
		Int("port", s.config.AdminPort).
		Msgf("🌱 Starting admin server on port %d", s.config.AdminPort)
	s.group.Add(s.admin)

	s.logger.Info().
		Int("port", s.config.DebuggerPort).
		Msgf("🌱 Starting debugger on port %d", s.config.DebuggerPort)
	s.group.Add(s.debugger)

	// only start blocks ticker if it exists
	if s.blocks != nil {
		s.group.Add(s.blocks)
	}

	// routines are shut down in insertion order, so database is added last
	s.group.Add(s.storage)

	err := s.group.Start()
	if err != nil {
		s.logger.Error().Err(err).Msg("❗  Server error")
	}

	s.Stop()
}

func (s *EmulatorServer) Emulator() emulator.Emulator {
	return s.emulator
}

func (s *EmulatorServer) AccessAdapter() *adapters.AccessAdapter {
	return s.accessAdapter
}

func (s *EmulatorServer) Stop() {
	if s.group == nil {
		return
	}

	s.group.Stop()

	s.logger.Info().Msg("🛑  Server stopped")
}

func configureStorage(conf *Config) (storageProvider storage.Store, err error) {
	if conf.RedisURL != "" {
		storageProvider, err = util.NewRedisStorage(conf.RedisURL)
		if err != nil {
			return nil, err
		}
	}

	if conf.SqliteURL != "" {
		if storageProvider != nil {
			return nil, fmt.Errorf("you cannot define more than one storage")
		}
		storageProvider, err = util.NewSqliteStorage(conf.SqliteURL)
		if err != nil {
			return nil, err
		}
	}

	if conf.Persist {
		if storageProvider != nil {
			return nil, fmt.Errorf("you cannot use persist with current configuration")
		}
		_ = os.Mkdir(conf.DBPath, os.ModePerm)
		storageProvider, err = util.NewSqliteStorage(conf.DBPath)
		if err != nil {
			return nil, err
		}
	}

	if storageProvider == nil {
		storageProvider, err = util.NewSqliteStorage(sqlite.InMemory)
		if err != nil {
			return nil, err
		}
	}

	if conf.Snapshot {
		snapshotProvider, isSnapshotProvider := storageProvider.(storage.SnapshotProvider)
		if !isSnapshotProvider {
			return nil, fmt.Errorf("selected storage provider does not support snapshots")
		}
		if !snapshotProvider.SupportSnapshotsWithCurrentConfig() {
			return nil, fmt.Errorf("selected storage provider does not support snapshots with current configuration")
		}
	}

	return storageProvider, err
}

func configureBlockchain(logger *zerolog.Logger, conf *Config, store storage.Store) (*emulator.Blockchain, error) {
	options := []emulator.Option{
		emulator.WithServerLogger(*logger),
		emulator.WithStore(store),
		emulator.WithGenesisTokenSupply(conf.GenesisTokenSupply),
		emulator.WithTransactionMaxGasLimit(conf.TransactionMaxGasLimit),
		emulator.WithScriptGasLimit(conf.ScriptGasLimit),
		emulator.WithTransactionExpiry(conf.TransactionExpiry),
		emulator.WithStorageLimitEnabled(conf.StorageLimitEnabled),
		emulator.WithMinimumStorageReservation(conf.MinimumStorageReservation),
		emulator.WithStorageMBPerFLOW(conf.StorageMBPerFLOW),
		emulator.WithTransactionFeesEnabled(conf.TransactionFeesEnabled),
		emulator.WithEVMEnabled(conf.EVMEnabled),
		emulator.WithChainID(conf.ChainID),
		emulator.WithContractRemovalEnabled(conf.ContractRemovalEnabled),
	}

	if conf.SkipTransactionValidation {
		options = append(
			options,
			emulator.WithTransactionValidationEnabled(false),
		)
	}

	if conf.SimpleAddressesEnabled {
		options = append(
			options,
			emulator.WithSimpleAddresses(),
		)
	}

	if conf.ServicePrivateKey != nil {
		options = append(
			options,
			emulator.WithServicePrivateKey(conf.ServicePrivateKey, conf.ServiceKeySigAlgo, conf.ServiceKeyHashAlgo),
		)
	} else if conf.ServicePublicKey != nil {
		options = append(
			options,
			emulator.WithServicePublicKey(conf.ServicePublicKey, conf.ServiceKeySigAlgo, conf.ServiceKeyHashAlgo),
		)
	}

	if conf.CoverageReportingEnabled {
		options = append(
			options,
			emulator.WithCoverageReport(runtime.NewCoverageReport()),
		)
	}

	emulatedBlockchain, err := emulator.New(options...)
	if err != nil {
		return nil, err
	}

	return emulatedBlockchain, nil
}

func sanitizeConfig(conf *Config) *Config {
	if conf.GRPCPort == 0 {
		conf.GRPCPort = defaultGRPCPort
	}

	if conf.RESTPort == 0 {
		conf.RESTPort = defaultRESTPort
	}

	if conf.AdminPort == 0 {
		conf.AdminPort = defaultAdminPort
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

	if conf.ChainID == "" {
		conf.ChainID = flowgo.Emulator
	}

	return conf
}
