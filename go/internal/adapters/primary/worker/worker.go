// Package worker is the background driving adapter that drains the alert
// queue into investigations.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Worker runs Concurrency investigations at a time.
type Worker struct {
	Queue        ports.AlertQueue
	Investigator ports.Investigator
	Concurrency  int
	Log          *slog.Logger
}

// Run blocks until ctx is cancelled and in-flight investigations finish.
func (w *Worker) Run(ctx context.Context) {
	n := max(w.Concurrency, 1)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.loop(ctx)
		}()
	}
	wg.Wait()
}

func (w *Worker) loop(ctx context.Context) {
	for {
		a, err := w.Queue.Dequeue(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				w.Log.ErrorContext(ctx, "dequeue failed", "err", err)
			}
			return
		}
		inv, err := w.Investigator.Investigate(ctx, a)
		if err != nil {
			w.Log.ErrorContext(ctx, "investigation not recorded", "err", err)
			continue
		}
		w.Log.InfoContext(ctx, "investigation finished",
			"id", inv.ID, "status", inv.Status, "outcome", inv.Outcome, "tool_calls", inv.ToolCalls)
	}
}
