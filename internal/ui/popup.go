// Package ui provides the system tray, the approval dialog, and the countdown
// toast. It is the only place that surfaces UI to the user. All Win32 UI runs on
// the Manager's dedicated goroutine.
package ui

import (
	"log"

	"agentfocus/internal/event"
)

// IDERestorer is the subset of the IDE actuator the UI needs: a way to pull the
// IDE back to the foreground.
type IDERestorer interface {
	Restore()
}

// CountdownActuator handles the RestoreIDE action: it shows a short countdown
// toast ("Codex 已完成，N 秒后跳回 VSCode") and then brings the IDE forward.
// It registers under the "ide" name so the dispatcher routes RestoreIDE here.
type CountdownActuator struct {
	mgr     *Manager
	ide     IDERestorer
	seconds int
}

// NewCountdownActuator returns an actuator that counts down `seconds` before
// restoring the IDE via ide.
func NewCountdownActuator(mgr *Manager, ide IDERestorer, seconds int) *CountdownActuator {
	return &CountdownActuator{mgr: mgr, ide: ide, seconds: seconds}
}

// Name routes RestoreIDE to this actuator.
func (c *CountdownActuator) Name() string { return "ide" }

// Do handles RestoreIDE; other actions are ignored.
func (c *CountdownActuator) Do(a event.Action) error {
	if a.Kind != event.RestoreIDE {
		log.Printf("[ui] ignoring unsupported action=%s", a.Kind)
		return nil
	}
	// Run the countdown (blocks on the UI goroutine) off the dispatcher
	// goroutine so the event pump is not stalled, then restore the IDE.
	go func() {
		if c.seconds > 0 {
			c.mgr.Countdown(c.seconds)
		}
		if c.ide != nil {
			c.ide.Restore()
		}
	}()
	return nil
}
