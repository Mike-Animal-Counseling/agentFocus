package ui

// Manager owns a single dedicated goroutine on which all Win32 UI runs (dialogs
// and the countdown window). Win32 windows are thread-affine and the modal
// dialogs serialize naturally, so funnelling every UI call through one goroutine
// keeps things correct and avoids concurrent dialogs fighting for focus.
//
// Requests arrive from arbitrary goroutines (HTTP handlers, the dispatcher) and
// are queued; the UI goroutine processes them one at a time.
type Manager struct {
	reqs chan uiRequest
}

// uiRequest is one unit of work for the UI goroutine.
type uiRequest struct {
	approval  *ApprovalRequest // non-nil -> show approval dialog
	resp      chan Decision    // approval result delivered here
	countdown int              // >0 -> show countdown for this many seconds
	done      chan struct{}    // closed when a countdown finishes
}

// NewManager creates a Manager. Call Run on a dedicated goroutine to start it.
func NewManager() *Manager {
	return &Manager{reqs: make(chan uiRequest, 16)}
}

// Run processes UI requests until stop is closed. It MUST be the only caller of
// the Win32 UI functions, and should run on its own goroutine for the lifetime
// of the program (ideally one locked to an OS thread — see cmd wiring).
func (m *Manager) Run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case r := <-m.reqs:
			switch {
			case r.approval != nil:
				d := showApprovalDialog(*r.approval)
				r.resp <- d
			case r.countdown > 0:
				showCountdown(r.countdown)
				close(r.done)
			}
		}
	}
}

// RequestApproval shows the approval dialog (on the UI goroutine) and blocks the
// caller until the user decides, returning the decision. Safe to call from any
// goroutine, including concurrently — requests queue and run one at a time.
func (m *Manager) RequestApproval(req ApprovalRequest) Decision {
	resp := make(chan Decision, 1)
	m.reqs <- uiRequest{approval: &req, resp: resp}
	return <-resp
}

// Countdown shows the "completing, jumping back in N seconds" window and blocks
// until it finishes. Safe to call from any goroutine.
func (m *Manager) Countdown(seconds int) {
	done := make(chan struct{})
	m.reqs <- uiRequest{countdown: seconds, done: done}
	<-done
}

// Approve implements watcher.Approver: shows the approval dialog for command and
// returns the decision as "allow", "deny", or "skip".
func (m *Manager) Approve(command string) string {
	return string(m.RequestApproval(ApprovalRequest{Command: command}))
}
