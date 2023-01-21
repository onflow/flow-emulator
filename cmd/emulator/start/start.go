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
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/psiemens/sconfig"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-emulator/server"
)

type Config struct {
	Port                      int           `default:"3569" flag:"port,p" info:"port to run RPC server"`
	RestPort                  int           `default:"8888" flag:"rest-port" info:"port to run the REST API"`
	AdminPort                 int           `default:"8080" flag:"admin-port" info:"port to run the admin API"`
	Verbose                   bool          `default:"false" flag:"verbose,v" info:"enable verbose logging"`
	LogFormat                 string        `default:"text" flag:"log-format" info:"logging output format. Valid values (text, JSON)"`
	BlockTime                 time.Duration `flag:"block-time,b" info:"time between sealed blocks, e.g. '300ms', '-1.5h' or '2h45m'. Valid units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'"`
	ServicePrivateKey         string        `flag:"service-priv-key" info:"service account private key"`
	ServicePublicKey          string        `flag:"service-pub-key" info:"service account public key"`
	ServiceKeySigAlgo         string        `default:"ECDSA_P256" flag:"service-sig-algo" info:"service account key signature algorithm"`
	ServiceKeyHashAlgo        string        `default:"SHA3_256" flag:"service-hash-algo" info:"service account key hash algorithm"`
	Init                      bool          `default:"false" flag:"init" info:"whether to initialize a new account profile"`
	GRPCDebug                 bool          `default:"false" flag:"grpc-debug" info:"enable gRPC server reflection for debugging with grpc_cli"`
	RESTDebug                 bool          `default:"false" flag:"rest-debug" info:"enable REST API debugging output"`
	Persist                   bool          `default:"false" flag:"persist" info:"enable persistent storage"`
	Snapshot                  bool          `default:"false" flag:"snapshot" info:"enable snapshots for emulator (this setting also automatically turns on persistent storage)"`
	DBPath                    string        `default:"./flowdb" flag:"dbpath" info:"path to database directory"`
	SimpleAddresses           bool          `default:"false" flag:"simple-addresses" info:"use sequential addresses starting with 0x01"`
	TokenSupply               string        `default:"1000000000.0" flag:"token-supply" info:"initial FLOW token supply"`
	TransactionExpiry         int           `default:"10" flag:"transaction-expiry" info:"transaction expiry, measured in blocks"`
	StorageLimitEnabled       bool          `default:"true" flag:"storage-limit" info:"enable account storage limit"`
	StorageMBPerFLOW          string        `flag:"storage-per-flow" info:"the MB amount of storage capacity an account has per 1 FLOW token it has. e.g. '100.0'. The default is taken from the current version of flow-go"`
	MinimumAccountBalance     string        `flag:"min-account-balance" info:"The minimum account balance of an account. This is also the cost of creating one account. e.g. '0.001'. The default is taken from the current version of flow-go"`
	TransactionFeesEnabled    bool          `default:"false" flag:"transaction-fees" info:"enable transaction fees"`
	TransactionMaxGasLimit    int           `default:"9999" flag:"transaction-max-gas-limit" info:"maximum gas limit for transactions"`
	ScriptGasLimit            int           `default:"100000" flag:"script-gas-limit" info:"gas limit for scripts"`
	WithContracts             bool          `default:"false" flag:"contracts" info:"deploy common contracts when emulator starts"`
	ContractRemovalEnabled    bool          `default:"true" flag:"contract-removal" info:"allow removal of already deployed contracts, used for updating during development"`
	SkipTransactionValidation bool          `default:"false" flag:"skip-tx-validation" info:"skip verification of transaction signatures and sequence numbers"`
	Host                      string        `default:"" flag:"host" info:"host to listen on for emulator GRPC/REST/Admin servers (default: all interfaces)"`
	ChainID                   string        `default:"emulator" flag:"chain-id" info:"chain to emulate for address generation. Valid values are: 'emulator', 'testnet', 'mainnet'"`
	RedisURL                  string        `default:"" flag:"redis-url" info:"redis-server URL for persisting redis storage backend ( redis://[[username:]password@]host[:port][/database] ) "`
	SqliteURL                 string        `default:"" flag:"sqlite-url" info:"sqlite db URL for persisting sqlite storage backend "`
}

