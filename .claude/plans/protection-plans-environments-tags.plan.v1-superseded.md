# Protection Plans — Environments + Tags (v1 plan, SUPERSEDED — input material only)
**Status:** v1 plan from 2026-09-19, produced by a lower-capability model. SUPERSEDED on 2026-09-22 by a from-scratch re-plan; treat as input material, not authoritative.
**Critique verdict at the time:** not sound as written — two critical + six high findings (below).
**Recorded decision Q1 (2026-09-19):** environment and tags are metadata only; the policy renderer is untouched.
_Source: planning workflow `wf_7d36e9c4-823` (11 agents) — raw result in `/Users/houssem/.claude/projects/-Users-houssem-Desktop-claude/bdc6e640-0d86-48b1-864a-5dbd0f2f88d0/workflows/wf_7d36e9c4-823.json`. Remarks about a concurrent dependency-upgrade workflow and 40 staged exporter lint files date from 2026-09-19._

## Overview

# Protection Plans — Environment & Tag taxonomies, end to end

## The short version

Environment and Tag are not new entities. They are two new **scope values** on the Category mechanism that already powers group and role categories.

I checked whether that was true rather than assuming it, and it holds at every layer but one:

| Layer | Absorbs a new scope? | Evidence |
|---|---|---|
| Category CRD | Yes, unchanged | `scope` is `type: string, maxLength: 50`, **no enum**. Its description already reads "(e.g., groups, roles, workloads)". |
| Built-in seeding | Yes, unchanged | `seedCategories()` just calls `WithBuiltins(existing)` and upserts. Scope-blind. |
| Validation | Yes, unchanged | `ValidateAndPrepareCategory` checks name and scope non-empty. No whitelist. |
| Cache | Yes, unchanged | `InvalidateCategoryCaches` derives the scope set at runtime from stored rows. |
| Routes | Yes, unchanged | The read route is literally `classification/categories/scope/{scope}/get`. |
| Route authz | Yes, unchanged | All six routes are `authz.Authenticated`; writes defer to the guard. |
| **Authz guard** | **No — the one blocker** | `GuardCategoryScope` uses the taxonomy scope *as* the authz scope and hard-denies unknown scopes. |
| UI endpoints / clients | Yes, unchanged | Everything is `GET_BY_SCOPE(scope)`. |
| Redux store | Yes, unchanged | State is `categoriesByScope: Record<string, Category[]>`. |
| Dropdown factory | Yes, near-unchanged | `getManageCategoriesButtonConfig`'s own doc comment offers it to "any feature that lists categories by scope". |
| Add/Edit panels | Type-wise yes, labels no | The `scope` prop type is derived from `SCOPES`, so it widens for free — but every label says "Category". |

So the backend is small. **The weight is in the frontend.**

## Honest sizing

Roughly **30 files**. Not a small change, and not evenly distributed:

- **Backend: 6 files.** One authz map promoted, one shared guard added, one CRD field pair, four discovery enumeration sites. That is all, because the Category stack is already generic.
- **Shared Go modules: 4 files.** Two of which are unreleased and become your problem to release and pin.
- **Frontend: ~16 files.** This is where the real work is, and it is in three unglamorous places: parametrising the labels on the shared category components (they all hardcode the word "Category"), building the two views, and threading two fields through the plan form pipeline, which touches five separate files to add one field pair.
- **Docs: 5 files.**

## The one decision that blocks

**Q1: metadata or enforcement?** Does tagging a plan `Production` merely label it, or does it change which workloads the plan protects?

I recommend **metadata only**. `buildScopes` in the renderer switches solely on `plan.Scope.Type`, and `RenderMeta` carries only `{PlanID, PlanName, CreatedBy, Mode}` — enforcement scoping lives entirely elsewhere and nothing needs to move.

This blocks because guessing wrong in the enforcement direction silently changes the protection coverage of live plans. Seven other decisions are listed and all have safe defaults; this one does not.

## Three things I found that are broken today

Worth surfacing because two of them would make this feature ship broken:

1. **`MainPage.tsx:20-27` early-returns the empty page**, which is a bare `EmptyState` with no toolbar. On a fresh install with zero plans, the new dropdown would be completely unreachable — invisible to exactly the people evaluating the product.
2. **`CategoryActionsColumn.tsx:81-83` ends in `: true`.** Any scope that is not literally `'groups'` or `'roles'` renders edit and delete **enabled**. Add two scopes without fixing this and you ship unprotected buttons.
3. **Built-ins have no server-side protection at all.** `DELETE /classification/categories/cat-00001-0001-0001/delete` succeeds today. The only protection is the UI hiding the button. One guard in the shared validate path fixes this for groups, roles and both new scopes at once — a smaller diff than guarding each caller, and it closes an existing hole rather than only avoiding a new one.

## Order, and why it matters

The protection plan CRD is a **strict structural schema** — the only `x-kubernetes-preserve-unknown-fields` in the file is on `policies[].params` (line 131), not at spec level. Add a Go field before the yaml and the API server prunes it silently: the write returns **200** and the data is gone.

So: **CRD first, structs second.** Steps 4 → 5, never the other way.

```
 1  shared-types     scopes + 8 built-ins + 8 rule keys
 2  backend          authz guard decoupling        ← the blocker
 3  backend          built-in protection guard
 4  backend          CRD fields                    ← before any struct
 5  shared-types     plan Go structs
 6  backend          4 discovery enumeration sites
 7  frontend-store   two constants (and nothing else)
 8  frontend-ui      shared label parametrisation + default-deny fix
 9  frontend-ui      permission tables
10  frontend-ui      dropdown + the two views
11  frontend-ui      plan form fields
12  frontend-ui      card + filters
13  docs             CRDS.md, architecture.md, INSTALL.md, chart docs
```

Every step has its own verify criterion. Step 7 is deliberately near-empty — its whole job is to *prove* the client and store needed no edit, by dispatching one thunk against a new scope and watching the built-ins arrive.

## Built-ins shipping

**Environment** (`plan-environments`): Production, Staging, Development, Quality Assurance — ids `cat-00001-0002-0001..4`

**Tag** (`plan-tags`): Compliance, Security, Reliability, Cost — ids `cat-00001-0003-0001..4`

The id shape is dictated by the CRD, not chosen: `^cat-[0-9a-f]{5}-[0-9a-f]{4}-[0-9a-f]{4}$`, exactly 19 characters. Marked by fixed id + `Type: built-in` + the frozen `2024-01-01T00:00:00Z` creation date. Re-reconciled on every exporter boot, so edits to a built-in revert on restart. Users add custom values exactly as they do today for groups and roles.

## Before implementation starts

A dependency-upgrade workflow is editing `go.mod`/`go.sum` across `internal/{rest,x-ware,data}` and all four services **right now**. Steps 1 and 5 touch two of those modules. Let it finish first.

Then: answer Q1, release `internal/data` and `internal/rest`, bump the pins, apply the CRD, restart the exporter. All five are yours — I do not touch pins, releases, CRD applies, version bumps, builds or git.

## Blocking decisions

- **Q1 — Are environment and tags on a protection plan metadata only (label, filter, group, report), or do they participate in enforcement scoping (i.e. change which workloads the plan actually protects)?**
  - _recommendation_: Metadata only. `services/discovery/internal/core/plans/policies/renderer.go` `buildScopes` switches solely on `plan.Scope.Type`, and `RenderMeta` carries only `{PlanID, PlanName, CreatedBy, Mode}`. Keeping the new fields out of the renderer means no already-rendered policy changes behaviour. Scoping stays where it is today: `scope.type` + applicationIds/namespaces.
  - _defaultIfNoAnswer_: Metadata only. Plans gain `environmentID` and `tagIDs`; the renderer is not touched.
  - _impactIfWrong_: If enforcement was intended, the renderer, `RenderMeta`, the generated policy match blocks and every already-rendered policy for every live plan need reworking — roughly doubles the backend and silently changes which workloads existing plans protect. Building it on a guess is the unsafe direction, which is why this one blocks.
  - _blocking_: yes

## Serious findings to fold into the steps BEFORE executing

- **The new taxonomy scopes are passed straight through as AUTHZ scopes, and no role in the system grants them — so every category endpoint for plan-environments/plan-tags will 403 for every user, including built-in Admin. Adding two entries to `categoryActions` is necessary but NOT sufficient; the plan's claim that guard.go is the single backend authz blocker is wrong.**
  - _severity_: critical
  - _evidence_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/guard.go:184-188 builds `xauthz.Requirement{Scope: categoryScope, MinLevel: level, Rule: xauthz.RuleKey(categoryScope, actions[level])}` — the authz scope IS the taxonomy scope string. /Users/houssem/Desktop/Github/internal/x-ware/authz/allow.go: `grantedLevel()` returns `grants.Levels[scope]` and falls back ONLY to `roledata.ScopeAll` ("ALL"); a miss returns ok=false and `Allows` returns false. /Users/houssem/Desktop/Github/internal/data/resources/role/def.go:81-86 defines exactly six scopes (applications, groups, users, roles, settings, protection-plans) and builtin.go:14-20 `builtinScopes` lists those six; `uniformPermissions()` grants one entry per built-in scope and never ScopeAll. So Requirement{Scope:"plan-environments"} matches nothing in grants.Levels for any built-in role. groups/roles work today only because "groups"/"roles" happen to be real role scopes.
  - _fix_: Decouple the taxonomy scope from the authz scope inside GuardCategoryScope: change `categoryActions` to map categoryScope -> {authzScope string; actions map[PermissionLevel]string}, with plan-environments/plan-tags -> roledata.ScopeProtectionPlans, and build the Requirement from authzScope for both Scope and RuleKey. Do NOT introduce new role scopes instead — that would require re-seeding built-in roles and migrating every existing custom role. Add a guard test asserting a built-in Contributor can create a plan-environment.
- **Three of the four authz plumbing layers have no step: the shared action-constant catalog, the backend role-rule list, and the two frontend permission catalogs. Without them the new actions cannot be denied per-role, the role editor cannot show them, and the UI has no ACTION_PERMISSIONS entry to gate the new toolbar items.**
  - _severity_: critical
  - _evidence_: Backend action constants live in /Users/houssem/Desktop/Github/internal/data/resources/role/rules.go (ActionAddGroupCategory:18, ActionEditGroupCategory:24, ActionDeleteGroupCategory:25, ActionAddRoleCategory:41, ...); the protection-plans block there (lines 55-61) has no category actions. Frontend role editor catalog /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/roles/constants/scopeRules.ts:100-113 — the `protection-plans` entry lists only the six plan actions, no category rules; `formatRuleKey` builds `${scope}.${rule}.deny`. Frontend gate catalog /Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx:211-241 — `protectionPlans` has viewViolations/create/edit/cancel/duplicate/reactivate/delete but no viewCategories/addCategory/editCategory/deleteCategory, unlike groups (120-135) and roles (173-193).
  - _fix_: Add Action{Add,Edit,Delete}Plan{Environment,Tag} (or a single Plan-taxonomy action set) to internal/data/resources/role/rules.go; reference them from the new categoryActions entries; add matching {key,label} rules to SCOPE_RULES['protection-plans'] at the right levels (Contributor for add, Owner for edit/delete, ReadOnly for view); add the ACTION_PERMISSIONS.protectionPlans.{viewCategories,addCategory,editCategory,deleteCategory} entries with deny keys `protection-plans.<action>.deny`.
- **Clearing the last tag/environment on a plan will silently no-op: the typed PATCH path drops empty slices, so discovery reports success while the CRD keeps the old list. The plan copies the ParticipantsIDs pattern, which carries this latent bug — and removing your last tag is a routine action, unlike emptying participants.**
  - _severity_: high
  - _evidence_: /Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go:39+ — `PatchProtectionPlanRequest.ParticipantsIDs []string \`json:"participantsIDs,omitempty"\``. /Users/houssem/Desktop/Github/internal/rest/mappers/payload.go — MapToJSONPayload skips a field when `omitEmpty && isZeroValue(field)`, and isZeroValue returns true for `reflect.Slice` when `v.Len() == 0`. /Users/houssem/Desktop/Github/internal/rest/clients/plans/protection/client.go:46-55 — `Patch()` serialises via MapToJSONPayload; the sibling `PatchRaw` doc comment explicitly exists because the typed path cannot clear fields. /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/update/patch.go:89-91 sets `patch.ParticipantsIDs = req.ParticipantsIDs` and returns changed=true, so the caller believes the write happened.
  - _fix_: For the new list fields, either route the clear case through `Client.PatchRaw` with an explicit empty array, or declare them as `*[]string` in PatchProtectionPlanRequest so nil (absent) and &[]string{} (clear) are distinguishable. Add a red test first: 'PATCH with tags:[] on a plan that has two tags leaves zero tags on the CRD'.
