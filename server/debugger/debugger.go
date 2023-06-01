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

package debugger

import (
	"bufio"
	"fmt"
	"github.com/google/go-dap"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/rs/zerolog"
	"io"
	"net"
	"sync"
)

type Debugger struct {
	logger      *zerolog.Logger
	emulator    emulator.Emulator
	port        int
	listener    net.Listener
	quit        chan interface{}
	wg          sync.WaitGroup
	stopOnce    sync.Once
	activeCode  string
	connections []net.Conn
}

func New(logger *zerolog.Logger, emulator emulator.Emulator, port int) *Debugger {
	return &Debugger{
		logger:   logger,
		emulator: emulator,
		port:     port,
		quit:     make(chan interface{}),
	}
}

func (d *Debugger) Start() error {

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", d.port))
	if err != nil {
		return err
	}
	d.listener = listener
	defer listener.Close()

	d.wg.Add(1)
	go d.serve()
	d.wg.Wait()
	return nil
}

func (d *Debugger) serve() {
	defer d.wg.Done()

	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.quit:
				return
			default:
				d.logger.Fatal().Err(err).Msg("failed to accept")
			}
		} else {
			d.wg.Add(1)
			go func() {
				d.handleConnection(conn)
				d.wg.Done()
			}()
		}
	}
}

func (d *Debugger) handleConnection(conn net.Conn) {
	d.connections = append(d.connections, conn)
	session := session{
		emulator: d.emulator,
		logger:   d.logger,
		readWriter: bufio.NewReadWriter(
			bufio.NewReader(conn),
			bufio.NewWriter(conn),
		),
		sendQueue: make(chan dap.Message),
	}
	go session.sendFromQueue()

	for {
		err := session.handleRequest()
		if err != nil {
			_, opError := err.(*net.OpError)
			if err == io.EOF {
				break
			}
			if opError {
				close(session.sendQueue)
				conn.Close()
				return
			}
			d.logger.Fatal().Err(err).Msg("Debug Server error")
		}
	}
	session.sendWg.Wait()
	close(session.sendQueue)
	conn.Close()
}

func (d *Debugger) Stop() {
	d.stopOnce.Do(func() {
		close(d.quit)
		if d.listener != nil {
			d.listener.Close()
		}
		for _, conn := range d.connections {
			conn.Close()
		}
	})
}
