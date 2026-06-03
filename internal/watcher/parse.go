package watcher

import (
	"encoding/json"
	"slices"
	"time"

	"agentfocus/internal/event"
)

// This file is the ONLY place in the codebase that knows the Codex App Server
// JSON-RPC protocol. Everything else deals in event.Event. If the protocol
// changes, it changes here.

// rpcEnvelope is the minimal shape shared by every JSON-RPC message we read
// from the App Server. id is kept as a raw value so we can echo it back
// verbatim when a message requires a reply.
type rpcEnvelope struct {
	Method string `json:"method"`
	ID     any    `json:"id"`
	// Params carries the method-specific payload; decoded lazily.
	Params json.RawMessage `json:"params"`
}

// rpcParams covers the union of fields we care about across the handful of
// methods we map. Missing fields simply stay zero.
type rpcParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	// thread/started carries the thread object instead of a flat threadId.
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
	// turn/started and turn/completed carry the turn object.
	Turn struct {
		ID string `json:"id"`
	} `json:"turn"`
	// thread/status/changed carries a status object whose activeFlags signal
	// transient states such as waiting on an approval.
	Status struct {
		ActiveFlags []string `json:"activeFlags"`
	} `json:"status"`
}

// approvalMethods are server->client requests that block Codex until we reply.
var approvalMethods = map[string]bool{
	"item/commandExecution/requestApproval": true,
	"item/fileChange/requestApproval":       true,
}

// parseMessage decodes one raw JSON-RPC line and, if it maps to a known event,
// returns it. The return tuple is:
//
//	ev         the decoded event (valid only when ok is true)
//	ok         whether the message mapped to a known event
//	needsReply whether the sender expects a reply (approval requests do)
//	replyID    the original request id to echo back in the reply
//
// It never panics: malformed or unknown input yields ok == false.
func parseMessage(data []byte) (ev event.Event, ok bool, needsReply bool, replyID any) {
	var env rpcEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return event.Event{}, false, false, nil
	}
	if env.Method == "" {
		return event.Event{}, false, false, nil
	}

	var p rpcParams
	if len(env.Params) > 0 {
		// Ignore params decode errors: a malformed params block just leaves
		// the optional fields empty; the method alone may still be mappable.
		_ = json.Unmarshal(env.Params, &p)
	}

	base := event.Event{
		ThreadID:  firstNonEmpty(p.ThreadID, p.Thread.ID),
		TurnID:    firstNonEmpty(p.TurnID, p.Turn.ID),
		Timestamp: time.Now(),
	}

	switch {
	case env.Method == "thread/started":
		base.Kind = event.SessionStarted
		return base, true, false, nil

	case env.Method == "thread/status/changed":
		if slices.Contains(p.Status.ActiveFlags, "waitingOnApproval") {
			base.Kind = event.ApprovalRequested
			return base, true, false, nil
		}
		return event.Event{}, false, false, nil

	case env.Method == "turn/completed":
		base.Kind = event.TaskCompleted
		return base, true, false, nil

	case env.Method == "thread/closed":
		base.Kind = event.SessionEnded
		return base, true, false, nil

	case approvalMethods[env.Method]:
		base.Kind = event.ApprovalRequested
		return base, true, true, env.ID

	default:
		return event.Event{}, false, false, nil
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
