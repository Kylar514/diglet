package tunnel

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/kylar514/diglet/config"
)

type ActiveTunnel struct {
	Connection config.Connection
	Cmd        *exec.Cmd
	Pid        int
}

var (
	active   = map[string]*ActiveTunnel{}
	builders = map[string]func(config.Connection) (*exec.Cmd, error){}
)

func Register(tunnelType string, builder func(config.Connection) (*exec.Cmd, error)) {
	builders[tunnelType] = builder
}

func StartProcess(conn config.Connection) error {
	if _, exists := active[conn.Name]; exists {
		return fmt.Errorf("tunnel %q already active", conn.Name)
	}

	builder, ok := builders[conn.TunnelType]
	if !ok {
		return fmt.Errorf("unknown tunnel_type %q", conn.TunnelType)
	}

	cmd, err := builder(conn)
	if err != nil {
		return fmt.Errorf("tunnel %q: %w", conn.Name, err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tunnel %q: %w", conn.Name, err)
	}

	active[conn.Name] = &ActiveTunnel{
		Connection: conn,
		Cmd:        cmd,
		Pid:        cmd.Process.Pid,
	}

	return nil
}

func Confirm(name string) error {
	if _, exists := active[name]; !exists {
		return fmt.Errorf("no active tunnel named %q", name)
	}
	return saveState()
}

func Abort(name string) {
	t, exists := active[name]
	if !exists {
		return
	}
	pid := t.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	if t.Cmd != nil {
		_ = t.Cmd.Wait()
	}
	delete(active, name)
}

func Stop(name string) error {
	t, exists := active[name]
	if !exists {
		return fmt.Errorf("no active tunnel named %q", name)
	}

	pid := t.Pid
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		if err2 := syscall.Kill(pid, syscall.SIGKILL); err2 != nil {
			return fmt.Errorf("failed to kill tunnel %q (pid %d): %w", name, pid, err2)
		}
	}

	if t.Cmd != nil {
		_ = t.Cmd.Wait()
	}

	delete(active, name)

	if err := saveState(); err != nil {
		_ = err
	}

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
