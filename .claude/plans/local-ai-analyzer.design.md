# Local AI Analyzer — design

## Status

DESIGN ONLY, not implemented. Trees read at telark `5bcfd94` (2026-09-23 16:13 +0200, working tree clean at read time; other workflows are applying approval-mode and dependency bumps, so line numbers may drift). Paths: telark = `/Users/houssem/Desktop/Github/telark`, internal = `/Users/houssem/Desktop/Github/internal`, ui = `/Users/houssem/Desktop/dashboard-ui`. Subchart line refs are into `charts/telark/charts/ollama-1.50.0.tgz` (extracted, not vendored).

Base = "minimal" design; grafts from "risk-first" and all judge mustFix items applied (see Decisions).

## Phase 1 trace (today)

| Hop | What | Where |
|---|---|---|
| Trigger | Leader-gated tick every `INSIGHTS_TICK_INTERVAL_SEC=300`; returns early unless `cfg.AI.Enabled && Provider != ""`; lists ALL apps; BuildSignals; one batch POST | `services/discovery/internal/controllers/insights/controller.go:69-94,96-199`; `main.go:217`; `charts/telark/values.yaml:254-257`; leadergate `internal/coordination/leadergate/gate.go:11-53` |
| Client | `AnalyzerClient.DispatchApplications`, 10s timeout, circuit-breaker slot, connectivity gate `connectivity:service:analyzer` (3s TTL heartbeat from Python) | `discovery/internal/clients/analyzer.go:16-45`; `internal/rest/clients/insights/client.go:15-29`; `internal/rest/endpoints/insights/def.go:5`; `internal/rest/clients/shared/utils.go:141-145`; `services/analyzer/main.py:366-382` |
| Service | Python FastAPI; `POST /api/v1/insights/applications` (HMAC service token) → ready if `analyzer:{ns}:{name}` has matching promptVersion, else `SET NX analyzer:enqueued EX 600` + `LPUSH analyzer:jobs`; BLPOP workers (1 if ollama else NUM_WORKERS, startup-only) | `main.py:119-127,268-342,391-445`; `api_server.py:134-146`; `insights.py:33-64` |
| Provider | ABC `BaseProvider.enrich`; Groq/Gemini/Ollama = instructor Mode.JSON over `/v1` (no num_ctx → 2048); Anthropic SDK. Factory knows anthropic/groq/gemini/ollama; UI saves claude/chatgpt → provider None → jobs dropped. Fallback "Could not analyze" cached 24h as real | `providers/base.py:45-128`; `providers/ollama.py:23-52`; `providers/anthropic.py:28-86`; `providers/__init__.py:23-54`; `ui/src/features/settings/sections/insightsGovernance/constants.ts:1-12`; `main.py:304-307`; `base.py:34-42,123-128` |
| Prompt/model | JSON-only prompt, `PROMPT_VERSION=sha256(source)[:8]`; result `AnalyzerResultLLM`; in-process cache keyed by structural signals excluding name | `prompts/k8s_app_analyzer_prompt.py:9-37,280-291`; `models.py:78-147`; `providers/cache.py:12-61` |
| AI config | `get_ai_config` GET exporter globalconfig with service token, 45s cache, last-good fallback | `provider_config.py:28-76` |
| Storage | Redis only: `analyzer:jobs`, `analyzer:dlq`, `analyzer:{ns}:{name}` TTL 86400, `analyzer:inflight` TTL 120, `analyzer:enqueued` TTL 600. No NATS, no CRD, no files | `constants.py:11-14,192,198-200`; Go mirror `internal/data/resources/application/insights.go:3-44` |
| Read API | discovery `GET insights/applications?apps=ns/name`, `Read(applications)`, per-key `cache.GetAnalyzer` (Redis GET), miss = pending, no namespace-visibility check | `discovery/internal/handlers/insights/handler.go:31-58`; `routes/routes.go:26`; `authz/requirements.go:43`; `discovery/cache/cache.go:29-52` |
| UI | `useApplicationInsights` polls every `max(10, fetchIntervalSeconds ?? 60)`s; panel enriched/enriching/empty; X-Session-Token header from localStorage on 4 axios instances; no EventSource/WebSocket anywhere | `ui/src/features/resources/applications/hooks/useApplicationInsights.ts:24-74`; `components/details/ApplicationInsightsSection.tsx:258-286`; `ui/src/api/client/instances.ts:45-51` |
| Settings | `AIInsightsSection` (Owner gate) validates key via analyzer `POST /provider/validate-api-key`, PATCHes exporter `{ai:{enabled,provider,apiKey}}`; exporter guard ai = Owner + controlainsights; apiKey diverted to Secret `<app.name>-ai-provider-key`, injected back on GET for Internal/Owner; Redux persists `data.ai.apiKey` to localStorage | `AIInsightsSection.tsx:48-203`; `aiInsights/AIInsightsSectionContent.tsx:6-14`; `exporter/internal/handlers/resources/globalconfig/handler.go:21-35,37-84,101-148`; `exporter/internal/authz/guard.go:226-230,241-250`; `exporter/internal/secrets/aikey.go:17-113`; `ui/src/store/persistConfig.ts:48-52` |
| CRD/type | CRD `spec.ai` = enabled+provider only; Go `AIConfig{Enabled,Provider,APIKey omitempty}` | `charts/telark-crds/templates/crds/resources/globalconfig.yaml:40-46`; `internal/data/resources/globalconfig/def.go:26-30` |
| Chart | Ollama dep 1.50.0 (appVersion 0.17.7), condition `app.ollama.enabled`, default **true** (README:57/INSTALL.md:210 say false = drift); pull `qwen2.5:3b` via postStart (invisible progress); PVC 10Gi; env CONTEXT_LENGTH 2048, KEEP_ALIVE -1, NUM_PARALLEL 1, FLASH_ATTENTION true; no resources/securityContext/NetworkPolicy; analyzer has SA only; HPA excludes only exporter | `charts/telark/Chart.yaml:51-54`; `values.yaml:102-103,307-327,546-567`; subchart `templates/deployment.yaml:170-193`, `values.yaml:125,150,157,356`; `templates/workloads/hpa.yaml:3-4`; `templates/analyzer/rbac/serviceaccount.yaml` |
| Publish | OCI `ghcr.io/<owner>/charts`, cosign keyless, helm-docs drift check; GitOps: `crds.enabled=false`, `app.serviceToken.value` for cluster-less renders | `.github/workflows/release-charts.yaml:14-199`; `ci.yaml:112-150`; `docs/INSTALL.md:12,18` |
| nginx | `/api/analyzer/` → `telark-analyzer-service:8080`, upstream keepalive_timeout 3s; global proxy_http_version 1.1, `Connection ""`, read 30s (discovery 60s), buffering on, no proxy_ignore_headers | `ui/nginx/nginx.conf:36-37,40-53,85-100` |

### Consumers (every path reading insight data or branching on AI config)

