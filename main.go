package main

import (
	"os"

	"github.com/eos175/vigia/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
