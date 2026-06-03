//go:build !windows

package actuator

import "fmt"

// bringToForeground is a no-op stub on non-Windows platforms so the package
// stays cross-compilable. The real implementation lives in ide_windows.go.
func bringToForeground(titleSubstr string) error {
	return fmt.Errorf("bringToForeground not supported on this platform")
}

// raiseWindowByTitle is a no-op stub on non-Windows platforms.
func raiseWindowByTitle(titleSubstr, tag string) error {
	return fmt.Errorf("raiseWindowByTitle not supported on this platform")
}

// raiseWindowByPID is a no-op stub on non-Windows platforms.
func raiseWindowByPID(pid uint32, tag string) error {
	return fmt.Errorf("raiseWindowByPID not supported on this platform")
}