| Path | Reads / branches on | Phase 4 |
|---|---|---|
| `discovery/internal/controllers/insights/controller.go:69-199` | GlobalConfig ai.enabled/provider; all apps → Signals | REMOVE (tick + BuildSignals) |
| `discovery/main.go:217` | starts insights controller behind leadergate | REMOVE line |
| `discovery/internal/clients/analyzer.go:16-45` + circuitbreaker ExecuteAnalyzer slot (`circuitbreaker/client.go:143-144`, `types.go:26`, per terrain) | HTTP dispatch | REMOVE |
| `internal/rest/clients/insights/client.go:15-29`, `endpoints/insights/def.go:5` | POST dispatch | ADAPT: drop DispatchApplications; add `Analyze`, `Events`, `Runtime`, `RuntimeValidate`, `RuntimePull` constants |
| `internal/rest/base/def.go:53` `Analyzer="analyzer"` | service id | KEEP |
| `discovery/internal/handlers/insights/handler.go:31-58`, `routes/routes.go:26`, `authz/requirements.go:43` | Redis read route | KEEP route/authz/key; ADAPT parser to `AppInsights`; ADAPT: drop `?apps=` entries whose namespace is in `GlobalConfig.excludedNamespaces` before the Redis GET (same rule as other resource reads: `gcfghelper.FetchExcludedNamespaces`, `informers/list.go:35`, `coordination/handler.go:180`) (U-3) |
| `discovery/internal/discovery/cache/cache.go:29-118` | GetAnalyzer/parseCachedAnalyzer | ADAPT to new document |
| `discovery/internal/discovery/cache/cache.go:136` `IsStale` | dead | REMOVE |
| `discovery/internal/handlers/cleanup/autoclean/rails.go:155-157`, `detector.go:100` (terrain) | `lock:enrich:{app}` never written | ADAPT: check `analyzer:inflight:{ns}:{name}` |
| `discovery/internal/discovery/prewarm/prewarm.go:29-36`, `constants/messages.go:21` | log text only | ADAPT wording |
| `discovery/internal/core/applications/core/publish.go:18-57` | (new) | ADAPT: best-effort XADD on OutcomeAuthored incident/recovery |
| `exporter/.../globalconfig/handler.go:21-35,37-84,101-148` | injectAIKey/divertAIKey | REMOVE divert/inject; keep GET/PATCH |
| `exporter/internal/authz/guard.go:226-230,241-250` | ai field Owner+controlainsights; MayControlAIInsights | KEEP field rule; REMOVE MayControlAIInsights key-injection use |
| `exporter/internal/secrets/aikey.go:17-113`, `exporter/internal/constants/config.go:131-133` | Secret store, env | REMOVE |
| `charts/telark/templates/shared/ai-key-secret.yaml:1-20`, `templates/exporter/rbac/role.yaml:7`, `values.yaml:220-221` | Secret shell, RBAC, env | REMOVE; the kept Secret (resource-policy keep) is deleted by a new `templates/shared/ai-key-secret-cleanup.yaml` pre-upgrade hook Job (U-5): `helm.sh/hook: pre-upgrade`, `hook-delete-policy: before-hook-creation,hook-succeeded`; hook-annotated SA + Role (`secrets` `delete`, `resourceNames: [<app.name>-ai-provider-key]`); image `curlimages/curl:8.20.0` (already used by `templates/tests/test-connection.yaml:20`) issuing `DELETE /api/v1/namespaces/$NS/secrets/<app.name>-ai-provider-key` against `kubernetes.default.svc` with the mounted SA token; 404 = success; no documented kubectl step |
| `internal/data/resources/globalconfig/def.go:26-30`, `builtin.go:23` | AIConfig | REPLACE `{Enabled, Model, AutoAnalyze}`; seed `{false, "qwen3:4b", false}` (R1, U-2: autoAnalyze off on fresh installs) |
| `charts/telark-crds/.../globalconfig.yaml:40-46` | CRD spec.ai | ADAPT FIRST: add `model` string, `autoAnalyze` bool; drop `provider` |
| `internal/data/resources/application/insights.go:3-44` | result type | REPLACE with `AppInsights`/`Insight` |
| `internal/data/insights/types.go:5-55` | Signal/DispatchRequest | REPLACE with job/event/runtime types |
| `services/analyzer/*` (Python, Dockerfile, requirements, .env.example) | all | REFACTOR IN PLACE, Python kept (U-7): delete the provider stack (`providers/{base,anthropic,gemini,groq,cache,config,constants}.py`, the instructor-based `providers/ollama.py`, `enricher.py`, `provider_config.py`, `prompts/k8s_app_analyzer_prompt.py`), the validate-api-key and dispatch routes, the BLPOP/DLQ worker and the connectivity heartbeat; drop `instructor`/`anthropic` from requirements.txt; keep the layout, FastAPI/uvicorn, `authz.py`, the Dockerfile base and the pytest suites |
| `charts/telark/values.yaml:307-327`, `modes/minimal.yaml:21-28`, `modes/performance.yaml:24-31` | env, NUM_WORKERS | REPLACE env; REMOVE NUM_WORKERS rows |
| `charts/telark/templates/workloads/hpa.yaml:3-4` | exporter-only exclusion | ADAPT: exclude analyzer too |
| `charts/telark/values.yaml:102-103,546-567`, `README.md:57`, `INSTALL.md:210`, `VALUES.md:29,52,182-195,296-297`, `PUBLISHING.md:55-56` | ollama default/values/docs | ADAPT (default false, env, resources, keep annotation, empty pull list, docs aligned) |
| `ui/nginx/nginx.conf:36-37` | keepalive_timeout 3s | KEEP: the analyzer stays uvicorn, whose idle close is 5s, so the upstream pool must expire first (comment at `nginx.conf:36`) (U-7) |
| `ui/.../insightsGovernance/AIInsightsSection.tsx`, `constants.ts:1-78`, `aiInsights/AIInsightsSectionContent.tsx` | ai form, providers, key validate | REPLACE (status card, model select, Pull, enabled, autoAnalyze; Owner gate kept) |
| `ui/src/features/globalconfig/store/globalConfigSlice.ts:7-21` | ai model shape | ADAPT `{enabled, model, autoAnalyze}` |
| `ui/src/store/persistConfig.ts:48-52` | persists data incl. apiKey | KEEP (field gone) |
| `ui/src/constants/rest/endpoints.ts:53-58,69-74`, `urls.ts:7-42`, `api.ts:5-31` | insights read, validate-api-key, analyzer base w/o `/api/v1` | ADAPT: remove validate; analyzer base gets `/api/v1`; add analyze/events/runtime paths |
| `ui/.../applications/clients/insights.ts:9-26`, `hooks/useApplicationInsights.ts:10-74` | poll | REPLACE hook: initial GET + SSE subscription + resync |
| `ui/.../applications/models/application.ts:52-88` | ApplicationInsights | REPLACE with `AppInsights` |
| `ui/.../components/details/ApplicationInsightsSection.tsx`, `pages/details/Content.tsx:97` | panel | REPLACE body (cards, run line, Analyze button) |
| `ui/.../applications/constants/texts.ts:118-131,43-44,74`, `errors.ts:4`, `constants/store/store.ts:4,12-13,93`, `constants/shared/common.ts:14` | strings; unreferenced HAS_CLUSTER_INSIGHTS/BY_INSIGHT_* | REPLACE strings; REMOVE dead constants |
| `ui/src/api/health/service-registry.ts:9`, `instances.ts:26` | analyzer health/instance | KEEP |
| `ui/.../permissionEngine.tsx:200-204`, `scopeRules.ts:96`, `SettingsMenuItems.tsx:30,151-154`, `useSettingsNavigation.ts:16-17,28-29`, `settings/constants/settings.ts:62-69,128` | Owner permission, nav | KEEP (labels: "Local analyzer") |
| `ui/.../notifications/utils/typeRegistry.ts:16-41` | no insight type | KEEP (V1 no notification) |
| `services/notifier`, `services/auth` | none (terrain grep) | — |

## Phase 2 design

### Hard-rule compliance

