// Command agentfocus assembles the full pipeline:
//
//	watcher (Codex App Server) -> core (state machine) -> dispatcher (actuators)
//
// plus a system tray for pause/resume/config/quit and an approval popup. The
// systray event loop owns the main goroutine; the watcher->core->dispatcher
// pump runs in a background goroutine. Ctrl-C and the tray "退出" item both
// funnel through one context cancel for an orderly shutdown.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"

	"agentfocus/internal/actuator"
	"agentfocus/internal/config"
	"agentfocus/internal/core"
	"agentfocus/internal/ui"
	"agentfocus/internal/watcher"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, cfgPath, err := config.LoadOrCreate()
	if err != nil {
		log.Printf("[main] config load failed, using defaults: %v", err)
		cfg = config.Default()
	}
	log.Printf("[main] config loaded from %s (hookPort=%d, relax=%v, popup=%v)",
		cfgPath, cfg.HookServerPort, cfg.RelaxEnabled, cfg.PopupEnabled)

	// --- assemble the pipeline ------------------------------------------------

	// UI manager: a dedicated goroutine (locked to one OS thread) that runs all
	// Win32 UI — the approval dialog and the countdown toast — one at a time.
	uiMgr := ui.NewManager()
	uiStop := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		uiMgr.Run(uiStop)
	}()

	// Events arrive from Codex hooks POSTed to our local HTTP server:
	//   /hook     -> UserPromptSubmit, Stop (fire-and-forget, drive the engine)
	//   /approval -> PermissionRequest (synchronous; uiMgr shows Allow/Deny/Skip)
	src := watcher.NewHTTPSource(cfg.HookServerPort, uiMgr)

	engine := core.New(cfg)

	// RestoreIDE is handled by the countdown actuator (shows the toast, then
	// brings the IDE forward). It registers under "ide", so it replaces the bare
	// IDE actuator in the dispatcher; the bare actuator is passed in for the
	// actual foreground restore.
	ide := actuator.NewIDE()
	dispatcher := actuator.NewDispatcher(
		actuator.NewBrowser(cfg.RelaxURLs),
		ui.NewCountdownActuator(uiMgr, ide, 3),
	)

	// --- shared lifecycle -----------------------------------------------------

	ctx, cancel := context.WithCancel(context.Background())

	// shutdownOnce guarantees the orderly teardown runs exactly once whether it
	// is triggered by Ctrl-C, SIGTERM, or the tray "退出" item.
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Printf("[main] shutting down")
			cancel()      // stop the event pump and signal goroutines
			close(uiStop) // stop the UI goroutine
			src.Stop()    // stop the hook HTTP server, close Events()
		})
	}

	// Hard-exit safety net: if an orderly shutdown ever wedges, force the
	// process to exit so the tray "退出" item can never hang.
	forceExit := func() {
		go func() {
			time.Sleep(4 * time.Second)
			log.Printf("[main] shutdown timed out; forcing exit")
			os.Exit(0)
		}()
	}

	// OS signals -> shutdown, then quit the tray so the main goroutine returns.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			log.Printf("[main] signal received")
			forceExit()
			shutdown()
			systray.Quit()
		case <-ctx.Done():
		}
	}()

	// Start the watcher and the event pump in the background.
	src.Start(ctx)
	go pump(ctx, src, engine, dispatcher)

	// --- system tray (blocks the main goroutine) ------------------------------

	tray := &ui.TrayController{
		SetEnabled: engine.SetEnabled,
		ConfigPath: cfgPath,
		Quit: func() {
			forceExit() // arm the hard-exit safety net first
			shutdown()  // then attempt orderly teardown
		},
	}
	log.Printf("[main] starting system tray")
	tray.Run() // returns after systray.Quit()

	log.Printf("[main] bye")
}

// pump moves events from the source through the engine into the dispatcher
// until the context is cancelled or the events channel closes.
func pump(ctx context.Context, src watcher.Source, engine core.Engine, act actuator.Actuator) {
	events := src.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			log.Printf("[main] event kind=%s thread=%s turn=%s",
				e.Kind, e.ThreadID, e.TurnID)

			for _, a := range engine.Process(e) {
				_ = act.Do(a)
			}
		}
	}
}
