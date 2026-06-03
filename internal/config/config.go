// Package config holds user-facing configuration and its persistence to
// %APPDATA%\AgentFocus\config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config holds user-configurable settings for agentfocus.
type Config struct {
	// CodexPath is the absolute path to the codex executable used to spawn
	// `codex app-server`. Empty means "resolve from PATH".
	CodexPath string `json:"codexPath"`

	// FakeEventInterval is how often the fake watcher emits events. Serialized
	// as a Go duration string (e.g. "5s").
	FakeEventInterval Duration `json:"fakeEventInterval"`

	// AutoApprove, when true, lets the engine auto-respond to approval
	// requests instead of surfacing a popup.
	AutoApprove bool `json:"autoApprove"`

	// RelaxEnabled gates the relax/focus surface. When false, the engine
	// emits neither OpenRelax nor CloseRelax actions.
	RelaxEnabled bool `json:"relaxEnabled"`

	// PopupEnabled gates the approval popup. When false, the engine does not
	// emit ShowApprovalPopup actions.
	PopupEnabled bool `json:"popupEnabled"`

	// RelaxURLs are the URLs opened by the browser actuator on OpenRelax.
	RelaxURLs []string `json:"relaxURLs"`

	// HookServerPort is the localhost port the hook HTTP server listens on,
	// receiving Codex hook payloads POSTed to /hook.
	HookServerPort int `json:"hookServerPort"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		CodexPath:         "",
		FakeEventInterval: Duration(5 * time.Second),
		AutoApprove:       false,
		RelaxEnabled:      true,
		PopupEnabled:      true,
		RelaxURLs: []string{
			"https://www.douyin.com",
			"https://www.xiaohongshu.com",
		},
		HookServerPort: 27182,
	}
}

// Path returns the absolute path to the config file:
// %APPDATA%\AgentFocus\config.json. It falls back to the OS user config dir
// when APPDATA is unset.
func Path() (string, error) {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		var err error
		dir, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "AgentFocus", "config.json"), nil
}

// LoadOrCreate reads the config from Path(). If the file does not exist, it
// writes Default() to disk and returns it. Returns the config and the resolved
// path.
func LoadOrCreate() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Config{}, "", err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		if werr := save(path, cfg); werr != nil {
			return Config{}, path, werr
		}
		return cfg, path, nil
	}
	if err != nil {
		return Config{}, path, err
	}

	cfg := Default() // start from defaults so missing JSON fields stay sane
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

// save writes cfg as indented JSON to path, creating parent directories.
func save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Duration is a time.Duration that marshals to/from a Go duration string in
// JSON (e.g. "5s") instead of an opaque nanosecond integer.
type Duration time.Duration

// MarshalJSON renders the duration as a quoted Go duration string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts either a duration string ("5s") or a numeric
// nanosecond count for compatibility.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case string:
		parsed, err := time.ParseDuration(val)
		if err != nil {
			return err
		}
		*d = Duration(parsed)
	case float64:
		*d = Duration(time.Duration(val))
	}
	return nil
}

// Std returns the underlying time.Duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }
