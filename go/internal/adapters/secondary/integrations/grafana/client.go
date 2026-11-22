// Package grafana implements Grafana Loki-backed agent tools (driven adapters
// for the ports.Tool port) and the HTTP client they share.
package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Source is this integration's tool/evidence source key.
const Source = "grafana"

// MaxLogLines caps lines returned from one Loki query.
const MaxLogLines = 5000

// Config holds Grafana settings (GRAFANA_INSTANCE_URL, GRAFANA_READ_TOKEN,
// GRAFANA_LOKI_DATASOURCE_UID).
type Config struct {
	InstanceURL string
	ReadToken   string
	LokiUID     string
}

// Configured reports whether Loki can be queried.
func (c Config) Configured() bool { return c.InstanceURL != "" && c.LokiUID != "" }

// Client is a minimal Grafana datasource-proxy client.
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
	cfg.InstanceURL = strings.TrimRight(cfg.InstanceURL, "/")
	return &Client{cfg: cfg, http: httpClient, now: time.Now}
}

// LogLine is one Loki log entry.
type LogLine struct {
	Timestamp string            `json:"timestamp"`
	Line      string            `json:"line"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// LogQL builds a stream selector for a service with optional line filters.
// Values are quoted so a label value cannot break out of the selector.
func LogQL(service string, contains ...string) string {
	q := "{service_name=" + strconv.Quote(service) + "}"
	for _, c := range contains {
		if c != "" {
			q += " |= " + strconv.Quote(c)
		}
	}
	return q
}

// QueryLogs runs a LogQL range query over the last window. truncated is true
// when results were cut at MaxLogLines.
func (c *Client) QueryLogs(ctx context.Context, query string, window time.Duration, limit int) (lines []LogLine, truncated bool, err error) {
	limit = min(limit, MaxLogLines)
	end := c.now()
	q := url.Values{
		"query": {query},
		"limit": {strconv.Itoa(limit)},
		"start": {strconv.FormatInt(end.Add(-window).UnixNano(), 10)},
		"end":   {strconv.FormatInt(end.UnixNano(), 10)},
	}
	var resp struct {
		Data struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][2]string       `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.get(ctx, c.proxy("/loki/api/v1/query_range"), q, &resp); err != nil {
		return nil, false, err
	}
	for _, stream := range resp.Data.Result {
		for _, v := range stream.Values {
			if len(lines) >= limit {
				return lines, true, nil
			}
			lines = append(lines, LogLine{Timestamp: v[0], Line: v[1], Labels: stream.Stream})
		}
	}
	return lines, false, nil
}

// LabelValues lists the values of a Loki label (e.g. service_name).
func (c *Client) LabelValues(ctx context.Context, label string) ([]string, error) {
	var resp struct {
		Data []string `json:"data"`
	}
	path := "/loki/api/v1/label/" + url.PathEscape(label) + "/values"
	if err := c.get(ctx, c.proxy(path), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) proxy(path string) string {
	return c.cfg.InstanceURL + "/api/datasources/proxy/uid/" + url.PathEscape(c.cfg.LokiUID) + path
}

func (c *Client) get(ctx context.Context, u string, q url.Values, out any) error {
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if c.cfg.ReadToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.ReadToken)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("grafana: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("grafana: HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(data, out)
}
