package tunnel

import (
	"encoding/json"
	"os"
	"syscall"

	"github.com/kylar514/diglet/config"
)

const stateFile = "/tmp/diglet-state.json"

const (
	stateConnecting = "connecting"
	stateActive     = "active"
)

type stateEntry struct {
	Pid        int               `json:"pid"`
	Status     string            `json:"status"`
	Connection config.Connection `json:"connection"`
}

type stateFile_ struct {
	Tunnels map[string]stateEntry `json:"tunnels"`
}

// readStateFile reads and parses the state file. Returns an empty map on any error.
func readStateFile() stateFile_ {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return stateFile_{Tunnels: map[string]stateEntry{}}
	}
	var sf stateFile_
	if err := json.Unmarshal(data, &sf); err != nil {
		return stateFile_{Tunnels: map[string]stateEntry{}}
	}
	if sf.Tunnels == nil {
		sf.Tunnels = map[string]stateEntry{}
	}
	return sf
}

// writeStateFile serializes and writes the state file atomically.
func writeStateFile(sf stateFile_) error {
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0o600)
}

// saveState writes the full in-memory active map to the state file,
// including connecting entries so they survive TUI restarts.
func saveState() error {
	mu.RLock()
	sf := stateFile_{
		Tunnels: make(map[string]stateEntry, len(active)),
	}
	for name, t := range active {
		status := stateActive
		if t.Status == StatusConnecting {
			status = stateConnecting
		}
		sf.Tunnels[name] = stateEntry{
			Pid:        t.Pid,
			Status:     status,
			Connection: t.Connection,
		}
	}
	mu.RUnlock()
	return writeStateFile(sf)
}

// UpdateEntryStatus reads the state file, updates a single entry's status,
// and writes it back. Called by the probe subprocess (separate process).
func UpdateEntryStatus(name, status string) error {
	sf := readStateFile()
	entry, ok := sf.Tunnels[name]
	if !ok {
		return nil // tunnel was stopped while probe was running — no-op
	}
	entry.Status = status
	sf.Tunnels[name] = entry
	return writeStateFile(sf)
}

// RemoveEntry reads the state file, removes a single entry, and writes it back.
// Called by the probe subprocess on failure.
func RemoveEntry(name string) error {
	sf := readStateFile()
	delete(sf.Tunnels, name)
	return writeStateFile(sf)
}

// RefreshFromState reads the state file and syncs the in-memory active map.
// Entries present in the file but not in memory are added (e.g. restored after
// the TUI was closed while a probe was running). Entries in memory but not in
// the file (e.g. stopped externally) are removed.
func RefreshFromState() {
	sf := readStateFile()

	mu.Lock()
	defer mu.Unlock()

	// Add or update entries from the state file.
	for name, entry := range sf.Tunnels {
		status := StatusActive
		if entry.Status == stateConnecting {
			status = StatusConnecting
		}
		if existing, ok := active[name]; ok {
			// Update status only — don't overwrite Cmd which we may still hold.
			existing.Status = status
		} else {
			active[name] = &ActiveTunnel{
				Connection: entry.Connection,
				Pid:        entry.Pid,
				Status:     status,
			}
		}
	}

	// Remove entries that are no longer in the state file.
	for name := range active {
		if _, ok := sf.Tunnels[name]; !ok {
			delete(active, name)
		}
	}
}

// Reconcile reads the state file on startup and health-checks each entry:
//   - connecting + pid alive  → restore as StatusConnecting (probe subprocess still running)
//   - connecting + pid dead   → remove entry (probe subprocess crashed)
//   - active + pid alive + port up   → restore as StatusActive
//   - active + pid alive + port down → kill zombie, remove entry
//   - active + pid dead       → remove entry
func Reconcile() {
	sf := readStateFile()
	if len(sf.Tunnels) == 0 {
		return
	}

	type result struct {
		name   string
		entry  stateEntry
		keep   bool
		status TunnelStatus
	}

	resultCh := make(chan result, len(sf.Tunnels))

	for name, entry := range sf.Tunnels {
		go func(name string, entry stateEntry) {
			if !pidAlive(entry.Pid) {
				resultCh <- result{name: name, entry: entry, keep: false}
				return
			}

			if entry.Status == stateConnecting {
				// Probe subprocess is still running — restore as connecting.
				resultCh <- result{name: name, entry: entry, keep: true, status: StatusConnecting}
				return
			}

			// Active entry — verify port is still reachable.
			if err := Probe(entry.Connection.LocalPort); err != nil {
				_ = syscall.Kill(-entry.Pid, syscall.SIGKILL)
				_ = syscall.Kill(entry.Pid, syscall.SIGKILL)
				resultCh <- result{name: name, entry: entry, keep: false}
				return
			}
			resultCh <- result{name: name, entry: entry, keep: true, status: StatusActive}
		}(name, entry)
	}

	changed := false
	mu.Lock()
	for range sf.Tunnels {
		r := <-resultCh
		if r.keep {
			active[r.name] = &ActiveTunnel{
				Connection: r.entry.Connection,
				Pid:        r.entry.Pid,
				Status:     r.status,
			}
		} else {
			changed = true
		}
	}
	mu.Unlock()
	close(resultCh)

	if changed {
		_ = saveState()
	}
}

// RunProbe is the entrypoint for the `diglet probe` subprocess.
// It probes localPort, updates the state file, and sends notifications.
func RunProbe(name string, localPort int) {
	Notify("diglet", name+": connecting...")

	if err := Probe(localPort); err != nil {
		// Probe timed out — remove the entry and notify failure.
		_ = RemoveEntry(name)
		// Kill the tunnel process using the pid from the state file.
		sf := readStateFile() // entry may already be gone if user stopped it
		if entry, ok := sf.Tunnels[name]; ok {
			_ = syscall.Kill(-entry.Pid, syscall.SIGKILL)
			_ = syscall.Kill(entry.Pid, syscall.SIGKILL)
		}
		Notify("diglet", name+": failed to connect")
		return
	}

	// Port is up — mark active in state file.
	if err := UpdateEntryStatus(name, stateActive); err != nil {
		Notify("diglet", name+": failed to update state")
		return
	}
	Notify("diglet", name+": connected")
}

// pidAlive returns true if the process with the given PID is running.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
