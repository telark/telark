# Critique — local-ai-analyzer-recommendations.plan (.md + .json)

Adversarial review of the plan against the working trees on 2026-09-24 (telark services/{analyzer,discovery,exporter}, charts/telark, internal/{data,rest,kcore,x-ware}, dashboard-ui, landing-page, scripts/local-build-push.sh). Every Critical/High and every cheap Medium is applied to both plan files; the JSON is valid (`python3 -m json.tool`), and a P1 revision-log row records the pass.

Counts: **1 Critical, 13 High, 20 Medium, 6 Low**. Applied: 1/1 C, 13/13 H, 17/20 M, 2/6 L.

## Critical

| # | Finding | Evidence | Fix | Applied |
|---|---|---|---|---|
| C1 | The image-pull sub-reasons are classified from event messages that are already cut to 160 chars. containerd puts the markers after char 160 (`… failed to resolve reference "…": not found`, `… pull access denied, repository does not exist …`, `… dial tcp: lookup … no such host`). As written, ib-notfound, ib-denied and ib-unreach all land on `image_pull.other`, so V2 can never pass. The scheduling and eviction parsers read the same truncated text. | `tools/k8s_tools.py:193` (`message[:EVENT_MESSAGE_MAX]`), `constants.py:426` (`EVENT_MESSAGE_MAX = 160`), `:427` dedupe 120; `_pod_summary` (k8s_tools.py:76-89) drops `state.waiting.message` | S3 keeps an untruncated `fullMessage` on each `run.events_cache` entry. It stays in memory only: never persisted, never in a model-visible tool result, never logged. S3 also keeps the container waiting message. S4 classifies from these. Fixtures use real messages longer than 160 chars. | yes (S3, S4) |

## High

