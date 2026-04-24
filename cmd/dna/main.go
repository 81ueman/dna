package main

import (
	"os"

	"github.com/81ueman/dna/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