| Rule | How |
|---|---|
| Read-only | Analyzer ClusterRole: core pods,events get/list; apps deployments,statefulsets,daemonsets,replicasets get/list. No write verbs, no secrets/configmaps/logs, no metrics (`_cluster_rbac.tpl:23-40` helper) |
| Typed tools, validated args | pydantic v2 arg models (`extra='forbid'`, `Literal` enums, `Field` ranges); ns/name bound to the run's app; workload names must come from `get_app_overview` |
| Namespace scoping | App namespaces from the CR; `GlobalConfig.excludedNamespaces` respected |
| Output caps | 2048 B per result, truncation flags set by the analyzer |
| No kubectl / raw passthrough / mutation | Only 4 fixed tools below |
| Model decides / code does | Model: hypotheses, next tool, correlation, stop, explanation. Code (the Python analyzer): authz, retrieval, diffs, timestamps, math, limits, dedup, persistence, evidence refs |
| Minimal context, compact summaries | User msg ~100 tokens; never manifests |
| Zero impact when off/absent | Discovery keeps the read route; trigger is a best-effort XADD; no HTTP client, no circuit-breaker slot, no connectivity gate on discovery's path |
| Reuse | Redis (keys, stream), the exporter over httpx with the service token (the `provider_config.py:42-54` pattern), `authz.py` (auth-service resolution, `authz.py:56-65`); the Kubernetes API over httpx with the pod's service-account token; no new dependency (U-7); new = 1 stream `insights:jobs`, 1 ClusterRole, 1 NetworkPolicy (SSE fan-out is an in-process asyncio queue while replicas=1, R10) |

### V1 tools

| Name | Wraps | Args | Output (≤2048 B) | Caps |
|---|---|---|---|---|
| `get_app_overview` | exporter `GET /api/v1/resources/applications/{name}/get` over httpx with `X-Service-Token` (`rest/endpoints/resources/applications` GetApplicationByName; U-7); fields `internal/data/resources/application/def.go:40-45,81-125` | none (ns/name injected) | health{status,reason,ready/total}, namespaces[], managedBy, workloads[{kind,name}] from Resources, generation, hasDrift, lastChange{gen,class,severity,isIncident,isRecovery}, snapshotIds[last 3] | 1/run (cached); 5s |
| `get_change_history` | slices `History.ChangeLog` from the CR already fetched (`def.go:81-104`) — no call | `{limit 1..10=5, onlyIncidents?}` | [{generation, detectedAt, changeClass, severity, isIncident, isRecovery, changes[≤6 {field,changeType,old/new ≤80ch}]}]; refs `gen:<generation>` | 2/run; 0 k8s |
| `get_workload_status` | in-cluster API over httpx with the pod's service-account token and CA (GET only; no client library, U-7): apps GET + core pods list by labelSelector; the per-run caps bound traffic to 16 GETs per run, so no client-side rate limiter | `{kind enum deployment\|statefulset\|daemonset, name}` (must match overview) | conditions, desired/updated/ready, pods[≤5 by restarts: phase,restarts,waitingReason,lastTerminated{reason,exitCode,at}], images name:tag; refs `workload:<kind>/<name>` | 3/run; 5s |
| `get_recent_events` | core events List per app namespace (`Application.Namespaces.Items`, `def.go:47-55`), `fieldSelector type=Warning` when warningsOnly, Limit 200, **no name selector**; the analyzer keeps only events whose involvedObject matches a generated-name shape of an app workload exactly (names from the overview; no `get_workload_status` needed, 0 extra k8s calls): the workload itself (kind+name), ReplicaSet `^<deploy>-[bcdfghjklmnpqrstvwxz2456789]{1,10}$`, Deployment pod `^<deploy>-[bcdfghjklmnpqrstvwxz2456789]{1,10}-[bcdfghjklmnpqrstvwxz2456789]{5}$`, DaemonSet pod `^<ds>-[bcdfghjklmnpqrstvwxz2456789]{5}$`, StatefulSet pod `^<sts>-\d+$` (pod-template-hash and pod suffixes use the k8s SafeEncodeString alphabet, `k8s.io/apimachinery/pkg/util/rand/rand.go:83`); a bare `<name>-*` prefix would pull sibling workloads of the same namespace (`api-gateway`, `api-worker-0`) into the app's refs and block resolve rule (2) (R8, R14). Pod-level BackOff/Failed/Unhealthy/FailedScheduling and ReplicaSet FailedCreate are kept | `{sinceMinutes 5..120=30, warningsOnly=true, namespace?}` (namespace must be one of the app's; default all, ≤5 namespaces per call) | ≤12 deduped (reason,object,msg[:120]) with count/first/last, msg ≤160ch; refs `<reason>@<object>@<lastSeen>` | 2/run; 5s per namespace |

Desired/ready per workload needs the k8s call (Application.Health is app-level only, `def.go:40-45`), so it lives in `get_workload_status`, not the overview. No 5th tool (OOMKilled shows in lastTerminated; metrics RBAC deferred).

### Insight schema

```go
type Insight struct {
  ID string; Kind enum[crashloop,oom,image_pull,probe_failure,scheduling,rollout_stuck,config_change_regression,resource_pressure,other] // attribute only, NOT part of the id (R5)
  Subject string           // canonical "deployment/api": kind lowercased, name must match a get_app_overview workload (Go rejects otherwise)
  Title string /*≤120*/; Summary string /*≤400 (R2)*/
  Confidence enum[low,medium,high]; Severity enum[info,warning,critical]
  Status enum[open,updated,resolved]
  Evidence []Ref           // {type enum[change,snapshot,event,workload], ref string}
  FirstSeenAt, LastSeenAt, ResolvedAt string; Runs int
}
type AppInsights struct {
  Insights []Insight
  LastRun struct{ Status enum[queued,running,done,failed]; Trigger enum[manual,incident,recovery]; StartedAt, FinishedAt, Error, Model string; Steps, ToolCalls int; Truncated bool }
  Version int              // monotonic; UI change detection
}
```

Contract applied and frozen in `internal/data/resources/application/insights.go:42-81` (S2); the Python analyzer mirrors it as pydantic models in `services/analyzer/models.py` with identical JSON names (U-7).

| Aspect | Design |
|---|---|
| Model emits | kind, subject, title, summary, confidence, severity, evidence refs (only refs returned by tools this run; unknown refs dropped by the analyzer; 0 refs → dropped unless kind=other → low). Two emitted insights with the same canonical subject → the analyzer keeps the higher severity one (R5) |
| Analyzer stamps | id, timestamps, status, runs, version; clamps confidence=low when refs<2 or run truncated |
| Dedup key | `id = sha256(ns\|name\|subject)[:16]` from analyzer-validated fields only; `kind` is a model-chosen attribute stored on the card, never keyed (a 3-4B model relabels the same failure run to run). One card per workload; two distinct problems on one workload share a card in V1 (R5, R6) |
| Lifecycle | Telark code owns every transition; model omission never resolves (R5, R6). Any run: emitted+existing open/updated → `updated` (summary/evidence replaced, runs++, lastSeenAt); emitted+existing resolved → **reopened** as `open` (runs++, resolvedAt cleared, firstSeenAt kept); new → `open`; not re-emitted → unchanged. Resolve = analyzer code only: (1) `recovery` job (ChangeLogEntry.IsRecovery, `def.go:89-90`, from `gate/incident.go:79-87`) marks every open/updated insight `resolved` **without calling the model**; (2) after an untruncated run, for each open insight whose subject was fetched this run, the analyzer resolves it when the cached `get_workload_status` shows ready==desired with no waitingReason and the cached events hold no Warning for that object newer than `lastSeenAt` (no extra k8s calls). Truncated run: writes low-confidence, resolves nothing. Failed run: only `lastRun` written. `resolved` pruned on write after 7d. **stale** = UI-derived (`lastSeenAt` > 24h and not resolved); no sweep |
| Evidence refs | `gen:<generation>` → ChangeLogEntry (`def.go:82-93`); `snap:<id>` → ApplicationSnapshot.ID (`def.go:117-125`); `<reason>@<object>@<lastSeen>` label (events expire ~1h); `workload:<kind>/<name>` label. No payloads |
| Storage | Redis `analyzer:{ns}:{name}` string JSON, one atomic SET per run, TTL 7d refreshed on write. Same key the discovery read route serves (`cache.go:29-52`) → insights readable with analyzer off/absent. Rejected: CRD (silent pruning, informer wakeups, exporter owns CRDs), exporter files (no list, RWO), per-insight hashes/index ZSET (nothing lists cross-app in V1) |
| App deleted | `get_app_overview` 404 at start or tool binding fails → DEL key, drop job |