- **Every user-visible string in the reused Category components is hardcoded to the word 'Category', and none of the components accepts a label/noun prop. The two new views would ship reading 'Manage Categories', 'Add Category', 'Category Name', 'Delete Category', and toast 'Category "production" created'. The Scope column would also render the raw internal string 'plan-environments' in a chip.**
  - _severity_: high
  - _evidence_: /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/config/getManageCategoriesButtonConfig.tsx reads CC.LABELS.TOOLBAR.MANAGE_CATEGORIES ('Manage Categories' / 'View Categories' / 'Add Category' + two permission tooltips) with no override param. components/display/list/CategoryColumns.tsx:24,50,70,87 read CC.LABELS.COLUMNS.{NAME='Category Name',TYPE,SCOPE,CREATED}; the SCOPE column (line ~70) renders `value` verbatim in a RowTag. components/delete/CategoryDeleteModal.tsx:26,30 read DELETE_MODAL_TITLE='Delete Category'/DELETE_MODAL_OK. panels/AddCategoryPanel.tsx + EditCategoryPanel.tsx read CC.LABELS.PANELS.{ADD,EDIT}_CATEGORY ('Add Category','Category Name','e.g. Engineering, Operations','A category with this name already exists'). CategoryActionsColumn.tsx:119,147 use 'You do not have permission to edit/delete categories'. Messages: CATEGORY_CREATED(name)=>`Category "${name}" created`. Labels live in constants/categories.ts:11-80.
  - _fix_: Thread a labels object (or a scope->labels map in CATEGORIES_CONSTANTS keyed by the four scopes) through getManageCategoriesButtonConfig, CategoryColumns, CategoryDeleteModal, Add/EditCategoryPanel and CategoryActionsColumn, defaulting to the current Category wording so groups/roles are untouched. Hide or map the Scope column in the plan views — users must never see 'plan-environments'.
- **The frontend plan payload shape is declared independently in six files plus two change-detection helpers. A step that only edits planFormValues.ts/usePlanFormState.ts leaves the field stripped before it reaches the wire, and leaves 'Update' disabled when the user changes only tags.**
  - _severity_: high
  - _evidence_: grep for participantsIDs under /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection: models/index.ts:98 (ProtectionPlan), components/create/types.ts:21 (FormValues), clients/prepare.ts:16 (PreparePlanPayload), clients/update.ts:16, hooks/usePlanActions.ts:21, store/thunks/protectionPlansThunks.ts:79 — six separate declarations, none derived from the others. Change detection is ALSO duplicated: utils/planFormValues.ts defines `arraysEqualUnordered` and uses it per-field in hasFormChanges, while hooks/usePlanFormState.ts:38 builds a separate signature `participantsIDs: [...(values.participantsIDs ?? [])].sort()`. utils/planFormValues.ts touches the field three times (DEFAULT_FORM_VALUES:17, planToFormValues:32, buildPreparePayload:72).
  - _fix_: Make the step enumerate all eight sites explicitly, and add the new arrays to BOTH hasFormChanges (arraysEqualUnordered) and the usePlanFormState signature; otherwise editing only the tags produces no PATCH. A cheap guard: a test that changes only tags and asserts the prepare payload contains them and the form reports dirty.
- **Step 5 adds environment/tag fields to `DuplicateProtectionPlanRequest` on the stated premise that it is one of "the four request structs that already carry `ParticipantsIDs`". It is not. Duplicate is an overrides-only DTO of three fields, and duplication already inherits every non-overridden field from the source plan. The edit therefore also invents a per-duplicate environment/tag override UI that was never requested.**
  - _severity_: high
  - _evidence_: /Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go:147-151 — `DuplicateProtectionPlanRequest` is `Name *string`, `TimeMode *string`, `TimeRange *TimeRangeRequest`. No `ParticipantsIDs`. Create (line 25), Patch (line 49) and Prepare (line 74) each carry it; Duplicate does not. /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/duplicate/duplicate.go:43-62 — `BuildRequest` copies Description/Severity/Priority/Scope/Policies/Mode/ParticipantsIDs straight off `source`; only Name/TimeMode/TimeRange come from `overrides`, and the file comment states the result is fed back through the standard prepare pipeline.
  - _fix_: CUT the `DuplicateProtectionPlanRequest` edit entirely — zero change to that struct. REPLACE with two lines in `duplicate.go:BuildRequest`: `EnvironmentID: source.EnvironmentID,` and `TagIDs: source.TagIDs,` in the returned `PrepareProtectionPlanRequest`. Duplication then inherits the taxonomy exactly as it already inherits participants. Also CUT any duplicate-panel UI field for environment/tag that this step implied.
- **Step 3 adds a server-side guard rejecting mutation/deletion of built-in categories. Nobody asked for it, it is an admitted behaviour change for the existing groups/roles built-ins (out of scope for a plans feature), it guards a non-boundary, and it forces a new exported symbol out of `internal/data`, which is UNRELEASED and resolved only via go.work replace — enlarging a release and pin-bump the user must own.**
  - _severity_: high
  - _evidence_: /Users/houssem/Desktop/Github/internal/data/classification/category/builtin.go — `WithBuiltins` rebuilds the built-in rows from `BuiltinCategories` on every read and drops any stored row whose ID matches a built-in, and `startup.SeedBuiltins` re-seeds on every exporter boot: a tampered built-in self-heals without a guard. /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryActionsColumn.tsx:83-85 — `showEdit = !isBuiltIn && !!onEdit; showDelete = !isBuiltIn` already hides both controls for built-ins. The proposed `IsBuiltin` export has no other consumer in the plan.
  - _fix_: CUT the whole step: the guard, the `IsBuiltin` export from `internal/data/classification/category`, its new error constant, and its test. REPLACE with nothing — `WithBuiltins` + `SeedBuiltins` already restore built-ins, and the UI already hides the controls. If server-side immutability of built-ins is genuinely wanted, it is a separate ticket covering groups and roles, not a rider on a protection-plans feature.
- **Steps 1 and 9 spec 8 new rule keys (view/add/edit/delete × environment × tag) plus their hand-synced `ACTION_PERMISSIONS` and `scopeRules` mirrors. Nothing in the request asks for granting environment management independently of tag management — both are one dropdown of plan taxonomy under one authz scope. This is configurability for a distinction the product does not make, and it doubles the delta in the unreleased `internal/data`.**
  - _severity_: high
  - _evidence_: /Users/houssem/Desktop/Github/internal/data/resources/role/rules.go:54-62 — the protection-plans block is 7 actions today; +8 more than doubles it. /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/guard.go:155-164 — `categoryActions` maps only Contributor→Add and Owner→Edit per scope; the guard reads exactly one action per level, and two scopes can point at the same rule keys. /Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx:115-130,168-183 — every rule key costs a hand-written 4-entry mirror per entity.
  - _fix_: CUT 8 keys to 4 shared ones: `ActionViewPlanTaxonomies`, `ActionAddPlanTaxonomy`, `ActionEditPlanTaxonomy`, `ActionDeletePlanTaxonomy`, all under `ScopeProtectionPlans`. REPLACE the two per-entity `categoryActions` blocks with two map entries (`plan-environments`, `plan-tags`) that resolve to the same four rule keys and the same authz scope — which is precisely the scope/taxonomy decoupling Step 2 already introduces. Halves the `rules.go` delta, the `permissionEngine.tsx` entries and the `scopeRules.ts` entries. Split environments from tags later only if a customer asks.

## Chosen approach

Reuse the existing Category classification mechanism with two new taxonomy scope values — `plan-environments` and `plan-tags` — instead of building Environment and Tag as separate entities. The Category stack (CRD, storage, routes, handlers, cache, seeding, API client, Redux store, list/panel components, dropdown factory) is already scope-generic end to end. The feature becomes: add two scope constants, add built-in rows, decouple one authz map, parametrise the shared UI labels, add one taxonomy view page, and thread two optional fields onto the plan.

## Why chosen

Because every layer except one already absorbs a new scope with zero edits, and I verified that rather than assuming it.

- `internal/data/classification/category/def.go` — `Scope` is a plain untyped `string`, already a free-form kind discriminator.
- `charts/telark-crds/templates/crds/classification/category.yaml` — `scope` is `type: string, minLength: 1, maxLength: 50` with **no enum**. Its own description reads "(e.g., groups, roles, workloads)" — the schema author already anticipated more scopes.
- `internal/data/classification/category/builtin.go` — `WithBuiltins` merges by **ID**, not scope. Appending new-scope rows needs no logic change.
- `services/exporter/internal/startup/seed.go` `seedCategories()` — completely scope-blind: reads the CR, calls `WithBuiltins(existing)`, upserts. Adding built-ins to `internal/data` is the only change needed to seed them.
- `services/exporter/internal/utils/classification/category/validate.go` — `ValidateAndPrepareCategory` checks name non-empty and scope non-empty. **No whitelist, no enum.**
- `services/exporter/internal/utils/classification/category/cache.go` — `InvalidateCategoryCaches` derives the scope set at runtime from stored rows.
- `internal/rest/endpoints/classification/category/def.go` — the read route is literally `classification/categories/scope/{scope}/get`. No new routes.
- `services/exporter/internal/authz/requirements.go:94-101` — all six category routes are `authz.Authenticated`; the in-file comment says writes "defer to GuardCategoryScope". No requirements change.
- `dashboard-ui/src/constants/rest/endpoints.ts:216-240` — `CATEGORIES.GET_BY_SCOPE(scope)` etc., all scope-parameterised.
- `categories/store/slices/categorySlice.ts` + `thunks/fetchThunks.ts` — state is `categoriesByScope: Record<string, Category[]>`, the thunk takes a `scope: string`. Genuinely generic.
- `categories/config/getManageCategoriesButtonConfig.tsx` — already a documented generic dropdown factory; its doc comment says "Use in roles, groups, or any feature that lists categories by scope."
- `categories/panels/AddCategoryPanel.tsx` — its `scope` prop type is derived from `CATEGORIES_CONSTANTS.SCOPES`, so widening the constant widens the prop automatically.

There is exactly **one** real backend blocker, and it is a single file. `services/exporter/internal/authz/guard.go:140-180` passes the taxonomy scope straight through as the authz scope (`xauthz.Requirement{Scope: categoryScope, ...}`) and hard-denies any scope missing from its `categoryActions` map. A `plan-environments` scope would resolve `grants.Levels["plan-environments"]`, which does not exist, and deny every write. The fix is a mirror of `globalConfigFields`, declared immediately below it in the same file, which already stores full `xauthz.Requirement` values in a map.

The competing clone-everything design would produce roughly 84 files — new CRDs, handlers, routes, clients, slices, panels, columns — to re-derive behaviour that eleven verified call sites already provide. That is the design I rejected.

## Grafted from the other designs

- From the clone design (A) — naming discipline. Use `tagIDs`, not `tags`, on the plan. `tags` reads as a policy-template param (the codebase already ships a `block-image-tags` template whose params are free-form under the one `x-kubernetes-preserve-unknown-fields` in the plan CRD), and `Group.categoryID` is the established reference-by-id suffix. Likewise `environmentID`, not `environment`.
- From the clone design (A) — CRD-first ordering. The plan CRD is a strict structural schema: the only `x-kubernetes-preserve-unknown-fields: true` in `charts/telark-crds/templates/crds/plans/protectionplan.yaml` is at line 131 on `policies[].params`, not at spec level. A Go field added before the yaml is silently pruned — write returns 200, data vanishes. So Step 4 edits the yaml before Step 5 touches any struct. A's insistence on this ordering is the single most valuable thing it contributed.
- From design B — `categoryActions` promoted from `map[scope]map[level]string` to `map[scope]map[level]xauthz.Requirement`, decoupling the taxonomy scope from the authz scope so plan taxonomies can authorise against `roledata.ScopeProtectionPlans`. B also spotted that this exact map shape already exists six lines down as `globalConfigFields`, which turns an invention into a mirror.
- From design B — one shared built-in protection guard. I verified built-ins have zero server-side protection today: `ValidateCategoryDeletion` only checks `len(categories) <= constants.MinCategoriesInCRD`, and the patch path has no `Type` check at all. Protection is UI-only (`CategoryActionsColumn.tsx:85-86`). Rather than guarding the two new scopes, put one guard in the shared validate path — it covers groups, roles, environments and tags at once and is a smaller diff than per-caller guards. All four `GuardCategoryScope` call sites in `handlers/classification/category/handler.go` (lines 48, 224, 242, 267) already funnel through the same layer.
- From design C — one new page-config hook plus a `viewMode` state for the second view, instead of new routes. I confirmed this is exactly how roles do it: `roles/pages/MainPage.tsx` holds `type ViewMode = 'roles' | 'categories'` and `useRoleListPageConfig.tsx` swaps columns, dataSource, pagination and breadcrumbs inside one `PageLayout config`. No router entry, no new route guard.
- From design C — default-deny fix for the UI permission fall-through. `CategoryActionsColumn.tsx:81-83` ends in `: true`, so any scope that is not literally 'groups' or 'roles' renders edit and delete **enabled**. Adding two scopes without fixing this ships unprotected buttons on both new views. C caught it; it is now Step 8.
- Correction to design B's claim, worth recording: the backend slice comparison helper in `services/discovery/internal/core/plans/protection/update/patch.go` is `stringSliceSetEqual`, not `arraysEqualUnordered`. `arraysEqualUnordered` is the separate *frontend* helper in `dashboard-ui/src/features/plans/protection/utils/planFormValues.ts`. Both exist; they are different files in different languages and both must be used in their own layer.

