//go:build windows

package actuator

import (
	"fmt"
	"log"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// All Windows window-management syscalls are confined to this file so they
// never leak into core or watcher.

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procShowWindow               = user32.NewProc("ShowWindow")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procIsIconic                 = user32.NewProc("IsIconic")
)

const swRestore = 9 // SW_RESTORE: restore a minimized window

// isMinimized reports whether hwnd is currently minimized (iconic).
func isMinimized(hwnd windows.HWND) bool {
	ret, _, _ := procIsIconic.Call(uintptr(hwnd))
	return ret != 0
}

// unminimizeOnly restores hwnd ONLY if it is minimized. It never touches a
// normal or maximized window, so a window the user maximized or resized keeps
// its exact size/state when we bring it to the foreground.
func unminimizeOnly(hwnd windows.HWND) {
	if isMinimized(hwnd) {
		_, _, _ = procShowWindow.Call(uintptr(hwnd), uintptr(swRestore))
	}
}

// bringToForeground finds the first visible top-level window whose title
// contains titleSubstr and pulls it to the foreground. It returns an error if
// no matching window is found.
func bringToForeground(titleSubstr string) error {
	return raiseWindowByTitle(titleSubstr, "ide")
}

// raiseWindowByTitle finds the first visible top-level window whose title
// contains titleSubstr and forces it to the foreground (working around Windows
// focus-stealing prevention). tag is used only for logging. Shared by the IDE
// and browser actuators. Defined per-platform; see ide_other.go for the stub.
func raiseWindowByTitle(titleSubstr, tag string) error {
	hwnd, err := findWindowByTitle(titleSubstr)
	if err != nil {
		return err
	}
	log.Printf("[actuator:%s] found %q window (hwnd=%#x)", tag, titleSubstr, hwnd)

	// Un-minimize only if minimized — never shrink a maximized/resized window.
	unminimizeOnly(hwnd)

	// Try a plain SetForegroundWindow first; it succeeds when our process
	// already owns the foreground (e.g. just launched). If the OS refuses it
	// (focus-stealing prevention), fall back to the AttachThreadInput trick.
	if ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd)); ret != 0 {
		log.Printf("[actuator:%s] brought %q to foreground (hwnd=%#x)", tag, titleSubstr, hwnd)
		return nil
	}

	if err := forceForeground(hwnd); err != nil {
		return err
	}
	log.Printf("[actuator:%s] forced %q to foreground via AttachThreadInput (hwnd=%#x)",
		tag, titleSubstr, hwnd)
	return nil
}

// forceForeground works around Windows focus-stealing prevention by attaching
// the calling thread's input queue to the current foreground window's thread,
// which lets SetForegroundWindow / BringWindowToTop take effect, then detaches.
func forceForeground(hwnd windows.HWND) error {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg == 0 {
		// No current foreground window: nothing to attach to. Try a bare raise.
		if ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd)); ret == 0 {
			return fmt.Errorf("SetForegroundWindow refused and no foreground window to attach to")
		}
		return nil
	}

	curThread := windows.GetCurrentThreadId()
	fgThread, _, _ := procGetWindowThreadProcessId.Call(fg, 0)
	tgtThread, _, _ := procGetWindowThreadProcessId.Call(uintptr(hwnd), 0)

	// Attach our input queue (and the target's) to the foreground thread's
	// queue so the OS treats the upcoming SetForegroundWindow as coming from
	// the active input thread. Detach again on every exit path.
	attached := attachInput(uintptr(curThread), fgThread)
	attachedTgt := attachInput(tgtThread, fgThread)
	defer func() {
		if attachedTgt {
			detachInput(tgtThread, fgThread)
		}
		if attached {
			detachInput(uintptr(curThread), fgThread)
		}
	}()

	// Un-minimize only if needed; do NOT SW_RESTORE a maximized window (that
	// would shrink it). BringWindowToTop + SetForegroundWindow raise it without
	// changing its size/state.
	unminimizeOnly(hwnd)
	_, _, _ = procBringWindowToTop.Call(uintptr(hwnd))
	ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	if ret == 0 {
		return fmt.Errorf("SetForegroundWindow still refused after AttachThreadInput")
	}
	return nil
}

