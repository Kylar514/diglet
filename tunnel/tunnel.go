package tunnel

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/kylar514/diglet/config"
)

var builders = map[string]func(config.Connection) (*exec.Cmd, error){}

func Register(tunnelType string, builder func(config.Connection) (*exec.Cmd, error)) {
	builders[tunnelType] = builder
}

func Start(conn config.Connection) (int, error) {
	statuses, err := Statuses([]config.Connection{conn})
	if err != nil {
		return 0, err
	}
	switch statuses[conn.Name].Status {
	case StatusActive, StatusConnecting:
		return 0, fmt.Errorf("tunnel %q is already running", conn.Name)
	case StatusOccupied:
		if owner := statuses[conn.Name].Owner; owner != "" {
			return 0, fmt.Errorf("port %d is currently used by tunnel %q", conn.LocalPort, owner)
		}
		return 0, fmt.Errorf("port %d is occupied by a process not owned by diglet", conn.LocalPort)
	}
	builder, ok := builders[conn.TunnelType]
	if !ok {
		return 0, fmt.Errorf("unknown tunnel_type %q", conn.TunnelType)
	}
	cmd, err := builder(conn)
	if err != nil {
		return 0, fmt.Errorf("tunnel %q: %w", conn.Name, err)
	}
	prepareCommand(cmd)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start tunnel %q: %w", conn.Name, err)
	}
	pid := cmd.Process.Pid
	identity, err := processIdentity(pid)
	if err != nil {
		if stopErr := terminateProcess(pid); stopErr != nil {
			return 0, fmt.Errorf("could not identify tunnel %q process: %v; cleanup failed: %w", conn.Name, err, stopErr)
		}
		return 0, fmt.Errorf("could not identify tunnel %q process: %w", conn.Name, err)
	}
	if err := addEntry(conn, pid, identity); err != nil {
		if stopErr := terminateProcess(pid); stopErr != nil {
			return 0, fmt.Errorf("%v; cleanup failed: %w", err, stopErr)
		}
		return 0, err
	}
	go func() { _ = cmd.Wait() }()
	return pid, nil
}

func Stop(conn config.Connection) error {
	return stop(conn.Name, 0)
}

func StopAll() ([]string, []error) {
	var stopped []string
	var errs []error
	entries, err := allEntries()
	if err != nil {
		return nil, []error{err}
	}
	for _, entry := range entries {
		if err := stop(entry.Connection.Name, entry.PID); err != nil {
			errs = append(errs, err)
		} else {
			stopped = append(stopped, entry.Connection.Name)
		}
	}
	return stopped, errs
}

func stop(name string, expectedPID int) error {
	entry, exists, err := entryFor(name)
	if err != nil {
		return err
	}
	if !exists || expectedPID != 0 && entry.PID != expectedPID {
		return fmt.Errorf("no tunnel owned by diglet named %q", name)
	}
	identity, err := processIdentity(entry.PID)
	if err != nil || identity != entry.Identity {
		_ = removeEntry(name, entry.PID, entry.Identity)
		return fmt.Errorf("tunnel %q is no longer running", name)
	}
	if err := terminateProcess(entry.PID); err != nil {
		return fmt.Errorf("failed to stop tunnel %q: %w", name, err)
	}
	return removeEntry(name, entry.PID, entry.Identity)
}

func RegisteredTypes() []string {
	types := make([]string, 0, len(builders))
	for k := range builders {
		types = append(types, k)
	}
	sort.Strings(types)
	return types
}