## Decisions for the user

- **Q1 — Are environment and tags on a protection plan metadata only (label, filter, group, report), or do they participate in enforcement scoping (i.e. change which workloads the plan actually protects)?**
  - _recommendation_: Metadata only. `services/discovery/internal/core/plans/policies/renderer.go` `buildScopes` switches solely on `plan.Scope.Type`, and `RenderMeta` carries only `{PlanID, PlanName, CreatedBy, Mode}`. Keeping the new fields out of the renderer means no already-rendered policy changes behaviour. Scoping stays where it is today: `scope.type` + applicationIds/namespaces.
  - _defaultIfNoAnswer_: Metadata only. Plans gain `environmentID` and `tagIDs`; the renderer is not touched.
  - _impactIfWrong_: If enforcement was intended, the renderer, `RenderMeta`, the generated policy match blocks and every already-rendered policy for every live plan need reworking — roughly doubles the backend and silently changes which workloads existing plans protect. Building it on a guess is the unsafe direction, which is why this one blocks.
  - _blocking_: yes
- **Q2 — Should the new dropdown carry two items (Environments, Tags), each opening its own view, or four (View/Add per entity) as the existing groups and roles dropdown does?**
  - _recommendation_: Two items, two views, with the Add action living inside each view's own toolbar. That is literally what was asked for ("hold those options and on click to render 2 views"), and it keeps the plans toolbar from growing a four-item menu next to six phase pills.
  - _defaultIfNoAnswer_: Two items → two views; Add lives inside each view.
  - _impactIfWrong_: Cosmetic and cheap to reverse — one config file. The Add panel is reused either way.
  - _blocking_: no
- **Q3 — Does the protection plan itself gain `environmentID` and `tagIDs` fields, or is this purely a taxonomy-management feature with no plan fields?**
  - _recommendation_: Plans gain both. Without them the feature is two lists that nothing consumes. `Group.categoryID` is the exact precedent — a managed taxonomy that the parent record references.
  - _defaultIfNoAnswer_: Plans gain `environmentID` (optional, single) and `tagIDs` (optional, array).
  - _impactIfWrong_: Adding them touches the plan CRD, which only you can apply to the cluster. Getting it wrong costs a CRD round trip, not data — the fields are optional and absent from `required:`, so existing plans stay valid either way.
  - _blocking_: no
- **Q4 — One environment per plan, or many?**
  - _recommendation_: One. An environment is a mutually-exclusive classification, which is why `Group.categoryID` is a scalar; tags carry the many-to-many role. Hence `environmentID string` and `tagIDs []string`.
  - _defaultIfNoAnswer_: Single environment, multiple tags.
  - _impactIfWrong_: Widening scalar → array later is a CRD change plus a migration of stored plans. Narrowing array → scalar is worse. Cheap to decide now, awkward later.
  - _blocking_: no
- **Q5 — Should the new taxonomies get dedicated permission rule keys, or reuse the generic protection-plan edit rules?**
  - _recommendation_: Dedicated keys mirroring the existing `addgroupcategory` / `addrolecategory` family: eight new constants in `internal/data/resources/role/rules.go`, all under the existing `protection-plans` scope. This keeps "can edit a plan" separable from "can redefine the environment taxonomy", which is the whole reason groups and roles have separate category rules.
  - _defaultIfNoAnswer_: Eight dedicated action keys under the `protection-plans` scope.
  - _impactIfWrong_: These constants live in the unreleased `internal/data` module, so changing your mind later means a second release + pin bump. Reusing the generic plan rules would mean any plan Contributor can rewrite the shared taxonomy for the whole tenant.
  - _blocking_: no
- **Q6 — Server-side built-in protection currently does not exist. Adding it changes existing behaviour for groups and roles built-ins too. Accept that?**
  - _recommendation_: Accept it. Today the API will happily PATCH or DELETE `cat-00001-0001-0001` (built-in "Engineering") — only the UI hides the buttons. Guarding the shared validate path is the root-cause fix; guarding only the two new scopes would leave groups and roles still open.
  - _defaultIfNoAnswer_: Add the guard for all scopes. Deletes and patches of any built-in ID are rejected, including for groups and roles.
  - _impactIfWrong_: If some caller legitimately relies on mutating a built-in, this breaks it. I found no such caller — `seedCategories()` writes through `api.Upsert` directly, not the HTTP route, so seeding is unaffected.
  - _blocking_: no
- **Q7 — Should the environments and tags views be reachable for a tenant with zero protection plans?**
  - _recommendation_: Yes. `plans/protection/pages/MainPage.tsx:20-27` returns `ProtectionPlansEmptyPage` before the toolbar renders, and `ProtectionPlansEmptyPage` is a bare `EmptyState` with no toolbar at all — so on a fresh install the new dropdown is unreachable and the built-in seed data is invisible. Move the taxonomy branch above the early return and surface the dropdown on the empty page.
  - _defaultIfNoAnswer_: Yes — fix the early return and add the dropdown to the empty page.
  - _impactIfWrong_: Leaving it means the feature is invisible on exactly the installs most likely to be evaluating it.
  - _blocking_: no
- **Q8 — Scope strings `plan-environments` / `plan-tags`, or bare `environments` / `tags`?**
  - _recommendation_: Prefixed. The scope string is rendered verbatim in the Category list's Scope column, so it should read as plan-owned; and a bare `tags` scope sits confusingly beside the `block-image-tags` policy template. Prefixed also leaves room for other features to own taxonomies later without collision.
  - _defaultIfNoAnswer_: `plan-environments` and `plan-tags`.
  - _impactIfWrong_: Renaming after any data is seeded means orphaned rows in the singleton Category CR with no UI surface to delete them.
  - _blocking_: no

## Steps

- **1. Add the two taxonomy scopes, eight built-in rows and eight permission rule keys**
  - _layer_: shared-types
  - _files_:
    - /Users/houssem/Desktop/Github/internal/data/classification/category/def.go
    - /Users/houssem/Desktop/Github/internal/data/constants/builtin.go
    - /Users/houssem/Desktop/Github/internal/data/classification/category/builtin.go
    - /Users/houssem/Desktop/Github/internal/data/resources/role/rules.go
  - _detail_: In `category/def.go` add `ScopePlanEnvironments = "plan-environments"` and `ScopePlanTags = "plan-tags"` next to the existing `CategoryType` constants. Note deliberately: these are Category scope values, NOT authz scopes — groups and roles happen to coincide (builtin.go currently uses `role.ScopeGroups`), but plan taxonomies do not, so they belong here rather than in `role/def.go`. In `constants/builtin.go` add eight ID constants continuing the existing `cat-00001-0001-000N` block: environments in `cat-00001-0002-000N`, tags in `cat-00001-0003-000N`. The CRD enforces `pattern: '^cat-[0-9a-f]{5}-[0-9a-f]{4}-[0-9a-f]{4}$'` with `minLength`/`maxLength` 19, so the format is not negotiable. In `builtin.go` append eight rows to `BuiltinCategories`, each `{ID: <const>, Name: <name>, Scope: <new scope>, Type: CategoryTypeBuiltIn, CreationDate: constants.BuiltinCreationDate}` — same shape as the five existing rows. Also export `isBuiltin` as `IsBuiltin` (Step 3 needs it; exposing a helper is the sanctioned alternative to an internal test). In `role/rules.go` add eight action constants alongside the existing `ActionAddGroupCategory` (line 18) / `ActionAddRoleCategory` (line 41) family.
  - _verify_: `go build ./...` in `/Users/houssem/Desktop/Github/internal/data`. Then the one runnable check: a table test asserting every `BuiltinCategories` ID matches the CRD regex, that all thirteen IDs are unique, and that `WithBuiltins(nil)` returns thirteen rows across four distinct scopes. Place it at `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/classification/builtin_test.go` so the released module stays free of test churn.
  - _dependsOn_:

- **2. Decouple the authz guard's taxonomy scope from its authz scope**
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/guard.go
  - _detail_: This is the only backend blocker and the only reason a new scope does not already work. Change `var categoryActions map[string]map[roledata.PermissionLevel]string` (around line 143) to `map[string]map[roledata.PermissionLevel]xauthz.Requirement`, each entry carrying its own `{Scope, MinLevel, Rule}`. The groups and roles entries keep `Scope: roledata.ScopeGroups` / `ScopeRoles` and are behaviourally identical. The two new entries use `Scope: roledata.ScopeProtectionPlans` (already defined at `internal/data/resources/role/def.go:85`) with the Step 1 rule keys. `GuardCategoryScope` (line 167) then looks the requirement up instead of constructing it, and keeps its existing unknown-scope hard deny. The target shape already exists six lines below as `globalConfigFields`, so mirror that declaration style exactly.
  - _verify_: Extend `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/guard_extra_test.go`: a protection-plans Contributor may create a `plan-environments` category; a groups Owner may not; a groups Owner is still allowed on the `groups` scope (no regression); an unknown scope still returns 403. `go test ./internal/tests/authz/...` and `golangci-lint run` both clean from `services/exporter`.
  - _dependsOn_:
    - 1
- **3. Reject patch and delete of any built-in category, server-side**
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/classification/category/validate.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/handlers/classification/category/handler.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants
  - _detail_: Built-ins have no server-side protection today — verified: `ValidateCategoryDeletion` only checks `len(categories) <= constants.MinCategoriesInCRD`, and the patch path has no `Type` check anywhere. The only protection is `CategoryActionsColumn.tsx:85-86` hiding the buttons. Add one `ValidateCategoryNotBuiltin` guard in `validate.go` using the `IsBuiltin` exported in Step 1, and call it from the patch path (handler.go around line 224, before the existing `GuardCategoryScope(oldScope, Owner)`) and the delete path (around line 267). Add the error string to the exporter constants package per the no-inline-literals rule. One guard in the shared layer covers groups, roles, environments and tags — a guard per caller would be a larger diff and would still miss siblings.
  - _verify_: New test under `/Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/classification/`: PATCH and DELETE of `cat-00001-0001-0001` are both rejected, PATCH and DELETE of a custom `cat-*` id still succeed, and the guard fires before the scope guard so the error is about the built-in, not permissions. `golangci-lint run` clean.
  - _dependsOn_:
    - 1
- **4. Add environmentID and tagIDs to the protection plan CRD — before any Go struct**
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/charts/telark-crds/templates/crds/plans/protectionplan.yaml
  - _detail_: Ordering is load-bearing. This CRD is a strict structural schema: the only `x-kubernetes-preserve-unknown-fields: true` is line 131, scoped to `policies[].params`, not spec level. A Go field added before this yaml is silently pruned by the API server — the write returns 200 and the data is simply gone. Add to the spec `properties:` block that begins at line 64, following the `description` field at line 74 which is the established optional precedent (present in `properties`, absent from the `required:` list that runs lines 49-63). `environmentID: {type: string, pattern: '^cat-[0-9a-f]{5}-[0-9a-f]{4}-[0-9a-f]{4}$', minLength: 19, maxLength: 19}` and `tagIDs: {type: array, items: {type: string, <same pattern>}}` with a sane `maxItems`. Neither goes in `required:`. The category CRD itself needs no change at all — its `scope` field is `type: string, maxLength: 50` with no enum.
  - _verify_: `rtk proxy "helm template charts/telark-crds ..."` and confirm both fields appear under the protectionplan spec properties, that `required:` is unchanged, and that the rendered output still contains exactly one `x-kubernetes-preserve-unknown-fields`. Then `kubectl apply --dry-run=server` against the rendered CRD — schema accepted. Actually applying it to the cluster is yours, not mine.
  - _dependsOn_:

- **5. Add the plan fields to the shared Go structs**
  - _layer_: shared-types
  - _files_:
    - /Users/houssem/Desktop/Github/internal/data/plans/protectionplan.go
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go
  - _detail_: In `protectionplan.go` add `EnvironmentID string `json:"environmentID,omitempty"`` and `TagIDs []string `json:"tagIDs,omitempty"`` to the `ProtectionPlan` struct, next to `ParticipantsIDs` (line 77). In `types.go` add the same pair to the four request structs that already carry `ParticipantsIDs`: `CreateProtectionPlanRequest` (line 3), `PatchProtectionPlanRequest` (line 39, using the `omitempty` convention its siblings already use), `PrepareProtectionPlanRequest` (line 64) and `DuplicateProtectionPlanRequest` (line 147). Match the existing `participantsIDs` json casing exactly — capital ID suffix, lowercase first letter.
  - _verify_: `go build ./...` in both `internal/data` and `internal/rest`. Then the anti-pruning check that matters: marshal a plan with both fields set, apply it against the Step 4 CRD with `--dry-run=server`, read it back, assert both fields survived. If they come back empty, Step 4 was not applied.
  - _dependsOn_:
    - 4
