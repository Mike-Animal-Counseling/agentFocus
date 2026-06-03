package watcher

import (
	"context"
	"time"

	"agentfocus/internal/event"
)

// fakeSource emits a synthetic event every 5 seconds, cycling through the
// event kinds. It is used to exercise the pipeline before the real App Server
// watcher exists.
type fakeSource struct {
	out    chan event.Event
	cancel context.CancelFunc
}

// NewFake returns a Source that emits a fake event every 5 seconds.
func NewFake() Source {
	return &fakeSource{
		out: make(chan event.Event),
	}
}

func (f *fakeSource) Events() <-chan event.Event {
	return f.out
}

func (f *fakeSource) Start(ctx context.Context) {
	ctx, f.cancel = context.WithCancel(ctx)
	go f.loop(ctx)
}

func (f *fakeSource) loop(ctx context.Context) {
	defer close(f.out)

	kinds := []event.EventKind{
		event.SessionStarted,
		event.ApprovalRequested,
		event.TaskCompleted,
		event.SessionEnded,
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e := event.Event{
				Kind:      kinds[i%len(kinds)],
				ThreadID:  "fake-thread",
				TurnID:    "fake-turn",
				Timestamp: time.Now(),
			}
			i++
			select {
			case f.out <- e:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (f *fakeSource) Stop() {
	if f.cancel != nil {
		f.cancel()
	}
}
