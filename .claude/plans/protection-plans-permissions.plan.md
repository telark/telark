# Protection Plans — Permissions end to end (one action and one deny rule per plan verb)

**Status: PLAN, not implemented.** Plan only; nothing in this document has been applied.

**Verified tree state (2026-09-23, working tree, never HEAD):**

| Repo | Branch | State |
|---|---|---|
| /Users/houssem/Desktop/Github/telark | e2e/set_plans_feature_ready_for_mvp_launch @5bcfd94 | dirty, 164 paths (Env+Tags, Reports, Approval applied uncommitted; Local AI Analyzer apply in flight) |
| /Users/houssem/Desktop/dashboard-ui | e2e/set_plans_feature_ready_for_mvp_launch @012d3ebd | dirty, 58 paths (same features) |
| /Users/houssem/Desktop/Github/internal/data | main @36ba327 (v1.14.8-1) | 19 staged paths (incl. rules.go ActionApproveProtectionPlan) |
| /Users/houssem/Desktop/Github/internal/rest | main @4fea536 (v0.14.4) | dirty; untouched by this plan |
| /Users/houssem/Desktop/Github/internal/x-ware | main @3554ee2 | dirty; untouched by this plan |

Companion JSON (identical content, machine-readable): `/Users/houssem/Desktop/Github/telark/.claude/plans/protection-plans-permissions.plan.json`.
Related: `protection-plans-exclusions` (EXCL) and `applications-cards-coverage` (APPS) are planned concurrently and applied through one merged schedule; D14 describes how the three compose. Part tag for the merged schedule: `PERM`.

## Status

PLAN, not implemented. Written 2026-09-23 against the working trees (never HEAD): telark e2e/set_plans_feature_ready_for_mvp_launch @5bcfd94 (164 dirty paths: Env+Tags, Reports and Approval applied uncommitted; Local AI Analyzer apply in flight), dashboard-ui same branch @012d3ebd (58 dirty paths), internal/data main @36ba327 (v1.14.8-1, 19 staged paths incl. rules.go ActionApproveProtectionPlan), internal/rest main @4fea536 (v0.14.4; untouched by this plan), internal/x-ware main @3554ee2 (untouched). Assumptions that must hold when the steps apply: (1) the Approval, Reports and Env+Tags applies are in the tree (verified: decide route + ApprovePlanRequirement, GuardPlanLifecycle, reports routes, plan-environments/plan-tags category scopes, ACTION_PERMISSIONS.protectionPlans.editTaxonomy); (2) the Local AI Analyzer apply edits exporter internal/authz/guard.go (globalconfig section) and dashboard-ui applications/settings files at the same time, so PERM hunks stay out of those regions; (3) the concurrent EXCL and APPS plans (read: protection-plans-exclusions.plan.json, applications-cards-coverage.plan.json) add no authz code and no plan route, and their hunks in ProtectionPlanCard.tsx, protectionPlans.ts, Content.tsx and both service READMEs do not overlap the PERM hunks (D14); (4) no custom role on any cluster relies on the pre-change semantics (approve deny blocking reject; plan create/edit denies blocking environment/tag changes), because all three features are unreleased; (5) dashboard-ui has no unit-test runner (package.json: type-check + lint), so UI steps are verified with check-all, greps and user screenshots; (6) CI (GOWORK=off) stays red until the user releases internal/data and bumps the service pins. Line numbers were read on 2026-09-23 and may drift, so every step re-reads its files before editing. No step commits, builds, pushes, deploys or bumps a version.

## Approach

PLANNED, not implemented. Authorization today: every route has exactly one x-ware Requirement, which is a level on a scope plus an optional deny Rule `<scope>.<action>.deny`. A custom role lists that Rule to withhold the action. Service-token callers are Internal and skip the check, so discovery's requirement map is the only gate for the plan actions discovery orchestrates. The UI copies this model in ACTION_PERMISSIONS/usePermission and in the role editor's SCOPE_RULES.

Gaps found in the code:
- Seven discovery plan routes carry no deny rule.
- Plan reads and report list/download/generate have no action at all.
- Approve and reject share one key.
- Environment and tag categories borrow the plan create/edit keys.
- Exporter plan DELETE skips discovery's policy cleanup.
- The role editor cannot add the protection-plans scope, because DEFAULT_AREAS lacks it.
- The UI keeps only the winning role's deny rules, gates Generate report with the cancel key, never checks the violations key, and leaves the plan routes and the sidebar entry ungated.

Design (the smallest one that closes every gap):
- Actions: one per user-visible plan verb. The 8 existing action constants stay, and internal/data gains 8 new ones: view plans, reject, view/download/generate reports, and add/edit/delete plan categories.
- Backend rules: every protection-plans route in discovery and in the exporter becomes Denyable with exactly one of these actions, at the level it already requires. Decide is the exception: approve and reject share one route, so its handler applies the approve rule or the reject rule for the decoded decision.
- Delete: exporter plan DELETE becomes Internal, which leaves discovery clear (policy cleanup first) as the only delete path.
- Categories: the category guard is keyed by operation, so plan taxonomies get their own add/edit/delete keys. Groups and roles keep their current semantics.
- Edit stays route-level: discovery update is the only session path that changes spec options, so any option added later is gated by editprotectionplan without a per-field table. Exclusions is the first such option.
- Structural tests fail any new plan route that lacks a protection-plans rule.
- Built-in roles are unchanged and carry no deny rules. Their behaviour is identical except that the exporter's plan DELETE now refuses sessions.
- UI: ACTION_PERMISSIONS and SCOPE_RULES gain the same 8 keys (editTaxonomy is removed). The scope index merges deny rules from all roles, as x-ware does. The role editor offers the Protection Plans area. Plan routes, the sidebar entry and the home widgets gate on the view key, and the Violations/Reports sections gate on their view keys. Every plan action follows the Applications pattern: it is shown when the phase allows it, and disabled with a 'You do not have permission to ...' tooltip when the role withholds it.
- Nothing changes in the CRD, rest, x-ware, auth-service, charts or install docs.

## Execution flow today (traced)

Verified on the working trees (file:line as read 2026-09-23).
1. Middleware: x-ware authz/handler.go:53-57 looks up Requirements[method+pattern] and returns 403 when the key is missing. Public skips identity. A matching X-Service-Token becomes Identity{Internal:true} and skips the scope check on every route (handler.go:75-80). A session is resolved to grants and checked with Allows (allow.go:12-43): the deny check comes first (Denied[scope] or Denied[ALL]), then the level (the explicit scope level, else ALL). Grants (grants.go:87-116) collect the Rules of every active, unexpired role, and each scope keeps the highest level any role grants it.
2. Discovery plan routes (routes/routes.go:39-49, authz/requirements.go:55-71): only decide is Denyable (Own + approveprotectionplan, which also covers reject). Prepare, cancel, duplicate, reactivate, update and reports/generate require Write with no rule. Clear requires Own with no rule. Templates, status and violations require Read with no rule.
3. Discovery calls the exporter with the service token (internal/rest clients/shared/servicetoken.go:29-35), so exporter requirements never apply to actions that discovery orchestrates.
4. Exporter plan routes (authz/requirements.go:156-169): list/get require Read with no rule. Create is Denyable(Write, createprotectionplan), patch is Denyable(Write, editprotectionplan) and delete is Denyable(Own, deleteprotectionplan). An Owner session can reach delete directly, and it removes the CR without the CleanupByPlanID that discovery Clear runs first (core/plans/protection/svc.go:521-528). GuardPlanLifecycle (guard.go:266-287) refuses lifecycle and material keys from sessions on create/patch.
5. Exporter reports (authz/requirements.go:146-154): list and download require Read with no rule; create and ledger put/get are Internal.
6. Categories (requirements.go:102-111) are Authenticated; writes defer to GuardCategoryScope (guard.go:156-212), whose map is keyed by level. plan-environments and plan-tags are governed by protection-plans: Contributor maps to createprotectionplan, and Owner maps to editprotectionplan for both edit AND delete. The handler calls it at category/handler.go:48 (create), 224 and 242 (patch old and new scope) and 267 (delete).
7. Built-in roles (internal/data resources/role/builtin.go:45-102): Admin holds ALL=Admin; Owner, Contributor and ReadOnly hold their level on every built-in scope. None carries Rules.
8. UI: GET my-permissions returns the raw roles. permissionsSlice.buildScopeIndex keeps the rules of the single winning role per scope (permissionsSlice.ts:18-60). usePermission(scope, level, denyKey) is at permissionEngine.tsx:267-281, and ACTION_PERMISSIONS.protectionPlans has 9 entries (211-257; editTaxonomy reuses the edit deny at Owner). The role editor's SCOPE_RULES has a protection-plans block (scopeRules.ts:100-117), but DEFAULT_AREAS (roles.ts:228-234) has no protection-plans area, so no custom role can hold plan rules from the UI. ProtectedRoute ignores its requiredScope/minimumLevel props (ProtectedRoute.tsx:37), and the plan routes pass none (AppRoutes.tsx:139-158). The sidebar entry is ungated (MenuButtons.tsx:101-117). Toolbar and card hide denied actions without a tooltip. Generate report is gated by the cancel key (Content.tsx:112-116). The violations key is never checked, and report list/download are ungated (Content.tsx:563-641, ReportsSection.tsx:131-147).

## Permission matrix after this plan (lifecycle)

PERMISSION MATRIX after this plan. Scope protection-plans; rule = protection-plans.<action>.deny, listed by a custom role; built-in roles grant levels only.

| Action | Key | Level | Backend route(s) | UI surface |
|---|---|---|---|---|
| View plans | viewprotectionplans | ReadOnly | exporter GET plans/protection/get, GET plans/protection/{id}/get; discovery GET plans/protection/templates, GET plans/protection/{id}/status (health refresh) | plan routes (PermissionGate, else home), sidebar entry, home plan widgets, APPS coverage (U5) |
| View violations | viewprotectionplanviolations | ReadOnly | discovery GET plans/protection/{id}/violations | Violations card (not rendered when withheld) |
| View reports | viewprotectionplanreports | ReadOnly | exporter GET reports/plans/{id}/get | Reports card: list + refresh (not rendered when withheld) |
| Download report | downloadprotectionplanreport | ReadOnly | exporter GET reports/plans/{id}/download | per-format download buttons (disabled + tooltip) |
| Create (and activate) | createprotectionplan | Contributor | discovery POST plans/protection/prepare; exporter POST plans/protection/create (sessions cannot send policies) | list toolbar Create (disabled + tooltip), empty-state CTA (hidden) |
| Edit (every field group and every future spec option, Exclusions included) | editprotectionplan | Contributor | discovery POST plans/protection/{id}/update; exporter PATCH plans/protection/{id}/patch (metadata keys only for sessions) | details Edit, card Edit (disabled + tooltip); PlanForm behind the Edit opener |
| Duplicate | duplicateprotectionplan | Contributor | discovery POST plans/protection/{id}/duplicate | details and card Duplicate |
| Reactivate | reactivateprotectionplan | Contributor | discovery POST plans/protection/{id}/reactivate | details and card Reactivate (reactivatable phases) |
| Cancel | cancelprotectionplan | Contributor | discovery POST plans/protection/{id}/cancel | details overflow and card Cancel (cancellable phases) |
| Generate report | generateprotectionplanreport | Contributor | discovery POST plans/protection/{id}/reports/generate | Generate report button |
| Add environment/tag | addprotectionplancategory | Contributor | exporter POST classification/categories/create (scope plan-environments or plan-tags) | taxonomy Add |
| Edit environment/tag | editprotectionplancategory | Owner | exporter PATCH classification/categories/{id}/patch (old and new scope) | taxonomy Edit |
| Delete environment/tag | deleteprotectionplancategory | Owner | exporter DELETE classification/categories/{id}/delete | taxonomy Delete |
| Delete plan | deleteprotectionplan | Owner | discovery DELETE plans/protection/{id}/clear (cleanup, then CR delete); exporter DELETE plans/protection/{id}/delete becomes Internal | details overflow and card Delete |
| Approve | approveprotectionplan | Owner | discovery POST plans/protection/{id}/decide {decision: approved} (rule applied in the handler) | Approve on toolbar and card (pending_approval) |
| Reject | rejectprotectionplan | Owner | discovery POST plans/protection/{id}/decide {decision: rejected} (rule applied in the handler) | Reject on toolbar and card (pending_approval) |