- **6. Thread the two fields through all four discovery enumeration sites**
  - _layer_: backend
  - _files_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/svc.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/update/patch.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/duplicate/duplicate.go
  - _detail_: Four sites enumerate plan fields by hand; miss one and the field silently drops on that path. `svc.go` `buildPlan()` — add beside `Severity` (line 396), `Priority` (397), `ParticipantsIDs` (403). `svc.go` `toCreateRequest()` — same, beside lines 579, 580, 601. `update/patch.go` — scalar diff for `EnvironmentID` in the style of lines 46-47/50-52, and slice diff for `TagIDs` using the existing `stringSliceSetEqual` helper as at lines 89-90 (note: `stringSliceSetEqual` is the backend helper; `arraysEqualUnordered` is the unrelated frontend one). `duplicate/duplicate.go` `BuildRequest()` — beside lines 54, 55, 61. No reference-existence validation: the CRD pattern already rejects malformed ids, and checking that a referenced category still exists would need a cross-CR read on every write. Deliberate simplification, listed in scope notes.
  - _verify_: New test under `/Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/plans/`: create a plan with both fields → read back both; patch changing only `environmentID` → the produced patch contains only that field; patch reordering `tagIDs` → no patch produced; duplicate → both fields carried. `golangci-lint run` clean from `services/discovery`.
  - _dependsOn_:
    - 5
- **7. Add the two scopes to the UI constant and confirm the client/store need no other edit**
  - _layer_: frontend-store
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/constants/categories.ts
  - _detail_: Add `PLAN_ENVIRONMENTS: 'plan-environments'` and `PLAN_TAGS: 'plan-tags'` to the `SCOPES` object at the top of the file. That is the entire API-client and store change. Verified, not assumed: `constants/rest/endpoints.ts:216-240` builds every category endpoint from a scope or id argument; `categories/clients/fetch.ts` takes `scope: string`; `store/thunks/fetchThunks.ts` `fetchCategoriesByScopeThunk` takes `scope: string`; `store/slices/categorySlice.ts` keys state as `categoriesByScope: Record<string, Category[]>`; `store/selectors/categorySelectors.ts` selects by scope argument. Widening `SCOPES` also automatically widens the `CategoryScope` union that types `AddCategoryPanel`'s `scope` prop. Do not add new endpoints, clients, thunks, slices or selectors.
  - _verify_: `npm run check-all` — zero errors. Then in the running app, dispatch `fetchCategoriesByScopeThunk('plan-environments')` and confirm `state.categories.categoriesByScope['plan-environments']` fills with the four seeded built-ins. That single check proves the whole client/store path needed no edit.
  - _dependsOn_:
    - 1
    - 2
- **8. Parametrise the shared category UI labels and close the default-allow permission hole**
  - _layer_: frontend-ui
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/constants/categories.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/panels/AddCategoryPanel.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/panels/EditCategoryPanel.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryColumns.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryActionsColumn.tsx
  - _detail_: Two jobs in the shared components, which is where the real frontend weight of this feature sits. (a) Labels: the panels and columns hardcode the word Category — `PANELS.ADD_CATEGORY.TITLE` is 'Add Category', `COLUMNS.NAME` is 'Category Name', and the success/failure messages all say category. Add a scope→labels map inside `categories.ts` (all user-facing strings stay in the constants file) and select from it by the existing `scope` prop. No new panel components. (b) Security: `CategoryActionsColumn.tsx:81-83` reads `scope === 'groups' ? canEditGroups : scope === 'roles' ? canEditRoles : true` — the trailing `: true` means any unrecognised scope renders edit and delete **enabled**. Widen the `scope?: 'groups' | 'roles'` prop type (line 18) to the `CategoryScope` union and replace the ternary chain with a scope→permission lookup that defaults to **deny**. Built-in hiding at lines 85-86 already works for the new scopes unchanged, because it keys off `record.type`.
  - _verify_: `npm run check-all` — zero errors, no `eslint-disable`. Manually: the Add panel opened from the Environments view is titled for environments, not categories; and a user holding only ReadOnly on `protection-plans` sees edit and delete **disabled** on that view. Regression check: groups and roles category views are visually and behaviourally unchanged.
  - _dependsOn_:
    - 7
- **9. Register the eight new permissions in the UI permission tables**
  - _layer_: frontend-ui
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/roles/constants/scopeRules.ts
  - _detail_: `permissionEngine.tsx` already has a `protectionPlans` block using `scope: 'protection-plans'`. Add `viewEnvironments`/`addEnvironment`/`editEnvironment`/`deleteEnvironment` and the four tag equivalents to it, mirroring the `groups.addCategory`/`editCategory`/`deleteCategory` entries at lines 120-135 — ReadOnly to view, Contributor to add, Owner to edit and delete, with deny keys matching the Step 1 rule constants. Then add the same eight rules to `roles/constants/scopeRules.ts` under the protection-plans scope, otherwise an admin can never grant or deny them in the role editor and the backend rules are unreachable. Note `usePermission` already default-denies when a scope is absent from `scopeIndex`, so no engine change is needed.
  - _verify_: `npm run check-all` — zero errors. In the role editor, the protection-plans scope lists the eight new rules at the right levels. Denying `protection-plans.addplanenvironment.deny` on a role disables the Add button in the Environments view for a user holding that role, and the backend independently rejects the call (Step 2).
  - _dependsOn_:
    - 2
    - 8
- **10. Add the toolbar dropdown and the two taxonomy views**
  - _layer_: frontend-ui
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/MainPage.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/ProtectionPlansEmptyPage.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlansToolbar.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/PlanTaxonomyPage.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanTaxonomyPageConfig.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/config/getManageCategoriesButtonConfig.tsx
  - _detail_: The dropdown: `getManageCategoriesButtonConfig` already returns a `ToolbarButtonConfig` with `dropdown: {items, onItemClick}` and its doc comment already claims generality for any scope-based category list — but it emits View+Add for one scope. Add a sibling factory in the same file for the two-scope, two-view shape (Q2), reusing the identical disabled-with-tooltip item structure. Slot it into the `buttons` array of the existing `toolbars` useMemo in `ProtectionPlansToolbar.tsx`, gated on the Step 9 view permissions. The views: `MainPage.tsx` gains `type PlanViewMode = 'plans' | 'environments' | 'tags'`; when not 'plans' it renders the new `PlanTaxonomyPage`, which builds a `PageLayoutConfig` via the new hook and renders `<PageLayout>` — exactly the shape of `roles/hooks/list/useRoleListPageConfig.tsx`, reusing `useCategories(scope)`, `useCategoryListView`, `CategoryColumns`, `CategoryActionsColumn`, `AddCategoryPanel` and `EditCategoryPanel` with no new table, no new columns and no new panels. Breadcrumb back to Protection Plans, as roles does. Critically, place the taxonomy branch **above** the empty-state early return at `MainPage.tsx:20-27`, and pass the dropdown into `ProtectionPlansEmptyPage` — that page is a bare `EmptyState` with no toolbar, so without this the whole feature is invisible on a fresh install (Q7). No new route entry.
  - _verify_: `npm run check-all` — zero errors. Both views render the four built-ins each. The breadcrumb returns to the plan grid with filters intact. Then the case that currently breaks: with **zero** plans, the dropdown is still reachable from the empty page and both views open. Re-measure `TOOLBAR_COMPACT_WIDTH.DEFAULT` (currently 760, and its own comment enumerates the exact controls that produced that number) with the extra button present — measure the container, do not guess a new breakpoint.
  - _dependsOn_:
    - 8
    - 9
- **11. Add the environment and tags controls to the plan create/edit form**
  - _layer_: frontend-ui
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/models/index.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/create/types.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/create/BasicInfoSection.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/planFormValues.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanFormState.ts
  - _detail_: Add `environmentID?: string` and `tagIDs?: string[]` to the `ProtectionPlan` interface (models/index.ts line 76) and to `FormValues` (create/types.ts). Add a single Select for environment and a multiple Select for tags to `BasicInfoSection.tsx`, fed by `useCategories(SCOPES.PLAN_ENVIRONMENTS)` and `useCategories(SCOPES.PLAN_TAGS)` — the same hook groups already use for its category select. Then all four sites in `planFormValues.ts`: `DEFAULT_FORM_VALUES`, `planToFormValues`, `buildPreparePayload`, and the dirty-check that already uses the local `arraysEqualUnordered` for `applicationIds`/`namespaces` — `tagIDs` needs the same unordered comparison there. Finally `normalizeFormSnapshot` in `usePlanFormState.ts`, or the edit form reports itself permanently dirty. Per project convention: leave `Select.colorText` unset so the dropdown portal is not mis-themed, size only through the `controls.ts` tokens, no hardcoded hex, no `any`.
  - _verify_: `npm run check-all` — zero errors. Create a plan with an environment and two tags, reload, both persist (this is also the end-to-end proof that Steps 4→6 landed — if the CRD step was skipped, they come back empty). Open edit without changing anything: Save stays disabled. Change only the environment: form goes dirty and the PATCH body contains only `environmentID`. Reorder the tags: form stays clean.
  - _dependsOn_:
    - 6
    - 7
    - 10
- **12. Surface environment and tags where plans are read and filtered**
  - _layer_: frontend-ui
  - _files_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/ProtectionPlansListPage.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/applyPlanFilters.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts
  - _detail_: Show the environment and tags on the plan card, resolving ids to names through `useCategories`. In `ProtectionPlansListPage.tsx`, add two `multiSelect` filter fields to the `filterFields` useMemo alongside the existing scope/createdBy/templates/targets entries, built with the file's own `uniqueValues` helper — but map option labels through the taxonomy so users filter by name, not by `cat-00001-0002-0001`. Extend `applyPlanFilters.ts` to honour them, and add the two keys plus their labels to `FILTER_KEYS` and `LABELS.FILTER` in the constants file. Without this step the taxonomy is write-only and the feature has no payoff on the main page.
  - _verify_: `npm run check-all` — zero errors. The card shows the environment and tags by name. Filtering by an environment narrows the grid, the filter chip renders with the name, and clearing restores the full set. Plans created before this change (no environment, no tags) still render and still pass the filters when nothing is selected.
  - _dependsOn_:
    - 11
- **13. Update the docs in the same change**
  - _layer_: docs
  - _files_:
    - /Users/houssem/Desktop/Github/telark/docs/CRDS.md
    - /Users/houssem/Desktop/Github/telark/docs/architecture.md
    - /Users/houssem/Desktop/Github/telark/docs/INSTALL.md
    - /Users/houssem/Desktop/Github/telark/charts/telark/README.md
    - /Users/houssem/Desktop/Github/telark/charts/telark/VALUES.md
  - _detail_: `docs/CRDS.md` line 27 currently describes `CategoryAsClassification` as "A classification category applied to applications", which is already wrong (it classifies groups and roles) and gets worse with two more scopes — correct it and document all four scope values, the built-in id blocks, and the two new optional plan fields. `docs/architecture.md` gets the taxonomy reuse decision recorded so the next reader does not build a third CRD. `docs/INSTALL.md` and the chart README/VALUES only where they actually describe the permission model or CRD inventory — this feature adds no chart values key, so if VALUES.md has no real delta, say so rather than inventing one. Keep all of it generic: no gmail addresses, no internal vendor names, derive nothing from `.Chart.Name`.
  - _verify_: `helm-docs` regenerates `charts/telark/VALUES.md` with an empty diff (proving no values key drifted). Grep `docs/CRDS.md` for `plan-environments`, `plan-tags`, `environmentID`, `tagIDs` and all eight built-in names — all present. Grep the whole diff for `gmail` and for a hardcoded product name outside the known pre-existing Go constants — no hits.
  - _dependsOn_:
    - 12

## Built-in data

**Concrete built-ins shipping with this change — 8 rows appended to `BuiltinCategories` in `/Users/houssem/Desktop/Github/internal/data/classification/category/builtin.go`.**

The id format is not a style choice: the Category CRD enforces `pattern: '^cat-[0-9a-f]{5}-[0-9a-f]{4}-[0-9a-f]{4}$'` with `minLength: 19` and `maxLength: 19`. The existing five built-ins occupy the `cat-00001-0001-000N` block, so the new ones continue into fresh blocks.

Environment — scope `plan-environments`:
| ID constant | ID | Name |
|---|---|---|
| `CategoryIDEnvProduction` | `cat-00001-0002-0001` | Production |
| `CategoryIDEnvStaging` | `cat-00001-0002-0002` | Staging |
| `CategoryIDEnvDevelopment` | `cat-00001-0002-0003` | Development |
| `CategoryIDEnvQualityAssurance` | `cat-00001-0002-0004` | Quality Assurance |

Tag — scope `plan-tags`:
| ID constant | ID | Name |
|---|---|---|
| `CategoryIDTagCompliance` | `cat-00001-0003-0001` | Compliance |
| `CategoryIDTagSecurity` | `cat-00001-0003-0002` | Security |
| `CategoryIDTagReliability` | `cat-00001-0003-0003` | Reliability |
| `CategoryIDTagCost` | `cat-00001-0003-0004` | Cost |

