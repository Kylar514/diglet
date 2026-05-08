package tunnel

import (
	"encoding/json"
	"os"
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

	changed := false
	for name, entry := range sf.Tunnels {
		if pidAlive(entry.Pid) {
			active[name] = &ActiveTunnel{
				Connection: entry.Connection,
				Pid:        entry.Pid,
			}
		} else {
			changed = true
		}
	}

	if changed {
		_ = saveState()
	}
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
