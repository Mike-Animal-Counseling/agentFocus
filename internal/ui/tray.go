package ui

import (
	_ "embed"
	"log"
	"os/exec"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var trayIcon []byte

// TrayController wires the system tray menu to the rest of the app. All fields
// are supplied by the caller (cmd/agentfocus) so the ui package stays decoupled
// from core/watcher concrete types.
type TrayController struct {
	// SetEnabled toggles the engine (pause/resume).
	SetEnabled func(enabled bool)
	// ConfigPath is the path to config.json, opened by the menu.
	ConfigPath string
	// Quit performs an orderly shutdown (context cancel + watcher stop). It is
	// invoked before systray.Quit on the "退出" menu item.
	Quit func()
}

// Run starts the systray event loop. It BLOCKS until the tray exits, so it must
// be called on the main goroutine (systray requirement on Windows/macOS).
func (t *TrayController) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *TrayController) onReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("AgentFocus")
	systray.SetTooltip("AgentFocus")

	mToggle := systray.AddMenuItem("暂停 AgentFocus", "暂停/恢复事件处理")
	mConfig := systray.AddMenuItem("打开配置文件", "用默认程序打开 config.json")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 AgentFocus")

	go func() {
		paused := false
		for {
			select {
			case <-mToggle.ClickedCh:
				paused = !paused
				if paused {
					mToggle.SetTitle("恢复 AgentFocus")
					if t.SetEnabled != nil {
						t.SetEnabled(false)
					}
					log.Printf("[tray] paused")
				} else {
					mToggle.SetTitle("暂停 AgentFocus")
					if t.SetEnabled != nil {
						t.SetEnabled(true)
					}
					log.Printf("[tray] resumed")
				}

			case <-mConfig.ClickedCh:
				t.openConfig()

			case <-mQuit.ClickedCh:
				log.Printf("[tray] quit requested")
				if t.Quit != nil {
					t.Quit()
				}
				systray.Quit()
				return
			}
		}
	}()
}

func (t *TrayController) onExit() {
	log.Printf("[tray] exited")
}

// openConfig opens the config file with the OS default program.
func (t *TrayController) openConfig() {
	if t.ConfigPath == "" {
		log.Printf("[tray] no config path to open")
		return
	}
	// cmd /c start "" <path> — the empty title arg avoids start treating a
	// quoted path as a window title.
	cmd := exec.Command("cmd", "/c", "start", "", t.ConfigPath)
	// Hide the transient cmd.exe console window so it doesn't flash on screen.
	hideChildWindow(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("[tray] failed to open config %s: %v", t.ConfigPath, err)
		return
	}
	go func() { _ = cmd.Wait() }()
	log.Printf("[tray] opened config %s", t.ConfigPath)
}
