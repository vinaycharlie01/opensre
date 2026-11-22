package ports

import (
	"context"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
)

// IntakeReceipt reports what happened to an accepted alert.
type IntakeReceipt struct {
	QueueDepth int
	Evicted    bool
}

// AlertIntake accepts alerts from external systems.
type AlertIntake interface {
	// Accept validates, normalizes, and queues an alert. A validation error
	// wraps one of the alert package's sentinel errors.
	Accept(ctx context.Context, a alert.Alert) (IntakeReceipt, error)
}

// Investigator runs and reads investigations.
type Investigator interface {
	// Investigate runs an investigation for the alert to completion and
	// returns its final state. The returned error is non-nil only when the
	// investigation could not be recorded; agent failures are captured as a
	// failed investigation.
	Investigate(ctx context.Context, a alert.Alert) (investigation.Investigation, error)
	// Get returns one investigation or ErrNotFound.
	Get(ctx context.Context, id investigation.ID) (investigation.Investigation, error)
	// List returns recent investigations, newest first.
	List(ctx context.Context, limit int) ([]investigation.Investigation, error)
}

// Answer is the reply to a free-form question.
type Answer struct {
	Text      string
	ToolCalls int
}

// Assistant answers free-form operator questions with the same tools.
type Assistant interface {
	// Ask runs one agent turn for the question.
	Ask(ctx context.Context, question string) (Answer, error)
}

// ToolCatalog lists the registered tools.
type ToolCatalog interface {
	// Specs returns registered tool specs sorted by name.
	Specs() []tool.Spec
}
