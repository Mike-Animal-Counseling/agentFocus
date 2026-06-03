package actuator

import (
	"log"

	"agentfocus/internal/event"
)

// logf is the package-wide logging shim, so individual actuators don't each
// import log directly. Kept tiny on purpose.
func logf(format string, args ...any) { log.Printf(format, args...) }

// routes maps each handled ActionKind to the Name() of the actuator that
// performs it. The UI popup actuator registers under "ui".
var routes = map[event.ActionKind]string{
	event.OpenRelax:         "browser",
	event.CloseRelax:        "browser",
	event.RestoreIDE:        "ide",
	event.ShowApprovalPopup: "ui",
}

// Dispatcher routes actions to the actuator whose Name() matches the action's
// route. A failure in one actuator is logged but never stops the others.
type Dispatcher struct {
	actuators []Actuator
	byName    map[string]Actuator
}

// NewDispatcher builds a Dispatcher over the given actuators, indexing them by
// Name() for routing.
func NewDispatcher(actuators ...Actuator) *Dispatcher {
	byName := make(map[string]Actuator, len(actuators))
	for _, a := range actuators {
		byName[a.Name()] = a
	}
	return &Dispatcher{actuators: actuators, byName: byName}
}

// Name identifies the dispatcher itself (it also satisfies Actuator so it can
// be dropped into the pipeline in place of a single actuator).
func (d *Dispatcher) Name() string { return "dispatcher" }

// Do routes a single action to the matching actuator. Unroutable or unmatched
// actions are logged as warnings; actuator errors are logged but not returned,
// so one failing action never blocks the rest of a batch.
func (d *Dispatcher) Do(a event.Action) error {
	name, ok := routes[a.Kind]
	if !ok {
		log.Printf("[dispatcher] no route for action=%s (reason=%q); skipping",
			a.Kind, a.Reason)
		return nil
	}
	act, ok := d.byName[name]
	if !ok {
		log.Printf("[dispatcher] no actuator named %q for action=%s; skipping",
			name, a.Kind)
		return nil
	}
	if err := act.Do(a); err != nil {
		log.Printf("[dispatcher] actuator %q failed on action=%s: %v",
			name, a.Kind, err)
	}
	return nil
}
