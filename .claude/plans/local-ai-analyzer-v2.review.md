# R1: adversarial review of local-ai-analyzer-v2 (2026-09-24)

Scope: the uncommitted, staged change in telark `services/analyzer`, `charts/telark`, the `telark-crds` globalconfig, `docs/INSTALL.md` and `docs/CRDS.md`; `internal/data` (`constants/builtin.go`, `insights/*`, `tests/insights`); dashboard-ui (analyzer files plus the perf/layout files listed in the task); and the landing-page docs. Inputs: plan R1, the critique, and `reports/local-ai-analyzer-v2-verification.md`.

**Totals: Critical 0 · High 1 (fixed) · Medium 3 (open) · Low 8 (open)**

## Gates (after the fixes)

| Gate | Result |
|---|---|
| analyzer CI (`compileall`, `tests/test_*_cov.py`, `test_authz/insights/exporter`, `coverage --fail-under=95`) | pass: 199 + 28 tests, 99 % (no analyzer file changed in R1) |
| `internal/data` `golangci-lint run` / `go test ./...` | 0 issues / ok (`tests/insights` covers `Mode`/`AutoPull` and the default model against the CRD pattern) |
| dashboard-ui `npm run check-all` | rc 0: 0 errors, 2 warnings (both pre-existing `any` in `components/display/table/utils.tsx:13`, outside this change) |
| `helm lint charts/telark` | 1 chart linted, 0 failed |
| Plan R1 verify: provider/key grep over `services/analyzer` and `values.yaml` | empty |

## R1 claims (plan order)

| # | Claim | Result | Evidence |
|---|---|---|---|
| 1 | Every chat payload carries `num_thread`, `num_ctx` and `keep_alive -1`; the fast payload has no `tools` | HOLDS | `providers/ollama.py:142-153`; narrate passes `[]` (`analyzer.py:271`); `test_ollama_cov.py:68,75` |
| 2a | A fast run is never marked `failed` for a missing model or a narration error | HOLDS | `narrate` catches `(OllamaError, ValueError)` (`analyzer.py:277-284`); `main.py:207-214`; `test_main_cov.py:660,676`; `test_analyzer_cov.py:480,484` |
| 2b | The analyze route gates only deep mode on runtime readiness | HOLDS | `api_server.py:179`; `test_api_server_cov.py:223,231` |
| 2c | `analysis.finished` is published once per run | HOLDS | `main.py:200-232`; `test_main_cov.py:694` |
| 2d | `runtime.changed` carries `mode` and `autoPull` | HOLDS | `runtime.py:80-81`; `test_runtime_cov.py:252`; seen live in V1 |
| 2e | `runs` increments once per run | HOLDS | a single `merge()` per run (`main.py:194`); `test_main_cov.py:647` |
| 3 | Narration cannot change `kind`, `severity`, `evidence` or `status` | HOLDS | `insights.py:193-204` assigns only title and summary |
| 4 | Rule precedence and the correlation window | HOLDS | `rules.py:193` `_SYMPTOM_RULES` order; window check at `rules.py:208` (`CHANGE_CORRELATION_WINDOW_S=1800`); `test_rules_cov.py:194,202` |
| 5 | Chart: no `limits.memory` on Ollama; the 443 egress rule renders only when `autoPull=true` | HOLDS | rendered with `storageClass=validate`: limits `{cpu: "2"}`, requests `cpu 1 / 3Gi`; netpol egress is `[53/UDP, 53/TCP, 443]` with `autoPull=true` and `[53/UDP, 53/TCP]` with `false` |
| 6 | No provider path is left | HOLDS | the grep is empty. `provider` appears only as the `providers/ollama` module name and in a back-compat test payload (`test_models_cov.py:76`) |
| 7 | Docs match the code | HOLDS | README env table = `values.yaml` `services.analyzer.env` (set diff empty); VALUES.md carries the new `ollama.resources` and `extraEnv` rows; INSTALL, CRD text, chart README and landing docs name `granite4:350m` as the default |
| 8 | Tests for each claim; coverage; check-all; `go test` | HOLDS | see the Evidence column and Gates |

