package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/derekurban2001/authmux/cmd/authmux"
	"github.com/derekurban2001/authmux/internal/app"
)

func main() {
	cmd := authmux.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		var codeErr app.ExitCodeError
		if errors.As(err, &codeErr) {
			os.Exit(codeErr.Code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
