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
	// Status subcommand: `diglet status [config-path]`
	// Prints the name of each active tunnel, one per line.
	if len(os.Args) >= 2 && os.Args[1] == "status" {
		cfgPath := config.DefaultConfigPath()
		if len(os.Args) > 2 {
			cfgPath = os.Args[2]
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(2)
		}
		statuses, err := tunnel.Statuses(cfg.Connections)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(2)
		}
		any := false
		for _, c := range cfg.Connections {
			if statuses[c.Name].Status == tunnel.StatusActive {
				fmt.Println(c.Name)
				any = true
			}
		}
		if !any {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Probe subcommand: `diglet probe <name> <localPort> <pid>`
	// Spawned as a detached subprocess by the TUI when a tunnel is started.
	if len(os.Args) == 5 && os.Args[1] == "probe" {
		name := os.Args[2]
		localPort, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe: invalid port %q: %v\n", os.Args[3], err)
			os.Exit(1)
		}
		tunnelPID, err := strconv.Atoi(os.Args[4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe: invalid pid %q: %v\n", os.Args[4], err)
			os.Exit(1)
		}
		tunnel.RunProbe(name, localPort, tunnelPID)
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

	if err := tui.Run(cfg, cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error running ui: %v\n", err)
		os.Exit(1)
	}
}
