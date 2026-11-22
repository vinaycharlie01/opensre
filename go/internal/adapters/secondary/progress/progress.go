// Package progress is an EventSink that writes loop progress lines to a
// terminal stream (stderr for the CLI) or to a structured logger.
package progress

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Writer prints one line per tool call.
type Writer struct {
	mu  sync.Mutex
	out io.Writer
}

var _ ports.EventSink = (*Writer)(nil)

// NewWriter returns a sink writing to out.
func NewWriter(out io.Writer) *Writer { return &Writer{out: out} }

// Emit implements ports.EventSink.
func (w *Writer) Emit(_ context.Context, e ports.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch e.Kind {
	case ports.EventToolStart:
		fmt.Fprintf(w.out, "  → %s\n", e.ToolName)
	case ports.EventToolEnd:
		status := "ok"
		if e.IsError {
			status = "error"
		}
		fmt.Fprintf(w.out, "  ← %s %s (%s)\n", e.ToolName, status, e.Duration.Round(1e6))
	case ports.EventHandoff:
		fmt.Fprintf(w.out, "  ! loop stopped: %s\n", e.Text)
	}
}

// Logger records events through slog at debug level.
type Logger struct{ Log *slog.Logger }

var _ ports.EventSink = Logger{}

// Emit implements ports.EventSink.
func (l Logger) Emit(ctx context.Context, e ports.Event) {
	l.Log.DebugContext(ctx, "agent event", "kind", e.Kind, "iteration", e.Iteration,
		"tool", e.ToolName, "is_error", e.IsError, "duration", e.Duration)
}
