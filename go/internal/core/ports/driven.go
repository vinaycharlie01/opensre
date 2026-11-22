// Package ports declares the hexagon's boundaries. Driven (secondary) ports
// are interfaces the core calls and adapters implement; driving (primary)
// ports are the use cases adapters such as HTTP or the CLI call into.
//
// Ports depend only on the domain packages and the standard library.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
)

// LLMRequest is one provider-neutral model call.
type LLMRequest struct {
	System    string
	Messages  []conversation.Message
	Tools     []tool.Spec
	MaxTokens int
}

// LLMResponse is the model's reply to one LLMRequest.
type LLMResponse struct {
	Text       string
	ToolCalls  []conversation.ToolCall
	StopReason conversation.StopReason
	Usage      conversation.Usage
	// Native is copied onto the assistant message so the adapter can replay
	// the turn verbatim on the next call.
	Native any
}

// ErrContextOverflow is returned by an LLM adapter when the provider rejected
// the request as too large, so the loop can shrink its budget and retry.
var ErrContextOverflow = errors.New("llm context window exceeded")

// LLM is a hosted language model.
type LLM interface {
	// Complete runs one model turn. Implementations must honour ctx
	// cancellation and wrap provider "prompt too long" errors in
	// ErrContextOverflow.
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

// Tool is an agent-callable capability implemented by an integration adapter.
type Tool interface {
	// Spec returns the tool's static metadata; it must not change after
	// registration.
	Spec() tool.Spec
	// Invoke runs the tool with arguments already validated against
	// Spec().InputSchema. A returned error is reported to the model as a
	// failed observation, not as a run failure.
	Invoke(ctx context.Context, args map[string]any) (tool.Output, error)
}

// AlertQueue buffers alerts between intake and investigation.
type AlertQueue interface {
	// Enqueue adds an alert. When full, the oldest alert is evicted and
	// evicted is true; the new alert is always accepted.
	Enqueue(ctx context.Context, a alert.Alert) (evicted bool, err error)
	// Dequeue blocks until an alert is available or ctx is done.
	Dequeue(ctx context.Context) (alert.Alert, error)
	// Depth is the number of queued alerts.
	Depth() int
}

// ErrNotFound is returned by repositories for an unknown id.
var ErrNotFound = errors.New("not found")

// InvestigationRepository persists investigations.
type InvestigationRepository interface {
	// Save inserts or replaces an investigation by id.
	Save(ctx context.Context, inv investigation.Investigation) error
	// Get returns the investigation or ErrNotFound.
	Get(ctx context.Context, id investigation.ID) (investigation.Investigation, error)
	// List returns up to limit investigations, newest first.
	List(ctx context.Context, limit int) ([]investigation.Investigation, error)
}

// Clock supplies the current time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

// IDGenerator mints unique identifiers.
type IDGenerator interface {
	// NewID returns a fresh unique id.
	NewID() string
}

// Event is a progress notification from the agent loop.
type Event struct {
	Kind      EventKind
	Iteration int
	ToolName  string
	IsError   bool
	Text      string
	Duration  time.Duration
}

// EventKind enumerates loop events.
type EventKind string

// Loop event kinds.
const (
	EventTurnStart EventKind = "turn_start"
	EventToolStart EventKind = "tool_start"
	EventToolEnd   EventKind = "tool_end"
	EventAnswer    EventKind = "answer"
	EventHandoff   EventKind = "handoff"
)

// EventSink observes the agent loop (terminal progress, tracing, metrics).
type EventSink interface {
	// Emit records one event. It must not block the loop for long.
	Emit(ctx context.Context, e Event)
}
