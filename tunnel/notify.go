package tunnel

import "os/exec"

// Notify sends a desktop notification when notify-send is available.
// Runs fire-and-forget in a goroutine; safe to call from the TUI process.
func Notify(title, body string) {
	go exec.Command("notify-send", title, body).Run() //nolint:errcheck
}

// NotifySync sends a desktop notification when notify-send is available, blocking until it
// completes. Must be used from short-lived processes (e.g. the probe subprocess)
// that exit immediately after calling this — a goroutine would be killed first.
func NotifySync(title, body string) {
	_ = exec.Command("notify-send", title, body).Run()
}
