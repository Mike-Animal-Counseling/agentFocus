//go:build windows

package ui

import (
	"encoding/binary"
	"log"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Approval dialog via Win32 TaskDialogIndirect (comctl32 v6 — requires the
// application manifest in cmd/agentfocus). Confined to this file.
//
// TASKDIALOGCONFIG and TASKDIALOG_BUTTON are declared #pragma pack(1) in the
// Windows headers (byte-packed, no padding). Go structs use natural alignment,
// which would insert padding after the uint32 fields and corrupt the layout
// (TaskDialogIndirect then returns E_INVALIDARG / 0x80070057). So we build the
// structures as raw byte buffers at the exact documented offsets.

var (
	comctl32               = windows.NewLazySystemDLL("comctl32.dll")
	procTaskDialogIndirect = comctl32.NewProc("TaskDialogIndirect")
)

// Custom button IDs (>= 100 to avoid colliding with common-button IDs).
const (
	idAllow = 101
	idDeny  = 102
	idSkip  = 103
)

const tdfAllowDialogCancellation = 0x0008

// putPtr / putU32 / putI32 write little-endian values at an offset.
func putPtr(b []byte, off int, p *uint16) {
	binary.LittleEndian.PutUint64(b[off:], uint64(uintptr(unsafe.Pointer(p))))
}
func putRaw(b []byte, off int, v uintptr) {
	binary.LittleEndian.PutUint64(b[off:], uint64(v))
}
func putU32(b []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(b[off:], v)
}

// buildButtons builds the packed TASKDIALOG_BUTTON array (12 bytes each:
// int32 id at 0, pointer at 4). Returns the buffer; texts must stay alive.
func buildButtons(ids []int32, texts []*uint16) []byte {
	const sz = 12
	buf := make([]byte, sz*len(ids))
	for i := range ids {
		binary.LittleEndian.PutUint32(buf[i*sz:], uint32(ids[i]))
		binary.LittleEndian.PutUint64(buf[i*sz+4:], uint64(uintptr(unsafe.Pointer(texts[i]))))
	}
	return buf
}

// showApprovalDialog shows the Allow/Deny/Skip task dialog and returns the
// user's choice. Must run on the UI goroutine.
func showApprovalDialog(req ApprovalRequest) Decision {
	// Keep all UTF-16 strings alive for the duration of the call.
	title := mustUTF16("AgentFocus")
	instruction := mustUTF16("Codex 正在等待授权")
	content := mustUTF16(buildContent(req.Command))
	allow := mustUTF16("Allow")
	deny := mustUTF16("Deny")
	skip := mustUTF16("Skip")

	ids := []int32{idAllow, idDeny, idSkip}
	texts := []*uint16{allow, deny, skip}
	btnBuf := buildButtons(ids, texts)

	// TASKDIALOGCONFIG packed layout, total 160 bytes.
	const cfgSize = 160
	cfg := make([]byte, cfgSize)
	putU32(cfg, 0, cfgSize) // cbSize
	// hwndParent (4), hInstance (12) left zero
	putU32(cfg, 20, tdfAllowDialogCancellation) // dwFlags
	// dwCommonButtons (24) zero
	putPtr(cfg, 28, title) // pszWindowTitle
	// pszMainIcon (36) zero
	putPtr(cfg, 44, instruction)                             // pszMainInstruction
	putPtr(cfg, 52, content)                                 // pszContent
	putU32(cfg, 60, uint32(len(ids)))                        // cButtons
	putRaw(cfg, 64, uintptr(unsafe.Pointer(&btnBuf[0])))     // pButtons
	binary.LittleEndian.PutUint32(cfg[72:], uint32(idAllow)) // nDefaultButton
	// remaining fields (radio buttons, verification, expanded, footer,
	// callback, cxWidth) left zero.

	var pressed int32
	ret, _, _ := procTaskDialogIndirect.Call(
		uintptr(unsafe.Pointer(&cfg[0])),
		uintptr(unsafe.Pointer(&pressed)),
		0, // pnRadioButton
		0, // pfVerificationFlagChecked
	)
	// Keep referenced buffers alive until the syscall has returned.
	runtime.KeepAlive(title)
	runtime.KeepAlive(instruction)
	runtime.KeepAlive(content)
	runtime.KeepAlive(allow)
	runtime.KeepAlive(deny)
	runtime.KeepAlive(skip)
	runtime.KeepAlive(btnBuf)

	if ret != 0 { // S_OK == 0
		log.Printf("[ui] TaskDialogIndirect failed (hr=%#x); defaulting to skip", ret)
		return DecisionSkip
	}
	switch pressed {
	case idAllow:
		return DecisionAllow
	case idDeny:
		return DecisionDeny
	default:
		return DecisionSkip
	}
}

// buildContent renders the dialog body showing the command, or a fallback.
func buildContent(command string) string {
	if command == "" {
		return "Codex 想执行一个需要授权的操作。\n\nAllow=允许  Deny=拒绝  Skip=交给 Codex 自行决定"
	}
	return "Codex 想执行命令：\n\n" + command +
		"\n\nAllow=允许  Deny=拒绝  Skip=交给 Codex 自行决定"
}

func mustUTF16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		p, _ = windows.UTF16PtrFromString("")
	}
	return p
}
