package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
)

func TestDequeueBlocksUntilEnqueueOrCancel(t *testing.T) {
	q := NewInbox(4)
	got := make(chan alert.Alert, 1)
	go func() {
		a, _ := q.Dequeue(context.Background())
		got <- a
	}()
	time.Sleep(10 * time.Millisecond)
	_, _ = q.Enqueue(context.Background(), alert.Alert{Text: "late"})
	select {
	case a := <-got:
		if a.Text != "late" {
			t.Fatalf("got %q", a.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked consumer was not woken")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := q.Dequeue(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}