const EnvPrefix = "FLOW"

var conf Config

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

			logger := initLogger(conf.Verbose)

			if conf.ServicePublicKey != "" {
				logger.Warn().Msg("❗  Providing '--public-key' is deprecated, provide the '--private-key' only.")
			}

			if conf.ServicePrivateKey != "" {
				checkKeyAlgorithms(serviceKeySigAlgo, serviceKeyHashAlgo)

				servicePrivateKey, err = crypto.DecodePrivateKeyHex(serviceKeySigAlgo, conf.ServicePrivateKey)
				if err != nil {
					Exit(1, err.Error())
				}

				servicePublicKey = servicePrivateKey.PublicKey()
			} else { // if we don't provide any config values use the serviceKeyFunc to obtain the key
				servicePrivateKey, serviceKeySigAlgo, serviceKeyHashAlgo = getServiceKey(
					conf.Init,
					serviceKeySigAlgo,
					serviceKeyHashAlgo,
				)
				servicePublicKey = servicePrivateKey.PublicKey()
			}

			flowChainID, err := getSDKChainID(conf.ChainID)
			if err != nil {
				Exit(1, err.Error())
			}

			serviceAddress := sdk.ServiceAddress(sdk.ChainID(flowChainID))
			if conf.SimpleAddresses {
				serviceAddress = sdk.HexToAddress("0x1")
			}

			serviceFields := map[string]any{
				"serviceAddress":  serviceAddress.Hex(),
				"servicePubKey":   hex.EncodeToString(servicePublicKey.Encode()),
				"serviceSigAlgo":  serviceKeySigAlgo.String(),
				"serviceHashAlgo": serviceKeyHashAlgo.String(),
			}

			if servicePrivateKey != nil {
				serviceFields["servicePrivKey"] = hex.EncodeToString(servicePrivateKey.Encode())
			}

			logger.Info().Fields(serviceFields).Msgf("⚙️   Using service account 0x%s", serviceAddress.Hex())

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
				Snapshot:                  conf.Snapshot,
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
				SkipTransactionValidation: conf.SkipTransactionValidation,
				SimpleAddressesEnabled:    conf.SimpleAddresses,
				Host:                      conf.Host,
				ChainID:                   flowChainID,
				RedisURL:                  conf.RedisURL,
				ContractRemovalEnabled:    conf.ContractRemovalEnabled,
				SqliteURL:                 conf.SqliteURL,
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

func initLogger(verbose bool) *zerolog.Logger {

	level := zerolog.InfoLevel
	if verbose {
		level = zerolog.DebugLevel
	}
	zerolog.MessageFieldName = "msg"

	switch strings.ToLower(conf.LogFormat) {
	case "json":
		logger := zerolog.New(os.Stdout).With().Timestamp().Logger().Level(level)
		return &logger
	default:
		writer := zerolog.ConsoleWriter{Out: os.Stdout}
		writer.FormatMessage = func(i interface{}) string {
			if i == nil {
				return ""
			}
			return fmt.Sprintf("%-44s", i)
		}
		logger := zerolog.New(writer).With().Timestamp().Logger().Level(level)
		return &logger
	}

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

func getSDKChainID(chainID string) (flowgo.ChainID, error) {
	switch chainID {
	case "emulator":
		return flowgo.Emulator, nil
	case "testnet":
		return flowgo.Testnet, nil
	case "mainnet":
		return flowgo.Testnet, nil
	default:
		return "", fmt.Errorf("Invalid ChainID %s, valid values are: emulator, testnet, mainnet", chainID)
	}
}

func checkKeyAlgorithms(sigAlgo crypto.SignatureAlgorithm, hashAlgo crypto.HashAlgorithm) {
	if sigAlgo == crypto.UnknownSignatureAlgorithm {
		Exit(1, "Must specify service key signature algorithm (e.g. --service-sig-algo=ECDSA_P256)")
	}

	if hashAlgo == crypto.UnknownHashAlgorithm {
		Exit(1, "Must specify service key hash algorithm (e.g. --service-hash-algo=SHA3_256)")
	}
}
