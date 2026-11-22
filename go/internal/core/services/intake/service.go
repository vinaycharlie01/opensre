// Package intake implements the AlertIntake use case.
package intake

import (
	"context"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Service validates incoming alerts and queues them for investigation.
type Service struct {
	queue ports.AlertQueue
	clock ports.Clock
}

var _ ports.AlertIntake = (*Service)(nil)

// New builds the service.
func New(queue ports.AlertQueue, clock ports.Clock) *Service {
	return &Service{queue: queue, clock: clock}
}

// Accept implements ports.AlertIntake.
func (s *Service) Accept(ctx context.Context, a alert.Alert) (ports.IntakeReceipt, error) {
	a = a.Normalize()
	if err := a.Validate(); err != nil {
		return ports.IntakeReceipt{}, err
	}
	if a.ReceivedAt.IsZero() {
		a.ReceivedAt = s.clock.Now()
	}
	evicted, err := s.queue.Enqueue(ctx, a)
	if err != nil {
		return ports.IntakeReceipt{}, err
	}
	return ports.IntakeReceipt{QueueDepth: s.queue.Depth(), Evicted: evicted}, nil
}
