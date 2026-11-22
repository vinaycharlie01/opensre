package grafana

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/evidence"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Tools returns every Grafana tool bound to client.
func Tools(c *Client) []ports.Tool {
	return []ports.Tool{serviceNamesTool{c}, logsTool{c}}
}

type serviceNamesTool struct{ c *Client }

func (serviceNamesTool) Spec() tool.Spec {
	return tool.Spec{
		Name:            "query_grafana_service_names",
		Description:     "List the service_name label values known to Grafana Loki. Call this before query_grafana_logs to find the exact service name.",
		Source:          Source,
		SideEffect:      tool.ReadOnly,
		ConcurrencySafe: true,
		InputSchema:     tool.Schema{Properties: map[string]tool.Property{}},
	}
}

func (t serviceNamesTool) Invoke(ctx context.Context, _ map[string]any) (tool.Output, error) {
	names, err := t.c.LabelValues(ctx, "service_name")
	if err != nil {
		return tool.Output{}, err
	}
	return tool.JSONOutput(map[string]any{"service_names": names})
}

type logsTool struct{ c *Client }

func (logsTool) Spec() tool.Spec {
	return tool.Spec{
		Name: "query_grafana_logs",
		Description: "Query Grafana Loki log streams for one exact service_name (discover it with query_grafana_service_names). " +
			"Optionally narrow to lines containing execution_run_id.",
		Source:          Source,
		SideEffect:      tool.ReadOnly,
		ConcurrencySafe: true,
		InputSchema: tool.Schema{
			Properties: map[string]tool.Property{
				"service_name":       {Type: "string", Description: "Exact Loki service_name label value"},
				"execution_run_id":   {Type: "string", Description: "Only lines containing this id"},
				"time_range_minutes": {Type: "integer", Description: "Lookback window in minutes", Default: float64(60)},
				"limit":              {Type: "integer", Description: "Maximum lines", Default: float64(100)},
			},
			Required: []string{"service_name"},
		},
	}
}

func (t logsTool) Invoke(ctx context.Context, args map[string]any) (tool.Output, error) {
	service := tool.String(args, "service_name")
	query := LogQL(service, tool.String(args, "execution_run_id"))
	window := time.Duration(tool.Int(args, "time_range_minutes", 60)) * time.Minute
	lines, truncated, err := t.c.QueryLogs(ctx, query, window, tool.Int(args, "limit", 100))
	if err != nil {
		return tool.Output{}, err
	}
	var errorLines []LogLine
	for _, l := range lines {
		lower := strings.ToLower(l.Line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "fatal") {
			errorLines = append(errorLines, l)
		}
	}
	ev := evidence.Entry{
		Source:  "grafana_logs",
		Label:   "Grafana Loki: " + service,
		Summary: fmt.Sprintf("%d lines, %d look like errors", len(lines), len(errorLines)),
	}
	if len(errorLines) > 0 {
		ev.Snippet = errorLines[0].Line
	}
	return tool.JSONOutput(map[string]any{
		"query": query, "service_name": service, "truncated": truncated,
		"total": len(lines), "error_logs": errorLines, "logs": lines,
	}, ev)
}
