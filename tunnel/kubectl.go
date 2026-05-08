package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

func init() {
	Register("kubectl", func(conn config.Connection) *exec.Cmd {
		portArg := fmt.Sprintf("%d:localhost:%d", conn.LocalPort, conn.RemotePort)
		return exec.Command("kubectl", "port-forward",
			"svc/"+conn.Service,
			portArg,
			"-n", conn.Namespace,
		)
	})
}
