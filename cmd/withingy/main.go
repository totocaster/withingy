package main

import (
	"context"
	"fmt"
	"os"

	"github.com/toto/withingy/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(version, commit, date)
	ctx := context.Background()
	if err := cli.Execute(ctx); err != nil {
		if cliErr, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(cliErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
