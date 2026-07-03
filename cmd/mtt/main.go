// Command mtt is a minimalist, file-backed task tracker for agents and humans.
package main

import (
	"os"

	"github.com/pashukhin/mtt/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
