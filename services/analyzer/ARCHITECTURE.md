# How the analyzer works

The analyzer produces the cards on Telark's Insights page. When an application goes wrong,
detection rules decide what is wrong and a small free model running inside the cluster only
rewrites the wording. This page maps the pieces, the order things happen in and the rules
each stage follows; the [analyzer README](README.md) covers configuration, the API, failures
and how to run it.

```mermaid
%%{init: {"theme":"base","themeVariables":{"fontFamily":"ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif","fontSize":"13px","lineColor":"#94a3b8","primaryColor":"#eef2ff","primaryBorderColor":"#6366f1","primaryTextColor":"#312e81","edgeLabelBackground":"#ffffff","clusterBkg":"#f8fafc","clusterBorder":"#e2e8f0"},"flowchart":{"curve":"basis","htmlLabels":true,"nodeSpacing":46,"rankSpacing":54,"padding":12}}}%%
flowchart TB
    user(["Operator"]):::actor
    ui("Web UI<br/>insights panel + Settings"):::client

    subgraph cluster["Inside the Kubernetes cluster"]
        disco("discovery<br/>watches your apps"):::svc
        ana("analyzer<br/>explains what went wrong"):::svc
        exp("exporter<br/>stores apps + settings"):::svc
        kube("Kubernetes API<br/>read-only"):::svc
        model{{"Ollama<br/>local model"}}:::ext
    end

    subgraph data["Redis (shared)"]
        jobs[("job stream<br/>insights:jobs")]:::store
        doc[("insight document<br/>per app")]:::store
    end

    user --> ui
    disco -->|"1 · incident / recovery job"| jobs
    ui -->|"1 · Analyze (manual job)"| ana
    ana -->|"2 · picks up one job"| jobs
    ana -->|"3 · reads settings + the app"| exp
    ana -->|"4 · reads workloads + events"| kube
    ana -->|"5 · rewrites the wording"| model
    ana -->|"6 · saves the result"| doc
    ana -.->|"7 · live events (SSE)"| ui
    ui -->|"8 · reads insights"| disco
    disco -->|"8 · reads the document"| doc

    classDef svc fill:#eef2ff,stroke:#6366f1,stroke-width:1.5px,color:#312e81;
    classDef store fill:#fff7ed,stroke:#f59e0b,stroke-width:1.5px,color:#92400e;
    classDef ext fill:#faf5ff,stroke:#a855f7,stroke-width:1.5px,color:#6b21a8;
    classDef client fill:#f1f5f9,stroke:#94a3b8,stroke-width:1.5px,color:#334155;
    classDef actor fill:#f8fafc,stroke:#cbd5e1,stroke-width:1.5px,color:#475569;
```

## Step by step

1. A job is added to the Redis stream `insights:jobs`. Discovery adds one when an
   app records an **incident** or a **recovery**; the **Analyze** button adds a
   **manual** one. There is no timer.
2. The analyzer's single worker picks up one job at a time. It skips jobs that are
   too old (30 min), already running, still cooling down, or automatic while
   `autoAnalyze` is off; a skipped **manual** run records why on the app's document,
   an automatic one leaves it untouched. A job that finds a background review of its
   app waits for it to finish (at most 20 s).
3. It reads the analyzer settings (enabled, model, autoAnalyze, excluded
   namespaces) and the application from exporter-service. Discovery enqueues an
   incident job before exporter holds the change that triggered it, so the job re-reads
   the application (3 reads, 1 s apart) until its `history.generation` reaches the
   job's; if it never does, the insight cites no change rather than an older one.
4. It reads the app with four read-only [tools](#tools) (app overview, change history,
   recent warning events, the status of up to three workloads), within fixed caps.
5. Detection rules turn what it read into at most three insights (crash loop, out of
   memory, image pull, scheduling, probes, evictions, stuck rollout, a regression
   after a recent change), each citing the evidence it rests on. Telark's own code
   merges them with the previous ones (same workload in the same namespace = same card; resolved cards
   reopen; nothing resolves just because a run stayed silent) and saves the document:
   the cards are visible within a second or two.
6. If the model is ready, one short call asks it to reword each card's title and
   summary from the same facts. It cannot change anything else; if it is slow, busy,
   missing or wrong, the rule wording stays. The document is saved again with the run
   marked done.
7. Each step is pushed to open browser tabs as a live event.
8. The UI then re-reads the document through discovery-service, which serves it
   even while the analyzer is off or absent.

A **recovery** job resolves the open insights directly, without asking the model.

