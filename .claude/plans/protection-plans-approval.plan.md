# Protection Plans — Approval gate (execute automatically vs. requires approval)

**Status: PLANNED, not implemented.** Plan only; nothing in this document has been applied.

**Verified tree state (2026-09-23, working tree, never HEAD):**

| Repo | Branch | State |
|---|---|---|
| /Users/houssem/Desktop/Github/telark | e2e/set_plans_feature_ready_for_mvp_launch | dirty, uncommitted (Plan 1 Environments + Tags on top of the branch) |
| /Users/houssem/Desktop/dashboard-ui | e2e/set_plans_feature_ready_for_mvp_launch | dirty, uncommitted (Plan 1 UI) |
| /Users/houssem/Desktop/Github/internal/data | main, v1.14.8-1-g36ba327 | staged Plan 1 edits (category builtins, plans/protectionplan.go, tests) |
| /Users/houssem/Desktop/Github/internal/rest | main, v0.14.4 | staged Plan 1 edits (endpoints/plans/types.go, payload mapper, tests) |

Companion JSON (identical content, machine-readable): `/Users/houssem/Desktop/Github/telark/.claude/plans/protection-plans-approval.plan.json`.
Related: Plan 1 `protection-plans-environments-tags.plan.md` (Decisions D1–D20, especially D3/D4/D5/D7/D8/D16/D17); Plan 2 (Reports) is planned separately — this plan only leaves a hook (`spec.approval.history`, two notification types).

## Approach

PLANNED, not implemented. Tree state verified 2026-09-23: telark on e2e/set_plans_feature_ready_for_mvp_launch (dirty, Plan 1 uncommitted), dashboard-ui same branch (dirty), internal/data main at v1.14.8-1-g36ba327 with staged Plan 1 edits, internal/rest main at v0.14.4 with staged Plan 1 edits. 'Executes' = rendering + server-side-applying policy-engine resources, which happens at exactly four sites today: Prepare (svc.go:162-166, also reached by Duplicate svc.go:107), controller Activate (svc.go:317-353 via controller.go:132-140), Reactivate (svc.go:232-236) and Update's ApplyClusterDiff on an ACTIVE plan (cluster_diff.go:34-36). Every other cluster-touching path (health repair repair.go:29, ReconcileForActive check.go:97, compute shortCircuit compute.go:39) keys on phase == active. Design: one new non-active phase `pending_approval` (CRD enum first) plus a spec.approvalMode string enum (automatic|required, absent = automatic) and a spec.approval object (state, requestedBy/At, decidedBy/At, comment, bounded history). A plan whose approvalMode is `required` is parked in pending_approval at every entry that would otherwise deploy (Prepare/Duplicate, Reactivate); nothing is rendered while pending and every existing phase==active guard keeps update/health/repair/controller inert for free. Default: approvalMode is resolved ONCE at create/duplicate and persisted: explicit request value wins, else `required` iff environmentID == constants.CategoryIDEnvProduction (cat-00002-0001-0001), else `automatic`. No approver gate on choosing `automatic`: Write suffices; the Production default of `required` makes an unapproved Production plan an explicit, visible choice (an Owner gate at create/duplicate was cut in r1 because a Staging-created automatic plan relabelled to Production bypassed it). One new discovery route POST plans/protection/{id}/decide {decision: approved|rejected, comment?, requestedAt} guarded by Denyable(Own(protection-plans), approveprotectionplan) with requester != decider enforced in core (403), a requestedAt fingerprint (409 on mismatch) and a per-plan cross-replica lock taken in the decide, cancel and update handlers (rollback pattern). Approve computes the target phase exactly like Reactivate (expired => 400, future startAt => scheduled, else active + deploy). Reject => phase canceled + approval.state=rejected + reason=comment (no new terminal phase). Controller learns one thing: a pending time_range plan whose endAt passes goes through the existing Terminate. Activate re-reads the plan before deploying (stale in-memory snapshot guard). Material edits of an approved active/scheduled required plan are refused (duplicate or cancel+reactivate instead); edits of a pending plan re-request approval. Exporter gains an Internal-only guard for lifecycle/approval AND material keys (policies, scope, mode, timeMode, timeRange) on the user-reachable plan create/patch (closes a pre-existing phase-forgery hole and the approval bypass through a direct spec PATCH). Notifications reuse the existing discovery -> exporter in-app path with two new types. UI: new phase label/colour, Execution select with environment-derived default and explanation, Approve/Reject with a comment modal on card + details, requester/decider rows, 'Pending approval' pill, notification deep-links. Backward compatible: absent fields mean automatic; existing CRs and older UI bundles change no behaviour; rollout order CRD chart -> exporter -> discovery -> UI.

## Execution flow today (traced)

Verified on the working tree (cited file:line as of 2026-09-23).
1. UI POST discovery plans/protection/prepare (dashboard-ui src/constants/rest/paths.ts:103; clients/prepare.ts:21-35) -> discovery routes.go:40, authz Write(protection-plans), not Denyable (services/discovery/internal/authz/requirements.go:59) -> handlers/plans/protection/prepare.go -> Service.Prepare (core/plans/protection/svc.go:134-176).
2. Prepare: validation.PrepareRequest -> NamespaceScope -> ensureNameAvailable -> resolveScope -> ids.GeneratePlanID -> buildPlan (svc.go:382, Plan 1 env/tags) -> assignInitialPhase (svc.go:417-430: time_range with future startAt => scheduled; else active + startedAt/By=creator).
3. shouldRender (svc.go:560-562, phase==active only) -> renderAndDeploy (svc.go:432-450): policies.Render + applier.Deploy BEFORE the CR exists (EXECUTION POINT 1). Failure => CleanupByPlanID, request fails, nothing persisted.
4. exporter.CreateOrError (svc.go:168 -> clients/protectionplans.go:83 -> exporter handlers/plans/protection/handler.go:25-62: GetSpec, Validate, ApplyCreateAudit (utils/plans/protection/audit.go:11-22 stamps startedAt/By when phase==active), GenericCreateCustomResource). Persist failure => rollbackOnPersistFailure (svc.go:452-459). Active => StampFirstHealth (svc.go:172-174; health/check.go:62-81).
5. Boundary controller (leader-gated, main.go:193-206): Run (controller.go:49-67) = ticker PROTECTION_PLAN_TICK_INTERVAL_SEC default 31s + one-shot timer to the nearest future startAt/endAt of plans where awaitsBoundary (controller.go:128-130 = scheduled|active). transition (controller.go:132-151): scheduled && startAt<=now => Service.Activate (svc.go:317-353: resolve, Render, Deploy, typed patch phase=active startedBy=system; failure => markFailedRemote svc.go:461-467) (EXECUTION POINT 2); active && endAt<=now => Service.Terminate (svc.go:355-366: CleanupByPlanID + raw BuildTerminatePatch svc.go:492-503 reason 'Plan window ended.'). Every other phase hits the default branch and is ignored. Activate uses the controller's in-memory plan and never re-reads it.
6. Every tick: ReconcileHealthForActive (controller.go:85-87 -> health/check.go:84-106, phase==active only at :97). repair.go:23-56 redeploys/repatches/deletes only when phase==active (:29) and after an exporter re-read confirms active (:32-42, activeAtSource :58-64). health.Check (check.go:34-57, used by GET {id}/status status.go:24 and StampFirst) ALWAYS PATCHes health for every phase (check.go:49 -> ToPatch health/patch.go:11-21), and exporter ApplyPatchAudit then bumps lastUpdatedAt/By=system (audit.go:24-31). dashboard-ui Content.tsx:94 refetches status whenever lastUpdatedAt changes.
7. Update: POST {id}/update -> handlers/plans/protection/update.go:20-71 -> update.Run (update/orchestrator.go:35-94): validateRequest (:96-126; updatable :158-163 = active|scheduled|failed|draft), policy x target diffs, ApplyClusterDiff (policies/cluster_diff.go:24-70, deploys ONLY when phase==active :34-36) (EXECUTION POINT 3), BuildPatch (update/patch.go:12-30: never phase/started/terminated/reason, always health=unknown), PatchOrError, RollbackPatchFailure on error, StampHealth if active.
8. Cancel: POST {id}/cancel -> cancel.go:16-57 -> svc.go:178-201, cancellable (svc.go:550-552 = active|scheduled|failed); CleanupByPlanID then raw BuildCancelPatch (svc.go:479-490).
9. Reactivate: POST {id}/reactivate -> svc.go:203-242, reactivatable (:554-558 = canceled|terminated|failed); computeReactivatePhase (:263-276: endAt past => expired 400; startAt future => scheduled; else active); active => renderAndDeploy BEFORE persistReactivate (:244-261, raw buildReactivatePatch :278-302 with startedBy=user, cleanup on patch failure) (EXECUTION POINT 4).
10. Duplicate: POST {id}/duplicate -> svc.go:86-108 -> duplicate.BuildRequest (duplicate/duplicate.go:43-73, Prepare-shaped; env/tags overridable) -> s.Prepare (:107) => steps 2-4 again.
11. Clear: DELETE {id}/clear (Own, requirements.go:64) -> svc.go:368-376 CleanupByPlanID + exporter Delete.
12. Reads: UI polls exporter plans/protection/get and {id}/get (clients/fetch.ts:16-31); exporter returns spec only (utils/shared/filter.go:27-70). CRD has no status subresource; everything is under spec (charts/telark-crds/templates/crds/plans/protectionplan.yaml:47-63 required list, :157 phase enum [draft, scheduled, active, terminated, canceled, failed]).
13. Exporter PATCH plans/protection/{id}/patch and create are user-reachable (nginx /api/exporter/; exporter authz/requirements.go:131-138 Write + createprotectionplan / editprotectionplan deny) and merge ANY spec key after only ValidatePatchScope + ApplyPatchAudit (handler.go:87-122). A Contributor can set phase/renderedPolicies directly today.
14. Authz plumbing: x-ware middleware (discovery main.go:73 xauthz.NewFromEnv) resolves the session, strips and re-sets X-User-ID from the resolved identity and stores Identity{UserID, Internal, Grants} on the request context (x-ware/authz/handler.go:137-142, context.go:7-14). Discovery handlers read X-User-ID (prepare.go:24-28, cancel.go:21-25); no discovery production code calls xauthz.FromContext today. Service-token callers are Identity{Internal:true} with the claimed X-User-ID and skip level and deny checks (handler.go:74-80), so discovery -> exporter calls never hit exporter deny flags.
15. Notifications: nothing in the plan lifecycle emits one. The only pattern is rollback (handlers/rollback/controller.go:361-382): notifclient.Notification{UserID, Type, Title, Message, Severity, Metadata} dispatched via async.Dispatch to clients.NotificationClient.Emit (clients/notifications.go:15-30, 3s timeout, fire-and-forget) -> exporter notifications/emit (Internal) -> Redis per-user list with dedup on unread (userId, type, targetId). services/notifier is a NATS -> Application-CR reconciler and plays no role. Types are duplicated in internal/rest/clients/notifications/types.go:28-32 and exporter types/notifications/types.go:23-27.
16. Cross-replica lock precedent: handlers/resources/applications/rollback.go:36-56 lockRollback uses coord.Lock.Acquire(ctx, key, uuid, constants.DefaultLockTTL) from the coordination bundle set at main.go:281, with lockRollbackLocal (in-process sync.Mutex) as the no-bundle fallback and 409 on contention. protection.Service has no access to the bundle today.
17. Discovery error mapping (handlers/plans/protection/errors.go:19-32) knows 404/400/503/500 only; no 403 or 409.
18. `draft` is in the CRD enum, data constants (internal/data/plans/protectionplan.go:6) and UI maps but is never assigned by any code path; only update.updatable accepts it.

## Proposed lifecycle / state machine

```
                 create / duplicate (Prepare)                     Reactivate
   automatic ───────────────┐   required                    required │ automatic
                            │      │                                 │     │
                            ▼      ▼                                 ▼     ▼
  future startAt ──► scheduled   pending_approval ◄─────────────────┘  (scheduled | active as today)
        │               ▲            │  │  │  │
        │ controller    │ decide     │  │  │  └─ endAt passes (controller) ──► terminated (Terminate, approval stays pending)
        │ Activate      │ approved   │  │  └──── Cancel (withdraw, lock) ─────► canceled (approval stays pending)
        │ (re-reads,    │ (future    │  └─────── decide rejected (comment) ───► canceled + approval.rejected + reason=comment
        │  Activatable) │  startAt)  └────────── decide approved (in window) ─► active (deploy first, startedBy=approver)
        ▼               │
      active ───────────┘
        │ endAt (Terminate) ──► terminated      material edit of an approved REQUIRED plan (active|scheduled) ──► 400
        │ Cancel ──────────────► canceled       edit of a pending plan: material ⇒ approval reset + approvers re-notified
        │ Activate failure ────► failed         approvalMode: resolved+persisted at create; immutable on update
  canceled | terminated | failed ── Reactivate ──► required ⇒ pending_approval ; automatic ⇒ today's path
  any ── Clear (Owner) ──► deleted
```