Hard rules:
- Free/OSS only, no keys: holds.
- No version bumps: the chart `Chart.yaml` files and `services.analyzer.version` are unchanged.
- Go tests live under `tests/`.
- dashboard-ui: no `any`, hex colours, `console.*` or inline strings in the scoped files. The one vendor name is noted at L6.
- Comments are minimal.

## Browser freeze on Settings → Local analyzer: no render loop found

I traced the render path. `AIInsightsSectionContent` renders `insightsGovernance/AIInsightsSection.tsx`, which uses `useAnalyzerRuntime`, which uses `keepInsightsStream`.

- **`useAnalyzerRuntime.ts:37-68`:** effect deps are `[]`, every update is functional, and cleanup aborts. The reconnect backoff is at least 1 s (`SSE_BACKOFF_MS.min`), so the stream cannot hot-loop.
- **`AIInsightsSection.tsx:103-113`:** the only effect is keyed on the `globalConfig.data` reference, which changes only when the fetch thunk settles. Nothing in the effect dispatches.
- **`useOpenedOnce.ts:7`:** setState during render is guarded (`open && !opened`) and converges after one extra render, which is the React-sanctioned pattern. This page also mounts no `BaseModal` or `ActionConfirmModal`.
- **`useElementWidth`:** not used on this page (only `ListToolbar`, the Applications `Success` page and the Plans list/reports pages). Scrollbar-driven width oscillation is prevented by `scrollbar-gutter: stable` (`styles/index.css:11`), and ResizeObserver delivers at most once per frame.
- **`useSidebarCollapse`:** event-driven. `publishState` fires only from `toggle`/`applyDraggedWidth`, and the listener only calls setState, so there is no self-trigger.
- **`keepUnchanged`:** returns `previous` when nothing changed, so there is no reference churn that would re-fire effects.
  - Immer use is correct: `current()` is applied only to drafts for the comparison, and immer finalizes kept drafts that are nested in the new array.
- **Pull progress events:** throttled server-side to 1/s (`constants.py:562` `PULL_PROGRESS_INTERVAL_S`). React 18 batches the setStates of a multi-frame chunk into one render.

Conclusion: nothing in the change can loop on this page. The freeze is unexplained by code. The most likely cause is the automation or dev-server session itself. To confirm, reproduce with DevTools Performance recording on `npm run dev`. If it recurs, attach the profile.

## High

**H1 (fixed): the Settings "enable the runtime" copy command was wrong and contradicted the upgrade docs.**
- Where: dashboard-ui `src/features/settings/sections/insightsGovernance/constants.ts:30` (`HELM_HINT_COMMAND`, which the Copy button puts on the clipboard at `AIInsightsSection.tsx:158`).
- It read `helm upgrade <release> telark --reuse-values --set app.ollama.enabled=true`.
  - `telark` is not a resolvable chart reference. The docs use `oci://ghcr.io/telark/charts/telark` (`docs/INSTALL.md:283`).
  - `--reuse-values` keeps the previous chart's defaults. `docs/INSTALL.md:291` and landing `operations/upgrading.mdx:55` both say not to upgrade with it. It would also skip the new Ollama sizing (no memory limit, `KEEP_ALIVE`, ctx 4096), which is the R11 refusal fix.
  - Without any reuse flag, the user's install flags (storageClass) would be dropped and the render would abort.
- Fix applied: `helm upgrade telark oci://ghcr.io/telark/charts/telark -n telark --reset-then-reuse-values --set app.ollama.enabled=true`. These are the same flags V1/V5 used live.

## Medium (open)

**M1: the narration guard lets a changed or invented plain number through.**
- Where: `services/analyzer/analyzer.py:288-300` (`_faithful`) and `constants.py:525`.
- `NARRATE_NAME_TOKEN_PATTERN` only catches hyphenated tokens that contain a digit. The `stated` check only requires the template's values to still appear somewhere; it does not forbid extra numbers.
- Scenario: "3 restarts" is kept in the text and "exit code 137" is added to an image-pull card. That passes the guard.
- The verification report saw exactly this before the guard existed ("an image-pull card that claimed `exit code 137`").
- The impact is prose only: kind, severity and evidence are code-owned (claim 3).
- Fix: in `_faithful`, add `all(n in source for n in re.findall(r"\d+", text))`, with the pattern as a constant, plus a test case.
- Left open because it changes the analyzer image and may lower the narrated rate further (61 % now). Decide it together with the report's "narrated ≥ 90 %" item.

