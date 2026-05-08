package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

func init() {
	Register("ssh", func(conn config.Connection) *exec.Cmd {
		sshArg := fmt.Sprintf("%d:localhost:%d", conn.LocalPort, conn.RemotePort)
		return exec.Command("ssh", "-L", sshArg, "-N", conn.SSHHost)
	})
}