| # | Finding | Evidence | Fix | Applied |
|---|---|---|---|---|
| H1 | Readiness detection requires the prefix `Readiness probe failed`. Exec timeouts and runtime errors produce `Readiness probe errored: … timeout 1s exceeded`, so the ib-ready-timeout seed (exec `sleep 3`, timeout 1 s) is never classified. | Feature B row `probe_failure.readiness`; kubelet prober wording | Match `failed` or `errored`. | yes |
| H2 | Reviews write the document but do not bump `version`. The UI refetches only when `e.data.version > version`. Discovery treats a document with version 0 and an empty `lastRun` as legacy and hides it. So a review-only app would never show recommendations, and new recommendation cards would never refresh an open page. | dashboard-ui `hooks/useApplicationInsights.ts:115`; discovery `internal/discovery/cache/cache.go:39-42`; `insights.py:251-253` (only `stamp_run` bumps) | Every review write bumps `version` and never touches `lastRun` (D7, S10, S11). | yes |
| H3 | `reliability.no_startup_probe` reads liveness `Unhealthy` events but declares only family W. A sweep review has no events, so the card an Analyze review creates is resolved by the next sweep. The card flaps. | catalog row 7; `review.gather` reads no events (S7) | New family E: this run's `events_cache`, complete only in-run. Without E, only the "startupProbe present" resolve branch is evaluated. | yes (families, row 7, S7, S8) |
| H4 | `change_risk.high_velocity` reads discovery's `changeVelocityPerDay`. Discovery computes it as total / (last − first) and returns 0 below a 1 h span, so rl-churn's changes "within minutes" give 0 and V3 can never pass. The value also never decays, so an app that churned once long ago is flagged forever. | discovery `internal/core/applications/metrics/derived.go:94-122`, `metrics/config.go:9` (`VelocityMinHistory: time.Hour`) | The rule computes a 7-day rate from `history.changeLog`. V3 sets the threshold to 1/day. rl-churn patches are spaced ≥ 15 s apart so discovery does not merge them (its coalescing window is 2 s, max wait 10 s, `constants/informers.go:13-14`). | yes (row 40, V3) |
| H5 | V3 seed contradictions: (a) rl-sec-a is privileged and adds NET_ADMIN, and expects `security.added_capabilities`, but rule 27's guard reports privileged containers under `privileged` only. (b) The lifecycle check patches rl-img to `:mainline` and expects `updated`, but the rule flags only `latest` or no tag, so the card resolves instead. | catalog rows 27 and 34; V3 lifecycle | Move NET_ADMIN to rl-sec-b and update both expected sets. Patch rl-img to the untagged `nginxinc/nginx-unprivileged` instead. | yes |
| H6 | S16 gates the page with "the existing applications view entry of `ACTION_PERMISSIONS`", but that entry does not exist. The Applications route is only a bare `<ProtectedRoute>`. An agent following S16 would invent a key, which breaks D17. | `permissionEngine.tsx:9-45` (viewRollbacks, viewSnapshots, viewSnapshotManifest, edit, forceSync, delete, rollback); `AppRoutes.tsx:90-94`, Groups pattern at `:134` | Use `<ProtectedRoute requiredScope='applications' minimumLevel='ReadOnly'>` on the route and `usePermission('applications','ReadOnly')` on the sidebar button. | yes (S16) |
| H7 | The sweep lists apps through the 5 s `EXPORTER_TIMEOUT_S`. At 1 012 apps the full list took 1.7 s, and discovery's 3 s list timeout already failed there (stress memory). At 2 000 apps the tick would be skipped forever. | `constants.py:266`, `exporter.py:37-38` | Add a dedicated `EXPORTER_LIST_TIMEOUT_S = 30`. | yes (S7, S11) |
| H8 | `security.token_automount` fires on any projected `serviceAccountToken`. On EKS, IRSA and Pod Identity inject an audience-scoped STS token even when `automountServiceAccountToken: false`. That produces a false positive on telark's own target platform. | catalog row 31 | Count only a projection with no `audience` (the `kube-api-access-*` volume). | yes |
| H9 | `networking.service_selector_mismatch` fires whenever the pod query returns 0. That happens for every replicas-0 or scaled-down workload whose selector is correct; the stress layout is full of replicas-0 Deployments. | catalog row 37 | Fire only when no app workload's template labels match the selector **and** there are 0 pods. | yes |
| H10 | Redis capacity is not budgeted. The redis subchart runs the bitnami `nano` preset (192Mi / 150m) with no override. The real document size with 40 recommendations and 7 days of resolved cards is about 40–60 KB, not "~25 KB", so 2 000 apps need roughly 80–120 MB plus the usage hash. | `charts/telark/values.yaml:408-420`; `charts/redis-23.0.10.tgz` values.yaml:317 (`resourcesPreset: "nano"`), common `_resources.tpl` (nano limits 192Mi / 150m); plan edge case "~25 KB" | V1 records the Redis limit. V5 adds run B (3 incidents + 40 recommendations with 16 params each), gated at ≤ 50% of the Redis limit and ≤ 70% of discovery's memory limit; a miss becomes a chart-value fix. Risk entry added and the edge case corrected. | yes |
| H11 | The landing-page `api-reference.mdx` lists every route but S17 does not touch it, so the docs would be incomplete. | landing-page `content/docs/reference/api-reference.mdx:27, 68-76` | Add the file and both routes to S17, and add a grep to S17's verify. | yes |
| H12 | Usage rules 14–17 carry one container's numbers, and underprovisioned severity differs by resource (memory = warning, CPU = info). D6 allows one card per (workload, rule), so rl-size (c1 cpu + c1 memory) and mixed severities collide in one card. | D6; catalog rows 14–17; rl-size seed | Their id hashes `subject#<container>/<resource>`. | yes (D6, S8) |
| H13 | The params cap of 12 is below what `crashloop.probe_kill` needs: workload, namespace, pod, container, probe, restarts, failure, port/status/timeout, message, ready, desired, generation, change = 13. A key would be dropped nondeterministically and a template placeholder would break. | D3, S1, S4 | Raise `MaxInsightParams` to 16 and add a max-param-set test. | yes (D3, contract, S1, S4, V2, V3) |

## Medium

