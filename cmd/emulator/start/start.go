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

package start

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-emulator/server"
)

type Config struct {
	Port                   int           `default:"3569" flag:"port,p" info:"port to run RPC server"`
	RestPort               int           `default:"8888" flag:"rest-port" info:"port to run the REST API"`
	AdminPort              int           `default:"8080" flag:"admin-port" info:"port to run the admin API"`
	Verbose                bool          `default:"false" flag:"verbose,v" info:"enable verbose logging"`
	LogFormat              string        `default:"text" flag:"log-format" info:"logging output format. Valid values (text, JSON)"`
	BlockTime              time.Duration `flag:"block-time,b" info:"time between sealed blocks, e.g. '300ms', '-1.5h' or '2h45m'. Valid units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'"`
	ServicePrivateKey      string        `flag:"service-priv-key" info:"service account private key"`
	ServicePublicKey       string        `flag:"service-pub-key" info:"service account public key"`
	ServiceKeySigAlgo      string        `default:"ECDSA_P256" flag:"service-sig-algo" info:"service account key signature algorithm"`
	ServiceKeyHashAlgo     string        `default:"SHA3_256" flag:"service-hash-algo" info:"service account key hash algorithm"`
	Init                   bool          `default:"false" flag:"init" info:"whether to initialize a new account profile"`
	GRPCDebug              bool          `default:"false" flag:"grpc-debug" info:"enable gRPC server reflection for debugging with grpc_cli"`
	RESTDebug              bool          `default:"false" flag:"rest-debug" info:"enable REST API debugging output"`
	Persist                bool          `default:"false" flag:"persist" info:"enable persistent storage"`
	DBPath                 string        `default:"./flowdb" flag:"dbpath" info:"path to database directory"`
	SimpleAddresses        bool          `default:"false" flag:"simple-addresses" info:"use sequential addresses starting with 0x01"`
	TokenSupply            string        `default:"1000000000.0" flag:"token-supply" info:"initial FLOW token supply"`
	TransactionExpiry      int           `default:"10" flag:"transaction-expiry" info:"transaction expiry, measured in blocks"`
	StorageLimitEnabled    bool          `default:"true" flag:"storage-limit" info:"enable account storage limit"`
	StorageMBPerFLOW       string        `flag:"storage-per-flow" info:"the MB amount of storage capacity an account has per 1 FLOW token it has. e.g. '100.0'. The default is taken from the current version of flow-go"`
	MinimumAccountBalance  string        `flag:"min-account-balance" info:"The minimum account balance of an account. This is also the cost of creating one account. e.g. '0.001'. The default is taken from the current version of flow-go"`
	TransactionFeesEnabled bool          `default:"false" flag:"transaction-fees" info:"enable transaction fees"`
	TransactionMaxGasLimit int           `default:"9999" flag:"transaction-max-gas-limit" info:"maximum gas limit for transactions"`
	ScriptGasLimit         int           `default:"100000" flag:"script-gas-limit" info:"gas limit for scripts"`
	WithContracts          bool          `default:"false" flag:"contracts" info:"deploy common contracts when emulator starts"`
}

const EnvPrefix = "FLOW"

var (
	conf Config
)

type serviceKeyFunc func(
	init bool,
	sigAlgo crypto.SignatureAlgorithm,
	hashAlgo crypto.HashAlgorithm,
) (crypto.PrivateKey, crypto.SignatureAlgorithm, crypto.HashAlgorithm)

