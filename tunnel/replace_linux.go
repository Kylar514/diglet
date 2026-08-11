//go:build linux

package tunnel

import "os"

func replaceFile(from, to string) error {
	return os.Rename(from, to)
}
