package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

type ActiveTunnel struct {
	Connection config.Connection
	Cmd        *exec.Cmd
}

var (
	active   = map[string]*ActiveTunnel{}
	builders = map[string]func(config.Connection) *exec.Cmd{}
)

func Register(tunnelType string, builder func(config.Connection) *exec.Cmd) {
	builders[tunnelType] = builder
}

func Start(conn config.Connection) error {
	if _, exists := active[conn.Name]; exists {
		return fmt.Errorf("tunnel %q already active", conn.Name)
	}

	builder, ok := builders[conn.TunnelType]
	if !ok {
		return fmt.Errorf("unknown tunnel_type %q", conn.TunnelType)
	}

	cmd := builder(conn)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to start tunnel %q: %w", conn.Name, err)
	}

	return nil
}

func Stop(name string) error {
	t, exists := active[name]
	if !exists {
		return fmt.Errorf("no active tunnel named %q", name)
	}

	if err := t.Cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill tunnel %q: %w", name, err)
	}

	delete(active, name)
	return nil
}

func List() []*ActiveTunnel {
	tunnels := make([]*ActiveTunnel, 0, len(active))
	for _, t := range active {
		tunnels = append(tunnels, t)
	}

	return tunnels
}

func IsActive(name string) bool {
	_, exists := active[name]
	return exists
}

func RegisteredTypes() []string {
	types := make([]string, 0, len(builders))
	for k := range builders {
		types = append(types, k)
	}
	return types
}
