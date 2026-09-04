package main

import (
	"os"

	"github.com/roshbhatia/seshy/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
