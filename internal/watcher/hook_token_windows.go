//go:build windows

package watcher

import (
	"golang.org/x/sys/windows"
	"os"
	"os/user"
	"path/filepath"
)

// Create credentials under a directory restricted to this user and SYSTEM.
func writeHookToken(token string) error {
	current, err := user.Current()
	if err != nil {
		return err
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "AgentFocus", "auth")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + current.Uid + ")(A;;FA;;;SY)")
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return err
	}
	path := filepath.Join(dir, "hook.token")
	// Remove the old file first so a pre-existing broader file ACL cannot survive.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
