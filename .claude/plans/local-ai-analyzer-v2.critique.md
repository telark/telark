# Critique: local-ai-analyzer-v2 plan (2026-09-24)

I checked the plan against the working trees: analyzer, the chart, internal/data, dashboard-ui, the landing page and `scripts/`. I also ran the S7 renders against today's chart.

Totals: Critical 1 · High 7 · Medium 8 · Low 5. Every Critical and High finding and all cheap Mediums are applied to both `local-ai-analyzer-v2.plan.md` and `.plan.json`. The JSON parses with `json.tool`.

## Critical

**C1: the JSON twin has no `change` field on any step.** Applied.
- Evidence: the v2 JSON step keys are `id, phase, title, repo, parallelGroup, dependsOn, route, files, testFirst, verify`. Every other plan JSON (`protection-plans-*.plan.json`, `local-ai-analyzer.plan.json`) also has `change`. The apply workflow reads the JSON, so implementers would get titles only. The JSON S3–S6 `verify` strings also lacked the venv bootstrap.
- Fix: I added each md step's Change text (after the critique fixes) to its JSON step line and synced `verify` from the md. `testFirst` for S5/S6 and `files` for S6 are updated too.

## High

**H1: fast-mode "rules without a model" can't be reached from a manual run.** Applied (S6, S8, edge cases, contract table, R1).
- Evidence:
  - `api_server.py:176-178` returns 503 `runtime_<state>` unless the runtime is `ready`.
  - `ApplicationInsightsSection.tsx:108` sets `canAnalyze = … && runtime?.state === 'ready'`.
  - `main.py:300` idles the worker while `runtime.pulling`.
- So V5 ("model not on the volume → Analyze → cards still written") would get a disabled button and a 503, and the edge case "Fast run, model missing/pulling" holds only for incident triggers.
- Fix:
  - In fast mode, S6 removes the route's ready gate and idles the worker on pulls only in deep mode.
  - S8 sets `canAnalyze` to `mode === 'fast' || state === 'ready'`, with a rules-only banner string.
  - Tests are added in `test_api_server_cov.py` and `test_main_cov.py`.

**H2: a narration failure can still stamp the run `failed` after the cards were written.** Applied (S5, S6, edge cases).
- Evidence: `analyzer._FAILURES` (analyzer.py:67-76) covers subclasses only. `providers/ollama.py:103` raises the `OllamaError` base for every other status, including the 500 "model requires more system memory" load refusal, which is the exact R11 failure. `resp.json()` can also raise `JSONDecodeError`. The plan caught only `_FAILURE_TYPES, ValidationError, ContextOverflow`, so these escape to `execute()` → `_fail(internal)`.
- Fix: `narrate` catches `(OllamaError, ValueError)` and returns None, so templates stay and the run is `done`. Added `test_narrate_generic_ollama_error_returns_none` and `test_fast_run_narration_ollama_500_still_done`.

**H3: the first write would publish `analysis.finished` twice.** Applied (S6).
- Evidence: the plan says the first write goes through `_announce(...)`, but `_announce` (main.py:142-152) always ends with `analysis.finished`. The plan's own event-order test contradicts this.
- Fix: move the per-card publishing loop out of `_announce`. The first write publishes created/updated only; `analysis.finished` is published once, after the second write.

**H4: the UI loses `mode`/`autoPull` after the first `runtime.changed` event.** Applied (S6, D7, contract table).
- Evidence: `runtime.py:73` publishes `dict(state, model, reason)`. `useAnalyzerRuntime.ts:21` replaces the runtime object with `{...e.data}`. After the first `runtime.changed`, `mode` and `autoPull` are undefined, so the air-gapped hint disappears, Install reappears and the mode label is blank.
- Fix: `_set` also publishes `mode` and `autoPull` (additive payload). The initial `RuntimeStatus` carries both. Added `test_runtime_changed_carries_mode_and_autopull`.

