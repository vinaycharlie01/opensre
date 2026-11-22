package httpapi_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/primary/httpapi"
	queuemem "github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/queue/memory"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/system"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
	"github.com/Tracer-Cloud/opensre/go/internal/core/services/intake"
)

type failingAssistant struct{}

func (failingAssistant) Ask(context.Context, string) (ports.Answer, error) {
	return ports.Answer{}, errors.New("anthropic 529 overloaded req_abc123 model internals")
}

type emptyInvestigator struct{}

func (emptyInvestigator) Investigate(context.Context, alert.Alert) (investigation.Investigation, error) {
	return investigation.Investigation{}, nil
}
func (emptyInvestigator) Get(context.Context, investigation.ID) (investigation.Investigation, error) {
	return investigation.Investigation{}, ports.ErrNotFound
}
func (emptyInvestigator) List(context.Context, int) ([]investigation.Investigation, error) {
	return nil, nil
}

func handler(tokens ...string) (http.Handler, *queuemem.Inbox) {
	q := queuemem.NewInbox(2)
	return httpapi.New(httpapi.Deps{
		Intake: intake.New(q, system.Clock{}), Investigator: emptyInvestigator{},
		Assistant: failingAssistant{}, Tokens: tokens, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}), q
}

func do(h http.Handler, remote, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.RemoteAddr = remote
	req.Host = "127.0.0.1:8080"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTrustBoundary(t *testing.T) {
	const body = `{"text":"disk full"}`
	noToken, _ := handler()
	withToken, _ := handler("s3cret")
	cases := []struct {
		name   string
		h      http.Handler
		remote string
		hdr    map[string]string
		want   int
	}{
		{"loopback curl", noToken, "127.0.0.1:5000", nil, http.StatusAccepted},
		{"remote without token configured", noToken, "10.0.0.8:5000", nil, http.StatusForbidden},
		{"browser page on loopback", noToken, "127.0.0.1:5000", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"wrong token", withToken, "10.0.0.8:5000", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"loopback still needs token when configured", withToken, "127.0.0.1:5000", nil, http.StatusUnauthorized},
		{"right token", withToken, "10.0.0.8:5000", map[string]string{"Authorization": "Bearer s3cret"}, http.StatusAccepted},
	}
	for _, c := range cases {
		if rec := do(c.h, c.remote, "/alerts", body, c.hdr); rec.Code != c.want {
			t.Errorf("%s: got %d want %d (%s)", c.name, rec.Code, c.want, rec.Body)
		}
	}
}

func TestAlertIntakeValidationAndEviction(t *testing.T) {
	h, q := handler()
	local := "127.0.0.1:1"
	for _, bad := range []string{`not json`, `{"text":""}`, `{"text":"x","unexpected":1}`, `[1]`} {
		if rec := do(h, local, "/alerts", bad, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: got %d", bad, rec.Code)
		}
	}
	huge := `{"text":"` + strings.Repeat("a", httpapi.MaxBodyBytes) + `"}`
	if rec := do(h, local, "/alerts", huge, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body: got %d", rec.Code)
	}
	for i := 0; i < 3; i++ {
		rec := do(h, local, "/alerts", `{"text":"a","source":" Datadog "}`, nil)
		if i == 2 && !strings.Contains(rec.Body.String(), "oldest alert dropped") {
			t.Errorf("third alert should report eviction: %s", rec.Body)
		}
	}
	if q.Depth() != 2 || q.Dropped() != 1 {
		t.Fatalf("depth=%d dropped=%d", q.Depth(), q.Dropped())
	}
	a, _ := q.Dequeue(context.Background())
	if a.Source != "datadog" || a.ReceivedAt.IsZero() {
		t.Fatalf("alert not normalized: %+v", a)
	}
}

func TestInternalErrorsAreRedacted(t *testing.T) {
	h, _ := handler()
	rec := do(h, "127.0.0.1:1", "/v1/prompt", `{"question":"why?"}`, nil)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "req_abc123") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