"Quality Assurance" is spelled out rather than "QA" to match the existing built-in of that name. "Security" duplicates an existing groups built-in *name*, which is fine — ids are unique, and the UI's duplicate-name validator in `AddCategoryPanel.tsx` scopes its check to `useCategories(scope)`, so uniqueness is per-scope by construction.

**How built-ins are marked**

Three things together, identical to the existing five:
1. A fixed id constant in `/Users/houssem/Desktop/Github/internal/data/constants/builtin.go` — never generated, so it is stable across installs.
2. `Type: CategoryTypeBuiltIn` (`"built-in"`), which the CRD constrains via `enum: [built-in, custom]`.
3. `CreationDate: constants.BuiltinCreationDate` — the frozen `"2024-01-01T00:00:00Z"`, so built-ins always sort to the bottom of a creation-date-descending list.

**How they are seeded**

`seedCategories()` in `/Users/houssem/Desktop/Github/telark/services/exporter/internal/startup/seed.go` (called from `main.go`) runs on **every exporter boot**: it reads the singleton `CategoryAsClassification` CR, calls `WithBuiltins(existing)`, and upserts. `WithBuiltins` clones the canonical built-in slice and re-appends only those stored rows whose id is *not* a built-in id. Net effect: built-ins are reconciled on every restart, and any edit a user managed to make to one is reverted. This function is completely scope-blind — adding the eight rows above is the entire seeding change. No exporter edit.

**How they are protected**

Today, honestly: they are not, server-side. I verified `ValidateCategoryDeletion` checks only `len(categories) <= constants.MinCategoriesInCRD`, and the patch path has no `Type` check at all. The only protection is cosmetic — `CategoryActionsColumn.tsx:85-86` hides the edit and delete buttons when `record.type === 'built-in'`. A direct `DELETE /classification/categories/cat-00001-0001-0001/delete` succeeds today (until the next reboot re-seeds it).

Step 3 closes that: one `ValidateCategoryNotBuiltin` guard in the shared `validate.go`, called from the patch and delete handler paths, rejecting any id in `BuiltinCategories`. One guard covering groups, roles, environments and tags — a smaller diff than four guards, and it fixes the two existing scopes that are open right now rather than only protecting the new ones. It does not affect seeding, because `seedCategories()` writes through `api.Upsert` directly and never touches the HTTP route.

Custom values need nothing special: they get a generated id from `GenerateUniqueCategoryID()`, `Type: custom`, and a real creation date, exactly as group and role categories do today.

## Docs to update (same change)

- /Users/houssem/Desktop/Github/telark/docs/CRDS.md — line 27 describes `CategoryAsClassification` as "A classification category applied to applications", which is already inaccurate (it classifies groups and roles) and becomes more so. Correct it, document all four scope values, the built-in id blocks, and the two new optional plan fields on the protection plan CRD.
- /Users/houssem/Desktop/Github/telark/docs/architecture.md — record that plan environments and tags are Category scopes rather than new entities, so the next person does not build a third CRD.
- /Users/houssem/Desktop/Github/telark/docs/INSTALL.md — only where it documents the permission model; the eight new rule keys are grantable in the role editor and belong in any permissions table it carries. Required by the project rule that user-facing changes update INSTALL.md in the same change.
- /Users/houssem/Desktop/Github/telark/charts/telark/README.md — only if it enumerates CRDs or permission scopes. This feature adds no values key.
- /Users/houssem/Desktop/Github/telark/charts/telark/VALUES.md — regenerate with helm-docs to prove no values drift. Expect an empty diff; if it is empty, say so rather than manufacturing an edit.

## User-owned follow-ups

- Release `internal/data` — Step 1 adds scope constants, eight built-in ids, eight built-in rows and eight action rule constants, plus exports `IsBuiltin`. Nothing downstream compiles against a released version until you cut it.
- Release `internal/rest` — Step 5 adds `EnvironmentID`/`TagIDs` to four request structs in `endpoints/plans/types.go`.
- Bump the `internal/data` and `internal/rest` pins in `services/exporter`, `services/discovery` and any other consumer once both are released. I do not touch `go.work`, pins, or `go.mod` — and a dependency-upgrade workflow is currently editing those files, so this must not be done concurrently.
- Apply the updated protection plan CRD to the cluster. Step 4 changes `charts/telark-crds/templates/crds/plans/protectionplan.yaml`; until it is applied, `environmentID` and `tagIDs` are silently pruned on write and Step 11's verification will fail with empty fields and a 200 response.
- Restart the exporter after the CRD is applied so `seedBuiltins()` reconciles the eight new built-in categories into the singleton Category CR. Nothing appears in the new views until this happens.
- Version bumps for the affected services and `Chart.yaml` — owned by the GitHub Action, not by this change.
- Building, pushing or publishing any images.
- All git operations: branching, committing, pushing, merging.
- Deciding Q1 (metadata versus enforcement scoping) before Step 4 is implemented — it is the one blocking decision and it determines whether the plan above is complete or roughly doubles.

## Out of scope

- Enforcement semantics. `policies/renderer.go` `buildScopes` and `RenderMeta` are not touched — environment and tags do not change which workloads a plan protects. This is Q1; if the answer is enforcement, this plan is incomplete and roughly doubles.
- Referential-integrity validation of `environmentID`/`tagIDs` on write. The CRD pattern already rejects malformed ids; confirming a referenced category still exists would mean a cross-CR read on every plan write. Deliberate simplification — the ceiling is that deleting a taxonomy entry leaves dangling references on plans, which render as unresolved ids. Add a validation pass if that becomes visible in practice.
- Cascade or cleanup when a taxonomy entry is deleted while plans reference it. No backfill, no reassignment, no blocking delete.
- Migrating or backfilling existing protection plans. Both fields are optional and absent from the CRD `required:` list, so existing plans stay valid and simply render without an environment or tags.
- A third CRD, new API group, new routes, new handlers, new Redux slice or new API client for Environment or Tag. That is the design that was rejected; reusing the Category stack is the entire point.
- Environment- or tag-based grouping in reports, insights, notifications or the violations view.
- Hierarchical or coloured tags, tag limits per plan beyond the CRD `maxItems`, or any tag autocomplete beyond the existing Select.
- Applying environment/tags to any resource other than protection plans (applications, groups, roles).
- `internal/composer` — legacy, unused, not part of this or any change.
- Finishing or reverting the ~40 files of in-flight lint work under `services/exporter/internal/tests/`.
- Version bumps, `Chart.yaml` edits, image builds, image pushes, `go.work` edits, internal module pins, module releases, and all git operations — every one of these is yours.

## Risks

- Structural schema pruning — the highest-severity trap in this feature. `charts/telark-crds/templates/crds/plans/protectionplan.yaml` carries exactly one `x-kubernetes-preserve-unknown-fields: true`, at line 131 on `policies[].params`, not at spec level. Adding Go fields before the yaml means the API server drops them silently: the write returns 200 and the data is gone. Mitigated by ordering (Step 4 before Step 5) and by Step 5's read-back assertion, but it will bite anyone who reorders the steps.
- The authz guard is the single point of failure for the whole backend. `GuardCategoryScope` at `services/exporter/internal/authz/guard.go:167` hard-denies any scope missing from `categoryActions`, and passes the taxonomy scope straight through as the authz scope. Ship the scopes without Step 2 and every create, patch and delete on the new views returns 403 while reads work perfectly — a confusing half-working feature.
- `CategoryActionsColumn.tsx:81-83` currently ends in `: true`. Adding the two scopes without Step 8 ships edit and delete buttons that are enabled for everyone on both new views. The backend would still reject the calls after Step 2, so it is a UX and trust failure rather than a breach, but it is exactly the kind of thing that reads as a security bug in review.
- `MainPage.tsx:20-27` returns `ProtectionPlansEmptyPage` before the toolbar exists, and that page is a bare `EmptyState` with no toolbar at all. Without the Step 10 fix the entire feature is unreachable on a fresh install — the worst possible audience to hide it from.
- `TOOLBAR_COMPACT_WIDTH.DEFAULT` is 760 and its own comment enumerates the controls that produced it: "count + 6 phase pills + filter + search + create". Adding a dropdown invalidates that measurement. Guessing a new number has been wrong repeatedly on this codebase — measure the container.
- Two unreleased shared modules sit on the critical path. `internal/data` and `internal/rest` resolve only through `go.work` replace directives, so everything builds locally and nothing builds in CI until they are released and pinned. Compounding this: a dependency-upgrade workflow is editing `go.mod`/`go.sum` across those modules and all four services right now, so implementation should not start until it finishes.
- `services/exporter` carries roughly 40 files of unfinished lint work from a stopped agent, mostly under `internal/tests/`. Steps 1-3 add tests into that same tree. Expect `golangci-lint run` noise that predates this change; do not try to fix or revert it, and do not let it mask a genuine new finding.
- The Step 3 built-in guard is a behaviour change for groups and roles, not only for the new scopes. Any existing client that mutates a built-in category through the API starts failing. I found no such caller, and seeding bypasses the HTTP route entirely, but it is a real change to existing surface area.
- Ids leak into the UI if Step 12 is skipped or half-done. `environmentID` and `tagIDs` store `cat-00001-0002-0001`-style values; every read surface has to resolve them through the taxonomy or users see opaque ids on cards and in filter chips.
- The singleton Category CR now holds four scopes in one array under a single `concurrency.GetLock(constants.CategoriesCRDName)` read-modify-write. Thirteen built-ins plus custom rows is nowhere near a problem, but the contention point is now shared across four features instead of two. Worth knowing, not worth fixing.

## TDD sequence

- **1. Built-in Environment and Tag values exist as Categories in two new taxonomy scopes (plan-environments, plan-tags), WithBuiltins restores them after tampering, keeps custom values in those scopes, and is idempotent across repeated seeds. Their IDs satisfy the category CRD id pattern.**
  - _layer_: shared-types (internal/data)
  - _testFile_: /Users/houssem/Desktop/Github/internal/data/tests/builtin/builtin_test.go
  - _testNames_:
    - TestBuiltinPlanTaxonomyCategories
    - TestBuiltinCategoryIDsMatchCRDPattern
    - TestWithBuiltinsKeepsCustomPlanTaxonomies
    - TestWithBuiltinsIsIdempotent
  - _redCriterion_: Compile-time RED. `cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/builtin/` fails to build with `undefined: category.ScopePlanEnvironments` / `undefined: constants.CategoryIDEnvProduction`, because the new scope constants and built-in rows do not exist. This is the intended RED signal (new test newly references missing code).
  - _greenCriterion_: `cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/builtin/ -v` passes all four new tests AND the five pre-existing tests in the file still pass (TestBuiltinRoleCategoriesResolve, TestBuiltinRoleScopes, TestWithBuiltinsKeepsUserCategoriesAndRestoresBuiltins, TestWithBuiltinsOnEmptySeedsOnlyBuiltins, TestBuiltinRolesAreProtected).
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/data/classification/category/def.go
    - /Users/houssem/Desktop/Github/internal/data/classification/category/builtin.go
    - /Users/houssem/Desktop/Github/internal/data/constants/builtin.go
- **2. Four shared deny-rule keys exist for plan taxonomies (view/add/edit/delete), they are distinct from every other action constant, and adding them does NOT change the six built-in role scopes or any built-in role's grants.**
  - _layer_: shared-types (internal/data)
  - _testFile_: /Users/houssem/Desktop/Github/internal/data/tests/builtin/builtin_test.go
  - _testNames_:
    - TestPlanTaxonomyActionsAreDistinct
    - TestBuiltinRoleScopesUnchangedByPlanTaxonomies
  - _redCriterion_: Compile-time RED. Same command as step 1 fails with `undefined: role.ActionViewPlanTaxonomies`, `undefined: role.ActionAddPlanTaxonomy`, `undefined: role.ActionEditPlanTaxonomy`, `undefined: role.ActionDeletePlanTaxonomy`.
  - _greenCriterion_: `cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/builtin/ -v` passes; TestBuiltinRoleScopes (pre-existing, asserts exactly 6 scopes per non-Admin role) still passes, proving no new role scope was introduced and no role re-seeding/migration is required.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/data/resources/role/rules.go
- **3. ProtectionPlan carries EnvironmentIDs and TagIDs, they survive the struct -> CR spec map round-trip that the exporter performs, and a plan created before the feature (no keys in the spec) decodes to nil slices without error.**
  - _layer_: shared-types (internal/data)
  - _testFile_: /Users/houssem/Desktop/Github/internal/data/tests/serialization/spec_test.go
  - _testNames_:
    - TestProtectionPlanTaxonomyRoundTrip
    - TestProtectionPlanPredatingTaxonomiesDecodes
  - _redCriterion_: Compile-time RED. `cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/serialization/` fails with `unknown field EnvironmentIDs in struct literal of type plans.ProtectionPlan`.
  - _greenCriterion_: `cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/serialization/ -run ProtectionPlan -v` passes. Reuse the existing `specMap(t, v)` helper already defined at the top of that file.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/data/plans/protectionplan.go
