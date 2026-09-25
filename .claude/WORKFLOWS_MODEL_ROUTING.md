# Workflow Model Routing

**Status**: Active. **Budget mode ON since 2026-09-23** (Fable 5.1 quota at 63% with the reset on Tuesday 2026-09-29): every rule below is written for budget mode. When the quota resets, only the "Budget mode" section changes; the effort floors stay.
**Scope**: Every subagent spawned by a Workflow script or Agent call in this repository.
**Applies to**: `agent()` calls in workflow scripts (`model` option), `meta.phases[].model`, and Agent-tool `model` overrides.
**Owner**: telark maintainers. Edit by changing a rule or a Major criterion below; do not add rules for cases these do not cover.

**Principle**: Opus 5.5 does the work. Fable 5.1 is reserved for the single judgment call per Major change that
everything else depends on: the final plan/design synthesis. Opus never runs below `high`; anything that is hard,
Major, or exists to catch a mistake runs at `xhigh`.

---

## Rules

Evaluate top to bottom. **The first rule that matches wins.**

| Priority | Condition | Model | Effort |
|---|---|---|---|
| 1 | Task is tagged `plan.apply.architecture` (maintainer-forced) | `claude-fable-5-1` | model default |
| 2 | Task **writes the final plan or design synthesis** of a Major change (one agent per plan: the synthesizer) | `claude-fable-5-1` | model default |
| 3 | Task **implements** a Major change, **designs an angle**, **revises** a plan, or **decides** anything Major other than the synthesis | `claude-opus-5-5` | `xhigh` |
| 4 | Everything else: read-only work and non-Major implementation | `claude-opus-5-5` | see [Opus effort](#opus-effort) |

Every `agent()` call sets `model` explicitly. **Nothing inherits the session model.**
Every Opus call also sets `effort` explicitly. **Nothing inherits the session effort.**

Model IDs are the bare form the Workflow tool's `model` option accepts. The `[1m]` suffix in
`~/.claude/settings.json` is a context-window selector for the main session, not part of the model ID.

---

## Definitions

### Major change (Priorities 2 and 3)

A task is Major if its **final objective** meets at least one of these criteria:

| ID | Criterion | Covers |
|---|---|---|
| M1 | **Contract** | Changes an interface consumed outside the edited module: API / gRPC / proto, CRD schema, Helm values schema, event or message format, exported API of a shared package (rest-pkg, data-pkg, …) |
| M2 | **Persisted data** | Changes what is stored, its shape, or where it lives (etcd keys, DB schema, object-storage layout), or requires a migration |
| M3 | **Security boundary** | Touches authn, authz, token or secret handling, or a policy evaluation / enforcement path |
| M4 | **Multi-deployable behaviour** | Changes the runtime behaviour of two or more separately deployed components in one task |
| M5 | **New shared pattern** | Introduces a pattern, package or abstraction that other components are expected to adopt |

**Not Major on its own**: diff size, file count, number of services touched without a contract or
behaviour change, or "it feels important".

### Classify by the final objective, not the first step

Nearly every implementation task starts with analysis. That does not demote it.
The test is one question: **when this task is done, does any of M1–M5 hold?**
If yes → Priority 2 (synthesis) or 3 (everything else Major). If no → Priority 4.

---

## Opus effort

Two levels only. **`high` is the floor**: no Opus agent in a workflow runs at `low` or `medium`.
`max` is not used in workflows.

| Effort | Use when the task… | Examples |
|---|---|---|
| `xhigh` | …is Major (P3), **exists to catch a mistake**, **decides a route**, or is a **hard problem** (concurrency, non-reproducible bug, plan synthesis fallback, anything a `high` run already failed once) | Implementing a Major step; designing a plan angle; revising a plan; judges; adversarial verification and fix-verifiers; PR review of a Major change; scouts classifying M1–M5; root-causing flaky or concurrency bugs |
| `high` | …anything else | Terrain maps and inventories; extraction into a `schema`; single-service non-Major fixes; tests; docs; lint-fix / build-fix loops; dependency bumps; cleanup passes; non-Major reviews and gates |

Rules:

- **`xhigh` is checked first.** A mechanical-looking review is still a review.
- **A `high` failure escalates once to `xhigh`** with a sharper prompt. If `xhigh` fails again, the task is
  either misclassified (re-check M1–M5) or too large (split it). Only then, and only for a P2/P3 task, may
  it go to Fable, with the reason in the label.
- **Keep the tiers few.** Workflow agents share prompt cache only when model *and* effort match. Two Opus
  tiers plus one Fable tier is the whole set.

### Tuning

After a week of runs, compare the `/workflows` token totals per label against the outcomes. Demote a shape
from `xhigh` to `high` when its results are never corrected downstream; promote it when a verifier keeps
refuting its output. Change this table, not individual calls.

---

## Budget mode (until the Fable quota resets on 2026-09-29)

- Fable appears **at most once per plan-producing workflow** (the synthesizer) and **never** in apply,
  cleanup, dependency or verification workflows.
- If the Fable quota is exhausted mid-run, the synthesizer also moves to Opus `xhigh` and the plan gets
  one extra fix-verifier round.
- After the reset, the only intended change is to allow Fable again for P3 implementation steps carrying
  M2 or M3 (persisted data, security boundary). Everything else in this document stays.

---

## Why

**Opus 5.5 does the work.** Opus 5.5 at `high`/`xhigh` handles long agentic implementation and review
runs; Fable is the model to reach for when Opus at `xhigh` still falls short, not the default for a class
of tasks. Opus 5.5 lists at $4 / $20 per million input / output tokens versus $10 / $50 for Fable 5.1, and
Fable is on a separate, smaller quota.

**Fable synthesizes, Opus verifies.** The one judgment that is expensive to get wrong is the plan itself,
so the synthesizer stays on Fable. Everything around it (terrain, design angles, judge, critics, revision,
apply, review) is either checked by that synthesis or checks it, and Opus `xhigh` is enough there.

**Effort floor is `high`.** Measured on 2026-09-23: the `low`/`medium` tiers produced the results that the
verifiers refuted most often, and the re-runs cost more than running `high` once. Effort applies to every
output token, so `high` is the cheapest reliable setting, and `xhigh` is reserved for the tasks where a
miss forces a redo.

**Precedence is explicit** so classification is deterministic and reviewable. The tag (P1) lets a
maintainer force Fable without arguing definitions.

---

## Ambiguity

Ambiguity is only ever about one question: does an M-criterion hold?

1. Decide from the task description when it settles the question.
2. When it does not, run a **read-only scout on Opus `xhigh`** first. Its prompt asks only which of M1–M5 the
   change would touch, with file-level evidence. Route the implementation on its answer.
3. If the scout reports a criterion, or cannot rule one out → P3 (Opus `xhigh`). Otherwise → P4 (`high`).

Record the route in the agent's `label` (e.g. `'apply:auth-routes [P3:M3 xhigh]'`, `'fix:flaky-test [P4 high]'`).

| Task shape | Resolution |
|---|---|
| "Investigate X and fix it if it's simple" | P4 `high`, unless the scout reports M1–M5 |
| "Write a plan for X" | terrain/design/judge/critics on Opus; the synthesizer on Fable if X is Major, else Opus `xhigh` |
| "Review this PR and post comments" | P4 `high`; `xhigh` if the PR is Major |
| Cross-service refactor, zero behaviour change, no contract change | P4 `high` |
| Same refactor, but renames an exported symbol of a shared package | P3 (M1) → Opus `xhigh` |
| Dependency version bump (`go.mod`) plus resulting fixes | P4 `high`; the breaking-change scout `xhigh` |
| Adversarial review of a Major implementation | Opus `xhigh` |

---

## Workflow shape (token budget)

Routing picks the model per agent; the **shape** decides how many agents run and how much each re-reads.
Measured on 2026-09-23: the 5-phase re-plan shape cost ~3.7M subagent tokens and ~2.5 h per feature. The
shape below produced the same plan quality at roughly half the cost, and in budget mode it puts a single
agent on Fable.

| Phase | Agents | Route | Discipline |
|---|---|---|---|
| Terrain | 4 read-only maps, one per subsystem | Opus `high` | Structured output only (schema). Cite file:line. No prose. |
| Design | **2** independent angles (minimal, risk-first); UX folded into both | Opus `xhigh` | **Trust terrain.** Open code only to verify ≤8 load-bearing claims. |
| Judge | **1** judge, all lenses in one rubric | Opus `xhigh` | Verify ≤6 claims per design, prefer the ones the designs disagree on. |
| Synthesize | 1 | **Fable** (P2) | Writes the plan `.md` + `.json`; every step carries its route. |
| Critique r1 | 2 critics (completeness, over-engineering) | Opus `xhigh` | Calibrated severity (below). One finding per defect, evidence as file:line. |
| Revise | 1 | Opus `xhigh` | **Targeted `Edit` calls only.** Never rewrite a plan file in full. |
| Critique r2 | 1 fix-verifier | Opus `xhigh` | Scoped to the blocking list + new criticals. |
| Revise (optional) | 1 | Opus `xhigh` | Only if r2 leaves blocking items. Then stop; remaining items are listed, not looped. |

Ceiling: 13 agents, 2 critique rounds, 1 Fable agent. Terrain and design agents never re-verify what a
structured upstream result already states; only judges and critics verify, and they are told how many claims.

**Severity calibration for every critic prompt.** `critical` = would ship a security / data-loss /
undefined-lifecycle defect, or the plan cannot be executed. `high` = an implementer following the step
text verbatim produces a defect that forces the step to be redone. Wording, citations, naming, test
placement and style are `medium` or `low`, however strongly felt. Only `critical`/`high` trigger a
revision.

**Apply workflows** (implementing an approved plan): one agent per plan step, all on Opus (`xhigh` for
P3 steps, `high` for P4 steps), backend and UI lanes in parallel, deterministic topological levels with
`parallel()` so `resumeFromRunId` can replay. A plan step whose `route.model` says Fable is applied on
Opus `xhigh` in budget mode; the step's `route` field is left as written for after the reset.

**Cleanup / dependency / gate workflows**: Opus only. Cleanup agents and bump agents `high`; the
cross-service reviewer, breaking-change scouts and every verifier `xhigh`; the final lint+test gate `high`.

**Do not** put the terrain JSON into judge or critic prompts; they read the plan files and the code.
**Do not** ask any agent to "ground every claim in code you open yourself"; that re-runs the terrain phase.

---

## Examples

| Task | Classification | Model | Effort |
|---|---|---|---|
| Write the final plan for moving session tokens out of etcd | P2 (M2, M3) | `claude-fable-5-1` | default |
| Design the risk-first angle for that plan | P3 | `claude-opus-5-5` | `xhigh` |
| Apply the approved plan to move session tokens out of etcd | P3 (M2, M3) | `claude-opus-5-5` | `xhigh` |
| Add durable violation capture in discovery and persist it via the exporter | P3 (M2, M4) | `claude-opus-5-5` | `xhigh` |
| Scout: which of M1–M5 would fixing the flaky exporter test touch? | P4 | `claude-opus-5-5` | `xhigh` |
| Adversarially verify a dependency-bump agent's lint counts | P4 | `claude-opus-5-5` | `xhigh` |
| Fix a nil-pointer panic in a single service's handler | P4 | `claude-opus-5-5` | `high` |
| Add unit tests for the exporter | P4 | `claude-opus-5-5` | `high` |
| Map how a Category is implemented end to end | P4 | `claude-opus-5-5` | `high` |
| Bump `golangci-lint` and fix the new findings until clean | P4 | `claude-opus-5-5` | `high` |
| Cleanup pass over one service (no behaviour change) | P4 | `claude-opus-5-5` | `high` |

---

## How to apply in a workflow script

```js
const OPUS = 'claude-opus-5-5'
const FABLE = 'claude-fable-5-1'

// P4 / high — the floor: inventories, tests, docs, non-Major fixes, cleanup, gates
await agent(inventoryPrompt, { label: 'inventory:charts [P4 high]', model: OPUS, effort: 'high', schema: chartsSchema })

// P3 or verification / xhigh — Major implementation, design angles, revisions, judges, critics, scouts
await agent(applyPrompt, { label: 'apply:auth-routes [P3:M3 xhigh]', model: OPUS, effort: 'xhigh' })
await agent(verifyPrompt, { label: 'verify:lint-counts [P4 xhigh]', model: OPUS, effort: 'xhigh' })

// P2 — the one Fable agent per plan
await agent(synthPrompt, { label: 'synthesize:plan [P2:M2,M3]', model: FABLE })
```
