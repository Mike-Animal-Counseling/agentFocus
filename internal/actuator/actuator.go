// Package actuator carries out actions decided by core.Engine. This step
// defines only the interface plus a fake implementation that logs actions.
package actuator

import "agentfocus/internal/event"

// Actuator performs a single side-effecting action.
type Actuator interface {
	// Do carries out the action, returning an error if it fails.
	Do(a event.Action) error
	// Name identifies the actuator for logging and diagnostics.
	Name() string
}
