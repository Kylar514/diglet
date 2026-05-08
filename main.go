package main

import (
	"fmt"
	"os"

	"github.com/kylar514/diglet/config"
	"github.com/kylar514/diglet/tui"

	_ "github.com/kylar514/diglet/tunnel"
)

func main() {
	cfgPath := config.DefaultConfigPath()
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := tui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error running ui: %v\n", err)
		os.Exit(1)
	}
}