No key, because there is no user action: activate (it is create, approve or reactivate; the controller activates scheduled plans as system), terminate (the controller at endAt; the user's stop verb is cancel), rollback and snapshots (application actions under the applications scope keys), approvalMode (a create/duplicate field, immutable on edit).

Invariant enforced by tests: every discovery and exporter route whose Scope is protection-plans carries a protection-plans rule. Discovery decide is the one exception (route Own, rule per decision in the handler).

## Decisions (defaults the user can override)

| # | Question | Decision | Why |
|---|---|---|---|
| D1 | What is the unit of permission? | Each user-visible plan verb gets its own action, and the route(s) for that verb are Denyable with rule `protection-plans.<action>.deny` at the level the route already requires. That gives 16 actions. Existing: viewprotectionplanviolations, createprotectionplan, editprotectionplan, cancelprotectionplan, duplicateprotectionplan, reactivateprotectionplan, deleteprotectionplan, approveprotectionplan. New: viewprotectionplans, rejectprotectionplan, viewprotectionplanreports, downloadprotectionplanreport, generateprotectionplanreport, addprotectionplancategory, editprotectionplancategory, deleteprotectionplancategory. | This is the only deny mechanism the backend has (x-ware Denyable + RuleKey, requirement.go:31-35, rule.go:5-7), and it is the vocabulary the role editor already renders (scopeRules.ts). Levels do not change, so built-in roles (no Rules, builtin.go:45-102) behave exactly as today and only custom roles use the new keys. |
| D2 | How granular is edit, and how are new spec options (Exclusions) covered? | One edit action covers every field group. It is enforced per route, never per field: discovery POST {id}/update becomes Denyable(Write, editprotectionplan), and the exporter PATCH keeps the same rule. There is no field table. In the UI, every field group stays inside PlanForm behind the single Edit opener. | Update is the only session path that changes spec options. The UI writes plans only through discovery, and the exporter's GuardPlanLifecycle refuses policies/scope/mode/time keys from sessions. So any option in the update body is gated by editprotectionplan with no new code; EXCL's scope.exclusions is the first. Per-group keys would need a new key plus a field-to-key table for every future option, which is exactly the special case the requirement rules out. TestEveryPlanRouteIsDenyable makes a future plan route without a rule fail CI. |
| D3 | Approve and reject: one key or two, one route or two? | Two keys (approveprotectionplan and the new rejectprotectionplan) on one route. The decide route requirement becomes Own(protection-plans) with no rule. The handler applies ApprovePlanRequirement() or the new RejectPlanRequirement() for the decoded decision and returns 403 when the role withholds it. The check runs before the per-plan lock and before any exporter call. The approval notifier still targets users who may approve. | A route requirement cannot read the body (x-ware keys on method + pattern, handler.go:53), so per-decision rules have to live in the handler. Splitting into /approve and /reject routes would change the rest endpoint, the UI client and the thunk for the same result. Two keys let a role reject (block a deploy) without being able to approve (cause one), which is the usual separation-of-duties split. |
| D4 | Is reading plans an action with a deny rule? | Yes: viewprotectionplans (ReadOnly) on exporter list/get and on discovery templates/status (status is the health refresh). In the UI, the plan routes, the sidebar entry, the home plan widgets and every plan read in other features (APPS coverage, via U5) gate on ACTION_PERMISSIONS.protectionPlans.view. | A level cannot go below ReadOnly or below an ALL-scope grant (MergeLevel ignores rank 0; grantedLevel falls back to ALL, allow.go:37-54). A deny rule is therefore the only way to keep plans away from a role that otherwise reads every scope. It also gives the UI one key for every plan read surface, where today it uses a bare level check (Dashboard.tsx:56). |
| D5 | Reports: which actions? | Three actions. viewprotectionplanreports (ReadOnly) on exporter GET reports/plans/{id}/get; downloadprotectionplanreport (ReadOnly) on exporter GET reports/plans/{id}/download; generateprotectionplanreport (Contributor) on discovery POST plans/protection/{id}/reports/generate. The UI Generate button stops borrowing cancelprotectionplan. | The requirement names generate, list and download separately, and each is one Denyable entry on an existing route. Report files contain admission decisions, so withholding violations alone does not hide them. That edge case is documented but not coupled in code, because coupling keys would be a special case. |
| D6 | Delete: which path carries deleteprotectionplan? | Discovery DELETE plans/protection/{id}/clear becomes Denyable(Own, deleteprotectionplan). Exporter DELETE plans/protection/{id}/delete becomes Internal. | Clear is the only path that removes deployed policies before deleting the CR (svc.go:521-528), and it is what the UI calls (dashboard-ui clients/delete.ts -> Endpoints.PROTECTION_PLANS.CLEAR). The exporter route let an Owner session delete a plan while its policies kept enforcing, with no plan left to cancel them. The fix is one requirement line, and discovery still reaches the route with the service token, as today. |
| D7 | Exporter create/patch: change them? | No. They keep Denyable(Write, createprotectionplan) and Denyable(Write, editprotectionplan). GuardPlanLifecycle and PlanLifecycleFields stay untouched: the approval plan owns them, and EXCL relies on the `scope` key in that list. | Session create is already unusable: the guard refuses policies/scope/mode and Validate requires policies. Session patch is metadata-only under the edit rule. Both therefore already fit the model. The approval plan explicitly rejected making these routes Internal, and permissions do not need it. |
| D8 | Environment and tag categories: which keys, and how does the guard tell edit from delete? | Three plan-category actions shared by both taxonomies: addprotectionplancategory (Contributor), editprotectionplancategory (Owner) and deleteprotectionplancategory (Owner). GuardCategoryScope takes an operation (constants CategoryOpCreate/Edit/Delete) and looks up a full xauthz.Requirement per scope and operation, so categoryGoverningScope goes away (the requirement carries its scope). Groups and roles keep exactly their current semantics, including delete checking their edit rule. | The map keyed by level cannot tell edit from delete, since both are Owner (guard.go:158-175). Keying by operation fixes that and is shorter than today's code. The names mirror groups/roles (addgroupcategory, editgroupcategory, ...). One key set covers both taxonomies because nothing asks to separate environments from tags. Pointing groups/roles delete at their unused delete constants would change enforcement in other features, so it is a follow-up. |
| D9 | Activate, terminate, rollback, snapshots, approvalMode: new keys? | No new keys:<br>- Activate is not a user route. Prepare activates immediately under create, approve and reactivate activate under their own keys, and the controller activates scheduled plans as system.<br>- Terminate runs only in the controller at endAt (svc.go:477-489); the user's stop verb is cancel.<br>- Rollback and snapshots are application actions under the applications scope (rollbackapplication, viewapplicationssnapshots, ...); no plan-scoped route exists.<br>- approvalMode is a create/duplicate field, immutable on update, so createprotectionplan and duplicateprotectionplan cover it. | A key without a route is dead vocabulary: the role editor would offer it and nothing would enforce it. A separate 'skip approval' key is not sound without three more gates (environment relabel on update, duplicate copy, material edits of existing automatic Production plans). It would also contradict the approval plan's decision that Write suffices and Production defaults to required. Override: ask for a skipprotectionplanapproval key. |
| D10 | Built-in roles and the role editor | Built-in roles are unchanged and carry no Rules. The role editor gains the Protection Plans area (DEFAULT_AREAS), so a custom role can hold the scope and every plan deny rule from the UI. | Without the area (roles.ts:228-234), the protection-plans deny rules in SCOPE_RULES cannot be reached from the UI and the whole model is API-only; the approval plan's risks already flagged this as pre-existing. A new role seeds the area at ReadOnly, like every other area. |
| D11 | How does the UI present an action the role withholds? | It follows the Applications pattern (ApplicationDetailsToolbar.tsx:71-120, ApplicationCardHeader.tsx:320-345). Every action the phase allows is rendered. When the permission check fails, the action is disabled with a 'You do not have permission to ...' tooltip, and that tooltip takes precedence over phase/state tooltips. Actions the phase does not allow stay hidden. The Violations and Reports sections are not rendered when their view key is withheld. The empty-state Create CTA stays hidden when create is withheld, because EmptyState has no tooltip. | Plans hide denied actions, while Applications and the taxonomy controls disable them with a tooltip. One rule across features is what 'buttons, menus and tooltips' asks for. This replaces the approval plan's 'a non-approver sees neither button' with 'sees both disabled, with the reason'. |
| D12 | UI permission evaluation vs x-ware | buildScopeIndex merges deny rules from every active role that contributes to a scope, whether explicit or expanded from ALL. Level selection is unchanged. | x-ware appends every active role's rules (grants.go:100-104) and checks Denied[scope] or Denied[ALL] (allow.go:26-35). The UI kept only the winning role's rules (permissionsSlice.ts:29-41), so a deny on a second role left buttons enabled that then returned 403. The level divergence (x-ware prefers an explicit scope level over ALL) predates this plan and affects every scope, so it is a follow-up and is not changed here. |
| D13 | Route guard | Reuse the existing PermissionGate, which currently has no caller: add optional `action` (deny key) and `fallback` props. Both plan routes wrap their FeatureErrorBoundary in PermissionGate(view) inside ProtectedRoute, with fallback <Navigate to={APP_ROUTES.HOME} replace />. ProtectedRoute's ignored requiredScope/minimumLevel props stay as they are. | ProtectedRoute renders its children only once permissions are ready (ProtectedRoute.tsx:55-57), so the gate never redirects while permissions load. Making ProtectedRoute honour its declared props would silently start enforcing the roles/users/groups routes (AppRoutes.tsx:109-137), which is outside this feature and becomes a follow-up. |
| D14 | How do the three concurrent plans compose? | PERM owns every permission key and authz entry.<br>- EXCL adds no key, route or per-field check. Exclusions travel through prepare/update/duplicate under create/edit/duplicate, inside ScopeSection/PlanForm behind the Edit opener, and the exporter `scope` guard key already covers them.<br>- APPS gates its plan read and coverage section on ACTION_PERMISSIONS.protectionPlans.view. APPS AP2 takes that key only when U1 has already landed (its D5), so PERM U5 makes it unconditional: U5 switches the gate in useApplicationCoverage.ts to the view key and is a checked no-op when AP2 already used it.<br>- Cross-plan edge the merged schedule must encode: PERM U5 runs after APPS AP2. dependsOn holds same-plan ids only, so the U5 JSON step carries it as `crossPlanDependsOn: ["APPS:AP2"]` (`<part tag>:<step id>`). AP2 and U1 share no file and may run in parallel.<br><br>Shared files and their disjoint hunks:<br>- Exporter README: EXCL edits the lifecycle-keys paragraph; PERM adds a routes table, changes the reports Authz cells and adds one categories sentence.<br>- Discovery README: EXCL adds an exclusions paragraph; PERM changes two phrases and appends a Permissions paragraph at the end of the API section.<br>- protectionPlans.ts: EXCL edits CREATE_PAGE.FORM/DETAIL_PAGE.FIELDS; APPS removes CARD_LAYOUT/ACCENT_TINT; PERM adds LABELS.PERMISSION_DENIED after LABELS.REPORTS.<br>- Content.tsx: EXCL edits the Scope card rows; PERM edits the hooks at 110-116 and the Violations/Reports cards.<br>- ProtectionPlanCard.tsx: APPS edits imports/styles only, never usePermission/menuItems; PERM edits the usePermission calls, menuItems and the phaseRules import only.<br>- useApplicationCoverage.ts: APPS AP2 creates it and owns every line; PERM U5 changes only the auth/hooks import, one module-level viewPlans destructure and the canViewPlans line. | No contract gets contradictory edits, and the merged schedule runs one step at a time on each shared file. The one cross-plan edge (APPS AP2 before PERM U5) is carried by U5, so the coverage gate ends on the view key whichever of U1 and AP2 applies first. |
| D15 | Migration of saved custom roles | None. | Approval, Env+Tags and Reports are unreleased, so no saved role can depend on approve blocking reject or on plan create/edit blocking environment/tag changes. The edge case is documented for anyone who created such a role on a dev cluster. |

## Edge cases

| Case | Behaviour | Covered by |
|---|---|---|
| A user holds two roles on protection-plans, and only one of them denies an action | The backend refuses, because rules from all roles are merged. The UI now disables the control too, instead of showing it enabled and failing with 403. | U1 (buildScopeIndex), R1 item 8 |
| An ALL-scope role whose rules list protection-plans.viewprotectionplans.deny | x-ware checks Denied[ALL] for every scope, and the UI expands ALL into protection-plans together with its rules, so plans are hidden in the UI and refused by the API. | S2/S3 vocabulary tests, U1 |
| An Owner denied approveprotectionplan but not rejectprotectionplan (or the reverse) | Approve returns 403 from the decide handler before any lock or exporter call, while reject proceeds (and the reverse). In the UI, the withheld button is disabled with its tooltip. | S3 TestRequestAllows, TestDecideRefusesDeniedDecision; U2 |
| A decision value that is neither approved nor rejected | The handler applies the approve rule, then core validation returns 400 as it does today. | S3 (ValidateDecision unchanged) |
| A request reaches decide with no identity in the context (misconfigured middleware) | 403, never a pass-through. | S3 TestDecideRefusesMissingIdentity |
| A role denied viewprotectionplans | Exporter list/get and discovery templates/status return 403. The sidebar entry is hidden, a deep link (including a notification link) redirects home, and the home plan widgets and APPS coverage are hidden. Write routes are not gated on view, so an API caller that holds the write keys can still act on a plan id it knows. | S2, S3, U4, U5, APPS D5 |
| A role denied violations but allowed reports | Report files contain admission decisions, so violation data stays readable through reports. To hide it, withhold viewprotectionplanreports and downloadprotectionplanreport as well. | S3 README Permissions paragraph |
| A role allowed to generate reports but denied viewing them | The API allows generating. The UI does not render the Reports card at all, so the Generate button is not shown. | U3 |
| A role denied create but allowed duplicate | Duplicate is its own action and still creates a plan. Deny both to stop new plans. | D1, discovery README |
| A session identity calls exporter DELETE plans/protection/{id}/delete | 401, because the route is Internal. The UI is unaffected, since it calls discovery clear. | S2 TestPlanDeleteIsInternal |
| A category moved between scopes by PATCH | The edit operation is required on both the old and the new scope, as today. | S2 (handler.go:224/242) |
| An unknown category scope or operation | 403 with ErrAuthzUnknownCategoryScope. | S2 TestGuardCategoryScope |
| A custom role saved on a dev cluster before this change, with approveprotectionplan, createprotectionplan or editprotectionplan denies | The approve deny no longer blocks reject. The plan create/edit denies no longer block environment/tag add, edit or delete, which now follow the three category keys. Re-save the role with the new keys. | D15 |
| An old dashboard bundle against the new backend | Works: every built-in grant is unchanged and the old bundle deletes through clear. A custom role that uses a new key sees the control enabled and gets a 403. | Rollout follow-up |
| A new dashboard bundle against an old backend | The UI is stricter than the backend until exporter/discovery roll out. Nothing breaks. | Rollout follow-up |
| Permissions still loading | ProtectedRoute renders its children only once permissions are ready, so PermissionGate never redirects early. The sidebar Plans entry appears when permissions load. | U4 |
| An approver whose view is denied receives plan.approval.requested | The notifier selects users on the approve rule only, and the notification's deep link redirects home. Accepted. | D3 |
| An approver who can reject but not approve | Not notified of requests, because the notifier uses ApprovePlanRequirement. Can still reject from the plan page. | D3 |
| A new custom role created in the editor | It seeds Protection Plans at ReadOnly, like every other area. Remove the row or add the view deny to withhold plans. | U1 |

## Phases

### P0 — Phase 0 — Contract: action vocabulary in internal/data

**Goal:** The 8 new action constants exist, and all 16 protection-plans action strings are pinned at their source before any service or UI uses them.

**Steps:** S1

**Acceptance criteria:**
- internal/data: `go test ./...` and `golangci-lint run` are green, and TestProtectionPlanActionVocabulary pins the 16 protection-plans actions (8 existing unchanged, 8 new).
- git diff of internal/data for this step touches only resources/role/rules.go and the new tests/role/actions_test.go. No built-in role, scope or other package changes.
- User-owned gate (not done here): release internal/data and bump the pins in services/{discovery,exporter,notifier,auth}/go.mod so CI (GOWORK=off) passes. Workspace builds via go.work do not wait for it.

### P1 — Phase 1 — Backend enforcement: exporter and discovery

**Goal:** Every protection-plans route carries exactly one plan action as its deny rule at an unchanged level. Approve and reject are separately deniable. Discovery clear is the only user delete path. Plan taxonomies have their own add/edit/delete rules.

**Steps:** S2, S3

**Acceptance criteria:**
- services/exporter: `go test ./...` and `golangci-lint run` are clean. TestRuleKeysMatchDashboardVocabulary (6 new plan/report rows), TestReportRoutesRequirements, TestPlanDeleteIsInternal, TestGuardCategoryScope and TestGuardCategoryScopePlanTaxonomies pass.
- services/discovery: `go test ./...` and `golangci-lint run` are clean. TestPlanRuleKeysMatchDashboardVocabulary (10 rows), TestEveryPlanRouteIsDenyable, TestDecideRouteIsOwnerWithPerDecisionRules, TestRequestAllows, TestDecideRefusesDeniedDecision and TestDecideRefusesMissingIdentity pass. Every existing test in both services passes unchanged, except the flagged edits named in S2/S3.
- Built-in roles see no behaviour change except that the exporter plan DELETE now refuses sessions (401).
- Manual check, run by the user on the dev cluster: a custom role with Owner on protection-plans that denies one action gets 403 on that action's route and 200 elsewhere, for every row of the matrix. An Owner denied approve can reject, and the reverse. A session DELETE on exporter /api/v1/plans/protection/<id>/delete returns 401. Environment/tag add, edit and delete honour the three category rules.

### P2 — Phase 2 — dashboard-ui

**Goal:** The UI offers, evaluates and displays the same 16 keys at the same levels as the backend: role editor, scope index, route guard, sidebar, sections, buttons, menus and tooltips.

**Steps:** U1, U2, U3, U4, U5

**Acceptance criteria:**
- `npm run check-all` reports zero errors, with no eslint-disable, no any, no console.*, no hex and no vendor name. Every new string lives in a constants file.
- ACTION_PERMISSIONS.protectionPlans has 16 entries whose deny strings equal the backend vocabulary tests, and SCOPE_RULES protection-plans lists the same 16 keys at the same levels. editTaxonomy is gone.
- No plan action is hidden because of a permission check alone. Every control disabled for a permission shows a PERMISSION_DENIED tooltip.
- No plan read uses a bare level check: `grep -rn "usePermission('protection-plans'" src` prints nothing (Dashboard.tsx via U4, APPS useApplicationCoverage.ts via U5).
- User screenshots:
  - Role editor with the Protection Plans row and its rules per level.
  - A ReadOnly user: details toolbar, card menu, list Create and taxonomy controls disabled with tooltips.
  - A view-denied user: no sidebar entry, a deep link lands on home, no home plan widgets, no Applications 'Protected by' section and no plans/protection/get 403.
  - A violations-denied user: no Violations card.
  - A reports-view-denied user: no Reports card.
  - A download-denied user: disabled download buttons with a tooltip.
  - A generate-denied user: Generate disabled with the permission tooltip.
  - An Owner denied reject: Reject disabled with its tooltip, Approve enabled.

### P3 — Phase 3 — Verification and review

**Goal:** Each module passes its gates and an adversarial review signs off. The orchestrator replaces V1-V4 and R1 with one integrated verification.

**Steps:** V1, V2, V3, V4, R1

**Acceptance criteria:**
- V1-V4 commands exit 0 with zero lint issues and zero check-all errors.
- R1 reports no open finding of severity high. Every finding is either fixed in place or recorded as a user-owned follow-up.

## Steps (execution order: shared module before consumers; exporter and discovery in parallel; UI foundation before UI surfaces; one step at a time per shared file)

### S1 — internal/data: 8 new protection-plans action constants + vocabulary test

- **Phase:** P0 · **Part:** backend · **Repo:** internal/data · **dependsOn:** — · **parallelGroup:** contract
- **Route:** claude-opus-5-5 / effort xhigh / P3 — criteria M1 — Exported API of a shared package consumed by exporter and discovery, and the string vocabulary persisted inside custom role CRs and mirrored by the UI.
- **Files:**
  - `/Users/houssem/Desktop/Github/internal/data/resources/role/rules.go`
  - `/Users/houssem/Desktop/Github/internal/data/tests/role/actions_test.go`

**Change:**

resources/role/rules.go, protection-plans const block (lines 54-63): keep the eight existing constants byte-identical and append, gofmt-aligned in the same block:
```go
ActionViewProtectionPlans          = "viewprotectionplans"
ActionRejectProtectionPlan         = "rejectprotectionplan"
ActionGenerateProtectionPlanReport = "generateprotectionplanreport"
ActionViewProtectionPlanReports    = "viewprotectionplanreports"
ActionDownloadProtectionPlanReport = "downloadprotectionplanreport"
ActionAddProtectionPlanCategory    = "addprotectionplancategory"
ActionEditProtectionPlanCategory   = "editprotectionplancategory"
ActionDeleteProtectionPlanCategory = "deleteprotectionplancategory"
```
Nothing else in the module changes: builtin.go stays as is (built-in roles carry no deny rules; a deny only takes effect through a custom role), def.go scopes/levels unchanged.
NEW tests/role/actions_test.go (package role; import roledata "github.com/telark/data/resources/role"): TestProtectionPlanActionVocabulary builds
`expected := map[string]string{roledata.ActionViewProtectionPlanViolations: "viewprotectionplanviolations", roledata.ActionCreateProtectionPlan: "createprotectionplan", roledata.ActionEditProtectionPlan: "editprotectionplan", roledata.ActionCancelProtectionPlan: "cancelprotectionplan", roledata.ActionDuplicateProtectionPlan: "duplicateprotectionplan", roledata.ActionReactivateProtectionPlan: "reactivateprotectionplan", roledata.ActionDeleteProtectionPlan: "deleteprotectionplan", roledata.ActionApproveProtectionPlan: "approveprotectionplan", roledata.ActionViewProtectionPlans: "viewprotectionplans", roledata.ActionRejectProtectionPlan: "rejectprotectionplan", roledata.ActionGenerateProtectionPlanReport: "generateprotectionplanreport", roledata.ActionViewProtectionPlanReports: "viewprotectionplanreports", roledata.ActionDownloadProtectionPlanReport: "downloadprotectionplanreport", roledata.ActionAddProtectionPlanCategory: "addprotectionplancategory", roledata.ActionEditProtectionPlanCategory: "editprotectionplancategory", roledata.ActionDeleteProtectionPlanCategory: "deleteprotectionplancategory"}`
and asserts `key == value` for every entry. A Go map literal with two equal constant keys does not compile, so the literal itself proves the 16 values are distinct. Comment (2 lines max): saved custom roles store these strings inside `<scope>.<action>.deny`, so renaming a value silently drops saved denials.

**Test first:** `/Users/houssem/Desktop/Github/internal/data/tests/role/actions_test.go` — `TestProtectionPlanActionVocabulary`
- red: compile error: undefined roledata.ActionViewProtectionPlans (and the 7 other new constants)
- green: all 16 constants equal their pinned strings; the map literal compiles, so they are pairwise distinct

**Verify:** cd /Users/houssem/Desktop/Github/internal/data && go test ./... && golangci-lint run

### S2 — Exporter: deny rules on plan reads and reports, Internal plan delete, operation-keyed category guard with plan category rules, README

- **Phase:** P1 · **Part:** backend · **Repo:** telark/exporter · **dependsOn:** S1 · **parallelGroup:** exporter
- **Route:** claude-opus-5-5 / effort xhigh / P3 — criteria M3, M1 — Authz enforcement on user-reachable exporter routes, plus a route contract change for session callers (plan DELETE becomes Internal).
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/requirements.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/guard.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/authz.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/handlers/classification/category/handler.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/rules_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/reports_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/requirements_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/guard_extra_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/exporter/README.md`

**Change:**

(1) internal/authz/requirements.go. addReports (lines 146-154): `ListPlanReports` -> `authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports)`; `DownloadPlanReport` -> `authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), roledata.ActionDownloadProtectionPlanReport)`; the three Internal entries unchanged; comment becomes "Reports are written by discovery at a plan boundary; users read them under the plan scope, and listing and downloading can each be withheld." addPlans (lines 156-169): `ListProtectionPlans` and `GetProtectionPlanByID` -> `authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans)`; create and patch unchanged; `DeleteProtectionPlanByID` -> `authz.Internal` with the comment "Users delete through discovery's clear route, which removes the deployed policies first." (the UI already calls discovery clear: dashboard-ui clients/delete.ts). No other add* function changes.
(2) internal/constants/authz.go: new block after the ErrAuthz block: `const ( CategoryOpCreate = "create"; CategoryOpEdit = "edit"; CategoryOpDelete = "delete" )` (gofmt, one per line).
(3) internal/authz/guard.go, category section ONLY (lines 156-212). Do not touch globalConfigFields / GuardGlobalConfigPatch (the Local AI Analyzer apply is editing that region right now) or GuardPlanLifecycle (approval plan; EXCL relies on it). Replace `categoryActions` and `categoryGoverningScope` with:
```go
// A category is governed by the authz scope it classifies, and each operation
// on it has its own rule.
var categoryRequirements = map[string]map[string]xauthz.Requirement{
	roledata.ScopeGroups: {
		constants.CategoryOpCreate: xauthz.Denyable(xauthz.Write(roledata.ScopeGroups), roledata.ActionAddGroupCategory),
		constants.CategoryOpEdit:   xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionEditGroupCategory),
		// Deletion checks the edit rule, as it did before operations were keyed.
		constants.CategoryOpDelete: xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionEditGroupCategory),
	},
	roledata.ScopeRoles: {
		constants.CategoryOpCreate: xauthz.Denyable(xauthz.Write(roledata.ScopeRoles), roledata.ActionAddRoleCategory),
		constants.CategoryOpEdit:   xauthz.Denyable(xauthz.Own(roledata.ScopeRoles), roledata.ActionEditRoleCategory),
		constants.CategoryOpDelete: xauthz.Denyable(xauthz.Own(roledata.ScopeRoles), roledata.ActionEditRoleCategory),
	},
	categorydata.ScopePlanEnvironments: planCategoryRequirements,
	categorydata.ScopePlanTags:         planCategoryRequirements,
}

