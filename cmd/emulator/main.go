package main

import (
	"github.com/dapperlabs/flow-emulator/cmd/emulator/start"
)

func main() {
	if err := start.Cmd.Execute(); err != nil {
		start.Exit(1, err.Error())
	}
}
