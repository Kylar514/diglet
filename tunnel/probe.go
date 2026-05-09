package tunnel

import (
	"fmt"
	"net"
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
