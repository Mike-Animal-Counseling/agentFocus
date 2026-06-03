//go:build !windows

package ui

import "os/exec"

// hideChildWindow is a no-op on non-Windows platforms (no console window to
// hide). Keeps tray.go cross-compilable.
func hideChildWindow(cmd *exec.Cmd) {}
