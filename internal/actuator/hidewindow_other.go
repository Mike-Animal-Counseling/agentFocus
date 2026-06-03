//go:build !windows

package actuator

import "os/exec"

// hideChildWindow is a no-op on non-Windows platforms (no console window to
// hide). Keeps browser.go cross-compilable.
func hideChildWindow(cmd *exec.Cmd) {}
