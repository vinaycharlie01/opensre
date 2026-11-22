package agent

import (
	"fmt"
	"sort"
	"sync"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Registry holds the tools available to the agent, keyed by name. Tools are
// registered once at process boot by the composition root; lookups are then
// read-only, so the registry is safe to share across concurrent runs.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]ports.Tool
}

// NewRegistry returns a registry pre-populated with tools.
func NewRegistry(tools ...ports.Tool) (*Registry, error) {
	r := &Registry{tools: make(map[string]ports.Tool, len(tools))}
	for _, t := range tools {
		if err := r.Register(t); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Register adds a tool. Names must be unique.
func (r *Registry) Register(t ports.Tool) error {
	spec := t.Spec()
	if err := spec.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.tools[spec.Name]; dup {
		return fmt.Errorf("tool %q registered twice", spec.Name)
	}
	r.tools[spec.Name] = t
	return nil
}

// Lookup returns a tool by name.
func (r *Registry) Lookup(name string) (ports.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Specs returns every spec sorted by name. The order is deterministic so the
// tool list sent to the model is byte-stable across calls (prompt caching).
func (r *Registry) Specs() []tool.Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]tool.Spec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Spec())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Len is the number of registered tools.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
