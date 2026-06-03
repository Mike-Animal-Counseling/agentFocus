// Package event defines the standard event and action types that flow through
// the agentfocus pipeline. It is pure data with no dependencies on other
// internal packages, so every layer can import it freely.
package event

import "time"

// EventKind enumerates the kinds of events the watcher can emit.
type EventKind int

const (
	// SessionStarted is emitted when a Codex session/turn begins.
	SessionStarted EventKind = iota
	// ApprovalRequested is emitted when the App Server asks the user to
	// approve an action (e.g. running a command).
	ApprovalRequested
	// TaskCompleted is emitted when a turn finishes.
	TaskCompleted
	// SessionEnded is emitted when the session/thread is closed.
	SessionEnded
)

// String returns a human-readable name for the event kind.
func (k EventKind) String() string {
	switch k {
	case SessionStarted:
		return "SessionStarted"
	case ApprovalRequested:
		return "ApprovalRequested"
	case TaskCompleted:
		return "TaskCompleted"
	case SessionEnded:
		return "SessionEnded"
	default:
		return "UnknownEvent"
	}
}

// Event is a normalized event produced by a watcher.Source.
type Event struct {
	Kind      EventKind
	ThreadID  string
	TurnID    string
	Timestamp time.Time
	// Detail carries optional human-facing context about the event. For
	// ApprovalRequested it holds the command Codex wants to run, so the
	// approval popup can show it.
	Detail string
}

// ActionKind enumerates the side-effecting actions an actuator can perform.
type ActionKind int

const (
	// OpenRelax opens the relax/focus surface.
	OpenRelax ActionKind = iota
	// ShowApprovalPopup surfaces an approval prompt to the user.
	ShowApprovalPopup
	// RestoreIDE brings the IDE back to the foreground.
	RestoreIDE
	// CloseRelax closes the relax/focus surface.
	CloseRelax
)

// String returns a human-readable name for the action kind.
func (k ActionKind) String() string {
	switch k {
	case OpenRelax:
		return "OpenRelax"
	case ShowApprovalPopup:
		return "ShowApprovalPopup"
	case RestoreIDE:
		return "RestoreIDE"
	case CloseRelax:
		return "CloseRelax"
	default:
		return "UnknownAction"
	}
}

// Action is a decision produced by core.Engine for an actuator to carry out.
type Action struct {
	Kind   ActionKind
	Reason string
}
