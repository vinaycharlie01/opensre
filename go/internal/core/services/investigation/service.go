// Package investigation implements the Investigator and Assistant use cases:
// turn an alert (or a question) into an agent run and record the outcome.
package investigation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	domain "github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
	"github.com/Tracer-Cloud/opensre/go/internal/core/services/agent"
)

// SystemPrompt frames every investigation run.
const SystemPrompt = `You are OpenSRE, an on-call site reliability engineer investigating production alerts.
Use the available tools to gather evidence before drawing conclusions. Prefer tools whose
source matches the alert's source. Never guess at service names, queries, or identifiers:
discover them with tools first.

Finish with a concise report in Markdown with these sections:
## Summary
## Likely root cause
## Evidence
## Recommended next steps

State your confidence, and say plainly when the evidence is inconclusive.`

// Service implements ports.Investigator and ports.Assistant.
type Service struct {
	runner *agent.Runner
	repo   ports.InvestigationRepository
	clock  ports.Clock
	ids    ports.IDGenerator
	log    *slog.Logger
}

var (
	_ ports.Investigator = (*Service)(nil)
	_ ports.Assistant    = (*Service)(nil)
)

// New builds the service.
func New(runner *agent.Runner, repo ports.InvestigationRepository, clock ports.Clock, ids ports.IDGenerator, log *slog.Logger) *Service {
	return &Service{runner: runner, repo: repo, clock: clock, ids: ids, log: log}
}

// Investigate implements ports.Investigator.
func (s *Service) Investigate(ctx context.Context, a alert.Alert) (domain.Investigation, error) {
	inv := domain.New(domain.ID(s.ids.NewID()), a, s.clock.Now())
	if err := inv.Start(s.clock.Now()); err != nil {
		return *inv, err
	}
	if err := s.repo.Save(ctx, *inv); err != nil {
		return *inv, fmt.Errorf("save investigation: %w", err)
	}

	res, err := s.runner.Run(ctx, agent.Input{
		System:   SystemPrompt,
		Messages: []conversation.Message{conversation.UserText(alertPrompt(a))},
	})
	if err != nil {
		// Full detail stays in the server log; the stored failure is generic
		// because investigations are served over HTTP.
		s.log.ErrorContext(ctx, "investigation failed", "id", inv.ID, "err", err)
		_ = inv.Fail("the agent run failed; see server logs", s.clock.Now())
	} else {
		_ = inv.Complete(domain.Result{
			Outcome:   outcome(res.Stop),
			Report:    res.Text,
			Evidence:  res.Evidence.Cited(),
			ToolCalls: res.ToolCalls,
			Usage:     res.Usage,
		}, s.clock.Now())
	}
	if err := s.repo.Save(ctx, *inv); err != nil {
		return *inv, fmt.Errorf("save investigation: %w", err)
	}
	return *inv, nil
}

// Get implements ports.Investigator.
func (s *Service) Get(ctx context.Context, id domain.ID) (domain.Investigation, error) {
	return s.repo.Get(ctx, id)
}

// List implements ports.Investigator.
func (s *Service) List(ctx context.Context, limit int) ([]domain.Investigation, error) {
	return s.repo.List(ctx, limit)
}

// Ask implements ports.Assistant.
func (s *Service) Ask(ctx context.Context, question string) (ports.Answer, error) {
	res, err := s.runner.Run(ctx, agent.Input{
		System:   SystemPrompt,
		Messages: []conversation.Message{conversation.UserText(question)},
	})
	if err != nil {
		return ports.Answer{}, err
	}
	return ports.Answer{Text: res.Text, ToolCalls: res.ToolCalls}, nil
}

func alertPrompt(a alert.Alert) string {
	var b strings.Builder
	b.WriteString("Investigate this alert.\n\n")
	if a.Name != "" {
		fmt.Fprintf(&b, "Alert name: %s\n", a.Name)
	}
	if a.Severity != "" {
		fmt.Fprintf(&b, "Severity: %s\n", a.Severity)
	}
	if a.Source != "" {
		fmt.Fprintf(&b, "Source: %s\n", a.Source)
	}
	if !a.ReceivedAt.IsZero() {
		fmt.Fprintf(&b, "Received at: %s\n", a.ReceivedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	b.WriteString("\n<alert>\n")
	b.WriteString(a.Text)
	b.WriteString("\n</alert>")
	return b.String()
}

func outcome(s agent.Stop) domain.Outcome {
	switch s {
	case agent.StopAnswered:
		return domain.OutcomeAnswered
	case agent.StopIterationCap:
		return domain.OutcomeIterationCap
	case agent.StopStagnated:
		return domain.OutcomeStagnated
	default:
		return domain.OutcomeModelStopped
	}
}
