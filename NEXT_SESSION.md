# Next session: deploy everything and verify every task live

Status as of 2026-09-24 00:50 CEST. Nothing below is committed unless stated. Do not commit, tag, bump versions or edit Chart.yaml unless the user asks.

## 1. What was done

| # | Work | Plan / design | State |
|---|---|---|---|
| 1 | Protection Plans environments + tags (categories) | `.claude/plans/protection-plans-environments-tags.plan.md` | Applied, reviewed, **pushed by the user** |
| 2 | Protection Plans reports (generate, list, download, ledger, reports PVC, GC) | `.claude/plans/protection-plans-reports.plan.md` | Applied, reviewed, **pushed by the user** |
| 3 | Global cleanup pass (all services, shared packages, dashboard-ui; discovery lint baseline 448 → 0) | – | Applied, **pushed by the user** |
| 4 | Protection Plans approval mode (automatic vs requires-approval, production default, approve/reject, per-plan lock, notifications) | `.claude/plans/protection-plans-approval.plan.md` | Applied, V5 review passed, uncommitted |
| 5 | Go dependency bumps: kyverno 1.19.1, k8s.io api/apimachinery/client-go 0.37.0 (x/time stays indirect) | – | Applied in internal/{data,rest,kcore}, exporter, discovery; uncommitted |
| 6 | dashboard-ui dependency bumps (antd 6.6.5, react 19.3, react-router 7.18, RTK 2.12, framer-motion 13, TS 6.0.3, vite 8.3.0) | – | Applied, check-all + build green; uncommitted |
| 7 | Local AI Analyzer: analyzer stays **Python**, refactored in place (Ollama only, 4 read-only tools, insight lifecycle, Redis-stream triggers, live SSE updates, provider keys removed, Ollama off by default, pre-upgrade hook deletes the old provider-key Secret) | `.claude/plans/local-ai-analyzer.design.md`, `.claude/plans/local-ai-analyzer.plan.md` | Applied, R1 review 0 critical/high, uncommitted |
| 8 | Protection Plans permissions end to end (specific action + deny rule for every plan action/option, UI gating = backend) | `.claude/plans/protection-plans-permissions.plan.md` | Applied (8/9 steps; U5 covered by APPS AP2, see §2), uncommitted |
| 9 | Protection Plans exclusions (applications scope: kinds + resources; namespaces scope: kinds only) | `.claude/plans/protection-plans-exclusions.plan.md` | Applied (10/10), uncommitted |
| 10 | Applications cards redesign (protection-plans card design, view-mode dropdown 3/row or 1/row, "covered by plans") | `.claude/plans/applications-cards-coverage.plan.md` | Applied (5/5), uncommitted |
| 11 | Docs + landing-page stale-data sync | – | Done: 8 telark/dashboard-ui doc files, 29 landing-page files (type-check, lint, build green); uncommitted |
| 12 | dashboard-ui Vite ESM fix (`"type": "module"`, `import.meta.dirname`) | – | Done, uncommitted |
| 13 | build-ui GHA: sync dashboard-ui package.json version before the image build, commit + tag that commit | – | Done (`.github/actions/release-service-image/action.yaml`, `.github/scripts/tag-service-repo.sh`), actionlint + shellcheck clean, uncommitted |

Uncommitted trees: `telark`, `/Users/houssem/Desktop/dashboard-ui`, `/Users/houssem/Desktop/Github/internal/{data,rest,kcore,x-ware}`, `/Users/houssem/Desktop/Github/landing-page`. A plugin hook keeps staging files; unstage with `git reset -q` (never `git restore`/`checkout`).

Routing: `.claude/WORKFLOWS_MODEL_ROUTING.md` (budget mode until 2026-09-29: Opus 5.5 does the work, effort floor high, xhigh for Major/verification, Fable only for a plan synthesizer).

## 2. Orchestrator outcome

