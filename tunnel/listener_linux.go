//go:build linux

package tunnel

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var listenerPIDPattern = regexp.MustCompile(`pid=(\d+)`)

func listeningPorts() (map[int]map[int]bool, error) {
	out, err := exec.Command("ss", "-H", "-ltnp").Output()
	if err != nil {
		return nil, fmt.Errorf("could not inspect listening ports with ss: %w", err)
	}
	ports := map[int]map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		address := fields[3]
		colon := strings.LastIndexByte(address, ':')
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(address[colon+1:])
		if err == nil {
			pid := 0
			if match := listenerPIDPattern.FindStringSubmatch(line); match != nil {
				pid, _ = strconv.Atoi(match[1])
			}
			if ports[port] == nil {
				ports[port] = map[int]bool{}
			}
			ports[port][pid] = true
		}
	}
	return ports, nil
}
