//go:build windows

package ui

import (
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Countdown toast window via Win32 CreateWindowEx. Confined to this file.

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindowC     = user32.NewProc("DestroyWindow")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procFillRect           = user32.NewProc("FillRect")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procDrawTextW          = user32.NewProc("DrawTextW")
	procBeginPaint         = user32.NewProc("BeginPaint")
	procEndPaint           = user32.NewProc("EndPaint")
	procInvalidateRect     = user32.NewProc("InvalidateRect")
	procUpdateWindowC      = user32.NewProc("UpdateWindow")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procTranslateMessageC  = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn       = user32.NewProc("SetWindowRgn")
	procCreateFontW        = gdi32.NewProc("CreateFontW")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procLoadCursorW        = user32.NewProc("LoadCursorW")
	procSystemParametersIW = user32.NewProc("SystemParametersInfoW")
	procCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procGetModuleHandleW   = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
)

const (
	wsExToolWindow = 0x00000080
	wsExTopMost    = 0x00000008
	wsPopup        = 0x80000000
	swShowNA       = 8 // SW_SHOWNA: show without activating (don't steal focus)

	wmPaint   = 0x000F
	wmDestroy = 0x0002

	smCxScreen = 0
	smCyScreen = 1

	transparentBkMode = 1

	dtSingleline = 0x20
	dtCenter     = 0x01
	dtVcenter    = 0x04

	pmRemove = 0x0001

	toastW    = 500 // wide enough for the full countdown string at fontSize
	toastH    = 120
	marginX   = 28  // gap from the right edge (keeps it fully on-screen)
	bottomGap = 240 // gap from the bottom — lifts the toast up to ~1/4 screen
	fontSize  = 24  // large white text
)

// rectW mirrors Win32 RECT.
type rectW struct{ left, top, right, bottom int32 }

// paintStruct mirrors Win32 PAINTSTRUCT.
type paintStruct struct {
	hdc         windows.Handle
	fErase      int32
	rcPaint     rectW
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

// wndClassExW mirrors Win32 WNDCLASSEXW.
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

// msgW mirrors Win32 MSG.
type msgW struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// countdownState holds the text currently drawn, read by the window proc.
var countdownText = "Codex 已完成"

var countdownClassRegistered bool
var countdownClassName = mustUTF16("AgentFocusCountdown")

func wndProc(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

		// Dark background fill.
		brush, _, _ := procCreateSolidBrush.Call(0x002B2B2B) // BGR dark grey
		rc := rectW{0, 0, toastW, toastH}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
		procDeleteObject.Call(brush)

		// White centered text in a large font.
		procSetBkMode.Call(hdc, transparentBkMode)
		procSetTextColor.Call(hdc, 0x00FFFFFF) // white
		var oldFont uintptr
		if f := countdownFont(); f != 0 {
			oldFont, _, _ = procSelectObject.Call(hdc, f)
		}
		txt := mustUTF16(countdownText)
		const padX = 18 // horizontal breathing room so text isn't clipped
		drawRc := rectW{padX, 0, toastW - padX, toastH}
		procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(txt)), ^uintptr(0),
			uintptr(unsafe.Pointer(&drawRc)), dtSingleline|dtCenter|dtVcenter)
		if oldFont != 0 {
			procSelectObject.Call(hdc, oldFont)
		}

		procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmDestroy:
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// countdownFontHandle caches the large font; created on first paint.
var countdownFontHandle uintptr
var countdownFontTried bool

// countdownFont lazily creates a large UI font (Microsoft YaHei for clean
// Chinese rendering) and returns its HFONT, or 0 on failure.
func countdownFont() uintptr {
	if countdownFontTried {
		return countdownFontHandle
	}
	countdownFontTried = true
	face := mustUTF16("Microsoft YaHei")
	// CreateFontW(height, width, esc, orient, weight, italic, underline,
	//   strikeout, charset, outPrec, clipPrec, quality, pitchFamily, face)
	const fwSemibold = 600
	const defaultCharset = 1
	const cleartypeQuality = 5
	height := int32(-fontSize) // negative = character height in pixels
	h, _, _ := procCreateFontW.Call(
		uintptr(uint32(height)), 0, 0, 0, fwSemibold,
		0, 0, 0, defaultCharset, 0, 0, cleartypeQuality, 0,
		uintptr(unsafe.Pointer(face)),
	)
	countdownFontHandle = h
	return h
}

