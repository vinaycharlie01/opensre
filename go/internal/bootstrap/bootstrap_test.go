package bootstrap_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/primary/worker"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/llm/scripted"
	"github.com/Tracer-Cloud/opensre/go/internal/bootstrap"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
	"github.com/Tracer-Cloud/opensre/go/internal/platform/config"
)

// redirect sends every outbound request to target, so the real Datadog client
// built by bootstrap talks to a local fake.
type redirect struct{ target *url.URL }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = r.target.Scheme, r.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// TestAlertToReport drives the full hexagon: HTTP intake → queue → worker →
// investigation service → agent loop → Datadog adapter → repository → HTTP read.
func TestAlertToReport(t *testing.T) {
	dd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"attributes":{"message":"OOMKilled pod checkout-7f9c","status":"error"}}]}`))
	}))
	defer dd.Close()
	target, _ := url.Parse(dd.URL)

	llm := scripted.New(
		func(req ports.LLMRequest) (ports.LLMResponse, error) {
			if len(req.Tools) != 3 || !strings.Contains(req.Messages[0].Text, "Source: datadog") {
				t.Errorf("unexpected first request: %d tools, %q", len(req.Tools), req.Messages[0].Text)
			}
			return ports.LLMResponse{StopReason: conversation.StopToolUse, ToolCalls: []conversation.ToolCall{{
				ID: "c1", Name: "query_datadog_logs", Input: json.RawMessage(`{"query":"service:checkout status:error"}`),
			}}}, nil
		},
		scripted.Reply(ports.LLMResponse{Text: "## Summary\nCheckout pods OOMKilled [E1].", StopReason: conversation.StopEndTurn}),
	)
	cfg, err := config.Load(func(k string) string {
		return map[string]string{"DD_API_KEY": "k", "DD_APP_KEY": "a"}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := bootstrap.Build(cfg, log, bootstrap.Overrides{LLM: llm, HTTPClient: &http.Client{Transport: redirect{target}}})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go (&worker.Worker{Queue: app.Queue, Investigator: app.Service, Concurrency: 1, Log: log}).Run(ctx)

	api := httptest.NewServer(app.Handler)
	defer api.Close()
	resp, err := http.Post(api.URL+"/alerts", "application/json",
		strings.NewReader(`{"text":"checkout error rate > 5%","source":"datadog","severity":"critical"}`))
	if err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("intake: %v %v", err, resp.Status)
	}

	var inv investigation.Investigation
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := http.Get(api.URL + "/investigations")
		if err != nil {
			t.Fatal(err)
		}
		var body struct{ Investigations []investigation.Investigation }
		_ = json.NewDecoder(r.Body).Decode(&body)
		r.Body.Close()
		if len(body.Investigations) == 1 && body.Investigations[0].Status == investigation.StatusCompleted {
			inv = body.Investigations[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if inv.Outcome != investigation.OutcomeAnswered || inv.ToolCalls != 1 || !strings.Contains(inv.Report, "[E1]") {
		t.Fatalf("unexpected investigation: %+v", inv)
	}
	if len(inv.Evidence) != 1 || inv.Evidence[0].Snippet != "OOMKilled pod checkout-7f9c" {
		t.Fatalf("evidence: %+v", inv.Evidence)
	}
}
