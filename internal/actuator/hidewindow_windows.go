//go:build windows

package actuator

import (
	"os/exec"
	"syscall"
)

// hideChildWindow configures cmd so the spawned process gets no visible console
// window. Without this, `cmd /c start <url>` flashes a console window when
// opening relax URLs.
func hideChildWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
