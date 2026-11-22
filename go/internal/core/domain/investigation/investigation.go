// Package investigation models one alert investigation and its lifecycle.
package investigation

import (
	"errors"
	"fmt"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/evidence"
)

// ID identifies an investigation.
type ID string

// Status is the lifecycle state. Transitions: pending → running → completed|failed.
type Status string

// Lifecycle states.
const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Outcome says how the agent loop ended for a completed investigation.
type Outcome string

// Outcomes. Anything other than OutcomeAnswered is a partial result.
const (
	OutcomeAnswered     Outcome = "answered"
	OutcomeIterationCap Outcome = "iteration_cap"
	OutcomeStagnated    Outcome = "stagnated"
	OutcomeModelStopped Outcome = "model_stopped"
)

// ErrInvalidTransition is returned for an illegal state change.
var ErrInvalidTransition = errors.New("invalid investigation state transition")

// Investigation is the aggregate root.
type Investigation struct {
	ID         ID                 `json:"id"`
	Alert      alert.Alert        `json:"alert"`
	Status     Status             `json:"status"`
	Outcome    Outcome            `json:"outcome,omitempty"`
	Report     string             `json:"report,omitempty"`
	Evidence   []evidence.Cited   `json:"evidence,omitempty"`
	ToolCalls  int                `json:"tool_calls"`
	Usage      conversation.Usage `json:"usage"`
	Failure    string             `json:"failure,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	StartedAt  *time.Time         `json:"started_at,omitempty"`
	FinishedAt *time.Time         `json:"finished_at,omitempty"`
}

// New creates a pending investigation.
func New(id ID, a alert.Alert, now time.Time) *Investigation {
	return &Investigation{ID: id, Alert: a, Status: StatusPending, CreatedAt: now}
}

// Start moves pending → running.
func (i *Investigation) Start(now time.Time) error {
	if i.Status != StatusPending {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, i.Status, StatusRunning)
	}
	i.Status = StatusRunning
	i.StartedAt = &now
	return nil
}

// Result is what a finished agent run contributes.
type Result struct {
	Outcome   Outcome
	Report    string
	Evidence  []evidence.Cited
	ToolCalls int
	Usage     conversation.Usage
}

// Complete moves running → completed.
func (i *Investigation) Complete(r Result, now time.Time) error {
	if i.Status != StatusRunning {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, i.Status, StatusCompleted)
	}
	i.Status = StatusCompleted
	i.Outcome = r.Outcome
	i.Report = r.Report
	i.Evidence = r.Evidence
	i.ToolCalls = r.ToolCalls
	i.Usage = r.Usage
	i.FinishedAt = &now
	return nil
}

// Fail moves running → failed. reason must already be safe to show externally.
func (i *Investigation) Fail(reason string, now time.Time) error {
	if i.Status != StatusRunning {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, i.Status, StatusFailed)
	}
	i.Status = StatusFailed
	i.Failure = reason
	i.FinishedAt = &now
	return nil
}
