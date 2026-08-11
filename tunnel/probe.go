package tunnel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	probeTimeout  = 30 * time.Second
	probeInterval = 100 * time.Millisecond
)

func Probe(localPort int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	deadline := time.Now().Add(probeTimeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, probeInterval)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(probeInterval)
	}

	return fmt.Errorf("port %d did not become reachable within %s", localPort, probeTimeout)
}

func RunProbe(name string, port, tunnelPID int) {
	NotifySync("diglet", name+": connecting...")
	if err := Probe(port); err != nil {
		_ = stop(name, tunnelPID)
		NotifySync("diglet", name+": failed to connect")
		return
	}
	NotifySync("diglet", name+": connected")
}

func StartProbe(name string, port, tunnelPID int) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable: %w", err)
	}
	cmd := exec.Command(exe, "probe", name, strconv.Itoa(port), strconv.Itoa(tunnelPID))
	prepareCommand(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start probe: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
