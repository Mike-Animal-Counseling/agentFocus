package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os/exec"
	"time"

	"agentfocus/internal/event"
)

const (
	// restartDelay is how long to wait before respawning a crashed App Server.
	restartDelay = 5 * time.Second
	// maxRestarts is the number of respawn attempts before giving up.
	maxRestarts = 5
	// stdoutBufMax bounds a single JSON-RPC line (some payloads are large).
	stdoutBufMax = 1 << 20 // 1 MiB
)

// codexSource spawns `codex app-server`, performs the JSON-RPC handshake, and
// translates incoming messages into event.Event via parseMessage. It owns the
// subprocess lifecycle including crash recovery.
type codexSource struct {
	codexPath string

	out    chan event.Event
	cancel context.CancelFunc
	done   chan struct{} // closed when the supervise loop exits
}

// NewCodex returns a Source backed by a real `codex app-server` subprocess.
// codexPath is the absolute path to the codex executable (config.CodexPath);
// it is not resolved from PATH here.
func NewCodex(codexPath string) Source {
	return &codexSource{
		codexPath: codexPath,
		out:       make(chan event.Event),
		done:      make(chan struct{}),
	}
}

func (c *codexSource) Events() <-chan event.Event {
	return c.out
}

func (c *codexSource) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	go c.supervise(ctx)
}

func (c *codexSource) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	// Wait for the supervise loop to finish closing the output channel so a
	// caller that ranges over Events() sees a clean close.
	<-c.done
}

// supervise runs the App Server, restarting it after a crash up to maxRestarts
// times. It closes the output channel when it returns.
func (c *codexSource) supervise(ctx context.Context) {
	defer close(c.out)
	defer close(c.done)

	for attempt := 0; attempt <= maxRestarts; attempt++ {
		if ctx.Err() != nil {
			return
		}
		if attempt > 0 {
			log.Printf("[watcher] App Server restart %d/%d in %s",
				attempt, maxRestarts, restartDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(restartDelay):
			}
		}

		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			// Cancellation is a clean shutdown, not a crash.
			return
		}
		if err != nil {
			log.Printf("[watcher] App Server exited: %v", err)
		} else {
			log.Printf("[watcher] App Server exited unexpectedly (no error)")
		}
	}

	log.Printf("[watcher] App Server gave up after %d restarts; stopping", maxRestarts)
}

// runOnce starts one App Server process, drives the handshake, and pumps its
// stdout until the process exits or ctx is cancelled. It always Kills the
// child before returning.
func (c *codexSource) runOnce(ctx context.Context) error {
	cmd := exec.Command(c.codexPath, "app-server")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Surface the child's stderr in our own logs for diagnostics.
	cmd.Stderr = logWriter{prefix: "[codex-stderr] "}

	// Hide the child's console window (Windows): otherwise a stray console
	// titled like the working directory flashes up when spawning codex.
	hideChildWindow(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}
	log.Printf("[watcher] started codex app-server (pid=%d)", cmd.Process.Pid)

	// Guarantee the child is killed on every exit path (cancel or crash).
	procDone := make(chan struct{})
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		case <-procDone:
		}
	}()
	defer close(procDone)

	if err := handshake(stdin); err != nil {
		return err
	}

	return c.readLoop(ctx, stdout, stdin)
}

// handshake sends the initialize request and the initialized notification.
func handshake(w io.Writer) error {
	msgs := []any{
		map[string]any{
			"method": "initialize",
			"id":     0,
			"params": map[string]any{
				"clientInfo": map[string]any{
					"name":    "agentfocus",
					"title":   "AgentFocus",
					"version": "0.1.0",
				},
			},
		},
		map[string]any{
			"method": "initialized",
			"params": map[string]any{},
		},
	}
	for _, m := range msgs {
		if err := writeJSONLine(w, m); err != nil {
			return err
		}
	}
	return nil
}

// readLoop reads newline-delimited JSON from stdout, maps each line to an
// event, and replies to approval requests. It returns when stdout closes
// (process exit) or ctx is cancelled.
func (c *codexSource) readLoop(ctx context.Context, stdout io.Reader, stdin io.Writer) error {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), stdoutBufMax)

	for sc.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		ev, ok, needsReply, replyID := parseMessage(line)
		if !ok {
			// Unknown method or malformed JSON: skip, never panic.
			continue
		}

		if needsReply {
			if err := c.sendDecline(stdin, replyID); err != nil {
				log.Printf("[watcher] failed to send approval reply: %v", err)
			}
		}

		select {
		case c.out <- ev:
		case <-ctx.Done():
			return nil
		}
	}
	// Scanner stopped: either EOF (process exited) or a read error.
	return sc.Err()
}

// sendDecline replies to an approval request with decision "decline".
func (c *codexSource) sendDecline(w io.Writer, replyID any) error {
	return writeJSONLine(w, map[string]any{
		"id":     replyID,
		"result": map[string]any{"decision": "decline"},
	})
}

// writeJSONLine marshals v and writes it followed by a newline.
func writeJSONLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

// logWriter adapts io.Writer to the standard logger with a prefix, so child
// stderr lines land in our logs.
type logWriter struct{ prefix string }

func (lw logWriter) Write(p []byte) (int, error) {
	log.Printf("%s%s", lw.prefix, string(p))
	return len(p), nil
}
