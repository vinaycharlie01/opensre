# Go migration roadmap: 4 weeks

**Window:** Mon 23 Nov – Fri 18 Dec 2026 (20 working days), plus Mon–Tue
21–22 Dec as buffer.
**Starting point:** PR #1. It holds the hexagonal core, the agent loop, the
Claude adapter, 5 Datadog/Grafana tools, the HTTP API, the worker, and the CLI.

Tick boxes in PRs as work lands. Each week ends with a demo and a go/no-go
check against its exit criteria.

---

## 1. What "finished" means

The migration is complete when all of the following hold:

- [ ] The Go service handles alert intake, investigations, `/v1/prompt`, and
      scheduled runs in production, and the Python service no longer takes
      traffic.
- [ ] Every **Tier 1** and **Tier 2** integration below is ported and passes
      its definition of done (§3).
- [ ] Slack, Telegram, and Discord gateways run on Go.
- [ ] CLI command parity for every non-interactive `opensre` command.
- [ ] Investigations, sessions, and schedules persist in Postgres (SQLite
      locally) with migrations.
- [ ] Shadow-mode parity run (§4, Week 4) shows Go reports are at least as good
      as Python on the agreed alert set.
- [ ] Release pipeline ships signed binaries and a Docker image. `install.sh`
      installs the Go binary.

**Cut line, decided up front:** the full interactive REPL (`surfaces/interactive_shell`,
~25k lines of terminal UI) and **Tier 3** integrations are post-month work.
Week 4 ships a minimal REPL. If the schedule slips, apply the cut list in §6
in order. Do not quietly drop quality gates.

## 2. Scope inventory

| Area | Python size | Plan |
| --- | --- | --- |
| `integrations/` (90 folders, ~200 `@tool`s) | 119k lines | Tier 1 → Week 2, Tier 2 → Week 3, Tier 3 → stretch |
| `core/agent_harness` (sessions, turns, grounding, task plan) | 28k | Week 1 (essentials), Week 3 (sessions) |
| `core/llm` (providers, transports) | 8k | Week 1 |
| `core/domain` (alerts, correlation, runbooks, memory, work items) | 8k | Weeks 1–2 |
| `infrastructure/scheduling` | 10k | Week 3 |
| `infrastructure/` other (safety, observability, analytics, filestorage, turn_host) | 27k | Weeks 1–3 |
| `gateway/` (Slack, Telegram, Discord, Buzz, web) | 17k | Week 3 |
| `tools/` (system tools, cross-vendor, registry) | 18k | Weeks 1–2 |
| `surfaces/cli` | ~29k | Week 4 |
| `surfaces/interactive_shell` | ~25k | Week 4 (minimal), rest stretch |
| `config/` | 13k | Ported as needed into `platform/config` |

**Human-owned files stay untouched.** `SKILL.md` files and
`core/agent_harness/prompts/opensre_system_prompt.md` are read by Go as-is,
through a `SkillSource` port. Go code must not copy, rewrite, or edit them.

## 3. Definitions of done

### Per integration

Copy this checklist into each integration's PR:

- [ ] `secondary/integrations/<vendor>/client.go` with timeouts, a body-size
      limit, and context cancellation
- [ ] `Config` + `Configured()`; env var names identical to Python's
      `config/constants/<vendor>.py`
- [ ] Every Python `@tool` for the vendor ported: same tool name, description
      intent, input schema, and side-effect level
- [ ] Evidence entries match the Python `evidence_mapper` keys
- [ ] Alert-source detector and routing entry, if the vendor sends alerts
- [ ] Verifier (`opensre integrations verify <vendor>`) ported
- [ ] Registered in `bootstrap` only when configured
- [ ] `httptest` test: auth headers, request shape, response mapping, HTTP error → tool error
- [ ] Any non-read-only tool sits behind the `ApprovalGate` (Week 1)
- [ ] `go/docs/integrations/<vendor>.md`: required/optional env vars, defaults, gotchas

### Per week

- [ ] `make vet test` green with `-race`; `archtest` green
- [ ] No new third-party imports outside `adapters/secondary`
- [ ] Docs updated for every new env var or command
- [ ] Demo recorded and posted in the PR

## 4. Week-by-week plan

### Week 1 (23–27 Nov): Foundations

Goal: everything later work depends on, so Weeks 2–3 are adapter-only.

**Day 1 (Mon): CI and repo hygiene**
- [ ] Merge PR #1
- [ ] GitHub Actions workflow for `go/`: `go vet`, `go test -race`, `gofmt` check, `govulncheck`
- [ ] `golangci-lint` config (errcheck, staticcheck, revive, gosec)
- [ ] Add a Go section to `CI.md` (scoped local validation)
- [ ] Integration porting template generator (`go run ./tools/newintegration <vendor>`) emitting client/tools/test/doc stubs

