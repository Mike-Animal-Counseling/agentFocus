//go:build !windows

package watcher

import (
	"os"
	"path/filepath"
)

func writeHookToken(token string) error {
	root, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "AgentFocus", "auth")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "hook.token")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(token), 0600)
}