- **4. PATCH request types carry the taxonomies, and clearing the last tag actually reaches the wire. MapToJSONPayload currently drops any `omitempty` slice whose len==0 (isZeroValue, reflect.Slice branch), so `TagIDs: []string{}` silently no-ops. Declaring the patch fields as *[]string makes nil mean 'untouched' and &[]string{} mean 'clear', which MapToJSONPayload already serializes as `[]` with NO mapper change (non-nil pointer is not zero; processFieldValue derefs to processSliceValue which returns []any{}).**
  - _layer_: api-contract (internal/rest)
  - _testFile_: /Users/houssem/Desktop/Github/internal/rest/tests/mappers/plans_patch_test.go
  - _testNames_:
    - TestPatchPayloadOmitsUntouchedTaxonomies
    - TestPatchPayloadClearsAllTags
    - TestPatchPayloadCarriesTaxonomyIDs
  - _redCriterion_: Runtime + compile RED. `cd /Users/houssem/Desktop/Github/internal/rest && go test ./tests/mappers/` first fails to build (`unknown field TagIDs`); after the plain `[]string` field is added it fails at runtime with `payload["tagIDs"] missing, want []` for TestPatchPayloadClearsAllTags — that failure is the real bug and MUST be observed before switching to *[]string.
  - _greenCriterion_: `cd /Users/houssem/Desktop/Github/internal/rest && go test ./tests/mappers/ -v` passes all three: untouched -> key absent, cleared -> key present as empty array, set -> key present with the ids.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/internal/rest/endpoints/plans/types.go
- **5. GuardCategoryScope decouples the taxonomy scope from the authz scope. plan-environments and plan-tags both resolve to the protection-plans role scope with the four shared rule keys, so an Owner/Contributor/ReadOnly user can manage them. Unknown scopes are still hard-denied. Without this every category write on the new scopes 403s for every non-Admin user, because x-ware grantedLevel() falls back only to ScopeAll and only Admin holds ScopeAll.**
  - _layer_: backend-authz (exporter)
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/authz/guard_extra_test.go
  - _testNames_:
    - TestGuardCategoryScope
    - TestGuardCategoryScopeUsesProtectionPlansRuleKeys
  - _redCriterion_: Runtime RED, no compile change needed. Add table rows to the existing TestGuardCategoryScope for an identity holding Levels{"protection-plans": Contributor} against scope "plan-environments" expecting true. `cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./internal/tests/authz/ -run TestGuardCategoryScope -v` fails with `GuardCategoryScope = false, want true` because categoryActions has no plan-environments key so `known` is false.
  - _greenCriterion_: Same command passes, including the existing rows (missing identity=false, internal=true, "galaxy"=false, roles-without-grant=false). Change categoryActions to map[string]map[roledata.PermissionLevel]xauthz.Requirement, mirroring the globalConfigFields shape that already sits 30 lines below in the same file.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/authz/guard.go
- **6. A category name must be unique within its scope. Today ValidateAndPrepareCategory checks only that name and scope are non-empty, so two 'Production' environments can be created and the UI-only guard is bypassable. Extract a pure exported helper that takes the existing slice so it is unit-testable without a cluster (no *_internal_test.go, per the tests-under-tests rule).**
  - _layer_: backend (exporter)
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/exporter/internal/tests/classification/classification_test.go
  - _testNames_:
    - TestDuplicateCategoryNameInScope
    - TestDuplicateCategoryNameIsScopeLocal
    - TestDuplicateCategoryNameIgnoresSelfOnRename
  - _redCriterion_: Compile-time RED. `cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./internal/tests/classification/` fails with `undefined: categoryutil.DuplicateNameInScope`.
  - _greenCriterion_: `cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./internal/tests/classification/ -v` passes all three plus the two pre-existing tests (TestExtractCategorySpecFromRequestBody, TestValidateAndPrepareCategoryFailures). Case-insensitive match; same name in a different scope is allowed; renaming a category to its own current name is allowed.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/utils/classification/category/validate.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/handlers/classification/category/handler.go
    - /Users/houssem/Desktop/Github/telark/services/exporter/internal/constants/config.go
- **7. Duplicating a plan copies its environments and tags, exactly as it already copies ParticipantsIDs. DuplicateProtectionPlanRequest stays a 3-field override struct (Name/TimeMode/TimeRange) — it carries no id lists today and must not grow any.**
  - _layer_: backend (discovery)
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planduplicate/duplicate_test.go
  - _testNames_:
    - TestBuildRequestCopiesTaxonomies
  - _redCriterion_: Runtime RED. Set EnvironmentIDs/TagIDs on the existing sourcePlan() fixture, then `cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planduplicate/ -run TestBuildRequestCopiesTaxonomies -v` fails with `environments = 0, want 1` because BuildRequest does not copy them.
  - _greenCriterion_: Same command passes and the four pre-existing tests in the file (TestBuildRequestDefaults, TestBuildRequestOverrides, TestBuildRequestModeSwitchDropsTimeRange, TestAvailableName*) still pass. The fix is two lines in the PrepareProtectionPlanRequest literal in BuildRequest.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/duplicate/duplicate.go
- **8. Creating a plan persists its taxonomies, and updating one emits a PATCH only when they actually changed — including the clear-to-empty case, which must produce an explicit empty array rather than being skipped. buildPatch is unexported, so export it as BuildPatch rather than writing an internal test file.**
  - _layer_: backend (discovery)
  - _testFile_: /Users/houssem/Desktop/Github/telark/services/discovery/internal/tests/planpatch/patch_test.go
  - _testNames_:
    - TestBuildPatchSetsTaxonomiesWhenChanged
    - TestBuildPatchSkipsTaxonomiesWhenUnchanged
    - TestBuildPatchClearsAllTags
    - TestBuildPatchIgnoresTaxonomyOrdering
  - _redCriterion_: Compile-time RED first (`undefined: update.BuildPatch`), then runtime RED once exported: `cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planpatch/ -v` fails with `patch.TagIDs = <nil>, want &[]` because applyComplexPatch only diffs ParticipantsIDs.
  - _greenCriterion_: `cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/planpatch/ -v` passes all four. Reuse the existing stringSliceSetEqual helper for the order-insensitive diff (it is the backend helper; arraysEqualUnordered is the unrelated frontend one).
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/update/patch.go
    - /Users/houssem/Desktop/Github/telark/services/discovery/internal/core/plans/protection/svc.go
- **9. protectionplan.yaml declares environmentIDs and tagIDs as optional string arrays. This CRD is a strict structural schema — its only x-kubernetes-preserve-unknown-fields is at line 131 on policies[].params — so without the schema entry the API server silently prunes both fields: the write returns 200 and the data vanishes. The fields must NOT be added to the `required:` block (lines 49-63) so plans that predate the feature stay valid. categoriesasclassifications.yaml needs NO change: its scope property is `type: string, minLength: 1, maxLength: 50` with no enum, and `cat-0000N-0001-000M` ids satisfy its `^cat-[0-9a-f]{5}-[0-9a-f]{4}-[0-9a-f]{4}$` pattern.**
  - _layer_: chart (CRD pruning gate)
  - _testFile_: /Users/houssem/Desktop/Github/telark/charts/telark-crds/templates/crds/plans/protectionplan.yaml
  - _testNames_:
    - render-check: environmentIDs is an array of string
    - render-check: tagIDs is an array of string
    - render-check: required block still has 15 entries and names neither new field
  - _redCriterion_: `rtk proxy "helm template telark-crds /Users/houssem/Desktop/Github/telark/charts/telark-crds --set app.name=telark | grep -c 'environmentIDs'"` returns 0. (Use rtk proxy — RTK truncates helm template output even through a redirect.)
  - _greenCriterion_: The same grep returns 2 (environmentIDs and tagIDs), each followed by `type: array` / `items: type: string` / `nullable: true`, and `grep -A16 'required:'` still lists exactly the original 15 field names.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/charts/telark-crds/templates/crds/plans/protectionplan.yaml
    - /Users/houssem/Desktop/Github/telark/docs/CRDS.md
- **10. dashboard-ui has NO test runner. @testing-library/react 16.3, @testing-library/jest-dom 6.9, @testing-library/user-event 14.6 and @types/jest 29.5 are all in `dependencies`, but jest, vitest, jsdom and happy-dom are absent from package-lock.json, there is no test script, and vite.config.ts has no test block. Every frontend step below is unrunnable until a runner exists. Add vitest + jsdom as devDependencies, a `test` and `test:coverage` script, and a `test` block in vite.config.ts (environment jsdom, setupFiles importing @testing-library/jest-dom/vitest). Do NOT enable `globals: true` — @types/jest is already installed and its globals would collide; import describe/it/expect from 'vitest' explicitly in every test.**
  - _layer_: tooling (dashboard-ui) — GATED, needs user approval
  - _testFile_: /Users/houssem/Desktop/dashboard-ui/vite.config.ts
  - _testNames_:
    - smoke: runner starts and reports 0 tests
  - _redCriterion_: `cd /Users/houssem/Desktop/dashboard-ui && npm test` fails with `Missing script: "test"`.
  - _greenCriterion_: `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run` exits 0 reporting `No test files found` (or the first suite), and `npm run check-all` is still at zero errors.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/package.json
    - /Users/houssem/Desktop/dashboard-ui/vite.config.ts
    - /Users/houssem/Desktop/dashboard-ui/src/setupTests.ts
- **11. Every string in the Category components is hardcoded to the noun 'Category' and the Scope column renders the raw scope value, so the new views would show a chip reading `plan-environments` and a panel titled 'Add Category'. Add PLAN_ENVIRONMENTS/PLAN_TAGS to SCOPES and a SCOPE_LABELS map giving singular/plural nouns per scope, with the raw scope as the fallback for unknown values, and thread it through the column, panels and delete modal.**
  - _layer_: ui-constants (dashboard-ui)
  - _testFile_: /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/constants/categories.test.ts
  - _testNames_:
    - returns Environment / Environments for the plan-environments scope
    - returns Tag / Tags for the plan-tags scope
    - returns Category / Categories for groups and roles
    - falls back to the raw scope string for an unknown scope
  - _redCriterion_: Runtime RED. `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run categories.test.ts` fails to resolve the import: `SCOPE_LABELS is not exported by constants/categories.ts`.
  - _greenCriterion_: `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run categories.test.ts` passes all four. Note: CATEGORIES_CONSTANTS.BUILT_IN_GROUPS_CAT / BUILT_IN_ROLES_CAT / LOGS (lines ~87-130) are pre-existing dead code with zero importers — do NOT delete them and do NOT mirror the new built-ins into them; the server is the only source of built-ins.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/constants/categories.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryColumns.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/panels/AddCategoryPanel.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/panels/EditCategoryPanel.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/delete/CategoryDeleteModal.tsx
- **12. CategoryActionsColumn's scope prop is optional and its permission resolution ends in a bare `: true` fall-through (lines ~80-83), so any scope it does not recognise gets edit and delete shown to everyone. Make scope required, make the lookup default-DENY, and register the plan taxonomy permissions so the new scopes resolve properly. The deny-rule catalogue must be extended in all three places that mirror it, otherwise the role editor cannot express the new denies.**
  - _layer_: ui-permissions (dashboard-ui)
  - _testFile_: /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryActionsColumn.test.tsx
  - _testNames_:
    - hides edit and delete for a built-in category
    - hides edit and delete for an unrecognised scope
    - shows edit and delete for plan-environments when the user is permitted
    - hides edit and delete for plan-tags when the user is denied
  - _redCriterion_: Runtime RED. `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run CategoryActionsColumn.test.tsx` fails on 'hides edit and delete for an unrecognised scope' — both buttons render, because the permission chain falls through to `true`. That failure is the real default-ALLOW hole, not a missing-code failure.
  - _greenCriterion_: All four pass, and `npm run type-check` reports the new required `scope: string` prop errors nowhere (both existing call sites in groups and roles already pass scope).
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/display/list/CategoryActionsColumn.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/auth/hooks/permissions/permissionEngine.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/roles/constants/scopeRules.ts
- **13. The plan form carries environmentIDs and tagIDs end to end. Critically, editing ONLY the tags must produce a PATCH: hasFormChanges (planFormValues.ts) and normalizeFormSnapshot (usePlanFormState.ts:38) are two independent change detectors and BOTH must learn the new fields, or the save button stays inert. A plan created before the feature must hydrate to empty arrays, not undefined.**
  - _layer_: ui-state (dashboard-ui)
  - _testFile_: /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/planFormValues.test.ts
  - _testNames_:
    - hydrates environment and tag ids from a plan
    - defaults taxonomies to empty arrays for a plan created before the feature
    - emits taxonomies in the prepare payload
    - detects a change when only the tags differ
    - detects a change when the last tag is removed
    - reports no change when only the taxonomy ordering differs
  - _redCriterion_: Runtime RED. `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run planFormValues.test.ts` fails on 'detects a change when only the tags differ' with `expected false to be true`, because hasFormChanges compares only name/description/severity/priority/mode/scope/time/participants.
  - _greenCriterion_: All six pass. Reuse the existing arraysEqualUnordered helper already in planFormValues.ts rather than writing a new comparator. All eight payload declaration sites must be updated or type-check fails: models/index.ts:98, components/create/types.ts:21, clients/prepare.ts:16, clients/update.ts:16, hooks/usePlanActions.ts:21, store/thunks/protectionPlansThunks.ts:79, utils/planFormValues.ts, hooks/usePlanFormState.ts:38.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/planFormValues.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanFormState.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/create/types.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/models/index.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/prepare.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/clients/update.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/usePlanActions.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/store/thunks/protectionPlansThunks.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/create/EnvironmentsAndTagsSection.tsx
