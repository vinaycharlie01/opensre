# Go migration: full task breakdown

Generated from a scan of the Python repository on 22 Nov 2026. It is the
detailed companion to [ROADMAP.md](ROADMAP.md): every task has an ID, the
Python sources it replaces, an estimate, its dependencies, and acceptance
criteria. Integration tool lists come straight from the `@tool` decorators and
`BaseTool` subclasses in the code.

## 1. What the scan found

| Area | Python (non-test lines) | Units |
| --- | --- | --- |
| `integrations/` | 119k | 90 vendor folders |
| Agent tools | – | **268** total: 219 read-only, 32 mutating, 17 external |
| `core/` | 54k | agent harness 28k (turns 7.4k, prompts 6.5k, session 5.8k, session_goal 3.6k, task_plan 1.9k), llm 8.5k, domain 7.9k (memory 3.6k, work_items 1.3k) |
| `surfaces/` | 54k | 28 top-level CLI commands, ~60 REPL slash commands |
| `infrastructure/` | 37k | scheduling 10.3k, analytics 4.8k, filestorage 3.7k, deployment 2.9k, observability 2.7k, safety 2.4k, turn_host 1.8k |
| `gateway/` | 17k | Slack 4.2k, Discord 1.9k, Buzz 1.7k, Telegram 1.6k, core 6.9k |
| `tools/` | 18k | 22 system tools, 2 cross-vendor tools, registry |
| `config/` | 13k | env var constants per vendor |
| Tests | 300k | used as parity fixtures, not ported line by line |
| LLM providers | – | anthropic, openai, azure_openai, bedrock, vertex, 8 OpenAI-compatible (openrouter, trustedrouter, deepseek, gemini, nvidia, minimax, groq, ollama), custom endpoint, ChatGPT/Grok subscriptions |
| HTTP routes | – | `POST /alerts`, `GET /healthz`, `GET /v1/prompt`, `POST /v1/prompt`, `POST /v1/loops/sync`, `GET /`, `/health`, `/ok`, `/readyz` |
| Persistent stores | – | scheduler `task_runs`; gateway `slack_handled_events`, `gateway_session_bindings`; telemetry `episodes`, `events`, `watch`; CI fix `prepared_pushes`, `unsettled_merge_revisions`; JSONL sessions with WAL recovery; session-goal and task-plan files |

**Note:** `AGENTS.md` links to `PERSISTENCE.md`, but that file does not exist in the repo.
PLT-DB-3 inventories the stores directly instead.

## 2. Capacity check

Estimated effort inside the month: **~569 focused hours** across 20 working days
(~28 h/day). Post-month (Tier 3) work: **~116 hours**.

Estimates assume AI-assisted porting from the scaffold generator (PLT-CI-3):
integrations are `1 h + 0.5 h per tool + 1 h per 600 Python lines`, adjusted by
hand for GitHub and Slack. Treat them as ±30%.

At ~6 focused hours per person-day, that is **4.7 people** for 4 weeks.
**One person cannot finish this scope in a month.** The plan below needs five
people, one per workstream. With fewer people, either extend the timeline
(roughly 561 h ÷ 120 h per person-month) or apply the cut list in ROADMAP.md §6
at the Day 10 checkpoint.

| Day | Date | Theme | Hours (all streams) |
| --- | --- | --- | --- |
| D01 | Mon 23 Nov | CI, generator, parity harness | 12 |
| D02 | Tue 24 Nov | Persistence | 10 |
| D03 | Wed 25 Nov | LLM providers | 15 |
| D04 | Thu 26 Nov | Safety, observability | 17 |
| D05 | Fri 27 Nov | Domain, credentials, prompts | 23 |
| D06 | Mon 30 Nov | Observability integrations | 20 |
| D07 | Tue 1 Dec | Logs/APM integrations | 22 |
| D08 | Wed 2 Dec | Kubernetes + AWS | 50 |
| D09 | Thu 3 Dec | Incident sources | 21 |
| D10 | Fri 4 Dec | System tools, memory, turns | 24 |
| D11 | Mon 7 Dec | GitHub, code hosts | 76 |
| D12 | Tue 8 Dec | Databases, queues | 50 |
| D13 | Wed 9 Dec | Pipelines, deploy, observability long tail | 64 |
| D14 | Thu 10 Dec | Gateway, Slack/Telegram/Discord, sessions | 60 |
| D15 | Fri 11 Dec | Scheduler, delivery, web routes | 21 |
| D16 | Mon 14 Dec | CLI parity | 51 |
| D17 | Tue 15 Dec | Minimal REPL, embedded API | 10 |
| D18 | Wed 16 Dec | Release, deploy | 12 |
| D19 | Thu 17 Dec | Shadow mode | 4 |
| D20 | Fri 18 Dec | Cutover, docs | 7 |

### Staffing by workstream

The day themes show *order*, not one person's day. Five workstreams run in parallel;
each fits in 4 weeks at about 6 focused hours a day (120 h):

| Workstream | Scope | Hours | Fits in 120 h? |
| --- | --- | --- | --- |
| WS1 | Platform and harness (CI, DB, LLM, safety, observability, domain, prompts, turns, memory) + system tools + pipeline integrations (D13) | 125 | tight |
| WS2 | Integrations A: observability, Kubernetes/AWS, incident sources (D06–D09) | 112 | yes |
| WS3 | Integrations B: GitHub, code hosts, Jira, databases and queues (D11–D12) | 125 | tight |
| WS4 | Gateway, sessions, turn host, scheduler, file storage + chat/delivery vendors + deploy-platform and long-tail observability integrations (D13–D15) | 122 | tight |
| WS5 | CLI, REPL, embedding, release, shadow mode, cutover, docs | 85 | yes |

Cross-stream dependencies to watch: WS2/WS3 mutating tools wait for PLT-SAF-1 (D04);
WS4 sessions wait for PLT-DB-3 (D02) and PLT-HAR-3 (D10); WS5 cutover waits for everything.

## 3. Conventions

- **IDs:** `PLT-*` platform, `INT-<vendor>` integration, `GH-*` GitHub sub-areas, `TS-*` system tools,
  `GW-*` gateway, `CLI-*` commands, `REPL-*`, `DEP-*` deployment, `CUT-*` cutover, `DOC-*`.
- **Day:** `D01`–`D20` match ROADMAP.md; `T3` means post-month.
- **Every integration task** also carries the per-integration definition of done in ROADMAP.md §3.
- **Tool names, input schemas and side-effect levels must match Python** (checked by PLT-CI-4).
- **Mutating and external tools** stay unregistered until PLT-SAF-1 (ApprovalGate) has landed.
- **Human-owned files** (`SKILL.md`, `opensre_system_prompt.md`) are loaded read-only, never edited.

## 4. Week 1 — platform foundations

### PLT-CI-1 · Go CI workflow
`D01` · WS1 · ~3 h · depends on: PR #1  
Python: `.github/workflows/, CI.md`

- [ ] Workflow on `go/**` changes: `go vet`, `go test -race`, `gofmt -l` must be empty
- [ ] `govulncheck` job
- [ ] Cache Go modules
- [ ] Add a Go section to CI.md (scoped local validation only)
- [ ] **Done when:** PRs touching `go/` show a required green Go check

### PLT-CI-2 · Lint config
`D01` · WS1 · ~2 h · depends on: PLT-CI-1  
Python: `ruff.toml / mypy.ini (Python equivalents)`

- [ ] `golangci-lint` with errcheck, staticcheck, revive, gosec, bodyclose
- [ ] Fix existing findings
- [ ] **Done when:** `golangci-lint run` clean in CI

### PLT-CI-3 · Integration scaffold generator
`D01` · WS1 · ~3 h · depends on: PR #1  
Python: `docs/adding-tools-and-integrations.md`

- [ ] `go run ./tools/newintegration <vendor>` writes client.go, tools.go, tools_test.go, docs page
- [ ] Generated test covers auth header, request shape, HTTP error → tool error
- [ ] Template includes the per-integration checklist as a PR comment block
- [ ] **Done when:** A generated stub compiles and its test passes

### PLT-CI-4 · Python ↔ Go parity harness
`D01` · WS1 · ~4 h · depends on: PR #1  
Python: `tests/ fixtures (300k lines of Python tests)`

- [ ] Export each Python tool's name, schema and side-effect level to JSON (one-off script)
- [ ] Go test fails if a ported tool's name/schema/side effect differs from the export
- [ ] Collect recorded vendor payloads from Python test fixtures for reuse in Go httptest
- [ ] **Done when:** Parity test lists every unported tool as pending, never as silently missing

### PLT-DB-1 · Database layer and migrations
`D02` · WS1 · ~5 h · depends on: PLT-CI-1  
Python: `infrastructure/database/sqlite.py, gateway/core/storage/{postgres,migrations}.py`

- [ ] Choose driver (pgx) and migration tool (goose); embed migrations in the binary
- [ ] SQLite for local/CLI, Postgres for gateway/server, selected by DSN
- [ ] Contract-test helper that runs a repository suite against both backends
- [ ] **Done when:** `opensre` creates and migrates both backends on first start

### PLT-DB-2 · Investigation + evidence tables
`D02` · WS1 · ~2 h · depends on: PLT-DB-1  
Python: (new in Go)

- [ ] Postgres/SQLite `InvestigationRepository`
- [ ] Replace in-memory default in bootstrap
- [ ] **Done when:** Investigations survive a restart

### PLT-DB-3 · Inventory and migration plan for existing Python stores
`D02` · WS1 · ~3 h · depends on: PLT-DB-1  
Python: see list

- [ ] Scheduler `task_runs` (infrastructure/scheduling/scheduler/storage)
- [ ] Gateway `slack_handled_events`, `gateway_session_bindings` (gateway/core/storage)
- [ ] Telemetry `episodes`, `events`, `watch` (core/telemetry/storage)
- [ ] CI fix `prepared_pushes`, `unsettled_merge_revisions` (integrations/github/tools/ci_fix/storage)
- [ ] Agent sessions: JSONL store with WAL recovery (core/agent_harness/session/persistence)
- [ ] Session goal and task plan files (core/agent_harness/{session_goal,task_plan}/persist.py)
- [ ] Decide per store: reuse schema as-is, migrate data, or start fresh; write it into ARCHITECTURE.md
- [ ] **Done when:** Every store above has a decision and an owner task

