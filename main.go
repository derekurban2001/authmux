package main

import (
	"os"

	"github.com/derekurban/profilex-cli/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
