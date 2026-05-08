package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

func init() {
	Register("docker", func(conn config.Connection) *exec.Cmd {
		portArg := fmt.Sprintf("%d:%d", conn.LocalPort, conn.RemotePort)
		return exec.Command("docker", "port", conn.Container, portArg)
	})
}
