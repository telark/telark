# Protection plans

How protection plans are implemented, for contributors and reviewers; the user-level explanation is in [Concepts](../concepts.md#protection-plans). discovery owns the lifecycle and the Kyverno policies; the exporter stores the `ProtectionPlan` CR and the reports. Paths starting `data/` are in the shared package `internal/data/`; `discovery/` is `services/discovery/internal/`.

## The resource

`ProtectionPlan` (`protectionplans.telark.io`, `v1alpha1`, namespaced, short name `tplan`; Go type in `data/plans/protectionplan.go`, schema in `charts/telark-crds/templates/crds/protectionplans.yaml`). The plan's name is its `metadata.name`; the REST view returns it as `id`, and flattens `.status` into the top level.

- `scope`: `applications` (`applicationRefs`, application names) or `namespaces`, with optional `exclusions` (kinds for both scope types, named resources for the applications scope only).
- `policies`: template ids and params; the nine [templates](../concepts.md#what-a-plan-contains) are in `data/plans/templates.go`.
- `mode`: `audit` or `enforce`. `timeMode`: `permanent` or `time_range` with `timeRange`.
- `approvalMode`: `automatic` or `required`; `approval` (state, requester, decider, comment, history), written only by discovery.
- `environmentRef`, `tagRefs` (category ids) and `participantRefs`.
- Status subresource: `phase`, `reason`, `conditions` (`Ready`, `Approved`, `PoliciesHealthy`), `observedGeneration`, `renderedPolicies`, `health`, `healthDetail`, start and termination stamps, and `approval`.

## Lifecycle

Phases: `draft`, `pending_approval`, `scheduled`, `active`, `terminated`, `canceled`, `failed` (`data/plans/protectionplan.go`). No code path sets `draft`.

| From | To | Trigger | Code |
|---|---|---|---|
| (create) | `pending_approval` | `approvalMode: required` | `InitialPhase`, `discovery/core/plans/protection/approval.go` |
| (create) | `scheduled` | time range starting in the future | same |
| (create) | `active` | otherwise; policies deploy immediately | same |
| `pending_approval` | `active` or `scheduled` | approve (`Decide`); refused once the window has ended | `discovery/core/plans/protection/svc.go` |
| `pending_approval` | `canceled` | reject | same |
| `scheduled` | `active` | window start reached (plan controller) | `discovery/controllers/plans/protection/controller.go`, `Activate` |
| `active`, `pending_approval` | `terminated` | window end reached | controller, `Terminate` |
| `active`, `scheduled`, `pending_approval`, `failed` | `canceled` | user cancel | `Cancel` (`cancellable`) |
| `canceled`, `terminated`, `failed` | re-enters the lifecycle | user reactivate | `Reactivate` (`reactivatable`) |
| `scheduled` | `failed` | activation fails (scope resolution, render or deploy) | `Activate` → `markFailedRemote` |

- **Approval mode** is derived server-side (`ResolveApprovalMode`): the Production environment category is always `required`. Elsewhere the default is `automatic`, a client-sent `required` is honored, a client-sent `automatic` counts only from an Owner on `protection-plans`, and a duplicate of a `required` plan stays `required` unless an Owner relaxes it. `environmentRef` must name a `plan-environments` category (unknown: 400; catalog unreadable: 503), and an edit may not move an `automatic` plan into Production. Nobody who put the current spec up for approval since the last approval may decide it: the creator, a reactivator and every material editor are recorded as `requested` events in the approval history (`ApprovalRequesters`, `RecordEditor`), and a decision must name the pending request it answers (`ValidateDecision` in `approval.go`).
- **Per-plan lock** `lock:plan-decision:<id>` serializes decide, update, cancel, reactivate and clear. It and the name lock are held with a heartbeat (`PlanLockTTL`, twice the deploy budget, extended every third of it), so a slow exporter cannot let the lock expire under its holder.
- **Controller**: runs on the discovery leader, every `PROTECTION_PLAN_TICK_INTERVAL_SEC` (default 31 s) plus a timer armed on the next window edge.
- **Names** are unique, case- and whitespace-insensitively, under a Redis lock `lock:plan-name:<name>` (409 on conflict).
- Validation rejects windows already over, targets in the release namespace or in `TelarkConfig.excludedNamespaces` (fail closed: while the excluded list has never loaded, a namespaces-scope create, edit or reactivation answers 503), duplicate templates, unknown params, and `{{` or `}}` in the name or description. `enforce` mode on a `namespaces` scope needs Owner on `protection-plans` (403 otherwise); a Contributor may create audit-mode namespace plans and enforce-mode application plans ([discovery README](../../services/discovery/README.md#responsibilities), Protect).

## Admission policies

- `Render` (`data/policies/renderer.go`) produces namespaced Kyverno `Policy` objects (never `ClusterPolicy`: a namespaced policy can't reference cluster-scoped kinds) per scope namespace and template, with the plan's exclusions applied.
- Name `telark-<plan id>-<template code>-<8 hex of sha256(namespace + application ids)>`, with the [plan labels and annotations](../CRDS.md#labels-and-finalizers) (`data/policies/shared.go`).
- `validationFailureAction` follows the plan `mode`; audit messages read "would be blocked". Kyverno substitutes `{{ }}` variables in `validate.message`, so the message names the generated plan id only; the user-chosen name appears only in the `plan-name` annotation, which is not substituted.
- Applied with server-side apply, field manager `telark-protection-plans`, forced (`discovery/core/plans/protection/policies/applier.go`); mode changes are merge patches; deletion selects by the plan label (`CleanupByPlanID`).
- The bundled Kyverno runs with `forceFailurePolicyIgnore`, so when its webhook is down, requests are admitted even for enforcing plans ([security](../security/README.md#kubernetes-privileges)).

## Health and violations

- On every tick the health pass lists the managed policies once and compares each with a fresh render (`discovery/core/plans/protection/health/`). A not-ready policy makes the plan `degraded`, which wins over drift. `drifted` covers an active plan with no rendered policies, a missing policy, a `validationFailureAction` that doesn't match the plan mode, a stale policy (a different `render-hash`, or live `spec.rules` that differ from the fresh render), a live policy the plan doesn't list and no longer renders, and one it renders but hasn't listed yet. Rules compare with `equality.Semantic.DeepEqual` after dropping the two rule flags the policy engine defaults to `true` (`skipBackgroundRequests`, `allowExistingViolations`), so those defaults are not drift, and a hand edit that keeps the `render-hash` annotation still reads `drifted`. Drifted policies are redeployed, only by this pass (and once after a deploy). The status route (`GET protectionplans/{id}/status`, Read) only computes: it neither repairs nor persists health. Its `drift` object lists policy names under `missing`, `unexpected`, `mismatched`, `stale` and `added` (each omitted when empty).
- The same pass sweeps orphans: a managed policy whose plan is gone or not `active`, and older than `OrphanGracePeriod` (four deploy budgets, so a create or approval still in flight is left alone), is deleted.
- Violations are Kubernetes Events with `reason=PolicyViolation` on the plan's policies (`discovery/core/plans/protection/violations/violations.go`). Events are pruned after the API server's event TTL, so `RetentionWindow` is 1 h.
- `CleanupByPlanID` runs on cancel, terminate, reject, clear, failed deployment and edits that withdraw policies, and the rendered-policy list is then blanked. An edit that withdraws policies checkpoints the ledger first. Cancel, terminate and reject patch the phase first and clean up after, so a failed patch never leaves an "active" plan enforcing nothing; a failed cleanup is left to the orphan sweep. A finished plan therefore reports zero live violations; its history lives only in reports ([AGENTS.md](../../AGENTS.md#go-services), Known pitfall).

## Reports

- discovery renders HTML, Markdown, JSON or CSV (`discovery/core/plans/protection/reports/`); the exporter stores them on the reports volume under `/reports/plans/<plan id>/`, keeping 10 on-demand reports per plan plus a `ledger.json` (`services/exporter/internal/utils/reports/store.go`).
- A leader checkpoint loop merges violation Events into the plan's ledger before they expire (`PROTECTION_PLAN_REPORT_CHECKPOINT_SEC`, default 900, clamped 60–1800). A final report is captured asynchronously right after a plan ends or is canceled. An edit that parks an active plan (moves its start into the future) also writes an end report for that run (`trigger=end`, reason "Plan parked by an edit.").
- The exporter garbage-collects report directories whose plan no longer exists.

## Permissions

- User actions go through discovery's plan routes (`services/discovery/internal/authz/requirements.go`, `addPlans`): viewing needs ReadOnly on `protection-plans`, create, edit, duplicate, cancel, reactivate and report generation need Contributor, and clear (delete) needs Owner; each carries its own deny rule. The decide route needs Owner and the handler applies the approve or reject rule. Two rules read the caller's level inside discovery: `enforce` on a `namespaces` scope and a client-sent `approvalMode: automatic` both need Owner. Plan request bodies are capped at 1 MiB and unknown fields are refused.
- The exporter's plan routes serve reads and reports to users, but a user request that touches lifecycle or spec fields (phase, approval, policies, scope, mode, time window, name, …) is refused (`GuardPlanLifecycle`, `services/exporter/internal/constants/plans.go`); only discovery, with the service token, writes them. Plan DELETE on the exporter is Internal.