PHASES (spec.phase; CRD enum gains one value): pending_approval (NEW), scheduled, active, failed, canceled, terminated, draft (unchanged, still never assigned). Health stays `unknown` while pending (compute.go shortCircuit). Rejection is NOT a phase: phase=canceled + approval.state=rejected.

SPEC FIELDS: approvalMode in {automatic, required}, absent => automatic (predicate RequiresApproval(plan) = plan.ApprovalMode == "required"; "" and "automatic" behave identically). approval {state pending|approved|rejected, requestedBy, requestedAt, decidedBy?, decidedAt?, comment?, history[<=20]{event requested|approved|rejected, by, at, comment?}}. Written ONLY by discovery through PatchRaw (nil clears decided* on re-request; the whole object is sent so merge-patch key-by-key merging cannot leave stale keys). Exporter refuses these keys, and the material keys (policies, scope, mode, timeMode, timeRange), from non-Internal identities.

TRANSITIONS (actor | authz | cluster effect):
A. create / duplicate -> pending_approval | scheduled | active (user | Write | Prepare svc.go:134): buildPlan sets ApprovalMode = ResolveApprovalMode(req.ApprovalMode, plan.EnvironmentID). No approver gate on the resolved mode (Write suffices). If RequiresApproval: phase=pending_approval, approval={pending, requestedBy=creator, requestedAt=now, history=[requested]}, startedAt/By nil, shouldRender false => nothing deployed; after CreateOrError notify approvers. Else today's assignInitialPhase.
B. pending_approval -> scheduled | active (approver | POST {id}/decide approved | Denyable(Own(protection-plans), approveprotectionplan) | per-plan lock in handler | core: phase must be pending_approval and approval non-nil else 409; req.requestedAt must equal approval.requestedAt else 409; decider != approval.requestedBy else 403 (Admin not exempt)): NamespaceScope + resolveScopeForPlan + validateTemplatesForPlan (as Reactivate); computeReactivatePhase: expired => 400 (plan stays pending; controller terminates it); scheduled => raw BuildApprovePatch(phase scheduled, approval approved + decidedBy/At/comment + history+=approved, startedAt/By nil, reason nil, health unknown); active => renderAndDeploy, then re-read: if phase != pending_approval or approval.requestedAt != req.requestedAt => CleanupByPlanID + 409 (controller expiry or a material edit raced), else raw patch with phase active, startedAt=now, startedBy=approver, renderedPolicies; patch failure => CleanupByPlanID (persistReactivate pattern); StampFirstHealth. Deploy failure => error, plan stays pending (never failed). Notify requester (decided).
C. pending_approval -> canceled/rejected (approver | decide rejected | same authz, lock, fingerprint, SoD | comment REQUIRED else 400): CleanupByPlanID (idempotent, defence in depth) then raw BuildRejectPatch = BuildCancelPatch(approver, comment) + approval {rejected, decidedBy/At, comment, history+=rejected}. Notify requester.
D. pending_approval -> canceled (user | Cancel | Write; handler takes the same per-plan lock): cancellable gains pending_approval; existing cleanup + BuildCancelPatch; approval block untouched (state stays pending, reads as withdrawn).
E. pending_approval -> terminated (system | leader controller): awaitsBoundary gains pending_approval so the timer is armed on its endAt; transition gains `case PhasePendingApproval: if shouldTerminate => Service.Terminate` (unchanged Terminate: cleanup no-op + BuildTerminatePatch reason 'Plan window ended.'; approval.state stays pending). startAt passing while pending does nothing; a later approve starts late, bounded by endAt.
F. scheduled -> active at startAt (controller Activate): Activate now re-reads the plan via the exporter and returns without deploying unless Activatable(current) = phase==scheduled && (!RequiresApproval || approval.state==approved). Closes the race where a scheduled plan re-parked or canceled after the controller's LIST would have been deployed from the stale snapshot.
G. update of a pending plan (user | Write; handler takes the same per-plan lock as decide/cancel): updatable gains pending_approval; ApplyClusterDiff is a no-op (non-active). MaterialChange (policies, scope targets, mode, timeMode/timeRange) => one raw patch = typed BuildPatch mapped to a map + approval reset {pending, requestedBy=editor, requestedAt=now, decided*=nil, history+=requested}; notify approvers (exporter dedup collapses unread duplicates). Metadata-only edits keep the request as-is. approvalMode is immutable on update: a request value whose EFFECTIVE mode differs from the stored effective mode => 400; the edit form never sends it.
H. update of an approved required plan (active OR scheduled): MaterialChange => 400 ErrApprovedPlanMaterialEdit ('duplicate the plan, or cancel and reactivate it to re-request approval'); metadata-only edits pass. Automatic plans: unchanged.
I. canceled|terminated|failed -> pending_approval (user | Reactivate | Write) when RequiresApproval: after the existing expired check (400), raw BuildPendingPatch: phase pending_approval, approval reset (requestedBy=user, requestedAt=now, decided*=nil, history kept + requested), startedAt/By nil, reason/terminated* nil, renderedPolicies [], health unknown; no deploy; notify approvers. Automatic plans: unchanged.
J. any -> deleted (Owner | Clear): unchanged.
K. duplicate: BuildRequest copies source.ApprovalMode unless the request overrides approvalMode, or overrides environmentID with a value different from the source's (the UI sends environmentID whenever tags are touched, so 'sent' is not 'changed'); otherwise ApprovalMode nil => Prepare re-derives from the new environment; approval state is never copied (Prepare-shaped DTO has no field). The copy re-enters A.
L. health self-loop: only active plans are reconciled/repaired (unchanged). health.Check stops PATCHing health for non-active plans (they already store health=unknown from every non-active transition), so a pending plan open in the details page no longer churns lastUpdatedAt/By=system on every status poll.
CONCURRENCY: decide, cancel and update take a per-plan cross-replica lock (coordination bundle, in-process mutex fallback, 409 on contention); the requestedAt fingerprint catches an edit between the approver's read and click; the post-deploy re-read (phase + approval.requestedAt) catches controller expiry and a racing material edit; Activate's re-read catches re-park/cancel between LIST and deploy. Ceiling accepted: an approve landing right before the controller's endAt pass leaves policies deployed for at most one tick before Terminate cleans them.

## Decisions (defaults the user can override)

| # | Question | Decision | Why |
|---|---|---|---|
| D1 | What does 'executes' mean and which transitions are gated? | Execute = render + deploy policy-engine resources. Gated: every entry into active/scheduled for approvalMode=required plans (Prepare, Duplicate->Prepare, Reactivate, hence controller Activate which only sees approved scheduled plans and now re-reads before deploying) plus material updates of approved required plans (refused). Metadata edits, cancel, clear and health repair are never gated. | Verified: the four deploy sites are svc.go:162-166, :232-236, :317-353 and cluster_diff.go:34; every other cluster-touching path keys on phase==active (repair.go:29, check.go:97, compute.go:39), so a non-active parking phase closes them by construction. |
| D2 | New phase value or reuse the never-assigned `draft`? | New phase `pending_approval` added to the CRD enum first; `draft` untouched. | Reusing draft is one line cheaper but lies in kubectl and in every older UI bundle ('Draft' implies not submitted). A new enum value also fails LOUDLY (422) if a service ships before the CRD chart, whereas draft + pruned approval keys would silently produce a 'Draft' plan with nil approval that decide cannot act on. Cost is one CRD line, one data constant and compiler-enforced UI Record maps. |
| D3 | Where does approval state live (spec vs status)? | spec.approvalMode (string enum) + spec.approval object. No status subresource. | No CRD in telark-crds has a status subresource and the exporter returns spec only (utils/shared/filter.go:27-70). A bool would be dropped by MapToJSONPayload when false (payload.go isZeroValue), so an explicit 'automatic on Production' needs a string. |
| D4 | Per-plan default and how is 'especially production' expressed? | Resolved once at create/duplicate and PERSISTED: request value wins; nil => required iff environmentID == constants.CategoryIDEnvProduction (internal/data/constants/builtin.go:18), else automatic. Absent on existing CRs => automatic. No approver gate on choosing automatic (Write suffices). No environment-level setting. | Zero new clients (Category has no settings field, def.go:7-13; discovery has no categories client). Persisting the resolved value makes later environment rename/delete/reassignment irrelevant (Plan 1 D4/D7 allow both). An Owner gate on the resolved mode was cut in r1: environmentID is a metadata edit (D9), so create-in-Staging-then-relabel-to-Production bypassed it, and closing that needs a second gate on update for a policy nobody asked for. Automatic on Production is an explicit choice shown as 'Automatic' on card and details. Upgrade path if custom production-like environments are requested: Category.requiresApproval + rest categories client + discovery wrapper. |
| D5 | Routes and actions? | One route POST plans/protection/{id}/decide {decision: approved|rejected, comment?, requestedAt} and one new action approveprotectionplan; Denyable(Own(protection-plans), approveprotectionplan). Cancel (Write) remains the requester's withdrawal. | One handler, one requirement entry, one UI client/thunk, and both decisions share the lock, fingerprint, SoD and re-read code (graft from the minimal design). |
| D6 | Who may approve; separation of duties; Admin? | Owner or Admin on protection-plans unless denied; decider != approval.requestedBy enforced in core (403); Admin is NOT exempt. Identity source for SoD is the X-User-ID header the middleware re-sets from the resolved session (x-ware handler.go:137-142), the same field requestedBy is stamped from. | Middleware knows only level and deny, so SoD must be a domain check. The solo Owner/Admin escape hatch is creating the plan with approvalMode=automatic, never self-approval; exempting Admin would be a new policy nobody asked for. |
| D7 | Rejection representation? | phase canceled + reason=comment + approval{rejected, decidedBy/At, comment}; re-request = existing Reactivate (which re-parks required plans). | Reuses BuildCancelPatch and reactivatable; no new terminal enum; an older UI shows Canceled with the comment as reason. |
| D8 | Window start / end while pending? | startAt: nothing happens; approve later starts late (startedAt = decision time, startedBy = approver). endAt: leader controller terminates via the unchanged Terminate ('Plan window ended.'); decide then 409 not pending; reactivate 400 expired. | Two lines in awaitsBoundary/transition; keeps the PlanService interface and every planctl fake untouched (graft). Starting without approval is exactly what the feature forbids. No 'expired' history event: terminated + approval.state=pending is unambiguous. |
| D9 | Editing an approved plan? | Material change (policies, scope targets, mode, timeMode/timeRange) on an approved required plan that is active OR scheduled => 400 with guidance; metadata edits (name, description, severity, priority, participants, environment, tags) pass. Pending plan: material edits re-request approval with the editor as requester. approvalMode immutable on update (effective-mode comparison; the edit form does not send it; change it by duplicating). | Re-parking an ACTIVE plan removes live protection silently; refusing is smaller and safer. Refusing scheduled too keeps one predicate (cancel -> reactivate -> edit-while-pending is the path, three clicks, zero extra code). Effective-mode comparison avoids a spurious 400 for a legacy CR with no key when a client sends 'automatic'. |
| D10 | Concurrency guards on decide? | Per-plan cross-replica lock taken in the decide, cancel AND update handlers (copy of lockRollback: coord.Lock.Acquire with constants.DefaultLockTTL, in-process mutex fallback, 409 on contention; bundle set from main.go beside line 281) + requestedAt fingerprint (409) + post-deploy re-read of phase AND approval.requestedAt (409 + cleanup) + Activate re-read. | The judge's must-fix list. lastUpdatedAt was rejected as the fingerprint because health status GETs bump it for every phase (check.go:49 -> audit.go:26); approval.requestedAt changes only on request/re-request/material edit, which is exactly what an approver must have seen. Update takes the same lock because an unlocked material edit interleaving with approve (either order) leaves an active plan whose spec was never approved, which health repair then deploys (r1 critical). |
| D11 | Audit trail and Reports hook? | approval.{requestedBy/At, decidedBy/At, comment} + bounded approval.history (maxItems 20; events requested/approved/rejected; the core helper drops the oldest before any patch so the CRD never 422s) + existing startedAt/By, terminatedAt/By. No separate event store. | Reactivate/re-request overwrite the current slot; history keeps prior rejections for audit and is the Plan 2 Reports hook. 20 = ten full request/decision cycles; a plan re-requested more than that is an outlier. Nothing else on the CR is append-only, so no new store. |
| D12 | Notifications: channel and recipients? | Existing discovery -> exporter in-app path, two new types (plan.approval.requested to every user whose grants satisfy ApprovePlanRequirement minus the requester; plan.approval.decided to the requester). Approver fan-out = AuthzClient.GetAllUsers (new wrapper method) + cached GrantsForUser + xauthz.Allows, inside async.Dispatch, fire-and-forget. Titles/messages in constants/messages.go. No expiry notification. | services/notifier is an Application-CR reconciler; per-user addressing is all that exists; distinct types keep the exporter dedup from replacing a request with a decision. The 'Pending approval' pill is the authoritative queue, so a lost notification is tolerable. // ponytail: O(users) grant lookups per request via the cached resolver; add an approver-set cache if user counts grow. |
| D13 | Exporter direct-write hole? | GuardPlanLifecycle on plan create and patch: fixed key list {approvalMode, approval, phase, renderedPolicies, startedAt, startedBy, terminatedAt, terminatedBy, reason, health, healthCheckedAt, healthDetail, policies, scope, mode, timeMode, timeRange}; Internal passes; any key present for a session identity => 403. Route requirements unchanged. | Copies GuardGlobalConfigPatch (guard.go:253-288) verbatim; keeps the hard-coded route tests untouched, unlike making the routes Internal. reason and healthDetail are included because a rejection comment lands in reason and a session must not forge it (graft). The material keys are included because the UI writes plans only through discovery (dashboard-ui paths.ts:100-110) and a direct exporter PATCH of mode/policies/timeRange on an approved or pending required plan would skip ErrApprovedPlanMaterialEdit and the approval reset, after which health repair applies the unapproved spec (r1 critical). |
| D14 | Approve deploy failure => failed or pending? | Stays pending, error returned to the approver. | Mirrors Prepare (nothing persisted on deploy failure); the approver retries after fixing templates/apps; avoids a failed plan that was never approved. |
| D15 | UI production default without a server hint? | One mirrored constant PLAN_APPROVAL.PRODUCTION_ENVIRONMENT_ID = 'cat-00002-0001-0001' in the plans feature constants, documented as a mirror of internal/data builtins; server stays authoritative (nil => derive). Re-derived on environment change only while the Execution field is untouched (form.isFieldsTouched, DuplicatePlanPanel.tsx:129 pattern). | Built-in ids are fixed by definition (restored by id on boot); an endpoint that echoes a constant is not worth it. Worst case the UI default is wrong and the server still gates. |
| D16 | List discoverability: pill or filter-panel field? | Add a 'Pending approval' quick-filter pill (existing phase-filter mechanism); re-measuring TOOLBAR_COMPACT_WIDTH is a user-owned follow-up that Plan 1 D17 already lists. | Pills are the only phase filter today; a second phase filter in the panel would be a parallel mechanism. The width constant is already stale after Plan 1's Organize button, so the re-measure is owed regardless (memory rule: measure, never guess). |
| D17 | Comment modal primitive? | ApprovalDecisionModal = thin wrapper over ActionConfirmModal (ReactivatePlanModal shape) whose customMessage holds the summary + an antd Input.TextArea (maxLength 500, showCount); comment required for reject, optional for approve. | FormModal is any-typed (FormModal.tsx:44, interfaces/layout/modal.ts:11) and would violate no-any; customMessage is already a ReactNode. |
| D18 | Pending phase colour? | PHASE_ACCENT.pending_approval = DEFAULT_COLORS.WARNING (same token as scheduled), PHASE_DOT_COLOR.pending_approval = DEFAULT_COLORS.WARNING; no hex. Flagged for the user's screenshot review. | colors.ts has only SUCCESS/WARNING/DANGER/DEFAULT tint pairs; inventing a new token is a design decision that needs a screenshot (memory rule), not a guess. |
| D19 | Do health status GETs keep patching non-active plans? | No: health.Check skips the PATCH when the plan is not active (returns Result{Health: unknown}). | Every non-active transition already writes health=unknown (cancel/terminate/failed/reactivate-scheduled/update), so the patch was pure churn that bumped lastUpdatedAt/By=system and made Content.tsx:94 refetch in a loop for a pending plan. Documented behaviour change: healthCheckedAt is no longer stamped for non-active plans. |
| D20 | Where do the notifier and lock live given Service holds concrete clients? | ApprovalNotifier is a plain struct with func fields (ListUsers, Grants, Emit, Requirement) in core/plans/protection/notify.go, constructed in wire.go and held by Service (nil-safe); the lock lives in handlers/plans/protection/lock.go (handler layer, like rollback) with a SetCoordinationBundle setter in init.go. | Func fields make the fan-out testable with fakes without an interface-with-one-implementation; the coordination bundle is a handler-layer concern today (applications/coordination.go:25) and injecting it into Service would widen NewService for one call site. |

