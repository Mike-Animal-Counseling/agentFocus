package core

import (
	"testing"

	"agentfocus/internal/config"
	"agentfocus/internal/event"
)

// kindsOf extracts the ActionKind sequence from a slice of actions for easy
// comparison.
func kindsOf(actions []event.Action) []event.ActionKind {
	ks := make([]event.ActionKind, len(actions))
	for i, a := range actions {
		ks[i] = a.Kind
	}
	return ks
}

func equalKinds(a, b []event.ActionKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ev(k event.EventKind) event.Event {
	return event.Event{Kind: k, ThreadID: "t", TurnID: "u"}
}

// bothOn is the default config with both surfaces enabled.
func bothOn() config.Config {
	return config.Config{RelaxEnabled: true, PopupEnabled: true}
}

func TestTransition_NormalPaths(t *testing.T) {
	tests := []struct {
		name      string
		state     State
		event     event.EventKind
		cfg       config.Config
		wantState State
		wantKinds []event.ActionKind
	}{
		{
			name:      "Idle + SessionStarted -> Working, OpenRelax",
			state:     Idle,
			event:     event.SessionStarted,
			cfg:       bothOn(),
			wantState: Working,
			wantKinds: []event.ActionKind{event.OpenRelax},
		},
		{
			name:      "Working + ApprovalRequested -> WaitingApproval, ShowApprovalPopup only (no CloseRelax)",
			state:     Working,
			event:     event.ApprovalRequested,
			cfg:       bothOn(),
			wantState: WaitingApproval,
			wantKinds: []event.ActionKind{event.ShowApprovalPopup},
		},
		{
			name:      "Working + TaskCompleted -> Idle, RestoreIDE only (browser left open)",
			state:     Working,
			event:     event.TaskCompleted,
			cfg:       bothOn(),
			wantState: Idle,
			wantKinds: []event.ActionKind{event.RestoreIDE},
		},
		{
			name:      "Working + SessionEnded -> Idle, RestoreIDE only",
			state:     Working,
			event:     event.SessionEnded,
			cfg:       bothOn(),
			wantState: Idle,
			wantKinds: []event.ActionKind{event.RestoreIDE},
		},
		{
			name:      "WaitingApproval + TaskCompleted -> Idle, RestoreIDE only",
			state:     WaitingApproval,
			event:     event.TaskCompleted,
			cfg:       bothOn(),
			wantState: Idle,
			wantKinds: []event.ActionKind{event.RestoreIDE},
		},
		{
			name:      "WaitingApproval + SessionEnded -> Idle, RestoreIDE only",
			state:     WaitingApproval,
			event:     event.SessionEnded,
			cfg:       bothOn(),
			wantState: Idle,
			wantKinds: []event.ActionKind{event.RestoreIDE},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotActions := transition(tc.state, ev(tc.event), tc.cfg)
			if gotState != tc.wantState {
				t.Errorf("state = %s, want %s", gotState, tc.wantState)
			}
			if !equalKinds(kindsOf(gotActions), tc.wantKinds) {
				t.Errorf("actions = %v, want %v", kindsOf(gotActions), tc.wantKinds)
			}
		})
	}
}

func TestTransition_WaitingApprovalIgnoresSecondApproval(t *testing.T) {
	gotState, gotActions := transition(WaitingApproval, ev(event.ApprovalRequested), bothOn())
	if gotState != WaitingApproval {
		t.Errorf("state = %s, want WaitingApproval", gotState)
	}
	if len(gotActions) != 0 {
		t.Errorf("actions = %v, want none (no duplicate popup)", kindsOf(gotActions))
	}
}

func TestTransition_IgnoredCombinations(t *testing.T) {
	// A representative set of (state, event) pairs that should be no-ops.
	cases := []struct {
		state State
		event event.EventKind
	}{
		{Idle, event.ApprovalRequested},
		{Idle, event.TaskCompleted},
		{Idle, event.SessionEnded},
	}
	for _, c := range cases {
		gotState, gotActions := transition(c.state, ev(c.event), bothOn())
		if gotState != c.state {
			t.Errorf("(%s,%s): state = %s, want unchanged %s", c.state, c.event, gotState, c.state)
		}
		if len(gotActions) != 0 {
			t.Errorf("(%s,%s): actions = %v, want none", c.state, c.event, kindsOf(gotActions))
		}
	}
}

