package main

import (
	"os"

	"github.com/irgipaulius/cogs/internal/cli"
	"github.com/irgipaulius/cogs/internal/cli/ux"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		ux.WriteError(os.Stderr, err)
		os.Exit(1)
	}
}