var planCategoryRequirements = map[string]xauthz.Requirement{
	constants.CategoryOpCreate: xauthz.Denyable(xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionAddProtectionPlanCategory),
	constants.CategoryOpEdit:   xauthz.Denyable(xauthz.Own(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlanCategory),
	constants.CategoryOpDelete: xauthz.Denyable(xauthz.Own(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlanCategory),
}
```
GuardCategoryScope becomes `func GuardCategoryScope(w http.ResponseWriter, r *http.Request, categoryScope, operation string) bool`: identity missing -> `denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))`, false; `identity.Internal` -> true; `requirement, known := categoryRequirements[categoryScope][operation]`; `!known` -> `denyForbidden(w, constants.ErrAuthzUnknownCategoryScope)`, false; `!xauthz.Allows(identity, requirement)` -> `denyForbidden(w, constants.ErrAuthzCategoryScopeDenied)`, false; else true. Comment: "An unknown scope or operation is refused outright rather than slipping past the check." Drop the `cmp` import if nothing else in the file uses it. Groups/roles semantics are exactly the old ones (Contributor + add rule; Owner + edit rule for edit AND delete); plan taxonomies move from createprotectionplan/editprotectionplan to their own three rules at the same levels.
(4) internal/handlers/classification/category/handler.go: line 48 `roledata.PermissionLevelContributor` -> `constants.CategoryOpCreate`; lines 224 and 242 -> `constants.CategoryOpEdit` (a scope move still needs edit on both scopes); line 267 -> `constants.CategoryOpDelete`; remove the now-unused roledata import.
(5) Tests. FLAGGED: the edited assertions pinned the old contract, which changes on purpose; no assertion is weakened.
- tests/authz/rules_test.go TestRuleKeysMatchDashboardVocabulary: add rows "GET /api/v1/plans/protection/get" and "GET /api/v1/plans/protection/{id}/get" -> "protection-plans.viewprotectionplans.deny"; "POST /api/v1/plans/protection/create" -> "protection-plans.createprotectionplan.deny"; "PATCH /api/v1/plans/protection/{id}/patch" -> "protection-plans.editprotectionplan.deny"; "GET /api/v1/reports/plans/{id}/get" -> "protection-plans.viewprotectionplanreports.deny"; "GET /api/v1/reports/plans/{id}/download" -> "protection-plans.downloadprotectionplanreport.deny".
- tests/authz/reports_test.go TestReportRoutesRequirements (FLAG): expected list/download become the two Denyable requirements of (1); delete the trailing `if got.Rule != constants.EmptyString` block (the struct equality already pins Rule) and the then-unused constants import; comment "Create and the ledger routes are reached by discovery only; list and download follow the plan read scope, each with its own deny rule."
- tests/authz/requirements_test.go: TestProtectionPlansUseTheirOwnScope (FLAG) drops "DELETE /api/v1/plans/protection/{id}/delete" from planRoutes; NEW TestPlanDeleteIsInternal asserts that route's requirement `== xauthz.Internal` (comment: users delete through discovery clear, which cleans up deployed policies first).
- tests/authz/guard_extra_test.go: TestGuardCategoryScope passes `constants.CategoryOpCreate` where it passed a level, plus one assertion that operation "rename" on roledata.ScopeRoles is refused for `levels(roledata.ScopeRoles, roledata.PermissionLevelAdmin)`. TestGuardCategoryScopePlanTaxonomies (FLAG: its 'edit deny bites owner tag edit' row pinned the borrowed editprotectionplan key) becomes rows {name, identity, scope, operation, want} using `ownerDenied(action) = denied(levels(plans, owner), plans, xauthz.RuleKey(plans, action))`: plans contributor adds environment (create) true; plans contributor edits tag (edit) false; plans owner edits environment (edit) true; plans owner deletes tag (delete) true; add deny bites tag add (ownerDenied(ActionAddProtectionPlanCategory), tags, create) false; edit deny bites tag edit (ownerDenied(ActionEditProtectionPlanCategory), tags, edit) false; edit deny spares tag delete (same identity, tags, delete) true; delete deny bites environment delete (ownerDenied(ActionDeleteProtectionPlanCategory), environments, delete) false; plan edit deny no longer governs taxonomies (ownerDenied(ActionEditProtectionPlan), environments, edit) true; plan create deny no longer governs taxonomies (ownerDenied(ActionCreateProtectionPlan), tags, create) true; groups grant does not pass through (levels(groups, owner), environments, create) false; groups category still governed by groups (levels(groups, contributor), groups, create) true; groups delete still checks the group edit rule (denied(levels(groups, owner), groups, xauthz.RuleKey(groups, ActionEditGroupCategory)), groups, delete) false. The denied-body assertion (contains ErrAuthzCategoryScopeDenied, never ErrAuthzUnknownCategoryScope) stays.
(6) services/exporter/README.md, API section. Leave the lifecycle-keys paragraph byte-identical (EXCL edits it). After it add:
"Protection plan routes (`{id}` = plan name). Deny rules are `protection-plans.<action>.deny` entries a custom role lists; built-in roles list none."

| Route | Authz | Purpose |
|---|---|---|
| `GET plans/protection/get` | Read, deny `viewprotectionplans` | List plans |
| `GET plans/protection/{id}/get` | Read, deny `viewprotectionplans` | Read one plan |
| `POST plans/protection/create` | Write, deny `createprotectionplan` | Create (discovery; sessions cannot send lifecycle or material keys) |
| `PATCH plans/protection/{id}/patch` | Write, deny `editprotectionplan` | Patch (sessions: metadata keys only) |
| `DELETE plans/protection/{id}/delete` | Internal | Delete the CR; users delete through discovery's `clear`, which removes deployed policies first |

In the reports table change the Authz cell of the list row to "Read (protection plans), deny `viewprotectionplanreports`" and of the download row to "Read (protection plans), deny `downloadprotectionplanreport`". After the reports table add one sentence: "Environment and tag categories (`plan-environments`, `plan-tags`) follow the `protection-plans` scope: create needs Contributor (deny `addprotectionplancategory`), edit and delete need Owner (deny `editprotectionplancategory` / `deleteprotectionplancategory`)." No INSTALL/chart/VALUES change (no config).

**Test first:** `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/rules_test.go` — `TestRuleKeysMatchDashboardVocabulary`, `TestReportRoutesRequirements`, `TestPlanDeleteIsInternal`, `TestGuardCategoryScope`, `TestGuardCategoryScopePlanTaxonomies`
- red: the 6 new vocabulary rows fail (rule "" on list/get/reports); TestPlanDeleteIsInternal fails (Denyable Own); guard_extra_test does not compile (GuardCategoryScope still takes a PermissionLevel; constants.CategoryOpCreate undefined)
- green: every row and case passes; the denied category rows answer ErrAuthzCategoryScopeDenied; the unknown operation answers ErrAuthzUnknownCategoryScope

**Verify:** cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./internal/tests/authz/... && go test ./... && golangci-lint run

### S3 — Discovery: a deny rule on every plan route, per-decision approve/reject rules in the decide handler, README

- **Phase:** P1 · **Part:** backend · **Repo:** telark/discovery · **dependsOn:** S1 · **parallelGroup:** discovery
- **Route:** claude-opus-5-5 / effort xhigh / P3 — criteria M3 — Authz enforcement on every user-facing plan route of the service that deploys policies, including a handler-level check.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/authz/requirements.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/handlers/plans/protection/decide.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/authz/requirements_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planhandlers/decide_authz_test.go`
  - `/Users/houssem/Desktop/Github/telark/services/discovery/README.md`

