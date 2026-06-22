package main

import (
	"os"

	"vigia/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