### PLT-LLM-1 · Provider registry and selection
`D03` · WS1 · ~3 h · depends on: PR #1  
Python: `config/llm_settings.py, core/llm/{factory,client_builders,provider_models,readiness}.py`

- [ ] `LLM_PROVIDER` values identical to Python (default `anthropic`)
- [ ] Per-provider API-key env map (`LLM_PROVIDER_API_KEY_ENVS`)
- [ ] Readiness check used by `doctor` and `health`
- [ ] **Done when:** Unknown provider fails at startup with a message naming `LLM_PROVIDER`

### PLT-LLM-2 · Anthropic adapter parity
`D03` · WS1 · ~3 h · depends on: PLT-LLM-1  
Python: `core/llm/transports/sdk/{anthropic_cache,anthropic_deferred_tools,anthropic_thinking}.py`

- [ ] Prompt caching breakpoints (stable tools → system → messages)
- [ ] Deferred tool loading (tool search) for large tool sets
- [ ] Streaming via the SDK; per-turn usage accounting
- [ ] **Done when:** Cache read tokens > 0 on the second turn of a run

### PLT-LLM-3 · OpenAI, Azure OpenAI, Bedrock, Vertex adapters
`D03` · WS1 · ~5 h · depends on: PLT-LLM-1  
Python: `core/llm/providers/{azure_openai,vertex_ai,bedrock_model_ids}.py, transports/sdk/bedrock_converse.py`

- [ ] OpenAI (official Go SDK)
- [ ] Azure OpenAI
- [ ] Bedrock Converse
- [ ] Vertex AI
- [ ] Map each provider's context-overflow error to `ports.ErrContextOverflow`
- [ ] **Done when:** Same scripted investigation passes on each provider against recorded responses

### PLT-LLM-4 · OpenAI-compatible providers and custom endpoints
`D03` · WS1 · ~2 h · depends on: PLT-LLM-3  
Python: `core/llm/providers/{openai_compat_providers,custom_endpoints}.py`

- [ ] openrouter, trustedrouter, deepseek, gemini, nvidia, minimax, groq, ollama (base URL + key env per provider)
- [ ] Custom OpenAI-compatible endpoint (`CUSTOM_OPENAI_*`)
- [ ] **Done when:** Each provider resolves model + base URL exactly as Python does (table test)

### PLT-LLM-5 · Retry, failure classification, hosted credits
`D03` · WS1 · ~2 h · depends on: PLT-LLM-3  
Python: `core/llm/{failure_classification,hosted_credits,llm_invoke_errors}.py, core/llm_invoke_errors.py`

- [ ] Retry decisions from facts captured on the current request only
- [ ] Credit/quota errors surfaced as typed errors
- [ ] **Done when:** Concurrent turns on one cached client never share retry state (race test)

### PLT-LLM-6 · Subscriptions (ChatGPT, Grok) and LiteLLM transport
`T3` · T3 · ~6 h · depends on: PLT-LLM-3  
Python: `core/llm/subscriptions/, core/llm/transports/litellm/`

- [ ] Decide whether LiteLLM routing is still needed in Go (likely replaced by native adapters)
- [ ] ChatGPT/Grok subscription OAuth flows
- [ ] **Done when:** Decision recorded; flows ported or explicitly dropped

### PLT-SAF-1 · ApprovalGate port and runner enforcement
`D04` · WS1 · ~4 h · depends on: PR #1  
Python: `core/agent_harness/turns/, gateway/core/middleware/approvals.py, surfaces/cli/ask/approval.py`

- [ ] `ports.ApprovalGate` (request, decision, expiry; `DEFAULT_APPROVAL_EXPIRY_SECONDS`)
- [ ] Runner refuses MUTATING/EXTERNAL tools without an approval
- [ ] Registry refuses to register them when no gate is wired
- [ ] CLI gate (prompt), HTTP gate (deny by default), chat gate (Week 3)
- [ ] **Done when:** A mutating tool call without approval returns an error observation, never runs

### PLT-SAF-2 · Guardrails, sandbox, auth
`D04` · WS1 · ~4 h · depends on: PLT-SAF-1  
Python: `infrastructure/safety/{guardrails,sandbox,auth}/, secret_redaction.py`

- [ ] Guardrail rules, evaluator, audit log (`opensre guardrails init|test|rules`)
- [ ] Sandbox for local command/code execution tools
- [ ] Secret redaction parity with Python patterns
- [ ] **Done when:** Python guardrail rule fixtures produce identical verdicts in Go

### PLT-SAF-3 · Masking policy
`D04` · WS1 · ~2 h · depends on: PR #1  
Python: `infrastructure/safety/masking/{detectors,policy,rules}.py`

- [ ] Identifier detectors (pod, namespace, cluster, service, hostname, account id)
- [ ] Policy config and extra patterns
- [ ] Reversible masking map per run if Python has it
- [ ] **Done when:** Python masking test vectors pass in Go

### PLT-OBS-1 · Tracing, error reporting, operations log
`D04` · WS1 · ~4 h · depends on: PR #1  
Python: `infrastructure/observability/{trace,errors,operations_log.py,otlp_parser.py,streaming.py}`

- [ ] OpenTelemetry spans for loop, iteration, LLM call, tool call (stable names: `run-react-loop`, `think`)
- [ ] Sentry adapter; `capture_exception` equivalent at boundaries
- [ ] Operations log
- [ ] OTLP parser (used by `triage`)
- [ ] **Done when:** One investigation produces a complete trace in a local collector

### PLT-OBS-2 · Analytics (PostHog), off by default
`D04` · WS1 · ~3 h · depends on: PLT-OBS-1  
Python: `infrastructure/analytics/ (4.8k lines)`

- [ ] Event catalog from METRICS.md
- [ ] Install/usage context, opt-out
- [ ] No user data in event properties
- [ ] **Done when:** Events match Python names; disabled unless configured

### PLT-DOM-1 · Alert domain completion
`D05` · WS1 · ~3 h · depends on: PR #1  
Python: `core/domain/alerts/{alert_source,extraction,fields,inbox}.py, integrations/alert_source_catalog.py`

- [ ] Alert-source detector registry and routing table (relevance + seed tool sources)
- [ ] Field extraction from vendor payloads
- [ ] Seed tools run first for routed alerts
- [ ] **Done when:** Python routing fixtures resolve to the same tool sources

### PLT-DOM-2 · Domain types, correlation, runbooks, knowledge
`D05` · WS1 · ~4 h · depends on: PR #1  
Python: `core/domain/{types,correlation,runbooks,knowledge,repositories}/`

- [ ] Incident window/anchor, retrieval controls, upstream types
- [ ] Correlation rules
- [ ] Runbook model + `runbooks add|list|remove|verify`
- [ ] Knowledge model
- [ ] **Done when:** Unit tests ported for each module

### PLT-CRD-1 · Credential store and integration catalog
`D05` · WS1 · ~4 h · depends on: PLT-DB-1  
Python: `integrations/{catalog.py,_catalog_impl.py,account_integrations.py,credentials_api/,config_models.py,effective_models.py}`

- [ ] `CredentialStore` port: env, local file, hosted credentials API
- [ ] Integration catalog: which integrations are configured, effective config
- [ ] Verifier framework (`integrations/verification`, `_verifiers_loader.py`)
- [ ] **Done when:** `opensre integrations list` shows the same configured set as Python

### PLT-MCP-1 · MCP client adapter
`D05` · WS1 · ~4 h · depends on: PR #1  
Python: `integrations/github/mcp.py, integrations/mcp_servers/, sentry_mcp, posthog_mcp`

- [ ] MCP client over stdio and streamable HTTP (official Go MCP SDK)
- [ ] Expose remote MCP tools through `ports.Tool` with their schemas
- [ ] Auth token injection; per-server tool allowlist (`GITHUB_MCP_TOOLSETS`)
- [ ] **Done when:** GitHub MCP server tools list and call through the adapter in a test

### PLT-HAR-1 · Prompts and skills (read-only)
`D05` · WS1 · ~4 h · depends on: PR #1  
Python: `core/agent_harness/prompts/ (6.5k lines), infrastructure/skills_registry, infrastructure/harness_providers/{prompt_fragments,skill_prerequisites}.py`

- [ ] `SkillSource` port loading existing `SKILL.md` files and `opensre_system_prompt.md` **without modifying them**
- [ ] Prompt assembly (fragments, skill guidance, integration context)
- [ ] Skill prerequisites check
- [ ] **Done when:** Go-assembled system prompt equals Python's for the same config (golden test)

### PLT-HAR-2 · Context budget parity
`D05` · WS1 · ~2 h · depends on: PR #1  
Python: `core/context_budget.py`

- [ ] Token estimate per model, ceiling scaling from provider counts (0.25–1.5), overhead for system + tools
- [ ] **Done when:** Budget decisions match Python on recorded transcripts

### PLT-PRC-1 · Process boot profiles
`D05` · WS1 · ~2 h · depends on: PR #1  
Python: `infrastructure/process/boot.py, surfaces/shared/process_boot.py, gateway/core/process/boot.py`

- [ ] Profiles: CLI, gateway, web, scheduler worker, scheduled command, embedded
- [ ] Fixed step order; one capability registrar list
- [ ] **Done when:** Each entrypoint boots through one profile

## 5. Integrations

Each `INT-<vendor>` task: port the client, config, verifier and every listed tool, then pass the
definition of done. Env vars are those found in `config/constants/<vendor>.py` or read directly
in the vendor folder; vendors with none listed reuse shared credentials (for example AWS) or
take config from the integration store.

### D06 · Week 2 · Mon 30 Nov · Observability core (~20 h)

#### INT-datadog · WS2 · 6 tools · 1553 lines · ~6.6 h
Python: `integrations/datadog/` · has verifier
Env: `DD_API_KEY`, `DD_APP_KEY`, `DD_SITE`

- [ ] Client + config + verifier
- [ ] `query_datadog_all` (read-only)
- [ ] `query_datadog_events` (read-only)
- [ ] `query_datadog_logs` (read-only)
- [ ] `query_datadog_metrics` (read-only)
- [ ] `query_datadog_monitors` (read-only)
- [ ] `get_pods_on_node` (read-only)
- [ ] httptest suite + docs page

