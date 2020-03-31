package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-emulator/server"
)

type Config struct {
	Port      int           `default:"3569" flag:"port,p" info:"port to run RPC server"`
	HTTPPort  int           `default:"8080" flag:"http-port" info:"port to run HTTP server"`
	Verbose   bool          `default:"false" flag:"verbose,v" info:"enable verbose logging"`
	BlockTime time.Duration `flag:"block-time,b" info:"time between sealed blocks"`
	RootKey   string        `flag:"root-key,k" info:"root account key"`
	Init      bool          `default:"false" flag:"init" info:"whether to initialize a new account profile"`
	GRPCDebug bool          `default:"false" flag:"grpc-debug" info:"enable gRPC server reflection for debugging with grpc_cli"`
	Persist   bool          `default:"false" flag:"persist" info:"enable persistent storage"`
	DBPath    string        `default:"./flowdb" flag:"dbpath" info:"path to database directory"`
}

const (
	EnvPrefix          = "FLOW"
	DefaultRootKeySeed = "c9ec2e748979be18d84c98f9835df044965eaa371621a34271f807163aa76d696a8975afe049f61b"
)

var (
	log  *logrus.Logger
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the Flow emulator server",
	Run: func(cmd *cobra.Command, args []string) {
		var rootKey flow.AccountPrivateKey

		if len(conf.RootKey) > 0 {
			rootKey = keys.MustDecodePrivateKeyHex(conf.RootKey)
		} else {
			rootKeySeed, _ := hex.DecodeString(DefaultRootKeySeed)
			rootKey, _ = keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, rootKeySeed)
		}

		if conf.Verbose {
			log.SetLevel(logrus.DebugLevel)
		}

		serverConf := &server.Config{
			GRPCPort:  conf.Port,
			GRPCDebug: conf.GRPCDebug,
			HTTPPort:  conf.HTTPPort,
			// TODO: allow headers to be parsed from environment
			HTTPHeaders:    nil,
			BlockTime:      conf.BlockTime,
			RootAccountKey: &rootKey,
			Persist:        conf.Persist,
			DBPath:         conf.DBPath,
		}

		emu := server.NewEmulatorServer(log, serverConf)
		emu.Start()
	},
}

func init() {
	initLogger()
	initConfig()
}

func initLogger() {
	log = logrus.New()
	log.Formatter = new(logrus.TextFormatter)
	log.Out = os.Stdout
}

func initConfig() {
	err := sconfig.New(&conf).
		FromEnvironment(EnvPrefix).
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}

func exit(code int, msg string) {
	fmt.Println(msg)
	os.Exit(code)
}

func main() {
	if err := Cmd.Execute(); err != nil {
		exit(1, err.Error())
	}
}
