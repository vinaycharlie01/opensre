// Package datadog implements Datadog-backed agent tools (driven adapters for
// the ports.Tool port) and the HTTP client they share.
package datadog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Source is this integration's tool/evidence source key.
const Source = "datadog"

// Config holds Datadog credentials (DD_API_KEY, DD_APP_KEY, DD_SITE).
type Config struct {
	APIKey string
	AppKey string
	// Site is the Datadog site, e.g. "datadoghq.com" or "datadoghq.eu".
	Site string
	// BaseURL overrides https://api.<Site> (tests).
	BaseURL string
}

// Configured reports whether credentials are present.
func (c Config) Configured() bool { return c.APIKey != "" && c.AppKey != "" }

func (c Config) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	site := c.Site
	if site == "" {
		site = "datadoghq.com"
	}
	return "https://api." + site
}

// Client is a minimal Datadog API client.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time
}

// NewClient builds a client. A nil httpClient uses a 30s-timeout default.
func NewClient(cfg Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{cfg: cfg, http: httpClient, now: time.Now}
}

// Log is one log event.
type Log struct {
	Timestamp string         `json:"timestamp"`
	Message   string         `json:"message"`
	Status    string         `json:"status"`
	Service   string         `json:"service"`
	Host      string         `json:"host"`
	Tags      []string       `json:"tags,omitempty"`
	Attrs     map[string]any `json:"attributes,omitempty"`
}

// SearchLogs runs a Log Search API (v2) query over the last window.
func (c *Client) SearchLogs(ctx context.Context, query string, window time.Duration, limit int) ([]Log, error) {
	now := c.now().UTC()
	body := map[string]any{
		"filter": map[string]any{
			"query": query,
			"from":  now.Add(-window).Format(time.RFC3339),
			"to":    now.Format(time.RFC3339),
		},
		"sort": "-timestamp",
		"page": map[string]any{"limit": min(limit, 1000)},
	}
	var resp struct {
		Data []struct {
			Attributes struct {
				Timestamp  string         `json:"timestamp"`
				Message    string         `json:"message"`
				Status     string         `json:"status"`
				Service    string         `json:"service"`
				Host       string         `json:"host"`
				Tags       []string       `json:"tags"`
				Attributes map[string]any `json:"attributes"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v2/logs/events/search", nil, body, &resp); err != nil {
		return nil, err
	}
	logs := make([]Log, 0, len(resp.Data))
	for _, d := range resp.Data {
		a := d.Attributes
		logs = append(logs, Log{
			Timestamp: a.Timestamp, Message: a.Message, Status: a.Status,
			Service: a.Service, Host: a.Host, Tags: a.Tags, Attrs: a.Attributes,
		})
	}
	return logs, nil
}

// Monitor is a Datadog monitor summary.
type Monitor struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Query        string   `json:"query"`
	OverallState string   `json:"overall_state"`
	Tags         []string `json:"tags,omitempty"`
}

// ListMonitors lists monitors matching an optional query.
func (c *Client) ListMonitors(ctx context.Context, query string) ([]Monitor, error) {
	q := url.Values{"page": {"0"}, "page_size": {"50"}}
	if query != "" {
		q.Set("query", query)
	}
	var monitors []Monitor
	if err := c.do(ctx, http.MethodGet, "/api/v1/monitor", q, nil, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

// Series is one metric timeseries; Points are [unix_ms, value] pairs.
type Series struct {
	Metric string       `json:"metric"`
	Scope  string       `json:"scope"`
	Points [][2]float64 `json:"pointlist"`
}

// QueryMetrics runs a metrics query over the last window.
func (c *Client) QueryMetrics(ctx context.Context, query string, window time.Duration) ([]Series, error) {
	now := c.now().UTC()
	q := url.Values{
		"query": {query},
		"from":  {strconv.FormatInt(now.Add(-window).Unix(), 10)},
		"to":    {strconv.FormatInt(now.Unix(), 10)},
	}
	var resp struct {
		Series []Series `json:"series"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/query", q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Series, nil
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	u := c.cfg.baseURL() + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("DD-API-KEY", c.cfg.APIKey)
	req.Header.Set("DD-APPLICATION-KEY", c.cfg.AppKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("datadog %s: %w", path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("datadog %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(data), 200))
	}
	return json.Unmarshal(data, out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
