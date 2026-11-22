// Package memory is an in-process InvestigationRepository. Data is lost on
// restart; a SQL adapter implements the same port for durable storage.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Repository stores investigations in a map.
type Repository struct {
	mu   sync.RWMutex
	byID map[investigation.ID]investigation.Investigation
}

var _ ports.InvestigationRepository = (*Repository)(nil)

// New returns an empty repository.
func New() *Repository {
	return &Repository{byID: map[investigation.ID]investigation.Investigation{}}
}

// Save implements ports.InvestigationRepository.
func (r *Repository) Save(_ context.Context, inv investigation.Investigation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[inv.ID] = inv
	return nil
}

// Get implements ports.InvestigationRepository.
func (r *Repository) Get(_ context.Context, id investigation.ID) (investigation.Investigation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inv, ok := r.byID[id]
	if !ok {
		return investigation.Investigation{}, ports.ErrNotFound
	}
	return inv, nil
}

// List implements ports.InvestigationRepository.
func (r *Repository) List(_ context.Context, limit int) ([]investigation.Investigation, error) {
	r.mu.RLock()
	out := make([]investigation.Investigation, 0, len(r.byID))
	for _, inv := range r.byID {
		out = append(out, inv)
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