#### INT-grafana · WS2 · 6 tools · 2597 lines · ~8.3 h
Python: `integrations/grafana/` · has verifier, alert-source detector
Env: `GRAFANA_INSTANCE_URL`, `GRAFANA_READ_TOKEN`, `GRAFANA_VERIFY_SSL`, `GRAFANA_CA_BUNDLE`, `GRAFANA_LOKI_DATASOURCE_UID`, `GRAFANA_TEMPO_DATASOURCE_UID`, `GRAFANA_MIMIR_DATASOURCE_UID`

- [ ] Client + config + verifier
- [ ] `query_grafana_logs` (read-only)
- [ ] `query_grafana_alert_rules` (read-only)
- [ ] `query_grafana_annotations` (read-only)
- [ ] `query_grafana_metrics` (read-only)
- [ ] `query_grafana_traces` (read-only)
- [ ] `query_grafana_service_names` (read-only)
- [ ] httptest suite + docs page

#### INT-opensre · WS2 · 0 tools · 489 lines · ~2 h
Python: `integrations/opensre/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.opensre`)
- [ ] httptest suite + docs page

#### INT-tempo · WS2 · 1 tools · 758 lines · ~2.8 h
Python: `integrations/tempo/` · has verifier
Env: `TEMPO_URL`, `TEMPO_API_KEY`, `TEMPO_USERNAME`, `TEMPO_PASSWORD`, `TEMPO_ORG_ID`

- [ ] Client + config + verifier
- [ ] `query_tempo` (read-only)
- [ ] httptest suite + docs page

### D07 · Week 2 · Tue 1 Dec · Logs, APM, errors (~22 h)

#### INT-sentry · WS2 · 5 tools · 2773 lines · ~8.1 h
Python: `integrations/sentry/` · has verifier
Env: `SENTRY_URL`, `SENTRY_ORG_SLUG`, `SENTRY_AUTH_TOKEN`, `SENTRY_PROJECT_SLUG`, `SENTRY_STATS_PERIOD`

- [ ] Client + config + verifier
- [ ] `list_sentry_issue_events` (read-only)
- [ ] `list_sentry_uptime_alerts` (read-only)
- [ ] `get_sentry_issue_details` (read-only)
- [ ] `search_sentry_issues` (read-only)
- [ ] `get_sentry_uptime_digest` (read-only)
- [ ] httptest suite + docs page

#### INT-cloudwatch · WS2 · 2 tools · 299 lines · ~2.5 h
Python: `integrations/cloudwatch/`
Env: –

- [ ] Client + config
- [ ] `get_cloudwatch_logs` (read-only)
- [ ] `get_cloudwatch_batch_metrics` (read-only)
- [ ] httptest suite + docs page

#### INT-elasticsearch · WS2 · 1 tools · 630 lines · ~2.5 h
Python: `integrations/elasticsearch/`
Env: –

- [ ] Client + config
- [ ] `query_elasticsearch_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-opensearch · WS2 · 1 tools · 311 lines · ~2.0 h
Python: `integrations/opensearch/` · has verifier
Env: `OPENSEARCH_URL`, `OPENSEARCH_API_KEY`, `OPENSEARCH_USERNAME`, `OPENSEARCH_PASSWORD`

- [ ] Client + config + verifier
- [ ] `query_opensearch_analytics` (read-only)
- [ ] httptest suite + docs page

#### INT-new_relic · WS2 · 2 tools · 1241 lines · ~4.1 h
Python: `integrations/new_relic/` · has verifier
Env: `NEW_RELIC_API_KEY`, `NEW_RELIC_ACCOUNT_ID`, `NEW_RELIC_API_URL`, `NEW_RELIC_INSTANCES`

- [ ] Client + config + verifier
- [ ] `query_new_relic_metrics` (read-only)
- [ ] `query_new_relic_alerts` (read-only)
- [ ] httptest suite + docs page

#### INT-honeycomb · WS2 · 1 tools · 578 lines · ~2.5 h
Python: `integrations/honeycomb/` · has verifier
Env: `HONEYCOMB_API_KEY`, `HONEYCOMB_DATASET`, `HONEYCOMB_API_URL`

- [ ] Client + config + verifier
- [ ] `query_honeycomb_traces` (read-only)
- [ ] httptest suite + docs page

### D08 · Week 2 · Wed 2 Dec · Kubernetes and AWS (~50 h)

#### INT-kubernetes · WS2 · 12 tools · 2832 lines · ~11.7 h
Python: `integrations/kubernetes/` · has verifier
Env: `KUBECONFIG`, `KUBECONFIG_CONTENT`, `KUBECONFIG_CONTEXT`, `KUBECONFIG_NAMESPACE`, `KUBECONFIG_NAMESPACES`

- [ ] Client + config + verifier
- [ ] `kubernetes_list_deployments` (read-only)
- [ ] `kubernetes_describe_pod` (read-only)
- [ ] `kubernetes_list_services` (read-only)
- [ ] `kubernetes_list_configmaps` (read-only)
- [ ] `kubernetes_list_ingresses` (read-only)
- [ ] `kubernetes_list_pods` (read-only)
- [ ] `kubernetes_list_statefulsets` (read-only)
- [ ] `kubernetes_get_resource` (read-only)
- [ ] `kubernetes_get_pod_logs` (read-only)
- [ ] `kubernetes_list_daemonsets` (read-only)
- [ ] `kubernetes_get_events` (read-only)
- [ ] `kubernetes_list_nodes` (read-only)
- [ ] httptest suite + docs page

#### INT-eks · WS2 · 11 tools · 1732 lines · ~9.4 h
Python: `integrations/eks/`
Env: –

- [ ] Client + config
- [ ] `get_eks_events` (read-only)
- [ ] `list_eks_deployments` (read-only)
- [ ] `get_eks_deployment_status` (read-only)
- [ ] `list_eks_clusters` (read-only)
- [ ] `describe_eks_cluster` (read-only)
- [ ] `get_eks_nodegroup_health` (read-only)
- [ ] `get_eks_node_health` (read-only)
- [ ] `list_eks_namespaces` (read-only)
- [ ] `describe_eks_addon` (read-only)
- [ ] `get_eks_pod_logs` (read-only)
- [ ] `list_eks_pods` (read-only)
- [ ] httptest suite + docs page

#### INT-helm · WS2 · 5 tools · 1044 lines · ~5.2 h
Python: `integrations/helm/` · has verifier
Env: `OSRE_HELM_INTEGRATION`, `HELM_PATH`, `HELM_KUBE_CONTEXT`, `HELM_KUBECONFIG`, `HELM_NAMESPACE`, `HELM_MANIFEST_MAX_CHARS`

- [ ] Client + config + verifier
- [ ] `helm_list_releases` (read-only)
- [ ] `helm_release_status` (read-only)
- [ ] `helm_release_history` (read-only)
- [ ] `helm_get_release_values` (read-only)
- [ ] `helm_get_release_manifest` (read-only)
- [ ] httptest suite + docs page

#### INT-aws · WS2 · 1 tools · 2252 lines · ~5.3 h
Python: `integrations/aws/` · has verifier
Env: `AWS_ROLE_ARN`, `AWS_EXTERNAL_ID`, `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`

- [ ] Client + config + verifier
- [ ] `execute_aws_operation` (**mutating → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-ec2 · WS2 · 1 tools · 238 lines · ~1.9 h
Python: `integrations/ec2/`
Env: –

- [ ] Client + config
- [ ] `ec2_instances_by_tag` (read-only)
- [ ] httptest suite + docs page

#### INT-elb · WS2 · 1 tools · 252 lines · ~1.9 h
Python: `integrations/elb/`
Env: –

- [ ] Client + config
- [ ] `get_elb_target_health` (read-only)
- [ ] httptest suite + docs page

#### INT-aws_lambda · WS2 · 4 tools · 323 lines · ~3.5 h
Python: `integrations/aws_lambda/`
Env: –

- [ ] Client + config
- [ ] `inspect_lambda_function` (read-only)
- [ ] `get_lambda_configuration` (read-only)
- [ ] `get_lambda_errors` (read-only)
- [ ] `get_lambda_invocation_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-s3 · WS2 · 4 tools · 278 lines · ~3.5 h
Python: `integrations/s3/`
Env: –

- [ ] Client + config
- [ ] `inspect_s3_object` (read-only)
- [ ] `check_s3_marker` (read-only)
- [ ] `get_s3_object` (read-only)
- [ ] `list_s3_objects` (read-only)
- [ ] httptest suite + docs page

#### INT-sqs · WS2 · 1 tools · 360 lines · ~2.1 h
Python: `integrations/sqs/`
Env: –

- [ ] Client + config
- [ ] `get_sqs_queue_attributes` (read-only)
- [ ] httptest suite + docs page

#### INT-rds · WS2 · 2 tools · 497 lines · ~2.8 h
Python: `integrations/rds/`
Env: `RDS_DB_INSTANCE_IDENTIFIER`, `RDS_REGION`

- [ ] Client + config
- [ ] `describe_rds_instance` (read-only)
- [ ] `describe_rds_events` (read-only)
- [ ] httptest suite + docs page

#### INT-cloudtrail · WS2 · 1 tools · 406 lines · ~2.2 h
Python: `integrations/cloudtrail/`
Env: –

- [ ] Client + config
- [ ] `lookup_cloudtrail_events` (read-only)
- [ ] httptest suite + docs page

### D09 · Week 2 · Thu 3 Dec · Incident and alert sources (~21 h)

#### INT-pagerduty · WS2 · 4 tools · 1240 lines · ~5.1 h
Python: `integrations/pagerduty/` · has verifier
Env: `PAGERDUTY_API_KEY`, `PAGERDUTY_BASE_URL`

- [ ] Client + config + verifier
- [ ] `pagerduty_incident_detail` (read-only)
- [ ] `pagerduty_incidents` (read-only)
- [ ] `pagerduty_oncall` (read-only)
- [ ] `pagerduty_services` (read-only)
- [ ] httptest suite + docs page