**Day 2 (Tue): Persistence**
- [ ] `ports.InvestigationRepository` → `secondary/repository/postgres` (pgx) + `sqlite` (local)
- [ ] Migration tool (goose or atlas); first migrations: investigations, evidence
- [ ] Read `PERSISTENCE.md` and map existing Python tables; decide on reusing or migrating data
- [ ] Repository contract tests run against both backends

**Day 3 (Wed): LLM providers**
- [ ] Provider registry in `bootstrap` selected by `LLM_PROVIDER` (same values as Python)
- [ ] OpenAI + OpenAI-compatible endpoints (`openai_compat_providers.py`)
- [ ] Azure OpenAI, Bedrock, Vertex AI adapters
- [ ] Retries driven by facts from the current request, never by shared client flags
- [ ] Streaming in the `LLM` port (`CompleteStream`) for chat surfaces

**Day 4 (Thu): Safety and observability**
- [ ] `ApprovalGate` driven port; runner refuses non-read-only tools without approval
- [ ] Guardrail rules from `infrastructure/safety/guardrails` → `core/domain/guardrails`
- [ ] Full masking policy (identifier detectors) from `infrastructure/safety/masking`
- [ ] OpenTelemetry `EventSink` + tracing spans; Sentry error adapter
- [ ] Analytics adapter (PostHog) behind a port, off by default

**Day 5 (Fri): Domain completion and credentials**
- [ ] Alert-source detection and routing table (`core/domain/alerts/alert_source.py`)
- [ ] Incident window and anchor types (`core/domain/types`)
- [ ] Credential store port + adapters (env, file, hosted `credentials_api`)
- [ ] `SkillSource` port that loads the existing `SKILL.md` files and system prompt read-only
- [ ] **Week 1 exit:** Postgres-backed `serve` runs one investigation on Claude and one on OpenAI, with traces visible

### Week 2 (30 Nov – 4 Dec): Tier 1 integrations

Goal: the integrations most investigations rely on. Use the §3 checklist for each one.

**Day 6: Observability core**
- [ ] Datadog: remaining tools (`query_datadog_all`, `query_datadog_events`), verifier, incident anchor
- [ ] Grafana: metrics (Mimir), traces (Tempo), alert rules, annotations
- [ ] Tempo (standalone)

**Day 7: Logs, APM, and errors**
- [ ] Sentry (+ `sentry_mcp`)
- [ ] CloudWatch
- [ ] Elasticsearch, OpenSearch
- [ ] New Relic, Honeycomb

**Day 8: Kubernetes and AWS**
- [ ] Kubernetes (client-go), EKS (11 tools), Helm
- [ ] AWS core: EC2, ELB, Lambda, S3, SQS, RDS, CloudTrail

**Day 9: Incident and alert sources**
- [ ] PagerDuty, incident.io, Alertmanager (webhook payload parsing → `alert.Alert`)
- [ ] Vendor alert-payload parsers on `POST /alerts` (Datadog, Grafana, PagerDuty, Alertmanager)
- [ ] Tracer (12 tools)

**Day 10: System tools and cross-vendor**
- [ ] `tools/system`: runbook guidance, SRE guidance, structured file, file count, workspace paths
- [ ] `tools/cross_vendor` tools
- [ ] Agent memory and work items (with repository ports)
- [ ] **Week 2 exit:** the Python eval/fixture alert set runs end to end on Go with Tier 1 tools

### Week 3 (7–11 Dec): Tier 2 integrations, gateway, scheduler

**Day 11: Code hosts and ticketing**
- [ ] GitHub (33 tools; split read-only from mutating, mutating behind `ApprovalGate`)
- [ ] GitLab, Bitbucket, local git
- [ ] Jira

**Day 12: Databases and queues**
- [ ] PostgreSQL, MySQL, MariaDB, Azure SQL (read-only query guards)
- [ ] Redis, MongoDB, MongoDB Atlas, ClickHouse
- [ ] Kafka, RabbitMQ

**Day 13: Pipelines, deploy, and remaining observability**
- [ ] Airflow, Dagster, Prefect, Temporal, Jenkins, ArgoCD
- [ ] SigNoz, Groundcover, Coralogix, Splunk, Better Stack, VictoriaLogs, OpenObserve, PostHog
- [ ] Vercel, Railway, Supabase, Neon, Azure, Snowflake

**Day 14: Chat gateway**
- [ ] `SessionRepository` port + Postgres adapter (multi-turn conversations)
- [ ] Shared turn host: one turn runner used by every transport and the CLI
- [ ] Gateway middleware: inbound decision, identity, approval steps
- [ ] `primary/chat/slack` (Events API + interactive approvals), outbound Slack delivery adapter
- [ ] `primary/chat/telegram`, `primary/chat/discord`

