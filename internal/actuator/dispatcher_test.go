package actuator

import (
	"errors"
	"testing"

	"agentfocus/internal/event"
)

// recordingActuator is a test double that records the actions it receives and
// can be told to fail, so we can assert dispatcher routing without real side
// effects.
type recordingActuator struct {
	name string
	got  []event.ActionKind
	fail bool
}

func (r *recordingActuator) Name() string { return r.name }

func (r *recordingActuator) Do(a event.Action) error {
	r.got = append(r.got, a.Kind)
	if r.fail {
		return errors.New("boom")
	}
	return nil
}

func TestDispatcher_RoutesByActionKind(t *testing.T) {
	browser := &recordingActuator{name: "browser"}
	ide := &recordingActuator{name: "ide"}
	uiAct := &recordingActuator{name: "ui"}
	d := NewDispatcher(browser, ide, uiAct)

	_ = d.Do(event.Action{Kind: event.OpenRelax})
	_ = d.Do(event.Action{Kind: event.CloseRelax})
	_ = d.Do(event.Action{Kind: event.RestoreIDE})
	_ = d.Do(event.Action{Kind: event.ShowApprovalPopup})

	if want := []event.ActionKind{event.OpenRelax, event.CloseRelax}; !equalKinds(browser.got, want) {
		t.Errorf("browser got %v, want %v", browser.got, want)
	}
	if want := []event.ActionKind{event.RestoreIDE}; !equalKinds(ide.got, want) {
		t.Errorf("ide got %v, want %v", ide.got, want)
	}
	if want := []event.ActionKind{event.ShowApprovalPopup}; !equalKinds(uiAct.got, want) {
		t.Errorf("ui got %v, want %v", uiAct.got, want)
	}
}

func TestDispatcher_UnroutedActionIsSkipped(t *testing.T) {
	browser := &recordingActuator{name: "browser"}
	d := NewDispatcher(browser)

	// An ActionKind with no route entry should be a no-op, not an error.
	const unrouted = event.ActionKind(9999)
	if err := d.Do(event.Action{Kind: unrouted}); err != nil {
		t.Errorf("Do returned error %v, want nil", err)
	}
	if len(browser.got) != 0 {
		t.Errorf("browser unexpectedly received %v", browser.got)
	}
}

func TestDispatcher_MissingActuatorIsSkipped(t *testing.T) {
	// RestoreIDE routes to "ide", but no ide actuator is registered.
	browser := &recordingActuator{name: "browser"}
	d := NewDispatcher(browser)

	if err := d.Do(event.Action{Kind: event.RestoreIDE}); err != nil {
		t.Errorf("Do returned error %v, want nil", err)
	}
	if len(browser.got) != 0 {
		t.Errorf("browser unexpectedly received %v", browser.got)
	}
}

func TestDispatcher_ActuatorFailureDoesNotPropagate(t *testing.T) {
	failing := &recordingActuator{name: "browser", fail: true}
	d := NewDispatcher(failing)

	// Even though the actuator returns an error, Do swallows it (logs only).
	if err := d.Do(event.Action{Kind: event.OpenRelax}); err != nil {
		t.Errorf("Do returned error %v, want nil (failures are logged only)", err)
	}
	// The action was still attempted.
	if want := []event.ActionKind{event.OpenRelax}; !equalKinds(failing.got, want) {
		t.Errorf("actuator got %v, want %v", failing.got, want)
	}
}

// equalKinds is a small slice comparison helper local to the test file.
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
