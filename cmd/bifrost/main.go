package main

import (
	"embed"
	"os"

	"github.com/kogungor/bifrost/internal/cli"
)

//go:embed all:commands
var commands embed.FS

func main() {
	cli.CommandsFS = commands
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
