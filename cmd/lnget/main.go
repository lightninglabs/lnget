// Package main is the entry point for the lnget CLI.
package main

import (
	"fmt"
	"os"

	"github.com/lightninglabs/lnget/cli"
)

func main() {
	rootCmd := cli.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
