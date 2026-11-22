# OpenSRE (Go)

A Go implementation of the OpenSRE AI SRE agent, built on a hexagonal
(ports-and-adapters) architecture. It runs next to the Python implementation
during migration. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the
design, the package mapping, and the roadmap.

## Requirements

- Go 1.25 or later (the Anthropic Go SDK needs it)
- Claude credentials: `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, or an
  `ant auth login` profile

## Build and test

```bash
cd go
make test     # unit, adapter, architecture and end-to-end tests (with -race)
make build    # writes bin/opensre
```

## Use

```bash
# Investigate one alert from the terminal
bin/opensre investigate --text "checkout 5xx rate above 5%" --source datadog --severity critical

# Ask a free-form question with the same tools
bin/opensre ask "Which Datadog monitors are alerting right now?"

# List the tools enabled by your environment
bin/opensre tools

# Run the HTTP API and the background investigation worker
bin/opensre serve --addr 127.0.0.1:8080
curl -X POST localhost:8080/alerts -d '{"text":"disk 95% on db-1","source":"grafana"}'
curl localhost:8080/investigations
```

## HTTP API

| Method and path | Body | Response |
| --- | --- | --- |
| `GET /healthz` | – | `{"status":"ok"}` |
| `POST /alerts` | `{"text": required, "alert_name", "severity", "source", "received_at"}` | `202 {"queued":true,"queue_depth":N}` |
| `POST /v1/prompt` | `{"question": required}` | `200 {"answer":"…","tool_calls":N}` |
| `GET /investigations?limit=1-500` | – | newest first (default 50) |
| `GET /investigations/{id}` | – | one investigation, or 404 |

Every endpoint except `/healthz` requires `Authorization: Bearer <token>` when
`OPENSRE_GATEWAY_LISTENER_TOKEN` is set. Without a token, only loopback callers
that are not browsers are accepted. Request bodies are capped at 1 MiB, and
unknown JSON fields are rejected.

## Configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `ANTHROPIC_API_KEY` | yes, unless another Claude credential is set | – | Claude API key |
| `OPENSRE_MODEL` | no | `claude-opus-5-5` | Claude model id |
| `OPENSRE_EFFORT` | no | `high` | `low`, `medium`, `high`, `xhigh`, or `max` |
| `OPENSRE_DISABLE_LLM_FALLBACKS` | no | off | `1` turns off server-side refusal fallbacks |
| `OPENSRE_MAX_ITERATIONS` | no | `25` | tool-loop iteration cap |
| `OPENSRE_WORKERS` | no | `2` | concurrent investigations in `serve` |
| `OPENSRE_ADDR` | no | `127.0.0.1:8080` | `serve` listen address |
| `OPENSRE_GATEWAY_LISTENER_TOKEN` | needed for non-loopback callers | – | bearer token for the HTTP API (`OPENSRE_ALERT_LISTENER_TOKEN` is also accepted) |
| `DD_API_KEY`, `DD_APP_KEY` | no; both needed to enable Datadog tools | – | Datadog credentials |
| `DD_SITE` | no | `datadoghq.com` | Datadog site |
| `GRAFANA_INSTANCE_URL`, `GRAFANA_LOKI_DATASOURCE_UID` | no; both needed to enable Grafana tools | – | Grafana Loki |
| `GRAFANA_READ_TOKEN` | no | – | Grafana bearer token |

Invalid values (for example `OPENSRE_EFFORT=extreme`) stop the process at
startup with a message naming the variable.

**Note:** investigations are kept in memory in this release and are lost when
the process restarts.
