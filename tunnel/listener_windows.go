//go:build windows

package tunnel

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func listeningPorts() (map[int]map[int]bool, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("could not inspect listening ports with netstat: %w", err)
	}
	ports := map[int]map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		colon := strings.LastIndexByte(fields[1], ':')
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(fields[1][colon+1:])
		if err == nil {
			pid, pidErr := strconv.Atoi(fields[4])
			if pidErr == nil {
				if ports[port] == nil {
					ports[port] = map[int]bool{}
				}
				ports[port][pid] = true
			}
		}
	}
	return ports, nil
}