#### INT-incident_io · WS2 · 1 tools · 862 lines · ~2.9 h
Python: `integrations/incident_io/` · has verifier
Env: `INCIDENT_IO_API_KEY`, `INCIDENT_IO_BASE_URL`

- [ ] Client + config + verifier
- [ ] `incident_io_incidents` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-alertmanager · WS2 · 2 tools · 771 lines · ~3.3 h
Python: `integrations/alertmanager/` · has verifier
Env: `ALERTMANAGER_URL`, `ALERTMANAGER_BEARER_TOKEN`, `ALERTMANAGER_USERNAME`, `ALERTMANAGER_PASSWORD`

- [ ] Client + config + verifier
- [ ] `alertmanager_alerts` (read-only)
- [ ] `alertmanager_silences` (read-only)
- [ ] httptest suite + docs page

#### INT-tracer · WS2 · 12 tools · 1644 lines · ~9.7 h
Python: `integrations/tracer/` · has verifier
Env: `TRACER_API_URL`, `JWT_TOKEN`, `AIRFLOW_DAG_ID`

- [ ] Client + config + verifier
- [ ] `get_failed_jobs` (read-only)
- [ ] `get_error_logs` (read-only)
- [ ] `get_tracer_run` (read-only)
- [ ] `get_airflow_metrics` (read-only)
- [ ] `fetch_failed_run` (read-only)
- [ ] `get_batch_statistics` (read-only)
- [ ] `get_tracer_tasks` (read-only)
- [ ] `get_recent_airflow_failures` (read-only)
- [ ] `get_airflow_dag_runs` (read-only)
- [ ] `get_airflow_task_instances` (read-only)
- [ ] `get_host_metrics` (read-only)
- [ ] `get_failed_tools` (read-only)
- [ ] httptest suite + docs page

### D11 · Week 3 · Mon 7 Dec · Code hosts and ticketing (~75 h)

#### INT-github · WS3 · 34 tools · 26248 lines · ~57 h (split below)
Env: `GITHUB_MCP_MODE`, `GITHUB_MCP_URL`, `GITHUB_MCP_COMMAND`, `GITHUB_MCP_ARGS`, `GITHUB_MCP_AUTH_TOKEN`, `GITHUB_MCP_TOOLSETS`, `GITHUB_TOKEN`, `GH_TOKEN`, `OPENSRE_INSTALL_GITHUB_TOKEN`, `OPENSRE_GITHUB_MCP_REPO_PROBE_LIMIT`, `OPENSRE_GITHUB_MCP_VERIFY_ORGS`, `GITHUB_EVENT_PATH`, `GITHUB_REPOSITORY`

- [ ] **GH-1** Connection layer: MCP mode (`GITHUB_MCP_*`), REST token, GitHub App connection, identity, rate limits, accessible repos, repo scope; verifier; runbooks loader — `integrations/github/{mcp,rest_token,app_connection,connections,identity,rate_limit,accessible_repos,repo_scope,client,verifier,cli_setup}.py, runbooks/` · ~8 h · after PLT-MCP-1
- [ ] **GH-2** Repository, code, commits, issues read tools — `integrations/github/tools/*.py (top level)` · ~5 h · after GH-1
  - [ ] `get_github_repository` (read-only)
  - [ ] `get_github_repository_tree` (read-only)
  - [ ] `get_github_file_contents` (read-only)
  - [ ] `search_github_code` (read-only)
  - [ ] `list_github_commits` (read-only)
  - [ ] `search_github_issues` (read-only)
  - [ ] `list_github_work_items` (read-only)
  - [ ] `summarize_github_pr_status` (read-only)
  - [ ] `list_github_security_alerts` (read-only)
  - [ ] `get_github_star_history` (read-only)
- [ ] **GH-3** Actions read tools — `integrations/github/tools/*.py` · ~3 h · after GH-1
  - [ ] `list_github_actions_workflow_runs` (read-only)
  - [ ] `list_github_actions_active_runs` (read-only)
  - [ ] `list_github_actions_run_jobs` (read-only)
  - [ ] `get_github_actions_step_log` (read-only)
- [ ] **GH-4** Issue mutation flow (propose → approve → execute) — `integrations/github/tools/*.py` · ~3 h · after GH-1, PLT-SAF-1
  - [ ] `propose_github_issue_mutation_from_slack` (read-only)
  - [ ] `execute_github_issue_mutation` (**mutating → ApprovalGate**)
- [ ] **GH-5** Deploy timeline, community follow-ups, work status report — `tools/git_deploy_timeline_tool, community_followup_tool, work_status_report_tool` · ~3 h · after GH-2
  - [ ] `get_git_deploy_timeline` (read-only)
  - [ ] `summarize_community_followups` (read-only)
  - [ ] `generate_work_status_report` (read-only)
- [ ] **GH-6** CI health scan + CI reliability analytics (3.8k lines) + scheduled loop — `tools/ci_health_scan, tools/ci_analytics, ci_health_runner.py, ci_epochs.py` · ~8 h · after GH-3, PLT-SCH-1
  - [ ] `scan_github_ci_health` (read-only)
  - [ ] `analyze_github_ci_reliability` (read-only)
  - [ ] `schedule_ci_reliability_loop` (**mutating → ApprovalGate**)
- [ ] **GH-7** CI fix: storage (prepared_pushes, unsettled_merge_revisions), runner, push flow — `tools/ci_fix (3.5k lines), pull_requests.py, pull_request_checks.py, pull_request_checkout.py, repair_workspace.py, workspace_credentials.py` · ~10 h · after GH-3, PLT-DB-1, PLT-SAF-1
  - [ ] `fix_github_pr_ci` (**mutating → ApprovalGate**)
- [ ] **GH-8** CI repair loop, run, demo, repair-access probe; hidden `ci-repair-worker` command — `tools/ci_repair_loop, ci_repair_run, ci_repair_demo, repair_access, pr_sweep_runner.py, repair_outcomes.py` · ~8 h · after GH-7
  - [ ] `schedule_ci_repair_loop` (**mutating → ApprovalGate**)
  - [ ] `get_ci_repair_loop` (read-only)
  - [ ] `run_ci_repair_demo` (**mutating → ApprovalGate**)
  - [ ] `seed_ci_repair_demo` (**mutating → ApprovalGate**)
  - [ ] `finish_ci_repair_demo` (**mutating → ApprovalGate**)
  - [ ] `probe_github_repair_access` (read-only)
- [ ] **GH-9** Security alert fix (1.9k lines) — `tools/security_fix` · ~5 h · after GH-2, PLT-SAF-1
  - [ ] `fix_github_security_alert` (**mutating → ApprovalGate**)
- [ ] **GH-10** Architecture issue tool (clone, observe, cleanup) and `gh` CLI wrapper — `tools/architecture_issue_tool, tools/github_cli` · ~4 h · after GH-1, PLT-SAF-2
  - [ ] `architecture_clone_repo` (**mutating → ApprovalGate**)
  - [ ] `architecture_save_observations` (**mutating → ApprovalGate**)
  - [ ] `architecture_cleanup_repo` (**mutating → ApprovalGate**)
  - [ ] `github_cli` (**mutating → ApprovalGate**)

#### INT-gitlab · WS3 · 4 tools · 1193 lines · ~5.0 h
Python: `integrations/gitlab/` · has verifier
Env: `GITLAB_BASE_URL`, `GITLAB_ACCESS_TOKEN`

- [ ] Client + config + verifier
- [ ] `list_gitlab_mrs` (read-only)
- [ ] `get_gitlab_file` (read-only)
- [ ] `list_gitlab_pipelines` (read-only)
- [ ] `list_gitlab_commits` (read-only)
- [ ] httptest suite + docs page

#### INT-bitbucket · WS3 · 3 tools · 766 lines · ~3.8 h
Python: `integrations/bitbucket/` · has verifier
Env: `BITBUCKET_WORKSPACE`, `BITBUCKET_USERNAME`, `BITBUCKET_APP_PASSWORD`, `BITBUCKET_BASE_URL`

- [ ] Client + config + verifier
- [ ] `list_bitbucket_commits` (read-only)
- [ ] `search_bitbucket_code` (read-only)
- [ ] `get_bitbucket_file_contents` (read-only)
- [ ] httptest suite + docs page

#### INT-git · WS3 · 0 tools · 2028 lines · ~4.4 h
Python: `integrations/git/`
Env: `GIT_TERMINAL_PROMPT`, `GIT_ALLOW_PROTOCOL`, `GIT_OPTIONAL_LOCKS`

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.git`)
- [ ] httptest suite + docs page

#### INT-jira · WS3 · 4 tools · 852 lines · ~4.4 h
Python: `integrations/jira/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `jira_add_comment` (**external side effect → ApprovalGate**)
- [ ] `jira_create_issue` (**external side effect → ApprovalGate**)
- [ ] `jira_issue_detail` (read-only)
- [ ] `jira_search_issues` (read-only)
- [ ] httptest suite + docs page

### D12 · Week 3 · Tue 8 Dec · Databases and queues (~50 h)

#### INT-postgresql · WS3 · 6 tools · 1252 lines · ~6.1 h
Python: `integrations/postgresql/` · has verifier
Env: `POSTGRESQL_HOST`, `POSTGRESQL_PORT`, `POSTGRESQL_DATABASE`, `POSTGRESQL_USERNAME`, `POSTGRESQL_PASSWORD`, `POSTGRESQL_SSL_MODE`

- [ ] Client + config + verifier
- [ ] `get_postgresql_replication_status` (read-only)
- [ ] `get_postgresql_table_stats` (read-only)
- [ ] `get_postgresql_lock_status` (read-only)
- [ ] `get_postgresql_server_status` (read-only)
- [ ] `get_postgresql_current_queries` (read-only)
- [ ] `get_postgresql_slow_queries` (read-only)
- [ ] httptest suite + docs page

#### INT-mysql · WS3 · 5 tools · 1056 lines · ~5.3 h
Python: `integrations/mysql/` · has verifier
Env: `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_DATABASE`, `MYSQL_USERNAME`, `MYSQL_PASSWORD`, `MYSQL_SSL_MODE`