func registerCountdownClass() error {
	if countdownClassRegistered {
		return nil
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)
	// Standard arrow cursor (IDC_ARROW = 32512); without this the window shows
	// the busy/wait cursor on hover.
	const idcArrow = 32512
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))
	wc := wndClassExW{
		style:         0,
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     windows.Handle(hInst),
		hCursor:       windows.Handle(cursor),
		lpszClassName: countdownClassName,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		return fmt.Errorf("RegisterClassExW failed: %v", err)
	}
	countdownClassRegistered = true
	return nil
}

// showCountdown displays the bottom-right toast counting down `seconds` to 0,
// updating the number each second, then closes. Must run on the UI goroutine.
func showCountdown(seconds int) {
	if err := registerCountdownClass(); err != nil {
		log.Printf("[ui] countdown class: %v", err)
		return
	}

	// Use the work area (excludes the taskbar) so the toast never lands under
	// the taskbar or off the right edge.
	const spiGetWorkArea = 0x0030
	var wa rectW
	right := int32(0)
	bottom := int32(0)
	if ret, _, _ := procSystemParametersIW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0); ret != 0 {
		right = wa.right
		bottom = wa.bottom
	} else {
		// Fallback to full screen metrics.
		scrW, _, _ := procGetSystemMetrics.Call(smCxScreen)
		scrH, _, _ := procGetSystemMetrics.Call(smCyScreen)
		right = int32(scrW)
		bottom = int32(scrH)
	}
	x := right - toastW - marginX
	y := bottom - toastH - bottomGap
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	// NOTE: do NOT use WS_EX_LAYERED here. A layered window is invisible until
	// SetLayeredWindowAttributes/UpdateLayeredWindow is called; we paint via
	// WM_PAINT (GDI), so a plain top-most tool window is what we want.
	hwndRet, _, _ := procCreateWindowExW.Call(
		uintptr(wsExToolWindow|wsExTopMost),
		uintptr(unsafe.Pointer(countdownClassName)),
		uintptr(unsafe.Pointer(mustUTF16("AgentFocus"))),
		uintptr(wsPopup),
		uintptr(x), uintptr(y), uintptr(toastW), uintptr(toastH),
		0, 0, hInst, 0,
	)
	hwnd := windows.HWND(hwndRet)
	if hwnd == 0 {
		log.Printf("[ui] CreateWindowEx failed for countdown")
		return
	}
	defer procDestroyWindowC.Call(uintptr(hwnd))

	// Rounded corners.
	rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, toastW+1, toastH+1, 16, 16)
	procSetWindowRgn.Call(uintptr(hwnd), rgn, 1)

	// Show without stealing focus.
	procShowWindowUI(hwnd, swShowNA)

	for n := seconds; n >= 1; n-- {
		countdownText = fmt.Sprintf("Codex 已完成，%d 秒后跳回 VSCode", n)
		procInvalidateRect.Call(uintptr(hwnd), 0, 1)
		procUpdateWindowC.Call(uintptr(hwnd))
		pumpMessages(hwnd, 1*time.Second)
	}
}

// procShowWindowUI is ShowWindow but named to avoid clashing with the IDE
// actuator's usage in another package; here we resolve it locally.
var procShowWindowLocal = user32.NewProc("ShowWindow")

func procShowWindowUI(hwnd windows.HWND, cmd int) {
	procShowWindowLocal.Call(uintptr(hwnd), uintptr(cmd))
}

// pumpMessages processes window messages for roughly d, so the toast paints and
// stays responsive during the per-second wait.
func pumpMessages(hwnd windows.HWND, d time.Duration) {
	deadline := time.Now().Add(d)
	var msg msgW
	for time.Now().Before(deadline) {
		ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmRemove)
		if ret != 0 {
			procTranslateMessageC.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		} else {
			time.Sleep(15 * time.Millisecond)
		}
	}
}
