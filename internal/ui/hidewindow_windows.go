//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// hideChildWindow configures cmd so the spawned process gets no visible console
// window. Without this, `cmd /c start ...` flashes a console window when the
// tray opens the config file.
func hideChildWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
