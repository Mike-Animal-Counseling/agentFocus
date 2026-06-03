// Package core holds the decision state machine that maps incoming events to
// actions. It depends only on internal/event — never on the concrete watcher
// or actuator implementations — so the policy stays isolated and testable.
package core

import "agentfocus/internal/event"

// Engine consumes events and decides which actions to take.
type Engine interface {
	// Process handles a single event and returns the actions to perform, in
	// order. Returning an empty slice means "do nothing".
	Process(e event.Event) []event.Action
	// SetEnabled toggles the engine. When disabled, Process returns no
	// actions (AgentFocus is "paused"). Safe for concurrent use with Process.
	SetEnabled(enabled bool)
}
