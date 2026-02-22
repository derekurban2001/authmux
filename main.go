package main

import (
	"os"

	"github.com/derekurban2001/proflex-cli/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