// attachInput attaches thread `from` to thread `to`. It is a no-op (returns
// false) when the two are the same thread, since AttachThreadInput fails on
// self-attach.
func attachInput(from, to uintptr) bool {
	if from == to || from == 0 || to == 0 {
		return false
	}
	ret, _, _ := procAttachThreadInput.Call(from, to, 1) // 1 = TRUE (attach)
	return ret != 0
}

// detachInput reverses attachInput.
func detachInput(from, to uintptr) {
	if from == to || from == 0 || to == 0 {
		return
	}
	_, _, _ = procAttachThreadInput.Call(from, to, 0) // 0 = FALSE (detach)
}

// findWindowByTitle enumerates top-level windows and returns the handle of the
// first visible one whose title contains titleSubstr.
func findWindowByTitle(titleSubstr string) (windows.HWND, error) {
	var found windows.HWND
	want := strings.ToLower(titleSubstr)

	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if !isVisible(hwnd) {
			return 1 // continue enumeration
		}
		title := windowText(hwnd)
		if title != "" && strings.Contains(strings.ToLower(title), want) {
			found = hwnd
			return 0 // stop enumeration
		}
		return 1
	})

	// EnumWindows returns false both when the callback stops early and on real
	// errors, so we rely on `found` rather than its return value.
	_, _, _ = procEnumWindows.Call(cb, 0)

	if found == 0 {
		return 0, fmt.Errorf("no visible window with title containing %q", titleSubstr)
	}
	return found, nil
}

func isVisible(hwnd windows.HWND) bool {
	ret, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return ret != 0
}

// raiseWindowByPID finds a visible top-level window owned by process pid (or any
// of its descendant chrome processes via the same pid) and forces it to the
// foreground. tag is for logging. Used by the browser actuator to raise the
// dedicated relax Chrome window, which it identifies precisely by PID rather
// than title (so it never grabs the user's own browser windows).
// raiseWindowByPID brings the window owned by pid to the foreground WITHOUT
// changing its size or maximized/normal state — it only un-minimizes a
// minimized window. This preserves whatever size the user set: if they resized
// or kept it maximized, the window stays exactly as-is when raised. (First
// launch is sized via Chrome's --start-maximized, so no forced maximize here.)
func raiseWindowByPID(pid uint32, tag string) error {
	hwnd := findWindowByPID(pid)
	if hwnd == 0 {
		return fmt.Errorf("no visible window for pid %d", pid)
	}
	log.Printf("[actuator:%s] found window for pid=%d (hwnd=%#x)", tag, pid, hwnd)

	unminimizeOnly(hwnd)
	if ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd)); ret != 0 {
		log.Printf("[actuator:%s] brought pid=%d to foreground", tag, pid)
		return nil
	}
	if err := forceForeground(hwnd); err != nil {
		return err
	}
	log.Printf("[actuator:%s] forced pid=%d to foreground via AttachThreadInput", tag, pid)
	return nil
}

// findWindowByPID returns the first visible, titled top-level window whose
// owning process id equals pid.
func findWindowByPID(pid uint32) windows.HWND {
	var found windows.HWND
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if !isVisible(hwnd) {
			return 1
		}
		var wpid uint32
		_, _, _ = procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wpid)))
		if wpid == pid && windowText(hwnd) != "" {
			found = hwnd
			return 0
		}
		return 1
	})
	_, _, _ = procEnumWindows.Call(cb, 0)
	return found
}

// windowText returns the title text of hwnd, or "" if it has none.
func windowText(hwnd windows.HWND) string {
	n, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	length := int(n)
	if length <= 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	r, _, _ := procGetWindowTextW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
