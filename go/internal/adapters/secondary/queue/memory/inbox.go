// Package memory is an in-process, bounded AlertQueue that drops the oldest
// alert when full (mirrors the Python AlertInbox).
package memory

import (
	"context"
	"sync"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// DefaultCapacity matches the Python inbox default.
const DefaultCapacity = 256

// Inbox is a bounded FIFO safe for concurrent producers and consumers.
type Inbox struct {
	mu       sync.Mutex
	items    []alert.Alert
	capacity int
	dropped  int
	notify   chan struct{} // closed and replaced whenever an item is added
}

var _ ports.AlertQueue = (*Inbox)(nil)

// NewInbox returns an inbox holding at most capacity alerts.
func NewInbox(capacity int) *Inbox {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Inbox{capacity: capacity, notify: make(chan struct{})}
}

// Enqueue implements ports.AlertQueue.
func (q *Inbox) Enqueue(_ context.Context, a alert.Alert) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	evicted := false
	if len(q.items) >= q.capacity {
		q.items = q.items[1:]
		q.dropped++
		evicted = true
	}
	q.items = append(q.items, a)
	close(q.notify)
	q.notify = make(chan struct{})
	return evicted, nil
}

// Dequeue implements ports.AlertQueue.
func (q *Inbox) Dequeue(ctx context.Context) (alert.Alert, error) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			a := q.items[0]
			q.items = q.items[1:]
			q.mu.Unlock()
			return a, nil
		}
		wait := q.notify
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return alert.Alert{}, ctx.Err()
		case <-wait:
		}
	}
}

// Depth implements ports.AlertQueue.
func (q *Inbox) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Dropped is the number of alerts evicted since creation.
func (q *Inbox) Dropped() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropped
}