| # | Finding | Evidence | Fix | Applied |
|---|---|---|---|---|
| M1 | Check (a) in S5 already exists: `_faithful` rejects numbers that are not in the source. | `analyzer.py:302`, `constants.py:525` | Keep it; only widen `source` to include the template title and summary. | yes (S5) |
| M2 | S4 needs `image_parts` (from S6) and the full image. The status payload strips the registry. | `k8s_tools.py:121` | S4 depends on S3 and S6; `spec_cache` keeps full images. | yes |
| M3 | Candidates without a status read (more than `FAST_STATUS_MAX`=3 workloads with events) lack pod/container/exitCode, so "every placeholder is a guaranteed param" is false. | `rules.py:98-101, 268-272`; `constants.py:458` | Declare required params per reason, fall back to an event-only detail variant, and test with `status=None`. | yes (S4) |
| M4 | `resource_pressure.preempted` is unreachable: the fast path reads warnings only and `Preempted` is a Normal event. The `OOMKilling` path of `oom.node` is a Node event the workload matchers never own. | `models.py:207-208`, `analyzer.py:250`, `rules.py:85-87` | Detect preemption from the `DisruptionTarget`/`PreemptionByScheduler` pod condition; drop the OOMKilling path. | yes |
| M5 | The `401`/`403`/`429` markers match as substrings, so image tags and IPs trigger them. | Feature B row `image_pull.unauthorized` | Use whole-word regexes. | yes |
| M6 | `no_readiness_probe` fires on "declares a containerPort" alone (metrics ports, sidecars). "Selected by a Service" at pod level flags every container in the pod. | catalog row 4 | Fire only when a selecting Service's targetPort resolves to this container's port; if S is incomplete, do not evaluate. | yes |
| M7 | "App container" is undefined, so injected sidecars (istio-proxy/istio-init with NET_ADMIN, root) would be judged by security and resources rules. | catalog rows 4–33 | Judge template containers only; resources are matched by name on the pod; security rules also cover template initContainers. | yes (S7) |
| M8 | `plaintext_secret_env` false positives, e.g. `JWT_SECRET_NAME=my-secret`, `TOKEN_ENDPOINT`. | catalog row 33 | Add skip suffixes `_NAME`, `_REF`, `_ENDPOINT`, `_HOST`, `_PORT`, `_ISSUER`, `_AUDIENCE`, `_EXPIRY`, `_HEADER`, `_MODE`, `_TYPE`, `_ENABLED`. | yes |
| M9 | `resources.overprovisioned` claims high confidence, but samples come once per review (~2 h). Daily peaks are missed, and lowering requests on that basis can cause OOMs. | D10 | Confidence set to medium. | yes |
| M10 | The usage strings are kcore's format (`%dm` / `%.2f` cores, `%.2f` + `B\|Ki\|Mi\|Gi`), not Kubernetes quantities. | kcore `metrics/metricsutils/converter.go:16-18`, `constants/formatting.go:5-27`, `metrics/usage.go:36-40` | S6 parsers accept these formats. | yes |
| M11 | The in-run review runs on the single worker (wall up to 20 s) and delays queued incident jobs. | `main.py:238-304, 376` | Skip the in-run review when `queue_len() > 0`; the sweep catches up. | yes (D7, S11) |
| M12 | Four namespace lists per app are re-read for each app in the same namespace (the stress layout has 100 apps per namespace). | S7; guideline §3 "slow-storage reads are cached" | Cache namespace lists per sweep tick. | yes (S7) |
| M13 | The rl-mem seed OOM-loops: 58 MiB held plus the interpreter exceeds 64Mi. Seeds that replace the command or add c2 break the base probes, which fires `no_liveness_probe`/`no_startup_probe` outside the expected sets. | seed table | Size the allocation from the RSS measured in V1; add seed invariants (keep 8080 serving; c2 copies the securityContext and an exec liveness probe). rl-net-a gets a grouping label. | yes |
| M14 | S17's verify can never fail (`\| tail -3` masks the exit code), and landing-page uses pnpm. V5's `grep -c NotFound` passes with 1 of 4. | landing-page `package.json:11`, `pnpm-lock.yaml` | `pnpm run build`; compare the count to 4. | yes |
| M15 | D22 links to `/applications/<ns>/<name>` but the route is `/applications/:name/details`. UI texts say "Governance → Protection plans" and "History", but the sidebar shows "Protection Plans" under Discovery, and the details page labels its sections "Snapshots" and "History Changes". | `app.ts:25`, `menu.ts`, applications `texts.ts:193, 328` | Fixed in D22 and row 42. | yes |
| M16 | The ETag ignores time, so rows that go stale keep returning 304. | D21, S12 | Hash in the stale bucket. | yes (S12) |
| M17 | V5's synthetic documents would be deleted by the sweep's cleanup (the exporter does not list those apps). | S11 cleanup, V5 | Run the scale test with the sweep off. | yes |
| M18 | V1 describes local-build-push as a go.work build. It is not: it does a temp-context `go mod edit -replace` plus a Dockerfile build. V1 also restarts twice and runs analyzer before the RBAC exists. | `scripts/local-build-push.sh:59-67, 139-181` | Use `-n` for no restart, then a single helm-upgrade rollout; check that the log shows `replaced …`. | yes (D24, V1) |
| M19 | A no-op triage (acknowledge twice) would still bump the version (guideline §3 "reject no-ops that mutate"). | D15 | Return 200 with no write. | yes (S10) |
| M20 | The dependsOn values block parallel apply without reason: S14 waits on S11 but needs only the codes from S9; S13 waits on S7/S12 but its knobs and RBAC are fixed by the plan. | steps | S14 → [S9]; S13 → [S1, S2]. | yes |

