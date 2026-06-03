package core

import (
	"log"

	"agentfocus/internal/config"
	"agentfocus/internal/event"
)

// State is the lifecycle state of a Codex session as tracked by the engine.
type State int

const (
	// Idle: no active session.
	Idle State = iota
	// Working: a session is running and the relax surface may be open.
	Working
	// WaitingApproval: the session is blocked on a user approval.
	WaitingApproval
)

// String returns a human-readable name for the state.
func (s State) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Working:
		return "Working"
	case WaitingApproval:
		return "WaitingApproval"
	default:
		return "UnknownState"
	}
}

// transition is the pure heart of the engine: given the current state, an
// event, and config, it returns the next state and the actions to perform. It
// has no side effects — callers are responsible for executing the actions.
//
// Config gates which actions are emitted:
//   - RelaxEnabled=false suppresses OpenRelax and CloseRelax.
//   - PopupEnabled=false suppresses ShowApprovalPopup.
func transition(state State, e event.Event, cfg config.Config) (State, []event.Action) {
	switch state {
	case Idle:
		switch e.Kind {
		case event.SessionStarted:
			return Working, startActions(cfg)
		}

	case Working:
		switch e.Kind {
		case event.SessionStarted:
			// New prompt while relax is already open: reuse the existing
			// browser pages, emit nothing. (We never close the browser, so the
			// user keeps watching where they left off.)
			return Working, nil
		case event.ApprovalRequested:
			// Surface the approval popup. We do NOT close the browser here;
			// clicking the popup brings the IDE forward.
			var actions []event.Action
			if cfg.PopupEnabled {
				actions = append(actions, event.Action{
					Kind:   event.ShowApprovalPopup,
					Reason: "approval requested",
				})
			}
			return WaitingApproval, actions
		case event.TaskCompleted, event.SessionEnded:
			// Task done: bring the IDE back. Leave the browser open for reuse.
			return Idle, restoreActions(cfg, reasonFor(e.Kind))
		}

	case WaitingApproval:
		switch e.Kind {
		case event.SessionStarted:
			// New prompt arrived while still flagged as waiting: reuse browser.
			return Working, nil
		case event.ApprovalRequested:
			// Already waiting; do not re-emit the popup.
			return WaitingApproval, nil
		case event.TaskCompleted, event.SessionEnded:
			return Idle, restoreActions(cfg, reasonFor(e.Kind))
		}
	}

	// Any other (state, event) combination is ignored.
	log.Printf("[core] ignoring event kind=%s in state=%s", e.Kind, state)
	return state, nil
}

// startActions returns the actions for entering Working from Idle on the first
// prompt of a work stretch: open the relax surface. Gated by RelaxEnabled.
// The browser is never closed afterwards, so subsequent prompts reuse it.
func startActions(cfg config.Config) []event.Action {
	if !cfg.RelaxEnabled {
		return nil
	}
	return []event.Action{{
		Kind:   event.OpenRelax,
		Reason: "session started",
	}}
}

// restoreActions returns the actions when a task finishes or the session ends:
// bring the IDE back to the foreground. The browser is intentionally left open
// so the user keeps their relax pages for the next prompt; CloseRelax is no
// longer emitted (cfg is unused but kept for signature stability).
func restoreActions(cfg config.Config, reason string) []event.Action {
	_ = cfg
	return []event.Action{{Kind: event.RestoreIDE, Reason: reason}}
}

func reasonFor(k event.EventKind) string {
	switch k {
	case event.TaskCompleted:
		return "task completed"
	case event.SessionEnded:
		return "session ended"
	default:
		return k.String()
	}
}