- **14. The plans toolbar gains one dropdown with two items (Environments, Tags); picking one flips a viewMode useState in MainPage and renders that taxonomy's list. MainPage currently early-returns ProtectionPlansEmptyPage whenever plans.length===0 with NO viewMode guard, so with zero plans the new views would be unreachable — the groups precedent guards it (`if (shouldShowEmpty && viewMode === 'groups')`). Unlike groups/roles there is no plan page-config hook, so one small list component is needed; it composes the existing useCategoryListView hook and CategoryColumns component rather than reimplementing a table.**
  - _layer_: ui-views (dashboard-ui)
  - _testFile_: /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/MainPage.test.tsx
  - _testNames_:
    - renders the plans list by default
    - switches to the environments view when chosen from the toolbar dropdown
    - switches to the tags view when chosen from the toolbar dropdown
    - keeps the taxonomy views reachable when there are no plans
    - shows a taxonomy empty state when a scope has no values
    - surfaces a retry action when the taxonomy fetch fails
  - _redCriterion_: Runtime RED. `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run MainPage.test.tsx` fails on 'keeps the taxonomy views reachable when there are no plans' — the empty page renders instead of the environments table, because of the unguarded early return. The first two cases fail because no dropdown is rendered.
  - _greenCriterion_: All six pass. Build the two-item ToolbarButtonConfig inline in the existing useMemo in ProtectionPlansToolbar.tsx — do NOT reuse getManageCategoriesButtonConfig (it is a fixed View/Add pair, the wrong shape) and do NOT add a new factory. The taxonomy views must derive loading/error locally: categorySlice keeps data per-scope but loading/error are global, so a plans-side fetch can flip the groups and roles category views.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/MainPage.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/layout/ProtectionPlansToolbar.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/PlanTaxonomyListPage.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts
- **15. Taxonomy ids resolve to names on plan cards and the detail page, a dangling id (its category was deleted) renders a neutral fallback instead of a blank chip, and deleting a taxonomy value that plans still reference warns with the count first. The existing getCategoryName helper in categories/utils/helpers.ts already returns the em-dash fallback for an unknown id — reuse it, do not write a second resolver.**
  - _layer_: ui-integrity (dashboard-ui)
  - _testFile_: /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/planTaxonomyChips.test.ts
  - _testNames_:
    - resolves environment and tag ids to their names
    - renders the fallback for an id whose category was deleted
    - counts the plans referencing a taxonomy value
    - reports zero for a taxonomy value no plan references
  - _redCriterion_: Runtime RED. `cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run planTaxonomyChips.test.ts` fails to resolve `countPlansUsingTaxonomy` from the module under test.
  - _greenCriterion_: All four pass and the delete modal shows the in-use count. No filter work in this step: FILTER_KEYS, LABELS.FILTER and applyPlanFilters stay untouched — filtering plans by environment/tag was not asked for.
  - _implementationFiles_:
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/utils/planTaxonomyChips.ts
    - /Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/pages/details/Content.tsx
    - /Users/houssem/Desktop/dashboard-ui/src/features/access-and-permissions/categories/components/delete/CategoryDeleteModal.tsx
- **16. Every touched Go module builds, tests and lints clean, and dashboard-ui type-checks, lints and tests clean. No step is complete until these four commands are green together — a module that passed in isolation can still be broken by a later step's shared-type change.**
  - _layer_: gate (whole project)
  - _testFile_: (no new test file — whole-project gates)
  - _testNames_:
    - go test ./... in internal/data, internal/rest, services/exporter, services/discovery
    - golangci-lint run ./... in internal/data, internal/rest, services/exporter, services/discovery
    - npm run check-all at zero errors
    - npm test -- --run all green
  - _redCriterion_: Any of the four commands reports a failure, a non-zero error count, or a NEW lint finding relative to the baseline captured before step 1.
  - _greenCriterion_: All four green. services/exporter carries ~40 files of unfinished lint work from a stopped agent (mostly under internal/tests/) — capture its golangci-lint baseline BEFORE step 5 and compare deltas; do not finish or revert that work, and do not count its pre-existing findings as this feature's failures. Warnings are acceptable in dashboard-ui; errors must be zero and no eslint-disable may be added.
  - _implementationFiles_:
    - /Users/houssem/Desktop/Github/telark/.golangci.yml
    - /Users/houssem/Desktop/Github/internal/data/.golangci.yml
    - /Users/houssem/Desktop/Github/internal/rest/.golangci.yml
    - /Users/houssem/Desktop/dashboard-ui/eslint.config.mjs

## TDD — test framework notes

BACKEND (Go) — read, not assumed, from 5 files in internal/data/tests, 41 in exporter/internal/tests, 40 in discovery/internal/tests.

