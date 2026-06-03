package core

import (
	"sync"

	"agentfocus/internal/config"
	"agentfocus/internal/event"
)

// engine is the default Engine implementation. It holds the current state and
// delegates each decision to the pure transition function.
type engine struct {
	mu      sync.Mutex
	state   State
	cfg     config.Config
	enabled bool
}

// New returns the default Engine, starting in the Idle state and enabled.
func New(cfg config.Config) Engine {
	return &engine{
		state:   Idle,
		cfg:     cfg,
		enabled: true,
	}
}

// SetEnabled toggles processing. When disabled, Process is a no-op. The state
// machine is left as-is so processing resumes coherently when re-enabled.
func (e *engine) SetEnabled(enabled bool) {
	e.mu.Lock()
	e.enabled = enabled
	e.mu.Unlock()
}

// Process advances the state machine for one event and returns the actions to
// perform. When disabled it returns no actions. Guarded by a mutex so the
// systray goroutine can toggle enabled while events are processed.
func (e *engine) Process(ev event.Event) []event.Action {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.enabled {
		return nil
	}
	next, actions := transition(e.state, ev, e.cfg)
	e.state = next
	return actions
}
