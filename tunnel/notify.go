package tunnel

import "os/exec"

// Notify sends a desktop notification via notify-send.
// Runs fire-and-forget in a goroutine; silently does nothing if notify-send
// is not installed.
func Notify(title, body string) {
	go exec.Command("notify-send", title, body).Run() //nolint:errcheck
}
