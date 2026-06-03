package actuator

import (
	"log"

	"agentfocus/internal/event"
)

// ideWindowTitle is the substring matched against top-level window titles to
// locate the IDE window. VS Code titles look like
// "<file> - <folder> - Visual Studio Code".
const ideWindowTitle = "Visual Studio Code"

// IDEActuator brings the IDE window back to the foreground. It handles
// RestoreIDE. The actual window enumeration / SetForegroundWindow calls live in
// the platform-specific bringToForeground (see ide_windows.go); other platforms
// get a no-op stub so the package stays cross-compilable.
type IDEActuator struct{}

// NewIDE returns an IDEActuator.
func NewIDE() *IDEActuator { return &IDEActuator{} }

// Name identifies this actuator for dispatch and logging.
func (i *IDEActuator) Name() string { return "ide" }

// Do carries out RestoreIDE. Unsupported actions are ignored.
func (i *IDEActuator) Do(a event.Action) error {
	switch a.Kind {
	case event.RestoreIDE:
		i.Restore()
		return nil
	default:
		log.Printf("[actuator:ide] ignoring unsupported action=%s", a.Kind)
		return nil
	}
}

// Restore brings the IDE window to the foreground. It is exported so other
// modules (e.g. the UI approval popup's "jump to VSCode" button) can call it
// directly without constructing an Action. Failures are logged, never fatal.
func (i *IDEActuator) Restore() {
	if err := bringToForeground(ideWindowTitle); err != nil {
		log.Printf("[actuator:ide] could not restore IDE window: %v", err)
	}
}