func Cmd(getServiceKey serviceKeyFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts the Flow emulator server",
		Run: func(cmd *cobra.Command, args []string) {
			var (
				servicePrivateKey  crypto.PrivateKey
				servicePublicKey   crypto.PublicKey
				serviceKeySigAlgo  crypto.SignatureAlgorithm
				serviceKeyHashAlgo crypto.HashAlgorithm
				err                error
			)

			serviceKeySigAlgo = crypto.StringToSignatureAlgorithm(conf.ServiceKeySigAlgo)
			serviceKeyHashAlgo = crypto.StringToHashAlgorithm(conf.ServiceKeyHashAlgo)

			logger := initLogger()

			if len(conf.ServicePublicKey) > 0 {
				checkKeyAlgorithms(serviceKeySigAlgo, serviceKeyHashAlgo)

				servicePublicKey, err = crypto.DecodePublicKeyHex(serviceKeySigAlgo, conf.ServicePublicKey)
				if err != nil {
					Exit(1, err.Error())
				}
			} else if len(conf.ServicePrivateKey) > 0 {
				checkKeyAlgorithms(serviceKeySigAlgo, serviceKeyHashAlgo)

				servicePrivateKey, err = crypto.DecodePrivateKeyHex(serviceKeySigAlgo, conf.ServicePrivateKey)
				if err != nil {
					Exit(1, err.Error())
				}

				servicePublicKey = servicePrivateKey.PublicKey()
			} else {
				servicePrivateKey, serviceKeySigAlgo, serviceKeyHashAlgo = getServiceKey(
					conf.Init,
					serviceKeySigAlgo,
					serviceKeyHashAlgo,
				)
				servicePublicKey = servicePrivateKey.PublicKey()
			}

			if conf.Verbose {
				logger.SetLevel(logrus.DebugLevel)
			}

			serviceAddress := sdk.ServiceAddress(sdk.Emulator)
			serviceFields := logrus.Fields{
				"serviceAddress":  serviceAddress.Hex(),
				"servicePubKey":   hex.EncodeToString(servicePublicKey.Encode()),
				"serviceSigAlgo":  serviceKeySigAlgo,
				"serviceHashAlgo": serviceKeyHashAlgo,
			}

			if servicePrivateKey != nil {
				serviceFields["servicePrivKey"] = hex.EncodeToString(servicePrivateKey.Encode())
			}

			logger.WithFields(serviceFields).Infof("⚙️   Using service account 0x%s", serviceAddress.Hex())

			minimumStorageReservation := fvm.DefaultMinimumStorageReservation
			if conf.MinimumAccountBalance != "" {
				minimumStorageReservation = parseCadenceUFix64(conf.MinimumAccountBalance, "min-account-balance")
			}

			storageMBPerFLOW := fvm.DefaultStorageMBPerFLOW
			if conf.StorageMBPerFLOW != "" {
				storageMBPerFLOW = parseCadenceUFix64(conf.StorageMBPerFLOW, "storage-per-flow")
			}

			serverConf := &server.Config{
				GRPCPort:  conf.Port,
				GRPCDebug: conf.GRPCDebug,
				AdminPort: conf.AdminPort,
				RESTPort:  conf.RestPort,
				RESTDebug: conf.RESTDebug,
				// TODO: allow headers to be parsed from environment
				HTTPHeaders:               nil,
				BlockTime:                 conf.BlockTime,
				ServicePublicKey:          servicePublicKey,
				ServicePrivateKey:         servicePrivateKey,
				ServiceKeySigAlgo:         serviceKeySigAlgo,
				ServiceKeyHashAlgo:        serviceKeyHashAlgo,
				Persist:                   conf.Persist,
				DBPath:                    conf.DBPath,
				GenesisTokenSupply:        parseCadenceUFix64(conf.TokenSupply, "token-supply"),
				TransactionMaxGasLimit:    uint64(conf.TransactionMaxGasLimit),
				ScriptGasLimit:            uint64(conf.ScriptGasLimit),
				TransactionExpiry:         uint(conf.TransactionExpiry),
				StorageLimitEnabled:       conf.StorageLimitEnabled,
				StorageMBPerFLOW:          storageMBPerFLOW,
				MinimumStorageReservation: minimumStorageReservation,
				TransactionFeesEnabled:    conf.TransactionFeesEnabled,
				WithContracts:             conf.WithContracts,
				SimpleAddresses:           conf.SimpleAddresses,
			}

			emu := server.NewEmulatorServer(logger, serverConf)
			if emu != nil {
				emu.Start()
			} else {
				Exit(-1, "")
			}
		},
	}

	initConfig(cmd)

	return cmd
}

func initLogger() *logrus.Logger {
	var logger = logrus.New()

	switch strings.ToLower(conf.LogFormat) {
	case "json":
		logger.Formatter = new(logrus.JSONFormatter)
	default:
		logger.Formatter = new(logrus.TextFormatter)
	}

	logger.Out = os.Stdout

	return logger
}

func initConfig(cmd *cobra.Command) {
	err := sconfig.New(&conf).
		FromEnvironment(EnvPrefix).
		BindFlags(cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}

func Exit(code int, msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}

func parseCadenceUFix64(value string, valueName string) cadence.UFix64 {
	tokenSupply, err := cadence.NewUFix64(value)
	if err != nil {
		Exit(
			1,
			fmt.Sprintf(
				"Failed to parse %s from value `%s` as an unsigned 64-bit fixed-point number: %s",
				valueName,
				conf.TokenSupply,
				err.Error()),
		)
	}

	return tokenSupply
}

func checkKeyAlgorithms(sigAlgo crypto.SignatureAlgorithm, hashAlgo crypto.HashAlgorithm) {
	if sigAlgo == crypto.UnknownSignatureAlgorithm {
		Exit(1, "Must specify service key signature algorithm (e.g. --service-sig-algo=ECDSA_P256)")
	}

	if hashAlgo == crypto.UnknownHashAlgorithm {
		Exit(1, "Must specify service key hash algorithm (e.g. --service-hash-algo=SHA3_256)")
	}
}