- [ ] Client + config + verifier
- [ ] `get_mysql_slow_queries` (read-only)
- [ ] `get_mysql_current_processes` (read-only)
- [ ] `get_mysql_table_stats` (read-only)
- [ ] `get_mysql_replication_status` (read-only)
- [ ] `get_mysql_server_status` (read-only)
- [ ] httptest suite + docs page

#### INT-mariadb · WS3 · 5 tools · 1012 lines · ~5.2 h
Python: `integrations/mariadb/` · has verifier
Env: `MARIADB_HOST`, `MARIADB_PORT`, `MARIADB_DATABASE`, `MARIADB_USERNAME`, `MARIADB_PASSWORD`, `MARIADB_SSL`

- [ ] Client + config + verifier
- [ ] `get_mariadb_innodb_status` (read-only)
- [ ] `get_mariadb_process_list` (read-only)
- [ ] `get_mariadb_replication_status` (read-only)
- [ ] `get_mariadb_slow_queries` (read-only)
- [ ] `get_mariadb_global_status` (read-only)
- [ ] httptest suite + docs page

#### INT-azure_sql · WS3 · 5 tools · 1026 lines · ~5.2 h
Python: `integrations/azure_sql/` · has verifier
Env: `AZURE_SQL_SERVER`, `AZURE_SQL_DATABASE`, `AZURE_SQL_PORT`, `AZURE_SQL_USERNAME`, `AZURE_SQL_PASSWORD`, `AZURE_SQL_DRIVER`, `AZURE_SQL_ENCRYPT`

- [ ] Client + config + verifier
- [ ] `get_azure_sql_slow_queries` (read-only)
- [ ] `get_azure_sql_current_queries` (read-only)
- [ ] `get_azure_sql_server_status` (read-only)
- [ ] `get_azure_sql_wait_stats` (read-only)
- [ ] `get_azure_sql_resource_stats` (read-only)
- [ ] httptest suite + docs page

#### INT-redis · WS3 · 7 tools · 1181 lines · ~6.5 h
Python: `integrations/redis/` · has verifier
Env: `REDIS_HOST`, `REDIS_PORT`, `REDIS_USERNAME`, `REDIS_PASSWORD`, `REDIS_DATABASE`, `REDIS_SSL`

- [ ] Client + config + verifier
- [ ] `get_redis_slowlog` (read-only)
- [ ] `scan_redis_keys` (read-only)
- [ ] `get_redis_server_info` (read-only)
- [ ] `get_redis_replication` (read-only)
- [ ] `get_redis_list_depth` (read-only)
- [ ] `get_redis_client_list` (read-only)
- [ ] `get_redis_latency_doctor` (read-only)
- [ ] httptest suite + docs page

#### INT-mongodb · WS3 · 5 tools · 892 lines · ~5.0 h
Python: `integrations/mongodb/` · has verifier
Env: `MONGODB_CONNECTION_STRING`, `MONGODB_DATABASE`, `MONGODB_AUTH_SOURCE`, `MONGODB_TLS`

- [ ] Client + config + verifier
- [ ] `get_mongodb_profiler_data` (read-only)
- [ ] `get_mongodb_current_ops` (read-only)
- [ ] `get_mongodb_server_status` (read-only)
- [ ] `get_mongodb_replica_status` (read-only)
- [ ] `get_mongodb_collection_stats` (read-only)
- [ ] httptest suite + docs page

#### INT-mongodb_atlas · WS3 · 5 tools · 1008 lines · ~5.2 h
Python: `integrations/mongodb_atlas/` · has verifier
Env: `MONGODB_ATLAS_PUBLIC_KEY`, `MONGODB_ATLAS_PRIVATE_KEY`, `MONGODB_ATLAS_PROJECT_ID`, `MONGODB_ATLAS_BASE_URL`

- [ ] Client + config + verifier
- [ ] `get_mongodb_atlas_alerts` (read-only)
- [ ] `get_mongodb_atlas_performance_advisor` (read-only)
- [ ] `get_mongodb_atlas_cluster_metrics` (read-only)
- [ ] `get_mongodb_atlas_clusters` (read-only)
- [ ] `get_mongodb_atlas_cluster_events` (read-only)
- [ ] httptest suite + docs page

#### INT-clickhouse · WS3 · 2 tools · 1072 lines · ~3.8 h
Python: `integrations/clickhouse/` · has verifier
Env: `CLICKHOUSE_HOST`, `CLICKHOUSE_PORT`, `CLICKHOUSE_DATABASE`, `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`, `CLICKHOUSE_SECURE`

- [ ] Client + config + verifier
- [ ] `get_clickhouse_query_activity` (read-only)
- [ ] `get_clickhouse_system_health` (read-only)
- [ ] httptest suite + docs page

#### INT-kafka · WS3 · 2 tools · 507 lines · ~2.8 h
Python: `integrations/kafka/` · has verifier
Env: `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_SECURITY_PROTOCOL`, `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD`

- [ ] Client + config + verifier
- [ ] `get_kafka_consumer_group_lag` (read-only)
- [ ] `get_kafka_topic_health` (read-only)
- [ ] httptest suite + docs page

#### INT-rabbitmq · WS3 · 5 tools · 1011 lines · ~5.2 h
Python: `integrations/rabbitmq/` · has verifier
Env: `RABBITMQ_HOST`, `RABBITMQ_MANAGEMENT_PORT`, `RABBITMQ_USERNAME`, `RABBITMQ_PASSWORD`, `RABBITMQ_VHOST`, `RABBITMQ_SSL`, `RABBITMQ_VERIFY_SSL`

- [ ] Client + config + verifier
- [ ] `get_rabbitmq_broker_overview` (read-only)
- [ ] `get_rabbitmq_queue_backlog` (read-only)
- [ ] `get_rabbitmq_connection_stats` (read-only)
- [ ] `get_rabbitmq_consumer_health` (read-only)
- [ ] `get_rabbitmq_node_health` (read-only)
- [ ] httptest suite + docs page

### D13 · Week 3 · Wed 9 Dec · Pipelines, deploy, remaining observability (~64 h)

#### INT-airflow · WS1 · 0 tools · 345 lines · ~1.6 h
Python: `integrations/airflow/`
Env: `AIRFLOW_USERNAME`, `AIRFLOW_BASE_URL`, `AIRFLOW_TIMEOUT_SECONDS`, `AIRFLOW_VERIFY_SSL`, `AIRFLOW_MAX_RESULTS`

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.airflow`)
- [ ] httptest suite + docs page

#### INT-dagster · WS1 · 5 tools · 985 lines · ~5.1 h
Python: `integrations/dagster/` · has verifier
Env: `DAGSTER_ENDPOINT`, `DAGSTER_API_TOKEN`

- [ ] Client + config + verifier
- [ ] `list_dagster_assets` (read-only)
- [ ] `get_dagster_run_logs` (read-only)
- [ ] `list_dagster_runs` (read-only)
- [ ] `list_dagster_schedule_ticks` (read-only)
- [ ] `list_dagster_sensor_ticks` (read-only)
- [ ] httptest suite + docs page

#### INT-prefect · WS1 · 2 tools · 833 lines · ~3.4 h
Python: `integrations/prefect/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `prefect_flow_runs` (read-only)
- [ ] `prefect_worker_health` (read-only)
- [ ] httptest suite + docs page

#### INT-temporal · WS1 · 4 tools · 1032 lines · ~4.7 h
Python: `integrations/temporal/` · has verifier
Env: `TEMPORAL_API_URL`, `TEMPORAL_NAMESPACE`, `TEMPORAL_API_KEY`

- [ ] Client + config + verifier
- [ ] `temporal_namespace_info` (read-only)
- [ ] `temporal_task_queue` (read-only)
- [ ] `temporal_workflow_history` (read-only)
- [ ] `temporal_workflows` (read-only)
- [ ] httptest suite + docs page

#### INT-jenkins · WS1 · 5 tools · 1260 lines · ~5.6 h
Python: `integrations/jenkins/` · has verifier
Env: `JENKINS_URL`, `JENKINS_USER`, `JENKINS_API_TOKEN`

- [ ] Client + config + verifier
- [ ] `list_jenkins_builds` (read-only)
- [ ] `get_jenkins_build_log` (read-only)
- [ ] `get_jenkins_pipeline_stages` (read-only)
- [ ] `list_jenkins_jobs` (read-only)
- [ ] `list_jenkins_running_builds` (read-only)
- [ ] httptest suite + docs page

#### INT-argocd · WS1 · 2 tools · 939 lines · ~3.6 h
Python: `integrations/argocd/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `argocd_application_diff` (read-only)
- [ ] `argocd_application_status` (read-only)
- [ ] httptest suite + docs page

#### INT-signoz · WS4 · 3 tools · 1485 lines · ~5.0 h
Python: `integrations/signoz/` · has verifier
Env: `SIGNOZ_URL`, `SIGNOZ_API_KEY`

- [ ] Client + config + verifier
- [ ] `query_signoz_traces` (read-only)
- [ ] `query_signoz_logs` (read-only)
- [ ] `query_signoz_metrics` (read-only)
- [ ] httptest suite + docs page

#### INT-groundcover · WS4 · 3 tools · 1331 lines · ~4.7 h
Python: `integrations/groundcover/` · has verifier
Env: `GROUNDCOVER_API_KEY`, `GROUNDCOVER_MCP_TOKEN`, `GROUNDCOVER_MCP_URL`, `GROUNDCOVER_TENANT_UUID`, `GROUNDCOVER_BACKEND_ID`, `GROUNDCOVER_TIMEZONE`

- [ ] Client + config + verifier
- [ ] `query_groundcover_logs` (read-only)
- [ ] `get_groundcover_query_reference` (read-only)
- [ ] `query_groundcover_traces` (read-only)
- [ ] httptest suite + docs page

#### INT-coralogix · WS4 · 1 tools · 606 lines · ~2.5 h
Python: `integrations/coralogix/` · has verifier
Env: `CORALOGIX_API_KEY`, `CORALOGIX_API_URL`, `CORALOGIX_APPLICATION_NAME`, `CORALOGIX_SUBSYSTEM_NAME`

- [ ] Client + config + verifier
- [ ] `query_coralogix_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-splunk · WS4 · 1 tools · 446 lines · ~2.2 h
Python: `integrations/splunk/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `query_splunk_logs` (**mutating → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-betterstack · WS4 · 1 tools · 671 lines · ~2.6 h
Python: `integrations/betterstack/` · has verifier
Env: `BETTERSTACK_QUERY_ENDPOINT`, `BETTERSTACK_USERNAME`, `BETTERSTACK_PASSWORD`, `BETTERSTACK_SOURCES`

- [ ] Client + config + verifier
- [ ] `query_betterstack_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-victoria_logs · WS4 · 1 tools · 426 lines · ~2.2 h
Python: `integrations/victoria_logs/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `victoria_logs_query` (read-only)
- [ ] httptest suite + docs page

#### INT-openobserve · WS4 · 1 tools · 300 lines · ~2.0 h
Python: `integrations/openobserve/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `query_openobserve_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-posthog · WS4 · 0 tools · 592 lines · ~2.0 h
Python: `integrations/posthog/` · has verifier
Env: `POSTHOG_BASE_URL`, `POSTHOG_PROJECT_ID`, `POSTHOG_PERSONAL_API_KEY`, `POSTHOG_TIMEOUT_SECONDS`

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.posthog`)
- [ ] httptest suite + docs page

