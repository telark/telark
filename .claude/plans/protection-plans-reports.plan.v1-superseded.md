# Protection Plans — Reports generation (implementation plan)
**Status:** planned 2026-09-19, NOT executed. Nothing landed in any tree as of 2026-09-22.
**Critique verdict:** not executable as written — two critical + seven high findings (below) must be folded into the steps first.
**Mandatory invariant:** violations must be captured BEFORE `CleanupByPlanID` in discovery `svc.go` Terminate/Cancel — cleanup blanks `renderedPolicies`, and the violations filter then returns empty for a finished plan.
**Open decisions:** D1 native PDF vs print-optimised HTML; D2 whole-life coverage vs boundary capture + coverage callout; reports PVC size; hook placement across all six `CleanupByPlanID` call sites; who may generate/download.
**Prerequisites:** release `internal/rest` and `internal/data`, bump pins; chart adds a SECOND PVC — snapshots PVC name must stay byte-identical; `values.schema.json` has `additionalProperties:false` on `app.persistence`.
_Source: planning workflow `wf_98eac0b0-8b3` (11 agents) — raw result in `/Users/houssem/.claude/projects/-Users-houssem-Desktop-claude/bdc6e640-0d86-48b1-864a-5dbd0f2f88d0/workflows/wf_98eac0b0-8b3.json`. Remarks about a concurrent dependency-upgrade workflow and 40 staged exporter lint files date from 2026-09-19._

## Overview

> ⚠️ **PLAN ONLY — no code written.** Nothing below has been implemented. Confirm before I start.

# Protection Plan Reports

## Honest size-up

**This is a large change, and it touches the product's recovery path.** Not because the refactor is big — it isn't — but because of where the weight actually sits.

| | Weight | Danger |
|---|---|---|
| **Part A** — make snapshot utilities generic | Small. 5 functions, 4 call sites, all in one consumer file | **Low.** Perimeter around GC/retention/resolve is provably untouched |
| **Part B** — reports feature | Large. 2 route families, 2 shared-module edits, 4 renderers, a new PVC, first blob download in the UI | **Concentrated in two places** |

The two dangerous places, named up front:

1. **The chart PVC edit.** If the snapshots claim name drifts by one byte, Helm makes a new empty volume, every stored snapshot is orphaned, and rollback breaks. The whole chart design below is arranged so this cannot happen.
2. **The `svc.go` capture hooks.** They sit one statement from `CleanupByPlanID` on the plan teardown path. A hook that throws could stop a plan terminating.

Everything else is ordinary work.

---

## The seam: I picked the smallest one, and the census decided it

Three strategies were on the table. I grepped instead of guessing, and the census **falsified one of them.**

Every reference to the five candidate-shared symbols outside `utils/snapshot/`:

```
exporters/snapshot/def.go:40   WriteSnapshotJSON
exporters/snapshot/def.go:151  FormattedFileSize
exporters/snapshot/def.go:213  IsWithinBase
exporters/snapshot/def.go:539  IsWithinBase
```

**Four production call sites. One file.** Everything else is tests.

- The **extract-a-package-with-shims** angle was sized for 29 call sites and proposed 29 shims to hold `internal/exporters/` at zero diff. With four, that's ceremony — you just edit the four lines.
- The **thread-a-Store-through** angle existed to dissolve the `storageStats` singleton (`types.go:14`). Reports ship no storage panel, so the singleton is never contended by the second caller. It buys nothing today.

**Chosen: move exactly five functions into a new `internal/utils/artifact` package.** `WriteFileAtomic`, `IsWithinBase`, `ValidateIdentity`, `FormatBytes`, `FormattedFileSize`. Move, not copy — the originals are deleted so the compiler finds every reference.

### Why not the elegant version

The most attractive rejected idea was making reports versioned `V<n>.<ext>` artifacts, so retention and generation-resolution come free. It requires widening the version codec that `retention.go:25`, the GC's `isSweepable` predicate, and the stats counter all read independently.

That means **editing the delete predicate of an hourly volume sweeper that runs against the snapshots PVC** — to get free retention for a feature whose retention is fifteen lines of ReadDir-sort-Remove. Declined.

### On constraint C3

C3 says make the existing snapshot code generic, don't duplicate it. This honours it for **everything reports actually use** — atomic write, path containment, identity validation, byte formatting. Nothing is duplicated; `report/store.go` calls `artifact.WriteFileAtomic`, it doesn't reimplement it.

It declines to generalise GC and storage-stats, which reports never call. Generalising code the second caller never exercises isn't reuse — it's moving untested code into a package whose name now promises something unproven.

---

## Getting a trustworthy baseline without touching your unfinished work

Your tree is **not clean**, and I verified exactly how:

- 40 staged exporter files, **+1036/−684** — unfinished lint work including a heavily rewritten `tests/snapshot/logic_test.go`
- `MM services/exporter/go.mod` — staged *and* unstaged, because the dependency-upgrade workflow is mutating it right now

I'm not allowed to finish or revert either. So: **two baselines, zero mutation.**

**(a) HEAD baseline** — `git worktree add /Users/houssem/Desktop/Github/telark-baseline HEAD`

The path must be a **sibling inside `Desktop/Github/`** so the parent `go.work` still resolves the `internal/*` replaces. A worktree anywhere else silently loses them and everything fails for the wrong reason.

**(b) Working-tree baseline** — same three commands in the real tree, staged edits present.

**(b) is the comparison target.** (a) exists only to classify failures: in both → pre-existing; only in (b) → came from your lint work; in neither → I broke it.

Record the **golangci-lint issue count**, not "zero" — discovery's recorded baseline is 462 issues, auth's is 99. And never run `go mod tidy`; the dep workflow owns those files.

---

## Part A — refactor (steps 1–5)

| # | Step | Verify |
|---|---|---|
| 1 | Dual baseline | `git status --porcelain` byte-identical before and after |
| 2 | **Golden-file test for the cross-service contract** | Must pass on unrefactored code; rename `"path"` and confirm it fails |
| 3 | Create `utils/artifact` | Compiles with no snapshot-specific import |
| 4 | Delete originals, repoint 5 lines in `def.go` | `def.go` diff is exactly 5 lines |
| 5 | **Part A gate** | Four checks below |

### Step 2 is the highest-value file in Part A

`exporters/snapshot/def.go:136` builds its response with a **raw string literal** `"path"`. `discovery/internal/clients/snapshots.go:122` reads it back. A mismatch doesn't fail loudly — it degrades **silently** through `circuitbreaker.NotCounted`. Nothing protects this today. One golden file does.

### Step 5 — proving behaviour, not compilation

1. **Perimeter diff EMPTY** on `gc.go`, `retention.go`, `resolve.go`, `manifest.go`, `request.go`, `sanitize.go`, `types.go`, `exporters/snapshot/gc.go`, `managers/envs/snapshots.go`
2. **Golden contract** tests green
3. **Test parity** — the *same set* of passing and failing test names as baseline (b). Not "all green"; some tests are expected to fail from your lint work and must fail identically
4. **Lint parity** — count ≤ baseline. No `--no-config`, no new `//nolint`

---

## Part B — the feature (steps 6–20)

**Backend** 6–14 · **Chart** 15 · **Frontend** 16–18 · **Docs** 19 · **Gate** 20

Discovery gathers and renders; the exporter stores bytes. Follows the existing split and the domain-separation rule.

### The Events problem — and it's worse than the 1-hour TTL

`violations.go:34-36` documents `RetentionWindow = "1h"` as mirroring the apiserver `--event-ttl`. But the teardown path destroys the data **faster than the TTL does**:

```
svc.go:356  Terminate → CleanupByPlanID    (deletes rendered policies)
svc.go:361  →  BuildTerminatePatch
svc.go:491  →  FieldRenderedPolicies = []  (blanks the filter set)
```

`buildViolation` filters Events against that rendered set. **After a terminate, violations return empty immediately.** Not degraded — empty.

So a report generated after termination — the most likely moment anyone wants one — would show **zero enforcement activity for a plan that blocked thousands of requests.** That's not incomplete, it's misleading.

**Fix (step 13, in scope):** two hooks, one added statement each, *before* the `CleanupByPlanID` calls in `Terminate` and `Cancel`. Each persists `violations.Collect` output as a durable sidecar. Errors are logged and swallowed — **a plan must still terminate if capture fails.**

**Ceiling, stated plainly:** this captures the final window, not the plan's whole life. A three-week plan still loses week one. Those Events are already gone today.

**Therefore the coverage callout on every report cover is mandatory, not decorative.** Without it the document looks complete and isn't — worse than an empty report.

Full-life history needs continuous capture: a poller multiplying LIST load by active-plans × namespaces, on a cluster with documented list-flood starvation history. **Roughly doubles Part B.** Costed, default off — see D2.

### Formats: four, all stdlib, zero new dependencies

**HTML** (`html/template`) — the fancy one. Self-contained, embedded CSS, no CDN, no JS, `@media print` rules for clean browser print-to-PDF. Auto-escaping matters: the report embeds cluster-controlled resource names.
**Markdown** (`text/template`) · **CSV** (`encoding/csv`) — what auditors actually ask for · **JSON** (`encoding/json`)

All four render **eagerly** and all four are persisted. Documents are tens of KB. This makes "choose between them" literally true at download time **and deletes the format picker, the options modal and the progress UI from the frontend.**

**PDF is held back** — it's the only format needing a dependency, and it's a second full renderer maintained forever alongside the HTML template. See D1.

### The chart — where the danger is

`_helpers.tpl:82-86` `telark.exporterSnapshotsPvcName` is the single source for the PVC, the volume `claimName`, and the `SNAPSHOTS_PVC_NAME` env.

**Do NOT parameterise it.** A shared `telark.exporterPvcName "snapshots"` helper is exactly how the name drifts. Add a plain sibling instead.

- `pvc.yaml` — wrap the existing body in a `range` over a 2-entry inline list. Body stays **inline** (`vpa.yaml` idiom, ~18 lines)
- **Access mode: no new key, and `_mode.tpl` needs zero edits.** `_mode.tpl:32` already derives it from replica count; both claims read the same value
- **Storage class: shared, no new key.** `_storage_guard.tpl` untouched
- **`_deployment.tpl`: zero edits.** It already ranges volumes/volumeMounts and `tpl`s claimName

**`values.schema.json` is a hard gate.** I verified `app.persistence` is `additionalProperties: false` with only `{enabled, storageClass, size}`. Adding `reportsSize` to values without the schema **breaks every install.** Same commit, not a follow-up.

Verify with `rtk proxy "helm template ..."` — RTK truncates multi-doc output and has shown 1 of 15 documents before.

### The download is the first blob in dashboard-ui

There is **no existing download pattern** — the snapshot manifest is fetched as JSON and rendered in a panel. So there's no precedent to copy, good or bad.

**Forbidden:** any `<a href>` or `window.open` carrying a token. Session tokens leaked into nginx access logs once through `/sessions/tokens/<token>/`. A download link is exactly that shape.

**Required:** header auth (`X-Session-Token`), `responseType: 'blob'`, `createObjectURL` → programmatic `<a download>` → `revokeObjectURL` in a `finally`. `request.ts` returns `response.data` only, so this needs a small dedicated helper rather than disturbing every existing caller.

---

## Decisions I need from you

| | Question | My call | Blocking |
|---|---|---|---|
| **D1** | Native PDF, or print-optimised HTML? | HTML. PDF is a second renderer forever | No |
| **D2** | Full-life history, or final window + coverage callout? | **Final window.** Highest-consequence decision here | No |
| **D3** | Reports PVC size? | 2Gi / 1Gi minimal / 10Gi performance, unconditional | No |
| **D4** | Keep existing log message strings when moving `WriteFileAtomic`? | Keep byte-identical — zero log delta | No |
| **D5** | Capture hooks on Terminate + Cancel only, or all six `CleanupByPlanID` sites? | Terminate + Cancel. Worth a deliberate look at all six during step 13 | No |
| **D6** | Who generates, who downloads? | Generate = write + new action; list/download = denyable read. Needs two new role constants in `internal/data` | No |

**None are blocking** — every one has a default that produces useful, safe work. **D2 is the one to actually think about:** if your mental model is "this report tells me everything the plan blocked over three weeks", the default version produces a document that looks complete and isn't. The coverage callout is the mitigation, which is why it's mandatory rather than nice-to-have.

---

## What I will not do

Version bumps · Chart.yaml · image builds · module releases · pin bumps · cluster apply · any git operation.

I also won't finish or revert your 40 staged lint files — the plan works around them and leaves them exactly as found.

---

**Confirm to proceed, or tell me what to change.**

## Serious findings to fold into the steps BEFORE executing

- **Part A cannot both compile and pass its own gate. Step 4 deletes IsWithinBase from package snapshot, but utils/snapshot/request.go calls it unqualified — and request.go is on step 5's mandatory zero-diff perimeter. Step 4's verify grep cannot detect this, because an intra-package call has no `snapshot.` qualifier to grep for.**
  - _severity_: critical
  - _evidence_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/request.go:156-158 — `func LoadSnapshotData(path string) (map[string]any, error) { cleaned := filepath.Clean(path); if !IsWithinBase(cleaned, envmanager.GetSnapshotsPath()) { return nil, os.ErrNotExist }`. The SEAM section says the five functions MOVE and "the snapshot originals are deleted"; step 4 says "Remove `IsWithinBase` and the traversal guard from `paths.go`". Step 4's `files:` list omits request.go entirely. Step 5 check 1 requires `git diff -- internal/utils/snapshot/request.go` to be EMPTY. Step 4's verify is `grep -rn 'snapshot.IsWithinBase|snapshot.FormatBytes|snapshot.FormattedFileSize' services/` — which returns nothing for request.go:158 whether or not the code compiles. This is the snapshot READ path: LoadSnapshotData is called at internal/exporters/snapshot/def.go:376 and :413, and IsWithinBase there is the path-containment guard on snapshot reads, i.e. the rollback fetch. Same omission class: storage.go:214 and :217 also call FormatBytes unqualified and the plan's table says FormatBytes moves with "Change: none", never enumerating those two call sites.
  - _fix_: Either (a) add request.go to step 4's edit list and REMOVE it from step 5's perimeter, replacing the zero-diff requirement for that one file with an exact enumerated diff (`+artifact` import, one qualifier on line 158) the way def.go is handled; or (b) keep a thin `IsWithinBase` wrapper in package snapshot the way the plan already keeps WriteSnapshotJSON and ValidateSnapshotIdentity wrappers — that preserves the true zero diff on request.go and costs three lines. (b) is the smaller change and is consistent with the plan's own wrapper pattern. Do NOT let an implementer resolve this by re-declaring a second copy of IsWithinBase in the snapshot package — that is duplicated containment logic on the rollback read path and violates C3. Also add storage.go:214/217 to step 4's enumerated edits.
