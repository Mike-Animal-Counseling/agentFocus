package watcher

import (
	"testing"

	"agentfocus/internal/event"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantOK     bool
		wantKind   event.EventKind
		wantReply  bool
		wantThread string
	}{
		{
			name:       "thread/started -> SessionStarted",
			raw:        `{"method":"thread/started","params":{"thread":{"id":"th-1"}}}`,
			wantOK:     true,
			wantKind:   event.SessionStarted,
			wantThread: "th-1",
		},
		{
			name:      "commandExecution requestApproval -> ApprovalRequested needsReply",
			raw:       `{"method":"item/commandExecution/requestApproval","id":7,"params":{"threadId":"th-2","turnId":"tu-2"}}`,
			wantOK:    true,
			wantKind:  event.ApprovalRequested,
			wantReply: true,
		},
		{
			name:      "fileChange requestApproval -> ApprovalRequested needsReply",
			raw:       `{"method":"item/fileChange/requestApproval","id":"abc","params":{"threadId":"th-3"}}`,
			wantOK:    true,
			wantKind:  event.ApprovalRequested,
			wantReply: true,
		},
		{
			name:     "thread/status/changed with waitingOnApproval -> ApprovalRequested",
			raw:      `{"method":"thread/status/changed","params":{"threadId":"th-4","status":{"activeFlags":["busy","waitingOnApproval"]}}}`,
			wantOK:   true,
			wantKind: event.ApprovalRequested,
		},
		{
			name:   "thread/status/changed without flag -> ignored",
			raw:    `{"method":"thread/status/changed","params":{"status":{"activeFlags":["busy"]}}}`,
			wantOK: false,
		},
		{
			name:     "turn/completed -> TaskCompleted",
			raw:      `{"method":"turn/completed","params":{"threadId":"th-5","turn":{"id":"tu-5"}}}`,
			wantOK:   true,
			wantKind: event.TaskCompleted,
		},
		{
			name:     "thread/closed -> SessionEnded",
			raw:      `{"method":"thread/closed","params":{"threadId":"th-6"}}`,
			wantOK:   true,
			wantKind: event.SessionEnded,
		},
		{
			name:   "unknown method -> ignored",
			raw:    `{"method":"item/agentMessage/delta","params":{"text":"hi"}}`,
			wantOK: false,
		},
		{
			name:   "response without method -> ignored",
			raw:    `{"id":0,"result":{"userAgent":"x"}}`,
			wantOK: false,
		},
		{
			name:   "random garbage -> false",
			raw:    `}{not json at all%%%`,
			wantOK: false,
		},
		{
			name:   "empty input -> false",
			raw:    ``,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, ok, needsReply, replyID := parseMessage([]byte(tc.raw))

			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return // nothing else to assert for ignored/garbage input
			}
			if ev.Kind != tc.wantKind {
				t.Errorf("Kind = %s, want %s", ev.Kind, tc.wantKind)
			}
			if needsReply != tc.wantReply {
				t.Errorf("needsReply = %v, want %v", needsReply, tc.wantReply)
			}
			if tc.wantReply && replyID == nil {
				t.Errorf("needsReply is true but replyID is nil")
			}
			if !tc.wantReply && replyID != nil {
				t.Errorf("replyID = %v, want nil for non-reply event", replyID)
			}
			if tc.wantThread != "" && ev.ThreadID != tc.wantThread {
				t.Errorf("ThreadID = %q, want %q", ev.ThreadID, tc.wantThread)
			}
			if ev.Timestamp.IsZero() {
				t.Errorf("Timestamp is zero, want it set")
			}
		})
	}
}

// TestParseMessageReplyIDPreserved verifies the original request id is echoed
// back verbatim, including string ids, so the decline reply routes correctly.
func TestParseMessageReplyIDPreserved(t *testing.T) {
	_, ok, needsReply, replyID := parseMessage(
		[]byte(`{"method":"item/commandExecution/requestApproval","id":"req-xyz","params":{}}`))
	if !ok || !needsReply {
		t.Fatalf("expected an approval request needing reply")
	}
	if replyID != "req-xyz" {
		t.Errorf("replyID = %v, want \"req-xyz\"", replyID)
	}
}