After a run has saved its result, the analyzer also **reviews the app's setup** against 60
fixed rules (single replica, no probes, missing requests, privileged containers, moving image
tags, a Service that selects nothing, a production app without a protection plan, …). Each
finding is saved as a **recommendation** card in the same document, never reworded by the
model. A slow background sweep reviews the other apps too ([Recommendations](#recommendations)).

**Deep mode** (`ANALYZER_MODE=deep`, for 4+ vCPU or a GPU) instead gives the model the
four tools, lets it investigate for a few steps and asks it for the insights; the
same code checks and merges them. On a small CPU node this takes minutes.

## Fast and deep runs

**fast** (default, seconds on 2 vCPU): gather → rules → write → narrate → write.

1. **Gather** (≈ 1 s): the worker calls the tools itself: overview, change history (5), warning
   events of the last 60 min, then the status of at most 3 workloads (those named by events first).
2. **Rules** (`rules.py`, ms): one candidate per workload, by precedence `oom` > `image_pull` >
   `crashloop` > `scheduling` > `probe_failure` > `resource_pressure` > `rollout_stuck` >
   `config_change_regression` > `other`, at most 3 per run, with evidence refs and template prose.
   - A workload with no replica ready (of at least one wanted) is critical whatever the kind.
   - A paused Deployment is no incident unless its pods show a symptom (the
     `reliability.deployment_paused` recommendation says it is paused). Pausing resolves its open
     `rollout_stuck`, `config_change_regression` and `other` cards on the next run (the shortfall is
     intentional); a pod-symptom card (`crashloop`, `oom`, …) still waits for every replica.
   - Pods that cannot create their container or mount a volume keep their `other.*` cause after
     the rollout deadline.
   - The cited change (`gen:<n>`, also in the summary) is the newest incident, or rollout
     (`deployment`), `config` or `resources` change, from the last 30 min, no later than 1 min after
     the incident's start (a change made after it began is never its cause), and not newer than the
     app's newest incident entry when no recovery followed it (an edit made during an outage, even
     one whose new pods date the symptom, did not cause it). It names the entry's first field other
     than discovery's synthetic `health`; an entry with only `health`, such as the one for an app
     first seen down, is no change and is never cited. A field logged as added (a ConfigMap or Secret
     ref swapped for another) takes its old value from the entry's field-level change to the same
     value, so it reads `configMapRef <old>→<new>`. "N min after change" is measured to the
     incident's start (its earliest matched event), or to the run when no event dates it.
   - Events of a pod that no longer exists (a replaced pod's leftover) and a `FailedScheduling`
     event whose pod has a node since are history, whatever the workload's readiness. On a fully
     ready workload, events that precede their pod's current Ready state are history too: they
     raise no card, date no incident and do not block the resolve.
   - A pod with a restarted container that has run for less than 60 s has not recovered yet: a
     crash loop's container is Ready for the seconds it runs between two back-offs, so its events
     still count.
3. **First write**: the cards are merged and announced (`insight.created` / `insight.updated`);
   `lastRun` is still `running`.
4. **Narrate**: only when the runtime is `ready` and there are cards, one `/api/chat` call with no
   tools and a JSON schema (one `title` ≤ 80 / `summary` ≤ 240 per card, in order),
   `ANALYZER_NARRATE_TIMEOUT_SEC`. The model polishes each card's template title and summary (sent
   with its facts). A rewrite is kept only when its title names the workload exactly, it keeps every
   value the template states (pod, image, exit code, restarts, …) and the sub-reason's phrase, it
   adds no number, no markup, no pod/image-like name absent from the facts and no other kind's
   symptom (`out of memory`, `pull`, `crash`, `evict`, `schedul`, `probe`, `rollout`, `quota`, …),
   and it was not cut at the schema's length cap. Any failure (timeout, busy, any Ollama error, bad
   JSON, wrong item count, empty or unfaithful text) keeps the template prose. A runtime that is not
   ready is asked for the model (auto-pull) and skipped.
5. **Second write**: the narrated `title`/`summary` (nothing else), the observed resolves and
   `lastRun done`, then `insight.updated` for rewritten cards, `insight.resolved` and exactly one
   `analysis.finished`. `lastRun.steps` is 1 when the narration was applied, 0 for rules only;
   `toolCalls` counts the gather reads. A fast run never fails for the model.

**deep**: the worker checks the runtime (a run fails when it is not `ready`), then runs the native
Ollama `/api/chat` tool loop and one EMIT call constrained by a JSON schema. One write, one
`analysis.finished`. Caps: `ANALYZER_MAX_STEPS` loop steps and `ANALYZER_MAX_TOOL_CALLS` tool calls
(8 each, at most 2 per step), a per-request context fit check (`ANALYZER_CONTEXT_TOKENS` ×
`ANALYZER_CHARS_PER_TOKEN`), an `ANALYZER_WALL_SEC` wall (480 s) with a 300 s EMIT reserve, and
`ANALYZER_LOOP_TIMEOUT_SEC` / `ANALYZER_EMIT_TIMEOUT_SEC` per model call.

