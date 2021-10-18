package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Account struct {
	Type    string   `json:"type"`
	Address string   `json:"address"`
	Scopes  []string `json:"scopes"`
	KeyId   int      `json:"keyId"`
	Label   string   `json:"label"`
}

type WalletConfig struct {
	Address    string `json:"address"`
	KeyId      int    `json:"keyId"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	AccessNode string `json:"accessNode"`
	UseAPI     bool   `json:"useAPI"`
	Suffix     string `json:"suffix"`
}

type WalletApiServer struct {
	config *WalletConfig
}

func NewWalletApiServer(config WalletConfig) *WalletApiServer {
	return &WalletApiServer{
		config: &config,
	}
}

func (m WalletApiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	if upath == "/api/config" {
		m.Config(w, r)
	}

}

func (m WalletApiServer) Config(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(m.config)
}