`wf_8cfe7cf5-97c` finished 2026-09-24 01:40 (49 agents). Integration gate **allClean**: every Go module 0 lint issues + tests green, all Go services build, analyzer pytest 174 passed (99 % coverage), both charts lint + render in all modes, dashboard-ui check-all 0 errors + build green. Cross-feature review: PASS, no high defect in PERM/EXCL/APPS code; reverted one cleanup refactor in `services/discovery/internal/core/plans/protection/svc.go` (rollback path).

**Must handle before any commit:**
- `internal/rest/go.mod` carries a local `replace github.com/telark/data => /Users/houssem/Desktop/Github/internal/data` (added by an unknown process at 00:37). Never commit it; release order is tag data → bump rest's data pin → tag rest → bump service pins.
- Something outside the agents keeps staging every change in all repos (not the security-guidance hook, which only uses `--intent-to-add`). Everything was unstaged at 01:45; check `git diff --cached --stat` before committing.

**Open questions from the plans (answer or accept defaults):**
- PERM D3: approve and reject have separate keys (new `rejectprotectionplan`) instead of one decide key.
- PERM D11: denied plan actions are shown disabled with a "You do not have permission" tooltip instead of hidden.
- PERM D9: no separate "skip approval" key; approvalMode is covered by create/duplicate.
- PERM D6: exporter plan DELETE is now Internal-only; users delete through discovery.
- EXCL: Kyverno acceptance of `exclude.any {kinds+names}` beside a `*` match is gated by the real-validator test `TestKyvernoAcceptsExclusions`; verify live.
- EXCL: namespaces-scope excluded kinds use the fixed 14-kind list.
- APPS: scheduled and pending_approval plans show as "upcoming" on the card; a multi-namespace app counts as covered when any of its namespaces is in a namespaces-scope plan.

**Open medium defect:** EXCL `useScopeExclusionOptions` fetches application details for every selected app at once (no concurrency cap); at ~2,000 apps this should batch or load lazily. Low: discovery exclusion validation lacks kind/namespace length checks.

## 3. Deploy (agent runs all of this itself)

1. **Cluster access.** On 2026-09-24 00:45 the EKS endpoint did not resolve (`kubectl` → `no such host`). Check first:
   ```
   kubectl config current-context
   kubectl get nodes
   ```
   If unreachable: `aws eks update-kubeconfig --region eu-west-3 --name telark-dev`, then ask the user whether the cluster is up. Infra changes are Terraform edits in `/Users/houssem/Desktop/Github/infra` only, never `aws`/`eksctl` against the cluster.
2. **Go services** (discovery, auth, exporter, notifier): `./scripts/local-build-push.sh -s` (smart mode: picks every service with local changes and ships unreleased `internal/*` modules through replace pins; pushes to the tag the chart pins; restarts the deployment). Logs under `/tmp`.
3. **analyzer (Python)** is not in the build script. Build its image from `services/analyzer/Dockerfile` with the same registry, repository and tag the chart pins (`yq '.services.analyzer' charts/telark/values.yaml`), `docker buildx build --platform <same platform as the script> -t <image> --push services/analyzer`, then `kubectl rollout restart` its deployment. Do not edit the script unless the user agrees.
4. **Charts from the local checkout** (CRDs first: GlobalConfig ai and ProtectionPlan approval/exclusions fields changed; unknown fields are pruned silently):
   ```
   helm upgrade --install telark-crds charts/telark-crds -n telark
   helm dependency build charts/telark
   helm upgrade --install telark charts/telark -n telark --reset-then-reuse-values --set app.persistence.storageClass=efs-sc
   ```
   Use `--reset-then-reuse-values` (plain `--reuse-values` keeps OLD chart defaults). Keep existing dev overrides. Ollama stays disabled by default; the analyzer test (§4) enables it with `--set app.ollama.enabled=true` only when that test runs. The pre-upgrade hook deletes `telark-ai-provider-key`.

   **Node capacity (2026-09-24).** The AWS account is on the free plan, which only launches free-tier types, so xlarge nodes fail with `InstanceLaunchFailures: not eligible for Free Tier`. The cluster runs 1 on-demand `m7i-flex.large` (2 vCPU / 8 GiB) plus 1 spot `m7i-flex.large` or `c7i-flex.large` (the spot one can have only 4 GiB). Each node has about 1.93 vCPU allocatable. Rendered requests: the release without Ollama is 1700m / 2.2 GiB (2500m / 3.2 GiB at HPA max); Ollama's default is **2 CPU / 5 GiB**, more CPU than any node can offer, so the pod stays Pending forever. For the analyzer test, enable it with:
   ```
   --set app.ollama.enabled=true --set ollama.resources.requests.cpu=1
   ```
   The CPU limit stays 4, so Ollama can still use idle CPU. It can only fit on the 8 GiB `m7i-flex` node. If analysis is too slow, use the README's CPU small profile (`qwen2.5:1.5b`, `ollama.resources.requests.memory=3Gi`, `ollama.resources.limits.memory=4Gi`). Expected on this cluster, not bugs: some pods Pending when every HPA is at max, and part of the release Pending while the spot node is reclaimed. Check `kubectl describe pod` for `Insufficient cpu/memory` before treating a Pending pod as a defect.
