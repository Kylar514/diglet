package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/kylar514/diglet/config"
	"github.com/kylar514/diglet/tui"
	"github.com/kylar514/diglet/tunnel"
)

func main() {
	// Probe subcommand: `diglet probe <name> <localPort>`
	// Spawned as a detached subprocess by the TUI when a tunnel is started.
	// Probes the port, updates the state file, and sends desktop notifications.
	// Runs independently of the TUI and survives the TUI being closed.
	if len(os.Args) == 4 && os.Args[1] == "probe" {
		name := os.Args[2]
		localPort, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe: invalid port %q: %v\n", os.Args[3], err)
			os.Exit(1)
		}
		tunnel.RunProbe(name, localPort)
		os.Exit(0)
	}

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
