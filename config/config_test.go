package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAllowsSharedPorts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.yaml")
	data := `connections:
  - name: one
    tunnel_type: ssh
    local_port: 9000
    remote_port: 80
  - name: two
    tunnel_type: kubectl
    local_port: 9000
    remote_port: 81
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Connections) != 2 {
		t.Fatalf("Load() returned %d connections, want 2", len(cfg.Connections))
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.yaml")
	data := `connections:
  - name: bad
    tunnel_type: ssh
    local_port: 70000
    remote_port: 80
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "invalid local_port 70000") {
		t.Fatalf("Load() error = %v, want invalid port error", err)
	}
}