5. **Port-forwards:** kill stale ones, then run `scripts/local-port-forward.sh` in the background; confirm with `pgrep -fl port-forward` before calling localhost.
6. **UI:** the user runs `npm run dev` in `/Users/houssem/Desktop/dashboard-ui`. For the live tests the agent **opens a browser window and drives the UI itself** (Claude in Chrome tools, as in earlier test rounds: `tabs_context_mcp` first, then a new tab on the dev server URL, log in, click through every flow, read console + network, take screenshots as evidence; record a GIF for multi-step flows). Backend checks still use API calls, `kubectl` read-backs and logs next to the UI checks.

## 4. Live verification (every task, 100 % evidence)

For each row (UI side in the browser, backend side via API/kubectl/logs): derive the test cases from the plan's **Phases → acceptance** and **Edge cases** sections, run them against the live cluster (API calls through the port-forwards with a real session, `kubectl get -o yaml` read-back of CRs, Redis via `kubectl exec` in the telark namespace, pod logs), and record PASS/FAIL with the exact evidence (request, response, CR field, log line). A task is done only when every acceptance line has evidence. On FAIL: root-cause, fix, rebuild only the affected service, re-test.

| Task | Must prove live |
|---|---|
| Env + tags | create/edit/duplicate with environment + tags persist on the CR; filters; category CRUD gated |
| Reports | generate/list/download html/md/json/csv; nosniff + CSP sandbox headers; ledger checkpoints; GC removes orphans; reports PVC bound |
| Approval mode | production env defaults to requires-approval; plan parks in pending_approval with nothing deployed; approve deploys, reject cancels; self-decision 403; concurrent decide/update → 409; history capped at 20; notifications sent |
| Dependency bumps | all pods Running, no restart loops; kyverno policies still render and admit/deny as before |
| UI deps bump | UI loads, no console errors on the pages the user checks; transitions and Select sizing unchanged |
| Local AI Analyzer | disabled by default: no Ollama pod, no impact; enable → Ollama deployed, model pull progress in Settings, validation rejects a model without tools; manual Analyze produces insights with evidence refs; live SSE updates; namespace filtering; old provider-key Secret gone |
| Permissions | 16 protection-plans action keys; for each plan action: a role without the permission is denied (backend 403 AND UI hidden/disabled), the deny rule blocks an otherwise-allowed user, builtin roles behave as the plan says |
| Exclusions | applications scope excludes a kind and a specific resource; namespaces scope excludes kinds only (no resource picker); rendered policies carry the excludes; material-edit rules; duplicate copies exclusions; old plans unchanged |
| Applications cards | new card design, dropdown 3/row ↔ 1/row, each app lists the plans covering it (by app and by namespace), correct at the current app count |
| Docs + landing page | spot-check each changed claim against the live product |

Write the results into §5 as you go, then a final summary.

## 5. Results