### Triggers

| Trigger | Source | Debounce |
|---|---|---|
| Manual | `POST /api/v1/insights/applications/{ns}/{name}/analyze` on analyzer, `Write(applications)`; app must exist (exporter GET) | inflight → 202 with running runId; `analyzer:cooldown:manual:{ns}:{name}` SET NX EX 60 → 429; queue length ≥100 → 429 |
| Incident | discovery `PublishApplications` (`core/publish.go:18-57`): when `outcomes[i]==OutcomeAuthored` (`history/diff/app_diff.go:34-40`) and newest `History.ChangeLog` entry has `IsIncident` (dedup already done by `history/gate/incident.go:16-33,104-140`) → `PublishWithMaxLen("insights:jobs", {ns,name,trigger,generation}, 1000)` (`x-ware/redis/stream/stream.go:35-47`), errors logged only | `analyzer:cooldown:auto:{ns}:{name}` SET NX EX 600; skipped when `GlobalConfig.ai.autoAnalyze=false` (checked by the analyzer, not discovery). `autoAnalyze` seeds **false**: a fresh install runs manual analyses only until an admin enables it in Settings (U-2) |
| Recovery | same XADD when the newest entry has `IsRecovery` (`def.go:89-90`, `gate/incident.go:79-87`); the analyzer handles it **in code only**: resolve open insights, write document, emit `analysis.finished` — no model call (R6) | no cooldown (no LLM cost); still gated by `autoAnalyze` |
| Periodic | not V1 (5-min all-apps tick was the cluster-wide load; adds cost without closing a failure) | — |

Queue: Redis stream `insights:jobs`, consumer group `analyzer`, one asyncio worker task, replicas=1 (HPA exclusion). Inflight key `analyzer:inflight:{ns}:{name}` SET NX EX 540 (wall 480 + 60 slack, R2). Jobs older than 30 min at consume → ACK+drop. Why stream not NATS: JetStream is `WorkQueuePolicy` with one durable consumer per topic (`x-ware/nats/streams/cons.go:11`, `conusmer.go:20-26`). Why stream not list: ACK + `ClaimStale` (`stream.go:20-90`) make a later replicas>1 a config change, not a rewrite.

### Tool-calling protocol

Model capability evidence:

