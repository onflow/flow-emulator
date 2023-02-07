package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type ConfigInfo struct {
	ServiceKey string `json:"service_key"`
	StartupDir string `json:"startup_dir"`
}

var _ http.Handler = &ConfigInfo{}

func NewConfigInfo(emulatorServer *EmulatorServer) *ConfigInfo {
	startupDir, _ := os.Getwd()

	return &ConfigInfo{
		ServiceKey: fmt.Sprintf("0x%s", emulatorServer.blockchain.ServiceKey().Address.Hex()),
		StartupDir: startupDir,
	}
}

func (c *ConfigInfo) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s, _ := json.MarshalIndent(c, "", "\t")
	_, _ = w.Write(s)
}