**Change:**

(1) internal/authz/requirements.go: add import `"context"`. Replace addPlans (lines 55-67) with:
```go
func addPlans(r map[string]authz.Requirement) {
	view := authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans)
	r[router.Key(base.Get, planseps.GetProtectionPlanTemplates)] = view
	r[router.Key(base.Get, planseps.GetProtectionPlanStatus)] = view
	r[router.Key(base.Get, planseps.GetProtectionPlanViolations)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanViolations)
	r[router.Key(base.Post, planseps.PrepareProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionCreateProtectionPlan)
	r[router.Key(base.Post, planseps.CancelProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionCancelProtectionPlan)
	r[router.Key(base.Post, planseps.DuplicateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionDuplicateProtectionPlan)
	r[router.Key(base.Post, planseps.ReactivateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionReactivateProtectionPlan)
	// Every field, including spec options added later, is edited through this one route.
	r[router.Key(base.Post, planseps.UpdateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlan)
	// Approve and reject share the route; the handler applies each decision's own rule.
	r[router.Key(base.Post, planseps.DecideProtectionPlan)] = authz.Own(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.GenerateProtectionPlanReport)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionGenerateProtectionPlanReport)
	r[router.Key(base.Delete, planseps.ClearProtectionPlan)] = authz.Denyable(
		authz.Own(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlan)
}
```
Keep ApprovePlanRequirement unchanged and add below it:
```go
func RejectPlanRequirement() authz.Requirement {
	return authz.Denyable(authz.Own(roledata.ScopeProtectionPlans), roledata.ActionRejectProtectionPlan)
}

// Internal peers pass, as they do in the route middleware.
func RequestAllows(ctx context.Context, requirement authz.Requirement) bool {
	identity, ok := authz.FromContext(ctx)
	return ok && (identity.Internal || authz.Allows(identity, requirement))
}
```
core/plans/protection/wire.go stays unchanged (notifier Requirement = ApprovePlanRequirement(): approval requests still go to users who may approve). If routes.go registers a protection-plans route missing from this table when the step applies (added by another workflow), give it `authz.Denyable(authz.Read(...), roledata.ActionViewProtectionPlans)` when it only reads, else the action of the verb it performs; never exempt it in the structural test.
(2) internal/handlers/plans/protection/decide.go: add import `discoveryauthz "github.com/telark/discovery/internal/authz"` (same alias as wire.go). Right after the body decode (line 40) and before the context/lock (line 42) insert:
```go
requirement := discoveryauthz.ApprovePlanRequirement()
if req.Decision == protection.DecisionRejected {
	requirement = discoveryauthz.RejectPlanRequirement()
}
if !discoveryauthz.RequestAllows(r.Context(), requirement) {
	respondError(w, http.StatusForbidden, dataerrors.ErrAuthzForbidden, nil)
	return
}
```
(the denial happens before lockPlanDecision and before svc.Decide; ValidateDecision and every other status mapping are unchanged).
(3) Tests. tests/authz/requirements_test.go: REPLACE TestDecideRouteIsDenyableOwner (FLAGGED: it pinned the shared approve rule on the route, which intentionally moves into the handler) with TestDecideRouteIsOwnerWithPerDecisionRules: the decide route requirement has Scope protection-plans, MinLevel Owner and Rule ""; ApprovePlanRequirement() has MinLevel Owner and Rule `xauthz.RuleKey(plans, ActionApproveProtectionPlan)`; RejectPlanRequirement() has MinLevel Owner and Rule `xauthz.RuleKey(plans, ActionRejectProtectionPlan)`. ADD TestPlanRuleKeysMatchDashboardVocabulary with exact strings: "GET /api/v1/plans/protection/templates" and "GET /api/v1/plans/protection/{id}/status" -> "protection-plans.viewprotectionplans.deny"; "GET /api/v1/plans/protection/{id}/violations" -> "protection-plans.viewprotectionplanviolations.deny"; "POST /api/v1/plans/protection/prepare" -> "protection-plans.createprotectionplan.deny"; "POST /api/v1/plans/protection/{id}/update" -> "protection-plans.editprotectionplan.deny"; "POST /api/v1/plans/protection/{id}/duplicate" -> "protection-plans.duplicateprotectionplan.deny"; "POST /api/v1/plans/protection/{id}/reactivate" -> "protection-plans.reactivateprotectionplan.deny"; "POST /api/v1/plans/protection/{id}/cancel" -> "protection-plans.cancelprotectionplan.deny"; "POST /api/v1/plans/protection/{id}/reports/generate" -> "protection-plans.generateprotectionplanreport.deny"; "DELETE /api/v1/plans/protection/{id}/clear" -> "protection-plans.deleteprotectionplan.deny". ADD TestEveryPlanRouteIsDenyable: for every requirement with Scope == roledata.ScopeProtectionPlans except the decide key, Rule must start with `roledata.ScopeProtectionPlans + "."` and end with ".deny" (comment: a plan route without a rule is an action no role can withhold; decide applies its rule per decision). ADD TestRequestAllows (table; ctx from `xauthz.WithIdentity(context.Background(), id)`): Internal identity -> true for approve and reject; Owner on protection-plans -> true for both; Owner with Denied{plans: [reject rule]} -> approve true, reject false; Contributor -> false for both; `context.Background()` without identity -> false.
NEW tests/planhandlers/decide_authz_test.go (package planhandlers; pattern of report_test.go and reshandlers/reset_test.go): `handlers.InitService(protection.NewService(nil, nil, nil, nil, nil, nil, nil, nil))` + `t.Cleanup(func() { handlers.InitService(nil) })`; a helper builds `httptest.NewRequest(http.MethodPost, "/api/v1/plans/protection/plan-a/decide", body)` with body `{"decision":"<d>","comment":"c","requestedAt":"2026-09-23T00:00:00Z"}`, header constants.HeaderUserID "u-approver", `mux.SetURLVars(req, map[string]string{constants.IDPathParam: "plan-a"})` and, when given, `xauthz.WithIdentity` of the identity. TestDecideRefusesDeniedDecision: rows {approve withheld: DecisionApproved + Owner denied ActionApproveProtectionPlan}, {reject withheld: DecisionRejected + Owner denied ActionRejectProtectionPlan} -> status 403 and body contains dataerrors.ErrAuthzForbidden. TestDecideRefusesMissingIdentity: no identity -> 403.
(4) services/discovery/README.md, API section. Change "renders an on-demand report (Write access):" to "renders an on-demand report (Write access; deny `generateprotectionplanreport`):" and "(Owner on `protection-plans` unless the `approveprotectionplan` deny flag is set)" to "(Owner on `protection-plans`; the `approveprotectionplan` and `rejectprotectionplan` deny rules withhold approving and rejecting separately)". Append as the LAST paragraph of the API section (after EXCL's exclusions paragraph if it is already there):
"Permissions: every plan route is gated on the `protection-plans` scope, and a custom role can withhold each action with the deny rule `protection-plans.<action>.deny` (built-in roles carry none). Templates and status: Read, `viewprotectionplans`. Violations: Read, `viewprotectionplanviolations`. Prepare: Write, `createprotectionplan`. Update, which covers every field including options added later: Write, `editprotectionplan`. Duplicate, reactivate, cancel: Write, `duplicateprotectionplan` / `reactivateprotectionplan` / `cancelprotectionplan`. Reports generate: Write, `generateprotectionplanreport`. Clear (delete): Owner, `deleteprotectionplan`. Decide: Owner, `approveprotectionplan` for an approval and `rejectprotectionplan` for a rejection. Report files include admission decisions, so withholding violations alone does not hide them. A new plan route must carry one of these rules (tests/authz TestEveryPlanRouteIsDenyable)."
No INSTALL/chart/VALUES change (no config).

**Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/authz/requirements_test.go` — `TestPlanRuleKeysMatchDashboardVocabulary`, `TestEveryPlanRouteIsDenyable`, `TestDecideRouteIsOwnerWithPerDecisionRules`, `TestRequestAllows`, `TestDecideRefusesDeniedDecision`, `TestDecideRefusesMissingIdentity`
- red: compile error: authz.RejectPlanRequirement / authz.RequestAllows undefined; the vocabulary and structural tests list 10 plan routes with an empty rule; without the handler check the decide handler reaches svc.Decide with a nil exporter instead of answering 403
- green: every plan route carries the pinned rule; decide is Owner with no route rule; RequestAllows honours Internal, level and the per-decision deny; both handler tests answer 403 before the lock

**Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/authz/... ./internal/tests/planhandlers/... && go build ./... && go test ./... && golangci-lint run

### U1 — UI permission foundation: ACTION_PERMISSIONS, scope index union, role editor area and rules, PermissionGate, taxonomy keys, denied strings

- **Phase:** P2 · **Part:** ui · **Repo:** dashboard-ui · **dependsOn:** S1 · **parallelGroup:** ui-a
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — UI mirror of the contract; the server stays authoritative; no M1-M5 criterion on the UI side; type-checked.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/auth/store/slices/permissionsSlice.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/roles/constants/scopeRules.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/roles/constants/roles.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryActionsColumn.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/PlanTaxonomyPage.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/phaseRules.ts`

**Change:**

(1) src/features/auth/hooks/permissions/permissionEngine.tsx, ACTION_PERMISSIONS.protectionPlans (lines 211-257): keep viewViolations, create, edit, cancel, duplicate, reactivate, delete and approve byte-identical; DELETE `editTaxonomy`; ADD, in the same object shape (`scope: 'protection-plans' as const, level: '<Level>' as PermissionLevel, deny: '<full key>'`):
- view: ReadOnly, 'protection-plans.viewprotectionplans.deny'
- viewReports: ReadOnly, 'protection-plans.viewprotectionplanreports.deny'
- downloadReport: ReadOnly, 'protection-plans.downloadprotectionplanreport.deny'
- generateReport: Contributor, 'protection-plans.generateprotectionplanreport.deny'
- reject: Owner, 'protection-plans.rejectprotectionplan.deny'
- addCategory: Contributor, 'protection-plans.addprotectionplancategory.deny'
- editCategory: Owner, 'protection-plans.editprotectionplancategory.deny'
- deleteCategory: Owner, 'protection-plans.deleteprotectionplancategory.deny'
(levels equal the backend route levels in the matrix). PermissionGate (lines 295-309): props gain `action?: string` and `fallback?: React.ReactNode`; destructure `fallback = null`; body `const allowed = usePermission(requiredScope, requiredLevel, action); return <>{allowed ? children : fallback}</>;`. It has no caller today, so nothing else changes.
(2) src/features/auth/store/slices/permissionsSlice.ts buildScopeIndex (lines 18-60): a new entry stores `rules: [...rules]`; when an entry exists compute `const merged = Array.from(new Set([...existing.rules, ...rules]));`, store `{ level, rules: merged, priority }` when the incoming role wins, otherwise `existing.rules = merged`. One comment line: deny rules add up across roles, as the backend applies every active role's rules. Level/priority selection unchanged.
(3) src/features/access-and-permissions/roles/constants/scopeRules.ts, protection-plans block (lines 100-117) becomes:
ReadOnly: [{ key: 'viewprotectionplans', label: 'View Protection Plans' }, { key: 'viewprotectionplanviolations', label: 'View Violations' }, { key: 'viewprotectionplanreports', label: 'View Reports' }, { key: 'downloadprotectionplanreport', label: 'Download Reports' }];
Contributor: [{ key: 'createprotectionplan', label: 'Create Protection Plan' }, { key: 'editprotectionplan', label: 'Edit Protection Plan' }, { key: 'cancelprotectionplan', label: 'Cancel Protection Plan' }, { key: 'duplicateprotectionplan', label: 'Duplicate Protection Plan' }, { key: 'reactivateprotectionplan', label: 'Reactivate Protection Plan' }, { key: 'generateprotectionplanreport', label: 'Generate Reports' }, { key: 'addprotectionplancategory', label: 'Add Environments and Tags' }];
Owner: [{ key: 'deleteprotectionplan', label: 'Delete Protection Plan' }, { key: 'approveprotectionplan', label: 'Approve Protection Plan' }, { key: 'rejectprotectionplan', label: 'Reject Protection Plan' }, { key: 'editprotectionplancategory', label: 'Edit Environments and Tags' }, { key: 'deleteprotectionplancategory', label: 'Delete Environments and Tags' }];
Admin: [].
(4) src/features/access-and-permissions/roles/constants/roles.ts SCOPE.DEFAULT_AREAS (line 228): append `{ key: 'protection-plans', label: 'Protection Plans' }`. The role create form then seeds it at ReadOnly like every area, and the role/user/group views label plan scopes.
(5) src/features/access-and-permissions/categories/components/display/list/CategoryActionsColumn.tsx PERMISSIONS_BY_SCOPE (lines 36-43): PLAN_ENVIRONMENTS and PLAN_TAGS -> `{ edit: ACTION_PERMISSIONS.protectionPlans.editCategory, delete: ACTION_PERMISSIONS.protectionPlans.deleteCategory }` (disabled + EDIT_/DELETE_PERMISSION_DENIED_TOOLTIP behaviour unchanged).
(6) src/features/plans/protection/pages/PlanTaxonomyPage.tsx canAdd (lines 54-58) -> `ACTION_PERMISSIONS.protectionPlans.addCategory` (.scope/.level/.deny); disabled + ADD_CATEGORY_DISABLED_TOOLTIP behaviour unchanged.
(7) src/features/plans/protection/constants/protectionPlans.ts: add, as a new key of LABELS placed directly after LABELS.REPORTS (after line 61, before TAXONOMY), and no other hunk in this file (EXCL edits CREATE_PAGE.FORM / DETAIL_PAGE.FIELDS, APPS removes CARD_LAYOUT/ACCENT_TINT):
```ts
PERMISSION_DENIED: {
  CREATE: 'You do not have permission to create protection plans',
  EDIT: 'You do not have permission to edit protection plans',
  DUPLICATE: 'You do not have permission to duplicate protection plans',
  REACTIVATE: 'You do not have permission to reactivate protection plans',
  CANCEL: 'You do not have permission to cancel protection plans',
  DELETE: 'You do not have permission to delete protection plans',
  APPROVE: 'You do not have permission to approve protection plans',
  REJECT: 'You do not have permission to reject protection plans',
  GENERATE_REPORT: 'You do not have permission to generate reports',
  DOWNLOAD_REPORT: 'You do not have permission to download reports',
},
```
(8) src/features/plans/protection/utils/phaseRules.ts: after getDecideBlockedTooltip add `export const permissionTooltip = (allowed: boolean, denied: string, otherwise?: string): string | undefined => (allowed ? otherwise : denied);` (the permission reason always wins over a phase/state reason).

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx` — `npm run check-all`, `grep -c "'protection-plans\." src/features/auth/hooks/permissions/permissionEngine.tsx`
- red: deleting editTaxonomy makes tsc report CategoryActionsColumn.tsx (property editTaxonomy does not exist) until (5) lands; grep counts 9 plan deny keys
- green: check-all reports zero errors; grep counts 16 plan deny keys, and each equals a string pinned by the S2/S3 vocabulary tests; the scopeRules protection-plans block holds the same 16 keys at the same levels

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all (errors in files this step did not touch are reported to the orchestrator, not fixed here); user screenshot: role editor showing the Protection Plans row with its rules per level

### U2 — UI action surfaces: details toolbar, card menu and list Create follow the disabled-with-tooltip rule; reject gets its own key

- **Phase:** P2 · **Part:** ui · **Repo:** dashboard-ui · **dependsOn:** U1 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Component wiring over keys defined in U1; the server stays authoritative.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlanDetailsToolbar.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlansToolbar.tsx`

**Change:**

Rule for every control below: an action the phase allows is always rendered; the permission check only disables it and sets the tooltip via permissionTooltip(allowed, PD.<ACTION>, <existing phase/state tooltip>) where `const PD = PPC.LABELS.PERMISSION_DENIED;`. Phase rules (APPROVABLE/REACTIVATABLE/CANCELLABLE/NON_EDITABLE) are unchanged. Add permissionTooltip to each file's existing phaseRules import (toolbar: '../../utils/phaseRules' line 26; card: '../../../utils/phaseRules' line 55).
(1) components/layout/ProtectionPlanDetailsToolbar.tsx: add `canReject = usePermission(...protectionPlans.reject.scope/.level/.deny)` beside canApprove (lines 90-94). Toolbar builder: approve/reject are pushed whenever `APPROVABLE_PHASES.includes(phase)` (drop `canApprove &&`): approve `disabled: !canApprove || decideDisabled`, `tooltip: permissionTooltip(canApprove, PD.APPROVE, blockedTooltip)`; reject `disabled: !canReject || decideDisabled`, `tooltip: permissionTooltip(canReject, PD.REJECT, blockedTooltip)`. Reactivate is pushed whenever `REACTIVATABLE_PHASES.includes(phase)`: `disabled: !canReactivate || reactivating || expired`, `tooltip: permissionTooltip(canReactivate, PD.REACTIVATE, expired ? PPC.LABELS.DETAIL_PAGE.ACTIONS.REACTIVATE_DISABLED_EXPIRED_TOOLTIP : undefined)`. Edit is always pushed: `disabled: !canEdit || editing || editDisabled`, `tooltip: permissionTooltip(canEdit, PD.EDIT, editDisabled ? PPC.LABELS.DETAIL_PAGE.ACTIONS.EDIT_DISABLED_TOOLTIP : undefined)`. Duplicate is always pushed: `disabled: !canDuplicate || duplicating`, `tooltip: permissionTooltip(canDuplicate, PD.DUPLICATE)`. refreshHealth unchanged. The 'more' button is always pushed; items: cancel when `CANCELLABLE_PHASES.includes(phase)` with `disabled: !canCancel || cancelling, title: permissionTooltip(canCancel, PD.CANCEL)`; delete always with `disabled: !canDelete || deleting, title: permissionTooltip(canDelete, PD.DELETE)`. Add canReject to the memo deps. If the builder's cognitive complexity exceeds 12, move the decide pair and the overflow items into two module-level helper functions in the same file (no new file).
(2) components/cards/view/ProtectionPlanCard.tsx. Hunk limited to the usePermission block (lines 230-259), the menuItems construction (lines 424-495) and the phaseRules import line. APPS AP1 edits the imports/styles/elements of this file and never these regions. Add `canRejectPlan` (protectionPlans.reject). Menu items: approve/reject when `canDecide` with `disabled: !canApprovePlan || decideDisabled` / `!canRejectPlan || decideDisabled` and `title: permissionTooltip(canApprovePlan, PD.APPROVE, decideBlockedTooltip)` / `permissionTooltip(canRejectPlan, PD.REJECT, decideBlockedTooltip)`; edit always with `disabled: !canEditPlan || editDisabled || editPanelOpen, title: permissionTooltip(canEditPlan, PD.EDIT, editDisabled ? PPC.LABELS.DETAIL_PAGE.ACTIONS.EDIT_DISABLED_TOOLTIP : undefined)`; duplicate always with `disabled: !canDuplicatePlan || duplicatePanelOpen, title: permissionTooltip(canDuplicatePlan, PD.DUPLICATE)`; reactivate when `canReactivate` with `disabled: !canReactivatePlan || reactivating || reactivateExpired, title: permissionTooltip(canReactivatePlan, PD.REACTIVATE, reactivateExpired ? PPC.LABELS.DETAIL_PAGE.ACTIONS.REACTIVATE_DISABLED_EXPIRED_TOOLTIP : undefined)`; cancel when `canCancel` with `disabled: !canCancelPlan || cancelling, title: permissionTooltip(canCancelPlan, PD.CANCEL)`; then the divider and delete always with `disabled: !canDeletePlan || deleting, title: permissionTooltip(canDeletePlan, PD.DELETE)`. The `menuItems.length > 0` render guard (line 651), handlers, modals, panels and navigation stay untouched.
(3) components/layout/ProtectionPlansToolbar.tsx: remove `if (!canCreate) return [search];` (line 130); the create-plan button is always present with `disabled: !canCreate` and `tooltip: canCreate ? undefined : PPC.LABELS.PERMISSION_DENIED.CREATE`. The Organize dropdown stays as is (the page itself is gated on view by U4). ProtectionPlansEmptyPage.tsx is unchanged: it already uses the create key, and EmptyState has no tooltip.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlanDetailsToolbar.tsx` — `npm run check-all`, `grep -nE 'if \((canEdit|canDuplicate|canDelete|canApprove)\)|\(canApprove &&|\(canReactivate &&|\.\.\.\(can(Approve|Edit|Duplicate|Reactivate|Cancel)Plan|if \(canDeletePlan\)|if \(!canCreate\) return' src/features/plans/protection/components/layout/ProtectionPlanDetailsToolbar.tsx src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx src/features/plans/protection/components/layout/ProtectionPlansToolbar.tsx`
- red: grep lists the permission-conditional pushes/spreads that hide actions today
- green: grep is empty; check-all reports zero errors; every permission-disabled control carries a PPC.LABELS.PERMISSION_DENIED tooltip/title

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshots: ReadOnly user on a plan's details (Edit, Duplicate and overflow Delete disabled with tooltips) and card menu; Owner denied reject on a pending plan (Reject disabled with its tooltip, Approve enabled); Contributor denied create (Create disabled with tooltip)

### U3 — UI details sections: violations and reports gated on their view keys; generate/download keys and tooltips

- **Phase:** P2 · **Part:** ui · **Repo:** dashboard-ui · **dependsOn:** U1 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Section gating in one page plus two hooks; server stays authoritative.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/details/ReportsSection.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanViolations.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanReports.ts`

**Change:**

(1) src/features/plans/protection/hooks/usePlanViolations.ts: signature `usePlanViolations(planId: string, enabled: boolean)`; first statement of the fetch effect `if (!enabled) return;`; add `enabled` to its deps. Nothing else changes.
(2) src/features/plans/protection/hooks/usePlanReports.ts: signature `usePlanReports(planId: string, revision: string, enabled: boolean)`; same guard as the first statement of the list effect; add `enabled` to its deps. generate/download unchanged.
(3) src/features/plans/protection/pages/details/Content.tsx: replace the canGenerateReport block that borrowed protectionPlans.cancel (lines 112-116) with four checks: canViewViolations (viewViolations), canViewReports (viewReports), canGenerateReport (generateReport), canDownloadReport (downloadReport), each `usePermission(entry.scope, entry.level, entry.deny)`. Call `usePlanViolations(plan.id, canViewViolations)` and `usePlanReports(plan.id, `${plan.phase}:${plan.lastUpdatedAt ?? ''}`, canViewReports)`. Render the Violations SettingsCard (line ~563) only when canViewViolations and the Reports SettingsCard (line ~598) only when canViewReports. Generate button: `disabled={!canGenerateReport || reportNotStarted || !currentUserId}` and its Tooltip `title={permissionTooltip(canGenerateReport, PPC.LABELS.PERMISSION_DENIED.GENERATE_REPORT, reportNotStarted ? PPC.LABELS.REPORTS.NOT_STARTED_HINT : undefined)}` (add permissionTooltip to the existing `import { isRejected } from '../../utils/phaseRules'` at line 36). Pass `canDownload={canDownloadReport}` to ReportsSection. Header, Overview, Scope card (EXCL edits it, lines 427-467) and Health card untouched.
(4) src/features/plans/protection/components/details/ReportsSection.tsx: new prop `canDownload: boolean`; in `REPORT_FORMATS.map` (line 131) every download Button gets `disabled={!canDownload}`, and the wrapper Tooltip title becomes `!canDownload ? PPC.LABELS.PERMISSION_DENIED.DOWNLOAD_REPORT : key === 'html' ? LABELS.PRINT_HINT : undefined`. As today, the Tooltip is rendered only when a title exists.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx` — `npm run check-all`, `grep -n 'protectionPlans.cancel' src/features/plans/protection/pages/details/Content.tsx`
- red: grep shows the Generate report button gated by the cancel key
- green: grep is empty; check-all reports zero errors; usePlanViolations/usePlanReports skip their fetch when their key is withheld

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshots: violations-denied (no Violations card), reports-view-denied (no Reports card), download-denied (download buttons disabled with tooltip), generate-denied (Generate disabled with the permission tooltip)

### U4 — UI navigation: plan routes, sidebar entry and home widgets gated on the view key

- **Phase:** P2 · **Part:** ui · **Repo:** dashboard-ui · **dependsOn:** U1 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Three small gating edits using the U1 key and the existing PermissionGate.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/routes/AppRoutes.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/layout/sidebar/MenuButtons.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/home/pages/Dashboard.tsx`

**Change:**

(1) src/routes/AppRoutes.tsx: import `PermissionGate, ACTION_PERMISSIONS` from '../features/auth/hooks'; module-level `const { view: viewPlans } = ACTION_PERMISSIONS.protectionPlans;`. In both plan routes (APP_ROUTES.PROTECTION_PLANS line ~139 and PROTECTION_PLAN_DETAILS line ~149) wrap the FeatureErrorBoundary, inside ProtectedRoute, with `<PermissionGate requiredScope={viewPlans.scope} requiredLevel={viewPlans.level} action={viewPlans.deny} fallback={<Navigate to={APP_ROUTES.HOME} replace />}>` (Navigate is already imported). Other routes untouched.
(2) src/components/layout/sidebar/MenuButtons.tsx ProtectionPlansMenuButton (lines 101-117): import `usePermission, ACTION_PERMISSIONS` from '../../../features/auth/hooks' (Sidebar.tsx already imports from features/); module-level `const { view: viewPlans } = ACTION_PERMISSIONS.protectionPlans;`; inside the component after useLocation: `const canView = usePermission(viewPlans.scope, viewPlans.level, viewPlans.deny); if (!canView) return null;`. Other menu buttons untouched.
(3) src/features/home/pages/Dashboard.tsx: next to `const { viewSnapshots } = ACTION_PERMISSIONS.applications;` (line 51) add `const { view: viewPlans } = ACTION_PERMISSIONS.protectionPlans;`; line 56 becomes `const canViewPlans = usePermission(viewPlans.scope, viewPlans.level, viewPlans.deny);`. Everything that already keys on canViewPlans (fetch and widgets) follows.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/routes/AppRoutes.tsx` — `npm run check-all`, `grep -c PermissionGate src/routes/AppRoutes.tsx`, `grep -n "usePermission('protection-plans', 'ReadOnly')" src/features/home/pages/Dashboard.tsx`
- red: PermissionGate count 0; Dashboard uses the bare level check
- green: PermissionGate appears in both plan routes (plus the import); the Dashboard grep is empty; check-all reports zero errors

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshots: view-denied user (no sidebar Plans entry; /governance/plans/protection lands on home; no home plan widgets)

### U5 — UI: APPS coverage read gate on the view key

- **Phase:** P2 · **Part:** ui · **Repo:** dashboard-ui · **dependsOn:** U1 · **crossPlanDependsOn:** APPS:AP2 · **parallelGroup:** ui-b
- **Route:** claude-opus-5-5 / effort low / P4 — criteria none — One import, one destructure and one gate line in a file APPS AP2 created; a no-op when AP2 already used the view key.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/hooks/useApplicationCoverage.ts`

**Change:**

Runs after U1 and after APPS AP2 (cross-plan edge, D14). src/features/resources/applications/hooks/useApplicationCoverage.ts is created by APPS AP2. If its read gate already calls usePermission with ACTION_PERMISSIONS.protectionPlans.view (AP2 applied after U1 and took its D5 branch), change nothing. Otherwise:
- the import from '../../../auth/hooks' gains ACTION_PERMISSIONS;
- add module-level `const { view: viewPlans } = ACTION_PERMISSIONS.protectionPlans;` after the imports;
- `const canViewPlans = usePermission('protection-plans', 'ReadOnly');` becomes `const canViewPlans = usePermission(viewPlans.scope, viewPlans.level, viewPlans.deny);`.
The rest of the hook (its comment, the useProtectionPlans(canViewPlans) call, the index and the return shape) stays APPS-owned and untouched. A view-denied user then dispatches no plan fetch and sees no coverage section, instead of a 403 from exporter plans/protection/get (Denyable viewprotectionplans after S2). If the file does not exist, AP2 has not run: stop and report the broken edge; never create the file.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/hooks/useApplicationCoverage.ts` — `grep -n "usePermission('protection-plans'" src/features/resources/applications/hooks/useApplicationCoverage.ts`, `grep -c 'ACTION_PERMISSIONS.protectionPlans' src/features/resources/applications/hooks/useApplicationCoverage.ts`, `npm run check-all`
- red: the first grep prints AP2's bare level gate (when AP2 already used the view key, U5 is a checked no-op)
- green: the first grep is empty; the second prints 1 or more; check-all reports zero errors

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all; user screenshot (after APPS AP3): view-denied user on Applications (no 'Protected by' section, no plans/protection/get 403)

### V1 — Verify internal/data: go test + golangci-lint

- **Phase:** P3 · **Part:** verify · **Repo:** internal/data · **dependsOn:** S1 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Machine-checked gate.
- **Files:**
  - `/Users/houssem/Desktop/Github/internal/data`

**Change:**

Run `go test ./...` and `golangci-lint run` in internal/data. Fix every failure this plan caused. Report unrelated failures without fixing them.

**Test first:** `/Users/houssem/Desktop/Github/internal/data/tests/role/actions_test.go` — `go test ./...`, `golangci-lint run`
- red: n/a
- green: exit 0, zero issues

**Verify:** cd /Users/houssem/Desktop/Github/internal/data && go test ./... && golangci-lint run

### V2 — Verify exporter: go test + golangci-lint

- **Phase:** P3 · **Part:** verify · **Repo:** telark/exporter · **dependsOn:** S2 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Machine-checked gate.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/exporter`

**Change:**

Run `go test ./...` and `golangci-lint run` in services/exporter (workspace mode). Fix every failure this plan caused. Report unrelated failures, for example from the in-flight Local AI Analyzer, without fixing them.

**Test first:** `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/rules_test.go` — `go test ./...`, `golangci-lint run`
- red: n/a
- green: exit 0, zero issues

**Verify:** cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./... && golangci-lint run

### V3 — Verify discovery: go build + go test + golangci-lint

- **Phase:** P3 · **Part:** verify · **Repo:** telark/discovery · **dependsOn:** S3 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Machine-checked gate.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark/services/discovery`

**Change:**

Run `go build ./...`, `go test ./...` and `golangci-lint run` in services/discovery. Fix every failure this plan caused, and report unrelated ones.

**Test first:** `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/authz/requirements_test.go` — `go test ./...`, `golangci-lint run`
- red: n/a
- green: exit 0, zero issues

**Verify:** cd /Users/houssem/Desktop/Github/telark/services/discovery && go build ./... && go test ./... && golangci-lint run

### V4 — Verify dashboard-ui: npm run check-all + vocabulary greps

- **Phase:** P3 · **Part:** verify · **Repo:** dashboard-ui · **dependsOn:** U2, U3, U4, U5 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Machine-checked gate.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui`

**Change:**

Run `npm run check-all` and fix every error this plan caused, without eslint-disable, any, console.* or hex. Format only this plan's files with `npx prettier --write <file>`; never run `npm run format` or `lint:f`, because they rewrite other workflows' uncommitted files. Then check that the 16 deny keys in permissionEngine.tsx equal the 16 strings pinned by the exporter/discovery vocabulary tests, and that scopeRules.ts lists the same keys at the same levels. Finally `grep -rn "usePermission('protection-plans'" src` must print nothing: every plan read gates on ACTION_PERMISSIONS.protectionPlans.view (Dashboard.tsx via U4, useApplicationCoverage.ts via U5).

**Test first:** `/Users/houssem/Desktop/dashboard-ui/package.json` — `npm run check-all`, `grep -rn "usePermission('protection-plans'" src`
- red: n/a
- green: zero errors; key sets equal; the bare plan-read grep is empty

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all

### R1 — Adversarial review of the permission change set

- **Phase:** P3 · **Part:** verify · **Repo:** telark · **dependsOn:** V1, V2, V3, V4 · **parallelGroup:** review
- **Route:** claude-opus-5-5 / effort xhigh / P3 — criteria M3 — Adversarial verification of a security-boundary change; the routing rules put catch-a-mistake reviews at xhigh.
- **Files:**
  - `/Users/houssem/Desktop/Github/telark`
  - `/Users/houssem/Desktop/Github/internal/data`
  - `/Users/houssem/Desktop/dashboard-ui`

**Change:**

Read-only review against this plan. Report findings as file:line; fix only trivial ones in place and record the rest as follow-ups. Checklist:
(1) Every route in discovery and exporter whose Scope is protection-plans carries exactly the action and level listed in the matrix. Decide is Own with no route rule, and its handler checks the approve or reject requirement before lockPlanDecision and svc.Decide. Exporter plan DELETE is Internal.
(2) The 16 strings in internal/data, the backend vocabulary tests, ACTION_PERMISSIONS and SCOPE_RULES are identical, and each UI level equals the backend MinLevel.
(3) GuardCategoryScope keeps the exact old semantics for groups/roles (compare with the removed categoryActions map), refuses an unknown scope or operation, and the category handler passes the right operation at all four call sites.
(4) RequestAllows passes Internal identities, refuses when the identity is missing, and uses xauthz.Allows unchanged.
(5) No UI plan action is hidden only because of a permission check. Every permission-disabled control shows a PERMISSION_DENIED string, and the permission reason takes precedence over phase/state reasons.
(6) usePlanViolations/usePlanReports do not fetch when their key is withheld.
(7) PermissionGate is used only inside ProtectedRoute, so it never redirects before permissions are ready.
(8) buildScopeIndex merges rules from all roles, including ALL-scope roles, as x-ware does, and level selection is unchanged.
(9) The DEFAULT_AREAS addition renders in the role editor, the role view, and the user/group role lists.
(10) Every test lives under internal/tests/<area>/ or the module's tests/ dir, and each flagged test edit is listed in the PR body.
(11) The READMEs list every route and action. The UI has no any, console.*, hex or vendor name, and its strings live in constants.
(12) The EXCL and APPS hunks in shared files are untouched; in useApplicationCoverage.ts PERM changed only the auth/hooks import, the viewPlans destructure and the canViewPlans line (U5).

**Test first:** `/Users/houssem/Desktop/Github/telark/.claude/plans/protection-plans-permissions.plan.md` — `review checklist items 1-12`
- red: n/a
- green: no high-severity finding left open

**Verify:** The review report goes into the PR body. Every finding is either closed or listed under user-owned follow-ups.

## Docs

- /Users/houssem/Desktop/Github/telark/services/exporter/README.md — API section: a new protection plan routes table with the Authz column (list/get Read + viewprotectionplans; create Write + createprotectionplan; patch Write + editprotectionplan; delete Internal, since users delete through discovery clear); the reports table Authz cells name viewprotectionplanreports and downloadprotectionplanreport; one sentence on the environment/tag category rules. The lifecycle-keys paragraph is left to EXCL. (S2)
- /Users/houssem/Desktop/Github/telark/services/discovery/README.md — API section: the generate sentence names generateprotectionplanreport; the decide sentence names the separate approve/reject rules; a new Permissions paragraph gives every plan route with its level and deny action, notes that update covers future options, that report files contain admission decisions, and that a new plan route must carry a rule. (S3)
- No change needed: docs/INSTALL.md, charts/telark/README.md, charts/telark/VALUES.md (no values, env or install step changes); docs/CRDS.md and charts/telark-crds/README.md (no CRD change); dashboard-ui README (it already says the permission set gates routes and actions).

## Risks

- Concurrent edits to shared files. Exporter internal/authz/guard.go is edited by the Local AI Analyzer (globalconfig section) and by PERM (category section). Both service READMEs are edited by EXCL and PERM. protectionPlans.ts, Content.tsx and ProtectionPlanCard.tsx are edited by EXCL/APPS and PERM. useApplicationCoverage.ts is created by APPS AP2 and edited by PERM U5, which must run after AP2 (cross-plan edge, D14). Mitigation: each step names its exact region and D14 lists the disjoint hunks. The merged schedule must run one step at a time per file.
- Visible UI change: denied plan actions are now shown disabled with a reason instead of hidden, so ReadOnly users see more controls. This matches Applications. Screenshot review is in P2.
- Semantics change for any custom role saved before this plan: an approve deny no longer blocks reject, and plan create/edit denies no longer block environment/tag changes. The features are unreleased, so there is no migration; see edge case and D15.
- Exporter plan DELETE becomes Internal: any out-of-band automation that deletes plans with a session gets 401. No such caller is known; the UI uses discovery clear.
- CI with GOWORK=off stays red until the user releases internal/data (the new constants ride in the same unreleased batch as Plans 1-3) and bumps the service pins.
- The UI still selects levels differently from x-ware when a user holds both an ALL role and a lower explicit scope role (x-ware prefers the explicit level). This predates the plan and is a follow-up; deny rules now match exactly.
- The structural tests (TestEveryPlanRouteIsDenyable and the vocabulary pins) fail any later plan route that has no rule. That is intended, and EXCL/APPS add no plan route.
- New custom roles created in the editor now include protection-plans at ReadOnly by default, like every other area, so they can see plans unless the author removes the row or adds the view deny.
- Duplicate is not blocked by a create deny, because it is a separate action. This is documented in the discovery README and in the edge cases.

## Out of scope

- Level-merge semantics when a user holds an ALL role and a lower explicit scope role (x-ware grantedLevel prefers the explicit level; the UI takes the maximum). Affects every scope.
- auth-service isRoleExpired treats an unparsable expiresAt as active, while x-ware treats it as expired (auth handlers/authorisation/permissions.go:119-121 vs x-ware authz/grants.go:127-130).
- Group/role category deletion checking the edit rule, while the UI and the data constants define deletegroupcategory/deleterolecategory. Behaviour is kept as-is.
- ProtectedRoute ignoring requiredScope/minimumLevel on the roles/users/groups routes, and ungated sidebar entries for features other than plans.
- A skipprotectionplanapproval key (see D9), plan-scoped rollback/snapshot actions, a user-facing terminate action.
- Making exporter plan create/patch Internal, and deleting GuardPlanLifecycle/PlanLifecycleFields (approval plan; EXCL relies on them).
- Deny rules on built-in roles; permission checks inside plan panels/modals (their openers are gated and the server is authoritative); notifying reject-only approvers.
- Any version bump, build, push, tag, chart publish or deploy; internal/composer; release-manager.

## User-owned follow-ups

- Release internal/data (8 new action constants, same unreleased batch as Plans 1-3), then bump the pins in services/{discovery,exporter,notifier,auth}/go.mod. CI (GOWORK=off) stays red until then.
- Roll out exporter and discovery before the dashboard bundle. Both orders are safe: an old UI on the new backend sees 403 only for new-key denies, and a new UI on the old backend is stricter than the backend. Rollback runs in reverse and needs no data change.
- Manual check on the dev cluster (P1 acceptance): one custom role per deny key, confirming 403 on that route and 200 elsewhere; approve-denied vs reject-denied Owners; a session DELETE on the exporter plan route returns 401; environment/tag add, edit and delete keys.
- Screenshot review of every surface listed in P2 acceptance, especially how dense the ReadOnly details toolbar and card menu look with disabled controls.
- Decide the ALL-vs-explicit level semantics (x-ware grantedLevel vs UI buildScopeIndex) and align one side.
- Decide whether group/role category deletion should check deletegroupcategory/deleterolecategory (one line each in categoryRequirements; it changes enforcement for existing custom roles).
- Decide whether ProtectedRoute should honour requiredScope/minimumLevel on the roles/users/groups routes.
- If a 'choose automatic in Production' permission is wanted, ask for skipprotectionplanapproval. It needs gates at prepare, duplicate and on an environment relabel in update.

## Revision log

| Id | Severity | Finding | Resolution |
|---|---|---|---|
| R0 | n/a | Initial synthesis | v1 written from the perm-backend and perm-ui terrain. Load-bearing claims were re-verified in code: both requirement maps, the guard's category section and its call sites, the decide handler order and identity context, x-ware Allows/grants merging, the UI engine/slice/scopeRules/DEFAULT_AREAS, that the UI delete client calls discovery clear, that no UI test runner exists, and that the EXCL/APPS plan files touch no authz code and no plan route. |
| R1 | high | D14/U4 (cross-plan edge with APPS AP2): the APPS coverage read was said to gate on ACTION_PERMISSIONS.protectionPlans.view, but no PERM step made it so and no dependsOn edge ordered APPS AP2 after U1 (applications-cards-coverage.plan.md:161 AP2 dependsOn —; :258 its view-key switch is conditional). In a parallel merged schedule AP2 ships usePermission('protection-plans', 'ReadOnly'); after S2 a view-denied user still fetches exporter plans/protection/get, gets 403, and Applications shows an error instead of hiding the section. | Folded in with the PERM-side fix (PERM files only): new step U5 (dependsOn U1; crossPlanDependsOn APPS:AP2) switches useApplicationCoverage.ts to the view key and is a checked no-op when AP2 already used it. D14 states the cross-plan edge and lists the file under shared hunks; V4 dependsOn U5 and greps src for any bare usePermission('protection-plans' check; P2 steps/acceptance, D4, the matrix, the view-denied edge case, R1 item 12 and Risks name U5. The APPS-side edge (AP2 dependsOn PERM U1, view key unconditional) is left to the APPS plan; with U5 either order ends on the view key. |