Medium, not applied (deliberately):
- The sparse usage sampling itself: moving sampling into the sweep would still see only reviewed apps. Accept it with medium confidence and the sample count in the text.
- Info-level noise on unhardened clusters (rules 5, 24, 25-unverified, 26, 30, 31, 39 fire on most workloads; cap 40 per app). Already a stated risk. Revisit after the first real cluster review.
- `replicas_same_node` fires on every multi-replica workload of a single-node cluster (true but not actionable). Detecting the node count needs nodes RBAC, which D11 excludes.

## Low (not applied unless noted)

- L1: `Row.summary` is unused by the table. Dropped (applied in S12).
- L2: A sticky header covers the `#insights` target. `scroll-margin-top` added (applied in D22).
- L3: `docs/architecture.md` diagram (lines 16-49) lacks the discovery → `analyzer:index` edge. Optional.
- L4: `triage.by` stores a userID, so Details shows an opaque id. Resolve it through the users store later.
- L5: The index maps environments only through the primary namespace (the document key); a namespaces-scope plan covering a secondary namespace is missed.
- L6: When analyzer is disabled (`services.analyzer.enabled=false`), triage buttons would hit a missing upstream. Hide them when the runtime endpoint 404s.

## Verified correct (no change)

- Discovery really drops unknown fields on re-encode: `cache.go:35`, typed `json.Unmarshal`, handler returns `*AppInsights`. Rollout order is D4/V1.
- Plan `scope.applicationIds` are app names: `plans/protection/applications/resolver.go:26-33`. D13/D20 hold.
- The service token passes every exporter requirement (x-ware `authz/handler.go:75-79`). The plans and categories routes exist (`rest/endpoints/plans/def.go:8`, `classification/category/def.go:9`). The summary view keeps `history.generation` and namespaces (`exporter/internal/exporters/application/list.go:59-91`).
- auth-service returns `userID` (`auth/internal/handlers/authorisation/types.go:26-29`).
- Analyzer RBAC already has cluster-wide pods list, so selector checks need no new rule. miniredis is already in discovery's go.mod.
- `insights:jobs` is a stream: `queue_len` = XLEN after XACK+XDEL (`insights.py:111-117`).
- `resolve_observed` would KeyError on `service/…` subjects (`insights.py:227`). The plan's category scoping (S10) is required, not optional.

## Deploying discovery to telark-dev without a release

`scripts/local-build-push.sh -n discovery`. For each module in `-r data,kcore,rest,x-ware` (the default), the script checks whether `../internal/<mod>` differs from the version discovery's go.mod pins (commits after the tag, uncommitted edits, untracked files, or a missing tag). Each module that differs is rsynced into a temp copy of `services/discovery`, then the script runs `go mod edit -replace …=./_local/<mod>` and `go mod tidy`. It rewrites the Dockerfile so COPY runs before `go mod download`, builds for the node arch, and pushes to `<registry>/<repository>:<services.discovery.version>`, the tag the chart already pins.

The repo's go.mod, go.work, chart and versions stay untouched, and nothing is committed. The subsequent `helm upgrade` rolls the deployment (pullPolicy Always pulls the tag). Analyzer is Python and is not in the script, so it is built with `docker buildx` at its pinned tag.

## Parallel apply grouping (no file conflicts)

1. **Wave 0:** S1 (internal/data) ∥ S2 (internal/rest).
2. **Wave 1**, all in parallel:
   - Analyzer lane: S3 → S6 → S4 → S5 → S7 → S8 → S9 → S10 → S11. This lane must be serial because every step edits `constants.py` and `models.py`.
   - S12 (services/discovery).
   - S13 (charts/telark + docs/INSTALL.md).
3. **Wave 2** (dashboard-ui, serial because S14/S16 share `models`, `constants`, `texts`, `clients` and `endpoints`): S14 → S15 → S16. It starts once S9 is done, runs beside S10/S11, and S16 also needs S12.
4. **Wave 3:** S17 (landing-page) after S13 and S16. It can overlap S15/S16 if written from the plan text.
5. **Then serially:** V1 → V5 → R1.