Run 2026-09-24 (fresh `telark-dev` cluster: 2× `m7i-flex.large`, 1 on-demand + 1 spot).

**Deploy.** All 4 Go images and the analyzer image were built and pushed to the pinned tags. Instead of installing the CRDs chart separately, this fresh install used one `helm upgrade --install telark charts/telark -n telark --create-namespace --set app.persistence.storageClass=efs-sc` (the local `telark-crds` is bundled through `file://`). Result: 15 pods Running with 0 restarts; exporter snapshots and reports PVCs Bound on `efs-sc` (RWX); 9 CRDs installed. Seed apps: `shop-prod`, `shop-dev` (`web`, `cart`), `payments-prod` (`ledger`, `billing`), each a Deployment + Service + ConfigMap.

| # | Area | Result | Evidence / fix |
|---|---|---|---|
| R1 | UI deps / Vite ESM | FAIL → fixed | `npm run dev`: `storage.getItem is not a function` (redux-persist). Cause: `"type": "module"` makes Vite 8 apply Node ESM interop, so the default import of the CJS `redux-persist/lib/storage` resolved to `{ default }`. Fix: `src/store/persistConfig.ts` imports `redux-persist/es/storage`. Login page renders with 0 console errors; check-all 0 errors. |
| R2 | UI deps / antd 6.6 | FAIL → fixed | Message text rendered white on the white toast. antd 6.6 renders the message as the notice title (`colorTextHeading`, white under the dark algorithm). Fix: `App.tsx` Message token `colorTextHeading: TEXT_ON_SURFACE`. Live: `.ant-message-notice-title` color `rgb(17,24,39)` on `rgb(255,255,255)`. |
| R3 | Env + tags | PASS (21/21, 1 N/A) | Create/edit/duplicate persist `environmentID`/`tagIDs` (UI plan `ui-freeze-prod` → `cat-00002-0001-0001`, 2 tagIDs); category CRUD gating READER 403 / CONTRIB create-only / Owner+Admin all; list filtering is client-side by design (D11). Details: scratchpad `results/env-tags.md`. |
| R4 | Exclusions | PASS after fix (24/26 → fixed) | Excludes rendered + Kyverno Ready + admission proves exclude (Enforce); ns scope rejects resource exclusions (UI shows kinds only); material-edit rules, duplicate copy, rollback on partial failure. FAIL F1: excluded resource kind/namespace not length/subresource-checked (API server 422 → 503/500). Fix: `validation.go invalidExcludedResource` reuses `invalidExclusionKind` + `ExclusionNamespaceMaxLength` (internal/data) + 3 test rows; live → 400. |
| R5 | Approval mode | PASS (36, 1 partial, 2 blocked) | Production defaults to required, parks with 0 Policies, approve by other user deploys (UI plan: `active`, Policy Ready with ConfigMap exclude), reject cancels, self-decision 403 (UI Approve/Reject disabled for requester), decide×update/cancel races → exactly one 409, Redis lock cross-replica, history cap 20, pending plan terminated at endAt, notifications to approvers/requester. Fixes: invalid decision on non-pending plan 409 → 400 (input validated before state, `approval.go ValidateDecision`); approval-request notification shows requester username instead of raw id (`notify.go displayName`). Open (design): approvers without view permission still notified; deleted requester still notified; requestedAt 1 s resolution. |
| R6 | Permissions | PASS (15, 0 fail) | 539 authz calls / 39 roles, 0 mismatches: 16 keys present; single-permission roles allowed, all others 403; deny rules win; builtin matrix exact; exporter plan DELETE internal-only (401), discovery clear works. |
| R7 | Reports | PASS (21, 2 blocked live) | Generate/list/download 4 formats with nosniff + CSP sandbox; boundary captures after terminal patch; ledger checkpoints on PVC; busy → 429 + Retry-After (plan decision); keep-10; authz; delete removes reports. Fix: coverage "Checkpoint interval" showed the smallest ledger gap (23 s) → renders the configured interval (`Generator.checkpointEvery`). Blocked: GC sweep (first tick ≈13:39 UTC, planted files) and exporter-4xx-through-discovery (unit-tested). |
| R8 | Discovery messages | FAIL → fixed | Plan handlers returned the raw `resource %s of kind %s …` format string → `planMessage()` formats id + kind (templates → list message, report → record created). |
| R9 | Applications cards | PASS | Grid 3/row ↔ list 1/row via icon dropdown marking the active mode, persisted across reload; card header / 4 stats / Protected by / footer; active chips green shield, upcoming orange clock with titles; multi-namespace `web`/`cart` covered via `shop-dev` only; bulk toolbar = Search, View, Exit bulk. Low: excluded kind displayed lowercase (`configmap`) on plan details. |
| R10 | User UI requests (live round) | Done | (1) Create/Edit plan panel uses the Roles section style: Details · Classification · Execution & approval · Scope · Policies · Schedule · Participants; 0 horizontal scroll containers. (2) Plans list responsive like Applications (useElementWidth): 3/2/1 per row at 1440/1000/500, toolbar folds phase pills into one dropdown + icon-only below `LIST_TOOLBAR_COMPACT_WIDTH`. (3) Applications cards: no text-box overlaps at 1440/1250/1190/1100/800/520; grid 3→2→1 by measured width (`CARD_MIN_WIDTH_PX`, `getCardGridColumns`); View control hidden when only one card fits. (4) Generate report moved to the details toolbar + card kebab; Reports section only lists/downloads (live: toolbar generate → "Report generated.", list 1 → 2). check-all 0 errors. |
| R11 | Local AI Analyzer | FAIL → fixed (in progress) | Every analyzer read (runtime, SSE events) returned 403 even for Admin: `authz.py _resolve` read `data.roles` but auth `auth/permissions` returns the bare `{userID, roles}` → Settings showed "Unreachable". Fix: read top-level `roles` (+ test fixture). Ollama on this cluster needs `ollama.resources.requests.cpu=500m` (1 CPU did not fit beside the test workloads). Live: runtime `model_missing` → "Install model" → progress 20→62→99 % → Ready. Validation: qwen3:4b 200 (tools), gemma3:270m 422 `model_lacks_tools`, bad name 400, READER 403; UI picker offers only the 4 tool-capable catalog models. Enable/Save → GlobalConfig `ai {enabled:true, autoAnalyze:false}`; SSE live state transitions without reload PASS. **BLOCKED — redesign:** (a) Ollama counts the pull's page cache (`memory.stat file=2.58 GB`) as used and refuses to load qwen3:4b (3.8 GiB needed, 3.5 free of 6 GiB) until the pod restarts; (b) on 2-vCPU nodes one qwen3:4b chat exceeds the 120 s step timeout, qwen2.5:1.5b also >100 s. User decision: analyzer must answer in seconds on small nodes, fully free/open-source, no API keys; air-gapped vs connected (open models only). Research in progress → `.claude/plans/local-ai-analyzer.research.md`. |
| R12 | Reports GC | PASS | 13:39:09 `reports sweep removed=1 temps=1 scanned=39`: old orphan dir + old temp removed, fresh orphan + fresh temp kept. |
| R13 | Permissions UI | PASS | READER: Create disabled + "You do not have permission to create protection plans", card menu Edit/Duplicate/Generate/Cancel/Delete disabled. pm-deny-view: no sidebar entry, deep link → `/`, no `plans/protection/get`. pm-deny-gen: Generate disabled + "You do not have permission to generate reports", downloads + Edit enabled. CONTRIB on pending plan: Approve + Reject disabled with their tooltips. pm-deny-appr: Approve disabled, Reject enabled. |
| R15 | Docs + landing page | 62/67 OK → 5 fixed | Wrong claims corrected: final report only for plans that ran (features/concepts/introduction/copy.ts/features-page.ts/telark README); on-demand only once started; reports contain no approval decisions (removed); `failed` plans can be reactivated; violations capped by `limit`, not paginated. Analyzer docs excluded (redesign). Type-check + lint clean. |
| R16 | Cross-plan Reports tab (new feature) | Done, live | Exporter `GET /api/v1/reports/get` (planId, trigger, from/to, limit ≤1000, `X-Total-Count`), same authz as per-plan list; tests + lint clean; exporter redeployed. UI: Plans / Reports Segmented tabs (`?tab=reports`, gated on view-reports), table with plan/env/trigger/generated-by/violations + Download menu. Live: 23 reports, READER 200, bad trigger 400. Open: needs rest release + pin bump for `GOWORK=off` CI; per-plan card trigger labels (On demand / Final) differ from the tab (Manual / Plan canceled / Plan ended). |
| R17 | UI perf + toolbar (user report) | Fixed | Tab switch froze ~1.2 s: (1) 674 `Intl.DateTimeFormat` + 644 CURRENT_USER parses per switch → cached formatters + timezone keyed on the stored user string (`utils/shared/time.ts`); (2) pages remounted on every switch → visited tabs stay mounted, stable tabs element, memo Reports page, stable `refetch`; (3) `useNavigate` in every plan card re-rendered all cards on `?tab` change → card takes a stable `onOpen`. Result: switch ≈250 ms (first visit ≈1 s). Toolbar: one 1230 px threshold made all buttons icon-only at 1296 px → two-stage `compactWidth.QUICK_FILTER` (pills fold at 1230) + labels until 520 px (measured row ≈497 px). check-all 0 errors. |
| R14 | Dependency bumps / UI deps | PASS | 0 pods with restarts or not Running cluster-wide (excluding test namespaces); Kyverno admit/deny proven by EXCL/approval agents; UI: no console errors on home, app details, plan details, members, groups, roles, settings; Generate report from card menu → "Report generated." |