func TestTransition_SessionStartedReusesBrowser(t *testing.T) {
	// A new SessionStarted while already active (Working or WaitingApproval)
	// must NOT reopen the browser: it stays in Working and emits nothing, so the
	// existing relax pages are reused.
	for _, from := range []State{Working, WaitingApproval} {
		gotState, gotActions := transition(from, ev(event.SessionStarted), bothOn())
		if gotState != Working {
			t.Errorf("from %s: state = %s, want Working", from, gotState)
		}
		if len(gotActions) != 0 {
			t.Errorf("from %s: actions = %v, want none (reuse browser)", from, kindsOf(gotActions))
		}
	}
}

func TestTransition_RelaxDisabled(t *testing.T) {
	cfg := config.Config{RelaxEnabled: false, PopupEnabled: true}

	// Idle + SessionStarted: no OpenRelax, still moves to Working.
	s, a := transition(Idle, ev(event.SessionStarted), cfg)
	if s != Working {
		t.Errorf("state = %s, want Working", s)
	}
	if len(a) != 0 {
		t.Errorf("actions = %v, want none (OpenRelax suppressed)", kindsOf(a))
	}

	// Working + ApprovalRequested: no CloseRelax, only ShowApprovalPopup.
	s, a = transition(Working, ev(event.ApprovalRequested), cfg)
	if s != WaitingApproval {
		t.Errorf("state = %s, want WaitingApproval", s)
	}
	if !equalKinds(kindsOf(a), []event.ActionKind{event.ShowApprovalPopup}) {
		t.Errorf("actions = %v, want [ShowApprovalPopup]", kindsOf(a))
	}

	// Working + TaskCompleted: no CloseRelax, RestoreIDE still fires.
	s, a = transition(Working, ev(event.TaskCompleted), cfg)
	if s != Idle {
		t.Errorf("state = %s, want Idle", s)
	}
	if !equalKinds(kindsOf(a), []event.ActionKind{event.RestoreIDE}) {
		t.Errorf("actions = %v, want [RestoreIDE]", kindsOf(a))
	}
}

func TestTransition_PopupDisabled(t *testing.T) {
	cfg := config.Config{RelaxEnabled: true, PopupEnabled: false}

	// Working + ApprovalRequested with popup disabled: nothing to emit (the
	// browser is never closed here, and the popup is off), but still advances.
	s, a := transition(Working, ev(event.ApprovalRequested), cfg)
	if s != WaitingApproval {
		t.Errorf("state = %s, want WaitingApproval", s)
	}
	if len(a) != 0 {
		t.Errorf("actions = %v, want none (popup disabled, no close)", kindsOf(a))
	}
}

// TestEngine_StatefulSequence drives the Engine through a full realistic
// sequence and asserts both the actions and the stateful behavior (e.g. a
// second SessionStarted being ignored because state advanced).
func TestEngine_StatefulSequence(t *testing.T) {
	eng := New(bothOn())

	// 1. SessionStarted -> OpenRelax
	if got := kindsOf(eng.Process(ev(event.SessionStarted))); !equalKinds(got, []event.ActionKind{event.OpenRelax}) {
		t.Fatalf("step1 actions = %v, want [OpenRelax]", got)
	}
	// 2. Second SessionStarted while Working -> reuse browser, no actions.
	if got := eng.Process(ev(event.SessionStarted)); len(got) != 0 {
		t.Fatalf("step2 actions = %v, want none (reuse browser)", kindsOf(got))
	}
	// 3. ApprovalRequested -> ShowApprovalPopup only (browser stays open).
	if got := kindsOf(eng.Process(ev(event.ApprovalRequested))); !equalKinds(got, []event.ActionKind{event.ShowApprovalPopup}) {
		t.Fatalf("step3 actions = %v, want [ShowApprovalPopup]", got)
	}
	// 4. Second ApprovalRequested while WaitingApproval -> ignored.
	if got := eng.Process(ev(event.ApprovalRequested)); len(got) != 0 {
		t.Fatalf("step4 actions = %v, want none (duplicate approval ignored)", kindsOf(got))
	}
	// 5. TaskCompleted -> RestoreIDE only, back to Idle (browser left open).
	if got := kindsOf(eng.Process(ev(event.TaskCompleted))); !equalKinds(got, []event.ActionKind{event.RestoreIDE}) {
		t.Fatalf("step5 actions = %v, want [RestoreIDE]", got)
	}
	// 6. Back in Idle: TaskCompleted is ignored.
	if got := eng.Process(ev(event.TaskCompleted)); len(got) != 0 {
		t.Fatalf("step6 actions = %v, want none (TaskCompleted in Idle ignored)", kindsOf(got))
	}
}
