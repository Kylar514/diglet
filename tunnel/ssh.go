package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

func init() {
	Register("ssh", func(conn config.Connection) (*exec.Cmd, error) {
		if conn.SSHHost == "" {
			return nil, fmt.Errorf("ssh tunnel %q requires 'ssh_host' to be set", conn.Name)
		}
		sshArg := fmt.Sprintf("%d:localhost:%d", conn.LocalPort, conn.RemotePort)
		return exec.Command("ssh", "-o", "ExitOnForwardFailure=yes", "-L", sshArg, "-N", conn.SSHHost), nil
	})
}