**H5: D11's build path can't build analyzer.** Applied (D11, V1, status).
- Evidence: `scripts/local-build-push.sh:156` exits with "is not a Go service" when there is no `go.mod`. Analyzer is Python and the UI is not under `services/`. NEXT_SESSION §3.3 already documents the manual `docker buildx` path.
- Fix: V1 builds analyzer with buildx and pushes it to the tag the chart already pins (no version edit), then runs `rollout restart`. The UI is seen through the user's `npm run dev`.

**H6: the V1/V3/V5 `helm upgrade` commands would fail or wipe dev overrides.** Applied.
- Evidence:
  - The commands had no `-n telark` and no `--reset-then-reuse-values`.
  - They also had no `app.persistence.storageClass=efs-sc`. A render without it aborts on the storage guard (`templates/workloads/deployments.yaml:2`, reproduced).
  - `values.dev.yaml` says it is "NOT loaded by Helm" (port-forward settings only).
  - Dropping the reuse flags would also drop the live `ollama.resources.requests.cpu=500m`, and Ollama would go Pending (R11).
- Fix: use the NEXT_SESSION §3.4 flags in V1, V3 and V5.

**H7: several Verify commands can never pass.** Applied.
- S7:
  - Every `helm template` fails without a storageClass.
  - `grep -c 'port: 443' | grep -x 0` returns 4 with `autoPull=false`, because kyverno and others also render port 443.
  - The RTK hook cuts helm output to about 1 KB even through `>` or `command helm`, so the render is unparsable.
  - Fix: renders go through `rtk proxy` with `storageClass=validate`, then `yq ea '[select(...)] | .[0]'` on the named Deployment and NetworkPolicy. I tested these against today's chart: they print `6Gi`, `4`, `1` and `0` before S7.
- S6 and S8: checks that `qwen3:4b` is absent contradict D10, which keeps `qwen3:4b` in `LICENSES` and in the catalog. They now assert `DEFAULT_ANALYZER_MODEL` / `MODELS.DEFAULT` instead.
- S6, S8 and R1: `wc -l | grep -x 0` never matches on macOS because `wc` pads the count (reproduced: rc=1). They now use `/usr/bin/grep -q`, which checks the exit status.
- S3–S6: `$T/venv` from S3 is not visible to later step agents. Every Verify now starts with the same idempotent venv bootstrap.
- S2: now greps the CRD source instead of a helm render (RTK).

## Medium

**M1: `ANALYZER_WALL_SEC 120` breaks deep mode.** Applied.
- Evidence: `EMIT_RESERVE_S = 300` (constants.py:431). With a 120 s wall, `run_analysis` forces the EMIT before the first tool step. That affects deep mode and old images under the new chart.
- Fix: keep 480. The fast path does not read the wall: the gather is bounded by `TOOL_TIMEOUT_S`, and narration by `ANALYZER_NARRATE_TIMEOUT_SEC`.

**M2: the runtime check delays the cards, and the `announce` callback isn't needed.** Applied.
- Evidence: the check ran before the gather. `runtime.check` makes two Ollama calls with a 5 s timeout each (`OLLAMA_META_TIMEOUT_S`), so a hung runtime delays the cards by up to 10 s.
- Fix: split into `gather_rules()` and `narrate()`. The worker runs: gather → first write → check → narrate → second write. This also removes the callback and keeps `run_analysis` un-renamed.

**M3: `num_predict = 16 + 90·n` is too tight.** Applied.
- Evidence: a full-length item is about 20 title tokens + 60 summary tokens + 12 JSON-syntax tokens. When the JSON is cut, every template stays.
- Fix: 120 per item. It only costs time when the model actually uses the tokens.

**M4: `LICENSES` would lose the `qwen2.5:3b` research row.** Applied.
- Evidence: `validate` (runtime.py:136) reads `LICENSES` for any tag the operator types. Without the row, the research-licence warning never fires.
- Fix: keep the row.

**M5: the timing log line would print namespace/app names.** Applied.
- Evidence: the line was `run {ns}/{name}`. The service never logs customer names (log-hygiene docstring, no `LOG_*` constant takes names), and D12 promised "no app data".
- Fix: log the runId. V3 joins log lines to apps by runId.

