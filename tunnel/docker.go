package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/kylar514/diglet/config"
)

func init() {
	Register("docker", func(conn config.Connection) (*exec.Cmd, error) {
		return nil, fmt.Errorf("docker tunnel type is not yet implemented")
	})
}
