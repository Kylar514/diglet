//go:build linux

package tunnel

import (
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/kylar514/diglet/config"
)

func TestStatusesDistinguishesOwnedAndOccupiedListeners(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	connection := config.Connection{Name: "test", TunnelType: "ssh", LocalPort: port, RemotePort: 80}
	shared := config.Connection{Name: "shared", TunnelType: "kubectl", LocalPort: port, RemotePort: 81}

	statuses, err := Statuses([]config.Connection{connection})
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses[connection.Name].Status; got != StatusOccupied {
		t.Fatalf("unowned listener status = %v, want occupied", got)
	}

	identity, err := processIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := addEntry(connection, os.Getpid(), identity); err != nil {
		t.Fatal(err)
	}
	statuses, err = Statuses([]config.Connection{connection, shared})
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses[connection.Name].Status; got != StatusActive {
		t.Fatalf("owned listener status = %v, want active", got)
	}
	if got := statuses[shared.Name]; got.Status != StatusOccupied || got.Owner != connection.Name {
		t.Fatalf("shared listener info = %+v, want occupied by %q", got, connection.Name)
	}
	if err := withState(func(state *stateFile) (bool, error) {
		entry := state.Tunnels[connection.Name]
		entry.Identity++
		state.Tunnels[connection.Name] = entry
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	statuses, err = Statuses([]config.Connection{connection})
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses[connection.Name].Status; got != StatusOccupied {
		t.Fatalf("stale ownership status = %v, want occupied", got)
	}
}