**M6: the V3 seeds collide with standing fixtures.** Applied (D11 detail).
- Evidence: `shop-prod/web` is a standing incident fixture, and `web`/`cart` are multi-namespace apps shared with `shop-dev` (NEXT_SESSION R9, §7). `_run` fails with `app_not_found` when `primary_namespace(app)` is not the job namespace. Deleting `shop-prod` would destroy those fixtures. The seeds also used `sh`/`python3` with no image named; without python3 the "OOM" seed exits 127 and becomes a crashloop.
- Fix: a new namespace `ai-lab` with apps `lab-pull`, `lab-crash`, `lab-oom` and `lab-multi`, with explicit `busybox` and `python:3.13-alpine` images.

**M7: the V5 air-gap check passes even if egress is open.** Applied.
- Evidence: the `ollama/ollama` image has no `wget`, so the command exits 127 and `grep -v exit=0` passes anyway. EKS VPC CNI also enforces NetworkPolicy only when its policy agent is on.
- Fix: test with a bash `/dev/tcp` connect and require exit 124 or 1. Check CNI enforcement first; if it is off, record it as an infra gap.

**M8: S4 doesn't say where `overview`/`history` come from.** Applied.
- Fix: they are `json.loads(ToolResult.content)`. `tools.fit` only drops whole list items, so the content stays valid JSON.

## Low

- L1 (applied): the V2 "pull qwen3:4b" is nearly a no-op because that model is already on the volume; the plan now notes this. Also, pods have no `lastTerminated.reason == Evicted` (an eviction is `pod.status.reason`), so the resource-pressure rule now relies on the event only.
- L2 (applied): the risk "a crash between writes re-runs via XAUTOCLAIM" was wrong. The re-delivery hits the surviving inflight key and is dropped as `run_in_progress`. This is pre-existing behaviour and the text now says so.
- L3 (applied): `ollama.extraEnv` is a name/value list. S7 now says to change the three entries in place and keep `OLLAMA_MAX_LOADED_MODELS 1`, which unloads the old model on a switch under `keep_alive -1`, and `FLASH_ATTENTION 1`, which q8_0 needs.
- L4 (not applied): `requests.memory 3Gi` is generous for a 0.7 GB model; 2Gi would fit the 8 GiB node more easily. The V1 measurement can tune it.
- L5 (not applied): the chart makes `ANALYZER_NUM_THREAD` and `ollama.resources.limits.cpu` separate settings that must be kept equal. The README rule and the V1 `NumThreads` assert cover it; deriving one from the other with tpl isn't worth the parsing edge cases (`"2000m"`).

## Claims confirmed (no change needed)

- Discovery re-encodes only `AppInsights` (`handlers/insights/handler.go:29`). `RuntimeStatus` has no Go consumer outside `internal/data`, so S1 needs no discovery change and no pin bump for the new fields (D7 holds).
- Env values are tpl-rendered (`_deployment.tpl:120`), so the `runtimeUrl | default …` expression works.
- The subchart keys `ollama.resources`, `ollama.extraEnv` and `ollama.ollama.gpu.enabled` match otwld 1.50.0. The parent's `limits: {}` default means removing `limits.memory` really removes it.
- `keep_alive` as the int `-1` is accepted by the Ollama API (number = seconds, negative = forever).
- The coverage gate is currently about 99% with a floor of 95. That is achievable with the listed tests.
- The two writes can't race: `InsightStore.update` holds an in-process lock, there is one worker, and the analyze route answers "running" while the inflight key is held.

## Decisions

- **D11 changed** (H5, H6, M6): analyzer is built with buildx at the pinned tag instead of `local-build-push.sh`, the upgrade uses `--reset-then-reuse-values` plus `storageClass`, and seeding moves to `ai-lab` with `shop-prod` untouched.
- **D7 amended**: `runtime.changed` gets two additive fields; event names are unchanged.
- **D3, D4, D5 and D6 are unchanged.** D4's no-memory-limit reasoning and subchart keys check out and V1/V2 still prove the fallback live. D5 stays chart-only: H1 only lets fast mode run without a ready model and adds no UI switch.