Every run keeps at most 3 insights and 4 evidence refs per insight. Every model request sets
`options.num_thread` (`ANALYZER_NUM_THREAD`), `num_ctx` (`ANALYZER_CONTEXT_TOKENS`) and
`keep_alive: -1`. Each fast run logs one line with no namespace or name:

```text
run <runId> timings: gather=0.4s rules=0.001s narrate=5.2s total=5.7s narrated=True reason=
```

`reason` is why nothing was narrated: the runtime state (`model_missing`, `pulling`, …), the exception
class (`OllamaTimeout`, `OllamaError`, `ValidationError`, …), `item_count` or `unfaithful` (every
rewrite was rejected).

## Tools

Fast mode calls the four tools directly; deep mode gives them to the model. Arguments are validated by
pydantic models (`extra='forbid'`), a workload must belong to the app, and each result is cut to
`ANALYZER_TOOL_RESULT_MAX_BYTES`.

| Tool | Reads | Per-run cap |
|---|---|---|
| `get_app_overview` | the Application from exporter (health, namespaces, workloads, drift, last change, snapshots) | fetched once, repeats served from memory |
| `get_change_history` | the Application's change log, newest first | 2 |
| `get_workload_status` | one Deployment / StatefulSet / DaemonSet and its most-restarted pods; optional `namespace` for a workload the app runs in several (default: the first) | 3 |
| `get_recent_events` | recent events of the app's workloads and their pods (Warning only by default) | 2 |

## Messages

The rules decide each incident's **kind** and a **sub-reason** (`reason`, 52 codes such as
`image_pull.not_found`, `crashloop.exit_127`, `probe_failure.readiness`) from the pod status, the
untruncated event text (the model sees 160 characters; containerd puts the cause after that) and the
spec limits, plus a small `params` map (workload, pod, container, image, registry, exit code,
ready/desired, probe, failure, port, …; at most 16 keys of at most 120 characters, never secret values
or the output of an exec probe). The server renders the **title** (`{workload} is down: image
not found`) and the **summary** (impact sentence, one factual detail, the change correlation) from
per-reason templates in `constants.py`; the UI renders the cause and the steps from the same `reason`
and `params`. A rules-only card is complete; the narration may only rephrase title and summary.

A liveness or startup probe failure that already restarted a container is reported as `crashloop`
(`crashloop.probe_kill`); `probe_failure.liveness`/`startup` apply only while no container has
restarted.

## Recommendations

The 60 setup-review rules fall into 10 families (`reliability`, `resources`, `scaling`, `security`,
`images`, `config`, `networking`, `change_risk`, `protection`, `consistency`). Their cards share the
document as `category: recommendation` (`kind` = family, `reason` = `<family>.<rule>`), one per
workload and rule (the four usage rules: per container and resource), capped at the 40 most severe
per app. A card ranked past the cap is removed, not resolved (it is still true), and comes back once
there is room; a dismissed card takes no place under the cap. They are never narrated: the template
text is exact.

- **When**: every analysis run reviews its app right after its final write (skipped while other jobs
  wait). An in-process sweep runs every `ANALYZER_REVIEW_TICK_SEC`: apps whose `history.generation`
  changed first, then those not reviewed for `ANALYZER_REVIEW_INTERVAL_SEC`, at most
  `ANALYZER_REVIEW_APPS_PER_MIN`. It yields while jobs are queued, skips an app another run holds,
  and forgets an app absent from two consecutive successful app listings (an empty one counts: the
  last app deleted; a failed one does not). `ANALYZER_REVIEW_INTERVAL_SEC=0` turns
  the sweep off (Analyze still reviews); nothing runs while `ai.enabled` is off.
- **Reads** (GET only): the workloads and their pods (reused from the run), four lists per app
  namespace (Services, PodDisruptionBudgets, HorizontalPodAutoscalers, NetworkPolicies, `limit=500`,
  shared across the apps of one tick), one pod probe per selector Service, and the protection plans
  and plan environments from exporter. At most `ANALYZER_REVIEW_WORKLOADS_MAX` workloads (those with
  an incident first), 5 s per read, 20 s per review.
