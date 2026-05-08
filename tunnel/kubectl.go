package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

func init() {
	Register("kubectl", func(conn config.Connection) (*exec.Cmd, error) {
		resource := conn.Resource
		if resource == "" {
			return nil, fmt.Errorf("kubectl tunnel %q requires 'resource' to be set (e.g. svc/my-service or pod/my-pod)", conn.Name)
		}

		portArg := fmt.Sprintf("%d:%d", conn.LocalPort, conn.RemotePort)
		args := []string{"port-forward", resource, portArg}
		if conn.Namespace != "" {
			args = append(args, "-n", conn.Namespace)
		}

		return exec.Command("kubectl", args...), nil
	})
}