- Plain stdlib `testing`. NO testify, NO gomock, NO assertion library anywhere in the tree. Assertions are hand-written `if got != want { t.Errorf(...) }`.
- Style: table-driven with an anonymous `[]struct{...}` slice plus `t.Run(tt.name, ...)`. `t.Fatalf` for setup/invariant failures, `t.Errorf` to keep going inside a loop.
- Layout rule confirmed in practice: every *_test.go lives under `<module>/tests/<area>/` (internal/data, internal/rest, internal/x-ware) or `services/<svc>/internal/tests/<area>/` (exporter, discovery). Package name = directory name (`package builtin`, `package authz`, `package planduplicate`). ZERO test files sit beside production code, so any unexported symbol under test must be exported first (step 8 exports BuildPatch).
- Shared helpers: discovery has `internal/tests/testutil` with generic `Equal[T comparable](t, name, got, want)` and `RedisEnv(t)` (miniredis via t.Setenv). Exporter has NO testutil package — its suites define local helpers per directory (e.g. `requestAs(...)`, `noIdentityRequest()` in tests/authz). internal/data/tests/serialization defines `specMap(t, v)` which mirrors the exporter's StructToSpecMap and is the right helper for step 3.
- Constants discipline extends into tests: they use `constants.DefaultInitValue` / `constants.SingleItem` / `constants.EmptyString` instead of bare 0, 1, "".
- HTTP handlers are tested with `httptest.NewRequest` + `httptest.NewRecorder`, no server spun up.
- Commands: `cd <module-root> && go test ./tests/...` (internal modules) or `cd /Users/houssem/Desktop/Github/telark/services/<svc> && go test ./internal/tests/...`. The telark workspace resolves internal/* via `replace` directives in /Users/houssem/Desktop/Github/telark/go.work; internal/data and internal/rest are standalone modules with no go.work of their own.

FRONTEND (dashboard-ui) — THE ANSWER IS: THERE IS NO TEST FRAMEWORK, AND FRONTEND TESTS CANNOT BE RUN TODAY.

Evidence gathered directly:
- package.json scripts are exactly: dev, generate:licenses, build, build:analyze, build:cluster, serve, lint, lint:f, format, type-check, check-all, check-all-and-build. There is NO `test` script. `npm test` errors with `Missing script: "test"`.
- Grepping package-lock.json for `node_modules/vitest`, `node_modules/jest`, `node_modules/@vitest/`, `node_modules/jsdom`, `node_modules/happy-dom` returns ZERO matches. No runner and no DOM environment is installed at any depth.
- No vitest.config.*, jest.config.*, or playwright.config.* anywhere outside node_modules. vite.config.ts contains no `test` block (grep for "test" returns 0 hits in that file).
- `find src -name "*.test.*" -o -name "*.spec.*"` returns nothing. No `__tests__` directory exists. There is not a single frontend test in the repo.
- What IS installed, misplaced in `dependencies` rather than devDependencies: @testing-library/react ^16.3.0, @testing-library/jest-dom ^6.9.1, @testing-library/user-event ^14.6.1, @types/jest ^29.5.14. Someone provisioned the assertion/render libraries and never added a runner, so they are currently dead weight.
- The only executable checks today are `npm run type-check` (tsc --noEmit), `npm run lint` (eslint), and `npm run check-all` (both). k6 exists under /k6 but it is cluster load testing, not UI testing.
- Package manager: npm (package-lock.json v3, no pnpm/yarn/bun lockfile, no `packageManager` field). The runner is a separate decision from the package manager and there is no runner.

CONSEQUENCE, and the one decision the execution workflow must confirm before starting the UI half: steps 11-15 are not runnable without step 10 (add vitest + jsdom). Vitest is the right choice over Jest because the project already builds on Vite 8 with @vitejs/plugin-react, so it inherits the existing resolve/alias/tsconfig setup with a ~6-line config block instead of a separate Babel/ts-jest transform chain. Two caveats: (1) @types/jest is installed, so `globals: true` must stay OFF and every test must `import { describe, it, expect } from 'vitest'` explicitly or the global type declarations collide; (2) `@vitest/coverage-v8` is a third dependency, needed only if coverage numbers are required.

FALLBACK IF THE USER DECLINES THE DEPENDENCY: skip step 10, drop the .test.tsx steps' runtime assertions, and downgrade steps 11-15 to compile-time RED via `npm run type-check` — a new required prop or a new field on FormValues produces a real tsc failure that is a valid compile-time RED signal under the skill's rules. This is materially weaker: it proves the shapes line up but proves NOTHING about behaviour, which means the three genuine behavioural bugs this plan targets (the default-ALLOW fall-through in CategoryActionsColumn, the unguarded empty-state early return in MainPage, and hasFormChanges ignoring the new fields) would ship unverified. Recommend approving step 10.

## TDD — coverage targets

GO — per-module, measured with the same runner already in use:
- `cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/... -cover` — the four files touched (classification/category/def.go, builtin.go, constants/builtin.go, resources/role/rules.go, plans/protectionplan.go) are declaration-only plus one function (WithBuiltins), so target 100% of WithBuiltins and full enumeration of both new built-in scopes. Steps 1-3 reach this.
- `cd /Users/houssem/Desktop/Github/internal/rest && go test ./tests/... -cover` — target: all three branches of the new PATCH taxonomy fields (nil / empty / populated) through MapToJSONPayload. Step 4.
- `cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./internal/tests/... -cover` — target: every (taxonomy-scope, permission-level) pair in the rewritten categoryActions map plus the unknown-scope deny and the internal-caller bypass; every branch of DuplicateNameInScope. Steps 5-6.
- `cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/... -cover` — target: BuildRequest taxonomy copy, and all four BuildPatch branches (unchanged / changed / cleared / reordered). Steps 7-8.
The 80% floor from the TDD skill applies to the NEW code only. Do not chase a global percentage on exporter or discovery — both carry large pre-existing surfaces that this feature does not touch, and exporter additionally carries ~40 files of unfinished lint work. Capture each module's `-cover` number before step 1 and compare the delta.

FRONTEND — only meaningful if step 10 is approved:
- `cd /Users/houssem/Desktop/dashboard-ui && npm run test:coverage` (requires adding @vitest/coverage-v8 alongside vitest and jsdom).
- Target 80%+ on the files this feature creates or rewrites: planFormValues.ts, planTaxonomyChips.ts, PlanTaxonomyListPage.tsx, CategoryActionsColumn.tsx, the SCOPE_LABELS block in categories.ts. Do NOT set a global threshold — the rest of the codebase has zero tests and any repo-wide threshold fails on the first run.
- If step 10 is declined, there is no coverage instrument at all for the frontend. State that plainly in the evidence report rather than reporting a number.

NOT COVERED BY ANY AUTOMATED TEST, and must be listed as known gaps in the step 8 evidence report:
1. The CRD pruning gate (step 9) is verified by grepping rendered helm output, not by a test. Real proof requires writing a plan with taxonomies to a live cluster and reading it back; that is a manual post-deploy check.
2. Seeding idempotency across an actual exporter restart. seedCategories() is scope-blind — it just calls WithBuiltins(existing) and upserts — so step 1's WithBuiltins tests cover the logic, but the boot-loop behaviour itself is only observable in-cluster.
3. Toolbar layout at the compact breakpoint (see executionNotes).
4. End-to-end 403 behaviour for a real Owner/Contributor/ReadOnly session against the live exporter. Step 5 covers the guard function; it does not cover the x-ware grants actually present on a real token.

## TDD — execution notes

ARCHITECTURE, CONFIRMED BY READING (not inherited on trust)
Environment and Tag are NOT new entities. They are two new values of the existing Category `Scope` field, which is a plain untyped `string` in internal/data/classification/category/def.go and is constrained in the CRD only by `minLength: 1, maxLength: 50` with no enum. The whole Category stack — CRD, storage, the six routes, handlers, cache invalidation, seeding, the API client, the Redux slice, CategoryColumns, useCategoryListView, the Add/Edit panels and the delete modal — is already scope-generic end to end. Consequences: no new routes, no new CRD, no change to categoriesasclassifications.yaml, no change to seed.go, no new Redux slice, no new thunks, no new API client.

BLOCKING QUESTION Q1 — DEFAULTED, FLAG TO THE USER ON FIRST CONTACT
Are environments/tags metadata only, or do they participate in policy enforcement scoping? Default taken: METADATA ONLY. Basis: policies/renderer.go buildScopes switches solely on plan.Scope.Type, and RenderMeta is {PlanID, PlanName, CreatedBy, Mode}. The renderer is not touched by any step. If the user wants enforcement scoping, steps 9 and 13-15 all change shape and the plan must be re-cut.

EXACT VERIFICATION COMMANDS (one per layer; each step above names its own -run filter)
- internal/data:  cd /Users/houssem/Desktop/Github/internal/data && go test ./tests/... && golangci-lint run ./...
- internal/rest:  cd /Users/houssem/Desktop/Github/internal/rest && go test ./tests/... && golangci-lint run ./...
- exporter:       cd /Users/houssem/Desktop/Github/telark/services/exporter && go test ./internal/tests/... && golangci-lint run ./...
- discovery:      cd /Users/houssem/Desktop/Github/telark/services/discovery && go test ./internal/tests/... && golangci-lint run ./...
- chart:          rtk proxy "helm template telark-crds /Users/houssem/Desktop/Github/telark/charts/telark-crds --set app.name=telark | grep -A4 environmentIDs"
- dashboard-ui:   cd /Users/houssem/Desktop/dashboard-ui && npm test -- --run && npm run check-all
No GOTOOLCHAIN prefix on golangci-lint (Go 1.27.1 + linter 2.13.2). Never pass --no-config: configs live at /Users/houssem/Desktop/Github/telark/.golangci.yml (covers all four services) and one each in internal/data, internal/rest, internal/x-ware, internal/kcore.
RTK filters command output and truncates even through `>` redirects — it mangled `cat` output during this analysis. Use `rtk proxy "<cmd>"` or the Read tool whenever exact file content or full helm output matters.

TWO CRITIQUE CONFLICTS, ADJUDICATED
1. Server-side built-in protection: CUT, siding with the over-engineering critic. WithBuiltins merges by ID and seedCategories() reconciles on EVERY exporter boot (confirmed in startup/seed.go), so a tampered or deleted built-in self-heals at the next restart — and the UI already hides edit/delete when type === 'built-in'. A guard, an exported IsBuiltin, its error constant and its test are all dead weight. Step 1's TestWithBuiltinsKeepsCustomPlanTaxonomies covers the self-heal instead.
2. Eight rule keys vs four: FOUR, shared. ActionViewPlanTaxonomies / ActionAddPlanTaxonomy / ActionEditPlanTaxonomy / ActionDeletePlanTaxonomy, all under the existing ScopeProtectionPlans, with both taxonomy scopes pointing at the same four. Registered in all three mirrors (role/rules.go, permissionEngine.tsx, scopeRules.ts) — the completeness critic was right that three of the four plumbing layers had no step, and steps 2 and 12 close that.

THE THREE REAL BUGS THIS PLAN TARGETS (each gets a runtime-RED step, not a compile-RED one)
- Step 5: plan taxonomy scopes would pass straight through as authz scopes. x-ware grantedLevel() falls back only to ScopeAll, and builtinScopes contains exactly six strings with uniformPermissions() never granting ScopeAll — so EVERY category write on the new scopes 403s for every Owner, Contributor and ReadOnly user. Only Admin would work. This is the single highest-risk item; do it before any UI work or the whole feature reads as broken.
- Step 12: CategoryActionsColumn's permission chain ends in a bare `: true`, and its `scope` prop is optional — any unrecognised scope shows edit and delete to everyone.
- Step 14: MainPage.tsx early-returns ProtectionPlansEmptyPage with no viewMode guard, so with zero plans the new views are unreachable. groups/pages/MainPage.tsx:225 has the correct precedent.
Plus one latent one, step 4: MapToJSONPayload skips any omitempty slice with len==0, so clearing the last tag silently no-ops. The fix is *[]string on the patch struct only — NOT a mapper change, because a non-nil pointer is not zero and processFieldValue already derefs to an empty JSON array. Note in the report that PatchProtectionPlanRequest.ParticipantsIDs has the identical latent bug; it is pre-existing and OUT OF SCOPE — mention it, do not fix it.

REUSE BEFORE WRITING (verified to exist; do not reimplement)
- getCategoryName(id, categories) in categories/utils/helpers.ts already returns an em-dash for unknown ids — this IS the dangling-id fallback for step 15.
- mapCategoriesToOptions and deduplicateCategoriesByName in the same file.
- useCategoryListView (sort + paginate) and CategoryColumns — groups consumes both from useGroupListPageConfig.tsx:121,186; the new taxonomy list page composes the same two.
- useCategories(scope) is the fetch hook; the client already takes a scope param.
- stringSliceSetEqual in discovery's update/patch.go for the backend order-insensitive diff; arraysEqualUnordered in planFormValues.ts for the frontend one. They are unrelated; do not cross them.
- internal/data/tests/serialization/spec_test.go's specMap(t, v) helper.

EXPLICITLY CUT — do not build these
- Any change to DuplicateProtectionPlanRequest. Confirmed by reading: it has exactly three fields (Name, TimeMode, TimeRange) and carries no id lists. The fix is two lines in duplicate.go BuildRequest.
- A new dropdown factory. Build the two-item ToolbarButtonConfig inline in the existing useMemo in ProtectionPlansToolbar.tsx. getManageCategoriesButtonConfig is a fixed View/Add pair with only two importers (roleListConfig, groupListConfig) — wrong shape, leave it alone.
- A page-config hook for the taxonomy views. Plans has no page-config hook at all (unlike groups/roles); one small list page component is the whole need.
- Filter work: FILTER_KEYS, LABELS.FILTER, applyPlanFilters. Filtering plans by environment or tag was not asked for.
- maxItems on the new CRD arrays (participantsIDs has none either).
- Mirroring built-ins into categories.ts. CATEGORIES_CONSTANTS.BUILT_IN_GROUPS_CAT / BUILT_IN_ROLES_CAT / LOGS at lines ~87-130 are pre-existing dead code with zero importers — leave them, do not extend them, do not delete them (not our mess).
- INSTALL.md, chart README, VALUES.md, helm-docs. The feature adds no values key. docs/CRDS.md only.

SEQUENCING AND LIVE-TREE HAZARDS
- A dependency-upgrade workflow is editing go.mod/go.sum across internal/{rest,x-ware,data} and all four services. Do not start step 1 until it has finished, or module resolution will thrash mid-RED.
- internal/data and internal/rest are UNRELEASED and resolved only via the replace directives in telark/go.work. Steps 1-4 add shared types to both. Until the USER releases those modules and bumps the pins, CI and image builds fail even though local `go test` and `golangci-lint` are green. Say this in the handoff; do not edit go.work, do not bump pins, do not release.
- services/exporter carries ~40 files of unfinished lint work from a stopped agent, mostly under internal/tests/. Capture `golangci-lint run ./...` output BEFORE step 5 and diff against it afterwards. Do not finish it, do not revert it, do not count its findings as this feature's.
- Out of bounds throughout: no service version or Chart.yaml bumps (a GHA owns those), no image builds/pushes, no git commit/push/merge/branch-switch, internal/composer is legacy.

TWO THINGS THAT NEED THE USER, NOT A GUESS
1. Step 10 adds vitest + jsdom (and optionally @vitest/coverage-v8) as devDependencies and churns package-lock.json. Ask before installing. See testFrameworkNotes for the fallback if declined.
2. TOOLBAR_COMPACT_WIDTH.DEFAULT in constants/protectionPlans.ts is 760, with a comment saying it was measured for "count + 6 phase pills + filter + search + create". Adding a dropdown will overflow it. Do NOT guess a new number — guessed responsive thresholds have been wrong repeatedly. Ask the user for a screenshot or the measured container width after step 14 renders, then set it. Never open a browser preview window to check it yourself.

TDD DISCIPLINE FOR THE EXECUTION WORKFLOW
Steps 4, 5, 7, 8, 12, 13 and 14 all reach genuine runtime RED — the test compiles, runs, and fails because the behaviour is wrong. Prefer those failures over compile failures wherever both are reachable: for step 4 and step 8 that means landing the type change first, OBSERVING the behavioural failure, and only then applying the fix. Steps 1, 2, 3, 6 and 11 are compile-time RED (the test newly references missing code), which the skill permits. Checkpoint commits per step: `test:` at RED, `fix:` at GREEN, optional `refactor:`. A test written but never compiled and executed does not count as RED. Commit only if the user asks; this sequence assumes they will.

## Critique verdicts

- **item**
  - _lens_: Completeness and correctness — hunting for layers the plan silently skips, claims that are not true of the real code, and migration/validation/authz/state gaps. Every finding below was verified against the actual files; nothing is reasoned from the plan text alone.
  - _planIsSound_: no
  - _verdict_: Not sound as written — two critical gaps stand. The plan's core architectural bet (reuse the Category stack with two new scope strings) is correct and well-grounded: I verified every file path it names (24 backend, 28 dashboard-ui) exists, and its load-bearing claims are all true — Category.Scope is a free-form string with no enum in def.go or in charts/telark-crds/.../classification/category.yaml; WithBuiltins merges by ID so new-scope built-ins need no logic change and seed.go re-applies them on every boot (so existing installs need no migration); InvalidateCategoryCaches derives scopes from stored data; the store is already keyed `categoriesByScope`; AddCategoryPanel's `CategoryScope` prop type widens automatically from the SCOPES object; protectionplan.yaml's only x-kubernetes-preserve-unknown-fields is on policies[].params:131, so the CRD-before-struct ordering is genuinely load-bearing; groups/pages/MainPage.tsx:225 `if (shouldShowEmpty && viewMode === 'groups')` is exactly the precedent for the plans empty-state fix at plans/pages/MainPage.tsx:20-27. What it misses is authorization and the write path. The new scopes are used verbatim as authz scopes (guard.go:184-188) while x-ware's grantedLevel only resolves scopes a role actually grants — and builtinScopes contains six fixed strings, never ScopeAll — so the whole feature would 403 for every user including Admin, with the failure surfacing as a blank table because no caller of useCategories renders its error. Three of the four permission catalogs (rules.go, SCOPE_RULES, ACTION_PERMISSIONS) have no step at all, so the actions could not be granted, denied, or gated even after the guard is fixed. On the write side, the typed PATCH path drops empty slices (payload.go isZeroValue + `,omitempty`), so 'remove my last tag' silently no-ops; and the frontend payload shape plus two independent change-detection helpers are duplicated across eight sites, not the two the plan names. Fix findings 1-3 before any test is written — they change the shape of the guard signature and the patch request type, which the TDD sequence must be built on.
- **item**
  - _lens_: Over-engineering and scope discipline — bias toward deletion. Every finding names what to CUT and what replaces it.
  - _planIsSound_: no
  - _verdict_: The plan's spine is right and I am not attacking its size — the request genuinely spans CRD, exporter authz, discovery DTOs, and four UI surfaces. Verified and credited as earned: (a) reusing `Category.Scope` with two new taxonomy values instead of minting Environment/Tag CRDs — `internal/data/classification/category/def.go` types `Scope` as a plain `string` and `builtin.go:WithBuiltins` merges by ID, not by scope, so the whole stack really is scope-generic; (b) ordering the `protectionplan.yaml` schema edit before the Go struct edit — structural-schema pruning would silently drop the fields otherwise; (c) the diagnosis that `GuardCategoryScope` is the one hard backend blocker (`services/exporter/internal/authz/guard.go:178-182` hard-denies any scope absent from `categoryActions`), and that `globalConfigFields` right below it is the mirror to copy; (d) the default-deny coverage-test fix; (e) the unguarded empty-state early return in the plans MainPage; (f) the `: true` fall-through in `CategoryActionsColumn`. None of those should be cut.

What fails is the padding around that spine. Three high findings stand, so planIsSound=false. One step rests on a premise that is factually false (`DuplicateProtectionPlanRequest` carries no `ParticipantsIDs` — it is an overrides-only DTO of three fields), and its edit invents a per-duplicate override UI nobody asked for. One whole step (server-side built-in protection) is an unrequested behaviour change to existing groups/roles built-ins that also forces a new export out of the unreleased `internal/data`, enlarging a release the user has to cut. And the permission surface is specced at 8 new rule keys where 4 cover the request, doubling the churn in that same unreleased module plus its hand-synced UI mirrors.

Cut list: the Duplicate DTO edit, Step 3 in full (plus its `IsBuiltin` export, constants and test), 4 of the 8 rule keys, the duplicate built-in test, the sibling factory in the shared categories config, the filter work, the `maxItems` cap, the extra taxonomy page file + hook, and three of the four doc targets. That removes roughly a third of the plan's edit sites and most of its unreleased-module delta without touching a single thing the user asked for.
