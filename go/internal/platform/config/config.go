// Package config loads process settings from environment variables. Names
// match the Python implementation so existing deployments keep working.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Environment variable names.
const (
	EnvAddr             = "OPENSRE_ADDR"
	EnvModel            = "OPENSRE_MODEL"
	EnvEffort           = "OPENSRE_EFFORT"
	EnvMaxIterations    = "OPENSRE_MAX_ITERATIONS"
	EnvWorkers          = "OPENSRE_WORKERS"
	EnvListenerToken    = "OPENSRE_GATEWAY_LISTENER_TOKEN"
	EnvLegacyToken      = "OPENSRE_ALERT_LISTENER_TOKEN"
	EnvDisableFallbacks = "OPENSRE_DISABLE_LLM_FALLBACKS"
	EnvDatadogAPIKey    = "DD_API_KEY"
	EnvDatadogAppKey    = "DD_APP_KEY"
	EnvDatadogSite      = "DD_SITE"
	EnvGrafanaURL       = "GRAFANA_INSTANCE_URL"
	EnvGrafanaToken     = "GRAFANA_READ_TOKEN"
	EnvGrafanaLokiUID   = "GRAFANA_LOKI_DATASOURCE_UID"
)

// Config is the resolved process configuration.
type Config struct {
	Addr             string
	Model            string
	Effort           string
	MaxIterations    int
	Workers          int
	ListenerTokens   []string
	DisableFallbacks bool

	DatadogAPIKey string
	DatadogAppKey string
	DatadogSite   string

	GrafanaURL     string
	GrafanaToken   string
	GrafanaLokiUID string
}

var validEfforts = map[string]bool{"": true, "low": true, "medium": true, "high": true, "xhigh": true, "max": true}

// Load reads the environment through getenv (os.Getenv when nil).
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	c := Config{
		Addr:             or(get(EnvAddr), "127.0.0.1:8080"),
		Model:            get(EnvModel),
		Effort:           or(get(EnvEffort), "high"),
		DisableFallbacks: get(EnvDisableFallbacks) == "1" || strings.EqualFold(get(EnvDisableFallbacks), "true"),
		DatadogAPIKey:    get(EnvDatadogAPIKey),
		DatadogAppKey:    get(EnvDatadogAppKey),
		DatadogSite:      get(EnvDatadogSite),
		GrafanaURL:       get(EnvGrafanaURL),
		GrafanaToken:     get(EnvGrafanaToken),
		GrafanaLokiUID:   get(EnvGrafanaLokiUID),
	}
	for _, k := range []string{EnvListenerToken, EnvLegacyToken} {
		if v := get(k); v != "" {
			c.ListenerTokens = append(c.ListenerTokens, v)
		}
	}
	if !validEfforts[c.Effort] {
		return Config{}, fmt.Errorf("%s must be one of low, medium, high, xhigh, max", EnvEffort)
	}
	var err error
	if c.MaxIterations, err = intVar(get, EnvMaxIterations, 25); err != nil {
		return Config{}, err
	}
	if c.Workers, err = intVar(get, EnvWorkers, 2); err != nil {
		return Config{}, err
	}
	return c, nil
}

func intVar(get func(string) string, key string, def int) (int, error) {
	v := get(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
