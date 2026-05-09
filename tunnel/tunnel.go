package tunnel

import (
	"fmt"
	"os/exec"
	"sort"
	"sync"
	"syscall"

	"github.com/kylar514/diglet/config"
)

type TunnelStatus int

const (
	StatusConnecting TunnelStatus = iota
	StatusActive
)

type ActiveTunnel struct {
	Connection config.Connection
	Cmd        *exec.Cmd
	Pid        int
	Status     TunnelStatus
}

var (
	mu       sync.RWMutex
	active   = map[string]*ActiveTunnel{}
	builders = map[string]func(config.Connection) (*exec.Cmd, error){}
)

func Register(tunnelType string, builder func(config.Connection) (*exec.Cmd, error)) {
	builders[tunnelType] = builder
}

func StartProcess(conn config.Connection) error {
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

	mu.Lock()
	if _, exists := active[conn.Name]; exists {
		mu.Unlock()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
		return fmt.Errorf("tunnel %q already active", conn.Name)
	}
	active[conn.Name] = &ActiveTunnel{
		Connection: conn,
		Cmd:        cmd,
		Pid:        cmd.Process.Pid,
		Status:     StatusConnecting,
	}
	mu.Unlock()

	// Persist the connecting entry immediately so it survives TUI restarts.
	return saveState()
}

func Abort(name string) {
	mu.Lock()
	t, exists := active[name]
	if !exists {
		mu.Unlock()
		return
	}
	delete(active, name)
	mu.Unlock()

	pid := t.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	if t.Cmd != nil {
		_ = t.Cmd.Wait()
	}
}

func Stop(name string) error {
	mu.Lock()
	t, exists := active[name]
	if !exists {
		mu.Unlock()
		return fmt.Errorf("no active tunnel named %q", name)
	}
	delete(active, name)
	mu.Unlock()

	pid := t.Pid
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		if err2 := syscall.Kill(pid, syscall.SIGKILL); err2 != nil {
			return fmt.Errorf("failed to kill tunnel %q (pid %d): %w", name, pid, err2)
		}
	}

	if t.Cmd != nil {
		_ = t.Cmd.Wait()
	}

	return saveState()
}

func List() []*ActiveTunnel {
	mu.RLock()
	defer mu.RUnlock()
	tunnels := make([]*ActiveTunnel, 0, len(active))
	for _, t := range active {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// IsActive returns true only if the tunnel is fully connected (StatusActive).
func IsActive(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	t, exists := active[name]
	return exists && t.Status == StatusActive
}

// IsConnecting returns true if the tunnel process is running but the probe
// has not yet completed.
func IsConnecting(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	t, exists := active[name]
	return exists && t.Status == StatusConnecting
}

func RegisteredTypes() []string {
	types := make([]string, 0, len(builders))
	for k := range builders {
		types = append(types, k)
	}
	sort.Strings(types)
	return types
}

// GetPid returns the PID of the named active tunnel, or 0 if not active.
func GetPid(name string) int {
	mu.RLock()
	defer mu.RUnlock()
	if t, exists := active[name]; exists {
		return t.Pid
	}
	return 0
}

// StopAll stops every active tunnel. Returns a slice of any errors encountered.
// Tunnels that fail to stop are left in the active map.
func StopAll() []error {
	mu.RLock()
	names := make([]string, 0, len(active))
	for name := range active {
		names = append(names, name)
	}
	mu.RUnlock()

	var errs []error
	for _, name := range names {
		if err := Stop(name); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