## 6. Orchestrator details

Journal: `~/.claude/projects/-Users-houssem-Desktop-claude/f560bcaf-1668-442f-bfec-249bd453a6dd/subagents/workflows/wf_8cfe7cf5-97c/journal.jsonl` (per-step results and deviations). Applied order: PERM S1-S3/U1-U4, EXCL S1-S6/U1-U2, APPS AP1-AP3, interleaved with per-module exclusivity.

## 7. Follow-ups agreed with the user

- **Insights inbox (after the analyzer redesign ships and is verified live):** cluster-wide page grouping AI insights across applications (open / updated / stale / resolved, triage, filters by namespace / severity / app, links to app details). Not before: the analyzer must first produce fast, automatic insights. The agent reminds the user when that condition is met.
- **Local AI Analyzer v2 (NEXT):** plan `.claude/plans/local-ai-analyzer-v2.plan.md` (+ `.plan.json`, research `local-ai-analyzer.research.md`): 6 phases, 15 steps, rules-first + one `granite4:350m` narration call (~5 s on 2 vCPU), qwen3:1.7b 4-vCPU tier, qwen3:4b GPU/deep; FOSS only, no API keys; air-gapped vs connected via chart `app.ollama.autoPull` + `runtimeUrl`. Defaults to confirm: D3 keep Ollama 0.17.7, D4 drop ollama memory limit (page-cache refusal), D5 mode chart-only, D6 port-443 egress, D11 dev images via local-build-push. Not yet critiqued: run one Opus critique pass, then apply.
- **Cluster state for the analyzer:** Ollama enabled (`--set app.ollama.enabled=true --set ollama.resources.requests.cpu=500m`), models qwen3:4b + qwen2.5:1.5b installed, GlobalConfig ai {enabled:true, autoAnalyze:false, model:qwen2.5:1.5b}; `shop-prod/web` deliberately broken (image `pause:does-not-exist`) as an incident fixture.
- **Low items:** excluded kind shown lowercase on plan details; unknown app routes render blank (no 404); approvers without view permission still get approval notifications; deleted requester still notified; requestedAt 1 s resolution; report 'Policies live at capture' label misleading on end/cancel; trigger label mismatch card vs Reports tab.
