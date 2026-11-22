// Package scripted is a deterministic LLM adapter that replays canned
// responses. It backs tests and offline wiring checks; it never calls a network.
package scripted

import (
	"context"
	"errors"
	"sync"

	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// ErrExhausted is returned once every scripted response has been used.
var ErrExhausted = errors.New("scripted llm: no responses left")

// Step produces one response, optionally inspecting the request.
type Step func(req ports.LLMRequest) (ports.LLMResponse, error)

// Reply returns a Step that always yields resp.
func Reply(resp ports.LLMResponse) Step {
	return func(ports.LLMRequest) (ports.LLMResponse, error) { return resp, nil }
}

// LLM replays steps in order and records every request.
type LLM struct {
	mu       sync.Mutex
	steps    []Step
	Requests []ports.LLMRequest
}

var _ ports.LLM = (*LLM)(nil)

// New returns an LLM that will serve steps in order.
func New(steps ...Step) *LLM { return &LLM{steps: steps} }

// Complete implements ports.LLM.
func (l *LLM) Complete(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
	if err := ctx.Err(); err != nil {
		return ports.LLMResponse{}, err
	}
	l.mu.Lock()
	l.Requests = append(l.Requests, req)
	if len(l.steps) == 0 {
		l.mu.Unlock()
		return ports.LLMResponse{}, ErrExhausted
	}
	step := l.steps[0]
	l.steps = l.steps[1:]
	l.mu.Unlock()
	return step(req)
}