- **Accuracy**: each rule declares its input families; a failed, truncated or timed-out read leaves
  its family incomplete, and its rules neither create nor resolve a card. Without the review's RBAC
  (see [Read-only by construction](#notes-for-developers)) those families stay incomplete. A card
  resolves only when a complete review no longer finds it. A workload GET that answers 404 is not a
  failure: discovery keeps an Application's last resource list when its last workload is deleted, so
  the workload counts as gone and its cards resolve.
- **Usage rules** need the Application metrics (`metrics.workloads[].usage`, metrics-server): each
  review appends one sample (max over instances) to `analyzer:usage` (≤ 48 per workload); the rules
  use the p95 of at least `ANALYZER_USAGE_MIN_SAMPLES` samples over `ANALYZER_USAGE_MIN_SPAN_SEC`.
- **Production**: an app is production when a namespace, or the environment of a protection plan that
  covers it, matches `ANALYZER_PRODUCTION_PATTERN`; that enables the protection rules. A workload is
  judged by its own namespace and the environments of the covering plans whose scope reaches it: that
  raises its `single_replica`/`no_pdb` to warning and enables digest pinning, so the dev namespace of
  an app that also runs in prod is not treated as production.
- **Lifecycle**: found → `open`; same facts → only `lastSeenAt` moves; new facts → `updated`; not
  found by a complete review, or about a namespace excluded since → `resolved` (kept 7 days); found
  again → reopened. Measurements that move with every sample (usage, samples, suggestion) refresh the
  text without making the card `updated`.

One line per review and one per sweep tick, with no namespace or name:

```text
review review-1790000000000 gets=13 findings=4 secs=0.41 incomplete=U
sweep due=12 reviewed=12 removed=0
```

## Notes for developers

- **Redis is shared** by discovery and the analyzer on **DB 0** (auth uses DB 1).
  The stream is `insights:jobs` (group `analyzer`); each app's document is
  `analyzer:<namespace>:<name>`; in-flight and cooldown keys use the
  `analyzer:inflight:` and `analyzer:cooldown:{manual,auto}:` prefixes.
- **The document**: code validates every insight against what the tools returned this run, merges
  it (dedup id, reopen, code-only resolve, 7-day prune) and writes it with `SET` (TTL 7 days). Every
  run, review and triage write increments `version`; a new or lost document starts from the clock in
  unix ms, so a rebuilt one never drops below a version an open panel holds.
- **One card per workload**: the id hashes the app, the workload's kind and name and its namespace,
  so an app that runs `web` in two namespaces gets a card for each. `params.namespace` names the
  workload's namespace (deep mode: the first app namespace that runs the workload). A card whose
  namespace is excluded after it was written resolves on the app's next untruncated run, and so does
  one whose workload's status read answers 404 (deleted, while the Application still lists it).
- **Live events**: every step publishes an event to the in-process broadcaster; the SSE endpoint
  relays it, and the UI refetches the document through discovery whenever an event carries a newer
  `version`. A setup review ends with `review.finished {version}`, since its write moves
  `lastReviewAt` even when no card changes.
- **Index and review keys**: `analyzer:index` (ZSET, member `<ns>/<name>`, score = last document
  write in unix ms; written after the document, removed after it; discovery's Insights page reads
  its rows from it), `analyzer:usage` (hash, per app: ≤ 48 usage samples per workload) and
  `analyzer:review` (hash, per app: `<generation>:<epoch>` of its last review). The sweep removes
  all three for a deleted app. Discovery's application reset (the API and auto-cleanup) removes them
  at once, with the app's document, cooldowns and run lease in every namespace, so a recreated app
  starts clean.
- **The contract is frozen in Go** (`internal/data/insights`,
  `internal/data/resources/application/insights.go`, `internal/rest/endpoints/insights`);
  `constants.py` and `models.py` mirror it with identical names.
- **exporter-service is the only service that reads/writes cluster config** (the
  `TelarkConfig` CR). The analyzer reads it through exporter with the service
  token, never from environment variables.
- **Read-only by construction**: there is no mutation tool and no workload-write RBAC. The tools
  and the review only issue `GET`s (`tools/k8s_tools.py` has a single `get` method, with the pod's
  service-account token). The chart's analyzer ClusterRole grants get/list on pods, events,
  deployments, statefulsets, daemonsets and replicasets, and for the review on `services`,
  `policy/poddisruptionbudgets`, `autoscaling/horizontalpodautoscalers` and
  `networking.k8s.io/networkpolicies`, nothing else (never ConfigMap or Secret contents).
- **Redis is untrusted**: a stream job whose namespace or name is not a DNS-1123 label is
  acknowledged and dropped, and the application name is percent-encoded in the exporter URL.
- **One replica, one event loop**: every document write goes through one
  in-process lock; running more replicas needs a Redis lock instead.
