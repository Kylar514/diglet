package tunnel

import (
	"encoding/json"
	"os"
	"sync"
	"syscall"

	"github.com/kylar514/diglet/config"
)

const stateFile = "/tmp/diglet-state.json"

type stateEntry struct {
	Pid        int               `json:"pid"`
	Connection config.Connection `json:"connection"`
}

type stateFile_ struct {
	Tunnels map[string]stateEntry `json:"tunnels"`
}

// saveState writes the current active tunnel map to the state file.
// Called after every Start/Stop.
func saveState() error {
	sf := stateFile_{
		Tunnels: make(map[string]stateEntry, len(active)),
	}
	for name, t := range active {
		sf.Tunnels[name] = stateEntry{
			Pid:        t.Pid,
			Connection: t.Connection,
		}
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0o600)
}

// Reconcile reads the state file and checks each stored tunnel:
//   - PID dead            → drop, clean up state file
//   - PID alive, port up  → restore into active map
//   - PID alive, port down → the process is alive but not forwarding;
//     treat as dead, kill it, drop from state
//
// All port probes run concurrently so startup isn't serialised across tunnels.
func Reconcile() {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}

	var sf stateFile_
	if err := json.Unmarshal(data, &sf); err != nil {
		_ = os.Remove(stateFile)
		return
	}

	type result struct {
		name  string
		entry stateEntry
		alive bool
	}

	results := make(chan result, len(sf.Tunnels))
	var wg sync.WaitGroup

	for name, entry := range sf.Tunnels {
		wg.Add(1)
		go func(name string, entry stateEntry) {
			defer wg.Done()
			if !pidAlive(entry.Pid) {
				results <- result{name: name, entry: entry, alive: false}
				return
			}
			// PID is alive — verify the port is actually reachable.
			if err := Probe(entry.Connection.LocalPort); err != nil {
				// Process exists but port isn't up — kill the zombie.
				_ = syscall.Kill(-entry.Pid, syscall.SIGKILL)
				_ = syscall.Kill(entry.Pid, syscall.SIGKILL)
				results <- result{name: name, entry: entry, alive: false}
				return
			}
			results <- result{name: name, entry: entry, alive: true}
		}(name, entry)
	}

	wg.Wait()
	close(results)

	changed := false
	for r := range results {
		if r.alive {
			active[r.name] = &ActiveTunnel{
				Connection: r.entry.Connection,
				Pid:        r.entry.Pid,
			}
		} else {
			changed = true
		}
	}

	if changed {
		_ = saveState()
	}
}

// pidAlive returns true if the process with the given PID is running.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