## Edge cases

| Case | Behaviour | Covered by |
|---|---|---|
| Existing CR without approvalMode/approval (every current plan) | RequiresApproval false on every path; controller/update/health unchanged; update never injects the keys; reactivate/duplicate follow today's rules except that a duplicate whose environment is overridden to Production re-derives required. | S2 (round-trip test), S6 (ResolveApprovalMode nil cases), S7 |
| Pending plan whose startAt passes | Controller ignores it (transition acts on pending only at endAt). UI header shows 'Awaiting approval · requested <ago>'. A later approve within the window starts it late with startedBy=approver. | S10 (TestPendingIsNotActivatedAtStartAt), S7 |
| Pending plan whose endAt passes | awaitsBoundary includes pending so the timer is armed; transition pending + endAt<=now => Terminate (cleanup no-op, reason 'Plan window ended.', approval.state stays pending). decide => 409 not pending; reactivate => 400 expired. | S10 (TestPendingTerminatesAtEndAt, leadership-start fixture) |
| Approve racing controller expiry at endAt | Approve-first: controller's next pass sees active + endAt past => Terminate cleans (<= one tick of deployed policies). Expiry-first: approve's post-deploy re-read sees terminated => CleanupByPlanID + 409. | S7 (post-deploy re-read), S10 |
| Approve after endAt before the controller fires | computeReactivatePhase => expired => 400; plan stays pending until the controller terminates it. UI disables Approve when isReactivateExpired(plan). | S7, U4 |
| Two approvers click at once on different replicas | Per-plan lock: second gets 409 'decision in progress'; after release its phase check sees scheduled/active => 409 not pending. | S8 (lock helper), S6 (ValidateDecision) |
| Update (material edit) racing decide on a pending plan | Update handler takes the same per-plan lock as decide/cancel, so the two serialise: edit-first => decide's Get sees the new requestedAt => 409 stale; approve-first => update's Get sees active/scheduled => 400 ErrApprovedPlanMaterialEdit. Defence in depth: decide's post-deploy re-read requires phase pending_approval AND approval.requestedAt == req.RequestedAt, else CleanupByPlanID + 409. | S8 (lock helper), S9 (lock in update handler), S7 (re-read) |
| Approver decides on a version they did not see (edit in between) | A material edit of a pending plan re-stamps approval.requestedAt; decide carries the requestedAt it saw => 409 stale; UI refetches and shows the new requester/time. | S6 (ValidateDecision fingerprint), S9 (reset on material edit), U4 |
| Self-approval (decider == requestedBy), including an Owner approving their own material edit | 403 from core; UI disables Approve/Reject with tooltip. Admin not exempt. | S6, S8 (403 mapping), U4 |
| Contributor sets approvalMode=automatic on a Production plan (explicitly, via duplicate, or by relabelling a Staging automatic plan to Production) | Accepted: Write suffices; no approver gate (cut in r1). The Production default is required, so automatic on Production is always an explicit choice, shown as 'Automatic' on card and details. | S7 (ResolveApprovalMode), U3, U4 |
| Contributor forges phase/approval, or edits policies/scope/mode/timeMode/timeRange, through exporter PATCH or CREATE (skipping discovery's update gate) | GuardPlanLifecycle => 403 for non-Internal identities on any listed key (lifecycle, approval and material keys); keys outside the list behave as before; Internal passes. | S5 |
| Reject without comment | 400 ErrApprovalCommentRequired; UI disables OK until non-empty. Comment > 500 chars => 400 (mirrors CRD maxLength). | S6, U4 |
| Decide on a plan that is not pending (active, scheduled, canceled, terminated, failed, or approvalMode absent) | 409 ErrApprovalNotPending regardless of grants; UI hides the buttons (APPROVABLE_PHASES = ['pending_approval']). | S6, S8, U4 |
| Approve when apps/templates no longer resolve or deploy fails | Error returned (400/503); renderAndDeploy cleans partials; plan stays pending, nothing persisted; approver retries. | S7 |
| Exporter patch fails after a successful deploy on approve | CleanupByPlanID then error (persistReactivate pattern); plan stays pending. | S7 |
| Cancel of a pending plan (withdraw) | Allowed (Write); handler takes the per-plan lock; cleanup no-op + BuildCancelPatch; approval.state stays pending. Reactivate re-parks with a fresh request. | S7 (cancellable), S8 (lock in cancel), U1 (CANCELLABLE_PHASES) |
| Reactivate of a rejected / canceled / terminated required plan | Expired check first (400). Then BuildPendingPatch: pending, approval reset (requestedBy=reactivator, decided* nil, history kept + requested), no deploy, approvers notified. Automatic plans unchanged. | S6 (BuildPendingPatch), S7 |
| Duplicate of an approved plan (or one touching only tags) | BuildRequest never copies approval; copies approvalMode unless approvalMode is overridden or the overridden environmentID differs from the source's (DuplicatePlanPanel sends environmentID whenever tags are touched, so 'sent' != 'changed'); the UI seeds the duplicate form from the source mode; the copy re-enters Prepare and parks if required. | S7 (planduplicate tests), U3 |
| Update of a pending plan | Allowed; no cluster effect; material change => single raw patch with approval reset + approvers notified (dedup collapses); metadata-only => request unchanged. | S9 |
| Update of an approved required plan (scheduled or active) | Material change => 400 ErrApprovedPlanMaterialEdit before resolveTargets/ApplyClusterDiff; metadata-only passes. UI locks Scope/Policies/Schedule/mode sections with an Alert pointing to Duplicate or Cancel+Reactivate. | S9, U3 |
| Legacy CR (no approvalMode) edited by a client that sends approvalMode 'automatic' | Effective modes equal ('' == automatic) => accepted, no key written. Sending 'required' => 400 immutable. | S9 (effective-mode test) |
| Scheduled approved plan re-parked/canceled between the controller's LIST and Activate | Activate re-reads; Activatable(current) false => logs and returns nil without deploying or patching. | S7 (Activatable predicate test) |
| Failed activation of an approved scheduled plan | markFailedRemote as today; approval.state stays approved; Reactivate re-parks (fresh approval after a failure is the safe default). | S7 |
| Environment deleted/renamed/changed after creation | approvalMode is persisted; nothing changes. Changing environmentID on any plan is metadata-only. | S9 (MaterialChange matrix: env-only false) |
| GET {id}/status on a pending plan | Compute short-circuits to unknown; no repair; health.Check no longer PATCHes non-active plans, so lastUpdatedAt/By stop churning and Content.tsx:94 stops re-fetching in a loop. | S10 (planhealth test) |
| Older UI bundle after backend rollout | Unknown phase renders the raw label 'pending_approval' with DEFAULT_COLORS.DEFAULT accent (ProtectionPlanCard.tsx:247-250), no Cancel (phaseRules.ts:4), Edit enabled (harmless: update re-requests), no Approve. Rejected plans show as Canceled with reason=comment. Unknown notification types show a bell with no navigation. Owners can Clear. Documented in the chart README rollout note. | D1 |
| New UI bundle before backend rollout | approvalMode is sent; old discovery decodes with encoding/json and ignores it => plan created automatic; decide route 404 => generic error toast. No corruption. | D1 |
| New discovery before the CRD chart upgrade | Exporter create/patch of phase pending_approval is rejected by the API server (enum) => Prepare returns 503 and nothing is deployed (deploy happens only for active). Loud, not silent. Chart README documents telark-crds before telark. | S1, D1 |
| Zero approvers exist (no Owner besides the requester) | Request succeeds; nobody notified; plan is visible under the pending pill; the requester's escape hatch is asking an Admin (Admin holds Own via ALL). | S11 |
| Requester deleted before the decision | Decide proceeds; decided notification Emit fails silently; UI renderUserAndTime falls back to the raw id as for createdBy today. | S11, U4 |
| approval.history exceeds 20 | AppendApprovalEvent drops the oldest entries before any raw patch; CRD maxItems never trips. | S6 |
| Leader failover while a pending plan's endAt is overdue | Boundary timer fires immediately on leadership start (controller.go:49-67) => overdue pending plans terminated on the first pass. | S10 (TestOverdueBoundaryFiresOnLeadershipStart pending fixture) |
| Health repair on a pending plan whose exporter re-read says active (impossible by construction) | repair.go activeAtSource re-read is unchanged; phase==pending never reaches repair (repair.go:29). | S10 |
| Notification fan-out cost | GetAllUsers + cached GrantsForUser per user inside async.Dispatch; O(users) per request; accepted at MVP scale with a ponytail comment naming the approver-set cache as the upgrade. | S11 |

## Phases

### P0 — Phase 0 — Contract: CRD, data, rest (shared modules; user releases)

**Goal:** Persisted shape and DTOs exist and are pruning-safe before any service writes them; everything optional and backward compatible.

**Steps:** S1, S2, S3

**Acceptance criteria:**
- `helm template charts/telark-crds` (run through `rtk proxy`) renders protectionplan.yaml with phase enum [draft, scheduled, active, terminated, canceled, failed, pending_approval], optional approvalMode enum [automatic, required] and optional approval object (state enum, requestedBy, requestedAt date-time, decidedBy, decidedAt date-time, comment maxLength 500, history maxItems 20); spec.required is byte-identical to today.
- internal/data: `go test ./...` and `golangci-lint run` green; spec_test proves absent approvalMode/approval round-trip to ""/nil and are omitted from JSON, and explicit 'automatic' survives.
- internal/rest: `go test ./...` and `golangci-lint run` green; payload_test proves Create with Approval nil emits no key and with Approval set emits the nested object with history.
- Every new Go json name has a matching CRD property with equal-or-tighter bounds (reviewer diffs yaml vs struct).
- User-owned gate (not done here): tag internal/data and internal/rest, bump pins in services/{discovery,exporter,notifier,auth}/go.mod; CI GOWORK=off green. Workspace builds via go.work do not wait for this.

### P1 — Phase 1 — Exporter: Internal-only lifecycle/approval keys

**Goal:** Only discovery (Internal) can write phase/approval/lifecycle keys and the material keys (policies, scope, mode, timeMode, timeRange); a user session PATCH or CREATE carrying them is refused.

**Steps:** S5

**Acceptance criteria:**
- services/exporter: `go test ./...` and `golangci-lint run` clean; tests/authz/requirements_test.go (incl. TestProtectionPlansUseTheirOwnScope) untouched and green.
- guard_plan_test: a session identity holding Write + editprotectionplan gets 403 on {phase}, {approval:{...}}, {approvalMode}, {reason}, {healthDetail}, {mode}, {timeRange}, {policies}; returns true on {name, description, severity, priority, participantsIDs, environmentID, tagIDs}; Internal identity passes every key; missing identity => 403.
- Manual (user-run on dev cluster): `curl -X PATCH .../api/exporter/v1/plans/protection/<id>/patch` with a session token and {"phase":"active"} => 403; with {"mode":"enforce"} => 403; with {"description":"x"} => 200.

### P2 — Phase 2 — Discovery: gate, decide route, controller expiry, notifications

**Goal:** No policy reaches the cluster for a required plan without a decide=approved call by a distinct approver; pending plans expire safely; approvers and requesters are notified.

**Steps:** S6, S10, S11, S7, S8, S9

**Acceptance criteria:**
- services/discovery: `go test ./...` and `golangci-lint run` zero issues; every existing test passes unchanged except the planctl fake extended in S10.
- tests/authz: TestRequirementsCoverEveryRoute / TestNoRequirementWithoutRoute / TestDestructiveRoutesRequireWriteAccess pass with the decide route; a new assertion proves decide is Denyable with MinLevel Owner and Rule protection-plans.approveprotectionplan.
- tests/planapproval: ResolveApprovalMode(nil, Production)=required, (nil, Staging)=automatic, ("automatic", Production)=automatic; InitialPhase parks required plans with pending approval and no startedAt; BuildApprovePatch active sets startedAt/By=approver + renderedPolicies + approval approved + history [requested, approved]; scheduled nulls startedAt/By; BuildRejectPatch phase canceled, renderedPolicies [], reason=comment, approval rejected; BuildPendingPatch nulls decided*/reason/terminated*/started* and keeps history; ApprovalPatchValue always emits all seven keys with history as the full capped array; AppendApprovalEvent caps at 20 dropping oldest; ValidateDecision returns ErrApprovalNotPending / ErrApprovalStale / ErrApprovalSelfDecision / ErrApprovalCommentRequired in the right cases; Activatable false for pending, canceled, and scheduled+required+pending.
- tests/planctl: pending never activated at startAt; pending terminated at endAt via Terminate with the timer armed on endAt; overdue pending terminated on leadership start.
- tests/planhealth: Check on a non-active plan performs no exporter PATCH.
- tests/planupdate: MaterialChange true for policies/targets/mode/timeRange and false for name/description/severity/priority/participants/environment/tags; approved required + material => ErrApprovedPlanMaterialEdit; effective-mode immutability ('' == automatic).
- tests/planduplicate: approval never copied; approvalMode copied unless approvalMode is overridden or the overridden environmentID differs from the source's (re-sent unchanged environmentID keeps it).
- tests/planhandlers: StatusForErr maps ErrApprovalSelfDecision to 403, ErrApprovalNotPending/ErrApprovalStale to 409.
- tests/planapproval/notify_test: requester excluded; users without Own or with the deny rule excluded; decided goes to the requester only; emitter errors never panic.
- Manual on dev cluster (user-run): create a plan in Production as a Contributor => phase pending_approval, `kubectl get policies -l <plan label>=<id>` empty; approve as another Owner with the right requestedAt => active + policies present + approval.decidedBy set; approve as requester => 403; stale requestedAt => 409; reject with comment => canceled + reason=comment + approval.state=rejected; cancel while pending => canceled; time_range pending past endAt => terminated within one tick; approver's bell shows plan.approval.requested, requester's shows plan.approval.decided.

### P3 — Phase 3 — dashboard-ui

**Goal:** Requester and approver flows within house patterns; permission, self-decision and empty states correct; check-all zero errors.

**Steps:** U1, U2, U3, U4, U5

**Acceptance criteria:**
- `npm run check-all` zero errors, no eslint-disable, no any, no hex, no console.*, no vendor names; every new string lives in constants/protectionPlans.ts or notifications/constants.
- User screenshots: create form with Production selected (Execution preselects 'Requires approval' with explanation); pending card + details header ('Awaiting approval · requested <ago>'); approver toolbar with Approve (primary) + Reject; requester view with both disabled + tooltip; non-approver view with neither; reject modal refusing OK until a comment is typed; details Overview with Execution / Requested by / Approved-or-Rejected by / Decision comment rows; 'Pending approval' pill with a correct count; bell item opening the plan details.
- Editing an approved required plan shows the material sections locked with the explanation Alert; metadata edits save.
- No key removed from PlanPhase or any Record<PlanPhase> map (older-bundle compatibility).

### P4 — Phase 4 — Docs, verification, review

**Goal:** Docs reflect every change in the same change set; each module/service passes its gates; an adversarial review signs off.

**Steps:** D1, V1, V2, V3, V4, V5

**Acceptance criteria:**
- docs/CRDS.md row, services/discovery/README.md (Protect bullet + API section), services/exporter/README.md (API section), charts/telark-crds/README.md and charts/telark/README.md (upgrade order) all mention the new phase, fields, route, permission and ordering.
- V1-V4 commands exit 0 with zero lint issues and zero check-all errors.
- V5 review reports no open finding of severity high; every finding is either fixed in place or recorded as a user-owned follow-up.

## Steps (execution order: CRD before Go; shared modules before consumers; exporter before discovery client use; discovery gate before UI)

### S1 — CRD: pending_approval phase + approvalMode + approval (before any Go field)

- **Phase:** P0 · **Repo:** telark/charts · **dependsOn:** — · **parallelGroup:** contract
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M1, M2 — CRD schema is a contract consumed by exporter, discovery and the UI, and it changes what is persisted; strict structural schema prunes silently if wrong.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/charts/telark-crds/templates/crds/plans/protectionplan.yaml`
- **Change:** In spec.properties: (1) phase.enum (line 157) becomes [draft, scheduled, active, terminated, canceled, failed, pending_approval]. (2) Add `approvalMode: {type: string, enum: [automatic, required], description: "Whether the plan deploys without approval; absent means automatic"}`. (3) Add `approval: {type: object, nullable: true, required: [state, requestedBy, requestedAt], properties: {state: {type: string, enum: [pending, approved, rejected]}, requestedBy: {type: string, maxLength: 64}, requestedAt: {type: string, format: date-time}, decidedBy: {type: string, maxLength: 64, nullable: true}, decidedAt: {type: string, format: date-time, nullable: true}, comment: {type: string, maxLength: 500, nullable: true}, history: {type: array, maxItems: 20, items: {type: object, required: [event, by, at], properties: {event: {type: string, enum: [requested, approved, rejected]}, by: {type: string, maxLength: 64}, at: {type: string, format: date-time}, comment: {type: string, maxLength: 500}}}}}}`. (4) Optional printer column `- name: Approval, type: string, jsonPath: .spec.approval.state` after Phase. Do NOT touch spec.required. Place the new properties after tagIDs, before health.
- **Test first:** `/Users/houssem/Desktop/Github/telark/charts/telark-crds/templates/crds/plans/protectionplan.yaml` — rtk proxy "helm template charts/telark-crds" | grep -c pending_approval, rtk proxy "helm template charts/telark-crds" | grep -A3 'approvalMode:'
  - red: grep count is 0 and approvalMode is absent
  - green: pending_approval appears once in the phase enum; approvalMode and approval render with the bounds above; `helm lint charts/telark-crds` passes; diff of the `required:` block is empty
- **Verify:** git diff charts/telark-crds shows only added optional properties, one enum value and one printer column; helm lint clean.

### S2 — internal/data: phase/approval constants, approval types, approve action

- **Phase:** P0 · **Repo:** internal/data · **dependsOn:** — · **parallelGroup:** contract
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M1 — Exported API of a shared package consumed by four services and the UI contract.
- **Files:**
  - `/Users/houssem/Desktop/Github/internal/data/plans/protectionplan.go`
  - `/Users/houssem/Desktop/Github/internal/data/resources/role/rules.go`
  - `/Users/houssem/Desktop/Github/internal/data/tests/serialization/spec_test.go`
- **Change:** plans/protectionplan.go (same file, one types file per package): constants `PhasePendingApproval = "pending_approval"`, `ApprovalModeAutomatic = "automatic"`, `ApprovalModeRequired = "required"`, `ApprovalStatePending/Approved/Rejected = "pending"/"approved"/"rejected"`, `ApprovalEventRequested/Approved/Rejected = "requested"/"approved"/"rejected"`, `ApprovalHistoryMax = 20`. Types: `type ProtectionPlanApprovalEvent struct { Event string `json:"event"`; By string `json:"by"`; At string `json:"at"`; Comment *string `json:"comment,omitempty"` }` and `type ProtectionPlanApproval struct { State string `json:"state"`; RequestedBy string `json:"requestedBy"`; RequestedAt string `json:"requestedAt"`; DecidedBy *string `json:"decidedBy,omitempty"`; DecidedAt *string `json:"decidedAt,omitempty"`; Comment *string `json:"comment,omitempty"`; History []ProtectionPlanApprovalEvent `json:"history,omitempty"` }`. ProtectionPlan gains, after TagIDs (line 79): `ApprovalMode string `json:"approvalMode,omitempty"`` and `Approval *ProtectionPlanApproval `json:"approval,omitempty"``. resources/role/rules.go: add `ActionApproveProtectionPlan = "approveprotectionplan"` to the protection-plan action block (lines 54-62). No other file.
- **Test first:** `/Users/houssem/Desktop/Github/internal/data/tests/serialization/spec_test.go` — TestProtectionPlanApprovalOmittedWhenAbsent, TestProtectionPlanApprovalRoundTrip, TestProtectionPlanApprovalModeExplicitAutomaticSurvives
  - red: compile error: undefined plans.ProtectionPlanApproval / PhasePendingApproval / ApprovalModeRequired
  - green: JSON of a plan with no approval has neither `approvalMode` nor `approval` keys; a plan with approval {pending, history[2]} round-trips with history order preserved; ApprovalMode "automatic" is present in JSON
- **Verify:** cd /Users/houssem/Desktop/Github/internal/data && go test ./... && golangci-lint run

### S3 — internal/rest: decide endpoint, DTO fields, notification types

- **Phase:** P0 · **Repo:** internal/rest · **dependsOn:** — · **parallelGroup:** contract
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M1 — Route contract and DTOs of a shared package consumed by discovery, exporter and the UI.
- **Files:**
  - `/Users/houssem/Desktop/Github/internal/rest/endpoints/plans/def.go`
  - `/Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go`
  - `/Users/houssem/Desktop/Github/internal/rest/clients/notifications/types.go`
  - `/Users/houssem/Desktop/Github/internal/rest/tests/unit/mappers/payload_test.go`
- **Change:** endpoints/plans/def.go: add `DecideProtectionPlan base.Endpoint = "plans/protection/{id}/decide"` in the Discovery block. endpoints/plans/types.go: (a) new `ApprovalEventRequest{Event, By, At string; Comment *string omitempty}` and `ApprovalRequest{State, RequestedBy, RequestedAt string; DecidedBy, DecidedAt, Comment *string omitempty; History []ApprovalEventRequest omitempty}` mirroring data (same json names, rest-local like HealthDetailRequest); (b) CreateProtectionPlanRequest gains `ApprovalMode string `json:"approvalMode,omitempty"`` and `Approval *ApprovalRequest `json:"approval,omitempty"`` after TagIDs; (c) PrepareProtectionPlanRequest and DuplicateProtectionPlanRequest gain `ApprovalMode *string `json:"approvalMode,omitempty"``; (d) new `DecideProtectionPlanRequest{Decision string `json:"decision"`; Comment *string `json:"comment,omitempty"`; RequestedAt string `json:"requestedAt"`}`. PatchProtectionPlanRequest is NOT extended (approval is only ever written via PatchRaw; approvalMode is immutable). clients/notifications/types.go: `TypePlanApprovalRequested = "plan.approval.requested"`, `TypePlanApprovalDecided = "plan.approval.decided"` in the Type block; `MetaKeyPlanID = "planId"`, `MetaKeyPlanName = "planName"`, `MetaKeyDecision = "decision"` in the MetaKey block. No rest client method (decide is served by discovery to the UI; no Go consumer).
- **Test first:** `/Users/houssem/Desktop/Github/internal/rest/tests/unit/mappers/payload_test.go` — TestCreatePlanPayloadOmitsNilApproval, TestCreatePlanPayloadEmitsNestedApproval
  - red: compile error: CreateProtectionPlanRequest has no field Approval
  - green: MapToJSONPayload(Create{}) has no approval/approvalMode keys; with Approval set the map has approval.state, approval.requestedBy and approval.history as a []any of length 1
- **Verify:** cd /Users/houssem/Desktop/Github/internal/rest && go test ./... && golangci-lint run

### S5 — Exporter: GuardPlanLifecycle on plan create/patch + notification type mirror

- **Phase:** P1 · **Repo:** telark/exporter · **dependsOn:** — · **parallelGroup:** exporter
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M3 — Authz enforcement on a user-reachable write path; closes a self-approval bypass.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/guard.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/plans.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/authz.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/handlers/plans/protection/handler.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/types/notifications/types.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/guard_plan_test.go`
- **Change:** constants/plans.go: `var PlanLifecycleFields = []string{"approvalMode", "approval", "phase", "renderedPolicies", "startedAt", "startedBy", "terminatedAt", "terminatedBy", "reason", "health", "healthCheckedAt", "healthDetail", "policies", "scope", "mode", "timeMode", "timeRange"}` (material keys included: the UI writes them only through discovery, and a direct exporter PATCH would skip S9's update gate). constants/authz.go: `ErrAuthzPlanLifecycleDenied = "lifecycle, approval and policy fields of a protection plan are managed by the platform"` beside ErrAuthzGlobalConfigDenied (line 23). authz/guard.go: `func GuardPlanLifecycle(w http.ResponseWriter, r *http.Request, body map[string]any) bool` shaped like GuardGlobalConfigPatch (lines 253-288): if no PlanLifecycleFields key is present in body => true; identity missing => denyForbidden(ErrAuthzIdentityMissing) false; identity.Internal => true; else denyForbidden(ErrAuthzPlanLifecycleDenied) false. handlers/plans/protection/handler.go: in CreatePlan call `if !authz.GuardPlanLifecycle(w, r, body) { return }` right after GetSpec (before ExtractStructFromBody); in PatchPlanByID call it right after GetSpec (before ValidatePatchScope). types/notifications/types.go: add `TypePlanApprovalRequested = "plan.approval.requested"` and `TypePlanApprovalDecided = "plan.approval.decided"` (mirror of rest; validator does not enumerate types). Route requirements untouched.
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/guard_plan_test.go` — TestGuardPlanLifecycleSessionDeniedOnLifecycleKeys, TestGuardPlanLifecycleSessionDeniedOnMaterialKeys, TestGuardPlanLifecyclePassesPlainFields, TestGuardPlanLifecycleInternalPasses, TestGuardPlanLifecycleMissingIdentity
  - red: compile error: authz.GuardPlanLifecycle undefined
  - green: session identity with Contributor on protection-plans (requestAs pattern from guard_test.go:20-23) => false + 403 for each of {phase}, {approval:{state:approved}}, {approvalMode}, {reason}, {healthDetail:[]}, {mode}, {timeRange:{...}}, {policies:[]}; true for {name, description, severity, priority, participantsIDs, environmentID, tagIDs}; Internal => true on every key; no identity + {phase} => false + 403
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./... && golangci-lint run

### S6 — Discovery core: approval helpers, patch builders, decision validation, ApprovePlanRequirement

- **Phase:** P2 · **Repo:** telark/discovery · **dependsOn:** S2, S3 · **parallelGroup:** discovery-a
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M2, M3 — Defines the persisted approval state shape written to the CR and the decision/SoD rules on the enforcement path.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/approval.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/constants.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/authz/requirements.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planapproval/approval_test.go`
- **Change:** NEW core/plans/protection/approval.go (package protection, pure functions, no I/O): `ResolveApprovalMode(req *string, environmentID string) string` (non-nil req wins; else required iff environmentID == dataconstants.CategoryIDEnvProduction; else automatic); `RequiresApproval(plan *plans.ProtectionPlan) bool` (== ApprovalModeRequired); `EffectiveApprovalMode(mode string) string` ("" => automatic); `NewApprovalRequest(requestedBy, now string, previous *plans.ProtectionPlanApproval) *plans.ProtectionPlanApproval` (state pending, decided* nil, history = previous.History + requested event, capped); `AppendApprovalEvent(history []ProtectionPlanApprovalEvent, event, by, at string, comment *string) []ProtectionPlanApprovalEvent` dropping oldest beyond plans.ApprovalHistoryMax; `ApprovalPatchValue(a *plans.ProtectionPlanApproval) map[string]any` with ALL seven keys present (state, requestedBy, requestedAt, decidedBy, decidedAt, comment, history; nil for absent decidedBy/decidedAt/comment; history always the full capped array because merge patch replaces arrays) so merge patch clears stale keys and every patch persists the audit trail; `InitialPhase(plan *plans.ProtectionPlan, now time.Time)` (moved from assignInitialPhase, exported for tests: required => phase pending_approval + Approval=NewApprovalRequest(createdBy) and return; else today's scheduled/active logic); `BuildPendingPatch(plan, requestedBy, now string) map[string]any` (phase pending_approval, reason nil, renderedPolicies [], startedAt/By nil, terminatedAt/By nil, lastUpdated*, health unknown, approval = ApprovalPatchValue(NewApprovalRequest(...))); `BuildApprovePatch(plan, targetPhase, approver string, comment *string, now string) map[string]any` (= buildReactivatePatch shape with phase targetPhase, startedAt/By=approver only when active, approval {approved, decidedBy/At, comment, history+=approved}); `BuildRejectPatch(plan, approver, comment, now string) map[string]any` (= BuildCancelPatch(approver, comment, now) + approval {rejected, decidedBy/At, comment, history+=rejected}); `Activatable(current *plans.ProtectionPlan) bool` (phase scheduled && (!RequiresApproval || (Approval != nil && State == approved))); `ValidateDecision(plan *plans.ProtectionPlan, deciderID string, req planseps.DecideProtectionPlanRequest) error` (phase != pending_approval || Approval nil => ErrApprovalNotPending; req.RequestedAt == "" => validation.Invalid(ErrApprovalRequestedAtRequired); != Approval.RequestedAt => ErrApprovalStale; deciderID == Approval.RequestedBy => ErrApprovalSelfDecision; Decision not in {approved, rejected} => validation.Invalid; rejected && (Comment nil || empty) => validation.Invalid(ErrApprovalCommentRequired); comment > 500 runes => validation.Invalid(ErrApprovalCommentTooLong)). constants.go: `FieldApprovalMode = "approvalMode"`, `FieldApproval = "approval"`, `FieldApprovalState/RequestedBy/RequestedAt/DecidedBy/DecidedAt/Comment/History`, `DecisionApproved = plans.ApprovalStateApproved`, `DecisionRejected = plans.ApprovalStateRejected`, `ApprovalCommentMax = 500`, `LogPlanApproved/LogPlanRejected/LogPlanParked/LogActivateSkippedStale` formats, and typed errors `ErrApprovalNotPending`, `ErrApprovalStale`, `ErrApprovalSelfDecision`, `ErrApprovalCommentRequired`, `ErrApprovalCommentTooLong`, `ErrApprovalRequestedAtRequired`, `ErrApprovalInvalidDecision`, `ErrApprovalModeInvalid`, `ErrApprovalModeImmutable`, `ErrApprovedPlanMaterialEdit`, `ErrPlanDecisionInFlight` as errors.Error. authz/requirements.go: `func ApprovePlanRequirement() authz.Requirement { return authz.Denyable(authz.Own(roledata.ScopeProtectionPlans), roledata.ActionApproveProtectionPlan) }` (used by S8 route registration, S7 handlers and S11 notifier; not yet registered on a route here so coverage tests stay green).
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planapproval/approval_test.go` — TestResolveApprovalMode, TestInitialPhaseParksRequiredPlans, TestBuildApprovePatchActiveAndScheduled, TestBuildRejectPatch, TestBuildPendingPatchClearsDecision, TestApprovalPatchValueAlwaysSevenKeys, TestAppendApprovalEventCaps, TestActivatable, TestValidateDecision
  - red: compile error: protection.ResolveApprovalMode undefined
  - green: table cases as listed in P2 acceptance; BuildPendingPatch has explicit nil for reason/terminatedAt/terminatedBy/startedAt/startedBy and approval.decidedBy == nil; ApprovalPatchValue emits exactly seven keys and its history has the same length as the input history; AppendApprovalEvent with 21 entries keeps the newest 20; ValidateDecision returns each typed error for its case
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planapproval/... ./internal/tests/authz/... && golangci-lint run

### S10 — Discovery controller: expire pending plans at endAt; health.Check stops patching non-active plans

- **Phase:** P2 · **Repo:** telark/discovery · **dependsOn:** S2 · **parallelGroup:** discovery-a
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M3, M4 — Changes when the enforcement path terminates a plan and the runtime behaviour of the leader controller and status route.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/controllers/plans/protection/controller.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/health/check.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planctl/boundary_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planhealth/health_test.go`
- **Change:** controller.go: awaitsBoundary (line 128) returns true also for plans.PhasePendingApproval; transition (line 132) gains `case plans.PhasePendingApproval: if shouldTerminate(plan, now) { Terminate; return true }` (log error as the active case does). No activation for pending. PlanService interface and Terminate signature untouched. health/check.go Check (line 34): after ComputeAndRepair, `if plan.Phase != plans.PhaseActive { plan.Health = plans.HealthUnknown; return plan, result, nil }` BEFORE the PatchOrError at line 49 (every non-active transition already stores health=unknown; the patch only churned lastUpdatedAt/By=system). Add one comment naming the churn as the why.
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planctl/boundary_test.go` — TestPendingIsNotActivatedAtStartAt, TestPendingTerminatesAtEndAt, TestOverdueBoundaryFiresOnLeadershipStart (pending fixture added), TestCheckSkipsPatchForNonActive (planhealth/health_test.go)
  - red: pending plan with startAt in the past is never activated (already true) but a pending plan with endAt in the past is never terminated (fails); Check on a canceled plan records one exporter PATCH (fails)
  - green: fake service records exactly one terminate event for the pending plan within maxLag of endAt and zero activate events; Check on non-active plans records zero PATCH calls and returns health unknown
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planctl/... ./internal/tests/planhealth/... && golangci-lint run

### S11 — Discovery notifications: ApprovalNotifier (approver fan-out + requester decision)

- **Phase:** P2 · **Repo:** telark/discovery · **dependsOn:** S3, S6 · **parallelGroup:** discovery-b
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M3, M4 — Recipient selection is an authz evaluation (who may approve) and it changes the behaviour of discovery and the exporter notification store together.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/notify.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/clients/authz.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/constants/messages.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planapproval/notify_test.go`
- **Change:** clients/authz.go: add `func (c *AuthzClient) GetAllUsers() ([]*userresource.UserAsResource, error)` via guardedExporterGet over c.users.GetAllUsers (rest users client.go:57). NEW core/plans/protection/notify.go (package protection): `type ApprovalNotifier struct { ListUsers func() ([]*userresource.UserAsResource, error); Grants func(userID string) (xauthz.Grants, error); Emit func(ctx context.Context, n notifclient.Notification) error; Requirement xauthz.Requirement; Logger Logger }` with nil-receiver-safe methods `RequestApproval(plan *plans.ProtectionPlan)` and `Decided(plan *plans.ProtectionPlan, decision string, comment *string)`. RequestApproval: async.Dispatch(func(ctx){ users := ListUsers(); for each user != plan.Approval.RequestedBy: grants := Grants(user.ID); if xauthz.Allows(xauthz.Identity{UserID: user.ID, Grants: grants}, Requirement) => Emit{UserID, Type: TypePlanApprovalRequested, Title: NotifPlanApprovalRequestedTitle, Message: fmt(NotifPlanApprovalRequestedFormat, plan.Name, requestedBy), Severity info, Metadata{targetId: plan.ID, planId, planName}} }); errors logged, never returned; `// ponytail: O(users) grant lookups per request through the cached resolver; add an approver-set cache when user counts grow`. Decided: one Emit to plan.Approval.RequestedBy with Type TypePlanApprovalDecided, Severity success (approved) / warning (rejected), Message fmt(NotifPlanApprovalDecidedFormat, plan.Name, decision, comment-or-empty), Metadata{targetId: plan.ID, planId, planName, decision}. constants/messages.go: `NotifPlanApprovalRequestedTitle = "Protection plan awaiting approval"`, `NotifPlanApprovalRequestedFormat = "**%s** was submitted for approval by %s."`, `NotifPlanApprovalDecidedTitle = "Protection plan decision"`, `NotifPlanApprovalDecidedFormat = "**%s** was %s. %s"` beside the rollback Notif* constants (lines 65-71). Wiring into Service happens in S7.
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planapproval/notify_test.go` — TestRequestApprovalNotifiesOnlyEligibleApprovers, TestRequestApprovalExcludesRequester, TestDecidedNotifiesRequesterOnly, TestNotifierNilReceiverIsNoop, TestNotifierSwallowsEmitErrors
  - red: compile error: protection.ApprovalNotifier undefined
  - green: with fake ListUsers [requester, owner, contributor, deniedOwner] and fake Grants, RequestApproval emits exactly one notification (owner) after async.Drain; Decided emits one to the requester with decision in metadata; nil notifier methods return without panic; an Emit error does not surface
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planapproval/... && golangci-lint run

### S7 — Discovery service gate: Prepare/Duplicate/Reactivate park, Decide, Activate re-read, notifier wiring, validation, duplicate copy rule

- **Phase:** P2 · **Repo:** telark/discovery · **dependsOn:** S6, S11 · **parallelGroup:** discovery-c
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M3, M4 — The approval gate on the policy deployment path across discovery and exporter; mistakes here deploy unapproved policies.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/svc.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/wire.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/validation/validation.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/duplicate/duplicate.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planduplicate/duplicate_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planvalidation/validation_test.go`
- **Change:** svc.go: Service gains field `notifier *ApprovalNotifier` and accessor `Notifier()`; NewService gains a trailing `notifier *ApprovalNotifier` param (tests may pass nil). wire.go BuildService constructs `&ApprovalNotifier{ListUsers: clients.NewAuthzClient().GetAllUsers, Grants: discoveryauthz.NewResolver().GrantsForUser, Emit: clients.NewNotificationClient().Emit, Requirement: discoveryauthz.ApprovePlanRequirement(), Logger: logger}`. Prepare signature unchanged: buildPlan sets `plan.ApprovalMode = ResolveApprovalMode(req.ApprovalMode, plan.EnvironmentID)` (no approver gate: Write suffices, cut in r1); replace assignInitialPhase with InitialPhase(plan, s.clock()); after CreateOrError: `if plan.Phase == pending_approval { s.notifier.RequestApproval(plan) }`. toCreateRequest forwards ApprovalMode and Approval (map data struct -> ApprovalRequest, incl. history). Duplicate unchanged apart from BuildRequest below. Reactivate: after the expired check and before renderAndDeploy, `if RequiresApproval(plan) { PatchRawOrError(userID, planID, BuildPendingPatch(plan, userID, now)); s.notifier.RequestApproval(re-read plan); return s.exporter.Get(planID) }` (no deploy). cancellable gains PhasePendingApproval. New `Decide(ctx, userID, planID string, req DecideProtectionPlanRequest) (*plans.ProtectionPlan, error)`: Get; ValidateDecision; rejected => CleanupByPlanID (logged), PatchRawOrError(BuildRejectPatch), notifier.Decided, return Get; approved => NamespaceScope + resolveScopeForPlan + validateTemplatesForPlan (as Reactivate), computeReactivatePhase (expired => validation.Invalid(ErrReactivateExpired)), if active renderAndDeploy then re-read `current := Get; if current.Phase != pending_approval || current.Approval == nil { CleanupByPlanID; return ErrApprovalNotPending }; if current.Approval.RequestedAt != req.RequestedAt { CleanupByPlanID; return ErrApprovalStale }` (the update handler holds the same lock, so this is defence in depth), PatchRawOrError(BuildApprovePatch(...)) with CleanupByPlanID on failure when active, StampFirstHealth when active, notifier.Decided, return Get. Activate (line 317): first `current, err := s.exporter.Get(plan.ID); if err != nil { return err }; if !Activatable(current) { log LogActivateSkippedStale; return nil }` then continue with `current`. validation/validation.go Fields: `if req.ApprovalMode != nil && *req.ApprovalMode not in {automatic, required} => Invalid(ErrApprovalModeInvalid)`. duplicate/duplicate.go BuildRequest: `if overrides.ApprovalMode != nil { req.ApprovalMode = overrides.ApprovalMode } else if source.ApprovalMode != "" && (overrides.EnvironmentID == nil || *overrides.EnvironmentID == source.EnvironmentID) { req.ApprovalMode = &source.ApprovalMode }` (else nil => Prepare derives; 'environmentID sent' is not 'environment changed' because DuplicatePlanPanel.tsx:129-132 sends environmentID whenever tags are touched).
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planduplicate/duplicate_test.go` — TestBuildRequestCopiesApprovalModeWhenNothingOverridden, TestBuildRequestDropsApprovalModeWhenEnvironmentChanged, TestBuildRequestKeepsApprovalModeWhenEnvironmentResent, TestBuildRequestApprovalModeOverrideWins, TestBuildRequestNeverCarriesApprovalState, TestFieldsRejectsUnknownApprovalMode (planvalidation/validation_test.go)
  - red: compile error: DuplicateProtectionPlanRequest has no field ApprovalMode (before S3 pins) / assertions fail because BuildRequest ignores approvalMode
  - green: each case returns the expected *string (nil when the environment changed, copied when unchanged or merely re-sent, override); the PrepareProtectionPlanRequest type has no approval field (compile-time); Fields returns a validation error for approvalMode "manual"
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go build ./... && go test ./... && golangci-lint run

### S8 — Discovery route, authz requirement, decide handler, per-plan lock (decide + cancel; S9 reuses it for update), 403/409 mapping

- **Phase:** P2 · **Repo:** telark/discovery · **dependsOn:** S7 · **parallelGroup:** discovery-d
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M1, M3 — New route contract with its authz requirement and cross-replica locking.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/routes/routes.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/authz/requirements.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/decide.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/lock.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/init.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/cancel.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/errors.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/constants/config.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/main.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/authz/requirements_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planhandlers/errors_test.go`
- **Change:** routes.go: `router.CreateRoute(base.Post, planseps.DecideProtectionPlan, protectionplanhandler.Decide)` after Update. authz/requirements.go addPlans: `r[router.Key(base.Post, planseps.DecideProtectionPlan)] = ApprovePlanRequirement()`. constants/config.go: `KeyPrefixLockPlanDecision = "lock:plan-decision:"` beside DefaultLockTTL (line 35). handlers init.go: `SetCoordinationBundle(bundle *coordination.CoordinationBundle)` + getter (copy of applications/coordination.go:17-29, no replica id needed); main.go: call `protectionhandler.SetCoordinationBundle(coord)` next to line 281. NEW handlers lock.go: `lockPlanDecision(ctx, w, planID) (release func(), ok bool)` = copy of applications/rollback.go:36-56 (coord.Lock.Acquire(ctx, KeyPrefixLockPlanDecision+planID, uuid, constants.DefaultLockTTL); no bundle => sync.Map local mutex fallback; not acquired => 409 with protection.ErrPlanDecisionInFlight; coordination error => 503). NEW handlers decide.go = copy of cancel.go: X-User-ID (401 if empty), path id, decode DecideProtectionPlanRequest with json.NewDecoder (body required: 400 on error/EOF), lockPlanDecision, ctx with constants.ProtectionPlanDeployTimeout, svc.Decide, respondDomainError / LogAndSendResponse 200 with the plan. cancel.go: acquire lockPlanDecision before svc.Cancel (release deferred). errors.go StatusForErr: `errors.Is(err, protection.ErrApprovalSelfDecision)` => 403; `errors.Is(err, protection.ErrApprovalNotPending) || errors.Is(err, protection.ErrApprovalStale)` => 409 (cases before the validation case).
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/authz/requirements_test.go` — TestDecideRouteIsDenyableOwner, TestRequirementsCoverEveryRoute (existing, must stay green), TestStatusForErr (planhandlers/errors_test.go: add forbidden and conflict cases)
  - red: TestRequirementsCoverEveryRoute reports 'POST plans/protection/{id}/decide' missing once the route is registered without a requirement; TestDecideRouteIsDenyableOwner fails on MinLevel/Rule; errors_test expects 403/409 and gets 500
  - green: requirement for the decide key has Scope protection-plans, MinLevel Owner, Rule protection-plans.approveprotectionplan; StatusForErr returns 403 for ErrApprovalSelfDecision, 409 for ErrApprovalNotPending and ErrApprovalStale; TestDestructiveRoutesRequireWriteAccess passes (Own >= Write)
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go build ./... && go test ./... && golangci-lint run

### S9 — Discovery update gate: material-change predicate, approved-plan refusal, pending re-request, approvalMode immutability

- **Phase:** P2 · **Repo:** telark/discovery · **dependsOn:** S7, S8 · **parallelGroup:** discovery-d
- **Route:** claude-fable-5-1 / effort default / P3 — criteria M3 — Without this gate a Contributor deploys unapproved policies through an approved shell via update on an active plan.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/update/orchestrator.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/update/patch.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/update.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planupdate/material_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planupdate/patch_test.go`
- **Change:** update/patch.go: exported `MaterialChange(plan *plans.ProtectionPlan, req *PrepareProtectionPlanRequest, newPolicies []plans.ProtectionPlanPolicy) bool` = !planPoliciesEqual || !scopeTargetsEqual || plan.Mode != req.Mode || plan.TimeMode != req.TimeMode || !timeRangeEqual (reuses the existing private comparators, placed beside them so they cannot drift). update/orchestrator.go: updatable gains PhasePendingApproval; validateRequest adds `if req.ApprovalMode != nil && protection.EffectiveApprovalMode(*req.ApprovalMode) != protection.EffectiveApprovalMode(plan.ApprovalMode) => validation.Invalid(ErrApprovalModeImmutable)`; in Run after validateRequest and newPolicies: `material := MaterialChange(...)`; `if protection.RequiresApproval(plan) && (plan.Phase == active || plan.Phase == scheduled) && material => return validation.Invalid(ErrApprovedPlanMaterialEdit)` BEFORE resolveTargets/ApplyClusterDiff; after BuildPatch, when `plan.Phase == pending_approval && material`: `body, err := restmapper.MapToJSONPayload(patch); body[FieldApproval] = protection.ApprovalPatchValue(protection.NewApprovalRequest(userID, now, plan.Approval)); deps.Exporter.PatchRawOrError(userID, planID, body)` (single patch) and then `deps.NotifyApprovers(updated)`; otherwise today's PatchOrError. Deps gains `NotifyApprovers func(*plans.ProtectionPlan)` (nil-safe). handlers update.go: acquire `lockPlanDecision(ctx, w, planID)` (S8 helper) before update.Run with the release deferred, so a material edit can never interleave with decide/cancel (409 ErrPlanDecisionInFlight on contention; Run's initial Get is then the authoritative read); buildUpdateDeps: `NotifyApprovers: svc.Notifier().RequestApproval`.
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planupdate/material_test.go` — TestMaterialChangeMatrix, TestApprovalModeImmutableComparesEffectiveModes, TestBuildPatchNeverEmitsApprovalKeys (patch_test.go)
  - red: compile error: update.MaterialChange undefined
  - green: true for policies/targets/mode/timeMode/timeRange changes; false for name/description/severity/priority/participants/environment/tags-only; legacy plan (ApprovalMode "") + req "automatic" is not immutable-violating, + req "required" is; MapToJSONPayload(BuildPatch(...)) has no approval/approvalMode key
- **Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planupdate/... && golangci-lint run

### U1 — UI model, constants, phase rules, permissions

- **Phase:** P3 · **Repo:** dashboard-ui · **dependsOn:** S3 · **parallelGroup:** ui-a
- **Route:** claude-opus-5-5 / effort medium / P4 — criteria none — UI consumer of the contract; no M1-M5 criterion; copy and token choices need judgement, types are machine-checked.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/models/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/phaseRules.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/roles/constants/scopeRules.ts`
- **Change:** models/index.ts: PlanPhase (line 1) += 'pending_approval'; `export type PlanApprovalMode = 'automatic' | 'required'; export type PlanApprovalState = 'pending' | 'approved' | 'rejected'; export type PlanApprovalDecision = 'approved' | 'rejected'; export interface PlanApprovalEvent { event: string; by: string; at: string; comment?: string } export interface PlanApproval { state: PlanApprovalState; requestedBy: string; requestedAt: string; decidedBy?: string; decidedAt?: string; comment?: string; history?: PlanApprovalEvent[] }`; ProtectionPlan += `approvalMode?: PlanApprovalMode; approval?: PlanApproval`. constants/protectionPlans.ts: PHASE_LABELS.pending_approval = 'Pending approval'; PHASE_ACCENT.pending_approval = DEFAULT_COLORS.WARNING; PHASE_DOT_COLOR.pending_approval = DEFAULT_COLORS.WARNING (token, not hex); `PLAN_APPROVAL = { PRODUCTION_ENVIRONMENT_ID: 'cat-00002-0001-0001', COMMENT_MAX: 500 } as const` beside PLAN_TAXONOMY_LIMITS with a one-line comment 'mirrors internal/data constants.CategoryIDEnvProduction'; CREATE_PAGE.APPROVAL_MODE_OPTIONS [{value:'automatic', label:'Automatic — deploys as soon as the plan starts'}, {value:'required', label:'Requires approval — an approver must confirm before anything is deployed'}]; CREATE_PAGE.FORM: APPROVAL_MODE_LABEL 'Execution', APPROVAL_HELP_REQUIRED, APPROVAL_HELP_AUTOMATIC, APPROVAL_PRODUCTION_HINT, APPROVAL_MODE_IMMUTABLE_TOOLTIP; PANELS.EDIT.APPROVED_LOCKED_ALERT; DETAIL_PAGE.ACTIONS: APPROVE, REJECT, APPROVE_MODAL_TITLE/BODY/OK, REJECT_MODAL_TITLE/BODY/OK, DECISION_COMMENT_LABEL, DECISION_COMMENT_PLACEHOLDER, REJECT_COMMENT_REQUIRED, SELF_DECISION_TOOLTIP, DECIDE_DISABLED_EXPIRED_TOOLTIP; DETAIL_PAGE.FIELDS: EXECUTION, REQUESTED_BY, APPROVED_BY, REJECTED_BY, DECISION_COMMENT; PHASE_INFO.AWAITING_APPROVAL_PREFIX 'Awaiting approval · requested'; ACTIONS: APPROVE_SUCCESS(name), APPROVE_ERROR, REJECT_SUCCESS(name), REJECT_ERROR; EXECUTION_LABELS {automatic:'Automatic', required:'Requires approval'}. phaseRules.ts: `APPROVABLE_PHASES: PlanPhase[] = ['pending_approval']`; CANCELLABLE_PHASES += 'pending_approval'; `isSelfDecision(plan, userId) => plan.approval?.requestedBy === userId`; `isRejected(plan) => plan.phase === 'canceled' && plan.approval?.state === 'rejected'`; `isMaterialEditLocked(plan) => plan.approvalMode === 'required' && (plan.phase === 'active' || plan.phase === 'scheduled')`. permissionEngine.tsx protectionPlans += `approve: { scope: 'protection-plans', level: 'Owner', deny: 'protection-plans.approveprotectionplan.deny' }`. scopeRules.ts protection-plans Owner row += { key: 'approveprotectionplan', label: 'Approve Protection Plan' }.
- **Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/models/index.ts` — npm run check-all
  - red: after adding 'pending_approval' to PlanPhase, tsc reports every Record<PlanPhase> map (PHASE_LABELS, PHASE_ACCENT, PHASE_DOT_COLOR) and the phaseCounts literal in ProtectionPlansListPage.tsx:120-129 as missing the key
  - green: check-all zero errors once the maps and (in U5) phaseCounts carry the key; no hex literal added
- **Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all

### U2 — UI decide client, paths, endpoints, store thunk + slice

- **Phase:** P3 · **Repo:** dashboard-ui · **dependsOn:** U1 · **parallelGroup:** ui-a2
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — Fully specified copy of the reactivate client/thunk/slice and machine-checked by check-all.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/constants/rest/paths.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/constants/rest/endpoints.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/constants/store/store.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/decide.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/store/thunks/protectionPlansThunks.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/store/slices/protectionPlansSlice.ts`
- **Change:** paths.ts PLANS_PATHS.PROTECTION.DECIDE: (id) => `plans/protection/${id}/decide`; endpoints.ts Endpoints.PROTECTION_PLANS.DECIDE mirroring REACTIVATE. store.ts: STORE_ACTIONS.PROTECTION_PLANS.DECIDE 'protectionPlans/decide', STORE_ERRORS.DECIDE_PROTECTION_PLAN 'Failed to record the decision', STORE_MESSAGES.ERROR_DECIDING_PROTECTION_PLAN. NEW clients/decide.ts = copy of reactivate.ts: `decidePlan(userId, planId, body: { decision: PlanApprovalDecision; comment?: string; requestedAt: string })` POST discovery with X-User-ID, returns res.data; export from clients/index.ts. thunks: `decidePlanThunk` (copy of cancelPlanThunk with args {userId, planId, decision, comment?, requestedAt}); slice: one addCase(decidePlanThunk.fulfilled) upserting plans[] and details exactly like reactivatePlanThunk.fulfilled (slice lines 99-104).
- **Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/decide.ts` — npm run check-all
  - red: type errors: PLANS_PATHS.PROTECTION.DECIDE / STORE_ACTIONS.PROTECTION_PLANS.DECIDE undefined
  - green: check-all zero errors; decide.ts compiles against PlanApprovalDecision
- **Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all

### U3 — UI form: Execution select with environment-derived default, duplicate/edit rules, payload plumbing

- **Phase:** P3 · **Repo:** dashboard-ui · **dependsOn:** U1 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort medium / P4 — criteria none — Multi-file form plumbing with antd form-instance behaviour that a checker cannot fully verify.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/shared/PlanTaxonomyFields.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/create/types.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/planFormValues.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanFormState.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanActions.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/prepare.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/update.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/duplicate.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/shared/PlanForm.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/panels/EditPlanPanel.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/panels/DuplicatePlanPanel.tsx`
- **Change:** PlanTaxonomyFields gains prop `{ editMode?: boolean }` and a third Form.Item name='approvalMode' label=FORM.APPROVAL_MODE_LABEL rendering a Select with CREATE_PAGE.APPROVAL_MODE_OPTIONS (grid becomes '1fr 1fr 1fr'): `extra` = APPROVAL_HELP_REQUIRED / APPROVAL_HELP_AUTOMATIC by current value plus APPROVAL_PRODUCTION_HINT when environmentID === PLAN_APPROVAL.PRODUCTION_ENVIRONMENT_ID && value === 'automatic'; whole Select disabled with APPROVAL_MODE_IMMUTABLE_TOOLTIP when editMode. Derived default: `const form = Form.useFormInstance(); const env = Form.useWatch('environmentID', form); useEffect(() => { if (editMode || env === initialEnvRef.current || form.isFieldsTouched(['approvalMode'])) return; form.setFieldValue('approvalMode', env === PRODUCTION_ENVIRONMENT_ID ? 'required' : 'automatic'); }, [env, ...])` (the DuplicatePlanPanel.tsx:129 touched pattern; initialEnvRef = useRef(initial environmentID) so a seeded duplicate keeps the source mode until the user actually changes the environment). No approver gate (cut in r1). create/types.ts FormValues += `approvalMode?: PlanApprovalMode`; DEFAULT_FORM_VALUES.approvalMode 'automatic'; planToFormValues maps plan.approvalMode ?? 'automatic'; buildPreparePayload includes approvalMode; normalizeFormSnapshot includes approvalMode. PreparePlanPayload (clients/prepare.ts), update.ts payload and usePlanActions PreparePlanInput += `approvalMode?: PlanApprovalMode`; usePlanActions.handleUpdate deletes approvalMode from the payload before posting (immutable server-side; the edit form never sends it). DuplicatePlanPanel: seed the form's approvalMode from the source plan (plan.approvalMode ?? 'automatic') instead of the env-derived default, and send payload.approvalMode only under a SEPARATE `form.isFieldsTouched(['approvalMode'])` check, never inside the existing environmentID/tagIDs touched block (lines 129-132 send environmentID whenever tags are touched). EditPlanPanel/PlanForm: when isMaterialEditLocked(plan) render the existing Alert pattern (PlanForm.tsx:143-150) with PANELS.EDIT.APPROVED_LOCKED_ALERT and pass `disabled` to Scope, Policies, Schedule and mode inputs (only metadata fields stay editable).
- **Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/shared/PlanTaxonomyFields.tsx` — npm run check-all
  - red: FormValues has no approvalMode; buildPreparePayload output type mismatch with PreparePlanPayload
  - green: check-all zero errors; grep shows no console.*, no any, no hex
- **Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshot: create with Production selected shows 'Requires approval' preselected with help text; switching environment after touching the field does not override; duplicate of a required non-Production plan with only tags changed preselects 'Requires approval' and the copy parks pending; edit of an approved required plan shows locked sections

### U4 — UI approve/reject: decision modal, details toolbar + view, card menu, header + Overview rows

- **Phase:** P3 · **Repo:** dashboard-ui · **dependsOn:** U2 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort medium / P4 — criteria none — Component work with visual/interaction judgement; server remains authoritative for authz.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/shared/ApprovalDecisionModal.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlanDetailsToolbar.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/DetailsView.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx`
- **Change:** NEW ApprovalDecisionModal.tsx: wrapper over ActionConfirmModal (ReactivatePlanModal.tsx shape) with props {open, decision: PlanApprovalDecision, plan, loading, onClose, onConfirm(comment: string)}; customMessage = body copy (APPROVE_MODAL_BODY / REJECT_MODAL_BODY) + a summary line (execution mode, environment name if available, policy count, target count, window) + antd Input.TextArea (maxLength PLAN_APPROVAL.COMMENT_MAX, showCount, placeholder DECISION_COMMENT_PLACEHOLDER) held in local state and reset on open; confirm disabled (via loading prop pattern or a wrapping disabled) while decision==='rejected' && comment.trim()===''; danger = decision==='rejected'; getContainer document.body. ProtectionPlanDetailsToolbar: new props approving, rejecting, onApprove, onReject; canApprove = usePermission(approve.*); when canApprove && APPROVABLE_PHASES.includes(phase) push 'approve' (variant primary, first) and 'reject' (variant danger) before Edit; both disabled with SELF_DECISION_TOOLTIP when isSelfDecision(plan, currentUserId) (currentUserId from getCurrentUser()?.id) or DECIDE_DISABLED_EXPIRED_TOOLTIP when isReactivateExpired(plan); Reactivate never shows for pending (not in REACTIVATABLE_PHASES). DetailsView: state {decisionModal: PlanApprovalDecision | null}, handleDecide(comment) copying handleConfirmReactivate: dispatch(decidePlanThunk({userId, planId, decision, comment, requestedAt: details.approval?.requestedAt ?? ''})), success toast APPROVE_/REJECT_SUCCESS, error toast with server message (409 stale surfaces the extracted message). Content.tsx: header span for phase==='pending_approval' && plan.approval => `${PHASE_INFO.AWAITING_APPROVAL_PREFIX} <TimeAgo date={requestedAt} />`; Overview rows after 'tags': Execution (EXECUTION_LABELS[plan.approvalMode ?? 'automatic']), Requested by (renderUserAndTime(approval.requestedBy, approval.requestedAt)) when approval, Approved by / Rejected by (renderUserAndTime(decidedBy, decidedAt)) when decidedBy, Decision comment when approval.comment (reason-row style, lines 227-235); phaseLabel uses 'Rejected' label when isRejected(plan). ProtectionPlanCard: canApprovePlan permission; menu items approve/reject (permission x APPROVABLE_PHASES, disabled + tooltip for self/expired) in the reactivate pattern (lines 354-406); modal state + handlers copying handleReactivate (lines 304-317); provenance row shows 'awaiting approval' + requester when pending.
- **Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/shared/ApprovalDecisionModal.tsx` — npm run check-all
  - red: type errors: decidePlanThunk args, missing toolbar props
  - green: check-all zero errors; ApprovalDecisionModal imports only ActionConfirmModal + antd Input (no FormModal)
- **Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshots: approver toolbar (Approve primary + Reject), requester view (disabled + tooltip), non-approver view (no buttons), reject modal refusing OK without comment, rejected plan details showing 'Rejected' + comment

### U5 — UI list pill + counts, notification registry, home dashboard sanity

- **Phase:** P3 · **Repo:** dashboard-ui · **dependsOn:** U1 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — Fully specified list/registry edits, machine-checked by check-all.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlansToolbar.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/ProtectionPlansListPage.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/notifications/constants/notifications.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/notifications/utils/typeRegistry.ts`
- **Change:** ProtectionPlansToolbar PHASE_PILLS (lines 25-32): insert { key: 'pending_approval', label: PHASE_LABELS.pending_approval } after 'scheduled'; getPillAccent uses PHASE_ACCENT. ProtectionPlansListPage QUICK_FILTER_KEYS (lines 43-50) += 'pending_approval'; phaseCounts literal (lines 120-129) += pending_approval: 0. Do NOT change TOOLBAR_COMPACT_WIDTH (user-owned re-measure, see follow-ups). notifications/constants: NOTIFICATION_TYPES += PLAN_APPROVAL_REQUESTED: 'plan.approval.requested', PLAN_APPROVAL_DECIDED: 'plan.approval.decided'; typeRegistry TYPE_REGISTRY entries for both with icon SafetyCertificateOutlined (requested) / CheckCircleOutlined (decided) and navigateTo: (m) => typeof m?.planName === 'string' ? APP_ROUTES.PROTECTION_PLAN_DETAILS.replace(':name', encodeURIComponent(m.planName)) : null. Home dashboard (home/utils/summary.ts, rows.ts) needs no change: pending plans are neither active nor failed and the label falls back to PHASE_LABELS; reviewer confirms by reading.
- **Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/ProtectionPlansListPage.tsx` — npm run check-all
  - red: phaseCounts Record<PlanPhaseQuickFilter> missing 'pending_approval' (tsc error from U1's type change)
  - green: check-all zero errors; pill renders with the WARNING accent
- **Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshot of the toolbar with 7 pills at the compact breakpoint; clicking a plan.approval.* bell item lands on the plan details

### D1 — Docs: CRDS.md, service READMEs, chart READMEs (rollout order)

- **Phase:** P4 · **Repo:** docs · **dependsOn:** S8, S9, S5, U5 · **parallelGroup:** docs
- **Route:** claude-opus-5-5 / effort medium / P4 — criteria none — Documentation only; wording must match the implemented behaviour, which needs reading, not a machine check.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/docs/CRDS.md`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/README.md`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/README.md`
  - `/Users/houssem/Desktop/Github/telark/charts/telark-crds/README.md`
  - `/Users/houssem/Desktop/Github/telark/charts/telark/README.md`
- **Change:** docs/CRDS.md line 14 ProtectionPlan row: transitions become 'pending_approval → scheduled → active → terminated'; mention approvalMode (automatic|required, absent = automatic; Production default) and approval (state, requester, decider, comment, bounded history; written only by discovery). discovery README: Protect bullet (line 61) adds 'plans that require approval are parked in pending_approval until an approver decides; nothing is deployed while pending'; API section (line 167-173) adds POST plans/protection/{id}/decide {decision, comment?, requestedAt} (Owner + approveprotectionplan, requester cannot decide, 409 on stale requestedAt or concurrent decision), approvalMode on prepare/duplicate (immutable on update), material edits of approved required plans refused, pending plans expire at endAt. exporter README API section (line 83): plan create/patch refuse lifecycle/approval and material keys (policies, scope, mode, timeMode, timeRange) from session identities (list them). telark-crds README Definitions: new phase and fields. telark README: rollout note 'upgrade telark-crds before telark: the pending_approval phase and approval fields are rejected by an older CRD; older dashboard bundles show pending plans with a raw label and no Cancel (Owners can Clear)'. charts/telark VALUES.md unchanged (no new tunable).
- **Test first:** `/Users/houssem/Desktop/Github/telark/docs/CRDS.md` — grep -n pending_approval docs/CRDS.md services/discovery/README.md services/exporter/README.md charts/telark-crds/README.md charts/telark/README.md
  - red: grep finds nothing
  - green: every file listed matches; reviewer reads each section for the items above
- **Verify:** grep as above returns one or more lines per file; no version, Chart.yaml or values change.

### V1 — Verify shared modules: go test + golangci-lint (internal/data, internal/rest)

- **Phase:** P4 · **Repo:** internal/data · **dependsOn:** S2, S3 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — Machine-checked, fully specified.
- **Files:**
  - `/Users/houssem/Desktop/Github/internal/data`
  - `/Users/houssem/Desktop/Github/internal/rest`
- **Change:** Run `go test ./...` and `golangci-lint run` in both modules; fix any issue introduced by S2/S3 only.
- **Test first:** `/Users/houssem/Desktop/Github/internal/data/tests/serialization/spec_test.go` — go test ./..., golangci-lint run
  - red: n/a (verification step)
  - green: both commands exit 0 in both modules with zero lint issues
- **Verify:** exit codes 0; lint output empty

### V2 — Verify exporter: go test + golangci-lint

- **Phase:** P4 · **Repo:** telark/exporter · **dependsOn:** S5 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — Machine-checked.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/exporter`
- **Change:** Run `go test ./...` and `golangci-lint run` (no GOTOOLCHAIN prefix) in services/exporter; fix issues introduced by S5 only.
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/guard_plan_test.go` — go test ./..., golangci-lint run
  - red: n/a
  - green: exit 0, zero issues
- **Verify:** exit codes 0

### V3 — Verify discovery: go build + go test + golangci-lint

- **Phase:** P4 · **Repo:** telark/discovery · **dependsOn:** S8, S9, S10 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — Machine-checked.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery`
- **Change:** Run `go build ./...`, `go test ./...` and `golangci-lint run` in services/discovery; fix issues introduced by S6-S11 only. Note: grouped lint output hides counts; read every reported line.
- **Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planapproval/approval_test.go` — go build ./..., go test ./..., golangci-lint run
  - red: n/a
  - green: exit 0, zero issues
- **Verify:** exit codes 0

### V4 — Verify dashboard-ui: npm run check-all

- **Phase:** P4 · **Repo:** dashboard-ui · **dependsOn:** U3, U4, U5 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — Machine-checked.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui`
- **Change:** Run `npm run check-all`; fix every error (warnings OK) without eslint-disable, any, console.* or hex.
- **Test first:** `/Users/houssem/Desktop/dashboard-ui/package.json` — npm run check-all
  - red: n/a
  - green: zero errors
- **Verify:** exit code 0 with zero errors

### V5 — Adversarial review of the whole change set

- **Phase:** P4 · **Repo:** telark/discovery · **dependsOn:** V1, V2, V3, V4, D1 · **parallelGroup:** review
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Read-only adversarial review that must catch mistakes in a Fable implementation.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark`
  - `/Users/houssem/Desktop/Github/internal/data`
  - `/Users/houssem/Desktop/Github/internal/rest`
  - `/Users/houssem/Desktop/dashboard-ui`
- **Change:** Read-only review against this plan: (1) grep every call of renderAndDeploy / applier.Deploy / ApplyClusterDiff / Activate and prove each is unreachable for phase pending_approval; (2) confirm exporter GuardPlanLifecycle is called before ValidatePatchScope and before ExtractStructFromBody; (3) confirm decide/cancel/update handlers acquire and release the lock on every path; (4) confirm requestedBy is never taken from a body field; (5) confirm no Go test file sits outside services/<svc>/internal/tests/ or the module's tests/; (6) confirm CRD yaml bounds >= Go validation; (7) confirm docs list every route/field; (8) UI: no any/console/hex/vendor names, strings centralised. Report findings with file:line; fix in place only trivial ones, otherwise record as follow-ups.
- **Test first:** `/Users/houssem/Desktop/Github/telark/.claude/plans/protection-plans-approval.plan.md` — review checklist items 1-8
  - red: n/a
  - green: no high-severity finding open
- **Verify:** Review report attached to the PR body; every finding closed or listed under user-owned follow-ups.

## Docs updated in the same change set

- /Users/houssem/Desktop/Github/telark/docs/CRDS.md — ProtectionPlan row: pending_approval phase, approvalMode, approval (+history) semantics, who writes them
- /Users/houssem/Desktop/Github/telark/services/discovery/README.md — Protect bullet + API: decide route, authz, SoD, 409 semantics, approvalMode on prepare/duplicate, immutability, approved-plan material-edit refusal, pending expiry, notifications
- /Users/houssem/Desktop/Github/telark/services/exporter/README.md — API: lifecycle/approval and material keys are Internal-only on plan create/patch
- /Users/houssem/Desktop/Github/telark/charts/telark-crds/README.md — Definitions: new phase and fields
- /Users/houssem/Desktop/Github/telark/charts/telark/README.md — rollout order note (telark-crds before telark; older UI bundle degrade); VALUES.md unchanged (no new tunable)

## User-owned follow-ups

- Release internal/data (contains Plan 1 + S2) and internal/rest (Plan 1 + RetentionWindow + S3) as tags, then bump pins in services/{discovery,exporter,notifier,auth}/go.mod; CI (GOWORK=off) stays red until then.
- Rollout order per release: telark-crds chart -> exporter image -> discovery image -> dashboard-ui bundle (images/versions bumped by the GHA, never here). Rollback order is the reverse; the new CRD keys may stay (optional).
- Re-measure TOOLBAR_COMPACT_WIDTH with the 7-pill toolbar plus the Plan 1 Organize button (already owed by Plan 1 D17); update the constant from a screenshot, never a guess.
- Screenshot review of the pending phase colour (WARNING, shared with scheduled) and of every surface listed in P3 acceptance; pick a distinct token if the overlap reads badly.
- Manual dev-cluster verification listed in P1/P2 acceptance (curl PATCH 403; create-in-Production parks; approve/reject/self/stale/cancel/expiry; bell items).
- Decide whether the role editor should expose protection-plans (DEFAULT_AREAS lacks it, Plan 1 D16): until then protection-plans.approveprotectionplan.deny is settable only via the API.
- Optional later: environment-level requiresApproval on Category (schema + rest categories client + discovery wrapper) if custom production-like environments are needed; an expiry notification to the requester when a pending plan's window ends; an approver-set cache if user counts make the fan-out slow.

## Out of scope

- Environment-level 'requires approval' setting on Category (no settings field, no categories client).
- Multi-approver quorum, delegation, request-changes state, approval SLA/expiry timers independent of the plan window.
- Email/webhook delivery (no channel exists; services/notifier is an Application-CR reconciler).
- Re-approval flow that un-deploys an approved active plan on material edit (refused instead; cancel -> reactivate -> edit-while-pending is the path).
- Changing approvalMode of an existing plan in place (immutable; duplicate instead).
- Owner-only gate on choosing automatic for Production plans (cut in r1: an environmentID relabel bypassed it and closing that needs a second gate on update; the Production default covers the requirement).
- Adding protection-plans to the role editor DEFAULT_AREAS (pre-existing gap).
- Reports (Plan 2): only the hook is provided (approval.history on the CR, two notification types).
- Fixing pre-existing hex literals in PHASE_DOT_COLOR/HEALTH_DOT_COLOR or the unused PPC.LABELS.CARD.STATE.
- Unit tests for Service.Prepare/Reactivate/Decide end-to-end (concrete exporter client; pre-existing); covered via exported pure helpers and fakes at controller/handler/orchestrator level.
- Optimistic concurrency (resourceVersion) on exporter PATCH; making exporter plan create/patch routes Internal.
- Terminate action in the UI; home dashboard 'awaiting approval' tile.
- Any version bump, build, push, tag, chart publish or deploy; internal/composer; release-manager.

## Risks

- Up to one controller tick (31s default) where policies of a plan approved at endAt-epsilon stay deployed after the window; converges via Terminate. Accepted ceiling.
- Approver fan-out is O(users) grant lookups per request (async, cached resolver); fine at MVP scale; ponytail comment names the approver-set cache.
- UI mirrors the built-in Production id as a constant; if data ever changes the id the UI default drifts (server still gates).
- Older UI bundles show a raw 'pending_approval' label with the DEFAULT accent and cannot cancel a pending plan; Owners can Clear. Documented in the chart README.
- approvalMode=automatic on Production by anyone with Write is an explicit choice (no approver gate; the default is required); visible on the card and details as 'Automatic'.
- Pre-existing: the role editor cannot manage protection-plans, so the new deny flag is API-only until DEFAULT_AREAS gains the scope.
- Shared modules already carry unreleased work; this batch rides the same release, so CI stays red until the user tags and pins bump.
- Refusing material edits on approved required plans (scheduled included) may surprise users; UI explains and points to Duplicate / Cancel+Reactivate. Reversible later without schema change.
- health.Check no longer patches non-active plans: healthCheckedAt stops advancing for scheduled/canceled/terminated/failed/pending plans (they store health=unknown already). Any consumer relying on that stamp changes behaviour; none known.
- The exporter guard also blocks a previously possible direct user PATCH/CREATE of phase/reason and of policies/scope/mode/timeMode/timeRange; no known caller (the UI writes plans only through discovery, paths.ts:100-110).
- MaterialChange must stay adjacent to the update comparators; a new spec field added later without updating it silently becomes 'metadata'. The matrix test lists every field to keep the drift visible.
- Two grants resolvers (middleware + notifier) each cache independently; memory cost negligible at MVP, and a stale approver set only delays a notification.

## Revision log

| Id | Severity | Finding | Resolution |
|---|---|---|---|
| R1-1 | critical/high (both critics) | update vs decide concurrency | folded now: update handler takes lockPlanDecision (S9 dependsOn S8; D10, lifecycle G/CONCURRENCY, edge-case row, V5 item 3); Decide post-deploy re-read also requires approval.requestedAt == req.RequestedAt else cleanup + 409 (S7, lifecycle B). Update.Run re-read not added: under the shared lock Run's initial Get is authoritative. |
| R1-2 | critical (over-engineering critic) | S5 guard covers lifecycle keys only; material fields patchable via exporter | folded now: PlanLifecycleFields += policies, scope, mode, timeMode, timeRange; error message, tests ({mode}, {timeRange}, {policies}), D13, P1 goal/acceptance, exporter README wording, edge case and risk rows updated. Routes stay non-Internal (D13 rationale unchanged). |
| R1-3 | high (both critics) | Owner gate on automatic-for-Production is bypassable by environment relabel | folded now: gate CUT (both critics accept the cut; the alternative adds a second gate on update for an unrequested policy). Removed canApprove from Prepare/Duplicate/handlers, ErrAutomaticOnProductionNeedsApprover and its 403 mapping, UI disable/force logic and tooltip; D4/D6/approach/lifecycle A+K/edge case/risk/out-of-scope updated. Production stays covered by the environment-derived default. |
| R1-4 | high (over-engineering critic) | duplicate drops source approvalMode whenever environmentID is sent | folded now: BuildRequest copies source.ApprovalMode unless approvalMode is overridden or the overridden environmentID differs from source.EnvironmentID (S7 + tests renamed/added; lifecycle K; P2 acceptance; edge case). U3: duplicate form seeded from the source mode, approvalMode sent only under its own isFieldsTouched check, derive effect skips the initial environment. |
| R1-5 | high (over-engineering critic) | ApprovalPatchValue emits six keys, history never persisted | folded now: seven keys with history always the full capped array (S6 change/test renamed TestApprovalPatchValueAlwaysSevenKeys/green; P2 acceptance). |
