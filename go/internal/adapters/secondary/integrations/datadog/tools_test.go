package datadog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/integrations/datadog"
)

func TestLogsToolSendsAuthAndMapsEvidence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") != "k" || r.Header.Get("DD-APPLICATION-KEY") != "a" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/logs/events/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			Filter struct{ Query string } `json:"filter"`
			Page   struct{ Limit int }    `json:"page"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Filter.Query != "service:checkout" || body.Page.Limit != 1000 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":[
			{"attributes":{"message":"OOMKilled","status":"error","service":"checkout"}},
			{"attributes":{"message":"ok","status":"info","service":"checkout"}}]}`))
	}))
	defer srv.Close()

	tools := datadog.Tools(datadog.NewClient(datadog.Config{APIKey: "k", AppKey: "a", BaseURL: srv.URL}, nil))
	logs := tools[0]
	args, err := logs.Spec().InputSchema.Decode(json.RawMessage(`{"query":"service:checkout","limit":5000}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := logs.Invoke(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, "OOMKilled") || len(out.Evidence) != 1 ||
		out.Evidence[0].Summary != "2 events, 1 at error or critical" || out.Evidence[0].Snippet != "OOMKilled" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestHTTPErrorsSurfaceAsToolErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":["Forbidden"]}`))
	}))
	defer srv.Close()
	c := datadog.NewClient(datadog.Config{APIKey: "k", AppKey: "a", BaseURL: srv.URL}, nil)
	if _, err := c.ListMonitors(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("err = %v", err)
	}
}
