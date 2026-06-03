//go:build windows

package watcher

import (
	"os/exec"
	"syscall"
)

// hideChildWindow configures cmd so the spawned process gets no visible console
// window. Without this, launching `codex app-server` from the GUI app pops a
// stray console window.
func hideChildWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
