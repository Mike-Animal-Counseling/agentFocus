package actuator

import (
	"log"

	"agentfocus/internal/event"
)

// fakeActuator logs each action instead of performing a real side effect.
type fakeActuator struct{}

// NewFake returns an Actuator that logs actions.
func NewFake() Actuator {
	return &fakeActuator{}
}

func (f *fakeActuator) Do(a event.Action) error {
	log.Printf("[actuator:%s] do action=%s reason=%q", f.Name(), a.Kind, a.Reason)
	return nil
}

func (f *fakeActuator) Name() string {
	return "fake"
}
