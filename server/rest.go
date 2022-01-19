package server

import (
	"context"
	"fmt"
	"github.com/onflow/flow-emulator/server/backend"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/rs/zerolog"
	"net"
	"net/http"
	"os"
)

type RestServer struct {
	server   *http.Server
	listener net.Listener
}

func (r *RestServer) Start() error {
	err := r.server.Serve(r.listener)
	if err != nil {
		return err
	}
	return nil
}

func (r *RestServer) Stop() {
	_ = r.server.Shutdown(context.Background())
}

func NewRestServer(be *backend.Backend, port int, debug bool) (*RestServer, error) {
	logger := zerolog.Logger{}
	if debug {
		logger = zerolog.New(os.Stdout)
	}

	srv, err := rest.NewServer(backend.NewAdapter(be), "127.0.0.1:3333", logger)
	if err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	return &RestServer{
		server:   srv,
		listener: l,
	}, nil
}
