package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Connection struct {
	Name       string `yaml:"name"`
	TunnelType string `yaml:"tunnel_type"`

	// kubectl fields
	Resource  string `yaml:"resource"`
	Namespace string `yaml:"namespace"`

	// docker fields
	Container string `yaml:"container"`

	// ssh fields
	SSHHost string `yaml:"ssh_host"`

	RemotePort int `yaml:"remote_port"`
	LocalPort  int `yaml:"local_port"`
}

type Config struct {
	Connections []Connection `yaml:"connections"`
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "diglet", "connections.yaml")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	return &cfg, nil
}

// IsNotExist reports whether the error from Load indicates the config file
// does not exist yet.
func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// scaffoldContent is the starter connections.yaml written by Scaffold.
const scaffoldContent = `# diglet — tunnel manager connections
# https://github.com/kylar514/diglet
#
# Each connection requires: name, tunnel_type, local_port, remote_port
# plus the type-specific fields shown in the examples below.
#
# Reload this file at any time by pressing 'e' inside diglet.

connections:

  # ── SSH ─────────────────────────────────────────────────────────────────────
  # Forwards local_port on 127.0.0.1 to remote_port through ssh_host.
  # ssh_host accepts anything valid for the ssh command: user@host, an alias
  # from ~/.ssh/config, etc.
  #
  # - name: my-server-postgres
  #   tunnel_type: ssh
  #   ssh_host: user@example.com
  #   local_port: 5432
  #   remote_port: 5432

  # ── kubectl ─────────────────────────────────────────────────────────────────
  # Port-forwards to a Kubernetes resource using the current kubeconfig context.
  # resource accepts: pod/<name>, svc/<name>, deployment/<name>
  # namespace is optional; omit it to use the current context's default namespace.
  #
  # - name: my-api
  #   tunnel_type: kubectl
  #   resource: svc/my-api
  #   namespace: production
  #   local_port: 8080
  #   remote_port: 80
  #
  # - name: postgres-lab
  #   tunnel_type: kubectl
  #   resource: pod/postgres
  #   namespace: lab
  #   local_port: 5432
  #   remote_port: 5432

  # ── docker ──────────────────────────────────────────────────────────────────
  # Docker tunnel support is not yet implemented.
  #
  # - name: my-container
  #   tunnel_type: docker
  #   container: my-container
  #   local_port: 3000
  #   remote_port: 3000
`

// Scaffold creates the config directory if needed and writes a starter
// connections.yaml with commented-out examples of every tunnel type.
// Returns an error if the file already exists or cannot be written.
func Scaffold(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	// Don't overwrite an existing file.
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists at %s", path)
	}

	if err := os.WriteFile(path, []byte(scaffoldContent), 0o600); err != nil {
		return fmt.Errorf("could not write config file: %w", err)
	}

	return nil
}