#### INT-vercel · WS4 · 2 tools · 1010 lines · ~3.7 h
Python: `integrations/vercel/` · has verifier
Env: `VERCEL_API_TOKEN`, `VERCEL_TEAM_ID`, `VERCEL_RUNTIME_LOGS_READ_TIMEOUT`

- [ ] Client + config + verifier
- [ ] `vercel_deployment_status` (read-only)
- [ ] `vercel_deployment_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-railway · WS4 · 1 tools · 486 lines · ~2.3 h
Python: `integrations/railway/` · has verifier
Env: `RAILWAY_TOKEN`, `RAILWAY_PROJECT`, `RAILWAY_SERVICE`, `RAILWAY_ENVIRONMENT`, `RAILWAY_PATH`

- [ ] Client + config + verifier
- [ ] `inspect_railway_deployment` (read-only)
- [ ] httptest suite + docs page

#### INT-supabase · WS4 · 2 tools · 492 lines · ~2.8 h
Python: `integrations/supabase/` · has verifier
Env: `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`

- [ ] Client + config + verifier
- [ ] `get_supabase_storage_buckets` (read-only)
- [ ] `get_supabase_service_health` (read-only)
- [ ] httptest suite + docs page

#### INT-neon · WS4 · 3 tools · 1149 lines · ~4.4 h
Python: `integrations/neon/` · has verifier
Env: `NEON_API_KEY`, `NEON_PROJECT_ID`, `NEON_BRANCH_ID`

- [ ] Client + config + verifier
- [ ] `get_neon_project_status` (read-only)
- [ ] `get_neon_storage_buckets` (read-only)
- [ ] `list_neon_storage_objects` (read-only)
- [ ] httptest suite + docs page

#### INT-azure · WS4 · 1 tools · 315 lines · ~2.0 h
Python: `integrations/azure/` · has verifier
Env: `AZURE_LOG_ANALYTICS_WORKSPACE_ID`, `AZURE_LOG_ANALYTICS_TOKEN`, `AZURE_LOG_ANALYTICS_ENDPOINT`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`, `AZURE_MAX_RESULTS`

- [ ] Client + config + verifier
- [ ] `query_azure_monitor_logs` (read-only)
- [ ] httptest suite + docs page

#### INT-snowflake · WS4 · 1 tools · 272 lines · ~2.0 h
Python: `integrations/snowflake/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `query_snowflake_history` (**mutating → ApprovalGate**)
- [ ] httptest suite + docs page

### D14 · Week 3 · Thu 10 Dec · Chat transports (tools + delivery) (~13 h)

#### INT-slack · WS4 · 9 tools · 4041 lines · ~8 h
Python: `integrations/slack/` · has verifier
Env: `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`, `SLACK_ACCESS_TOKEN`, `SLACK_DEFAULT_CHAT_ID`, `SLACK_WEBHOOK_URL`, `SLACK_GITHUB_ISSUES_WEBHOOK_URL`, `OPENSRE_SILO_TEAM_IDS`, `SLACK_TEAM_TASKS_LIST_ID`

- [ ] Client + config + verifier
- [ ] `slack_list_team_members` (read-only)
- [ ] `slack_join_channel` (**external side effect → ApprovalGate**)
- [ ] `slack_add_reaction` (**external side effect → ApprovalGate**)
- [ ] `slack_reply_message` (**external side effect → ApprovalGate**)
- [ ] `slack_search_messages` (read-only)
- [ ] `slack_read_list` (read-only)
- [ ] `slack_read_messages` (read-only)
- [ ] `slack_send_message` (**external side effect → ApprovalGate**)
- [ ] `replay_slack_thread_locally` (read-only)
- [ ] httptest suite + docs page

#### INT-telegram · WS4 · 1 tools · 1293 lines · ~3.7 h
Python: `integrations/telegram/` · has verifier
Env: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_DEFAULT_CHAT_ID`

- [ ] Client + config + verifier
- [ ] `telegram_send_message` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-discord · WS4 · 0 tools · 282 lines · ~1.5 h
Python: `integrations/discord/` · has verifier
Env: `DISCORD_BOT_TOKEN`, `DISCORD_APPLICATION_ID`, `DISCORD_PUBLIC_KEY`, `DISCORD_DEFAULT_CHANNEL_ID`, `DISCORD_ALLOWED_USERS`, `DISCORD_ALLOW_OPEN_GUILD`, `DISCORD_SILO_GUILD_IDS`

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.discord`)
- [ ] httptest suite + docs page

### D15 · Week 3 · Fri 11 Dec · Delivery channels (~4 h)

#### INT-smtp · WS4 · 0 tools · 470 lines · ~1.8 h
Python: `integrations/smtp/` · has verifier
Env: `SMTP_HOST`, `SMTP_PORT`, `SMTP_SECURITY`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_ADDRESS`, `SMTP_DEFAULT_TO`

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.smtp`)
- [ ] httptest suite + docs page

#### INT-scheduled_email · WS4 · 0 tools · 175 lines · ~1.3 h
Python: `integrations/scheduled_email/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.scheduled_email`)
- [ ] httptest suite + docs page

#### INT-morning_report · WS4 · 0 tools · 79 lines · ~1.1 h
Python: `integrations/morning_report/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.morning_report`)
- [ ] httptest suite + docs page

### T3 · Post-month · Tier 3 / stretch (~58 h)

#### INT-buzz · T3 · 1 tools · 857 lines · ~2.9 h
Python: `integrations/buzz/` · has verifier
Env: `BUZZ_PATH`, `BUZZ_PRIVATE_KEY`, `BUZZ_RELAY_URL`, `BUZZ_DEFAULT_CHANNEL`, `BUZZ_AUTH_TAG`

- [ ] Client + config + verifier
- [ ] `buzz_send_message` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-rocketchat · T3 · 1 tools · 941 lines · ~3.1 h
Python: `integrations/rocketchat/` · has verifier
Env: `ROCKETCHAT_SERVER_URL`, `ROCKETCHAT_AUTH_TOKEN`, `ROCKETCHAT_USER_ID`, `ROCKETCHAT_DEFAULT_CHANNEL`

- [ ] Client + config + verifier
- [ ] `rocketchat_send_message` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-whatsapp · T3 · 0 tools · 186 lines · ~1.3 h
Python: `integrations/whatsapp/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.whatsapp`)
- [ ] httptest suite + docs page

#### INT-twilio · T3 · 1 tools · 495 lines · ~2.3 h
Python: `integrations/twilio/` · has verifier
Env: `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_WHATSAPP_FROM`, `WHATSAPP_DEFAULT_TO`, `TWILIO_SMS_FROM`, `TWILIO_SMS_MESSAGING_SERVICE_SID`, `TWILIO_SMS_DEFAULT_TO`

- [ ] Client + config + verifier
- [ ] `twilio_notify` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-trello · T3 · 0 tools · 199 lines · ~1.3 h
Python: `integrations/trello/` · has verifier
Env: `TRELLO_BASE_URL`, `TRELLO_BOARD_ID`, `TRELLO_LIST_ID`

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.trello`)
- [ ] httptest suite + docs page

#### INT-servicenow · T3 · 0 tools · 217 lines · ~1.4 h
Python: `integrations/servicenow/` · has verifier
Env: `SERVICENOW_INSTANCE_URL`, `SERVICENOW_USERNAME`, `SERVICENOW_PASSWORD`

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.servicenow`)
- [ ] httptest suite + docs page

#### INT-google_docs · T3 · 1 tools · 738 lines · ~2.7 h
Python: `integrations/google_docs/` · has verifier
Env: `GOOGLE_CREDENTIALS_FILE`, `GOOGLE_DRIVE_FOLDER_ID`

- [ ] Client + config + verifier
- [ ] `create_google_docs_incident_report` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-pipedream · T3 · 2 tools · 669 lines · ~3.1 h
Python: `integrations/pipedream/`
Env: –

- [ ] Client + config
- [ ] `list_pipedream_tools` (read-only)
- [ ] `call_pipedream_tool` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-coding_agent · T3 · 0 tools · 1812 lines · ~4.0 h
Python: `integrations/coding_agent/`
Env: `CODING_AGENT_SANDBOX`, `CODEX_HOME`, `CODING_TIMEOUT_SECONDS`

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.coding_agent`)
- [ ] httptest suite + docs page