- **Baseline (a) is built on a go.work that does not exist at the stated path, and at HEAD contains none of the internal/* replaces. The worktree baseline therefore resolves the four shared modules from the module proxy instead of the local working copies, making it non-comparable to baseline (b) — which destroys the three-way classification rule that is the whole point of step 1.**
  - _severity_: critical
  - _evidence_: `cat /Users/houssem/Desktop/Github/go.work` → "No such file or directory". The workspace file is /Users/houssem/Desktop/Github/telark/go.work, a TRACKED file inside the telark repo. `git diff --cached -- go.work` shows the replaces are ADDED in the staged, uncommitted diff: HEAD has `go 1.26.5` and a bare `use (...)` block with zero replace directives; the four `replace github.com/telark/{data,rest,kcore,x-ware} => /Users/houssem/Desktop/Github/internal/*` lines and the `go 1.27.1` bump are all in the staged diff (+10/-1). So `git worktree add .../telark-baseline HEAD` yields a tree whose go.work has NO internal/* replaces and declares go 1.26.5. The plan's stated rule — "The path MUST be a sibling inside Desktop/Github/ so the parent go.work still resolves the internal/* replaces" — is false twice over: there is no parent go.work, and the replaces are ABSOLUTE paths, so worktree location is irrelevant. Compounding it, `.golangci.yml` is also staged-modified (+6/-2), so baseline (a) runs HEAD's linter config and baseline (b) runs the modified one; step 5 check 4 ("LINT PARITY ... count <= baseline 1b") and the 1a-vs-1b classification are contaminated by config drift, not by the staged lint work.
  - _fix_: Drop baseline (a) or fix its construction. If kept: after `git worktree add`, copy the working-tree go.work and .golangci.yml into the baseline worktree (they are the user's in-flight workspace/lint setup, not part of the code under test) so (a) and (b) differ ONLY by the staged Go source edits. Delete the "MUST be a sibling" justification — it is wrong and will mislead the implementer into debugging the wrong thing. Honestly, baseline (b) alone plus `git stash list`-style discipline is sufficient and cheaper; (a) only earns its keep if it is genuinely apples-to-apples.
- **Step 13 — the plan's self-declared HIGHEST-RISK step — has a circular dependency and names no reachable persistence mechanism. Discovery cannot write to the exporter's reports PVC, and the client that would let it is created in step 14, which depends on step 13.**
  - _severity_: high
  - _evidence_: Step 13 `files:` lists only services/discovery/.../svc.go and violations/violations.go, yet its body says the hook "persists the result as a `violations.json` sidecar in the plan's report directory through the step-6 store". The step-6 store is /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/report/store.go — exporter-side filesystem code on the exporter's PVC. Discovery has no mount and no import path to it (the plan's own architecture note: "exporter owns CRDs and storage; discovery orchestrates"). The only route is HTTP `reports/create` via a discovery wrapper, which is step 14's clients/reports.go. Step 14 `depends on: 12, 13`; step 13 `depends on: 11`. Circular. The project rule the plan itself cites ("discovery handlers consume wrappers in clients/<domain>.go, never import rest-pkg clients directly") forecloses the shortcut. Net effect: the edit sitting one statement from CleanupByPlanID on the plan teardown path — verified at svc.go:356 (Terminate, func at :355, Cleanup is the FIRST statement) and svc.go:187 (Cancel, func at :178) — would be implemented with an undefined write mechanism.
  - _fix_: Split step 14: move `clients/reports.go` (the discovery→exporter wrapper) into its own step that depends on 9, and make step 13 depend on it. Then step 13's hook is honestly described as "resolve namespaces, Collect, POST the sidecar through the reports client wrapper, log-and-swallow any error". Also add clients/reports.go to step 13's `files:` list so the implementer knows the hook is not self-contained.
- **No step creates internal/rest/clients/reports. Step 8 creates only the endpoints package; step 14 then says it wraps "the rest-pkg reports client" — a package nothing in the plan builds. This is a whole shared-module layer with no step, and the user-owned follow-up list inherits the same blind spot.**
  - _severity_: high
  - _evidence_: `ls /Users/houssem/Desktop/Github/internal/rest` shows `clients` and `endpoints` as separate top-level dirs. `ls /Users/houssem/Desktop/Github/internal/rest/clients` → auth, insights, notifications, plans, resources, shared, snapshots. The snapshots precedent uses BOTH: /Users/houssem/Desktop/Github/telark/services/discovery/internal/clients/snapshots.go imports `snapshotsclient "github.com/telark/rest/clients/snapshots"` and `"github.com/telark/rest/clients/shared"`. Step 8 creates only endpoints/reports/{def.go,types.go}. Step 14 says clients/reports.go is "a wrapper over the rest-pkg reports client". The USER-OWNED FOLLOW-UPS entry likewise says only "Release internal/rest — the new endpoints/reports package", confirming the clients package was never considered.
  - _fix_: Add a step (or extend step 8) creating /Users/houssem/Desktop/Github/internal/rest/clients/reports mirroring clients/snapshots, and update the release follow-up to name both packages. Without it, step 14 has nothing to wrap and the implementer will either invent an ad-hoc HTTP call in discovery (violating the wrapper rule) or import rest-pkg endpoints directly.
- **Report sections 6 (policy set as rendered) and 7 (health) declare the step-13 sidecar as their post-termination source, but the sidecar the plan specifies holds only the output of violations.Collect, which carries neither rendered policies nor health. Both sections render empty for terminated plans — the precise "looks complete and silently isn't" failure that D2 and the mandatory coverage callout exist to prevent, and the callout only covers section 5.**
  - _severity_: high
  - _evidence_: Report contents #6: "for a terminated plan this section MUST come from the step-13 sidecar". #7: "for terminated plans the last MEANINGFUL health value comes from the sidecar". Step 13 specifies the sidecar as "a `violations.json` sidecar" holding "the result" of `violations.Collect`. Verified signature at services/discovery/internal/core/plans/protection/violations/violations.go:148 — `Collect(ctx, dyn, namespaces []string, renderedPolicies []string, resultFilter string) ([]planseps.ProtectionPlanViolation, error)`. The element type, verified at /Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go, is `{Policy, Rule, Namespace, Resource, Result, Message, Timestamp}` — no rendered-policy list, no health. Meanwhile both destructive patches are confirmed to blank exactly those fields: svc.go:487-496 BuildTerminatePatch sets `FieldRenderedPolicies: []string{}` AND `FieldHealth: plans.HealthUnknown`; svc.go:474-485 BuildCancelPatch does the same.
  - _fix_: Widen the sidecar from `violations.json` to a plan-boundary capture record holding {violations, renderedPolicies, health, healthReason, capturedAt} — it is the same single write, and renderedPolicies/health are already in hand at the hook site (Terminate receives *plans.ProtectionPlan; Cancel does s.exporter.Get(planID) at svc.go:179 before the hook point). Then extend the coverage callout to state per-section provenance, not just the enforcement window, so sections 6 and 7 cannot silently render empty while the cover claims completeness.
- **Step 10 instructs the implementer to copy a builtin.go grant pattern that does not exist, and its verify criterion is unachievable. Built-in roles contain zero action references; ActionViewProtectionPlanViolations is granted nowhere. Following the instruction literally means inventing the new grant shape the step forbids, inside a protection-guarded, version-stamped table in a shared module.**
  - _severity_: high
  - _evidence_: Step 10: "Wire into `builtin.go` so the built-in roles grant them consistently with how `ActionViewProtectionPlanViolations` is granted today. Do not invent a new grant shape." Verify: "A user holding a built-in role that can view violations can also list reports." Reality: `grep -c Action /Users/houssem/Desktop/Github/internal/data/resources/role/builtin.go` → 0. Grepping ActionViewProtectionPlanViolations across internal/data, telark/services and dashboard-ui/src returns exactly ONE hit — its own declaration at rules.go:55. builtin.go references only the SCOPE (`ScopeProtectionPlans` at builtin.go:20, inside `builtinScopes`), and roles are built by `uniformPermissions(level)` producing `ScopeAndPermissions{Scope, Level}`. BuiltinRoles entries carry `builtinProtection()` = {PreventDeletion, PreventModification, PreventScopeChanges, LockName, LockCategory} and a `builtinVersion = "v1.0.0"` constant. The actual model is corroborated on the UI side: dashboard-ui permissionEngine.tsx entries are `{scope, level, deny: 'applications.viewapplicationssnapshots.deny'}` — actions are deny-flags layered on a scope+level grant, never granted in builtin.go.
  - _fix_: Delete the builtin.go edit from step 10. Adding the two constants to rules.go is sufficient and matches how ActionViewProtectionPlanViolations actually works. Rewrite the verify criterion to what is testable: the two constants exist, the exporter/discovery authz entries reference them, and a subject with the deny flag set is refused while one without it is allowed. Drop builtin.go from the step's `files:` list — editing a PreventModification-guarded, version-stamped built-in role table for no functional reason is gratuitous risk in a shared module that then needs releasing and pinning.
- **The baseline's true scope is understated by more than 2x, and the risk register's only entanglement warning names two exporter TEST files while missing that both step-13 production target files are themselves staged-modified.**
  - _severity_: high
  - _evidence_: Plan (step 1, and RISKS): "40 staged exporter files (+1036/-684)". That exporter figure is CORRECT — `git diff --cached --stat -- services/exporter` ends "40 files changed, 1036 insertions(+), 684 deletions(-)". But repo-wide `git diff --cached --stat` ends "104 files changed, 3765 insertions(+), 1108 deletions(-)", with 55 staged files in services/discovery alone, plus staged .github/ workflows, charts/telark-crds, charts/telark/templates/discovery/rbac/clusterrole.yaml, services/auth and services/notifier. Critically, `git status --porcelain` on the step-13 targets returns `M  services/discovery/internal/core/plans/protection/svc.go` and `M  services/discovery/internal/core/plans/protection/violations/violations.go` — both staged-modified. The plan's risk register says only "Part A edits two files (logic_test.go, persist_test.go) that are part of the user's staged unfinished work", and step 20's verify checks only that "the user's 40 staged exporter files are still staged". Every line number step 13 quotes was verified correct against the current working tree, so they are right TODAY — but they were read through the staged edits and will shift if the user finishes or reverts that work. (Good news for the Part A gate: `git status --porcelain -- services/exporter/internal/utils/snapshot/` returns nothing, so every perimeter production file is clean and `git diff` is a valid gate command there.)
  - _fix_: Restate the baseline as repo-wide with per-service figures. Add a risk-register entry for step 13: its two target files are staged-modified, so quoted line numbers are valid only against the current working tree and must be re-derived at implementation time by symbol name, not offset. Extend step 20's final check to cover the discovery staged set as well as the exporter's, and re-verify svc.go/violations.go staged diffs are unchanged except for the added hook statements.
- **Step 17 builds a Redux slice + thunks + selectors + store-constant entries + models for the reports list, when the identical sibling panel on the same page deliberately does not use Redux.**
  - _severity_: high
  - _evidence_: /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanViolations.ts is a ~40-line useState/useEffect hook returning {data, loading, error, refresh}. /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx:91 calls usePlanViolations(plan.id) and feeds ViolationsSection at :487 inside the fifth SettingsCard. The whole protection store directory contains exactly one slice (store/slices/protectionPlansSlice.ts, 4.1K) for the plan list itself. The reports panel is the same shape as the violations panel: one list, scoped to one plan, mounted on the details page, refetched on demand. Step 17 even budgets state 'keyed per plan ID so switching plans does not show a stale list' — a problem the local hook does not have, because unmount discards the state.
  - _fix_: CUT step 17 in its entirety: no slice, no thunks, no selectors, no FETCH_PLAN_REPORTS entries in src/constants/store/store.ts. Replace with one file — src/features/plans/protection/hooks/usePlanReports.ts, beside usePlanViolations.ts, returning {data, loading, error, generating, generate, refresh} with the same cancelled-flag effect. Keep only the response type in the feature's models/. Step 18's ReportsSection prop shape (data/loading/error) is unchanged, and step 16's clients/reports.ts is unchanged. Net: five files and a store-constant edit become one file.
- **Step 13 invents a bespoke violations.json sidecar plus a captured/live dual path in gather.go — a parallel capture-and-merge mechanism for a feature you are already building — and as scoped it still leaves report sections 6 and 7 empty for terminated plans.**
  - _severity_: high
  - _evidence_: I read the patch builders in services/discovery/internal/core/plans/protection/svc.go: BuildCancelPatch and BuildTerminatePatch BOTH set FieldRenderedPolicies: []string{} AND FieldHealth: plans.HealthUnknown. The plan's own REPORT CONTENTS says section 6 'MUST come from the step-13 sidecar' and section 7's 'last MEANINGFUL health value comes from the sidecar' — but step 13 persists only the output of violations.Collect. So the sidecar as specified closes one of three holes and gather.go pays for it with a CoverageSource branch and a prefer-sidecar read path. Meanwhile the expensive half of the boundary hook is identical under either design: the live Event LIST at violations/violations.go:197-199. Rendering four templates in memory is the cheap part.
  - _fix_: CUT the violations.json sidecar, the CoverageSource captured/live enum and the prefer-sidecar branch in gather.go. Replace with one swallowed statement before CleanupByPlanID in Terminate (svc.go:356) and Cancel (svc.go:187) that calls the step-11/12 generate path you are already building — gather, render four, persist through the step-6 store with the service token. At that instant the CR is still intact, so sections 1-7 are all populated, gather.go keeps exactly ONE path (always live), and the boundary report shows up as an ordinary row in the list with no new file format, no merge logic and no second read path. Keep the coverage callout: the Event TTL ceiling is unchanged either way. Fallback if the user objects to auto-generating for every expiring plan: keep the sidecar, but it must then carry renderedPolicies and health as well as violations — do not ship the violations-only version.

## Chosen approach

Angle 3 — smallest seam. Extract exactly the five functions that a second caller provably needs into a new `services/exporter/internal/utils/artifact` package, repoint the four production call sites, and leave the entire snapshot delete/retention/GC/stats machinery untouched. No `Store` type, no shim layer, no threading of a context parameter through fifteen functions. Reports then build on `artifact` as a peer of snapshots, with their own ~60-line store, their own PVC and their own routes.

## Why chosen

I settled this by census rather than taste, and the census falsified the premise of one of the three angles.

Grepping every reference to the five candidate-generic symbols (`WriteSnapshotJSON`, `IsWithinBase`, `ValidateSnapshotIdentity`, `FormatBytes`, `FormattedFileSize`) outside `internal/utils/snapshot/` returns **four production call sites, all in one file** — `services/exporter/internal/exporters/snapshot/def.go` lines 40, 151, 213 and 539. Everything else is tests. Angle 2 was sized for 29 production call sites and therefore proposed 29 `snaputil` shims to hold `internal/exporters/` at zero diff; with four call sites that shim layer is pure ceremony — you just edit the four lines. Angle 1 wanted to thread a `*Store` through ~15 functions to dissolve the `storageStats` package singleton at `utils/snapshot/types.go:14`, but reports ship no storage-stats panel, so the singleton is never contended by the second caller and dissolving it buys nothing today.

The deciding factor is where the danger is. `retention.go:25` globs `V*.json` and `exporters/snapshot/gc.go` runs `CollectOrphans` against the snapshots volume hourly, gated by an `isSweepable` predicate. Snapshots are the product's rollback path. Angle 1's most elegant idea — make reports versioned `V<n>.<ext>` artifacts so retention and generation-resolution come free — requires widening the version codec that `retention.go`, `isSweepable` and the stats counter all read independently. That means editing the delete predicate of an hourly volume sweeper to get free retention for a feature whose retention is fifteen lines of ReadDir-sort-Remove. That is a bad trade, and it is the trade Angle 3 declines to make.

On constraint C3 ("make the existing snapshot code generic, don't duplicate it"): Angle 3 honours it for every piece reports actually reuse — atomic write, path containment, identity validation, byte formatting, file sizing all become genuinely shared, single-implementation code. It declines to generalise GC and storage-stats, which reports never exercise. Generalising code the second caller never calls isn't reuse; it's relocating untested code into a package whose name now promises something it hasn't been proven to do. Nothing gets duplicated either way — `report/store.go` calls `artifact.WriteFileAtomic`, it does not re-implement it.

Net effect: Part A is a low-risk, fully verifiable refactor that touches five functions and five lines of one consumer, and leaves the recovery path byte-identical. The risk budget is then spent where it genuinely has to be — the second PVC and the plan-teardown capture hooks.

## Grafted from the other designs

- From Angle 2 — characterisation tests BEFORE the refactor. Write a golden-file test that pins the exporter's snapshot response JSON, specifically the raw `"path"` key built at `internal/exporters/snapshot/def.go:136`, because `services/discovery/internal/clients/snapshots.go:122` reads `dataMap[constants.SnapshotPathKey]` and a miss degrades SILENTLY via `circuitbreaker.NotCounted`. This is the single best protection in any of the three angles and it costs one file.
- From Angle 2 — the zero-diff invariant as a machine-checkable VERIFY criterion, not a good intention. Angle 3 can't claim zero diff on `def.go` (it edits 5 lines there), so the invariant is narrowed and made stronger: `git diff` must be literally EMPTY across the named danger perimeter (gc.go, retention.go, resolve.go, storage.go, manifest.go, request.go, sanitize.go, paths.go, exporters/snapshot/gc.go, managers/envs/snapshots.go), and the `def.go` diff must be exactly the five enumerated lines. A reviewer can check that in one command.
- From Angle 2 — treating durable violation capture as a first-class prerequisite with its own steps rather than a detail of the rendering step. Adopted; it is step 13 and it is flagged as the second-most-dangerous step in the plan.
- From Angle 2 — a clean-worktree baseline. Adapted to the verified reality: the tree is NOT clean (40 staged exporter files, +1036/-684) and a dependency-upgrade workflow is concurrently mutating go.mod/go.sum. So the technique becomes a `git worktree` at HEAD placed as a sibling inside `Desktop/Github/` so the parent `go.work` still resolves, plus a second baseline taken in the working tree as-is. Two baselines, one comparison target, zero mutation of the user's unfinished work.
- From Angle 1 — the caller-census discipline itself. Angle 1 argued the compiler is the migration tool because the old symbols cease to exist. That argument is kept and is why the five functions MOVE rather than get copied: after Part A, `snapshot.IsWithinBase` does not exist, so nothing can silently keep calling the old one.
- From Angle 1 — the warning that a reports GC must never share the snapshot GC lock key `exporter:snapshot:gc` (`internal/constants/config.go`), whose `gcTickAllowed` FAILS OPEN. Angle 3 sidesteps the hazard entirely by shipping no reports GC goroutine at all, but the warning is recorded as the reason, so nobody adds one carelessly later.
- From Angle 1 — the naming discipline for the snapshots-vs-reports axis. `ScopeDefinition{Name, Namespaced}` in `internal/managers/envs/snapshots.go` already means the second path segment. The new axis is a level ABOVE it and is never called `scope`. Angle 3 avoids needing the word at all by keeping the two stores as independent packages, but the prohibition stands for any identifier added in Part B.
- From Angle 3 itself, retained against both runners-up — render ALL formats eagerly at generation time. This deletes the format-picker modal from the UI entirely and makes 'the user can choose between them' literally true at download time with zero server work.

## Seam decision

**New package: `/Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/artifact/artifact.go`.**

Five functions MOVE there (move, not copy — the snapshot originals are deleted so the compiler finds every reference):

| New symbol | Moved from | Change |
|---|---|---|
| `WriteFileAtomic(path, id string, write func(io.Writer) error) error` | `snapshot/persist.go` `WriteSnapshotJSON` | JSON encoder becomes a caller-supplied writer func. The unique-temp-name + rename logic and its explanatory comment (persist.go:12-15) move verbatim. |
| `IsWithinBase(target, base string) bool` | `snapshot/paths.go:13-16` | none |
| `ValidateIdentity(id string) error` | `snapshot/paths.go:56-68` | the `scope == ""` check stays behind in the snapshot wrapper; only the `filepath.Base(id) != id` traversal guard moves |
| `FormatBytes(uint64) string` | `snapshot/storage.go` | none |
| `FormattedFileSize(path, id string) string` | `snapshot/file.go` (whole file, already fully generic) | none |

`utils/snapshot` keeps two thin domain wrappers so its own callers and error semantics are unchanged:
- `WriteSnapshotJSON(path, id, body)` → three lines: `artifact.WriteFileAtomic(path, id, func(w io.Writer) error { return json.NewEncoder(w).Encode(body) })`
- `ValidateSnapshotIdentity(id, scope)` → scope-empty check, then `artifact.ValidateIdentity(id)`

The four production call sites in `internal/exporters/snapshot/def.go` repoint: line 40 keeps calling `snaputil.WriteSnapshotJSON` (unchanged), lines 151, 213 and 539 switch to `artifact.FormattedFileSize` / `artifact.IsWithinBase`, plus one import line. Five changed lines in that file, all enumerable.

**Explicitly NOT touched in Part A** — this perimeter is the verify criterion: `snapshot/gc.go`, `snapshot/retention.go`, `snapshot/resolve.go`, `snapshot/storage.go` (beyond lifting `FormatBytes` out), `snapshot/manifest.go`, `snapshot/request.go`, `snapshot/sanitize.go`, `snapshot/paths.go` `BuildSnapshotDir`/`BuildSnapshotPath`/`APISnapshotPath`, `snapshot/types.go:14` `storageStats` singleton, `exporters/snapshot/gc.go`, `managers/envs/snapshots.go`.

Reports get `/Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/report/store.go` (~60 lines) calling into `artifact`: `BuildReportDir(root, planID)` → `<root>/plans/<planID>`, filenames `<reportID>.<ext>` with `reportID = "R" + unix-millis` (lexically time-sortable, no generation codec, no collision), plus `Write`, `List`, `Read` and a fifteen-line `Prune(planID, max)` that sorts by name and removes the oldest. No GC goroutine, no lock key, no stats walk.

## Decisions for the user

- **Ship a native PDF renderer in v1, or ship print-optimised HTML and document browser print-to-PDF?**
  - _recommendation_: Ship HTML with `@media print` rules, page-break control and an embedded stylesheet; document Cmd-P / Ctrl-P → Save as PDF. Do not add a PDF library in v1.
  - _defaultIfNoAnswer_: Print-optimised HTML, no PDF library. Zero new Go dependencies in the whole change.
  - _impactIfWrong_: If PDF is genuinely required for compliance sign-off, adding it later is a clean additive change — a new renderer file plus one format entry — but it is a SECOND full renderer maintained in parallel with the HTML template forever, because `github.com/go-pdf/fpdf` shares nothing with `html/template`. Roughly 400-600 lines of imperative layout code, and every future report section must then be written twice. Getting this wrong in the 'add it now' direction doubles the maintenance cost of every future section; getting it wrong in the 'skip it' direction costs one extra step later.
  - _blocking_: no
- **Is a report expected to cover the plan's ENTIRE life, or is the final window before termination plus an explicit coverage statement acceptable?**
  - _recommendation_: Accept the bounded window for v1. Capture violations durably at the plan boundary (step 13), and print an unmissable coverage callout on the cover page stating the exact window the enforcement section covers. Do NOT build continuous capture yet.
  - _defaultIfNoAnswer_: Bounded window plus capture-at-boundary plus printed coverage callout.
  - _impactIfWrong_: This is the highest-consequence decision in the plan. If your mental model is 'the report tells me everything this plan blocked over three weeks', the bounded version produces a document that LOOKS complete and silently isn't — worse than an empty report. The coverage callout is the mitigation and it is mandatory for that reason. Choosing continuous capture instead adds a whole subsystem (step 13b) — a poller multiplying LIST load by active-plans × namespaces on a cluster with documented list-flood starvation history, plus an unbounded-growth store and event dedup. Roughly doubles Part B.
  - _blocking_: no
- **How large should the reports PVC be, and is it acceptable that it is provisioned unconditionally whenever `app.persistence.enabled` is true?**
  - _recommendation_: `app.persistence.reportsSize`, defaulting 2Gi standard / 1Gi minimal / 10Gi performance. Provision unconditionally alongside the snapshots PVC — no `reports.enabled` gate, which would reproduce the existing ungated-volume mismatch in the deployment template.
  - _defaultIfNoAnswer_: 2Gi / 1Gi / 10Gi, unconditional.
  - _impactIfWrong_: Too small and report writes fail once the volume fills, with no GC goroutine to reclaim — only the per-plan `Prune`. Too large and every install over-provisions a second volume. On EFS (`efs-sc`) sizing is nominal so the risk is near zero; on EBS-backed classes it is real money. Adding a `reports.enabled` gate later is a breaking values change.
  - _blocking_: no
- **Should the moved atomic-write function keep the existing `ErrSnapshot*Context` log message constants, or get generically-worded siblings?**
  - _recommendation_: Move the four existing constants into a shared group and keep their format strings byte-identical. Snapshot log output is then provably unchanged; reports emit log lines that say 'snapshot' where they mean 'artifact', which is mildly odd but costs nothing operationally.
  - _defaultIfNoAnswer_: Keep the existing format strings exactly. Zero log delta for snapshots.
  - _impactIfWrong_: Rewording them is a behaviour delta in log text only — no control flow, no response body — but it would break any log-based alerting or dashboard filter keyed on those strings, and it would weaken the Part A claim that nothing observable changed.
  - _blocking_: no
- **Should the plan-boundary capture hooks go in `Terminate` and `Cancel` only, or also in `Clear` (svc.go:372) and the other two `CleanupByPlanID` call sites at svc.go:253, :333, :440, :451?**
  - _recommendation_: `Terminate` (before svc.go:356) and `Cancel` (before svc.go:187) only. Those are the two user-visible end-of-life transitions. The others are internal reconcile paths where a capture would produce duplicate or partial records.
  - _defaultIfNoAnswer_: Terminate and Cancel only.
  - _impactIfWrong_: Missing a path means a plan can end without its enforcement record being captured, and by the time anyone notices the Events are already gone — unrecoverable. Adding hooks to too many paths means duplicate captures and noise. Because the failure is silent and permanent, this one warrants a deliberate look at all six `CleanupByPlanID` call sites during step 13 rather than trusting my read.
  - _blocking_: no
- **Who may generate a report, and who may download one, given reports embed namespaces, resource names and user identities?**
  - _recommendation_: Generate: `authz.Write(roledata.ScopeProtectionPlans)` gated by a new `ActionGenerateProtectionPlanReport`. List and download: `authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), ActionViewProtectionPlanReports)` — mirroring how `GetSnapshotManifest` is denyable at `internal/authz/requirements.go:124-125`. Exporter's `reports/create` is service-token-only. Delete mirrors `DeleteSnapshot` with `authz.Own`.
  - _defaultIfNoAnswer_: As recommended — two new action constants, both denyable on read.
  - _impactIfWrong_: Too permissive and a read-only viewer exfiltrates a full cluster inventory with user identities in one download. Too restrictive and the feature is invisible to the people who need it. Note this requires editing `/Users/houssem/Desktop/Github/internal/data/resources/role/{rules.go,builtin.go}` — a shared module, so it lands in your release/pin follow-up list.
  - _blocking_: no

## Steps

- **1. Capture two baselines without touching the unfinished work**
  - _part_: A-refactor
  - _layer_: baseline
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter
    - /Users/houssem/Desktop/Github/go.work
  - _detail_: VERIFIED PROBLEM: `git status` on the exporter shows 40 staged files (+1036/-684) of unfinished lint work, including a heavily modified `internal/tests/snapshot/logic_test.go`, AND `MM services/exporter/go.mod` — staged and unstaged simultaneously, because a dependency-upgrade workflow is concurrently mutating go.mod/go.sum. Neither finishing nor reverting that work is permitted, and a single baseline would be untrustworthy.

Take TWO baselines, mutating nothing:

(a) HEAD baseline. `git worktree add /Users/houssem/Desktop/Github/telark-baseline HEAD`. The path MUST be a sibling inside `Desktop/Github/` so the parent `go.work` still resolves the `internal/*` replaces — a worktree anywhere else silently loses them and every build fails for the wrong reason. There run `go build ./services/exporter/...`, `go test ./services/exporter/... -count=1`, `golangci-lint run` and record the issue COUNT (not zero — discovery's recorded baseline is 462 issues, auth's is 99; a count is the contract).

(b) Working-tree baseline. In the real tree, as-is, staged edits present, run the same three commands and record the same three results.

(b) is the comparison target for every Part A step. (a) exists solely to classify any failure you see: present in (a) too means pre-existing, yours to leave alone; present only in (b) means it came from the staged lint work, also not yours; present in neither means you broke it.

Do NOT run `go mod tidy` at any point — the dep workflow owns go.mod/go.sum. Before trusting any later build failure, re-run `git diff --stat -- services/exporter/go.mod services/exporter/go.sum` to check whether that workflow moved underneath you.
  - _verify_: Both baselines recorded as concrete artifacts: build exit code, the exact set of passing and failing test names, and the golangci-lint issue count. `git status --porcelain` in the real tree is byte-identical before and after this step.
  - _behaviourPreserving_: yes
  - _dependsOn_:

- **2. Pin the cross-service response contract with a golden-file test**
  - _part_: A-refactor
  - _layer_: baseline
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/golden_contract_test.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/testdata/create_response.golden.json
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/testdata/read_response.golden.json
  - _detail_: NEW test file, written BEFORE any refactor. Grafted from Angle 2 and it is the highest-value single file in Part A.

`internal/exporters/snapshot/def.go:136` builds its response with a RAW string literal `"path"`, and `services/discovery/internal/clients/snapshots.go:122` reads it back as `dataMap[constants.SnapshotPathKey]`. A mismatch does not fail loudly — it degrades SILENTLY through `circuitbreaker.NotCounted`. No compiler and no existing test protects this.

Marshal the maps returned by `createSnapshotResponse` and by `ReadSnapshot` and assert the full JSON key set byte-for-byte against checked-in golden files: `id`, `scope`, `namespace`, `generation`, `path` for create; those plus `fileSize`, `pvcAvailable`, `pvcTotal`, `pvcUsedPercent`, `manifest` for read.

Per the project rule, this goes under `internal/tests/`, never beside production code. If `createSnapshotResponse` is unexported, expose a thin test helper rather than writing an `_internal_test.go`.
  - _verify_: New test passes against the CURRENT unrefactored code on first run. Deliberately rename the `"path"` literal in a scratch edit and confirm the test fails, then revert — a golden test that cannot fail is decoration.
  - _behaviourPreserving_: yes
  - _dependsOn_:
    - 1
- **3. Create the artifact package with the five provably-shared functions**
  - _part_: A-refactor
  - _layer_: shared-storage
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/artifact/artifact.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/config.go
  - _detail_: NEW package holding exactly the five functions the census proved a second caller needs:

`WriteFileAtomic(path, id string, write func(io.Writer) error) error` — the body of `snapshot/persist.go` with `json.NewEncoder(f).Encode(body)` replaced by `write(f)`. The unique-temp-name rationale comment at persist.go:12-15 moves verbatim; it documents a real concurrency fix (a fixed temp name lets concurrent writers truncate each other and rename a half-written file into place) and must not be lost.

`IsWithinBase` — verbatim from paths.go:13-16, including the `baseSeparatorShift` constant.
`ValidateIdentity(id string) error` — the traversal guard from paths.go:63-66 only.
`FormatBytes(uint64) string` — verbatim from storage.go.
`FormattedFileSize(path, id string) string` — the whole of `snapshot/file.go`, already fully generic.

Per decision D4, move the four `ErrSnapshotTempCreate/TempWrite/TempClose/Rename Context` constants to a shared group with format strings byte-identical, so snapshot log output does not change. `SnapshotTempSuffix` (".*.tmp") is the one `WriteFileAtomic` uses — note it is a DIFFERENT constant from `SnapshotTempFileSuffix` (".tmp"); both exist and confusing them silently breaks orphan-temp greppability.

Add a `ponytail:` comment recording the deliberate ceiling: this package holds only functions with two proven callers; GC, retention and storage-stats stay snapshot-only because reports never exercise them.
  - _verify_: `go build ./services/exporter/...` succeeds. `go vet` clean. The package compiles with no import of `utils/snapshot`, `managers/envs` or anything snapshot-specific — if it needs one of those, the wrong function moved.
  - _behaviourPreserving_: yes
  - _dependsOn_:
    - 2
- **4. Delete the originals and repoint all references**
  - _part_: A-refactor
  - _layer_: shared-storage
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/persist.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/paths.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/file.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/storage.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/exporters/snapshot/def.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/logic_test.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/persist_test.go
  - _detail_: MOVE, not copy — grafted from Angle 1: delete the originals so the compiler locates every reference and nothing can keep calling a stale copy.

Delete `snapshot/file.go` entirely. Remove `IsWithinBase` and the traversal guard from `paths.go`, `FormatBytes` from `storage.go`, and reduce `persist.go` to a three-line wrapper.

`utils/snapshot` keeps two domain wrappers so its own semantics are unchanged: `WriteSnapshotJSON(path, id, body)` delegating with a JSON-encoder closure, and `ValidateSnapshotIdentity(id, scope)` doing the scope-empty check then calling `artifact.ValidateIdentity`.

In `internal/exporters/snapshot/def.go` exactly five lines change: line 40 is unchanged in substance (still `snaputil.WriteSnapshotJSON`), line 151 `FormattedFileSize` → `artifact.`, lines 213 and 539 `IsWithinBase` → `artifact.`, plus one import line.

CAUTION on the test files: `internal/tests/snapshot/logic_test.go` and `persist_test.go` are part of the user's STAGED unfinished lint work and reference these symbols at logic_test.go:62-92, 159-216 and persist_test.go:34, 64. Repoint only the symbol qualifiers. Do not reformat, do not restructure, do not finish the lint edits, do not revert them.
  - _verify_: `go build ./services/exporter/...` succeeds. `grep -rn 'snapshot.IsWithinBase\|snapshot.FormatBytes\|snapshot.FormattedFileSize' services/` returns nothing. `git diff --stat -- services/exporter/internal/exporters/snapshot/def.go` shows exactly 5 changed lines.
  - _behaviourPreserving_: yes
  - _dependsOn_:
    - 3
- **5. Part A gate — prove behaviour is unchanged, not merely that it compiles**
  - _part_: A-refactor
  - _layer_: shared-storage
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter
  - _detail_: Four independent checks, all of which must hold before Part B starts. This is the step that makes Part A trustworthy.

1. DANGER-PERIMETER ZERO DIFF (grafted from Angle 2, narrowed). `git diff -- internal/utils/snapshot/gc.go internal/utils/snapshot/retention.go internal/utils/snapshot/resolve.go internal/utils/snapshot/manifest.go internal/utils/snapshot/request.go internal/utils/snapshot/sanitize.go internal/utils/snapshot/types.go internal/exporters/snapshot/gc.go internal/managers/envs/snapshots.go` must be EMPTY. These files hold the hourly volume sweeper's delete predicate, the `V*.json` retention glob, the generation resolver and the scope registry. If any of them moved, the refactor overreached.

2. GOLDEN CONTRACT. The step-2 tests pass unchanged, proving the `"path"` key and every other response field that discovery depends on still serialise identically.

3. TEST PARITY. `go test ./services/exporter/... -count=1` produces the SAME set of passing and failing test names as working-tree baseline (1b). Same set, not 'still compiles' and not 'all green' — some tests are expected to fail from the unfinished lint work, and they must fail identically.

4. LINT PARITY. `golangci-lint run` issue count is less than or equal to baseline (1b). No `--no-config`, no override flags, no `//nolint` additions.

If the dep-upgrade workflow moved go.mod/go.sum mid-step, re-take baseline 1b before concluding anything.
  - _verify_: All four checks pass. Specifically: perimeter diff empty, golden tests green, test-name set identical to 1b, lint count <= 1b. Any one failing stops Part B.
  - _behaviourPreserving_: yes
  - _dependsOn_:
    - 4
- **6. Report store on the exporter**
  - _part_: B-feature
  - _layer_: shared-storage
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/report/store.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/report/types.go
  - _detail_: ~60 lines, built on `artifact`, deliberately not on the snapshot machinery.

Layout `<reportsRoot>/plans/<planID>/<reportID>.<ext>`. `reportID` is `"R" + strconv.FormatInt(time.Now().UnixMilli(), 10)` — lexically time-sortable, collision-free in practice, and crucially NOT the `V<n>.json` generation codec, so nothing here is coupled to `parseGenerationFilename`, `retention.go`'s glob or `isSweepable`.

Functions: `Write(root, planID, reportID, ext string, body []byte) error` (calls `artifact.ValidateIdentity` on both planID and reportID, then `artifact.WriteFileAtomic`), `List(root, planID) ([]Report, error)`, `Read(root, planID, reportID, ext string) ([]byte, error)` (guarded by `artifact.IsWithinBase`), and `Prune(root, planID string, max int) error` — a ReadDir, a name sort, and `os.Remove` on the oldest. Fifteen lines, and it is why Angle 1's version-codec graft was rejected.

No GC goroutine. No storage-stats walk. No distributed lock. Explicitly do NOT reuse the `exporter:snapshot:gc` lock key (`internal/constants/config.go`) — its `gcTickAllowed` FAILS OPEN and sharing it would let two different sweepers run concurrently. Record that as the `ponytail:` ceiling comment with the upgrade path.

One type file per the project convention, all report types in `types.go`.
  - _verify_: Unit test under `internal/tests/report/` covering: traversal rejection for `../` in planID and reportID, atomic write visible only after rename, `Prune` keeping exactly `max` newest and removing the rest, `Read` outside base rejected. `golangci-lint run` clean.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 5
- **7. Reports env config on the exporter**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/config.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/managers/envs/reports.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/main.go
  - _detail_: Mirror the existing `SNAPSHOTS_*` block at `internal/constants/config.go` (SnapshotsPathEnv, SnapshotsPVCNameEnv, SnapshotsPVCNamespaceEnv, DefaultSnapshotsPath, DefaultSnapshotsMaxVersions) with a `REPORTS_*` group: `ReportsPathEnv = "REPORTS_PATH"`, `DefaultReportsPath = "/reports"`, `ReportsMaxPerPlanEnv`, `DefaultReportsMaxPerPlan = 10`, `ReportsPVCNameEnv`, `ReportsPVCNamespaceEnv`.

NEW `internal/managers/envs/reports.go` mirroring the seven-package-var shape of `snapshots.go` but WITHOUT touching that file — no `ScopeDefinition`, no namespaced/cluster-scoped registry, because reports are always keyed by plan ID only. This is the deliberate reason the `scope` naming collision never arises.

In `main.go`, add `initReportsConfig()` next to the existing `initSnapshotsConfig()` at line 65. Do NOT start any goroutine — no GC ticker, no stats refresher.
  - _verify_: `go build ./services/exporter/...`. Exporter starts with no `REPORTS_PATH` set and falls back to `/reports` without panicking. `golangci-lint run` clean.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 6
- **8. Report endpoint constants in the shared rest module**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/reports/def.go
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/reports/types.go
  - _detail_: NEW package mirroring `/Users/houssem/Desktop/Github/internal/rest/endpoints/snapshots/def.go` exactly — verb-suffixed constants, `base.Endpoint` typed (constraint C1: completely new routes in the same architectural style):

  CreateReport   base.Endpoint = "reports/create"
  GetReportInfos base.Endpoint = "reports/infos"
  GetReport      base.Endpoint = "reports/{id}/get"
  DeleteReport   base.Endpoint = "reports/{id}/delete"

The `{id}` is the reportID; planID and format travel as query parameters, exactly as `GetSnapshot` takes scope, namespace and generation as query params today. CRITICAL: no token ever appears in a path or query — auth is `X-Session-Token` / `X-Service-Token` headers only.

`types.go` holds the payload and response structs in one file per the project convention.

This edits a SHARED module. It works immediately via the go.work replace, but releasing `internal/rest` and bumping pins is a user-owned follow-up.
  - _verify_: `go build ./...` from `/Users/houssem/Desktop/Github/internal/rest`. Constants follow the exact naming and `base.Endpoint` typing of the snapshots package.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 6
- **9. Exporter report handlers, routes and authorization**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/exporters/report/def.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/handlers/report/handler.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/routes/base.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/requirements.go
  - _detail_: The route triple, mirroring snapshots at every layer.

`exporters/report/def.go` — Create/Read/List/Remove over the step-6 store. `ReadReport` is the byte-streaming one, modelled on `ReadSnapshotManifestWithAccept`: it sets `Content-Type` from the requested format and `Content-Disposition: attachment` via the existing `ContentDispositionAttachmentTemplate` constant, and writes raw bytes rather than the JSON envelope. Every other route uses the standard `responseutils.LogAndSendResponse` envelope.

`handlers/report/handler.go` — thin closures with zero logic, matching `handlers/snapshot/handler.go`.

`routes/base.go` — add `reportRoutes(optimizer)` beside `snapshotRoutes` (currently lines 480-523) using the identical `router.CreateRoute` + `performance.NewDynamicOptimizedHandlerFunc` shape, and append it in `InitRoutes` near line 80. Needs `constants.ResourceReport`.

`authz/requirements.go` — add `addReports(requirements)` beside the `addSnapshots` call at line 32. `CreateReport` is service-token-only (discovery writes it). `GetReport` and `GetReportInfos` are `authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports)`, mirroring the `GetSnapshotManifest` shape at lines 124-125. `DeleteReport` mirrors `DeleteSnapshot` with `authz.Own`. Deny-by-default: an unlisted route is refused, and the coverage test enforces that every registered route has an entry.
  - _verify_: The authz coverage test passes — it fails loudly if any new route lacks a requirement. A request with no token gets 401; a session token lacking the new action gets 403; the download returns raw bytes with the correct Content-Type and no JSON envelope. `golangci-lint run` clean.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 7
    - 8
- **10. New role actions in the shared data module**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/internal/data/resources/role/rules.go
    - /Users/houssem/Desktop/Github/internal/data/resources/role/builtin.go
  - _detail_: Add two constants to the protection-plan action block at `rules.go:55-62`, which currently holds `ActionViewProtectionPlanViolations`, `ActionCreateProtectionPlan`, `ActionEditProtectionPlan`, `ActionCancelProtectionPlan`, `ActionDuplicateProtectionPlan`, `ActionReactivateProtectionPlan`, `ActionDeleteProtectionPlan`:

  ActionGenerateProtectionPlanReport = "generateprotectionplanreport"
  ActionViewProtectionPlanReports    = "viewprotectionplanreports"

All-lowercase no-separator, matching every existing value. Both hang off the existing `ScopeProtectionPlans = "protection-plans"` (def.go:85) — no new scope.

Wire into `builtin.go` so the built-in roles grant them consistently with how `ActionViewProtectionPlanViolations` is granted today. Do not invent a new grant shape.

SHARED MODULE edit — live locally via go.work, but releasing `internal/data` and bumping pins is user-owned.
  - _verify_: `go build ./...` in `/Users/houssem/Desktop/Github/internal/data`. Built-in role tests pass. A user holding a built-in role that can view violations can also list reports, and one who cannot, cannot.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 8
- **11. Report model and gatherer in discovery**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/types.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/gather.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/constants.go
  - _detail_: Discovery owns generation — it already holds the plan service, the applier and the violations engine. The exporter only stores bytes. This follows the established split (exporter owns CRDs and storage; discovery orchestrates) and the domain-logic-separation rule: core logic lives in `core/plans/protection/report/`, handlers stay thin.

`types.go` — ONE file with the whole `Report` model: Cover, PlanDefinition, Lifecycle, ProtectedScope, Enforcement (with its `CoverageWindow` and `CoverageSource` fields), PolicySet, Health, StorageFootprint.

`gather.go` — assembles the model from: the ProtectionPlan CR (read through the existing exporter client wrapper in `internal/clients/`, never a rest-pkg client directly), `violations.Collect`, and `resolvePlanNamespaces` — which is currently UNEXPORTED at `violations/violations.go:124` and must be exported or given an exported wrapper.

Synchronous, no job model. One CR get, one namespace resolve, one violations collect, then render. The precedent is the UI's existing 10s violations timeout.

Every user-visible string goes in `constants.go`, never inlined. No string in the model or the rendered output may name the policy-engine vendor — use "policy engine".
  - _verify_: Unit test building a Report from a fixture plan asserts every section is populated and that `CoverageSource` is set to either `live` or `captured`, never empty. `golangci-lint run` clean.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 9
- **12. Four stdlib renderers**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/render_html.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/render_markdown.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/render_csv.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/render_json.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/report/templates/report.gohtml
  - _detail_: One `Render(r Report, format string) ([]byte, error)` dispatching to four implementations. ZERO new Go dependencies:

HTML (`html/template` + `embed`) — the fancy one. Self-contained single file: embedded CSS, no external fonts, no CDN, no JavaScript. Cover page, section headings, styled tables, status colour coding, and `@media print` rules with `page-break-inside: avoid` on tables so browser print-to-PDF produces a clean document. `html/template` gives contextual auto-escaping, which matters because the report embeds cluster resource names.

Markdown (`text/template`) — same sections, GitHub-flavoured tables.

CSV (`encoding/csv`) — the tabular sections only (violations, protected resources, policy set), the auditor/spreadsheet format.

JSON (`encoding/json`) — the full model, indented. The machine-readable appendix.

All four render EAGERLY at generation time and all four are persisted. Format choice becomes a download-time pick with zero server work, which deletes the format-picker modal from the UI entirely.

No vendor names anywhere in template text.
  - _verify_: Golden-file test per format against a fixture Report. HTML output opens standalone with no network fetches (assert no `http://`/`https://` in the bytes). CSV parses back with `encoding/csv`. JSON round-trips into the model. A fixture with `<script>` in a resource name is escaped in the HTML output.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 11
- **13. Durable violation capture at plan boundary — HIGHEST-RISK STEP**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/svc.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/violations/violations.go
  - _detail_: THE PROBLEM, verified in code. `violations/violations.go:34-36` documents `RetentionWindow = "1h"` as mirroring the kube-apiserver `--event-ttl` default, and `:197-199` reads Events LIVE via `dyn.Resource(eventGVR).List` with `FieldSelector: reason=PolicyViolation`. Worse than the 1h TTL: at `svc.go:356` `Terminate` calls `CleanupByPlanID` FIRST, deleting the rendered policies, and then at `:361` patches with `BuildTerminatePatch`, which at `:491` blanks `FieldRenderedPolicies` to an empty slice. `buildViolation` filters Events against that rendered set. So after a terminate, violations return EMPTY IMMEDIATELY — not after an hour. Two independent destruction mechanisms. A report generated post-terminate would show zero enforcement activity for a plan that blocked a thousand requests.

THE FIX, deliberately minimal. Two hooks:
- In `Terminate`, BEFORE the `CleanupByPlanID` at :356
- In `Cancel`, BEFORE the `CleanupByPlanID` at :187

Each calls `violations.Collect` for the plan and persists the result as a `violations.json` sidecar in the plan's report directory via the step-6 store. That sidecar IS the durable record. `gather.go` prefers it when present and sets `CoverageSource = captured`; otherwise it reads live and sets `live`.

SAFETY RULES, non-negotiable. The capture must never block or fail the termination. Wrap it so any error is logged and swallowed — a plan MUST still terminate if report capture fails. Add it as a separate statement before the existing call; do not restructure `Terminate` or `Cancel`, do not reorder anything else, do not touch `BuildTerminatePatch` or `BuildCancelPatch`.

Per decision D5, review all six `CleanupByPlanID` call sites (svc.go:187, 253, 333, 356, 372, 440, 451) before finalising which get hooks. Recommendation is Terminate and Cancel only.

Also export `resolvePlanNamespaces` (violations.go:124) or add an exported wrapper, since `gather.go` needs it.
  - _verify_: Integration test: create a plan, produce violations, terminate it, then generate a report and assert the enforcement section is NON-EMPTY and `CoverageSource == captured`. Second test: make the capture return an error and assert the plan STILL terminates cleanly and its status patch is applied. `git diff` on svc.go shows only added statements, no reordering of existing ones.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 11
- **14. Discovery reports client wrapper, handlers, routes and authz**
  - _part_: B-feature
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/clients/reports.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/reports.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/routes/base.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/authz/requirements.go
  - _detail_: `clients/reports.go` — a wrapper over the rest-pkg reports client, per the project rule that discovery handlers consume wrappers in `clients/<domain>.go` and never import rest-pkg clients directly. Mirrors the existing `clients/snapshots.go`. When reading the exporter's create response, reference response keys through constants the way `snapshots.go:122` uses `constants.SnapshotPathKey` — do not introduce raw literals, and note that a key miss there degrades silently via `circuitbreaker.NotCounted`, so assert the key exists rather than defaulting.

`handlers/plans/protection/reports.go` — thin handlers delegating to the step-11/12 core:
  POST `plans/protection/{id}/reports/generate` — gather, render all four formats, persist each via the exporter, return the new report's metadata
  GET  `plans/protection/{id}/reports` — list, proxied from the exporter

Register in `routes/base.go` alongside the existing violations route, and add the matching deny-by-default entries in `authz/requirements.go`: generate needs `authz.Write(ScopeProtectionPlans)` + `ActionGenerateProtectionPlanReport`; list is `authz.Denyable(authz.Read(...), ActionViewProtectionPlanReports)`.

Download is NOT proxied through discovery — the UI fetches bytes straight from the exporter's `reports/{id}/get`, matching how it already reads snapshot data from the exporter while writing through discovery.
  - _verify_: Discovery's authz coverage test passes. End to end against a live plan: generate returns 201 with report metadata, list returns it, and all four format files exist on the reports volume. `golangci-lint run` clean.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 12
    - 13
- **15. Second PVC, values, schema gate and mode overlays**
  - _part_: B-feature
  - _layer_: chart
  - _files_:
    - /Users/houssem/Desktop/Github/telark/charts/telark/templates/workloads/pvc.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/templates/_helpers.tpl
    - /Users/houssem/Desktop/Github/telark/charts/telark/values.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/values.schema.json
    - /Users/houssem/Desktop/Github/telark/charts/telark/modes/minimal.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/modes/performance.yaml
  - _detail_: Full detail in the pvcAndChart field. Summary of the edits:

`pvc.yaml` — wrap the existing 27-line body in a `range` over a two-entry inline list of dicts (name, size). Body stays INLINE, following the `vpa.yaml` idiom rather than the named-template idiom of `deployments.yaml`, because the body is ~18 lines.

`_helpers.tpl` — add a SIBLING `telark.exporterReportsPvcName` = `printf "%s-exporter-reports-pvc" (include "telark.fullname" .)`. Do NOT parameterise the existing `telark.exporterSnapshotsPvcName` (lines 82-86). That helper is the single source for the PVC name, the volume claimName and the `SNAPSHOTS_PVC_NAME` env; parameterising it is precisely how the snapshots claim name drifts by a byte, which orphans the live snapshots volume and breaks rollback.

`values.yaml` — `app.persistence.reportsSize: 2Gi`; exporter `REPORTS_PATH: "/reports"` with a YAML anchor mirroring the existing `&snapshotsPath`, plus the volume and volumeMount entries.

`values.schema.json` — add `reportsSize` to `app.persistence.properties`. VERIFIED HARD GATE: that object is `{"additionalProperties": false, "properties": {enabled, storageClass, size}}`. Omitting this makes EVERY install fail schema validation.

`modes/minimal.yaml` (line 18-19, size 1Gi) and `modes/performance.yaml` (line 21-22, size 50Gi) — add `reportsSize`.

No `_mode.tpl` edit and no `_deployment.tpl` edit are needed; both already do the right thing.
  - _verify_: `rtk proxy "helm template telark charts/telark"` — RTK truncates multi-document output, so raw mode is mandatory. Diff the rendered output against a pre-change render: the ONLY differences must be the new PVC document and the new exporter env/volume/volumeMount entries. The snapshots PVC name must be BYTE-IDENTICAL. Then `helm lint`, plus renders at `app.mode=minimal`, `standard`, `performance`, and at `app.singleNode=true` and `services.exporter.replicas=2` to confirm both claims track the derived accessMode.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 7
- **16. UI endpoints and the first blob-download helper**
  - _part_: B-feature
  - _layer_: api-client
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/constants/rest/endpoints.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/reports.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/fetch.ts
    - /Users/houssem/Desktop/dashboard-ui/src/api/client/download.ts
  - _detail_: Endpoint entries beside the existing snapshot manifest path at `src/constants/rest/endpoints.ts:84`.

`clients/reports.ts` — `generatePlanReport(planId)` and `fetchPlanReports(planId)` against discovery, mirroring `fetchPlanViolations` in `clients/fetch.ts:55` including its 10s timeout convention. `downloadPlanReport(planId, reportId, format)` against the exporter.

THE SECURITY-CRITICAL PIECE. I verified there is NO existing blob or download pattern anywhere in dashboard-ui — the snapshot manifest is fetched as JSON and rendered in a panel. So there is no precedent to copy, good or bad, and the download must be built deliberately:

FORBIDDEN: any `<a href>` or `window.open` carrying a token in the URL. Session tokens leaked into nginx access logs once through `/sessions/tokens/<token>/` paths; a download link is exactly the shape that reintroduces it.

REQUIRED: fetch through the authenticated client with `X-Session-Token` in the HEADER and `responseType: 'blob'`, then `URL.createObjectURL(blob)` → a programmatic `<a download>` click → `URL.revokeObjectURL` in a `finally`.

Note `src/api/client/request.ts` returns `response.data` only, so it cannot surface the `Content-Disposition` filename. Add a small dedicated `download.ts` helper rather than changing `request.ts` and disturbing every existing caller. Derive the filename client-side from plan name plus report ID plus format.

All errors through the project logger, never `console.*`. No `any` types.
  - _verify_: `npm run check-all` with ZERO errors. Manually confirm via the network panel that the download request carries `X-Session-Token` as a header and that no token appears in any request URL or in `document.location`. Confirm the object URL is revoked (no leaked blob URLs after repeated downloads).
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 14
- **17. Redux slice, thunks and selectors for reports**
  - _part_: B-feature
  - _layer_: frontend-store
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/store/slices
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/store/thunks
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/store/selectors
    - /Users/houssem/Desktop/dashboard-ui/src/constants/store/store.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/models
  - _detail_: Follow the existing protection-plan store layout exactly — `store/{slices,thunks,selectors}` already exist as siblings.

Thunks: `generatePlanReport` and `fetchPlanReports`, typed with the project's `AsyncThunkConfig`, never `any`.

Slice state: `reports[]`, `loading`, `generating`, `error`, keyed per plan ID so switching plans does not show a stale list.

Action-type strings and error messages go in `src/constants/store/store.ts` beside the existing `FETCH_SNAPSHOT_MANIFEST` entries at lines 20, 90 and 134 — never inlined.

Report types go in the feature's `models/`, matching the discovery response shape from step 11.

Download deliberately does NOT go through the store — it is a fire-and-forget side effect with no state worth holding, and putting a blob in Redux would be a mistake.
  - _verify_: `npm run check-all` with zero errors. Generate then list updates the panel without a manual refresh. Switching between two plans shows each one's own reports with no cross-contamination.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 16
- **18. Reports section on the plan details page**
  - _part_: B-feature
  - _layer_: frontend-ui
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/details/ReportsSection.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/texts.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx
  - _detail_: New `ReportsSection.tsx` modelled on the sibling `ViolationsSection.tsx`, added as a sixth SettingsCard in `pages/details/Content.tsx` (currently 502 lines, five cards).

Contents: a "Generate report" button, and a table of past reports with generated-at, generated-by, coverage window, and a per-row format menu (HTML / Markdown / CSV / JSON). Because all four formats are rendered eagerly, the format menu is a plain download picker — NO generation modal, NO format-selection step, NO job-progress UI.

Permissions through `permissionEngine.tsx`, following the `viewSnapshotManifest` entry at lines 20-23 with its allow/deny pair. Generate is hidden without the generate action; the list is hidden without the view action.

Styling rules that are easy to get wrong here: every string in `constants/texts.ts`, never inlined. Status colours from `DEFAULT_COLORS`, never hex. No box-shadow on the buttons. Control sizing comes from `controls.ts` — no `size=` props, no inline height, no nested ConfigProvider. No string may name the policy-engine vendor; say "policy engine".

Do not guess at layout. If anything looks visually wrong, ask for a screenshot rather than guessing, and never open a browser preview window.
  - _verify_: `npm run check-all` with ZERO errors and no `eslint-disable`. Verify by permission matrix: a role with neither action sees no section; view-only sees the list with no generate button; both actions see everything. Downloading each of the four formats yields a correctly-named file that opens.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 17
- **19. Documentation, in the same change**
  - _part_: B-feature
  - _layer_: docs
  - _files_:
    - /Users/houssem/Desktop/Github/telark/docs/INSTALL.md
    - /Users/houssem/Desktop/Github/telark/charts/telark/README.md
    - /Users/houssem/Desktop/Github/telark/charts/telark/VALUES.md
    - /Users/houssem/Desktop/Github/telark/services/exporter/README.md
    - /Users/houssem/Desktop/Github/telark/services/discovery/README.md
    - /Users/houssem/Desktop/Github/telark/docs/architecture.md
    - /Users/houssem/Desktop/Github/telark/README.md
  - _detail_: Docs are part of done, not a follow-up.

`docs/INSTALL.md` — the second PVC, `app.persistence.reportsSize`, and the note that on multi-replica exporter both volumes need an RWX-capable storage class such as `efs-sc`.
`charts/telark/README.md` — the new values keys with their gate semantics.
`charts/telark/VALUES.md` — REGENERATE with helm-docs, do not hand-edit.
`services/exporter/README.md` — the `REPORTS_*` env block in the same table style as `SNAPSHOTS_*`, and the four new routes.
`services/discovery/README.md` — the generate and list routes and the report model.
`docs/architecture.md` — reports as a second artifact store over the shared `artifact` layer, and the discovery-renders / exporter-stores split.
`README.md` — one line in the feature list.

Writing rules: literal `telark` throughout, no "substitute your app.name" notes, no shell scripts — inline commands only. Contact address is `contact@telark.io`, never a personal address. Generic phrasing about the policy engine.
  - _verify_: `helm-docs` produces no further diff after the VALUES.md regeneration. Every new values key and every new env var appears in at least one doc. `grep -ri 'kyverno' ` finds nothing in the new user-facing text.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 15
    - 18
- **20. Whole-change verification gate**
  - _part_: B-feature
  - _layer_: docs
  - _files_:
    - /Users/houssem/Desktop/Github/telark
    - /Users/houssem/Desktop/dashboard-ui
  - _detail_: Final pass across everything, comparing against the step-1 baselines.

1. `golangci-lint run` in exporter and discovery — issue count at or below baseline, no `--no-config`, no new `//nolint`.
2. `go test ./... -count=1` in both services — the same set of passing and failing test names as working-tree baseline 1b, plus the new tests green.
3. `npm run check-all` in dashboard-ui — zero errors.
4. Re-run the step-5 perimeter diff. It must STILL be empty; Part B must not have reached back into the snapshot machinery.
5. `rtk proxy "helm template ..."` at all three modes plus `singleNode=true` plus `replicas=2`, and confirm `helm lint` passes with the schema change.
6. Security pass: grep the whole diff for any token in a URL, and confirm the download path is header-authenticated.
7. Remove the `/Users/houssem/Desktop/Github/telark-baseline` worktree with `git worktree remove`.

Then stop. Building images, bumping versions, releasing modules, applying to the cluster and all git operations are yours.
  - _verify_: All seven checks pass. The baseline worktree is removed and `git worktree list` is clean. The user's 40 staged exporter files are still staged and still unfinished — `git diff --cached --stat` on the files that were staged at step 1 is unchanged except where step 4 repointed symbol qualifiers in the two snapshot test files.
  - _behaviourPreserving_: no
  - _dependsOn_:
    - 19

## Report contents

A report is a single self-contained document with nine sections. For each: what it contains, where the data comes from, and whether it survives plan termination — that last column is the one that determines whether the report is honest.

**1. Cover.** Plan name, plan ID, plan mode, owner, generation timestamp, generating user, and the COVERAGE CALLOUT (see section 5). Source: the ProtectionPlan CR via the exporter client wrapper, plus the authenticated caller's identity. Survives termination.

**2. Plan definition.** Target namespaces and selectors, the policy set requested, enforcement mode, and any exclusions. Source: ProtectionPlan CR `spec`. Survives.

**3. Lifecycle timeline.** Created / activated / updated / cancelled / terminated with timestamps and the actor where recorded. Source: CR `status` plus `metadata.creationTimestamp`. Survives. Note the actor is only present where the CR records it — `ProtectionPlanViolation` at `/Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go:122-142` has NO actor field, so per-violation attribution is genuinely unavailable and the report must not imply otherwise.

**4. Protected scope.** The namespaces the plan resolved to and the resources matched, at generation time. Source: `resolvePlanNamespaces` (`violations/violations.go:124`, currently unexported) plus a discovery resource query. LIVE ONLY — this is a point-in-time snapshot and is stamped "as of <timestamp>" rather than presented as a historical fact. After termination it reflects what matches now, which may be nothing.

**5. Enforcement activity — the heart of the document, and the bounded one.** A table of requests the policy engine denied: timestamp, namespace, resource, policy, rule, result, message. Source: `violations.Collect`, which at `violations/violations.go:197-199` reads Kubernetes Events live with `FieldSelector: reason=PolicyViolation`, OR the durable sidecar captured at the plan boundary by step 13. Every report states which of the two it used and over what window. THIS SECTION IS NOT COMPLETE FOR THE PLAN'S WHOLE LIFE and the document says so in its own words on the cover — see eventsRetentionResolution.

**6. Policy set as rendered.** The concrete policies the plan produced, by name and rule. Source: the CR's `renderedPolicies`. DESTROYED ON TERMINATION — `BuildTerminatePatch` at `svc.go:491` and `BuildCancelPatch` at `:478` both blank `FieldRenderedPolicies` to an empty slice. So for a terminated plan this section MUST come from the step-13 sidecar, or it renders as empty and the report silently understates what was enforced.

**7. Health.** Final health state and any recorded reasons. Source: CR `status`. Note `BuildTerminatePatch` also sets `FieldHealth` to `HealthUnknown`, so for terminated plans the last MEANINGFUL health value comes from the sidecar, not from the live CR.

**8. Storage footprint.** Size of each rendered format and the count of reports retained for this plan. Source: the step-6 report store's `List`. Deliberately does NOT include PVC-level capacity — that would require the `storageStats` singleton at `utils/snapshot/types.go:14`, which Part A leaves untouched on purpose.

**9. Appendix.** The complete gathered model as indented JSON, so nothing shown in prose is unverifiable.

Cross-cutting rules. No section, heading, table cell or footnote may name the policy-engine vendor — the term is "policy engine". The HTML renderer uses `html/template` for contextual auto-escaping because sections 4, 5 and 6 embed cluster-controlled resource names. Reports may contain namespaces, resource names and user identities, which is why both list and download are denyable reads gated on `ActionViewProtectionPlanReports`.

## Formats shipping

- HTML — html/template + embed (stdlib). The 'fancy' format: single self-contained file, embedded CSS, no external fonts, no CDN, no JavaScript. Cover page, styled tables, status colour coding, and @media print rules with page-break-inside:avoid so browser print-to-PDF produces a clean document. Contextual auto-escaping matters here because the report embeds cluster-controlled resource names.
- Markdown — text/template (stdlib). Same nine sections, GitHub-flavoured tables. The paste-into-a-ticket format.
- CSV — encoding/csv (stdlib). Tabular sections only: enforcement activity, protected resources, policy set. Roughly 40 lines. This is the format auditors and compliance reviewers actually ask for, and it is nearly free.
- JSON — encoding/json (stdlib). The full gathered model, indented. Machine-readable, and doubles as the report's own appendix.
- PDF — NOT shipped as a native renderer in v1. Delivered as print-optimised HTML plus documented browser print-to-PDF. See decision D1.
- NEW GO DEPENDENCIES: ZERO. All four shipping formats use only the standard library. This is a deliberate outcome, not a coincidence — the only format that would have required a dependency is PDF, and that is exactly the one held back. If you accept the go-pdf/fpdf alternative in D1 it becomes the single new dependency in the entire change, and it must be flagged and approved as such.
- RENDERING MODEL: all four formats are rendered EAGERLY at generation time and all four are persisted. Documents are tens of kilobytes. This makes 'the user can choose between them' literally true at download time with zero server work, and it deletes the format-picker modal, the generation-options step and any job-progress UI from the frontend entirely.

## Events retention resolution

**RESOLVED: capture-at-boundary is mandatory and is IN scope, as its own step (step 13). Continuous capture is a separate, costed, default-OFF option that I recommend against for v1.**

The problem is worse than the retention window, and the code settles it. `violations/violations.go:34-36` documents `RetentionWindow = "1h"` as mirroring the kube-apiserver `--event-ttl` default — on EKS that is not tunable from here. Line 197-199 reads Events LIVE via `dyn.Resource(eventGVR).List` with `FieldSelector: reason=PolicyViolation`. So a naive retrospective report covers at most the last hour.

But the plan teardown path destroys the data faster than the TTL does, twice over. At `svc.go:356` `Terminate` calls `CleanupByPlanID` FIRST — deleting the rendered policies — and only then, at `:361`, patches with `BuildTerminatePatch`, which at `:491` blanks `FieldRenderedPolicies` to an empty slice. `buildViolation` filters Events against that rendered set. The result: after a terminate, violations return EMPTY IMMEDIATELY. Not degraded, not one-hour-limited — empty. `Cancel` does the same at `:187` / `:478`.

A report generated after termination — which is the single most likely moment anyone wants one — would therefore show ZERO enforcement activity for a plan that may have blocked thousands of requests. That is not an incomplete report; it is an actively misleading one, and it is why this cannot be hidden inside a rendering step.

**The resolution, in scope, step 13.** Two hooks, each a single added statement placed before the existing `CleanupByPlanID` call — in `Terminate` before svc.go:356, and in `Cancel` before svc.go:187. Each calls the existing `violations.Collect` and persists the result as a `violations.json` sidecar in the plan's report directory through the step-6 store. That sidecar is a genuine durable record: it survives the CR patch, the policy cleanup and the Event TTL. `gather.go` prefers it when present and stamps `CoverageSource = captured`; with no sidecar it reads live and stamps `live`. Safety is absolute: the capture is wrapped so any failure is logged and swallowed — a plan MUST still terminate if report capture fails.

**The ceiling, stated plainly.** This captures the final window before the plan ended, not the plan's entire life. A plan that ran three weeks still loses week one's denials, because those Events expired long before anyone asked for a report. Nothing short of continuous capture recovers them, and they are already gone today.

**Therefore the coverage callout is MANDATORY, not decorative.** Every report prints, on its cover, the exact window its enforcement section covers and whether the data was captured or read live. Without it the document looks complete and silently isn't — worse than an empty report. This is the mitigation for the highest-consequence decision in the plan (D2).

**The alternative, costed and NOT recommended for v1.** Continuous capture — a poller on the existing 31s protection-plan controller tick appending observed violations to a durable store — is the only thing that yields full-life history. It is a MAJOR scope addition with its own step list: a poller whose LIST load multiplies by active-plans × namespaces, on a cluster with documented list-flood starvation history where session lookups once degraded to a 6.4s p95; an unbounded-growth store needing its own retention; event dedup across ticks; and a new failure mode where the poller falling behind produces gaps that look identical to "no violations". It roughly doubles Part B. If you want it, it becomes steps 13b through 13e — never a line inside a renderer.

## PVC and chart

**The single most dangerous line in the chart work: the snapshots PVC name must stay byte-identical.** `telark.exporterSnapshotsPvcName` (`_helpers.tpl:82-86`) is the single source for the PVC itself, the deployment's volume `claimName` and the `SNAPSHOTS_PVC_NAME` env. If the rendered name drifts by one character, Helm creates a NEW empty claim, the exporter mounts an empty volume, and every stored snapshot is orphaned — which breaks rollback, the product's recovery path. Everything below is arranged so that cannot happen.

**Second PVC without duplicating the template.** `templates/workloads/pvc.yaml` is currently 27 lines rendering one hardcoded claim. Wrap the existing body in a `range` over a two-entry inline list of dicts — `(list (dict "name" (include "telark.exporterSnapshotsPvcName" $) "size" $values.app.persistence.size) (dict "name" (include "telark.exporterReportsPvcName" $) "size" $values.app.persistence.reportsSize))` — with a `---` separator between documents. The body stays INLINE, following the `vpa.yaml` idiom rather than the named-template idiom used by `deployments.yaml`; the body is ~18 lines, well under the threshold where extraction earns its keep.

Crucially, the snapshots entry calls the EXISTING helper completely unchanged. Do NOT parameterise `telark.exporterSnapshotsPvcName` into a shared `telark.exporterPvcName "snapshots"` form — a parameterised helper is precisely the mechanism by which the byte-identical invariant gets broken. Add a plain sibling instead: `telark.exporterReportsPvcName` = `printf "%s-exporter-reports-pvc" (include "telark.fullname" .)`.

**Access mode — no new key, no template edit.** `_mode.tpl:32` already derives `app.persistence.accessMode` as `ReadWriteMany` when exporter replicas > 1 and `ReadWriteOnce` otherwise, with `app.singleNode` forcing replicas to 1 at `:28-30`. Both claims are mounted by the same pods, so both read that same derived value straight from `$values.app.persistence.accessMode`. Building the claims list inline in `pvc.yaml` rather than in `_mode.tpl` means **`_mode.tpl` needs ZERO edits**. Adding a per-claim accessMode key would be a second source of truth for a value that is already derived correctly.

**Storage class — shared, no new key.** Both claims use `app.persistence.storageClass` with the existing three-way semantics preserved verbatim: empty means cluster default, `"-"` renders `storageClassName: ""` to disable provisioning, anything else is a class name. `_storage_guard.tpl` is untouched. A separate class for reports would be configurability nobody asked for.

**Interaction with the exporter update strategy.** `_mode.tpl:31` sets `strategy` to `Recreate` at one replica and `RollingUpdate` above. At one replica both claims are RWO and `Recreate` is what makes that safe — the old pod releases both volumes before the new one binds them. At more than one replica the mode flips BOTH claims to RWX, which only works on an RWX-capable class such as `efs-sc`. That is exactly the existing constraint for snapshots today, so the reports PVC introduces **no new failure mode** — but it does double the exposure, because a misconfigured class now strands two volumes instead of one. This must be stated explicitly in `docs/INSTALL.md`.

**Values keys added.** `app.persistence.reportsSize` (default `2Gi`; `1Gi` in `modes/minimal.yaml` where size is currently 1Gi at line 18-19; `10Gi` in `modes/performance.yaml` where size is 50Gi at line 21-22). Under `services.exporter.env`: `REPORTS_PATH: "/reports"` with a YAML anchor mirroring the existing `&snapshotsPath`, plus `REPORTS_PVC_NAME` and `REPORTS_PVC_NAMESPACE` mirroring their snapshots counterparts, plus the matching `volumes` and `volumeMounts` entries.

**Schema — the hard gate.** I verified `app.persistence` in `values.schema.json` is `{"type":"object","additionalProperties":false,"properties":{"enabled","storageClass","size"}}`. `additionalProperties: false` means adding `reportsSize` to values WITHOUT adding it to the schema makes **every install and upgrade fail validation**. The schema edit is not optional and not a follow-up — it ships in the same commit as the values change.

**No `reports.enabled` gate.** Reports provision whenever `app.persistence.enabled` is true, same as snapshots. A separate gate would reproduce the existing ungated-volume mismatch in the deployment template, where a volume can be declared for a disabled feature.

**No deployment template edits at all.** `_deployment.tpl` already ranges over `volumes` and `volumeMounts` and runs `tpl` over `claimName` and env values, so the new mount and env come entirely from `values.yaml`.

**Verification.** `rtk proxy "helm template telark charts/telark"` — raw mode is mandatory because RTK truncates multi-document output and has previously shown 1 of 15 documents. Diff the full render against a pre-change render: the only differences permitted are the new PVC document and the new exporter env, volume and volumeMount entries. The snapshots PVC name must appear byte-identical in all three places it is rendered. Repeat at `app.mode` minimal / standard / performance, at `app.singleNode=true`, and at `services.exporter.replicas=2` to confirm both claims track the derived access mode together.

## Docs to update (same change)

- /Users/houssem/Desktop/Github/telark/docs/INSTALL.md — second PVC, app.persistence.reportsSize, and the explicit note that a multi-replica exporter needs an RWX-capable storage class (efs-sc) for BOTH volumes
- /Users/houssem/Desktop/Github/telark/charts/telark/README.md — new values keys with their gate semantics, matching the existing lean style (no per-key prose essays)
- /Users/houssem/Desktop/Github/telark/charts/telark/VALUES.md — REGENERATE with helm-docs, never hand-edit
- /Users/houssem/Desktop/Github/telark/services/exporter/README.md — the REPORTS_* env block in the same table style as the existing SNAPSHOTS_* block, plus the four new report routes
- /Users/houssem/Desktop/Github/telark/services/discovery/README.md — the generate and list routes, the report model, and the plan-boundary capture behaviour
- /Users/houssem/Desktop/Github/telark/docs/architecture.md — reports as a second artifact store over the shared artifact layer, and the discovery-renders / exporter-stores split
- /Users/houssem/Desktop/Github/telark/README.md — one line in the feature list
- /Users/houssem/Desktop/Github/telark/charts/telark/modes/minimal.yaml and modes/performance.yaml — reportsSize, which are values files but must move in lockstep with the docs or the documented defaults are wrong

## User-owned follow-ups

- Release /Users/houssem/Desktop/Github/internal/rest — the new endpoints/reports package. Works locally through the go.work replace; nothing ships until you tag it.
- Release /Users/houssem/Desktop/Github/internal/data — the two new role action constants in resources/role/{rules.go,builtin.go}. Same situation.
- Bump the internal/* pins in services/exporter/go.mod and services/discovery/go.mod after those releases. Note your dependency-upgrade workflow is currently mutating those same files — sequence this after it finishes.
- Bump services.<svc>.version in values.yaml and Chart.yaml. A GitHub Action owns versions; the plan never touches them.
- Build and push the exporter and discovery images.
- Apply the chart change to the cluster. This CREATES A NEW PVC — verify before applying that the snapshots claim name is byte-identical in the rendered output, and confirm the storage class can satisfy the access mode your replica count derives.
- All git operations: branch, commit, push, merge. The plan touches three repos (telark, internal, dashboard-ui) and I do none of them.
- Decide D1 (native PDF or print-optimised HTML) and D2 (bounded coverage or continuous capture). D2 in particular changes the size of Part B substantially.
- Confirm the step-13 hook placement after reviewing all six CleanupByPlanID call sites — a missed path means a silently unrecoverable gap.
- Remove the /Users/houssem/Desktop/Github/telark-baseline worktree if step 20 did not, and finish or discard the 40 staged exporter lint files, which this plan deliberately leaves exactly as it found them.

## Out of scope

- Native PDF rendering (go-pdf/fpdf or any PDF library). Print-optimised HTML plus documented browser print instead — see decision D1.
- Continuous violation capture over a plan's whole life. Capture-at-boundary only. This is the named major scope addition in eventsRetentionResolution and becomes steps 13b-13e only if you choose it.
- A reports GC goroutine, distributed lock or orphan sweeper. Per-plan Prune only. Explicitly do not reuse the exporter:snapshot:gc lock key.
- A reports storage-stats panel or PVC capacity display. That would require dissolving the storageStats package singleton at utils/snapshot/types.go:14, which Part A deliberately leaves untouched.
- Generalising snapshot retention, GC, generation resolution, manifest handling, sanitisation or request parsing. Only the five functions with two proven callers move. This is the boundary that keeps the rollback path untouched.
- Scheduled or recurring report generation, email or webhook delivery, and report diffing between two reports.
- A format-picker modal, generation-options UI or job-progress UI. All formats render eagerly, so the picker collapses into a plain download menu.
- Any change to snapshot behaviour, layout, retention policy or the V<n>.json filename codec.
- internal/composer — legacy, unused, not part of 'all packages'.
- Version bumps, Chart.yaml edits, image builds, module releases, pin bumps, cluster apply and all git operations. Listed under userOwnedFollowUps.
- Finishing or reverting the 40 staged exporter lint files, including the heavily modified tests/snapshot/logic_test.go. The plan works around them and leaves them exactly as found.
- Fixing the pre-existing ungated-volume mismatch in the deployment template, and splitting the 502-line Content.tsx. Both noted, neither touched.

## Risks

- HIGHEST — Chart: a drifted snapshots PVC name orphans the live snapshots volume and breaks rollback, the product's recovery path. Mitigated by refusing to parameterise telark.exporterSnapshotsPvcName and by a byte-identical render diff as the gate. Cheap to prevent, extremely expensive to discover in production.
- HIGHEST — svc.go step 13: the capture hooks sit one statement away from CleanupByPlanID on the plan teardown path. A hook that throws, blocks or reorders could prevent a plan from terminating. Mitigated by log-and-swallow error handling, by adding statements rather than restructuring, and by a test asserting the plan still terminates when capture fails.
- HIGH — values.schema.json has additionalProperties:false on app.persistence (verified). Adding reportsSize to values without the schema breaks EVERY install and upgrade. Single-line omission, total install failure.
- HIGH — The enforcement section can silently understate reality. For a terminated plan, renderedPolicies is blanked at svc.go:491 and live violations return empty immediately, so without the step-13 sidecar the report looks complete and shows zero denials for a plan that blocked thousands. The mandatory coverage callout is the mitigation and must not be dropped as 'clutter'.
- HIGH — The report download is the first blob download in dashboard-ui; there is NO existing pattern to copy. The obvious implementation is an <a href> with a token in the URL, which is exactly how session tokens leaked into nginx access logs before. Mitigated by header-only auth through a dedicated download helper, verified in the network panel.
- MEDIUM — Two shared modules (internal/rest, internal/data) are edited. Everything works locally via go.work and then breaks for everyone else until released and pinned. This is a hand-off cliff, not a code risk.
- MEDIUM — The baseline is genuinely compromised: 40 staged exporter files (+1036/-684) of unfinished lint work AND a concurrent dependency-upgrade workflow mutating go.mod/go.sum. Mitigated by the dual-baseline technique in step 1, but any build failure must be classified against both baselines before being treated as caused by this work.
- MEDIUM — Part A edits two files (logic_test.go, persist_test.go) that are part of the user's staged unfinished work. Repointing symbol qualifiers there risks entangling with edits I must neither finish nor revert. Mitigated by touching only the qualifier and by a final check that the staged diff is otherwise unchanged.
- MEDIUM — Reports have no GC goroutine, only per-plan Prune. A plan generating many reports, or plans deleted without their reports being pruned, leaves the reports volume growing. Accepted for v1 and recorded as a ponytail ceiling; the deliberate alternative — a GC sweeper — was rejected because the existing snapshot GC lock gcTickAllowed FAILS OPEN and getting a second sweeper wrong turns CollectOrphans into a volume-wide deleter.
- LOW — The cross-service raw "path" key at exporters/snapshot/def.go:136 is read by discovery at clients/snapshots.go:122 and a miss degrades SILENTLY through circuitbreaker.NotCounted. Part A does not intend to touch it; the step-2 golden test exists so that intent is enforced rather than trusted.
- LOW — Content.tsx is already 502 lines with five SettingsCards; a sixth pushes it further. Not a correctness risk, but the file is approaching the point where the details page wants splitting — worth mentioning, not worth doing inside this change.

## TDD — baseline procedure

GOAL: a trustworthy, RECORDED green baseline taken IN PLACE on the working tree. Do NOT stash, do NOT create a git worktree, do NOT finish or revert the unfinished lint work.

WHY the obvious baseline is unsound (verified, not assumed):
- /Users/houssem/Desktop/Github/go.work DOES NOT EXIST. The workspace file is /Users/houssem/Desktop/Github/telark/go.work and it is STAGED-MODIFIED. HEAD has `go 1.26.5` and ZERO replace directives; the working tree has `go 1.27.1` plus the four `replace github.com/telark/{data,rest,kcore,x-ware} => /Users/houssem/Desktop/Github/internal/*` lines. A clean worktree or a stash would compile against published module pins, not the local internal/* sources — a different program. Any baseline taken there is worthless.
- /Users/houssem/Desktop/Github/telark/.golangci.yml is ALSO staged-modified, so lint parity only exists in the working tree.
- Repo-wide staged scope is 106 files (not ~40): services/exporter AND ~55 in services/discovery, plus .github workflows, charts/telark-crds, a clusterrole, auth and notifier.
- GOOD NEWS, verified: `git status --porcelain -- services/exporter/internal/utils/snapshot/` is EMPTY. All 12 files in the Part A perimeter are clean, so `git diff -- <file>` IS a valid zero-diff gate there.
- BAD NEWS, verified: services/discovery/internal/core/plans/protection/svc.go and .../violations/violations.go are BOTH staged-modified. Every line number quoted for them in the plan is stale. Re-derive every location by SYMBOL NAME (grep -n 'func Collect'), never by the quoted line number.

PROCEDURE (run once, before step 1; artefacts to the session scratchpad, $SP):

B0. Quiesce check against the concurrent dependency-upgrade workflow.
    cd /Users/houssem/Desktop/Github/telark && git status --porcelain -- '*go.mod' '*go.sum' | tee $SP/dep.state
    shasum services/*/go.mod services/*/go.sum /Users/houssem/Desktop/Github/internal/*/go.mod /Users/houssem/Desktop/Github/internal/*/go.sum | tee $SP/dep.sha
    Re-run the shasum and diff against $SP/dep.sha before EVERY Part A green gate. If the hashes moved, the dep workflow touched the modules: RE-BASELINE, do not attribute the delta to your refactor.

B1. Freeze the reference state (read-only): git status --porcelain > $SP/baseline.status ; git rev-parse HEAD > $SP/baseline.head

B2. Build + test baseline, working tree, cache defeated:
    cd services/exporter && go build ./... && go test ./... -count=1 2>&1 | tee $SP/baseline.exporter.test
    cd services/discovery && go build ./... && go test ./... -count=1 2>&1 | tee $SP/baseline.discovery.test
    cd /Users/houssem/Desktop/Github/internal/rest && go test ./... -count=1 2>&1 | tee $SP/baseline.rest.test
    cd /Users/houssem/Desktop/Github/internal/data && go test ./... -count=1 2>&1 | tee $SP/baseline.data.test
    Reduce each to a per-package ok/FAIL set: grep -E '^(ok|FAIL|---)' $SP/baseline.<m>.test | sort > $SP/baseline.<m>.set
    CRITICAL: a package that is ALREADY FAILING because of the unfinished lint edits (notably the heavily-modified internal/tests/snapshot/logic_test.go, ~386 changed lines) IS THE BASELINE. Do not fix it. The Part A gate is SET EQUALITY against baseline.<m>.set, not "everything green".

B3. Lint baseline (project config only, never --no-config, no GOTOOLCHAIN prefix):
    cd services/<m> && golangci-lint run 2>&1 | tee $SP/baseline.<m>.lint
    grep -cE '^\S+\.go:[0-9]+' $SP/baseline.<m>.lint > $SP/baseline.<m>.lintcount
    Discovery is expected to be large (~462 issues: ~437 revive + ~17 misspell). Gate = the count must NOT INCREASE.

B4. Chart baseline. RTK truncates multi-document helm output even through a `>` redirect, so use rtk proxy:
    for m in minimal standard performance: rtk proxy "helm template t /Users/houssem/Desktop/Github/telark/charts/telark --set app.mode=$m --set app.persistence.storageClass=validate" > $SP/baseline.chart.$m.yaml
    cd /Users/houssem/Desktop/Github/telark && make helm-lint 2>&1 | tee $SP/baseline.helm.lint
    Extract the snapshots PVC alone for the later unchanged-assertion: awk '/kind: PersistentVolumeClaim/,/^---/' $SP/baseline.chart.$m.yaml > $SP/baseline.pvc.$m.yaml

B5. Frontend baseline: cd /Users/houssem/Desktop/dashboard-ui && npm run check-all 2>&1 | tee $SP/baseline.ui.check
    This MUST be zero errors before anything starts; if it is not, stop and report — the UI gate is unusable otherwise.

B6. Perimeter gate arming: confirm git status --porcelain -- services/exporter/internal/utils/snapshot/ is still empty. Record the ten files that must stay BYTE-IDENTICAL through all of Part A: file.go, storage.go, request.go, retention.go, gc.go, resolve.go, manifest.go, sanitize.go, payload.go, types.go. (payload.go is included deliberately — it holds coerceInt's float64 branch, the same decode class that once lost every scale-down.) Only persist.go and paths.go are expected to change in step 8, and only request.go/storage.go in steps 9/10.

GIT CHECKPOINTS: the TDD methodology asks for test:/fix: checkpoint commits. Git commit/push/branch is OUT OF BOUNDS for this work. Frame every checkpoint as user-owned: at each RED and each GREEN, print the suggested commit message and STOP for the user to commit if they want one. Never run git commit yourself.

## TDD sequence

- **1. The CreateSnapshot HTTP 200 body is a stable contract. It must keep the literal `path` key, because discovery's clients/snapshots.go reads dataMap[constants.SnapshotPathKey] and a miss degrades SILENTLY via circuitbreaker.NotCounted. Highest-value pin in the plan: it converts a silent cross-service break into a loud one before any code moves.**
  - _part_: A
  - _layer_: characterisation / cross-service contract
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshotapi/contract_test.go
  - _testNames_:
    - TestCreateSnapshotResponseIsByteStableGolden
    - TestCreateSnapshotResponseCarriesThePathKeyDiscoveryReads
  - _redCriterion_: Throwaway probe: rename the raw "path" string literal in exporters/snapshot/def.go (inside createSnapshotResponse) to "pathx". Both tests must FAIL with a key-set diff naming `path`. Revert the probe immediately. If the tests still pass with the literal renamed, they are tautological and must be rewritten.
  - _greenCriterion_: With def.go untouched, the golden matches byte for byte after normalising only the environment-dependent PVC fields (pvcAvailable/pvcTotal/pvcUsedPercent), and the `path` assertion passes. go test ./internal/tests/snapshotapi/ -count=1 green.
  - _implementationFiles_:

- **2. The snapshot-infos response key set is stable. Narrowed deliberately: ReadSnapshot has NO return value (it writes to an http.ResponseWriter) and GetStorageInfo performs a LIVE PVC lookup via core.GetPersistentVolumeClaimCapacityBytes, so a byte-for-byte golden on the read side is unachievable. Key set only; values are not asserted.**
  - _part_: A
  - _layer_: characterisation / read-side response
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshotapi/contract_test.go
  - _testNames_:
    - TestReadSnapshotInfosResponseKeySetIsStable
  - _redCriterion_: Throwaway probe: delete one key from the infos payload builder. The test must FAIL naming the missing key. Revert.
  - _greenCriterion_: Unmodified code passes. No helper may be added to def.go itself — any test helper goes in the test file, so step 1's 'exactly the probe lines changed' check on def.go stays meaningful.
  - _implementationFiles_:

- **3. A generation arriving as a JSON float64 decodes correctly, including a DECREASED generation, and a non-integral value is rejected rather than truncated. This is the exact decode class that once silently lost every scale-down.**
  - _part_: A
  - _layer_: characterisation / decode (scale-down bug class)
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/decode_test.go
  - _testNames_:
    - TestParseCreatePayloadAcceptsJSONFloat64Generations
    - TestParseCreatePayloadRejectsNonIntegralGenerations
    - TestParseCreateSnapshotRequestAcceptsADecreasedGeneration
    - TestParseCreatePayloadAcceptsIntAndStringGenerationsIdentically
  - _redCriterion_: Throwaway probe: in utils/snapshot/request.go delete `case float64:` from toPositiveInt so JSON numbers fall to `default`. The float64 tests must FAIL with ErrSnapshotInvalidInt. Revert.
  - _greenCriterion_: Unmodified code passes all four. Reached only through the EXPORTED ParseCreatePayload / ParseCreateSnapshotRequest — toPositiveInt is unexported and stays that way, so NO helper-exposure step is needed here.
  - _implementationFiles_:

- **4. Retention orders by PARSED generation, never lexically and never by mtime. V10 must outrank V2. A rewrite of an existing generation (the force-sync shape) must not clobber an older surviving generation, and non-generation files in the directory are left alone.**
  - _part_: A
  - _layer_: characterisation / retention ordering (force-sync vs flush-history class)
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/retention_order_test.go
  - _testNames_:
    - TestRetentionKeepsHighestGenerationsNotLexicallyLatest
    - TestRetentionIgnoresMtimeOrder
    - TestRetentionLeavesNonGenerationFilesAlone
    - TestRewritingAnExistingGenerationDoesNotDeleteOlderSurvivors
  - _redCriterion_: Throwaway probe: in retention.go replace cmp.Compare(a.ver, b.ver) with a strings.Compare over filepath.Base. V10 is then deleted before V2 and TestRetentionKeepsHighestGenerationsNotLexicallyLatest FAILS. Revert.
  - _greenCriterion_: Unmodified code passes. NOTE, verified: ApplyRetentionPolicy already takes `snapshotsPath string` as its first parameter — it is ALREADY generic. No refactor step is needed for it; this is characterisation only.
  - _implementationFiles_:

- **5. The sweep never removes the snapshots root or a scope directory, keeps orphans still inside the age grace, and tolerates a referenced path that no longer exists on disk without panicking. Extends the two existing gc_test.go cases at their weakest edges.**
  - _part_: A
  - _layer_: characterisation / GC and orphan files
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/gc_guards_test.go
  - _testNames_:
    - TestSnapshotGCNeverRemovesRootOrScopeDir
    - TestSnapshotGCKeepsOrphansInsideTheGracePeriod
    - TestSnapshotGCToleratesReferencedPathsThatNoLongerExist
    - TestSnapshotGCTreatsALeftoverTempFileAsSweepable
  - _redCriterion_: Probe A: pass a zero minAge into CollectOrphans from the test's own call (no production edit) and assert the young orphan is then listed — proving the grace gate is what keeps it; then restore the real grace and require it kept. Probe B: in gc.go make the sweepable check return true unconditionally; TestSnapshotGCNeverRemovesRootOrScopeDir must FAIL. Revert probe B.
  - _greenCriterion_: Unmodified code passes all four alongside the two pre-existing gc_test.go tests. Pin BOTH temp-suffix constants (SnapshotTempSuffix '.*.tmp' for writes vs SnapshotTempFileSuffix '.tmp' for the sweep) so a future unification cannot silently orphan files.
  - _implementationFiles_:

- **6. Snapshots are apply-clean PRE-update states. Sanitisation is idempotent, strips the same field set from a live object and a stored object, and a sanitised manifest survives a WriteSnapshotJSON -> LoadSnapshotData round trip unchanged. That stability is exactly what the 'reconcile trusts the newest snapshot only when history:post is stamped with the stored generation' rule depends on, and nothing currently pins it end to end.**
  - _part_: A
  - _layer_: characterisation / fingerprint + post-stamp reconcile rule
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/fingerprint_test.go
  - _testNames_:
    - TestSanitizeManifestIsIdempotent
    - TestSanitizedManifestSurvivesWriteAndLoadUnchanged
    - TestSanitizeStripsTheSameFieldsFromLiveAndStoredObjects
    - TestFingerprintOfASanitizedManifestIsStableAcrossARoundTrip
  - _redCriterion_: Throwaway probe: add one volatile field (e.g. a resourceVersion-like key) to the sanitize strip list in sanitize.go. TestSanitizeStripsTheSameFieldsFromLiveAndStoredObjects and the round-trip hash test must FAIL. Revert.
  - _greenCriterion_: Unmodified code passes all four. The fingerprint is a sha256 over canonical JSON computed IN THE TEST; no production fingerprint function is introduced in Part A.
  - _implementationFiles_:

- **7. A write that fails never produces a visible generation file and never leaves a temp file behind. Complements the two existing persist_test.go success-path cases with their failure-path mirrors — the flush-ordering / orphan-file class.**
  - _part_: A
  - _layer_: characterisation / partial writes and flush ordering
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/persist_failure_test.go
  - _testNames_:
    - TestWriteToUnwritableDirLeavesNoGenerationFile
    - TestFailedWriteLeavesNoTempFileBehind
    - TestWriteReturnsTheUnderlyingErrorRatherThanSwallowingIt
  - _redCriterion_: Throwaway probe: in persist.go drop the error return from os.Rename (return nil unconditionally). TestWriteReturnsTheUnderlyingErrorRatherThanSwallowingIt must FAIL. Revert. Skip the unwritable-dir cases when running as root, where a read-only dir is not enforced.
  - _greenCriterion_: Unmodified code passes all three, and the two pre-existing persist_test.go tests stay green.
  - _implementationFiles_:

- **8. Extract the two primitives with a PROVEN second caller — atomic JSON write and base containment — into a scope-neutral package, keeping three-line delegating wrappers in package snapshot. Everything else stays put, deliberately: FormatBytes, FormattedFileSize and ValidateSnapshotIdentity have no second caller yet, and moving them would force edits to file.go:21, storage.go:214, storage.go:217 and request.go:109, which all call them UNQUALIFIED intra-package (verified). Keeping them in place dissolves that defect instead of papering over it.**
  - _part_: A
  - _layer_: refactor / shared storage primitives
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/fileio/fileio_test.go
  - _testNames_:
    - TestWriteJSONAtomicIsCrashSafeUnderConcurrency
    - TestWriteJSONAtomicLeavesNoTempFile
    - TestWriteJSONAtomicFailsLoudlyOnAnUnwritableDir
    - TestIsWithinBaseRejectsTraversal
    - TestIsWithinBaseAcceptsTheBaseItself
    - TestIsWithinBaseRejectsASiblingSharingAPathPrefix
  - _redCriterion_: COMPILE-TIME RED (legitimate under the methodology): the new test file imports github.com/telark/exporter/internal/utils/fileio, which does not exist. go test ./internal/tests/fileio/ -count=1 fails to build with 'no required module provides package .../utils/fileio'. That build failure IS the RED signal.
  - _greenCriterion_: THREE conditions, all required. (1) The six new tests pass. (2) EVERY pre-existing test in internal/tests/snapshot and internal/tests/snapshotapi passes unchanged — not one line of an existing test rewritten. (3) git diff --stat -- services/exporter/internal/utils/snapshot/ shows changes in persist.go and paths.go ONLY; git diff --quiet -- <each of file.go storage.go request.go retention.go gc.go resolve.go manifest.go sanitize.go payload.go types.go> exits 0 for all ten.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/fileio/fileio.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/persist.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/paths.go
- **9. LoadSnapshotData currently hard-codes envmanager.GetSnapshotsPath() as its containment base (request.go, verified). Widen it to LoadArtifactData(path, base string) and keep LoadSnapshotData(path) as a three-line wrapper. This is the change that actually makes the read path support a second scope — the reports root — without duplicating it.**
  - _part_: A
  - _layer_: refactor / parameterise the artifact root
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/base_path_test.go
  - _testNames_:
    - TestLoadArtifactDataRejectsPathsOutsideTheSuppliedBase
    - TestLoadArtifactDataAcceptsAPathInsideASecondRoot
    - TestLoadSnapshotDataStillUsesTheSnapshotsRoot
  - _redCriterion_: COMPILE-TIME RED: LoadArtifactData is undefined; the test package fails to build.
  - _greenCriterion_: All three pass AND the pre-existing LoadSnapshotData traversal cases in logic_test.go (the /etc/hosts rejection and the missing-file case) stay green untouched. git diff -- request.go shows only the widened signature plus the added wrapper — no other hunk.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/request.go
- **10. GetStorageInfo hard-codes the snapshots PVC name and namespace from the env manager and performs a live PVC capacity lookup. Widen to StorageInfoFor(root, pvcNamespace, pvcName string) and keep GetStorageInfo() as a wrapper, so the reports PVC can report its own usage through the same code.**
  - _part_: A
  - _layer_: refactor / parameterise storage info
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/snapshot/storage_scope_test.go
  - _testNames_:
    - TestStorageInfoForReportsUnknownWhenThePVCLookupFails
    - TestStorageInfoForUsesTheSuppliedRootForUsageNotTheSnapshotsRoot
    - TestGetStorageInfoDelegatesToTheSnapshotsPVC
    - TestASecondRootDoesNotCorruptTheSnapshotsStatsCache
  - _redCriterion_: COMPILE-TIME RED: StorageInfoFor is undefined.
  - _greenCriterion_: All four pass AND the pre-existing TestStorageStatsCache, TestGetStorageInfo and TestComputeSnapshotsStorageStats in logic_test.go stay green untouched. The fourth test exists because the package-level storageStats singleton in types.go is shared state — without it the reports PVC would silently report the snapshots figures.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/snapshot/storage.go
- **11. Part A closes only when the refactor is provably behaviour-preserving against the recorded baseline.**
  - _part_: A
  - _layer_: gate / zero behaviour delta
  - _testFile_: (command gate, no test file)
  - _testNames_:
    - go test ./... -count=1 in services/exporter
    - golangci-lint run in services/exporter
    - git diff --stat over the snapshot perimeter
    - shasum diff against $SP/dep.sha
  - _redCriterion_: n/a — this is a gate, not a test.
  - _greenCriterion_: ALL FOUR: (1) the per-package ok/FAIL set equals $SP/baseline.exporter.set EXACTLY, including any package already failing from the unfinished lint edits; (2) the golangci-lint issue count is <= $SP/baseline.exporter.lintcount; (3) git diff --stat -- services/exporter/internal/utils/snapshot/ touches ONLY persist.go, paths.go, request.go, storage.go; (4) the dep-workflow shasum still matches $SP/dep.sha — if it moved, re-baseline before judging.
  - _implementationFiles_:

- **12. A report renders into each supported document format from one activity record. An unsupported format is REJECTED, never silently defaulted to a fallback. The supported-format list is a single source of truth shared by the renderer, the route validation and the response Content-Type.**
  - _part_: B
  - _layer_: exporter domain / format registry
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/report/render_test.go
  - _testNames_:
    - TestRenderProducesANonEmptyDocumentForEverySupportedFormat
    - TestRenderRejectsAnUnsupportedFormat
    - TestRenderNeverSilentlyFallsBackToADefaultFormat
    - TestSupportedFormatsIsTheSingleSourceOfTruth
    - TestRenderedDocumentCarriesTheContentTypeForItsFormat
  - _redCriterion_: COMPILE-TIME RED: package github.com/telark/exporter/internal/report does not exist; the test target fails to build.
  - _greenCriterion_: All five pass. TestSupportedFormatsIsTheSingleSourceOfTruth iterates the exported format list and asserts every entry has both a renderer and a Content-Type — so adding a format later without wiring it fails here rather than at runtime.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/types.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/formats.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/render.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/report.go
- **13. The document must narrate what actually happened in the cluster during the plan. It states the plan identity and window, the policy set in force, EVERY violation it was given with timestamp / namespace / resource / rule / result, and the health plus its reason at capture. A plan with no activity at all says so explicitly rather than rendering a blank section. A report generated AFTER the plan ended is NOT empty — it renders from the captured sidecar, because violations cannot be re-collected retrospectively (Kubernetes Events carry ~1h retention and both plan-destruction paths wipe the policy set).**
  - _part_: B
  - _layer_: exporter domain / report CONTENT as first-class behaviour
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/report/content_test.go
  - _testNames_:
    - TestReportNarratesEveryViolationItWasGiven
    - TestReportStatesThePlanIdentityAndWindow
    - TestReportListsThePolicySetThatWasInForce
    - TestReportRecordsHealthAndItsReason
    - TestReportForAPlanWithNoActivitySaysSoExplicitly
    - TestRetrospectiveReportGeneratedAfterThePlanEndedIsNotEmpty
    - TestReportDropsNothingWhenTheViolationListIsLarge
  - _redCriterion_: RUNTIME RED against the real renderer from step 12, which at this point emits only a header: each content assertion fails naming the section it could not find. TestRetrospectiveReportGeneratedAfterThePlanEndedIsNotEmpty fails because the renderer has no captured-activity path at all.
  - _greenCriterion_: All seven pass for EVERY supported format — the content assertions are format-parameterised, so a format that renders a pretty shell with no content cannot pass. TestReportDropsNothingWhenTheViolationListIsLarge asserts count equality against the input, catching any silent page cap leaking in from the violations Page() helper.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/render.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/sections.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/report.go
- **14. A report is persisted atomically under its OWN root via the step-8 primitives, never under the snapshots root. Retention keeps the newest N per plan, and the orphan sweep prunes the reports root without touching the snapshots root. Every report path is contained inside the reports root.**
  - _part_: B
  - _layer_: exporter storage / reports root
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/report/store_test.go
  - _testNames_:
    - TestWriteReportIsAtomicUnderConcurrency
    - TestFailedReportWriteLeavesNoArtifact
    - TestReportRetentionKeepsTheNewestPerPlan
    - TestReportGCSweepsTheReportsRootOnly
    - TestReportGCLeavesTheSnapshotsRootUntouched
    - TestReportPathIsContainedInTheReportsRoot
    - TestReportPathRejectsATraversingPlanID
  - _redCriterion_: COMPILE-TIME RED: internal/report/store.go and the reports env accessors do not exist.
  - _greenCriterion_: All seven pass, and re-running internal/tests/snapshot proves the snapshots retention and GC are unchanged. TestReportGCLeavesTheSnapshotsRootUntouched sets up BOTH roots and asserts cross-root isolation explicitly — this is the 'retention and GC on the new scope' edge the critique asked for.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/store.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/managers/envs/reports.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/report.go
- **15. Generation failure and partial writes surface as errors and never as an empty success. A failed render leaves no artifact; an unwritable root leaves no artifact; a zero-byte artifact is never reported as a successful report.**
  - _part_: B
  - _layer_: exporter domain / generation failure
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/report/failure_test.go
  - _testNames_:
    - TestGenerateReturnsAnErrorWhenRenderingFails
    - TestGenerateLeavesNoArtifactWhenRenderingFails
    - TestGenerateLeavesNoArtifactWhenTheRootIsUnwritable
    - TestGenerateNeverReportsSuccessWithAZeroByteArtifact
    - TestGenerateWithAnUnsupportedFormatWritesNothing
  - _redCriterion_: COMPILE-TIME RED first (internal/report/generate.go absent), then RUNTIME RED on TestGenerateNeverReportsSuccessWithAZeroByteArtifact against a naive implementation that writes before it validates.
  - _greenCriterion_: All five pass. Generate validates the format, renders fully in memory, and only then calls the atomic write — so there is no ordering in which a partial or empty artifact becomes visible.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/report/generate.go
- **16. Report endpoints are declared in the shared rest module in the SAME shape as snapshots/def.go (verified: snapshots/create, snapshots/{id}/get, snapshots/{id}/manifest, snapshots/infos, snapshots/{id}/delete), plus the response types. No token-bearing path segment anywhere.**
  - _part_: B
  - _layer_: rest module / endpoint definitions
  - _testFile_: /Users/houssem/Desktop/Github/internal/rest/tests/router/reports_key_test.go
  - _testNames_:
    - TestReportEndpointsProduceDistinctRouterKeys
    - TestReportEndpointPathsFollowTheSnapshotsShape
    - TestNoReportEndpointCarriesATokenBearingSegment
  - _redCriterion_: COMPILE-TIME RED: package github.com/telark/rest/endpoints/reports does not exist.
  - _greenCriterion_: All three pass AND — because telark/go.work replaces github.com/telark/rest with this absolute local path, so a break here breaks all four services instantly — cd /Users/houssem/Desktop/Github/telark && go build ./... succeeds immediately afterwards. Run that build as part of THIS step's green gate, not later.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/reports/def.go
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/reports/types.go
- **17. Completely new report routes (constraint C1) registered in routes/base.go in the same style as snapshotRoutes, with handlers that create, list, get and download. A format the server does not support is rejected at the route with 422, not rendered.**
  - _part_: B
  - _layer_: exporter handlers + routes + authz coverage
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/reportapi/handler_test.go
  - _testNames_:
    - TestCreateReportHandler
    - TestCreateReportHandlerRejectsAMalformedBodyWith422
    - TestListReportsHandler
    - TestGetReportHandlerReturnsTheDocument
    - TestDownloadReportHandlerSetsContentDispositionAndContentType
    - TestUnsupportedFormatIsRejectedWith422
    - TestGetReportForAnUnknownPlanReturns404
  - _redCriterion_: TWO genuine REDs. (a) Compile-time: the reportapi test package imports handlers/report which does not exist. (b) After registering the routes but BEFORE adding requirements, the PRE-EXISTING deny-by-default coverage test TestRequirementsCoverEveryRoute in internal/tests/authz/requirements_test.go FAILS naming the exact new router keys. That is a real, already-wired RED gate — use it, do not bypass it.
  - _greenCriterion_: All seven handler tests pass AND both TestRequirementsCoverEveryRoute and TestNoRequirementWithoutRoute are green again. Mirror the existing snapshot handler test style exactly: httptest recorders, gorilla mux.SetURLVars for path vars, direct handler.X()(rec, req) invocation.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/handlers/report/handler.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/exporters/report/def.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/routes/base.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/requirements.go
- **18. A report describes cluster activity and can carry resource names, namespaces and user identities, so download is not public. Generation requires write on the plan's scope; download requires read plus a denyable action. Auth travels ONLY in the X-Session-Token header — never a URL path or query segment. (Tokens leaked into nginx access logs once via /sessions/tokens/<token>/ paths; a download link is exactly the shape that reintroduces it.)**
  - _part_: B
  - _layer_: exporter authz / unauthorised download
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/report_routes_test.go
  - _testNames_:
    - TestReportDownloadRefusesACallerWithoutTheApplicationsScope
    - TestReportDownloadHonoursTheDenyAction
    - TestReportGenerationRequiresWriteNotRead
    - TestReportRoutesCarryNoTokenBearingPathSegment
    - TestReportDownloadIgnoresACallerSuppliedIdentityHeader
  - _redCriterion_: Set the download requirement to authz.Public. TestReportDownloadRefusesACallerWithoutTheApplicationsScope and TestReportDownloadHonoursTheDenyAction must FAIL (200 where 403 was required). Then set it correctly. TestReportDownloadIgnoresACallerSuppliedIdentityHeader is RED until the spoofable-header strip covers the new routes — model it on the existing TestCallerSuppliedUserIDHeaderIsReplaced in subject_routes_test.go.
  - _greenCriterion_: All five pass. The download requirement is Denyable(Read(ScopeApplications), ActionDownloadProtectionPlanReports); generation is Write(ScopeApplications). Reuse the existing stubResolver / sessionUsers / userGrants fixtures from internal/tests/authz/subject_routes_test.go rather than inventing a new fixture.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/requirements.go
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/reports/def.go
- **19. The two new denyable actions are declared in the protection-plan action block in rules.go. NOTHING is added to builtin.go: verified, `grep -c Action internal/data/resources/role/builtin.go` returns 0 — built-in roles contain zero action references, because actions are deny-flags on scope+level, never granted. Any instruction to 'grant' the action in builtin.go is unfollowable and is deleted from the plan.**
  - _part_: B
  - _layer_: internal/data / role actions
  - _testFile_: /Users/houssem/Desktop/Github/internal/data/tests/policies/report_actions_test.go
  - _testNames_:
    - TestProtectionPlanReportActionsAreRegisteredInTheirScope
    - TestReportActionKeysFollowTheDashboardVocabulary
    - TestReportActionsAreNotReferencedByAnyBuiltinRole
  - _redCriterion_: COMPILE-TIME RED: the action constants do not exist.
  - _greenCriterion_: All three pass, and the existing internal/data/tests/policies/coverage_test.go and tests/builtin/builtin_test.go stay green. TestReportActionsAreNotReferencedByAnyBuiltinRole is the guard that keeps a future contributor from re-introducing the mistake this step deletes. Follow with cd /Users/houssem/Desktop/Github/telark && go build ./... because of the go.work replace.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/data/resources/role/rules.go
- **20. Discovery cannot write to the exporter's reports PVC — exporter owns CRDs and storage. It must go over HTTP. This step creates the missing rest/clients/reports layer and the discovery wrapper that consumes it. It MUST precede the lifecycle hook (order 22) — that ordering is the fix for the plan's circular dependency: the hook cannot dispatch through a wrapper that does not exist yet.**
  - _part_: B
  - _layer_: rest client + discovery wrapper
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/clients/reports_test.go
  - _testNames_:
    - TestReportClientPostsTheGenerateRequest
    - TestReportClientSurfacesAGenerationFailure
    - TestReportClientDoesNotCountAClientErrorAgainstTheBreaker
    - TestReportClientCarriesTheServiceTokenInAHeader
  - _redCriterion_: COMPILE-TIME RED: github.com/telark/rest/clients/reports and internal/clients/reports.go are both absent.
  - _greenCriterion_: All four pass; cd /Users/houssem/Desktop/Github/telark && go build ./... succeeds. Follow the established shape exactly: the wrapper lives in discovery/internal/clients/ and holds a *reportsclient.Client built from a shared.ClientConfig — discovery handlers must NEVER import a rest-pkg client directly. Mirror internal/clients/snapshots.go, including its circuitbreaker.NotCounted handling for client-side errors.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/rest/clients/reports/client.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/clients/reports.go
- **21. At the moment a plan is cancelled or terminated, the activity about to be destroyed is captured IN FULL, before anything is wiped. Widened per critique from violations-only to {violations, renderedPolicies, health, healthReason, capturedAt}: violations.Collect returns violations only, but the report's policy-set and health sections need the other three, and both values are already in hand at the hook site — BuildCancelPatch and BuildTerminatePatch each set FieldRenderedPolicies to an empty slice AND FieldHealth to HealthUnknown, so after the patch they are gone forever. Capture must happen BEFORE CleanupByPlanID, which is the first statement of the terminate path.**
  - _part_: B
  - _layer_: discovery domain / activity capture sidecar
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planviolations/sidecar_test.go
  - _testNames_:
    - TestCaptureRecordsViolationsPoliciesAndHealthTogether
    - TestCancelAndTerminateCaptureTheSameShape
    - TestCaptureHappensBeforeCleanupByPlanID
    - TestCaptureHappensBeforeTheRenderedPoliciesArePatchedAway
    - TestCaptureOfAPlanWithNoViolationsStillRecordsPoliciesAndHealth
  - _redCriterion_: RUNTIME RED on the ordering assertions: with no capture wired, TestCaptureHappensBeforeCleanupByPlanID fails because no capture was observed at all; once naively wired AFTER cleanup it fails on sequence. TestCaptureHappensBeforeTheRenderedPoliciesArePatchedAway fails with an empty policy list.
  - _greenCriterion_: All five pass. UNEXPORTED-SYMBOL RULE: resolvePlanNamespaces is unexported — if the test needs it, add an EXPORTED Capture(...) that wraps it; do NOT add a *_internal_test.go beside production code. LINE NUMBERS: svc.go and violations.go are BOTH staged-modified, so locate every edit site by symbol name (grep -n 'func (s \*Service) Terminate', grep -n 'func BuildTerminatePatch'), never by a quoted line number. Reuse the existing dynamicfake.NewSimpleDynamicClientWithCustomListKinds fixtures and testutil.Equal from violations_test.go.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/violations/violations.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/svc.go
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go
- **22. Report generation is dispatched on plan creation and on both destruction paths, carrying the captured activity and the requested format. A generation failure NEVER blocks the lifecycle — the plan still cancels or terminates.**
  - _part_: B
  - _layer_: discovery lifecycle / dispatch
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planctl/report_dispatch_test.go
  - _testNames_:
    - TestTerminateDispatchesReportGenerationWithTheCapturedActivity
    - TestCancelDispatchesReportGeneration
    - TestReportGenerationFailureDoesNotBlockTermination
    - TestDispatchCarriesTheRequestedFormat
    - TestDispatchIsNotAttemptedTwiceForOneTermination
  - _redCriterion_: RUNTIME RED: with no dispatch wired, the stub report client records zero calls. TestReportGenerationFailureDoesNotBlockTermination is RED against a naive implementation that returns the client error up the lifecycle path.
  - _greenCriterion_: All five pass and the existing planctl / planhandlers suites stay green. The dispatch consumes the step-20 wrapper, never a rest-pkg client directly.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/svc.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/clients/reports.go
- **23. Reports get their OWN PVC (constraint C2), never sharing the snapshots PVC. Both claims render with correct names, sizes, access modes and storage classes across all three modes, and the snapshots PVC is BYTE-IDENTICAL to the baseline render.**
  - _part_: B
  - _layer_: chart / dedicated reports PVC
  - _testFile_: (chart rendering gate — helm lint + helm template assertions, no Go test)
  - _testNames_:
    - helm lint charts/telark
    - helm template per mode: exactly two PVCs render with the expected names
    - helm template per mode: accessMode is RWO at replicas=1 and RWX at replicas>1 for BOTH claims
    - helm template per mode: each claim carries its own size and storageClassName
    - diff of the snapshots PVC against $SP/baseline.pvc.$m.yaml is empty
    - make helm-validate across every K8S_VERSIONS entry
  - _redCriterion_: A genuine schema RED, free of charge: values.schema.json declares app.persistence with additionalProperties:false and exactly {enabled, storageClass, size} (verified). Adding app.persistence.reports to values.yaml WITHOUT the schema entry makes `helm lint charts/telark` FAIL immediately — that lint failure is this step's RED. Also RED first: run the two-PVC grep assertions against the unmodified chart, where the reports claim is absent.
  - _greenCriterion_: ALL of: helm lint clean on both charts; exactly two PersistentVolumeClaim documents named <fullname>-exporter-snapshots-pvc and <fullname>-exporter-reports-pvc; the snapshots claim diffs EMPTY against the baseline extract for each mode; accessMode derives from the same replica count as the snapshots claim via _mode.tpl (app.singleNode still forces replicas=1 -> RWO for both); storageClassName is emitted only when set and rendered as "" when the value is "-"; make helm-validate passes. Use the inline-body `range` idiom already used by templates/workloads/vpa.yaml rather than duplicating pvc.yaml. Capture renders through rtk proxy — RTK truncates multi-document helm output even through a > redirect.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/charts/telark/templates/workloads/pvc.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/templates/_helpers.tpl
    - /Users/houssem/Desktop/Github/telark/charts/telark/templates/_mode.tpl
    - /Users/houssem/Desktop/Github/telark/charts/telark/values.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/values.schema.json
    - /Users/houssem/Desktop/Github/telark/charts/telark/modes/minimal.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/modes/performance.yaml
    - /Users/houssem/Desktop/Github/telark/charts/telark/VALUES.md
    - /Users/houssem/Desktop/Github/telark/charts/telark/README.md
    - /Users/houssem/Desktop/Github/telark/INSTALL.md
- **24. The UI can list, create and DOWNLOAD a report. The download carries its auth in the X-Session-Token header through the existing axios interceptor and materialises the file client-side from a blob. No token ever appears in a URL.**
  - _part_: B
  - _layer_: frontend / paths + API client + download
  - _testFile_: (no executable frontend test — see testFrameworkNotes; the gate is check-all plus an explicit forbidden-pattern grep)
  - _testNames_:
    - npm run check-all (tsc --noEmit && eslint .) at zero errors
    - forbidden-pattern gate: window.open == 0 and token-bearing <a href> == 0
    - createObjectURL appears exactly once, inside the download helper
  - _redCriterion_: Record the pre-change counts first — verified currently ZERO occurrences of createObjectURL, new Blob, a.download, file-saver and responseType anywhere in src. RED for this step is defined as the gate failing: any non-zero window.open or token-in-href count, or a check-all error count above zero. Run the grep BEFORE and AFTER; the before-run proves the counts are genuinely zero rather than the grep being broken.
  - _greenCriterion_: npm run check-all exits zero with zero errors (warnings acceptable, and NO eslint-disable added). window.open and token-bearing href counts remain zero. createObjectURL appears exactly once. request.ts is NOT widened — it returns response.data by contract and structurally cannot surface Content-Disposition; add a sibling download helper returning {blob, filename} instead.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/constants/rest/paths.ts
    - /Users/houssem/Desktop/dashboard-ui/src/api/client/download.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/reports.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/index.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/models/index.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts
- **25. A reports section on the plan details page lists reports, offers the format choice, triggers generation and downloads. State is a LOCAL hook, not a Redux slice — the over-engineering cut: usePlanViolations is a ~40-line useState/useEffect hook with a cancelled guard and a reloadKey, and reports have exactly the same shape and no cross-feature consumer.**
  - _part_: B
  - _layer_: frontend / hook + UI section
  - _testFile_: (no executable frontend test — the gate is check-all)
  - _testNames_:
    - npm run check-all at zero errors
    - usePlanReports mirrors the usePlanViolations cancelled-guard + reloadKey shape
  - _redCriterion_: n/a — no frontend runner exists. Treat the TypeScript compiler as the gate: the new section referencing an absent hook/model is a compile error, the closest available analogue to compile-time RED.
  - _greenCriterion_: npm run check-all zero errors. Constraints, all enforced: no `any` (use the real types — Rule from antd/es/form, Style<object>, etc.); no console.* (use the project logger, never relax the no-console rule); no hardcoded hex (map status to DEFAULT_COLORS.SUCCESS/DANGER/WARNING/ERROR); every user-facing string in constants/protectionPlans.ts, none inline; control sizing from controls.ts CONTROL_HEIGHT/RADIUS/FONT_SIZE — no size= props, no inline height, no nested ConfigProviders, no !important; no box-shadow on buttons; no internal vendor names in any string (say 'policy engine', never the vendor). For ANY colour or layout judgement, ASK THE USER FOR A SCREENSHOT in Desktop/claude/ — never guess at a visual bug, and never open a browser preview window.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanReports.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/details/ReportsSection.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts
- **26. Every touched module is lint-clean against its own checked-in config, every suite is green, the charts render and validate, and the docs reflect the change in the same edit.**
  - _part_: B
  - _layer_: final gates
  - _testFile_: (command gates)
  - _testNames_:
    - golangci-lint run in services/exporter
    - golangci-lint run in services/discovery
    - go test ./... -count=1 in exporter, discovery, internal/rest, internal/data
    - make helm-lint && make helm-validate
    - npm run check-all in dashboard-ui
  - _redCriterion_: n/a — gates.
  - _greenCriterion_: ALL of: golangci-lint issue count <= the recorded baseline for every touched Go module, using the PROJECT config only — never --no-config, never an override flag, no GOTOOLCHAIN prefix; every module's test set green (or set-equal to baseline where a package was already failing from the unfinished lint work); make helm-lint and make helm-validate clean; npm run check-all at ZERO errors. DOCS ARE PART OF DONE: INSTALL.md, the chart README and VALUES.md (regenerated with make values-docs) updated in the SAME change. OUT OF BOUNDS throughout: no service version bumps, no Chart.yaml bump, no image build/push/publish, no go.work edits, no internal/* pin bumps or module releases, no git commit/push/merge/branch-switch. No personal gmail in any shipped artifact — use contact@telark.io.
  - _implementationFiles_:


## TDD — characterisation tests

- services/exporter/internal/tests/snapshotapi/contract_test.go :: TestCreateSnapshotResponseIsByteStableGolden — byte-stable golden of the CreateSnapshot 200 body (volatile PVC fields normalised)
- services/exporter/internal/tests/snapshotapi/contract_test.go :: TestCreateSnapshotResponseCarriesThePathKeyDiscoveryReads — pins the raw "path" literal that discovery's clients/snapshots.go reads; a miss currently degrades SILENTLY via circuitbreaker.NotCounted
- services/exporter/internal/tests/snapshotapi/contract_test.go :: TestReadSnapshotInfosResponseKeySetIsStable — key set only; ReadSnapshot has no return value and GetStorageInfo does a live PVC lookup, so a value-level golden is unachievable
- services/exporter/internal/tests/snapshot/decode_test.go :: TestParseCreatePayloadAcceptsJSONFloat64Generations — the float64 decode class that once lost every scale-down
- services/exporter/internal/tests/snapshot/decode_test.go :: TestParseCreatePayloadRejectsNonIntegralGenerations — pins the n != float64(int64(n)) guard
- services/exporter/internal/tests/snapshot/decode_test.go :: TestParseCreateSnapshotRequestAcceptsADecreasedGeneration — a scale-DOWN must survive the decode
- services/exporter/internal/tests/snapshot/decode_test.go :: TestParseCreatePayloadAcceptsIntAndStringGenerationsIdentically — int / int64 / string branches agree with float64
- services/exporter/internal/tests/snapshot/retention_order_test.go :: TestRetentionKeepsHighestGenerationsNotLexicallyLatest — V10 outranks V2; lexical sorting would silently delete the newest
- services/exporter/internal/tests/snapshot/retention_order_test.go :: TestRetentionIgnoresMtimeOrder — ordering is by parsed generation, never by file mtime
- services/exporter/internal/tests/snapshot/retention_order_test.go :: TestRetentionLeavesNonGenerationFilesAlone — the V*.json glob plus parseGenerationFilename filter
- services/exporter/internal/tests/snapshot/retention_order_test.go :: TestRewritingAnExistingGenerationDoesNotDeleteOlderSurvivors — the force-sync-vs-flush-history clobber shape
- services/exporter/internal/tests/snapshot/gc_guards_test.go :: TestSnapshotGCNeverRemovesRootOrScopeDir
- services/exporter/internal/tests/snapshot/gc_guards_test.go :: TestSnapshotGCKeepsOrphansInsideTheGracePeriod — the age gate is what protects a file mid-write
- services/exporter/internal/tests/snapshot/gc_guards_test.go :: TestSnapshotGCToleratesReferencedPathsThatNoLongerExist
- services/exporter/internal/tests/snapshot/gc_guards_test.go :: TestSnapshotGCTreatsALeftoverTempFileAsSweepable — pins BOTH temp constants (SnapshotTempSuffix '.*.tmp' for writes vs SnapshotTempFileSuffix '.tmp' for the sweep) so a future unification cannot silently orphan files
- services/exporter/internal/tests/snapshot/fingerprint_test.go :: TestSanitizeManifestIsIdempotent
- services/exporter/internal/tests/snapshot/fingerprint_test.go :: TestSanitizedManifestSurvivesWriteAndLoadUnchanged
- services/exporter/internal/tests/snapshot/fingerprint_test.go :: TestSanitizeStripsTheSameFieldsFromLiveAndStoredObjects — manifestdiff cleans BOTH sides; the post-stamp reconcile rule depends on it
- services/exporter/internal/tests/snapshot/fingerprint_test.go :: TestFingerprintOfASanitizedManifestIsStableAcrossARoundTrip — sha256 over canonical JSON computed in the test; no production fingerprint function added
- services/exporter/internal/tests/snapshot/persist_failure_test.go :: TestWriteToUnwritableDirLeavesNoGenerationFile
- services/exporter/internal/tests/snapshot/persist_failure_test.go :: TestFailedWriteLeavesNoTempFileBehind — the failure-path mirror of the existing success-path TestWriteLeavesNoTempFileBehind
- services/exporter/internal/tests/snapshot/persist_failure_test.go :: TestWriteReturnsTheUnderlyingErrorRatherThanSwallowingIt

## TDD — test framework notes

READ, NOT ASSUMED. I read persist_test.go, gc_test.go, snapshotapi_test.go, handler_test.go, requirements_test.go, subject_routes_test.go, planviolations/violations_test.go, testutil/testutil.go, the Makefile, dashboard-ui/package.json and the vite config surface.

GO — exporter (services/exporter/internal/tests/<area>/):
- Pure stdlib `testing`. NO testify, NO gomega, NO assertion library anywhere.
- Test package name == the area directory name, with NO _test suffix (package snapshot, package snapshotapi). Production code is imported by real path and aliased: snaputil "github.com/telark/exporter/internal/utils/snapshot", expsnap ".../internal/exporters/snapshot", snaphandler ".../internal/handlers/snapshot".
- Fixtures: t.TempDir(), t.Setenv(constants.SnapshotsPathEnv, dir) followed by envs.InitSnapshotsPath() to re-read the env — that re-init call is REQUIRED, the env manager caches. Canonical helpers: setRoot(t) / setupSnapshotsDir(t).
- Helpers are plain funcs with t.Helper() as the first line, living in the SAME package: writeSnapshotFile, mkdirSnapshotDir, assertRemoved, assertKept, createOK, manifestBody, snapQuery, withID. Shared consts (testAppID, testNamespace, testGeneration, kindDeployment, dirPerm, filePerm) are declared once per package in a const block.
- Style: table-driven where there are cases, t.Errorf for continuable assertions and t.Fatalf for setup failures, and Go 1.22+ idioms already in use — `for range writers`, `wg.Go(func(){...})`, slices.Sort / slices.Equal.
- Constants-over-literals is enforced even in tests: constants.DefaultInitValue for 0, constants.DefaultIncrementValue for 1, constants.EmptyString for "".
- HTTP: net/http/httptest recorders, handlers invoked directly as snaphandler.CreateSnapshot()(rec, req), path vars injected with mux.SetURLVars from github.com/gorilla/mux. No live server.
- ALREADY-WIRED DENY-BY-DEFAULT GATE — the most useful discovery for Part B: internal/tests/authz/requirements_test.go contains TestRequirementsCoverEveryRoute and TestNoRequirementWithoutRoute. Registering a route without a requirement, or a requirement without a route, FAILS an existing test naming the exact key. Step 17 gets a genuine RED for free.
- Authz fixtures: a stubResolver implementing UserIDForToken, a sessionUsers map and a userGrants map of xauthz.Grants keyed by roledata scope — reuse these from subject_routes_test.go, do not invent new ones.

GO — discovery (services/discovery/internal/tests/<area>/): same stdlib conventions, one directory per area (37 of them: planviolations, planctl, planhandlers, planhealth, clients, manifestdiff, rollbackctl, ...), PLUS two shared helpers in internal/tests/testutil/testutil.go: a generic Equal[T comparable](t, name, got, want) and RedisEnv(t) which starts miniredis and points REDIS_HOST/REDIS_PORT at it. Kubernetes is faked with dynamicfake.NewSimpleDynamicClientWithCustomListKinds over a custom list-kind map, and Kyverno admission Events are hand-built as *unstructured.Unstructured with reason "PolicyViolation" / type "Warning". Use these; do not introduce envtest or a new fake.

GO — shared modules: internal/rest and internal/data BOTH follow the same tests/<area>/ layout (rest/tests/router/, rest/tests/clients/shared/, data/tests/policies/, data/tests/builtin/). They are NOT members of telark/go.work (which lists only the four services), so they are tested from their own directories. CRITICAL: go.work replaces github.com/telark/{data,rest,kcore,x-ware} with absolute paths into /Users/houssem/Desktop/Github/internal/*, so an edit there changes what all four services compile against IMMEDIATELY — always follow a rest/data edit with `cd /Users/houssem/Desktop/Github/telark && go build ./...` in the same step. CI runs GOWORK=off, so go.work is a dev-only convenience; never edit it.

RULE CONFIRMED: every *_test.go lives under services/<svc>/internal/tests/<area>/, never beside production code. Nothing in this plan requires a *_internal_test.go. Two unexported symbols matter — toPositiveInt (reachable through the exported ParseCreatePayload, so no exposure needed) and resolvePlanNamespaces (step 21 adds an exported Capture wrapper instead of a peer test file).

CHART: no helm-unittest, no chart tests directory. Verification is `make helm-lint` (helm lint on both charts), `make helm-template` and `make helm-validate` (kubeconform against every entry in K8S_VERSIONS, with --set app.persistence.storageClass=validate per mode). Assertions are shell over the rendered YAML. values.schema.json has additionalProperties:false on app.persistence, which turns a missing schema entry into an immediate helm lint failure — a usable RED. accessMode is deliberately absent from the schema because _mode.tpl computes it post-validation; that is intentional, not a bug. RTK truncates multi-document helm output EVEN THROUGH a > redirect; always capture renders with rtk proxy "helm template ...".

FRONTEND — THE IMPORTANT FINDING: dashboard-ui HAS NO TEST RUNNER AT ALL.
- package.json scripts: dev, generate:licenses, build, build:analyze, build:cluster, serve, lint, lint:f, format, type-check, check-all, check-all-and-build. There is NO `test` script.
- devDependencies contain vite 8, eslint 9, prettier, typescript 5.9.3 and typescript-eslint — NO vitest, NO jest, NO playwright, no test config in the vite setup. There is no vitest.config.* or jest.config.* file.
- @testing-library/react, @testing-library/jest-dom, @testing-library/user-event and @types/jest ARE present but sit in `dependencies`, and with no runner they cannot execute. Their presence is misleading — do not read it as "tests exist".
- ZERO *.test.* and ZERO *.spec.* files under src. (Only k6 load scripts exist elsewhere, which are not unit tests.)
- THEREFORE: how frontend tests are run — they are not. The only gate is `npm run check-all` = `tsc --noEmit && eslint .`, which must finish at zero errors (warnings acceptable, no eslint-disable as a workaround).
- CONSEQUENCE FOR THIS PLAN: frontend steps 24 and 25 cannot do real red-green. I deliberately did NOT add vitest — that is a new dependency and a decision for the user, not something to smuggle in under a TDD plan. Instead the frontend contract is pinned where it CAN be executed: step 1's byte-stable Go golden and step 16's endpoint-shape test pin the exact JSON and route shape the UI consumes, so a backend break is caught by a Go test rather than by a human clicking. The UI itself is gated by tsc + eslint plus an explicit forbidden-pattern grep (window.open, token-in-href, createObjectURL count). If the user WANTS genuine frontend RED/GREEN, adding vitest + jsdom is a single self-contained step — flag it and let them choose; do not assume it.
- Verified current state of the download surface: ZERO occurrences of createObjectURL, new Blob, a.download, file-saver or responseType anywhere in src, and no export/PDF dependency in package.json. The before-counts are genuinely zero, which is what makes the step-24 grep gate meaningful.
- src/api/client/request.ts returns response.data and nothing else, so it structurally CANNOT surface a Content-Disposition header. Do not widen it; add a sibling download helper.
- src/features/plans/protection/hooks/usePlanViolations.ts is the template for usePlanReports: useState + useEffect with a `cancelled` guard and a reloadKey, returning {data, loading, error, filter, setFilter, refresh}. No Redux slice.

## TDD — coverage targets

Coverage is measured per Go module with `go test ./... -count=1 -coverprofile=...` and judged as a DELTA against the recorded baseline, not against an absolute 80% figure — several exporter and discovery packages are thin wiring where a percentage target would only invite padding tests.

Binding targets:
1. services/exporter/internal/utils/snapshot — coverage must NOT DROP after Part A. This is the whole point of the refactor gate: the existing test funcs plus the 22 new characterisation tests must cover at least as many statements after the extraction as before. Measure before step 8 and after step 11 and compare the two numbers directly.
2. services/exporter/internal/utils/fileio (new) — every exported function covered INCLUDING the failure branches (unwritable dir, rename failure, traversal reject, base-itself accept, prefix-sibling reject). This package is two functions; anything below full exported coverage means a branch was moved without its test.
3. services/exporter/internal/report (new) — full RED/GREEN discipline applies, so every branch arrives with a test. Specifically required: each supported format (loop over the exported format list), the unsupported-format reject, the empty-activity narration, the retrospective non-empty case, the render-failure path, the unwritable-root path and the zero-byte guard.
4. services/exporter/internal/authz/requirements.go — coverage here is not a percentage, it is the two existing structural tests. Both new routes must appear in TestRequirementsCoverEveryRoute and TestNoRequirementWithoutRoute, plus the five explicit refusal tests in step 18.
5. services/discovery — the capture sidecar and the dispatch path: both destruction paths (cancel, terminate), the ordering-before-cleanup assertion, the empty-violations case and the failure-does-not-block case.
6. internal/rest and internal/data — new packages only; existing coverage must not regress.
7. Charts and dashboard-ui — no coverage metric exists. Chart coverage is "all three modes rendered and kubeconform-validated"; UI coverage is "check-all at zero errors plus the forbidden-pattern grep".

Explicitly NOT a target: raising coverage on the pre-existing exporter or discovery packages that the unfinished lint work is currently touching. Leave them exactly as the baseline found them.

## TDD — execution notes

EXACT COMMANDS. $SP = the session scratchpad. $T = /Users/houssem/Desktop/Github/telark. $I = /Users/houssem/Desktop/Github/internal. $UI = /Users/houssem/Desktop/dashboard-ui.

Per-step single-test command (steps 1-15, 17-22):
  Exporter:  cd $T/services/exporter && go test ./internal/tests/<area>/ -run '<TestName>' -count=1 -v
  Discovery: cd $T/services/discovery && go test ./internal/tests/<area>/ -run '<TestName>' -count=1 -v
  rest:      cd $I/rest && go test ./tests/router/ -run '<TestName>' -count=1 -v
  data:      cd $I/data && go test ./tests/policies/ -run '<TestName>' -count=1 -v
`-count=1` is MANDATORY on every run — the Go test cache will happily replay a stale PASS and destroy the RED gate.

Per-step suite command (the GREEN gate for that area): cd $T/services/<svc> && go test ./internal/tests/<area>/ -count=1
Per-step module command (the regression gate):        cd $T/services/<svc> && go build ./... && go test ./... -count=1

Step-by-step targets:
  1,2   cd $T/services/exporter && go test ./internal/tests/snapshotapi/ -count=1 -v
  3-7   cd $T/services/exporter && go test ./internal/tests/snapshot/ -count=1 -v
  8     cd $T/services/exporter && go test ./internal/tests/fileio/ ./internal/tests/snapshot/ ./internal/tests/snapshotapi/ -count=1
        cd $T && git diff --stat -- services/exporter/internal/utils/snapshot/
        cd $T && for f in file.go storage.go request.go retention.go gc.go resolve.go manifest.go sanitize.go payload.go types.go; do git diff --quiet -- services/exporter/internal/utils/snapshot/$f || echo "PERIMETER BREACH: $f"; done
  9,10  cd $T/services/exporter && go test ./internal/tests/snapshot/ -count=1
  11    cd $T/services/exporter && go test ./... -count=1 2>&1 | grep -E '^(ok|FAIL|---)' | sort | diff - $SP/baseline.exporter.set
        cd $T/services/exporter && golangci-lint run 2>&1 | grep -cE '^\S+\.go:[0-9]+'    # compare with $SP/baseline.exporter.lintcount
        shasum $T/services/*/go.mod $T/services/*/go.sum $I/*/go.mod $I/*/go.sum | diff - $SP/dep.sha
  12-15 cd $T/services/exporter && go test ./internal/tests/report/ -count=1 -v
  16    cd $I/rest && go test ./tests/router/ -count=1 -v && cd $T && go build ./...
  17    cd $T/services/exporter && go test ./internal/tests/reportapi/ ./internal/tests/authz/ -count=1 -v
  18    cd $T/services/exporter && go test ./internal/tests/authz/ -count=1 -v
  19    cd $I/data && go test ./tests/... -count=1 -v && cd $T && go build ./...
  20    cd $I/rest && go build ./... && cd $T && go build ./... && cd $T/services/discovery && go test ./internal/tests/clients/ -count=1 -v
  21    cd $T/services/discovery && go test ./internal/tests/planviolations/ -count=1 -v
  22    cd $T/services/discovery && go test ./internal/tests/planctl/ -count=1 -v
  23    cd $T && make helm-lint
        for m in minimal standard performance; do rtk proxy "helm template t $T/charts/telark --set app.mode=$m --set app.persistence.storageClass=validate" > $SP/after.chart.$m.yaml; done
        grep -c 'kind: PersistentVolumeClaim' $SP/after.chart.$m.yaml            # expect 2
        grep -E 'name: .*-exporter-(snapshots|reports)-pvc' $SP/after.chart.$m.yaml
        awk '/kind: PersistentVolumeClaim/,/^---/' $SP/after.chart.$m.yaml | grep -A40 snapshots-pvc | diff - $SP/baseline.pvc.$m.yaml     # expect empty
        rtk proxy "helm template t $T/charts/telark --set app.mode=performance --set app.singleNode=true" | grep -A8 'kind: PersistentVolumeClaim' | grep -A1 accessModes   # expect ReadWriteOnce for BOTH
        cd $T && make helm-validate && make values-docs
  24,25 cd $UI && npm run check-all
        cd $UI && grep -rn 'window.open' src | wc -l                              # expect 0
        cd $UI && grep -rEn '<a [^>]*href=[^>]*([tT])oken' src | wc -l            # expect 0
        cd $UI && grep -rn 'createObjectURL' src | wc -l                          # expect exactly 1, in the download helper
  26    cd $T/services/exporter && golangci-lint run
        cd $T/services/discovery && golangci-lint run
        cd $I/rest && golangci-lint run ; cd $I/data && golangci-lint run
        cd $T && make helm-lint && make helm-validate
        cd $UI && npm run check-all

LINT RULES: run golangci-lint with the project's checked-in config only. NEVER --no-config, never an override flag, and NO GOTOOLCHAIN prefix (Go 1.27.1 + golangci-lint 2.13.2 since 2026-09-19). The exporter/discovery configs share $T/.golangci.yml, which is itself staged-modified — that is precisely why the baseline must be taken in the working tree.

CONCURRENCY HAZARD: a dependency-upgrade workflow is mutating go.mod/go.sum while this plan runs. Before EVERY Part A green gate, re-run the shasum diff from B0. If it moved, re-baseline; do not attribute the delta to the refactor. Do not touch go.mod, go.sum or go.work yourself.

RED-GATE DISCIPLINE: a test that was written but not compiled and executed does NOT count as RED. Every step above states either a runtime RED (throwaway production probe — assert the failure, then REVERT the probe immediately) or a compile-time RED (the new test references a package/symbol that does not exist yet, and the build failure IS the signal). Never edit production code before the RED is observed. Confirm each probe reverted with `git diff --quiet -- <probed file>` before moving on.

NO TEST MAY BE REWRITTEN TO ACCOMMODATE THE NEW SHAPE. Part A's contract is that not one line of an existing test changes. If a refactor forces a test edit, the refactor is wrong — add a wrapper instead. That is exactly why steps 8/9/10 keep delegating wrappers in package snapshot rather than relocating call sites.

GIT: commit/push/merge/branch-switch are OUT OF BOUNDS. At each RED and GREEN, print the suggested checkpoint message (`test: add reproducer for <X>` / `fix: <X>`) and let the USER decide. Do not stash, do not create a worktree, do not finish or revert the 106 staged files.

EVIDENCE REPORT: the methodology asks for a TDD evidence report with a guarantees table. Do NOT write it as a standalone .md unless the user asks — return the RED/GREEN evidence per step in the execution response instead.

KNOWN-FACT CORRECTIONS folded into the sequence above so the executor does not rediscover them: (a) /Users/houssem/Desktop/Github/go.work does not exist — the file is telark/go.work and its replaces + go 1.27.1 are STAGED-ONLY, which is why the clean-worktree baseline was dropped entirely; (b) request.go, storage.go and file.go call IsWithinBase / ValidateSnapshotIdentity / FormatBytes UNQUALIFIED intra-package, so those three symbols deliberately DO NOT MOVE — only WriteSnapshotJSON and IsWithinBase have a proven second caller, and IsWithinBase keeps a wrapper; (c) ApplyRetentionPolicy already takes snapshotsPath as a parameter and needs NO refactor, only characterisation; (d) payload.go is inside the zero-diff perimeter because it holds coerceInt's float64 branch; (e) grep -c Action builtin.go == 0, so the builtin.go edit is deleted from the plan; (f) the discovery client wrapper (step 20) precedes the lifecycle hook (steps 21-22), fixing the circular dependency; (g) the activity sidecar is widened to {violations, renderedPolicies, health, healthReason, capturedAt} because violations.Collect returns violations only while both destruction patches wipe the policy set AND the health; (h) svc.go and violations.go are staged-modified, so every edit site must be located by symbol name, never by a quoted line number; (i) the read-side golden is narrowed to a key set because ReadSnapshot returns nothing and GetStorageInfo does a live PVC lookup; (j) the frontend uses a local hook, not a Redux slice.

## Critique verdicts

- **item**
  - _lens_: Snapshot regression and completeness — Part A (behaviour-preserving refactor on the rollback path) as highest priority, plus layers with no step
  - _planIsSound_: no
  - _verdict_: The plan is unusually well-grounded — I checked roughly forty of its factual assertions against the real files and almost all of them are exact (see the "what holds" note below). But Part A as written cannot be executed: the refactor breaks a file the plan's own gate forbids touching, and the gate's grep is blind to the break. Two more structural defects sit in the highest-risk and most-security-sensitive areas: step 13 has a circular dependency and no actual persistence mechanism, and a whole shared-module layer (internal/rest/clients/reports) has no step. Two report sections name a data source that the plan's own capture step does not produce, which reintroduces the exact "looks complete, silently isn't" failure D2 exists to prevent. Baseline (a) is unsound because the go.work the plan relies on does not exist where it says and carries no internal/* replaces at HEAD.

WHAT HOLDS (verified, not flattery — this matters for what to keep): the five-symbol census is exactly right (four production call sites, def.go:40/151/213/539); the raw "path" literal at exporters/snapshot/def.go:136 vs constants.SnapshotPathKey + circuitbreaker.NotCounted at discovery clients/snapshots.go is real and step 2 is the single best file in Part A; values.schema.json app.persistence is genuinely {additionalProperties:false, properties:{enabled,storageClass,size}} so the schema-gate call is correct; _mode.tpl really does derive accessMode/strategy from replicas with singleNode forcing 1, so "no _mode.tpl edit" is right; telark.exporterSnapshotsPvcName really is the single source for the claim, values.yaml:227 claimName and values.yaml:216 SNAPSHOTS_PVC_NAME, and refusing to parameterise it is the correct call; every svc.go and violations.go line number quoted is exact; SnapshotTempSuffix ".*.tmp" vs SnapshotTempFileSuffix ".tmp" both exist and the warning is well-founded; there is genuinely no blob/download precedent in dashboard-ui (grep for createObjectURL/responseType returns nothing) and request.ts:14 does return response.data only, so the dedicated header-authenticated download.ts is the right answer to the prior token-in-URL incident; ProtectionPlanViolation really has no actor field. On the specific regression sites I was told to hunt: the census proves none of the five moved symbols is reachable from the fingerprint/post-stamp reconcile path, the force-sync/flush-history path, or GC, and gc.go/retention.go/resolve.go are genuinely left alone — Part A does not silently undo any known past snapshot fix. The damage is in executability and completeness, not in snapshot semantics.
- **item**
  - _lens_: over-engineering and scope discipline (bias: deletion)
  - _planIsSound_: no
  - _verdict_: The plan is NOT too large for the request — the request genuinely is large (nine-section document, four formats, a second PVC, new routes at three layers, a UI). Several of its most-attacked-looking decisions are correct and I endorse them explicitly: (1) The PDF decision is right. Four stdlib renderers, zero new Go dependencies, print-optimised HTML with @media print — go-pdf/fpdf shares nothing with html/template, so a native PDF renderer is a second full renderer maintained forever and every future section written twice. Do not add it. (2) Eager-render-all-formats is a real deletion, not a gold-plate: it removes the format-picker modal, the generation-options step and all job-progress UI. (3) Synchronous generation is the correct position. One CR get, one namespace resolve, one Event LIST, four in-memory template renders — that is a sub-second request; async job machinery (queue, status polling, progress UI) would be pure ceremony. (4) Declining to generalise retention.go and gc.go is correct and I verified why: ApplyRetentionPolicy is welded to BuildSnapshotDir, a literal "V*.json" glob and parseGenerationFilename, so reusing it means widening the codec that the hourly sweeper's delete predicate reads. A 15-line Prune is the cheaper trade. (5) The chart work is already lean — no _mode.tpl edit, no _deployment.tpl edit, no reports.enabled gate, no separate storageClass, and a plain sibling helper instead of parameterising telark.exporterSnapshotsPvcName. (6) Step 2's golden-file test is earned: def.go:136 builds a raw "path" key that clients/snapshots.go:122 reads back, and a miss degrades silently through circuitbreaker.NotCounted. Where the plan does carry unearned complexity, it is concentrated in three places: Part A moves five functions when only two have a proven second caller; the reports env/route surface ships knobs and a route with no consumer; and the UI adds a five-file Redux layer where the sibling panel uses a forty-line local hook. Two high findings stand, so planIsSound=false — but both are deletions, not redesigns: the plan gets smaller, not different.
