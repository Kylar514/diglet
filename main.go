package main

import (
	"fmt"
	"os"

	"github.com/kylar514/diglet/config"
	"github.com/kylar514/diglet/tui"
	"github.com/kylar514/diglet/tunnel"
)

func main() {
	cfgPath := config.DefaultConfigPath()
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if !config.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}

		// First run — scaffold a starter config and continue with empty connections.
		if scaffoldErr := config.Scaffold(cfgPath); scaffoldErr != nil {
			fmt.Fprintf(os.Stderr, "error creating config: %v\n", scaffoldErr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "created starter config at %s\npress 'e' inside diglet to edit it\n", cfgPath)

		cfg, err = config.Load(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading scaffolded config: %v\n", err)
			os.Exit(1)
		}
	}

	// Restore and health-check any tunnels from a previous session.
	tunnel.Reconcile()

	if err := tui.Run(cfg, cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error running ui: %v\n", err)
		os.Exit(1)
	}
}
