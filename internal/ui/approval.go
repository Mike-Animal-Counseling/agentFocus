package ui

// Decision is the user's choice in the approval dialog.
type Decision string

const (
	// DecisionAllow tells Codex to run the command.
	DecisionAllow Decision = "allow"
	// DecisionDeny tells Codex to reject the command.
	DecisionDeny Decision = "deny"
	// DecisionSkip declines to decide; Codex falls back to its own prompt.
	DecisionSkip Decision = "skip"
)

// ApprovalRequest carries what the approval dialog needs to show.
type ApprovalRequest struct {
	// Command is the shell command Codex wants to run (from tool_input.command).
	Command string
}

// ShowApproval displays the approval dialog for req and returns the user's
// decision. It is implemented per-platform (see approval_windows.go); on
// non-Windows it returns DecisionSkip.
//
// It must be called on the dedicated UI goroutine (Win32 UI requirement).
func ShowApproval(req ApprovalRequest) Decision {
	return showApprovalDialog(req)
}