#### INT-llm_cli · T3 · 0 tools · 4913 lines · ~9.2 h
Python: `integrations/llm_cli/`
Env: `OPENAI_API_KEY`, `KIMI_API_KEY`, `KIMI_SHARE_DIR`, `PI_AGENT_DIR`, `PI_CONFIG_DIR`, `NPM_CONFIG_PREFIX`, `APPDATA`, `LOCALAPPDATA`, `VOLTA_HOME`, `PNPM_HOME`, `XDG_DATA_HOME`, `XAI_API_KEY`, `COPILOT_GH_HOST`, `GH_HOST`, `CURSOR_API_KEY`

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.llm_cli`)
- [ ] httptest suite + docs page

#### INT-pi · T3 · 1 tools · 562 lines · ~2.4 h
Python: `integrations/pi/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] `pi_coding_task` (**mutating → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-mcp_servers · T3 · 0 tools · 923 lines · ~2.5 h
Python: `integrations/mcp_servers/` · has verifier
Env: –

- [ ] Client + config + verifier
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.mcp_servers`)
- [ ] httptest suite + docs page

#### INT-sentry_mcp · T3 · 2 tools · 782 lines · ~3.3 h
Python: `integrations/sentry_mcp/` · has verifier
Env: `SENTRY_MCP_URL`, `SENTRY_MCP_AUTH_TOKEN`, `SENTRY_MCP_HOST`, `SENTRY_MCP_MODE`, `SENTRY_MCP_COMMAND`, `SENTRY_MCP_ARGS`, `SENTRY_MCP_ORGANIZATION_SLUG`, `SENTRY_MCP_PROJECT_SLUG`, `SENTRY_MCP_SKILLS`

- [ ] Client + config + verifier
- [ ] `list_sentry_tools` (read-only)
- [ ] `call_sentry_tool` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-posthog_mcp · T3 · 2 tools · 1396 lines · ~4.3 h
Python: `integrations/posthog_mcp/` · has verifier
Env: `POSTHOG_MCP_URL`, `POSTHOG_MCP_AUTH_TOKEN`, `POSTHOG_MCP_PROJECT_ID`, `POSTHOG_MCP_MODE`, `POSTHOG_MCP_COMMAND`, `POSTHOG_MCP_ARGS`, `POSTHOG_MCP_READ_ONLY`, `POSTHOG_MCP_ORGANIZATION_ID`, `POSTHOG_MCP_FEATURES`

- [ ] Client + config + verifier
- [ ] `list_posthog_tools` (read-only)
- [ ] `call_posthog_tool` (**external side effect → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-hosted_gateway · T3 · 5 tools · 1953 lines · ~6.8 h
Python: `integrations/hosted_gateway/`
Env: –

- [ ] Client + config
- [ ] `check_hosted_gateway` (read-only)
- [ ] `start_hosted_gateway` (**mutating → ApprovalGate**)
- [ ] `stop_hosted_gateway` (**mutating → ApprovalGate**)
- [ ] `ask_hosted_gateway` (**external side effect → ApprovalGate**)
- [ ] `cancel_hosted_gateway_prompt` (**mutating → ApprovalGate**)
- [ ] httptest suite + docs page

#### INT-knowledge_scans · T3 · 0 tools · 396 lines · ~1.7 h
Python: `integrations/knowledge_scans/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.knowledge_scans`)
- [ ] httptest suite + docs page

#### INT-webapp_knowledge · T3 · 0 tools · 690 lines · ~2.1 h
Python: `integrations/webapp_knowledge/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.webapp_knowledge`)
- [ ] httptest suite + docs page

#### INT-webapp_requests · T3 · 0 tools · 370 lines · ~1.6 h
Python: `integrations/webapp_requests/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.webapp_requests`)
- [ ] httptest suite + docs page

#### INT-webapp_scheduled_loops · T3 · 0 tools · 662 lines · ~2.1 h
Python: `integrations/webapp_scheduled_loops/`
Env: –

- [ ] Client + config
- [ ] No tools of its own: port the client/config its callers use (find them with `grep -rn integrations.webapp_scheduled_loops`)
- [ ] httptest suite + docs page

### Platform-owned integration folders

- [ ] `integrations/credentials_api` → PLT-CRD-1
- [ ] `integrations/verification` → PLT-CRD-1 (verifier framework)
- [ ] `integrations/setup` → CLI-W (wizard)

## 6. System and cross-vendor tools (D10, ~10 h)

- [ ] **TS-1** `tools/system/agent_memory` · after PLT-DOM-3
  - [ ] `memory_remember` (**mutating → ApprovalGate**)
  - [ ] `memory_forget` (**mutating → ApprovalGate**)
  - [ ] `memory_recall` (read-only)
- [ ] **TS-2** `tools/system/structured_file` · after –
  - [ ] `read_structured_file` (read-only)
- [ ] **TS-3** `tools/system/scheduled_loops` · after PLT-SCH-1
  - [ ] `schedule_prompt_loop` (**mutating → ApprovalGate**)
  - [ ] `run_scheduled_loop` (**mutating → ApprovalGate**)
  - [ ] `stop_scheduled_loop` (**mutating → ApprovalGate**)
  - [ ] `edit_scheduled_loop` (**mutating → ApprovalGate**)
  - [ ] `list_scheduled_loops` (read-only)
- [ ] **TS-4** `tools/system/work_items` · after PLT-DOM-3
  - [ ] `work_task_add` (**mutating → ApprovalGate**)
  - [ ] `work_task_list` (read-only)
  - [ ] `work_task_complete` (**mutating → ApprovalGate**)
  - [ ] `work_task_update` (**mutating → ApprovalGate**)
  - [ ] `work_task_prioritize` (read-only)
  - [ ] `work_task_schedule_checkin` (**mutating → ApprovalGate**)
- [ ] **TS-5** `tools/system/sre_guidance_tool` · after PLT-HAR-1
  - [ ] `get_sre_guidance` (read-only)
- [ ] **TS-6** `tools/system/repository_preferences` · after –
  - [ ] `repository_preferences` (**mutating → ApprovalGate**)
- [ ] **TS-7** `tools/system/workspace_git_scan` · after –
  - [ ] `scan_local_git_workspace` (read-only)
- [ ] **TS-8** `tools/system/runbook_guidance_tool` · after PLT-DOM-2
  - [ ] `load_runbook_guidance` (read-only)
- [ ] **TS-9** `tools/system/file_count` · after –
  - [ ] `count_files` (read-only)
- [ ] **TS-10** `tools/system/local_repo_insights` · after –
  - [ ] `analyze_local_repositories` (read-only)
- [ ] **TS-11** `tools/system/python_execution_tool` · after PLT-SAF-2
  - [ ] `execute_python_code` (**external side effect → ApprovalGate**)
- [ ] **TS-12** `tools/cross_vendor/fix_sentry_issue` · after INT-sentry, GH-7
  - [ ] `fix_sentry_issue` (**mutating → ApprovalGate**)
- [ ] **TS-13** `tools/cross_vendor/resolve_merge_conflicts` · after GH-7
  - [ ] `resolve_merge_conflicts` (**mutating → ApprovalGate**)

## 7. Harness, memory, sessions, scheduler

### PLT-DOM-3 · Agent memory and work items
`D10` · WS1 · ~6 h · depends on: PLT-DB-1  
Python: `core/domain/memory (3.6k lines), core/domain/work_items (1.3k lines)`

- [ ] `MemoryRepository` + `WorkItemRepository` ports and SQL adapters
- [ ] Memory recall ranking
- [ ] Work item scheduling/check-ins
- [ ] **Done when:** Memory and work tools (TS-*) pass against both DB backends

### PLT-HAR-3 · Turn orchestration and grounding
`D10` · WS1 · ~8 h · depends on: PLT-HAR-1, PLT-SAF-1  
Python: `core/agent_harness/turns/ (7.4k lines), grounding/, closed_llm_verdict.py, agent_builder.py`

- [ ] Turn lifecycle, cancellation, goals verification (`core/agent/goals.py`)
- [ ] Closed-LLM verdict
- [ ] Agent build config
- [ ] Tool discovery stagnation limits
- [ ] **Done when:** Python turn-level fixtures pass

### PLT-HAR-4 · Sessions, session goals, task plans
`D14` · WS4 · ~10 h · depends on: PLT-DB-3, PLT-HAR-3  
Python: `core/agent_harness/session (5.8k), session_goal (3.6k), task_plan (1.9k)`

- [ ] `SessionRepository` port; JSONL+WAL store or SQL per PLT-DB-3 decision
- [ ] Session goals and task plans persisted
- [ ] Transcript window and history settings (`core/state`)
- [ ] **Done when:** A multi-turn session survives a restart with goal and plan intact

### PLT-HAR-5 · Shared turn host
`D14` · WS4 · ~4 h · depends on: PLT-HAR-4  
Python: `infrastructure/turn_host/ (1.8k lines), infrastructure/process/turn_capacity`

- [ ] Single TurnRunner used by CLI, REPL and every gateway transport
- [ ] Session locks, concurrency caps, unattended sessions, status messages
- [ ] **Done when:** CLI and Slack run turns through the same runner (wiring test)

### PLT-FS-1 · File storage
`D15` · WS4 · ~4 h · depends on: PLT-CRD-1  
Python: `infrastructure/filestorage/ (3.7k lines)`

- [ ] Storage providers, exclusions, exposure rules, syncable exports
- [ ] **Done when:** Exports written by Go are readable by Python and vice versa during migration

### PLT-SCH-1 · Scheduler core
`D15` · WS4 · ~10 h · depends on: PLT-DB-1, PLT-HAR-5  
Python: `infrastructure/scheduling/ (10.3k lines)`

- [ ] Task registry and task types
- [ ] Run store (`task_runs`), backlog status, background service
- [ ] Cron parsing; scheduled loops (`tools/system/scheduled_loops`)
- [ ] Daily update, morning report, scheduled email runners
- [ ] `opensre cron add|list|status|remove|run|logs|start`
- [ ] **Done when:** A scheduled task runs, records a `task_runs` row, and delivers its report

## 8. Gateway

### GW-1 · Gateway core
`D14` · WS4 · ~8 h · depends on: PLT-HAR-5  
Python: `gateway/core/{lifecycle,prompt_intake,attachments,config,process,session}/`

- [ ] Prompt intake (2.1k lines)
- [ ] Lifecycle (start/stop/status/logs)
- [ ] Attachments
- [ ] Thread history and chat context
- [ ] **Done when:** Gateway starts with no transports configured and serves health

### GW-2 · Gateway middleware
`D14` · WS4 · ~5 h · depends on: GW-1, PLT-SAF-1  
Python: `gateway/core/middleware/ (inbound_decision, identity_policy, approvals, conversation_locks, active_turns, attention, terminal_outcome)`

- [ ] Each middleware step as a composable handler
- [ ] Approval UI hook per transport
- [ ] **Done when:** Middleware order and decisions match Python fixtures

### GW-3 · Gateway storage
`D14` · WS4 · ~4 h · depends on: PLT-DB-1  
Python: `gateway/core/storage/ (events, feedback, session bindings, security audit)`

- [ ] `slack_handled_events` dedupe
- [ ] `gateway_session_bindings`
- [ ] Feedback and security audit tables
- [ ] **Done when:** Duplicate Slack events are ignored across restarts

### GW-4 · Slack transport
`D14` · WS4 · ~8 h · depends on: GW-2, GW-3  
Python: `gateway/transports/slack/ (4.2k lines)`

- [ ] Events API + signature verification
- [ ] Turn stack, processing, delivery
- [ ] Interactive approvals (buttons)
- [ ] Settings and startup
- [ ] **Done when:** Mention → threaded answer with tool progress and approval buttons on staging

### GW-5 · Telegram transport
`D14` · WS4 · ~4 h · depends on: GW-2  
Python: `gateway/transports/telegram/ (1.6k lines)`

- [ ] Webhook/polling, pairing (`opensre messaging pair|allow|revoke|status`)
- [ ] **Done when:** Paired chat gets answers

### GW-6 · Discord transport
`D14` · WS4 · ~4 h · depends on: GW-2  
Python: `gateway/transports/discord/ (1.9k lines)`

- [ ] Gateway connection, message handling, delivery
- [ ] **Done when:** Channel mention gets answers

### GW-7 · Gateway web routes and billing
`D15` · WS4 · ~3 h · depends on: GW-1  
Python: `gateway/web/, gateway/core/billing/`

- [ ] `GET /`, `/health`, `/ok`, `/readyz`
- [ ] `GET /v1/prompt`, `POST /v1/prompt`
- [ ] `POST /v1/loops/sync`
- [ ] Billing/credits client
- [ ] **Done when:** Route table matches Python; auth uses the shared trust boundary

### GW-8 · Buzz transport
`T3` · T3 · ~4 h · depends on: GW-2  
Python: `gateway/transports/buzz/ (1.7k lines)`

- [ ] Port transport
- [ ] **Done when:** Buzz messages answered

## 9. CLI commands (D16 unless noted)

### CLI-0 · CLI framework
`D16` · ~4 h · depends on: PLT-PRC-1  
Python: `surfaces/cli/{app,group,args,invocation,startup,signals,error_mapping,layout}.py, commands/command_specs.py`

- [ ] cobra root with lazy command loading (help must not load every command)
- [ ] Exit codes (`infrastructure/process/exit_codes.py`)
- [ ] Error mapping to user messages
- [ ] Shared terminal output (tables, prompts, banner)
- [ ] **Done when:** `opensre --help` lists the same top-level commands as Python

### CLI-W · Setup and onboarding wizard
`D16` · ~8 h · depends on: CLI-0, PLT-CRD-1  
Python: `surfaces/cli/wizard/ (+ local_grafana_stack, local_llm, configurators)`

- [ ] `opensre setup`, `opensre onboard`
- [ ] LLM credential flow, custom endpoints, Azure OpenAI
- [ ] Integration configurators + validators + health probes
- [ ] Forms show required/optional fields and defaults; validate before saving; keep values on error
- [ ] **Done when:** Fresh machine → configured LLM + one integration via the wizard

### Per-command ports

| Done | Command | Subcommands | Python (`surfaces/cli/commands/`) | Day | Hours |
| --- | --- | --- | --- | --- | --- |
| [ ] | `ask` | – | `ask.py, surfaces/cli/ask/` | D16 | 3 |
| [ ] | `account` | login, logout, usage, credits status | `account.py, account_auth.py, account_ui.py` | D16 | 3 |
| [ ] | `credits` | – | `account.py` | D16 | 1 |
| [ ] | `login / auth` | auth, connect, status, verify, disconnect | `login.py` | D16 | 3 |
| [ ] | `config` | show, set | `config.py` | D16 | 1 |
| [ ] | `model` | list, show, set, restore, toolcall set/show | `model.py` | D16 | 2 |
| [ ] | `integrations` | setup, list, show, remove, verify | `integrations.py` | D16 | 3 |
| [ ] | `skills` | status, update, push, rollback, history, pull | `skills.py` | D16 | 3 |
| [ ] | `runbooks` | add, list, remove, verify | `runbooks.py` | D16 | 2 |
| [ ] | `guardrails` | init, test, rules | `guardrails.py` | D16 | 1 |
| [ ] | `messaging` | pair, allow, revoke, status | `messaging.py` | D16 | 2 |
| [ ] | `cron` | add, list, status, remove, run, logs, start | `cron.py, cron_results.py, scheduling.py` | D16 | 3 |
| [ ] | `gateway` | start, web, stop, status, logs | `gateway.py` | D16 | 2 |
| [ ] | `knowledge` | list, show, scan, audit | `knowledge.py` | D16 | 2 |
| [ ] | `work` | list, add, complete, next, schedule-checkin, path | `work.py` | D16 | 2 |
| [ ] | `triage` | watch, reports | `triage.py` | D16 | 3 |
| [ ] | `sentry` | digest, uptime check/watch, schedule add/list/remove/run | `sentry_digest.py` | T3 | 3 |
| [ ] | `posthog` | report run, schedule add/list/remove/run | `posthog_report.py` | T3 | 2 |
| [ ] | `debug` | sentry | `debug.py` | D16 | 1 |
| [ ] | `remote-sync` | – | `remote_sync.py, remote_sync_progress.py` | T3 | 3 |
| [ ] | `doctor / health` | – | `doctor.py, general.py` | D16 | 2 |
| [ ] | `update / uninstall / version` | – | `general.py, lifecycle/` | D18 | 2 |
| [ ] | `ci-repair-worker (hidden)` | – | `ci_repair_worker.py` | D11 | 1 |

## 10. Interactive shell and embedding

### REPL-1 · Minimal REPL
`D17` · WS5 · ~8 h · depends on: PLT-HAR-5, CLI-0  
Python: `surfaces/interactive_shell/{main,controller}.py, runtime/, session/, prompt_history/`

- [ ] Prompt loop with history, streaming answer, tool progress, cancel (Ctrl-C)
- [ ] Approval prompts
- [ ] Slash commands: /help /exit /new /resume /sessions /model /effort /tools /integrations /status /cost /cancel /clear /compact
- [ ] **Done when:** Daily-driver session works end to end

### REPL-2 · Full REPL parity
`T3` · T3 · ~40 h · depends on: REPL-1  
Python: `surfaces/interactive_shell/ui, command_registry/*`

- [ ] Remaining slash commands: /account /alerts /auth /auto /choose /config /context /credits /cron /debug /demo /doctor /gateway /goal /guardrails /health /history /knowledge /login /logout /loops /mcp /memory /messaging /onboard /posthog /privacy /quit /remote /remote-sync /rename /runbooks /sentry /setup /skills /stop /tasks /theme /triage /trust /uninstall /update /verbose /verify /version /work
- [ ] Themes, sound, autonomy modes (`repl_autonomy`, `repl_theme`, `repl_sound`)
- [ ] **Done when:** Slash catalog parity test passes

### EMB-1 · Embedded API
`D17` · WS5 · ~2 h · depends on: PLT-HAR-5  
Python: `surfaces/embedded.py`

- [ ] `StartEmbeddedSession` Go package API
- [ ] **Done when:** Example program runs a turn

## 11. Release and deployment

### DEP-1 · Release binaries
`D18` · WS5 · ~4 h · depends on: PLT-CI-1  
Python: `opensre.spec (PyInstaller), infrastructure/deployment/packaging`

- [ ] goreleaser: linux/darwin/windows × amd64/arm64
- [ ] Checksums + signing
- [ ] `opensre update` / `uninstall` against Go releases
- [ ] **Done when:** Tagged release publishes artifacts

### DEP-2 · Container and cloud deploy
`D18` · WS5 · ~4 h · depends on: DEP-1  
Python: `Dockerfile, DEPLOYMENT.md, infrastructure/deployment/{container,ec2,contracts}`

- [ ] Distroless image
- [ ] EC2/ECS task definitions behind a flag
- [ ] Health/readiness probes
- [ ] **Done when:** Staging runs the Go image

### DEP-3 · Installers
`D18` · WS5 · ~2 h · depends on: DEP-1  
Python: `install.sh, install.ps1, infrastructure/deployment/cloudflare_install_proxy`

- [ ] Installers fetch the Go binary
- [ ] Install proxy routes to Go artifacts
- [ ] **Done when:** Clean install on Linux, macOS, Windows

## 12. Parity, cutover, docs

### CUT-1 · Shadow mode
`D19` · WS5 · ~4 h · depends on: DEP-2  
Python: `infrastructure/alert_intake.py`

- [ ] Mirror production alerts to Go; store, do not deliver
- [ ] Comparison report: tool calls, evidence, report quality, latency, cost
- [ ] **Done when:** Agreed alert set compared; zero P0/P1 gaps

### CUT-2 · Cutover and rollback
`D20` · WS5 · ~4 h · depends on: CUT-1  
Python: `DEPLOYMENT.md`

- [ ] Switch endpoints one at a time: /alerts → /v1/prompt → gateways → scheduler
- [ ] Rollback runbook tested (< 5 min)
- [ ] **Done when:** Python receives no traffic; rollback rehearsed

### DOC-1 · User docs
`D20` · WS5 · ~3 h · depends on: CUT-2  
Python: `docs/ (+ docs/docs.json)`

- [ ] Update install, configuration, CLI and integration pages
- [ ] New `.mdx` pages added to docs.json navigation
- [ ] **Done when:** Docs describe the Go binary only

## 13. Post-month backlog (Tier 3)

- [ ] **PLT-LLM-6** Subscriptions (ChatGPT, Grok) and LiteLLM transport (~6 h)
- [ ] **GW-8** Buzz transport (~4 h)
- [ ] **REPL-2** Full REPL parity (~40 h)
- [ ] Tier 3 integrations listed under T3 in §5 (~58 h)
- [ ] CLI `sentry` (~3 h)
- [ ] CLI `posthog` (~2 h)
- [ ] CLI `remote-sync` (~3 h)
- [ ] Remove the Python implementation after 2 weeks of stable Go traffic

