package actuator

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"agentfocus/internal/event"
)

// raiseRelaxWindow brings the relax Chrome window (owned by pid) to the
// foreground. The window may take a moment to appear after launch, so it retries
// briefly. Safe to call in a goroutine.
func raiseRelaxWindow(pid uint32) {
	for i := 0; i < 20; i++ { // up to ~4s
		if err := raiseWindowByPID(pid, "browser"); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	logf("[actuator:browser] could not raise relax window pid=%d (gave up)", pid)
}

// BrowserActuator opens "relax" URLs in a dedicated Chrome instance and reuses
// it across prompts. It handles OpenRelax. (CloseRelax is no longer emitted by
// the engine — the browser is intentionally left open so the user keeps their
// pages; it is still handled here as a no-op-ish safety.)
//
// Reuse logic: the relax pages open in an isolated Chrome profile
// (--user-data-dir), which (unlike the user's normal Chrome) yields a real,
// long-lived process we can track. On OpenRelax we check whether that process
// is still alive:
//   - alive  -> the user hasn't closed it; do nothing (reuse existing pages)
//   - gone   -> the user closed it; launch a fresh relax window
type BrowserActuator struct {
	urls       []string
	chromePath string // "" if Chrome not found (falls back to `cmd /c start`)
	profileDir string

	mu        sync.Mutex
	relaxProc *os.Process // current relax Chrome process (nil if none)
	relaxDead bool        // set true by the Wait goroutine when it exits
}

// NewBrowser returns a BrowserActuator that opens the given URLs on OpenRelax.
func NewBrowser(urls []string) *BrowserActuator {
	return &BrowserActuator{
		urls:       urls,
		chromePath: findChrome(),
		profileDir: filepath.Join(os.Getenv("LOCALAPPDATA"), "AgentFocus", "relax-profile"),
	}
}

// findChrome returns the path to chrome.exe, or "" if not found.
func findChrome() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), `Google\Chrome\Application\chrome.exe`),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `Google\Chrome\Application\chrome.exe`),
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
	}
	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return ""
}

// Name identifies this actuator for dispatch and logging.
func (b *BrowserActuator) Name() string { return "browser" }

// Do carries out OpenRelax / CloseRelax. Unsupported actions are ignored.
func (b *BrowserActuator) Do(a event.Action) error {
	switch a.Kind {
	case event.OpenRelax:
		return b.openRelax()
	case event.CloseRelax:
		return b.closeRelax()
	default:
		logf("[actuator:browser] ignoring unsupported action=%s", a.Kind)
		return nil
	}
}

func (b *BrowserActuator) openRelax() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Reuse: if the relax window is still open, just bring it to the foreground.
	if b.relaxProc != nil && !b.relaxDead {
		pid := uint32(b.relaxProc.Pid)
		logf("[actuator:browser] relax window still open (pid=%d); reusing + raising", pid)
		go raiseRelaxWindow(pid)
		return nil
	}

	// No Chrome found: fall back to opening URLs via the default browser
	// (cannot track reuse in this mode, but still opens the pages).
	if b.chromePath == "" {
		return b.openViaDefault()
	}

	// Launch a dedicated relax Chrome window with all URLs, maximized.
	args := []string{"--user-data-dir=" + b.profileDir, "--new-window", "--start-maximized"}
	args = append(args, b.urls...)
	cmd := exec.Command(b.chromePath, args...)
	hideChildWindow(cmd)
	if err := cmd.Start(); err != nil {
		logf("[actuator:browser] failed to launch relax Chrome: %v", err)
		return err
	}

	b.relaxProc = cmd.Process
	b.relaxDead = false
	logf("[actuator:browser] opened relax window (pid=%d, %d urls)", cmd.Process.Pid, len(b.urls))

	// Bring the newly opened window to the foreground once it appears.
	go raiseRelaxWindow(uint32(cmd.Process.Pid))

	// Watch for the process exiting (user closed the window) so the next
	// OpenRelax knows to launch a fresh one.
	proc := cmd.Process
	go func() {
		_, _ = cmd.Process.Wait()
		b.mu.Lock()
		if b.relaxProc == proc {
			b.relaxDead = true
			logf("[actuator:browser] relax window closed (pid=%d); will reopen next time", proc.Pid)
		}
		b.mu.Unlock()
	}()
	return nil
}

// openViaDefault opens each URL with the OS default browser via `cmd /c start`.
// Used only when chrome.exe is not found; reuse tracking is unavailable here.
func (b *BrowserActuator) openViaDefault() error {
	var firstErr error
	for _, url := range b.urls {
		cmd := exec.Command("cmd", "/c", "start", url)
		hideChildWindow(cmd)
		if err := cmd.Start(); err != nil {
			logf("[actuator:browser] failed to open %s: %v", url, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		logf("[actuator:browser] opened %s via default browser", url)
		go func(c *exec.Cmd) { _ = c.Wait() }(cmd)
	}
	return firstErr
}

// closeRelax closes the dedicated relax window if we launched one. The engine no
// longer emits CloseRelax, but we keep this functional for completeness.
func (b *BrowserActuator) closeRelax() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.relaxProc != nil && !b.relaxDead {
		if err := b.relaxProc.Kill(); err != nil {
			logf("[actuator:browser] could not close relax window pid=%d: %v", b.relaxProc.Pid, err)
		} else {
			logf("[actuator:browser] closed relax window pid=%d", b.relaxProc.Pid)
		}
	}
	b.relaxProc = nil
	b.relaxDead = false
	return nil
}
