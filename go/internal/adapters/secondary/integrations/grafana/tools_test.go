package grafana_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/integrations/grafana"
)

func TestLogQLQuotesValues(t *testing.T) {
	got := grafana.LogQL(`api"} |= "x`, "run-1")
	want := `{service_name="api\"} |= \"x"} |= "run-1"`
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestLogsToolQueriesLokiThroughDatasourceProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" ||
			r.URL.Path != "/api/datasources/proxy/uid/loki-1/loki/api/v1/query_range" ||
			r.URL.Query().Get("query") != `{service_name="checkout"}` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"result":[{"stream":{"service_name":"checkout"},
			"values":[["1","ERROR payment timeout"],["2","started"],["3","healthy"]]}]}}`))
	}))
	defer srv.Close()

	c := grafana.NewClient(grafana.Config{InstanceURL: srv.URL + "/", ReadToken: "tok", LokiUID: "loki-1"}, nil)
	logs := grafana.Tools(c)[1]
	out, err := logs.Invoke(context.Background(), map[string]any{"service_name": "checkout", "limit": float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, `"truncated": true`) || out.Evidence[0].Snippet != "ERROR payment timeout" {
		t.Fatalf("unexpected output: %s %+v", out.Content, out.Evidence)
	}
}