| Model | Tools cap | Size | Layers/KV heads/head dim | KV f16 per token | KV @8k | Weights+KV | License | Notes |
|---|---|---|---|---|---|---|---|---|
| qwen2.5:3b | yes (https://ollama.com/library/qwen2.5) | 1.9 GB | 36/2/128 (https://huggingface.co/Qwen/Qwen2.5-3B-Instruct/raw/main/config.json) | 36 KiB | 0.28 GiB | ~2.2 GB | **Qwen RESEARCH — non-commercial only** (https://huggingface.co/Qwen/Qwen2.5-3B-Instruct/raw/main/LICENSE lines 16,19; same text in the Ollama blob) | NOT shippable as default (R1, R12); operator choice only, Settings shows a license warning; chart's own note rates MEDIUM (`values.yaml:550`) |
| qwen3:4b | yes + thinking (https://ollama.com/library/qwen3) | 2.5 GB | 36/8/128 (https://huggingface.co/Qwen/Qwen3-4B/raw/main/config.json) | 144 KiB | 1.125 GiB | ~3.6 GB | Apache 2.0 | **default** (R1; confirmed U-1: smallest tool-capable, stable, commercially-licensed model, ~4.5 GB loaded, 5Gi/6Gi); `think:false` on every call (format+thinking empties response: https://github.com/ollama/ollama-python/issues/597) |
| qwen2.5:7b / 1.5b | yes | 4.7 / 0.99 GB | — | — | — | — | Apache 2.0 | documented alternatives (7b quality, 1.5b footprint); sizes from https://ollama.com/library/qwen2.5, KV not computed |
| granite4.1:3b | yes (https://ollama.com/library/granite4.1) | 2.1 GB | 40/8/64 | 80 KiB | 0.625 GiB | ~2.7 GB | Apache 2.0 | loadability on 0.17.7 UNVERIFIED → not default |
| gemma3 | **no** (only `vision`, https://ollama.com/library/gemma3; absent from https://ollama.com/search?c=tools) | — | — | — | — | — | — | rejected by capability, never assumed |

KV formula: `2 × layers × kvHeads × headDim × 2 B`. `/api/show` `capabilities` derives `tools` from the chat template (https://raw.githubusercontent.com/ollama/ollama/v0.17.7/server/images.go); a tools request on a model without it fails "does not support tools". Tool-call reliability of any 3-4B model is not measured anywhere trustworthy → final default is ASSUMPTION until a smoke benchmark on 0.17.7.

Loop (native `/api/chat`, never `/v1`: no num_ctx, no tool_choice — https://docs.ollama.com/api/openai-compatibility):

1. messages = [system (read-only investigator, cite refs, stop when evidence suffices, ≤8 calls), user `{ns, name, trigger, lastChange{gen,class,severity}}` ~100 tok]
2. per step: `POST /api/chat {model, messages, tools[4], stream:false, think:false, keep_alive:"30m", truncate:false, shift:false, options{num_ctx:8192, temperature:0.2, num_predict:400}}`, HTTP timeout 120s (cold load ~30 + new prompt 735/60 ≈ 12 + 400/8 = 50 → 92s worst case, R2). `truncate:false, shift:false` (api/types.go:111-117 on v0.17.7; defaults are true at routes.go:2213,2292) make an overflow a hard error instead of Ollama silently dropping the oldest non-system messages (server/prompt.go:20-70 keeps only system + latest), which would lose the user message and early tool results without any signal (R3). Before every call the analyzer runs the fit check in Caps
3. `tool_calls` → ≤2 per step; validate into the tool's pydantic args model (`extra='forbid'`; unknown field/enum/range/binding error → tool message `{error}`); 2 consecutive invalid → abort `invalid_tool_calls`. No tool_calls → content JSON `{name,arguments}` recovery once, else done
4. EMIT: one `/api/chat` with the **identical `tools[4]` array plus** `format=<Insight[] schema, maxItems 3>` (`api/types.go` Format json.RawMessage), a short appended user message ("emit now", ~100 tok), num_predict 1024, HTTP timeout 180s (1024/8 ≈ 128s gen + cached prefix). Tools stay in the request because the qwen2.5/qwen3 templates render `<tools>` inside the system block (registry blob sha256:eb4402837c…: `{{- if or .System .Tools }}<|im_start|>system … {{- if .Tools }}`); dropping them changes the prompt from token 0 and discards the whole KV prefix cache, re-evaluating ~6.8k tokens (~85s on CPU) (R2). ASSUMPTION: format+tools accepted together on 0.17.7 — verify in the smoke benchmark; fallback = make the last loop turn the emit (tools present, format set). Output sizing: per insight ≈ 275 tok (title ≤120 ch ≈ 35, summary ≤400 ch ≈ 110, enums ≈ 30, ≤4 refs ≈ 60, JSON scaffolding ≈ 40) × 3 = 825 < 1024, so num_predict cannot cut the JSON mid-array (was maxItems 5 × ~330 tok > 1024). Strict decode; one retry with decode error; then run failed. Two-phase (tools, then schema) is the schema-constrained path; no in-loop oneOf fallback because only tools-capable models pass validation

### Caps (from the 8192 window, ~3.5 chars/token on JSON — 3.7 was optimistic, R3)

| Cap | Value | Derivation |
|---|---|---|
| Context | 8192 | 2048 today cannot hold 4 schemas + 8 results; 16k doubles KV for no step gain |
| Fixed prompt | ~1.2k | system 600 + 4 schemas 500 + user 100 |
| Per step | ~735 | result ≤2048 B ≈ 585 + assistant ≤150 |
| Steps | 8 (hard) | the fit check below binds at ~7 when every result is full-size: 1.2k + 7×735 ≈ 6.3k |
| Emit | 100 in + 1024 out | appended user msg ~100 tok; num_predict 1024 for 3 insights ≈ 825 tok |
| Token budget (per request, NOT cumulative) | prompt ≤ 8192 − 400 − 1124 ≈ 6650 for a loop step; prompt + 1024 ≤ 8192 for EMIT | Size of the request **about to be sent** = max(bytes(messages)/3.5, last `prompt_eval_count`+`eval_count`) — never a sum across calls: each `/api/chat` resends the whole history, so a cumulative sum grows quadratically (≈35k over 8 steps) and would trip at step 3 (R3, R7). Fit check before each loop step: prompt + 400 (num_predict) ≤ 8192 **and** prompt + 735 (next growth) + 100 + 1024 (emit reserve) ≤ 8192, else forced EMIT with truncated=true. With `truncate:false, shift:false` an overflow Ollama still reports is an error: at a loop step → forced truncated EMIT (fits by construction); at EMIT → run failed `context_overflow` |
| Tool calls | 8 (≤2/step) | steps × 1 avg |
| Result bytes | 2048 | ~585 tok |
| Wall | 480s (one value, CPU-derived; GPU simply finishes earlier, no separate GPU wall — nothing measured to derive one, R11) | CPU, qwen3:4b, 4 vCPU (ASSUMPTION ~60 tok/s prompt, ~8 tok/s gen): cold load ~30 + 8 steps × (735/60 ≈ 12 + ~150 gen/8 ≈ 19 → ~31s) ≈ 250 + EMIT (100/60 + 825/8 ≈ 105s) ≈ 385s → cap 480 (R2). Per-call HTTP timeouts: loop 120s, emit 180s, each also bounded by remaining wall. **EMIT reserve (R13)**: before each loop step, if remaining wall < 120 + 180 = 300s → skip the step and force the truncated EMIT (timeout = min(180, remaining)); the loop's own worst case (30 + 8×62 = 526s) exceeds 480s on a contended CPU node, and without the reserve EMIT would be cut off and all gathered evidence lost |
| Concurrency | 1 | = OLLAMA_NUM_PARALLEL 1 on every profile; replicas 1 |

### Failure matrix

| Failure | Behaviour |
|---|---|
| Ollama down | `GET /api/tags` 5s before each run fails → lastRun failed `runtime_unreachable`, job ACKed (cooldown prevents storms), worker backoff 15s; runtime state pushed (`runtime.changed`); analyzer `/status/ready` stays 200 |
| Model missing / pulling | `/api/show` 404 → if `autoPull` and no pull running: start pull, job failed `model_not_installed` (retry via cooldown/manual); pulling → worker pauses; air-gapped (`autoPull=false`) never pulls |
| Model lacks tools | refused before `/api/chat`; runtime `model_unsupported`; Settings shows it |
| HTTP timeout (120s loop / 180s emit) | run failed `model_timeout`, nothing written |
| Context overflow reported by Ollama (`truncate:false`) | loop step → forced truncated EMIT; EMIT → failed `context_overflow`, nothing written (R3) |
| Ollama 503 (MAX_QUEUE) | busy: sleep 15s, retry once, then failed |
| k8s timeout 5s | tool returns `{error:timeout}`; run truncated → low confidence, no resolves |
| Malformed tool call | error tool message; 2 consecutive → failed `invalid_tool_calls` |
| Loop exhaustion | forced EMIT with truncated=true → low confidence, no resolves |
| Wall nearly exhausted (remaining < 300s before a loop step) | forced truncated EMIT with timeout min(180, remaining) → low confidence, no resolves; only an EMIT that itself times out is `model_timeout` (R13) |
| Partial evidence | <2 refs → low; 0 refs → dropped; an empty list resolves **nothing** (resolution is code-only, see Lifecycle, R5/R6) |
| App deleted mid-run | 404 → abort, DEL document |
| Duplicate / stale | dedup id (Go-validated subject); inflight key; 7d TTL; 24h UI stale; `lastRun.running` older than 540s rendered as failed by UI |
| Analyzer disabled | worker not started (30s GlobalConfig poll); manual → 409 `analyzer_disabled`; discovery read route still serves last document |
| Analyzer absent | discovery XADD still succeeds (stream sits at MAXLEN 1000); UI hides Analyze via existing health interceptors |
| Redis down | XADD logged; discovery core untouched; analyzer 503 |
| Crash mid-run | inflight expires 540s; UI marks failed |

## Phase 3 deployment

### Who creates Ollama

| Option | RBAC | GitOps drift | Cleanup | Leader/replicas | Verdict |
|---|---|---|---|---|---|
| (a) Telark reconciles Deployment/Service/PVC | new Role: apps deployments + core services/pvcs create/update/delete on a SA that must not be the analyzer's (exporter Role has none, `role.yaml:1-9`) | resources outside git; Argo prune/selfHeal fights; upgrades move from values to Go | ownerRefs/pre-delete hook; PVC delete = Telark-owned data-loss decision | reconciler needs a lock | rejected (largest new surface). Trigger: click-only enable for admins with no Helm/Git |
| (b) Helm renders subchart, Settings toggles usage | none | native | `--set app.ollama.enabled=false`; PVC kept via annotation | moot (replicaCount 1) | **chosen** |
| (c) separate telark-ollama chart | none | native | separate release | moot | rejected: second artifact/pipeline/install. Trigger: GPU node pools or independent lifecycle |

Decision (b)+status: Settings shows runtime state (`absent/unreachable/model missing/pulling/unsupported/ready`) and, when absent, the exact `helm upgrade ... --set app.ollama.enabled=true` / GitOps value. Nothing in Telark gains workload-write RBAC.

Chart changes: `app.ollama.enabled: false` (fixes README:57/INSTALL.md:210 drift); `ollama.ollama.models.pull: []`; `ollama.persistentVolume.annotations: {helm.sh/resource-policy: keep}` (subchart `templates/pvc.yaml:6-9`, `values.yaml:356`); resources; securityContext; `templates/shared/ollama-networkpolicy.yaml`; analyzer added to the HPA exclusion (`hpa.yaml:3-4`); analyzer ClusterRole via `_cluster_rbac.tpl:23`; `templates/shared/ai-key-secret-cleanup.yaml` pre-upgrade hook Job that deletes the old `<app.name>-ai-provider-key` Secret (see Consumers row; U-5).

### Model acquisition

| Mode | Mechanism |
|---|---|
| Online | Settings "Install model" → analyzer `POST /api/v1/insights/runtime/pull` (Owner+controlainsights) → streams Ollama `POST /api/pull stream:true` (ProgressResponse status/completed/total per layer, https://raw.githubusercontent.com/ollama/ollama/v0.17.7/docs/api.md); progress held in-process (replicas=1), exposed on `GET runtime` and pushed as `runtime.pull` |
| Air-gapped | `app.ollama.autoPull=false` + either (1) `ollama.persistentVolume.existingClaim` pre-seeded with `~/.ollama/models` (blobs+manifests), or (2) a baked image: `FROM ollama/ollama:0.17.7` + `COPY models /models` and values `ollama.image.repository/tag`, `ollama.persistentVolume.enabled=false`, `ollama.extraEnv: [{name: OLLAMA_MODELS, value: /models}]`. The model MUST live outside `/root/.ollama`: the subchart always mounts a volume there (`templates/deployment.yaml:131` mountPath, `:237-244` emptyDir when persistence is off), which would hide anything baked at the default path and leave `/api/tags` empty for good (R4). Analyzer sees the model via `/api/tags`, never pulls. `OLLAMA_NO_CLOUD=true` stays |

### Runtime config (values, `ollama.extraEnv`)

| Var | Value | Why |
|---|---|---|
| OLLAMA_CONTEXT_LENGTH | 8192 | analysis budget; VRAM-tier default is 4096 (<24 GiB, https://docs.ollama.com/context-length); also sent as `options.num_ctx` |
| OLLAMA_NUM_PARALLEL | 1 (every profile) | = analyzer concurrency 1 (one worker, replicas 1); a second slot could never be used and would cost a full KV block (qwen3:4b +1.125 GiB) (R11) |
| OLLAMA_MAX_LOADED_MODELS | 1 | CPU default 3 keeps the old model resident after a Settings switch (https://docs.ollama.com/faq) |
| OLLAMA_KEEP_ALIVE | 30m (every profile) | disable in Settings frees RAM/VRAM within 30m (zero impact); cold load ~30s fits the 480s wall; no reason to pin -1 on GPU (R11) |
| OLLAMA_MAX_QUEUE | 8 | only the analyzer calls; fail fast 503 instead of 512 deep |
| OLLAMA_FLASH_ATTENTION | 1 | required for KV quant; auto on supported backends |
| OLLAMA_KV_CACHE_TYPE | f16 | q8_0 on CPU backend + high-GQA Qwen2 precision UNVERIFIED; size for f16 |

### Profiles (weights + KV f16 @8k × 1 slot + ~0.8 GiB graph/runtime, rounded; NUM_PARALLEL=1 everywhere)

| Profile | Model | Sum | requests | limits |
|---|---|---|---|---|
| CPU default | qwen3:4b (R1) | 2.5 + 1.125 + 0.8 ≈ 4.4 GB | cpu 2, mem 5Gi | cpu 4, mem 6Gi |
| CPU small | qwen2.5:1.5b (Apache) | 0.99 + KV (not computed) + 0.8 ≈ 2.5 GB (ASSUMPTION) | cpu 2, mem 3Gi | cpu 4, mem 4Gi |
| GPU | qwen3:4b | VRAM = weights 2.5 + KV@8k 1.125 × 1 slot + graph ~0.8 ≈ 4.4 GiB (R11); host mem 2Gi/3Gi | cpu 1, `nvidia.com/gpu: 1` | cpu 2 |

Arithmetic, not measured; validate scheduling on the 2-node dev layout (single node ~88% CPU per memory).

| Item | Design |
|---|---|
| PVC | 10Gi stays (confirmed U-6; default 4b 2.5 + 7b 4.7 + 1.5b 1.0 = 8.2 GB; comment: 20Gi when trialing 8b); keep annotation above |
| Probes | subchart `/` liveness/readiness unchanged; model presence = analyzer `GET runtime`, not a probe |
| securityContext | `allowPrivilegeEscalation:false`, `capabilities.drop:[ALL]`, `seccompProfile RuntimeDefault`. NOT runAsNonRoot/readOnlyRootFilesystem: model dir is `/root/.ollama` (subchart `values.yaml:125`), image user root = ASSUMPTION; nonroot is a follow-up (set mountPath+OLLAMA_MODELS+HOME, verify) |
| NetworkPolicy | new `templates/shared/ollama-networkpolicy.yaml` gated by `app.ollama.enabled`: podSelector `app.kubernetes.io/name=ollama`; ingress 11434 only from pods labelled analyzer-service; egress DNS+443 only when `autoPull` (subchart has none; Redis policy `values.yaml:404-406` is the precedent) |

### Settings vs values

| Knob | Where | Apply |
|---|---|---|
| `ai.enabled` (default false), `ai.model` (default `qwen3:4b`, U-1), `ai.autoAnalyze` (default false, U-2) | Settings (GlobalConfig CR; CRD yaml first) | hot: analyzer polls exporter GET every 30s with last-good fallback; discovery reads nothing |
| Model pull | Settings button → analyzer | immediate |
| Context, parallel, keep-alive, queue, KV type, resources, GPU, PVC, image, autoPull, NetworkPolicy | values | helm upgrade rollout |
| Caps (steps 8, calls 8, result 2048, wall 480, loop/emit timeouts 120/180, chars-per-token 3.5, queue 100, cooldown 600) | values `services.analyzer.env` | rollout |

Validation: model name allowlist regex `^[a-z0-9][a-z0-9._-]*(:[a-z0-9._-]+)?$`; `POST /api/v1/insights/runtime/validate {model}` (Owner+controlainsights) → `/api/show` capabilities must contain `tools` (422 `model_lacks_tools`; confirmed U-1: Settings lets the user change the model, validation rejects any model without the tools capability); response also carries the license label from the builtin model list (qwen3:4b / qwen2.5:7b / qwen2.5:1.5b = Apache-2.0; qwen2.5:3b = research/non-commercial warning) (R1). **No live smoke `/api/chat`**: it is synchronous from the UI, would take ~50s on CPU (cold load 30 + one step), and both nginx `proxy_read_timeout 30s` (`ui/nginx/nginx.conf:48`, `/api/analyzer/` at `:98-100` has no override) and the UI `API_TIMEOUT = 30000` (`ui/src/constants/rest/api.ts:3`) would 504/abort it on first enable; the capability check plus per-run strict decode and 2-strike abort already cover reliability (R9). Analyzer re-checks `/api/show` at every run start so a `kubectl edit` cannot bypass. UI disables the enabled toggle while runtime != ready. Exporter cannot reach Ollama (NetworkPolicy), so validation lives on the analyzer.

## Phase 4 consumers & real-time UI

### Transport

| Option | nginx (`nginx.conf:40-53,85-100`) | auth | fan-out | reconnect | connection limits | verdict |
|---|---|---|---|---|---|---|
| SSE via fetch streaming | `X-Accel-Buffering: no` honoured (no proxy_ignore_headers); `: ping` every 15s < read 30s; zero nginx edits (the analyzer upstream keepalive 3s stays below uvicorn's 5s idle close, `nginx.conf:36-37`) | header `X-Session-Token` sent by fetch (EventSource cannot) | in-process broadcaster (one bounded `asyncio.Queue` per connection) while replicas=1; replicas>1 later = one XREAD reader task per replica on a stream, never one per connection (R10) | reconnect = GET snapshot (document `Version` is monotonic); no Last-Event-ID, no replay | 1 stream per tab, only on insight pages; HTTP/1.1 6/origin documented | **chosen** |
| WebSocket | needs Upgrade forwarding (`Connection ""` at :41) in nginx and every customer ingress | same header problem | same | manual | same | rejected |
| ETag / long-poll | 30s read caps hold at ~25s | ok | none | n/a | one hanging request per tab | rejected: still 10-60s latency, "Analyzing…" feels dead |

Server: `GET /api/v1/insights/events?apps=ns/name` on the analyzer, `Read(applications)` plus the visible-namespaces rule (U-3): `?apps=` entries whose namespace is in `GlobalConfig.excludedNamespaces` are dropped from the subscription (the analyzer already holds GlobalConfig from its 30s poll), and events are fanned out only to subscriptions that kept the app; the discovery list route applies the same filter (Consumers row), so list and stream never leak an excluded namespace; the worker publishes to an in-process broadcaster that fans out over per-connection bounded `asyncio.Queue`s (slow consumer → queue drained + one `resync`). **No per-connection Redis XREAD**: a blocking XREAD pins a pooled connection for the whole block, and with `REDIS_POOL_SIZE: "10"` (`charts/telark/values.yaml:315`) ~10 open tabs would starve the worker's own SET NX / SET / cooldown calls, so viewers could fail analyses (R10). The endpoint is a FastAPI `StreamingResponse` over an async generator reading the connection's queue: the SSE relay of what the worker publishes while it consumes `insights:jobs` (a per-connection Redis XREAD stays rejected, see above); uvicorn sets no write timeout, so no per-write deadline is needed, uvicorn `timeout_graceful_shutdown` keeps open streams from holding a rollout, and a client disconnect cancels the generator, whose `finally` unsubscribes (U-7); every event carries `{app, version}`. UI hook ~40 lines: ReadableStream + TextDecoder, parses `event:/data:`, one initial GET, then one GET whenever an event's `version` is greater than the local one (all events of one run share a version → one GET per run), GET on reconnect and `resync`. No optimistic card patches (one mechanism, not three).

### Events

| Event | Data | UI reaction |
|---|---|---|
| analysis.queued | app, runId, trigger | "Queued", Analyze disabled |
| analysis.started | app, runId, model | "Analyzing… (manual/incident/recovery)" spinner |
| analysis.failed | app, runId, error | inline error from constants, cards untouched, button re-enabled |
| analysis.finished | app, runId, version, truncated, created/updated/resolved counts | GET if version moved, spinner cleared |
| insight.created / updated / resolved | app, id, version, status | GET if version moved (deduped with analysis.finished); the UI renders new marker / refresh / collapsed Resolved from the fetched document, not from the event (R10) |
| runtime.changed | state, model, reason | Settings status card; banner in panel when != ready |
| runtime.pull | model, completed, total | Settings progress bar |
| resync | — | GET refetch (also on every reconnect) |

Webhooks: none from the analyzer. External integrations remain a notifier-service concern (V1 emits no notification; trigger: user wants alerts → `Emit` on critical `insight.created`).

## Decisions

1. **In-place Python refactor of `services/analyzer`** (FastAPI + uvicorn, pydantic v2, `redis.asyncio`, httpx, loguru — all already in `requirements.txt`; `instructor`/`anthropic` leave with the providers; same chart slot, image repo, Dockerfile base, SA, nginx location, service id; name kept for V1, confirmed U-4; language confirmed U-7). Why: no host/upstream/registry churn, and the layout, the `authz.py` auth-service check, the pytest suites and the CI jobs carry over. Ollama and the Kubernetes API are reached with httpx: no ollama SDK, no kubernetes client, no instructor, no LangChain/agent framework. Rejected: Go rewrite (user decision U-7); fold into discovery (200s LLM loop in the informer service); rename to analyzer-service (cosmetic churn, deferred).
2. **Read route stays on discovery, key `analyzer:{ns}:{name}`, one document, one SET, UI-derived stale; list and SSE both filter `?apps=` by the caller's visible namespaces (`excludedNamespaces` rule, U-3).** Why: zero impact when analyzer is off/absent; O(1) read; no sweep/lock. Rejected: risk-first move of reads to the analyzer (panel goes empty), hash+ZSET+sweep.
3. **Triggers = manual + incident/recovery XADD at `publish.go`**, no periodic tick. Why: recovery signal resolves insights; removes HTTP client, breaker slot, connectivity gate and the all-apps tick. Rejected: keep the 5-min tick (cluster-wide load); NATS subscriber (WorkQueuePolicy).
4. **4 tools; `get_change_history` from the CR ChangeLog.** Why: no unverified exporter route; zero extra RBAC. Rejected: 5th metrics tool.
5. **Evidence refs `gen:`/`snap:` + event labels.** Why: durable; event UIDs expire.
6. **Native `/api/chat` two-phase (tools → tools+format schema, maxItems 3), think:false, truncate/shift false, 2-strike abort.** Tools stay in EMIT to keep the KV prefix (R2). Rejected: `/v1` (no num_ctx/tool_choice); in-loop oneOf fallback (tools-only models).
7. **Context 8192; caps 8/8/2048/480s; token budget is per-request (next prompt ≤ 6650 loop / +1024 emit), fit check in code before every call (byte floor + counters).** Rejected: cumulative budget (quadratic, trips at step 3, R3/R7); trusting `prompt_eval_count` alone (prefix cache); Ollama's silent truncate/shift.
8. **Default qwen3:4b think:false (5Gi/6Gi, Apache-2.0; confirmed U-1), qwen2.5:7b/1.5b documented Apache alternatives, f16 KV; Settings model change validated for the tools capability.** Reliability ASSUMPTION until smoke benchmark (a dev-time task, not a runtime check). Rejected: qwen2.5:3b as default (Qwen RESEARCH license, non-commercial only — operator choice with a warning, R1/R12); gemma3 (no tools); granite4.1 (unverified on 0.17.7).
9. **Helm owns Ollama, default off, PVC 10Gi (U-6) with keep annotation, empty pull list, NetworkPolicy, conservative securityContext.** Rejected: (a) reconciler, (c) second chart, runAsNonRoot with `/root/.ollama`.
10. **KEEP_ALIVE 30m on every profile; NUM_PARALLEL 1 on every profile.** Why: disabling frees 2-4 GB; parallel slots > analyzer concurrency are dead KV (R11).
11. **replicas=1 via HPA exclusion; stream consumer group + inflight key already make >1 a values change plus a Redis pull lock (SET NX, the inflight pattern) and WATCH/MULTI document writes in place of the in-process `asyncio.Lock`.**
12. **Fetch-streaming SSE from the analyzer (FastAPI `StreamingResponse`), in-process fan-out, events carry `{app, version}`, GET on version change/reconnect; no nginx edit (the analyzer upstream keepalive 3s stays below uvicorn's 5s idle close).** Rejected: per-connection XREAD + Last-Event-ID replay + optimistic patches (pool exhaustion, three mechanisms for one panel, R10); adaptive polling (task requires replacing polling); WebSocket; long-poll.
13. **GlobalConfig: CRD yaml first (model, autoAnalyze; drop provider), then Go type; seed `autoAnalyze=false` (U-2); apiKey/Secret/Role rule/validate-api-key removed; the old Secret is deleted by the pre-upgrade hook Job (U-5), no documented kubectl step.**
14. **No vendor names in UI strings**: "Local analyzer", "engine", "model".

## Risks

| Risk | Mitigation |
|---|---|
| Tool-call reliability of qwen3:4b unmeasured | dev-time smoke benchmark before ship (no runtime smoke, R9); strict decode; 2-strike abort; low clamp; qwen2.5:7b via Settings |
| format + tools in one `/api/chat` request unverified on 0.17.7 | smoke benchmark; fallback = emit on the last loop turn with tools present (R2) |
| Wall/CPU estimates unmeasured; dev node ~88% CPU | validate on 2-node layout; caps are env |
| Memory sums arithmetic (graph ~0.8 GiB assumed) | measure `/api/ps`; MAX_LOADED_MODELS=1 |
| `prompt_eval_count` semantics under prefix cache | byte floor in code; verify on 0.17.7 |
| granite4.1 on 0.17.7, image user root, q8_0 on CPU | all ASSUMPTION, none load-bearing |
| Namespace visibility: today's read route authorizes on `Read(applications)` only (`handler.go:31-58`) | closed (U-3): list and SSE drop apps in `GlobalConfig.excludedNamespaces` (Decision 2) |
| CRD change before Go type (silent pruning) | ordering in Decision 13 |
| Kept Secret left behind | pre-upgrade hook Job deletes it (U-5); hook needs API reachability from the Job pod (NetworkPolicy egress to the API server) |
| `(b)` cannot create pods from Settings | documented gap; (a) trigger-bound |
| HTTP/1.1 6/origin with many tabs | one stream per tab on insight pages only |
| Autoclean rail repoint changes cleanup timing | bounded by 540s inflight TTL |
| One card per workload subject merges two distinct problems | accepted V1 ceiling; a code-derived category from observed evidence is the upgrade path (R5) |
| nginx upstream hostnames hardcoded `telark-*` | pre-existing; unchanged |
| Line numbers on dirty trees | re-read before implementation |
| Kubernetes API called with a hand-rolled httpx helper, no client library (U-7) | GET is the helper's only verb; token re-read per request (projected tokens rotate); CA from the mounted `ca.crt`; ClusterRole get/list only |
| One asyncio event loop serves the API, SSE and the worker (U-7) | only `redis.asyncio` and `httpx.AsyncClient` I/O; no sync redis/httpx, no threads |

## Out of scope

Chatbot, gateway, provider abstraction, agent framework, RAG/vector DB, multi-agent, MCP. Also: periodic sweep, cross-app insight index/list page, notifications for insights, metrics tool, nonroot Ollama, rename to analyzer-service, Ollama image bump.

## Open questions for the user (all answered 2026-09-23)

1. Default qwen3:4b (5Gi/6Gi, Apache-2.0) confirmed? — **ANSWERED (U-1)**: yes, qwen3:4b think:false (~4.5 GB; 5Gi request / 6Gi limit); user can change it in Settings; validation rejects models without the tools capability.
2. `autoAnalyze` default true or false on fresh installs? — **ANSWERED (U-2)**: OFF; manual Analyze only until an admin enables it in Settings.
3. Should the analyzer filter insights/SSE by the caller's visible namespaces? — **ANSWERED (U-3)**: yes, list and SSE both filter by visible namespaces (same `excludedNamespaces` rule as other resources), closing the leak in today's read route.
4. Keep the service id/host name for V1, or rename now? — **ANSWERED (U-4)**: keep the V1 ids, hostnames and chart keys; no rename in V1 (the rename to `analyzer` landed on 2026-09-25).
5. Documented `kubectl delete secret` or a Helm pre-upgrade hook? — **ANSWERED (U-5)**: Helm pre-upgrade hook Job removes `<app.name>-ai-provider-key` automatically; no documented kubectl step.
6. PVC 10Gi default stays, or raise to 15Gi now? — **ANSWERED (U-6)**: 10Gi stays.

## Revision log

| # | Severity | Section | Resolution |
|---|---|---|---|
| R1 | high | Tool-calling model table / Decision 8 / Profiles | Applied: qwen2.5:3b is Qwen RESEARCH (non-commercial); default → qwen3:4b think:false (Apache-2.0); seed, CPU default profile 5Gi/6Gi, PVC math, open question 1, license label in validate response |
| R2 | high | Loop EMIT + Caps wall/emit | Applied: EMIT keeps the identical tools array + format (KV prefix preserved; format+tools on 0.17.7 marked ASSUMPTION with fallback); maxItems 3, summary ≤400; num_predict 1024 sized 3×275; timeouts 120s loop / 180s emit; wall 300→480, inflight 360→540 |
| R3 | high | Caps token budget + request options | Applied: budget is the size of the next request (max of byte floor at 3.5 chars/tok and last counters), fit check prompt+400 and prompt+735+100+1024 ≤ 8192; `truncate:false, shift:false` on every call; overflow → forced truncated EMIT or `context_overflow` |
| R4 | high | Model acquisition air-gapped | Applied: baked image must use `/models` + `OLLAMA_MODELS=/models` via `ollama.extraEnv` because the subchart always mounts a volume (emptyDir) at `/root/.ollama` (`deployment.yaml:131,237-244`); exact values documented |
| R5 | high | Insight dedup key + lifecycle | Applied: id = sha256(ns\|name\|subject) from Go-validated canonical subject; kind is an attribute; model omission never resolves; Go resolves on recovery job or on observed ready==desired + no newer Warning |
| R6 | high | Lifecycle (recovery, reopen) | Applied: recovery jobs are Go-only (no model call), resolve all open; resolved re-emitted → reopened `open`, runs++, resolvedAt cleared; recovery bypasses cooldown |
| R7 | high | Caps token budget (quadratic) | Applied with R3: "cumulative" dropped; per-request cap derived from 8192 − 400 − emit reserve 1124 |
| R8 | high | V1 tools get_recent_events | Applied: Warning events listed per app namespace with Limit 200 and no name selector; Go filters involvedObject to workloads, `<deploy>-*` ReplicaSets and `<workload>-*` pods; optional namespace arg |
| R9 | high | Settings validation | Applied: live smoke removed (nginx `proxy_read_timeout 30s` nginx.conf:48, UI `API_TIMEOUT` api.ts:3); `/api/show` tools capability only; smoke benchmark stays a dev-time task |
| R10 | high | Transport SSE server | Applied: no per-connection XREAD (`REDIS_POOL_SIZE: "10"` values.yaml:315); in-process broadcaster while replicas=1, one reader per replica later; no `insights:events` stream, no Last-Event-ID replay, no optimistic patches; events carry version, UI GETs on version change/reconnect |
| R11 | high | Runtime config + GPU profile | Applied: NUM_PARALLEL=1 on every profile; GPU VRAM = weights + KV@8k × 1 + graph ≈ 4.4 GiB for qwen3:4b; single 480s wall (no unmeasured GPU wall); KEEP_ALIVE 30m everywhere |
| R12 | high | Model table / Decision 8 (license, duplicate of R1) | Applied with R1 |
| R13 | high | Caps wall / Loop step 4 EMIT | Applied: EMIT reserve — a loop step is taken only if remaining wall ≥ 300s (120 loop + 180 emit), else forced truncated EMIT with timeout min(180, remaining); failure-matrix row added |
| R14 | high | V1 tools get_recent_events filter | Applied: prefix match replaced by exact generated-name regexes (RS `<deploy>-hash`, Deployment pod `<deploy>-hash-5`, DaemonSet pod `<ds>-5`, StatefulSet pod `<sts>-N`, alphabet `bcdfghjklmnpqrstvwxz2456789` from `apimachinery/pkg/util/rand/rand.go:83`) — no extra k8s calls, works before `get_workload_status` |
| U-1 | user | Model table / Decision 8 / Settings validation | Applied: default qwen3:4b think:false confirmed (~4.5 GB; 5Gi/6Gi unchanged); model changeable in Settings; tools-capability validation confirmed |
| U-2 | user | Consumers seed / Triggers / Settings knobs / Decision 13 | Applied: `autoAnalyze` seeds false; incident/recovery XADDs are ignored until an admin enables it; manual Analyze only on fresh installs |
| U-3 | user | Consumers read route / Transport server / Decision 2 / Risks | Applied: discovery list route and analyzer SSE drop `?apps=` entries in `GlobalConfig.excludedNamespaces` (rule of `informers/list.go:35`); namespace-visibility risk closed |
| U-4 | user | Decision 1 | Applied: service name "analyzer" kept for V1 (ids, hostnames, chart keys) |
| U-5 | user | Consumers Secret row / Chart changes / Decision 13 / Risks | Applied: `templates/shared/ai-key-secret-cleanup.yaml` pre-upgrade hook Job (curl DELETE via SA token, 404 = success, hook-scoped SA/Role with `resourceNames`) replaces the documented `kubectl delete secret` |
| U-6 | user | Profiles PVC / Decision 9 | Applied: PVC stays 10Gi |
| U-7 | user | Consumers analyzer + nginx rows / Hard-rule compliance / V1 tools / Insight schema + aspects / Triggers / Queue / Loop / Failure matrix / Transport + server / Decisions 1, 7, 11, 12 / Risks | Applied (2026-09-23 20:00, final): the analyzer stays Python — `services/analyzer` is refactored in place (FastAPI + uvicorn, pydantic v2, `redis.asyncio`, httpx, loguru; `instructor`/`anthropic` removed with the providers; Ollama and the Kubernetes API over httpx, no new dependency). Every Go statement about the analyzer service replaced: pydantic `extra='forbid'` arg models, one asyncio worker task, `asyncio.Queue` fan-out, FastAPI `StreamingResponse` SSE relaying the in-process broadcaster that the `insights:jobs` consumer feeds (a per-connection Redis XREAD stays rejected, R10), nginx upstream keepalive stays 3s (uvicorn 5s idle close, `nginx.conf:36-37`). Contracts unchanged (CRD, internal/data, internal/rest, discovery XADD). |