**M2: `authz.py:90` logs the raw exception.**
- Code: `LOG_AUTHZ_RESOLVE_FAILED.format(error=exc)`.
- An httpx error can embed the auth-service URL or body. Every other module logs `type(e).__name__`, but `authz.py` is not in `tests/test_log_hygiene_cov.py` `_GUARDED_MODULES` (lines 13-27).
- This predates the change (HEAD line 92).
- Fix: `error=type(exc).__name__`, and add `"authz.py"` to `_GUARDED_MODULES`. Changes the image.

**M3: a restored copy of another user's permissions renders until a post-commit effect clears it.**
- Where: dashboard-ui `features/auth/hooks/permissions/useInitializePermissions.ts:25-41`, with the persist whitelist `store/persistConfig.ts:56-60`.
- Logout clears permissions (`utils/logout/logout.ts:30`). But when a session ends without logout (expiry, or a login as someone else in another tab), user A's persisted `roles`/`scopeIndex`/`ready` pass the route gate on the first paint. The mismatch check runs only in `useEffect`.
- The impact is UI only: the backend enforces its own authorization.
- Fix: also gate on `permissions.userID === getCurrentUser()?.id` where `App.tsx` computes `permissionsReady`, or drop the mismatched slice synchronously before children render (for example in the `PersistGate` `onBeforeLift`).

The verification report's own open Mediums still stand and are not repeated here:
- narrated 61 % (below the 90 % target) on `granite4:350m`;
- discovery sends no recovery trigger;
- VPC CNI network policy is off on telark-dev.

## Low (open)

- **L1:** `dashboard-ui .../clients/insightsStream.ts:57-68`: `sleep()` adds an `abort` listener on each reconnect and never removes it when the timer fires. During a long outage this is about one listener per backoff step on the page-lifetime signal. Fix: remove the listener in the timer callback.
- **L2:** `insightsStream.ts:44-54` dispatches every frame of a chunk synchronously. The UI reviewer ranked this High as the freeze cause. I downgraded it because the server throttles pull progress to 1/s and React 18 batches the updates into one render (see the freeze section). No change needed unless a profile shows otherwise.
- **L3:** `ProtectionPlanCard.tsx:206-210` runs `users.find` for each card on every render (O(plans × users)). Fix: build a `Map` once with `useMemo` at the list level.
- **L4:** `providers/ollama.py:163` streams the pull with `timeout=None`. A stalled pull keeps `pulling` true indefinitely, and in deep mode the worker idles (`main.py:387`). Fix: `httpx.Timeout(None, read=<bound>)`.
- **L5:** `main.py:353`: `ack_job` sits outside a try, so a transient Redis error leaves the finished job pending. `xautoclaim` then re-runs it once after `CLAIM_MIN_IDLE_MS` (10 min). This is bounded and idempotent.
- **L6:** the UI string `HELM_HINT_COMMAND` contains `app.ollama.enabled`. It is the chart key the user must type, so the vendor name is unavoidable here. Accepted.
- **L7:** landing `reference/environment-variables.mdx:84-91` lists only 3 analyzer variables. `ANALYZER_CONTEXT_TOKENS`, which deep mode needs, is documented only in `features/ai-analyzer.mdx`. Fix: add the row, or link to the page.
- **L8:** `settings/sections/insightsGovernance/AIInsightsSection.tsx` is rendered by `sections/aiInsights/AIInsightsSectionContent.tsx`, while `insightsGovernance/` renders a different page. Moving the file to `aiInsights/` would make the path traceable.

The report's Lows also stand:
- the `oom` rule misses `Error/137`;
- `RuntimeStatus.model` shows a model pulled on the side;
- the card prose flickers between the two writes;
- deep mode on qwen3:1.7b hit `invalid_tool_calls`.

## Deployed image impact

The R1 fix touches only dashboard-ui (`constants.ts`). **No analyzer file changed, so the deployed `telark/analyzer:0.1.5` image does not need a rebuild.** Fixing M1 or M2 later would require one.
