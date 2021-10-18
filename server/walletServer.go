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
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

var (
	//go:embed devWallet
	devWallet embed.FS
)

const (
	ApiPath = "/api/"
)

type WalletServer struct {
	httpServer *http.Server
	zipFS      *zip.Reader
}

func NewWalletServer(
	config WalletConfig,
	port int,
	headers []HTTPHeader,
) *WalletServer {

	mux := http.NewServeMux()
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	zipContent, _ := devWallet.ReadFile("devWallet/html.zip")
	zipFS, _ := zip.NewReader(bytes.NewReader(zipContent), int64(len(zipContent)))

	me := &WalletServer{
		httpServer: httpServer,
		zipFS:      zipFS,
	}

	mux.Handle("/", me)

	// API handler
	mux.Handle(ApiPath, NewWalletApiServer(config))

	return &WalletServer{
		httpServer: httpServer,
	}
}

func (m WalletServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	if strings.HasSuffix(upath, "/") {
		upath = "/index.html"
		r.URL.Path = upath
	}

	file, err := m.zipFS.Open(upath[1:])

	if err != nil {
		//try with .html suffix
		file, err = m.zipFS.Open(upath[1:] + ".html")
		if err != nil {
			w.WriteHeader(500)
			return
		}
	}

	//detect mime type
	if strings.Contains(upath, ".") {
		parts := strings.Split(upath, ".")
		extension := parts[len(parts)-1]
		mimeType := mime.TypeByExtension("." + extension)
		if mimeType != "" {
			w.Header().Add("Content-Type", mimeType)
		}
	}

	v, _ := file.Stat()
	target := v.Size()
	var buffer []byte = make([]byte, 32768)

	for target > 0 {
		count, _ := file.Read(buffer)
		w.Write(buffer[:count])
		target = target - int64(count)
	}

}

func (h *WalletServer) Start() error {
	err := h.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (h *WalletServer) Stop() {
	_ = h.httpServer.Shutdown(context.Background())
}