**Day 15: Scheduler and delivery**
- [ ] `primary/scheduler` driving adapter (cron, scheduled loops, daily update, morning report)
- [ ] `ports.Notifier` + adapters: Slack, email (SMTP), Telegram, Discord
- [ ] Gateway web routes: `loop_routes`, `prompt_routes`
- [ ] **Week 3 exit:** Slack conversation with approvals works on staging; a scheduled run posts a report

### Week 4 (14–18 Dec): Surfaces, parity, cutover

**Day 16: CLI parity**
- [ ] Inventory every `surfaces/cli/commands` command; port the non-interactive ones (cobra)
- [ ] `integrations` subcommands: list, setup wizard, verify
- [ ] Account/auth commands, onboarding wizard (forms show required/optional fields and defaults, validate before saving, keep values on error)

**Day 17: Minimal REPL and embedding**
- [ ] `primary/repl`: prompt, streaming answer, tool progress, slash commands for the top 10 actions
- [ ] Embedded entry point (`surfaces/embedded.py` equivalent) as a Go package API

**Day 18: Packaging and deployment**
- [ ] Multi-arch release binaries (goreleaser), checksums, signing
- [ ] Docker image (distroless); update `Dockerfile` / `DEPLOYMENT.md`
- [ ] `install.sh` / `install.ps1` install the Go binary
- [ ] EC2/ECS deployment config switched to the Go image behind a flag

**Day 19: Shadow mode and parity**
- [ ] Mirror production alerts to Go in shadow mode (Go reports stored, not delivered)
- [ ] Compare Go vs Python on the agreed alert set: tool calls, evidence, report quality, latency, cost
- [ ] Fix parity gaps; confirm the defect bar (zero P0/P1)

**Day 20: Cutover**
- [ ] Switch intake, `/v1/prompt`, and gateways to Go endpoint by endpoint; keep Python on standby
- [ ] Rollback runbook tested (flip back to Python in < 5 min)
- [ ] User-facing docs updated (`docs/` pages and `docs/docs.json` navigation)
- [ ] **Week 4 exit:** §1 checklist complete, or the remaining items explicitly moved to the post-month list

### Buffer (21–22 Dec)
- [ ] Spill-over from Week 4
- [ ] Decommission plan for Python packages (schedule, owners) — not executed in this window

## 5. Post-month (stretch, Tier 3)

- [ ] Full interactive shell UI parity (themes, history, prompt UI)
- [ ] Buzz, Rocket.Chat, WhatsApp, Twilio transports
- [ ] Trello, ServiceNow, Google Docs, Pipedream
- [ ] `coding_agent`, `llm_cli`, `pi`, `mcp_servers` (MCP client)
- [ ] Hosted gateway, billing, knowledge scans, webapp knowledge/requests/scheduled loops
- [ ] Remove the Python implementation after 2 weeks of stable Go traffic

## 6. Cut list (apply in order if behind schedule)

1. Minimal REPL → postpone (CLI `ask` covers interactive use)
2. Tier 2 "remaining observability" (Day 13, second bullet) → post-month
3. Pipelines and deploy tools (Day 13, first bullet) → post-month
4. Discord transport → post-month (keep Slack and Telegram)
5. Azure OpenAI / Vertex providers → post-month (keep Anthropic, OpenAI, Bedrock)

Never cut: CI, persistence, `ApprovalGate`, masking, the shadow-mode parity
run, or the rollback runbook.

## 7. Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Throughput: ~90 integrations in ~10 days | Slip in Weeks 2–3 | Template generator (Day 1); port by tool count, not folder; cut list §6 |
| Behaviour drift from Python | Worse investigations | Shadow mode on real alerts (Day 19); keep tool names and schemas identical |
| Mutating tools ported before approvals exist | Unsafe actions in production | `ApprovalGate` lands Day 4; arch/registry check refuses non-read-only tools without it |
| Data migration from Python storage | Lost history | Decide reuse vs migrate on Day 2; dry run before cutover |
| Slack/Telegram webhook differences | Broken chat | Run both gateways on staging in Week 3; replay recorded payloads in tests |
| Single-person bandwidth | Whole plan slips | Daily check against this file; apply cuts on Day 10 and Day 15 if behind |

## 8. Cadence

- **Daily:** open PRs against the day's checklist; end the day with CI green and boxes ticked.
- **Day 5, 10, 15, 20:** demo plus go/no-go against the week's exit criteria; apply cuts if needed.
- **Tracking:** one GitHub issue per day (or per integration in Weeks 2–3), linked from its checkbox.
