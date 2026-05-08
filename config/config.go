package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Connection struct {
	Name       string `yaml:"name"`
	TunnelType string `yaml:"tunnel_type"`
	Resource   string `yaml:"resource"`
	Namespace  string `yaml:"namespace"`
	Container  string `yaml:"container"`
	SSHHost    string `yaml:"ssh_host"`
	RemotePort int    `yaml:"remote_port"`
	LocalPort  int    `yaml:"local_port"`
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
