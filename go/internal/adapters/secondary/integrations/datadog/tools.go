package datadog

import (
	"context"
	"fmt"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/evidence"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Tools returns every Datadog tool bound to client.
func Tools(c *Client) []ports.Tool {
	return []ports.Tool{logsTool{c}, monitorsTool{c}, metricsTool{c}}
}

var windowProp = tool.Property{Type: "integer", Description: "Lookback window in minutes", Default: float64(60)}

type logsTool struct{ c *Client }

func (logsTool) Spec() tool.Spec {
	return tool.Spec{
		Name:            "query_datadog_logs",
		Description:     "Search Datadog logs with Datadog log query syntax (e.g. `service:checkout status:error`) over a lookback window.",
		Source:          Source,
		SideEffect:      tool.ReadOnly,
		ConcurrencySafe: true,
		InputSchema: tool.Schema{
			Properties: map[string]tool.Property{
				"query":              {Type: "string", Description: "Datadog log search query"},
				"time_range_minutes": windowProp,
				"limit":              {Type: "integer", Description: "Maximum log events", Default: float64(50)},
			},
			Required: []string{"query"},
		},
	}
}

func (t logsTool) Invoke(ctx context.Context, args map[string]any) (tool.Output, error) {
	query := tool.String(args, "query")
	logs, err := t.c.SearchLogs(ctx, query, minutes(args), tool.Int(args, "limit", 50))
	if err != nil {
		return tool.Output{}, err
	}
	errorCount := 0
	for _, l := range logs {
		if l.Status == "error" || l.Status == "critical" {
			errorCount++
		}
	}
	ev := evidence.Entry{
		Source:  "datadog_logs",
		Label:   "Datadog logs: " + query,
		Summary: fmt.Sprintf("%d events, %d at error or critical", len(logs), errorCount),
	}
	if len(logs) > 0 {
		ev.Snippet = truncate(logs[0].Message, 300)
	}
	return tool.JSONOutput(map[string]any{"query": query, "total": len(logs), "logs": logs}, ev)
}

type monitorsTool struct{ c *Client }

func (monitorsTool) Spec() tool.Spec {
	return tool.Spec{
		Name:            "query_datadog_monitors",
		Description:     "List Datadog monitors and their current state, optionally filtered by a monitor search query (e.g. `tag:service:checkout`).",
		Source:          Source,
		SideEffect:      tool.ReadOnly,
		ConcurrencySafe: true,
		InputSchema: tool.Schema{Properties: map[string]tool.Property{
			"query": {Type: "string", Description: "Monitor search query"},
		}},
	}
}

func (t monitorsTool) Invoke(ctx context.Context, args map[string]any) (tool.Output, error) {
	monitors, err := t.c.ListMonitors(ctx, tool.String(args, "query"))
	if err != nil {
		return tool.Output{}, err
	}
	alerting := 0
	for _, m := range monitors {
		if m.OverallState == "Alert" || m.OverallState == "Warn" {
			alerting++
		}
	}
	return tool.JSONOutput(map[string]any{"total": len(monitors), "monitors": monitors}, evidence.Entry{
		Source:  "datadog_monitors",
		Label:   "Datadog monitors",
		Summary: fmt.Sprintf("%d monitors, %d alerting or warning", len(monitors), alerting),
	})
}

type metricsTool struct{ c *Client }

func (metricsTool) Spec() tool.Spec {
	return tool.Spec{
		Name:            "query_datadog_metrics",
		Description:     "Query a Datadog metric timeseries (e.g. `avg:trace.http.request.errors{service:checkout}`) over a lookback window.",
		Source:          Source,
		SideEffect:      tool.ReadOnly,
		ConcurrencySafe: true,
		InputSchema: tool.Schema{
			Properties: map[string]tool.Property{
				"query":              {Type: "string", Description: "Datadog metrics query"},
				"time_range_minutes": windowProp,
			},
			Required: []string{"query"},
		},
	}
}

func (t metricsTool) Invoke(ctx context.Context, args map[string]any) (tool.Output, error) {
	query := tool.String(args, "query")
	series, err := t.c.QueryMetrics(ctx, query, minutes(args))
	if err != nil {
		return tool.Output{}, err
	}
	return tool.JSONOutput(map[string]any{"query": query, "series": series}, evidence.Entry{
		Source:  "datadog_metrics",
		Label:   "Datadog metrics: " + query,
		Summary: fmt.Sprintf("%d series", len(series)),
	})
}

func minutes(args map[string]any) time.Duration {
	return time.Duration(tool.Int(args, "time_range_minutes", 60)) * time.Minute
}
