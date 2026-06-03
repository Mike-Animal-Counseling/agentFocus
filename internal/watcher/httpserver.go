package watcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"agentfocus/internal/event"
)

// Approver decides a PermissionRequest. Implemented by the UI layer; it blocks
// until the user chooses and returns "allow", "deny", or "skip". It must be safe
// for concurrent calls (the UI layer serializes them).
type Approver interface {
	Approve(command string) string
}

// httpSource is a watcher.Source that receives Codex hook payloads over HTTP
// instead of connecting to the app-server. Codex hooks (configured in
// config.toml) POST their JSON payload to:
//   - /hook     for fire-and-forget events (UserPromptSubmit, Stop)
//   - /approval for PermissionRequest, which blocks until the user decides and
//     returns the decision so the hook script can hand it back to Codex.
type httpSource struct {
	addr     string
	out      chan event.Event
	server   *http.Server
	approver Approver
	ctx      context.Context // source lifetime; cancelled on Stop
	cancel   context.CancelFunc
	done     chan struct{} // closed when the server goroutine exits
}

// NewHTTPSource returns a Source that listens on localhost:<port>. approver may
// be nil (then /approval always returns skip).
func NewHTTPSource(port int, approver Approver) Source {
	return &httpSource{
		addr:     fmt.Sprintf("127.0.0.1:%d", port),
		out:      make(chan event.Event),
		approver: approver,
		done:     make(chan struct{}),
	}
}

func (h *httpSource) Events() <-chan event.Event { return h.out }

func (h *httpSource) Start(ctx context.Context) {
	ctx, h.cancel = context.WithCancel(ctx)
	h.ctx = ctx

	mux := http.NewServeMux()
	mux.HandleFunc("/hook", h.handleHook)
	mux.HandleFunc("/approval", h.handleApproval)
	h.server = &http.Server{Addr: h.addr, Handler: mux}

	go h.serve(ctx)
}

// serve runs the HTTP server and shuts it down when the context is cancelled.
// It closes the output channel and done channel when finished.
func (h *httpSource) serve(ctx context.Context) {
	defer close(h.out)
	defer close(h.done)

	// Shut the server down on cancellation.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = h.server.Shutdown(shutdownCtx)
	}()

	log.Printf("[watcher] hook HTTP server listening on http://%s/hook", h.addr)
	if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("[watcher] hook HTTP server error: %v", err)
	}
}

// handleHook parses one Codex hook POST and, if it maps to a known event,
// forwards it on the output channel.
func (h *httpSource) handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Append every received POST to a debug log so we can confirm hooks fire
	// and inspect their payloads. Best-effort; never blocks the response.
	logReceivedHook(body)

	// Always 200 the hook quickly: the hook script must not fail or block
	// Codex regardless of whether we recognized the payload.
	w.WriteHeader(http.StatusOK)

	ev, ok := parseHookPayload(body)
	if !ok {
		return // unknown or malformed payload: ignore
	}

	// Deliver to the consumer. Do NOT key this on r.Context(): the hook client
	// (a short-lived PowerShell POST) often closes the connection the instant
	// it gets the 200 above, cancelling r.Context() before we hand the event to
	// the pump — which would silently drop the event. Only give up if the whole
	// source is shutting down, or after a safety timeout.
	select {
	case h.out <- ev:
	case <-h.ctx.Done():
	case <-time.After(5 * time.Second):
		log.Printf("[watcher] dropped hook event kind=%s (no consumer)", ev.Kind)
	}
}

// handleApproval handles a PermissionRequest synchronously: it shows the
// approval dialog, blocks until the user decides, and returns the decision as
// JSON {"decision":"allow|deny|skip"}. The hook script translates that into the
// format Codex expects on the hook's stdout.
//
// Concurrency: multiple PermissionRequests may arrive at once (parallel tool
// calls). Each runs in its own HTTP handler goroutine and calls the Approver,
// which serializes the actual dialogs (one at a time) — so requests queue
// naturally rather than popping overlapping dialogs.
func (h *httpSource) handleApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	logReceivedHook(body)

	command := ""
	if p, ok := parseHookPayloadRaw(body); ok {
		command = p.ToolInput.Command
	}

	decision := "skip"
	if h.approver != nil {
		decision = h.approver.Approve(command)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"decision": decision})
}

func (h *httpSource) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	<-h.done
}

// parseHookPayloadRaw decodes the raw hook JSON into the struct without mapping
// to an event. Used by the approval handler to read tool_input.command.
func parseHookPayloadRaw(data []byte) (hookPayload, bool) {
	var p hookPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return hookPayload{}, false
	}
	return p, true
}

// logReceivedHook appends a timestamped record of a raw hook POST body to
// %APPDATA%\AgentFocus\hook_received.log. Best-effort: any error is logged and
// ignored so request handling is never affected.
func logReceivedHook(body []byte) {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		var err error
		if dir, err = os.UserConfigDir(); err != nil {
			return
		}
	}
	logPath := filepath.Join(dir, "AgentFocus", "hook_received.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		log.Printf("[watcher] hook log mkdir failed: %v", err)
		return
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[watcher] hook log open failed: %v", err)
		return
	}
	defer f.Close()

	stamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(f, "==== %s ====\n%s\n", stamp, body)
}

// --- Codex hook protocol knowledge is confined below this line --------------

// hookPayload is the JSON Codex sends to a command hook on stdin (which our
// hook script forwards to us verbatim). Only the fields we use are declared.
type hookPayload struct {
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id"`
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// parseHookPayload decodes a Codex hook JSON payload and maps it to an
// event.Event. It returns ok == false for malformed JSON or any hook event we
// do not handle. This is the ONLY place that knows the Codex hook protocol.
func parseHookPayload(data []byte) (event.Event, bool) {
	var p hookPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, false
	}

	ev := event.Event{
		ThreadID:  p.SessionID,
		TurnID:    p.TurnID,
		Timestamp: time.Now(),
	}

	switch p.HookEventName {
	case "UserPromptSubmit":
		// Turn-level "start": each submitted prompt opens the relax surface.
		ev.Kind = event.SessionStarted
		return ev, true
	case "Stop":
		ev.Kind = event.TaskCompleted
		return ev, true
	default:
		// PermissionRequest is handled synchronously by /approval, not here, so
		// it is intentionally not mapped on the fire-and-forget /hook path.
		return event.Event{}, false
	}
}
