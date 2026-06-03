package watcher

import (
	"testing"

	"agentfocus/internal/event"
)

func TestParseHookPayload(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantOK     bool
		wantKind   event.EventKind
		wantThread string
		wantTurn   string
		wantDetail string
	}{
		{
			name:       "UserPromptSubmit -> SessionStarted",
			raw:        `{"session_id":"s1","turn_id":"t1","hook_event_name":"UserPromptSubmit"}`,
			wantOK:     true,
			wantKind:   event.SessionStarted,
			wantThread: "s1",
			wantTurn:   "t1",
		},
		{
			name:   "SessionStart no longer mapped -> ignored",
			raw:    `{"session_id":"s1","hook_event_name":"SessionStart","source":"startup"}`,
			wantOK: false,
		},
		{
			// PermissionRequest is handled by /approval, not the /hook pump, so
			// parseHookPayload must ignore it.
			name:   "PermissionRequest ignored on /hook path",
			raw:    `{"session_id":"s2","turn_id":"t2","hook_event_name":"PermissionRequest","tool_name":"Bash","tool_input":{"command":"mkdir test"}}`,
			wantOK: false,
		},
		{
			name:       "Stop -> TaskCompleted",
			raw:        `{"session_id":"s3","turn_id":"t3","hook_event_name":"Stop"}`,
			wantOK:     true,
			wantKind:   event.TaskCompleted,
			wantThread: "s3",
			wantTurn:   "t3",
		},
		{
			name:   "unknown hook event -> ignored",
			raw:    `{"session_id":"s4","hook_event_name":"PreToolUse"}`,
			wantOK: false,
		},
		{
			name:   "missing hook_event_name -> ignored",
			raw:    `{"session_id":"s5","tool_name":"Bash"}`,
			wantOK: false,
		},
		{
			name:   "random garbage -> false",
			raw:    `}{not json%%%`,
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
			ev, ok := parseHookPayload([]byte(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if ev.Kind != tc.wantKind {
				t.Errorf("Kind = %s, want %s", ev.Kind, tc.wantKind)
			}
			if ev.ThreadID != tc.wantThread {
				t.Errorf("ThreadID = %q, want %q", ev.ThreadID, tc.wantThread)
			}
			if tc.wantTurn != "" && ev.TurnID != tc.wantTurn {
				t.Errorf("TurnID = %q, want %q", ev.TurnID, tc.wantTurn)
			}
			if ev.Detail != tc.wantDetail {
				t.Errorf("Detail = %q, want %q", ev.Detail, tc.wantDetail)
			}
			if ev.Timestamp.IsZero() {
				t.Errorf("Timestamp is zero, want it set")
			}
		})
	}
}
