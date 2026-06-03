//go:build !windows

package ui

import "log"

// showApprovalDialog is a no-op stub on non-Windows platforms so the package
// stays cross-compilable. The real implementation lives in approval_windows.go.
func showApprovalDialog(req ApprovalRequest) Decision {
	log.Printf("[ui] approval dialog not supported on this platform; command=%q", req.Command)
	return DecisionSkip
}

// showCountdown is a no-op stub on non-Windows platforms. Real implementation in
// countdown_windows.go.
func showCountdown(seconds int) {
	log.Printf("[ui] countdown not supported on this platform")
}
