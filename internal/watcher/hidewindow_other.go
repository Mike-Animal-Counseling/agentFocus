//go:build !windows

package watcher

import "os/exec"

// hideChildWindow is a no-op on non-Windows platforms (no console window to
// hide). Keeps codex.go cross-compilable.
func hideChildWindow(cmd *exec.Cmd) {}
