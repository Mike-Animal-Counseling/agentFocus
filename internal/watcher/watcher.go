// Package watcher connects to the Codex App Server and emits normalized
// event.Event values. This step defines only the interface plus a fake
// implementation used to exercise the pipeline.
package watcher

import (
	"context"

	"agentfocus/internal/event"
)

// Source produces a stream of events. Implementations connect to some upstream
// (e.g. the Codex App Server) and translate its messages into event.Event.
type Source interface {
	// Events returns the channel on which events are delivered. The channel
	// is owned by the Source and closed when the Source stops.
	Events() <-chan event.Event
	// Start begins producing events. It returns immediately; production runs
	// until the context is cancelled or Stop is called.
	Start(ctx context.Context)
	// Stop halts production and releases resources.
	Stop()
}
