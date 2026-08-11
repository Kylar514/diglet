package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kylar514/diglet/config"
)

type Status int

const (
	StatusInactive Status = iota
	StatusConnecting
	StatusActive
	StatusOccupied
)

type Info struct {
	Status Status
	PID    int
	Owner  string
}

type stateEntry struct {
	Connection config.Connection `json:"connection"`
	PID        int               `json:"pid"`
	Identity   uint64            `json:"identity"`
}

type stateFile struct {
	Tunnels map[string]stateEntry `json:"tunnels"`
}

func statePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("could not locate user cache directory: %w", err)
	}
	return filepath.Join(dir, "diglet", "state.json"), nil
}

func withState(fn func(*stateFile) (bool, error)) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock := path + ".lock"
	deadline := time.Now().Add(2 * time.Second)
	for {
		file, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			break
		}
		if !errors.Is(err, os.ErrExist) || time.Now().After(deadline) {
			return fmt.Errorf("could not lock tunnel registry: %w", err)
		}
		if info, statErr := os.Stat(lock); statErr == nil && time.Since(info.ModTime()) > 10*time.Second {
			_ = os.Remove(lock)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer os.Remove(lock)

	state := stateFile{Tunnels: map[string]stateEntry{}}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("could not read tunnel registry: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if state.Tunnels == nil {
		state.Tunnels = map[string]stateEntry{}
	}
	changed, err := fn(&state)
	if err != nil || !changed {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(tmpName, path)
}

func Statuses(connections []config.Connection) (map[string]Info, error) {
	ports, err := listeningPorts()
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]Info, len(connections))
	err = withState(func(state *stateFile) (bool, error) {
		changed := false
		for name, entry := range state.Tunnels {
			identity, err := processIdentity(entry.PID)
			if err != nil || identity != entry.Identity {
				delete(state.Tunnels, name)
				changed = true
			}
		}
		owners := make(map[int]stateEntry, len(state.Tunnels))
		for _, entry := range state.Tunnels {
			owners[entry.Connection.LocalPort] = entry
		}
		for _, connection := range connections {
			entry, owned := state.Tunnels[connection.Name]
			listeners, listening := ports[connection.LocalPort]
			switch {
			case owned && listening && listeners[entry.PID]:
				statuses[connection.Name] = Info{Status: StatusActive, PID: entry.PID}
			case owned && listening:
				statuses[connection.Name] = Info{Status: StatusOccupied, PID: entry.PID}
			case owned:
				statuses[connection.Name] = Info{Status: StatusConnecting, PID: entry.PID}
			case listening:
				info := Info{Status: StatusOccupied}
				if owner, exists := owners[connection.LocalPort]; exists && listeners[owner.PID] {
					info.Owner = owner.Connection.Name
				}
				statuses[connection.Name] = info
			default:
				statuses[connection.Name] = Info{Status: StatusInactive}
			}
		}
		return changed, nil
	})
	return statuses, err
}

func addEntry(connection config.Connection, pid int, identity uint64) error {
	return withState(func(state *stateFile) (bool, error) {
		if _, exists := state.Tunnels[connection.Name]; exists {
			return false, fmt.Errorf("tunnel %q is already running", connection.Name)
		}
		for _, entry := range state.Tunnels {
			if entry.Connection.LocalPort == connection.LocalPort {
				return false, fmt.Errorf("port %d is already owned by %q", connection.LocalPort, entry.Connection.Name)
			}
		}
		state.Tunnels[connection.Name] = stateEntry{Connection: connection, PID: pid, Identity: identity}
		return true, nil
	})
}

func entryFor(name string) (stateEntry, bool, error) {
	var entry stateEntry
	var found bool
	err := withState(func(state *stateFile) (bool, error) {
		entry, found = state.Tunnels[name]
		return false, nil
	})
	return entry, found, err
}

func removeEntry(name string, pid int, identity uint64) error {
	return withState(func(state *stateFile) (bool, error) {
		entry, exists := state.Tunnels[name]
		if !exists || entry.PID != pid || entry.Identity != identity {
			return false, nil
		}
		delete(state.Tunnels, name)
		return true, nil
	})
}

func allEntries() ([]stateEntry, error) {
	var entries []stateEntry
	err := withState(func(state *stateFile) (bool, error) {
		for _, entry := range state.Tunnels {
			entries = append(entries, entry)
		}
		return false, nil
	})
	return entries, err
}
