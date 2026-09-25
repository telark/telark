# Applications — new cards, view modes and protection-plan coverage

## Status

**PLAN, not implemented. Written 2026-09-23. No step commits, stashes, builds, pushes or deploys; no version or Chart.yaml change. Line numbers were read 2026-09-23 on the working trees and every step re-reads before editing.**

**Tree state (working trees, never HEAD):**

- /Users/houssem/Desktop/dashboard-ui: branch e2e/set_plans_feature_ready_for_mvp_launch @ 012d3ebd, ~62 staged entries (58 modified, 5 added), nothing unstaged. Staged and touched by this plan: ProtectionPlanCard.tsx (approval-mode decide menu, ApprovalDecisionModal, provenance), plans constants/protectionPlans.ts, applications constants/texts.ts, models/application.ts, models/index.ts. Clean and touched: ApplicationCard.tsx, ApplicationCardHeader.tsx, ApplicationsToolbar.tsx, Success.tsx, applicationsSlice.ts, persistConfig.ts, Toolbar.tsx, interfaces/layout/toolbar.ts, src/constants/index.ts, applications constants/index.ts and applications.ts, plans hooks/useProtectionPlans.ts.
- /Users/houssem/Desktop/Github/telark: branch e2e/set_plans_feature_ready_for_mvp_launch @ 5bcfd94, ~152 porcelain entries. Not touched by this plan (no backend, CRD, chart or docs change).
- internal/{data,rest,kcore,x-ware} and landing-page: not touched.

**Assumptions:**

- The Local AI Analyzer apply finishes its dashboard-ui edits to applications models/application.ts, models/index.ts and constants/texts.ts before AP2 and AP3 start (the orchestrator gates or serializes on those files).
- The staged approval-mode edits of ProtectionPlanCard.tsx and protectionPlans.ts stay; AP1 builds on them.
- scope.applicationIds hold Application.name (usePlanFormData.ts:91-96 option value app.name; discovery resolver.go:26-38 indexes by Name) and application names are unique (they key the details route).
- The Permissions plan owns every permission key; PERM U1 adds ACTION_PERMISSIONS.protectionPlans.view and AP2 gates the plan read on it (AP2 crossPlanDependsOn PERM:U1).
- Cross-plan edges live in crossPlanDependsOn as `<part tag>:<step id>` (PERM = protection-plans-permissions, EXCL = protection-plans-exclusions), the convention the PERM plan uses; dependsOn holds only this plan's ids. Every dashboard-ui step of APPS, PERM and EXCL runs in one serialized lane (D15).
- The Exclusions plan stores exclusions outside scope.applicationIds and scope.namespaces (it never removes an application from applicationIds to exclude its resources); coverage never reads them.
- Node on the implementer machine supports type stripping and module.registerHooks (local v25.9.0) for the AP2 node check.

Companion JSON (identical content, machine-readable): `/Users/houssem/Desktop/Github/telark/.claude/plans/applications-cards-coverage.plan.json`. Part tag for the merged schedule: `APPS` (sibling plans: protection-plans-permissions, protection-plans-exclusions).

## Approach

Everything is UI; no backend read path. The applications page already holds the full application summary list (3.06 MB at 2,000 apps) and the plan list is one unpaginated call the home dashboard already polls, so coverage is derived client-side: plans are indexed once per plan-list change by application name and by namespace (O(total scope entries)), and each of the 12 cards on a page costs one lookup per namespace. Card reuse without forking: the protection-plan card's tokens, style objects and small pieces (StatCell, icon chip, status pill, chip, chip section) move verbatim into src/constants/layout/cards.ts and src/components/display/card/, and the plan card consumes them with an identical render (AP1). The applications card is rebuilt on the same parts: header (icon chip in the health accent, title, description, namespace tags, sync pill, status pill, the unchanged kebab menu), a 4-stat grid, a 'Protected by' chip section and a created/updated footer. View modes become a toolbar dropdown with grid (CARD_LAYOUT.CARDS_PER_ROW = 3) and list (1 per row); the persisted field is renamed layoutMode to viewMode so stale 'single'/'double' values need no migration. Three implementation steps: AP1 (shared parts) and AP2 (coverage derivation, after PERM U1 adds the plans view key) do not depend on each other, AP3 (list UI) depends on both; like every dashboard-ui step of the three plans they run one at a time (D15). Files shared with the Permissions, Exclusions and Local AI Analyzer plans get minimal, listed hunks.

## Execution flow today (traced)

ApplicationsGlobalView (pages/main/GlobalView.tsx) loads state.applications.applications (exporter resources/applications/get?view=summary), filters in memory (search, filter panel, health quick filter, excluded namespaces), paginates client-side with APPLICATIONS_PAGE_SIZE = 10 and polls every fetchIntervalSeconds (5 s while a sync runs). ApplicationsSuccess renders DiscoveryStatusBar, ApplicationsToolbar (a ListToolbar with HealthPills, search and a More menu holding the single/double layout toggle and Bulk), an inline CSS grid of 1 or 2 columns keyed on the persisted layoutMode ('single' | 'double'), TablePagination and the bulk reset modal. ApplicationCard is an older design: ApplicationCardHeader (checkbox, 17px title, health dot, description, sync pill, primary namespace tag, permission-gated kebab) plus a six-cell metrics row with two inline literals. Nothing under features/resources/applications reads protection plans; plans are fetched by the plans page, the home dashboard (gated by usePermission('protection-plans', 'ReadOnly')) and plan details, and persisted under protectionPlans.plans. ProtectionPlanCard (the new design) keeps its style objects, StatCell and BlockedRow file-local and borrows APPLICATION_SECTION_LAYOUT.CARD_RADIUS across features; CARD_LAYOUT and ACCENT_TINT live in the plans constants.

## Coverage lifecycle (which phases count)

Coverage is a function of plan phase and plan scope only. active: policies are deployed now (discovery svc.go shouldRender requires phase == active) -> listed as covering, SUCCESS accent, shield icon, title '<plan> · enforcing now'. scheduled: the controller activates it at timeRange.startAt; pending_approval: an approval moves it to scheduled or active -> both listed as upcoming, WARNING accent, clock icon, title '<plan> · scheduled or awaiting approval'. draft (never rendered), failed, canceled and terminated (renderedPolicies cleared) -> never listed. Scope: applications scope covers an app when scope.applicationIds contains app.name; namespaces scope covers an app when scope.namespaces intersects app.namespaces.items[].name; the list that does not match scope.type is ignored. Kind or resource exclusions (Exclusions plan) and per-template kind skips never remove coverage. The index is rebuilt on every plan-list poll, so a plan that activates, is canceled or ends moves on the next fetchIntervalSeconds tick.

## Decisions (defaults the user can override)

| # | Question | Decision | Why |
|---|---|---|---|
| D1 | Backend read path for coverage, or client-side derivation? | Client-side, from selectProtectionPlans and the applications list; no endpoint. | The plan list is fetched whole (exporter handler.go:90-95, no pagination) and already polled and persisted; scope ids are application names; the app summary keeps namespaces (list.go:60-69). An index makes the cost O(total scope entries) once per poll plus a few Map lookups for the 12 cards on a page. A backend endpoint would return what the UI can already compute from the two lists it holds. |
| D2 | Which plan phases count as covering? | active covers now; scheduled and pending_approval are shown as upcoming (distinct accent, icon and title); draft, failed, canceled and terminated never count. | Only active has deployed policies (svc.go:170-174, 702-704); scheduled activates at startAt (controller.go:132-147); approval moves pending to scheduled or active (approval.go:100-104); cancel, terminate and fail clear renderedPolicies (svc.go:617-651). Showing upcoming plans answers 'is this app about to be protected' without claiming it is protected now. |
| D3 | How is an application matched to a plan scope? | applications scope: scope.applicationIds contains app.name. namespaces scope: any of app.namespaces.items[].name is in scope.namespaces. Only the list matching scope.type is read. Coverage means 'in the plan's scope', not 'every resource protected'. | The resolver looks apps up by name and places each in its first namespace (resolver.go:26-43); a namespaces plan renders one policy per namespace matching every resource of the template kinds there (renderer.go:63-70, shared.go:277-279). A multi-namespace app is partially reached either way; the card states scope membership, the plan details state reach. |
| D4 | Do kind or resource exclusions uncover an application? | No. Coverage reads phase, scope.type, scope.applicationIds and scope.namespaces only; exclusions are never read wherever the Exclusions plan stores them. | Required by the feature. The AP2 node check includes a plan carrying exclusions at plan level and inside scope and asserts it still covers. Composition rule for the Exclusions plan: exclusions must not be expressed by removing an app from applicationIds or a namespace from namespaces. |
| D5 | Who may see coverage, and what does a viewer without plan access see? | Gate unconditionally on ACTION_PERMISSIONS.protectionPlans.view (ReadOnly, deny 'protection-plans.viewprotectionplans.deny'), which PERM U1 adds; AP2 carries crossPlanDependsOn PERM:U1, so the key exists when it applies. When denied, no plan fetch is dispatched and the section is not rendered. Never the bare usePermission('protection-plans', 'ReadOnly') and never viewViolations. | Listing plans needs protection-plans Read on the exporter (requirements.go:158) and, after PERM S2, no viewprotectionplans deny; PERM D4/D14 put every plan read in other features on this key, and PERM U4 moves Dashboard.tsx:56 off the bare level check. A bare level check would let a denied role poll into 403s and render chips from the persisted list (persistConfig.ts:42-46); viewViolations (permissionEngine.tsx:212-216) is a different action. Showing 'not covered' to someone who cannot read plans would be false. PERM U5 re-checks this gate and is a no-op. |
| D6 | What does the card show before the plan list has been read? | known = plans.length > 0 \|\| plans !== mountedPlans, where `const [mountedPlans] = useState(plans)` holds the list the page mounted with (after PersistGate rehydration, main.tsx). Unknown shows the EMPTY placeholder ('—') in the section. known never reads loading or error, so it never flips back while polling, and no frame claims 'not covered' before the first fetch fulfills. | Of the list fetch's cases only fulfilled replaces state.plans (protectionPlansSlice.ts:54-57; the plan-action reducers at :69-113 also replace it, and any replaced list is a read one); every poll's pending sets loading = true and error = null (:50-53) and rejected leaves the list (:58-61), so `!loading && !error` flipped an empty list between 'No protection plan…' and '—' on all 12 cards every fetchIntervalSeconds (>= 5 s). A useState flag latched when that condition first holds latches on the first render (initialState loading false, error null, :16-27) and claims 'none' through the whole first fetch; setting it in an effect fails react-hooks/set-state-in-effect (error in eslint-plugin-react-hooks 7.1.1 recommended). No slice edit: other plans change the slice. |
| D7 | How is the new card design reused without forking? | AP1 moves the plan card's tokens (CARD_LAYOUT, ACCENT_TINT), style objects (MICRO_LABEL_STYLE, TRUNCATE_STYLE, DIVIDED_BLOCK_STYLE, CARD_* header/stats/footer styles, getCardShellStyle, getCardMenuButtonStyle) and pieces (StatCell, CardIconChip, CardStatusPill, CardChip, CardChipSection) to src/constants/layout/cards.ts and src/components/display/card/; the plan card consumes them and renders identically. The window block and menus stay plan-only. | The feature forbids forking; the pieces were file-local (terrain) and the card borrowed APPLICATION_SECTION_LAYOUT across features. Style constants replace inline objects without restructuring the plan card's JSX around the Dropdown that the Permissions plan edits. The three existing style names keep their names so other plans' step text still resolves. |
| D8 | Where does CARD_LAYOUT live, and what about the radius? | Root src/constants/layout/cards.ts, re-exported from protectionPlans.ts (`export { CARD_LAYOUT } from '../../../../constants';`); new key CARD_LAYOUT.RADIUS_PX = 8. | Hoist shared constants to root; the re-export keeps ProtectionPlansListPage.tsx and details/Content.tsx (edited by the Permissions and Exclusions plans) untouched. Root constants cannot import SETTINGS_CONSTANTS, so the card radius becomes a root token of the same value, which also removes the plan card's cross-feature import. |
| D9 | Which view modes, and where is the control? | 'grid' (CARD_LAYOUT.CARDS_PER_ROW = 3, the plans grid) and 'list' (1 per row), default grid, in an icon-only toolbar dropdown whose menu marks the active mode; the old More-menu layout toggle and its two strings are removed. | The feature asks for a dropdown with these two modes. ListToolbar's dropdown could not mark the active item (Toolbar.tsx passes no selectedKeys); one optional selectedKeys field fixes that for every toolbar, the same way CompactHealthPills does it. Icon-only keeps the added width fixed at one control. |
| D10 | How are persisted 'single'/'double' values handled? | Rename the field to viewMode (type ApplicationViewMode = 'grid' \| 'list') and whitelist viewMode instead of layoutMode; no migration code. | persistConfig.ts has no version or migrate; keeping layoutMode with new values would leave a type that lies about stale runtime values. The stale key rehydrates as an unused field and is dropped on the next write; everyone lands on grid once. |
| D11 | Page size? | APPLICATIONS_PAGE_SIZE 10 -> 12. | 12 fills every grid row at 3 per row (10 left one card alone) and is fine at 1 per row; GlobalView clamps a persisted page beyond the new last page (effectivePage, GlobalView.tsx:148). |
| D12 | What goes on the application card? | Header: bulk checkbox, icon chip (application icon in the health accent), title, optional description, namespace tags (max CARD_LAYOUT.MAX_TARGET_TAGS plus '+N more'), sync pill, health status pill, the unchanged permission-gated kebab. Stats: Resources, Incidents, Recoveries, Managed by. Section: 'Protected by' chips. Footer: 'created <ago>' / 'updated <ago>'. | Mirrors the plan card's anatomy (header, stats, chip section, provenance footer) with the application's existing data; Created at / Last updated move from the metrics row to the footer as on the plan card. |
| D13 | Health accents on the card? | APPLICATION_HEALTH_ACCENT (healthy SUCCESS, degraded WARNING, down DANGER, else DEFAULT), DEFAULT_COLORS only. | The shared tint lookup (ACCENT_TINT) is keyed by DEFAULT_COLORS accents; getApplicationHealthAccentColor returns a connectivity warning colour and TEXT_MUTED, which have no tint. The util stays for its other callers. |
| D14 | Toolbar compact threshold? | TOOLBAR_COMPACT_WIDTH.DEFAULT 640 -> 676 (one icon-only control: TOOLBAR_CONTROL.HEIGHT 30 + TOOLBAR_ITEM_GAP 6); BULK unchanged; confirmed by screenshot in V1. | The row gains exactly one fixed-width control (toolbars are spaced by TOOLBAR_ITEM_GAP, ListToolbar.tsx:172); in bulk mode the labelled More button disappears, so the bulk row gets narrower. Thresholds are confirmed live, never guessed. |
| D15 | How does this plan compose with the Permissions and Exclusions plans? | No shared contract is changed: the ProtectionPlan type, plan clients, thunks, slices and permission keys are untouched. Shared files and hunk limits: ProtectionPlanCard.tsx (AP1: imports, local definitions, style/element swaps; never usePermission calls, menuItems, handlers, Dropdown, window block, modals, panels), protectionPlans.ts (AP1: CARD_LAYOUT/ACCENT_TINT definitions out, one re-export line in), useProtectionPlans.ts (AP2: an enabled parameter defaulting to true), useApplicationCoverage.ts (created by AP2; PERM U5 re-checks its gate). Lane: every dashboard-ui step of APPS, PERM and EXCL (lane key: repo dashboard-ui) runs one at a time, never beside another dashboard-ui step. Cross-plan edges (crossPlanDependsOn): AP2 after PERM:U1 (the view key, D5); V1 after PERM:U2, PERM:U3, PERM:U4, PERM:U5 and EXCL:U2, so it checks the integrated tree. AP1 and PERM U2 may run in either order: both re-read their anchors and the D16 checks judge only each step's own hunks; a fixed AP1-first order would be an edge on PERM U2 (crossPlanDependsOn APPS:AP1), which the PERM plan owns. Every check-all gate here fixes only this plan's files and reports other errors to the orchestrator. | `npm run check-all` type-checks and lints the whole project, so a gate run while another dashboard-ui step is mid-edit fails on that step's files. File-level serialization alone does not prevent it: AP2 and AP3 share no file with PERM U1-U4 or EXCL U1-U2, which have red windows (PERM U1 before CategoryActionsColumn is fixed; EXCL U1 payload/FormValues types; EXCL U2 ScopeSection props). Keeping this plan's hunks structural and away from the permission and exclusion hunks means either order compiles and neither plan contradicts the other. |
| D16 | How are diff checks scoped when other plans edit the same files? | Per-step copies, never git. Before its first edit, each AP step copies every file of its Files list that already exists into `<scratch>/APPS/<step>/pre/`, keeping the repo-relative path (`cd /Users/houssem/Desktop/dashboard-ui && rsync -R <files> <scratch>/APPS/<step>/pre/`); after its last edit (prettier included) it copies the same files into `<scratch>/APPS/<step>/post/`. `<scratch>` is the orchestrator run's scratchpad directory, outside every repo, kept until R1 finishes. A step's own check is `diff -u <scratch>/APPS/<step>/pre/<file> <file>` and reads 'this step's hunks are limited to X; every other hunk is untouched'; R1 diffs pre against post. | The dashboard-ui baseline is fully staged (Status), so `git diff` shows every later hunk of every plan: PERM U2 edits ProtectionPlanCard.tsx's usePermission block and menuItems, and the Local AI Analyzer edits applications models/application.ts, models/index.ts and constants/texts.ts. A git-based check fails the gate or pushes an implementer to revert another plan's work. With the serialized lane (D15), a pre copy holds exactly the other plans' state. |

## Edge cases

| Case | Behaviour | Covered by |
|---|---|---|
| Viewer can read applications but not plans (no protection-plans ReadOnly, or a role denied viewprotectionplans) | No plan fetch is dispatched; the 'Protected by' section is not rendered (never 'not covered', never chips from the persisted list). | PERM U1 (view key), AP2 (gate on ACTION_PERMISSIONS.protectionPlans.view, useProtectionPlans enabled), AP3 (card renders the section only when coverage is defined), R1 item 3 |
| Cold visit with no persisted plans | Section shows '—' until the first fetch fulfills, then the result; no 'No protection plan covers…' frame before it (D6). | AP2 (known), AP3 |
| Plan-list fetch fails | With persisted plans: coverage from the persisted list (as the plans page shows). Without, and no fetch fulfilled since mount: '—'. After one fulfilled fetch: the last read list stays shown. | AP2 (known) |
| Empty plan list while polling (fresh install, MVP demo) | Text stays 'No protection plan covers this application' across polls: pending and rejected never replace the list, and known reads only the list (D6). | AP2 (known), R1 item 3 |
| Plan phases | active: covering (SUCCESS, shield). scheduled, pending_approval: upcoming (WARNING, clock, explanatory title). draft, failed, canceled, terminated: not listed. | AP2 node check, AP3 |
| Namespaces plan on one namespace of a multi-namespace application | Listed: the plan reaches the app's resources in that namespace. Coverage states scope membership, not full reach (an applications-scope plan also reaches only the app's first namespace, resolver.go:39-43). | AP2 node check (billing, shop) |
| Plan with kind or resource exclusions | Still listed; exclusions are never read. | AP2 node check (p-excl) |
| scope.type is namespaces but applicationIds is also populated (or the reverse) | Only the list matching scope.type counts, as the renderer does. | AP2 node check (p-mixed) |
| Same plan reaches the app through two namespaces | Listed once (Set of names); chips keyed by plan name (unique CR name). | AP2 node check (p-both) |
| 2,000 applications and many plans | Index rebuilt once per plan-list poll in O(total scope entries); 12 lookups per page render; no per-card scan of applicationIds. | AP2 (buildCoverageIndex), AP3 (Success passes coverage per paged app) |
| Persisted layoutMode 'single' or 'double' | Ignored; the new viewMode defaults to grid; the stale key leaves storage on the next write. | AP3 (3) |
| Persisted currentPage beyond the new last page (10 -> 12 per page) | Clamped by effectivePage (GlobalView.tsx:148). | AP3 (7), existing GlobalView |
| Bulk mode in grid or list | Checkbox leads the identity row; card click toggles selection (unchanged); toolbar shows search, view, Exit bulk (More is dropped because it only held Bulk). | AP3 (8), (10), (11) |
| Narrow toolbar row | The view control is already icon-only; below the raised DEFAULT threshold every control falls back to its icon as today. | AP3 (6), V1 screenshot |
| Plan activates, ends, is canceled or deleted while the page is open | Moves or disappears on the next plan-list poll (fetchIntervalSeconds). | AP2 (useProtectionPlans polling) |
| Plan card after the extraction | Same DOM and computed styles; only its source moves to shared parts. | AP1, R1 item 1, V1 screenshot |
| Plans updated before fix F4 still carrying stale renderedPolicies | Irrelevant: coverage uses phase and scope, never renderedPolicies or health. | AP2 |

## Phases

### P0 — Phase 0 — Shared card parts and coverage derivation (independent steps, one at a time in the dashboard-ui lane)

**Goal:** The new card's parts are shared from root without changing the plan card, and coverage can be computed for any application from data already in the store.

**Steps:** AP1, AP2

**Acceptance criteria:**
- npm run check-all zero errors after each step (an error in a file the step did not touch is reported to the orchestrator, D15).
- ProtectionPlanCard.tsx holds no local style objects, StatCell or BlockedRow and no APPLICATION_SECTION_LAYOUT import; /protection-plans renders as before (screenshot).
- The AP2 node check prints 'coverage check ok'.
- useProtectionPlans() keeps its behaviour for MainPage (enabled defaults to true).

### P1 — Phase 1 — Applications list UI

**Goal:** The applications list uses the new card, offers grid (3 per row) and list (1 per row) in a dropdown, and each card names the plans that cover it.

**Steps:** AP3

**Acceptance criteria:**
- Toolbar shows an icon-only view dropdown that marks the active mode; grid lays out 3 cards per row, list 1; the choice survives a reload (viewMode persisted).
- Each card shows header, 4 stats, a 'Protected by' section (active chips SUCCESS/shield, upcoming chips WARNING/clock with titles; 'No protection plan covers this application' when none; '—' while unknown; hidden without plan read permission) and a created/updated footer.
- Kebab menu, bulk selection, sync pill and reset flow behave as before.
- No 'layoutMode', 'Total incidents' or TOOLBAR_LAYOUT_* left in src; no hex, any, console.* or vendor names in the touched files.

### P2 — Phase 2 — Verification and review

**Goal:** Machine checks green, screenshots confirm the layout, review finds no high-severity issue.

**Steps:** V1, R1

**Acceptance criteria:**
- check-all zero errors in this plan's files (any other error reported to the orchestrator); node check green.
- User screenshots: /applications grid (sidebar expanded and collapsed), list, bulk, toolbar around 676px; /protection-plans unchanged.
- R1 reports no open high-severity finding.

## Steps (execution order, one dashboard-ui step at a time: shared parts, and coverage after PERM U1; then the list UI; then verification after the other plans' UI steps)

### AP1 — Shared entity-card parts extracted from the protection-plan card (pixel-identical plan card)

- **Phase:** P0 · **Part:** APPS · **Repo:** dashboard-ui · **dependsOn:** — · **parallelGroup:** ui-shared
- **Route:** claude-opus-5-5 / effort xhigh / P3 — criteria M5 — Introduces the shared entity-card parts (tokens, styles, components) that the protection-plan card and the applications card both adopt; a wrong move changes the plan card's look.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/constants/layout/cards.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/constants/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/interfaces/layout/card.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/card/StatCell.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/card/CardIconChip.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/card/CardStatusPill.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/card/CardChip.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/card/CardChipSection.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/card/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/constants/protectionPlans.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx`

**Change:**

Pure extraction: the protection-plan card must render the same DOM with the same computed styles. Every shared piece is a verbatim move of what ProtectionPlanCard.tsx / protectionPlans.ts hold today (line numbers read 2026-09-23 on the staged approval-mode version; re-read before editing).

(1) NEW src/constants/layout/cards.ts. Imports: `import type { CSSProperties } from 'react'; import { DEFAULT_COLORS } from '../shared/colors';`. MOVE from features/plans/protection/constants/protectionPlans.ts: `CARD_LAYOUT` (lines 448-479; doc comment becomes 'Entity cards (protection plans, applications): the elevated panel geometry every block inside it measures from.') with ONE added key `RADIUS_PX: 8` (the value APPLICATION_SECTION_LAYOUT.CARD_RADIUS resolves to via SETTINGS_CONSTANTS.CONTENT.CARD_BORDER_RADIUS; root constants cannot import a feature), and `ACCENT_TINT` (lines 498-504 with its doc comment). MOVE from ProtectionPlanCard.tsx, exported with the SAME names so any later step text that names them still resolves: `MICRO_LABEL_STYLE` (65-72), `TRUNCATE_STYLE` (74-79), `DIVIDED_BLOCK_STYLE` (81-85). ADD, each a verbatim copy of an inline style object of the plan card: `CARD_HEADER_STYLE` (header div 528-535: display flex, alignItems flex-start, justifyContent space-between, gap 12); `CARD_IDENTITY_STYLE` (536: display flex, alignItems flex-start, gap 10, minWidth 0); `CARD_TITLE_COLUMN_STYLE` (554: display flex, flexDirection column, gap 5, minWidth 0); `CARD_TITLE_STYLE` (title span: ...TRUNCATE_STYLE, fontSize CARD_LAYOUT.TITLE_FONT_SIZE_PX, fontWeight 600, color DEFAULT_COLORS.TEXT_PRIMARY, lineHeight 1.3); `CARD_TAG_ROW_STYLE` (567 and 589: display flex, flexWrap wrap, gap 4, minWidth 0); `CARD_ASIDE_STYLE` (623: display inline-flex, alignItems center, gap 6, flexShrink 0); `CARD_STATS_GRID_STYLE` (743-750: ...DIVIDED_BLOCK_STYLE, display grid, gridTemplateColumns 'repeat(4, minmax(0, 1fr))', columnGap CARD_LAYOUT.CHIPS_GAP_PX); `CARD_FOOTER_STYLE` (803-812: ...DIVIDED_BLOCK_STYLE, display flex, alignItems center, justifyContent space-between, gap 12, fontSize CARD_LAYOUT.META_FONT_SIZE_PX, color DEFAULT_COLORS.TEXT_MUTED); and two functions `getCardShellStyle(hovered: boolean): CSSProperties` (the outer div style at 512-525, borderRadius CARD_LAYOUT.RADIUS_PX in place of APPLICATION_SECTION_LAYOUT.CARD_RADIUS) and `getCardMenuButtonStyle(open: boolean): CSSProperties` (menuButtonStyle at 304-315, menuOpen renamed open).

(2) src/constants/index.ts: add `export * from './layout/cards';` after `export * from './layout/buttons';`.

(3) NEW src/interfaces/layout/card.ts (RowTagProps lives in interfaces/layout/table.ts; same convention): `StatCellProps { label: string; value: React.ReactNode; accent?: string }`, `CardIconChipProps { icon: React.ReactNode; accent: string }`, `CardStatusPillProps { label: React.ReactNode; accent: string }`, `CardChipItem { key: string; label: string; icon: React.ReactNode; accent: string; title?: string }`, `CardChipProps = Omit<CardChipItem, 'key'>`, `CardChipSectionProps { label: string; items: CardChipItem[]; emptyText: string; moreLabel: (hidden: number) => string }`.

(4) NEW src/components/display/card/ (RowTag.tsx shape: `const X: React.FC<XProps> = (...) => ...; export default X;`). StatCell.tsx = StatCell (ProtectionPlanCard.tsx 136-157) verbatim. CardIconChip.tsx = the icon chip span (537-552) with `background: ACCENT_TINT[accent] ?? DEFAULT_COLORS.DEFAULT_TINT`, `color: accent`, `{icon}` as its child. CardStatusPill.tsx = the phase pill span (624-650) with the same tint rule, `color: accent`, the inner dot's background `accent`, `{label}` as text. CardChip.tsx = BlockedRow (159-200) with the avatar `background: ACCENT_TINT[accent] ?? DEFAULT_COLORS.DEFAULT_TINT, color: accent`, `{icon}` in place of `<CloseOutlined />`, and `title={title ?? label}` on the label span. CardChipSection.tsx = the refuses block (765-800): `<div style={DIVIDED_BLOCK_STYLE}>`, `<span style={MICRO_LABEL_STYLE}>{label}</span>`, the `<p>` with emptyText when items is empty, else the `<ul>` of `<CardChip key={item.key} label icon accent title />` for `items.slice(0, CARD_LAYOUT.MAX_POLICY_CHIPS)` plus the trailing `<li style={{ ...MICRO_LABEL_STYLE, alignSelf: 'center' }}>{moreLabel(hidden)}</li>` when hidden > 0. index.ts: `export { default as StatCell } from './StatCell';` and the same for CardIconChip, CardStatusPill, CardChip, CardChipSection.

(5) features/plans/protection/constants/protectionPlans.ts: delete the CARD_LAYOUT and ACCENT_TINT definitions and put `export { CARD_LAYOUT } from '../../../../constants';` where CARD_LAYOUT was. That keeps ProtectionPlansListPage.tsx:18 and pages/details/Content.tsx:31 untouched (both are edited by the Permissions and Exclusions plans). No other hunk in this file.

(6) ProtectionPlanCard.tsx. Imports: drop the APPLICATION_SECTION_LAYOUT import (line 19) and ACCENT_TINT, CARD_LAYOUT from the protectionPlans import; extend the root constants import with the names still used after the edits below (CARD_ASIDE_STYLE, CARD_FOOTER_STYLE, CARD_HEADER_STYLE, CARD_IDENTITY_STYLE, CARD_LAYOUT, CARD_STATS_GRID_STYLE, CARD_TAG_ROW_STYLE, CARD_TITLE_COLUMN_STYLE, CARD_TITLE_STYLE, MICRO_LABEL_STYLE, TRUNCATE_STYLE, getCardMenuButtonStyle, getCardShellStyle); add `import { CardChipSection, CardIconChip, CardStatusPill, StatCell } from '../../../../../../components/display/card';`. Delete the local MICRO_LABEL_STYLE, TRUNCATE_STYLE, DIVIDED_BLOCK_STYLE, StatCell and BlockedRow, `const phaseTint` (279), `shownPolicies` and `hiddenPolicies`. Replace: menuButtonStyle (304-315) with `const menuButtonStyle = getCardMenuButtonStyle(menuOpen);`; the outer div's style object with `style={getCardShellStyle(hovered)}`; the header, identity, title-column, title, both tag-row and aside inline style objects with the CARD_* constants; the icon chip span with `<CardIconChip icon={<SafetyCertificateOutlined />} accent={phaseAccent} />`; the phase pill span with `<CardStatusPill label={phaseLabel} accent={phaseAccent} />`; the stats div style with CARD_STATS_GRID_STYLE; the whole refuses block (765-800) with `<CardChipSection label={isPermanent ? CARD_LABELS.BLOCKED_LABEL_PERMANENT : CARD_LABELS.BLOCKED_LABEL} emptyText={CARD_LABELS.REFUSES_NONE} moreLabel={CARD_LABELS.MORE_BLOCKED} items={policyLabels.map((label) => ({ key: label, label, icon: <CloseOutlined />, accent: DEFAULT_COLORS.DANGER }))} />`; the provenance div style with CARD_FOOTER_STYLE. Do NOT touch the usePermission calls, menuItems, handlers, the Dropdown element and its onClick, the window block, the modals or the panels: the Permissions and Exclusions plans own those hunks. Shared-file note: this step's hunks in ProtectionPlanCard.tsx and protectionPlans.ts are style/element swaps and definition moves only; run it before or after the other plans' card hunks, never concurrently.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx`

- `grep -nE "APPLICATION_SECTION_LAYOUT|function (StatCell|BlockedRow)|const (MICRO_LABEL_STYLE|TRUNCATE_STYLE|DIVIDED_BLOCK_STYLE|phaseTint)" src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx`
- `npm run check-all`

- red: Today the grep prints the cross-feature APPLICATION_SECTION_LAYOUT import and the six local definitions; deleting them before the shared module exists makes check-all report unresolved MICRO_LABEL_STYLE / StatCell / BlockedRow.
- green: grep prints nothing; check-all zero errors; `grep -n "CARD_LAYOUT\|ACCENT_TINT" src/features/plans/protection/constants/protectionPlans.ts` prints only the re-export line.

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all (zero errors; an error in a file this step did not touch is reported to the orchestrator, not fixed here); npx prettier --write <this step's 11 files only> (never `npm run format` or `lint:f`: they rewrite other workflows' uncommitted files); against the pre copies taken before the first edit (D16), `diff -u <scratch>/APPS/AP1/pre/src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx src/features/plans/protection/components/cards/view/ProtectionPlanCard.tsx` shows this step's hunks limited to imports, deletions and style/element swaps, none inside the usePermission calls, menuItems, handlers, Dropdown, window block, modals or panels, and every other hunk already in the pre copy untouched; the same diff of protectionPlans.ts shows only (5) and of src/constants/index.ts only (2); then save the post copies; /protection-plans screenshot identical before and after (collected in V1).

### AP2 — Coverage derivation: plan-scope index, coverage util, useApplicationCoverage hook

- **Phase:** P0 · **Part:** APPS · **Repo:** dashboard-ui · **dependsOn:** — · **crossPlanDependsOn:** PERM:U1 · **parallelGroup:** ui-coverage
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Client-side derivation from two lists the UI already holds, behind the plans view key PERM U1 adds; no contract, persisted shape or authz change.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/models/application.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/models/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/constants/coverage.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/constants/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/utils/coverage.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/hooks/useApplicationCoverage.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/plans/protection/hooks/useProtectionPlans.ts`

**Change:**

(1) models/application.ts: add `import type { ProtectionPlan } from '../../../plans/protection/models';` and, right after `export type ApplicationHealthQuickFilter …` (line ~375): `export type ApplicationCoverageState = 'active' | 'upcoming';`, `export interface ApplicationCoverage { known: boolean; active: string[]; upcoming: string[] }` (known is false until the plan list has been read, so the card never claims 'not covered' on missing data; active/upcoming hold plan names, sorted), `export interface ApplicationCoverageIndex { byApplication: Map<string, ProtectionPlan[]>; byNamespace: Map<string, ProtectionPlan[]> }`. models/index.ts: add the three names to the `export type { … }` list. No other hunk in either file (the Local AI Analyzer edits them).

(2) NEW constants/coverage.ts, type-only imports and no runtime import (the node check loads it directly): `import type { PlanPhase } from '../../../plans/protection/models'; import type { ApplicationCoverageState } from '../models';` then `// Only active plans have policies deployed; scheduled and pending-approval plans will. Every other phase never covers.` and `export const APPLICATION_COVERAGE_BY_PHASE: Partial<Record<PlanPhase, ApplicationCoverageState>> = { active: 'active', scheduled: 'upcoming', pending_approval: 'upcoming' };`. constants/index.ts: add `export * from './coverage';`.

(3) NEW utils/coverage.ts, exactly (every type import is `import type`; the constant comes from '../constants/coverage', not the '../constants' barrel whose texts.ts pulls in the root constants):
```ts
import type { ProtectionPlan } from '../../../plans/protection/models';
import type {
  Application,
  ApplicationCoverage,
  ApplicationCoverageIndex,
  ApplicationCoverageState,
} from '../models';
import { APPLICATION_COVERAGE_BY_PHASE } from '../constants/coverage';

const append = (map: Map<string, ProtectionPlan[]>, key: string, plan: ProtectionPlan): void => {
  const list = map.get(key);
  if (list) list.push(plan);
  else map.set(key, [plan]);
};

// Built once per plan-list change, so each card costs one lookup per namespace
// instead of a scan of every plan's scope (2,000 ids per plan at scale).
export const buildCoverageIndex = (plans: ProtectionPlan[]): ApplicationCoverageIndex => {
  const index: ApplicationCoverageIndex = { byApplication: new Map(), byNamespace: new Map() };
  plans
    .filter((plan) => APPLICATION_COVERAGE_BY_PHASE[plan.phase] !== undefined)
    .forEach((plan) => {
      const byNamespace = plan.scope.type === 'namespaces';
      const keys = (byNamespace ? plan.scope.namespaces : plan.scope.applicationIds) ?? [];
      const target = byNamespace ? index.byNamespace : index.byApplication;
      keys.forEach((key) => append(target, key, plan));
    });
  return index;
};

// Scope and phase only: excluding kinds or resources narrows what a plan
// blocks, never whether the application is in its scope.
export const getApplicationCoverage = (
  index: ApplicationCoverageIndex,
  application: Pick<Application, 'name' | 'namespaces'>,
): Omit<ApplicationCoverage, 'known'> => {
  const plans = [
    ...(index.byApplication.get(application.name) ?? []),
    ...(application.namespaces?.items ?? []).flatMap((ns) => index.byNamespace.get(ns.name) ?? []),
  ];
  const namesIn = (state: ApplicationCoverageState): string[] =>
    [
      ...new Set(
        plans
          .filter((plan) => APPLICATION_COVERAGE_BY_PHASE[plan.phase] === state)
          .map((plan) => plan.name),
      ),
    ].sort((a, b) => a.localeCompare(b));
  return { active: namesIn('active'), upcoming: namesIn('upcoming') };
};
```

(4) features/plans/protection/hooks/useProtectionPlans.ts: signature `export const useProtectionPlans = (enabled = true) => {`; first statement of the polling effect `if (!enabled) return undefined;`; add `enabled` to that effect's dependency array. MainPage.tsx's call stays `useProtectionPlans()`.

(5) NEW hooks/useApplicationCoverage.ts, exactly:
```ts
import { useCallback, useMemo, useState } from 'react';
import { ACTION_PERMISSIONS, usePermission } from '../../../auth/hooks';
import { useProtectionPlans } from '../../../plans/protection/hooks/useProtectionPlans';
import type { Application, ApplicationCoverage } from '../models';
import { buildCoverageIndex, getApplicationCoverage } from '../utils/coverage';

const { view: viewPlans } = ACTION_PERMISSIONS.protectionPlans;

// undefined = the viewer may not read plans: the card hides the section rather
// than claim the application is not covered.
export const useApplicationCoverage = (): ((
  application: Application,
) => ApplicationCoverage | undefined) => {
  const canViewPlans = usePermission(viewPlans.scope, viewPlans.level, viewPlans.deny);
  const { plans } = useProtectionPlans(canViewPlans);
  const [mountedPlans] = useState(plans);
  const index = useMemo(() => buildCoverageIndex(plans), [plans]);
  // Known once the list holds plans or a fulfilled fetch replaced the list this page
  // mounted with; a poll's pending/rejected never touch the list, so it never flickers.
  const known = plans.length > 0 || plans !== mountedPlans;
  return useCallback(
    (application: Application) =>
      canViewPlans ? { known, ...getApplicationCoverage(index, application) } : undefined,
    [canViewPlans, index, known],
  );
};
```
Read gate: ACTION_PERMISSIONS.protectionPlans.view (ReadOnly, deny 'protection-plans.viewprotectionplans.deny'), added by PERM U1, hence crossPlanDependsOn PERM:U1; ACTION_PERMISSIONS and usePermission both come from features/auth/hooks/index.ts. Never the bare `usePermission('protection-plans', 'ReadOnly')` and never viewViolations (a different action, permissionEngine.tsx:212-216). It is the key PERM U4 puts on Dashboard.tsx's plan fetch; PERM U5 (crossPlanDependsOn APPS:AP2) finds this exact gate in place and changes nothing. known reads only the plan list (D6): pending and rejected never replace state.plans (protectionPlansSlice.ts:50-61), so an empty list keeps its text across polls. The hook as written was linted in memory against eslint.config.mjs (react-hooks 7.1.1 recommended, prettier): 0 errors, 0 warnings. Import the hook by path; do not add it to hooks/index.ts. Nothing consumes the hook until AP3.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/utils/coverage.ts`

```sh
cd /Users/houssem/Desktop/dashboard-ui && node --no-warnings --import 'data:text/javascript,import{registerHooks}from"node:module";registerHooks({resolve:(s,c,n)=>{try{return n(s,c)}catch{return n(s+".ts",c)}}})' --input-type=module -e '
import assert from "node:assert/strict";
import { buildCoverageIndex, getApplicationCoverage } from "./src/features/resources/applications/utils/coverage.ts";
const plan = (name, phase, type, ids) => ({ name, phase, scope: type === "namespaces" ? { type, namespaces: ids } : { type, applicationIds: ids } });
const index = buildCoverageIndex([
  plan("p-app", "active", "applications", ["shop"]),
  plan("p-ns", "scheduled", "namespaces", ["payments"]),
  plan("p-pending", "pending_approval", "applications", ["shop"]),
  plan("p-terminated", "terminated", "applications", ["shop"]),
  plan("p-canceled", "canceled", "namespaces", ["payments"]),
  plan("p-failed", "failed", "applications", ["shop"]),
  plan("p-draft", "draft", "namespaces", ["web"]),
  { name: "p-mixed", phase: "active", scope: { type: "namespaces", namespaces: ["elsewhere"], applicationIds: ["shop"] } },
  { name: "p-excl", phase: "active", exclusions: { kinds: ["Secret"] }, scope: { type: "applications", applicationIds: ["shop"], exclusions: { kinds: ["Secret"], resources: [{ kind: "Secret", name: "db" }] } } },
  plan("p-both", "active", "namespaces", ["web", "payments"]),
]);
const shop = { name: "shop", namespaces: { total: 2, items: [{ name: "web" }, { name: "payments" }] } };
assert.deepEqual(getApplicationCoverage(index, shop), { active: ["p-app", "p-both", "p-excl"], upcoming: ["p-ns", "p-pending"] });
assert.deepEqual(getApplicationCoverage(index, { name: "billing", namespaces: { total: 1, items: [{ name: "payments" }] } }), { active: ["p-both"], upcoming: ["p-ns"] });
assert.deepEqual(getApplicationCoverage(index, { name: "lonely", namespaces: { total: 1, items: [{ name: "other" }] } }), { active: [], upcoming: [] });
assert.deepEqual(getApplicationCoverage(buildCoverageIndex([]), shop), { active: [], upcoming: [] });
console.log("coverage check ok");' 
```
- `npm run check-all`

- red: The node check fails with ERR_MODULE_NOT_FOUND because utils/coverage.ts does not exist yet.
- green: The node check prints 'coverage check ok' and exits 0: active phase covers now; scheduled and pending_approval are upcoming; terminated, canceled, failed and draft never cover; applications scope matches by name; namespaces scope matches any of the app's namespaces; only the list matching scope.type counts (p-mixed); exclusions at plan or scope level change nothing (p-excl); a plan reached twice is listed once (p-both). check-all zero errors. Needs Node with type stripping and module.registerHooks (>= 23.6 or >= 22.18; local v25.9.0).

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all (zero errors; an error in a file this step did not touch is reported to the orchestrator, not fixed here) and the node check above; `grep -n "usePermission('protection-plans'\|viewViolations" src/features/resources/applications/hooks/useApplicationCoverage.ts` prints nothing; npx prettier --write <this step's 7 files only>; against the pre copies (D16), `diff -u` shows useProtectionPlans.ts limited to the parameter, the guard and the dependency, and models/application.ts, models/index.ts and constants/index.ts limited to (1) and (2), every other hunk untouched; then save the post copies.

### AP3 — Applications list: view-mode dropdown (grid 3 / list 1), new card design, coverage section

- **Phase:** P1 · **Part:** APPS · **Repo:** dashboard-ui · **dependsOn:** AP1, AP2 · **parallelGroup:** ui-apps
- **Route:** claude-opus-5-5 / effort xhigh / P3 — criteria M2 — Renames the persisted redux-persist field (applications.layoutMode to viewMode) that lives in every user's browser storage, and rebuilds the list card on the shared parts.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/models/application.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/models/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/store/slices/applicationsSlice.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/store/persistConfig.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/interfaces/layout/toolbar.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/components/display/toolbar/Toolbar.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/constants/cards.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/constants/index.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/constants/texts.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/constants/applications.ts`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/components/layout/ApplicationsToolbar.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/pages/main/Success.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/components/cards/view/ApplicationCard.tsx`
  - `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/components/cards/view/ApplicationCardHeader.tsx`

**Change:**

(1) models/application.ts: replace `export type ApplicationLayoutMode = 'single' | 'double';` with `export type ApplicationViewMode = 'grid' | 'list';` and, in ApplicationsState, `layoutMode: ApplicationLayoutMode;` with `viewMode: ApplicationViewMode;`. models/index.ts: ApplicationLayoutMode becomes ApplicationViewMode in the export list.

(2) store/slices/applicationsSlice.ts: initialState `viewMode: 'grid'` in place of `layoutMode: 'single'`; reducer `setViewMode: (state, action: PayloadAction<ApplicationViewMode>) => { state.viewMode = action.payload; }` in place of setLayoutMode (add ApplicationViewMode to the existing `import type` block); export setViewMode in place of setLayoutMode.

(3) src/store/persistConfig.ts: applicationsPersistConfig whitelist 'layoutMode' becomes 'viewMode'. No migration: a stale persisted `layoutMode` rehydrates as an unused extra field and is dropped from storage on the next write; every user lands on grid once.

(4) src/interfaces/layout/toolbar.ts: ToolbarButtonConfig.dropdown gains `selectedKeys?: string[];`. src/components/display/toolbar/Toolbar.tsx: in the dropdown branch's `menu={{ … }}` add `selectedKeys: button.dropdown.selectedKeys,` (the CompactHealthPills pattern, ApplicationsToolbar.tsx:81); nothing else in either file.

(5) NEW constants/cards.ts, exactly:
```ts
import { CARD_LAYOUT, DEFAULT_COLORS } from '../../../../constants';
import type { ApplicationCoverageState, ApplicationViewMode } from '../models';

export const APPLICATION_VIEW_MODES: { key: ApplicationViewMode; label: string }[] = [
  { key: 'grid', label: `Grid · ${CARD_LAYOUT.CARDS_PER_ROW} per row` },
  { key: 'list', label: 'List · 1 per row' },
];

/** Card accents stay on DEFAULT_COLORS so ACCENT_TINT can wash them. */
export const APPLICATION_HEALTH_ACCENT: Record<string, string> = {
  healthy: DEFAULT_COLORS.SUCCESS,
  degraded: DEFAULT_COLORS.WARNING,
  down: DEFAULT_COLORS.DANGER,
};

export const APPLICATION_COVERAGE_ACCENT: Record<ApplicationCoverageState, string> = {
  active: DEFAULT_COLORS.SUCCESS,
  upcoming: DEFAULT_COLORS.WARNING,
};

const COVERAGE_STATE_LABEL: Record<ApplicationCoverageState, string> = {
  active: 'enforcing now',
  upcoming: 'scheduled or awaiting approval',
};

export const APPLICATION_CARD = {
  VIEW_MODE_LABEL: 'View',
  STATS: {
    INCIDENTS: 'Incidents',
    RECOVERIES: 'Recoveries',
  },
  COVERAGE: {
    LABEL: 'Protected by',
    NONE: 'No protection plan covers this application',
    CHIP_TITLE: (plan: string, state: ApplicationCoverageState) =>
      `${plan} · ${COVERAGE_STATE_LABEL[state]}`,
  },
  MORE: (count: number) => `+${count} more`,
  CREATED_PREFIX: 'created',
  UPDATED_PREFIX: 'updated',
} as const;
```
constants/index.ts: add `export * from './cards';`.

(6) constants/texts.ts: delete TOOLBAR_LAYOUT_SINGLE and TOOLBAR_LAYOUT_DOUBLE; TOOLBAR_COMPACT_WIDTH.DEFAULT 640 becomes 676 and its comment's '~585px otherwise' becomes '~621px otherwise: the icon-only view control adds TOOLBAR_CONTROL.HEIGHT + TOOLBAR_ITEM_GAP'; BULK stays 1040 (bulk mode swaps the labelled More button for the icon-only view control, so the row gets narrower). No other hunk (the Local AI Analyzer edits this file).

(7) constants/applications.ts: `APPLICATIONS_PAGE_SIZE = 12` with the comment `// A multiple of CARD_LAYOUT.CARDS_PER_ROW, so the grid's last row is full.`

(8) components/layout/ApplicationsToolbar.tsx: props `viewMode: ApplicationViewMode; onViewModeChange: (mode: ApplicationViewMode) => void;` replace layoutMode/onLayoutModeChange; MORE_MENU_KEYS keeps only BULK; delete nextLayoutMode and nextLayoutLabel; handleMoreMenuClick becomes `if (key === MORE_MENU_KEYS.BULK) onToggleBulkMode();` with deps [onToggleBulkMode]. Module level: `const VIEW_MODE_ICON: Record<ApplicationViewMode, React.ReactNode> = { grid: <AppstoreOutlined />, list: <BarsOutlined /> };`. In the toolbars memo add `const view: ToolbarConfig = { buttons: [{ key: 'viewMode', label: APPLICATION_CARD.VIEW_MODE_LABEL, icon: VIEW_MODE_ICON[viewMode], variant: 'ghost', iconOnly: true, dropdown: { items: APPLICATION_VIEW_MODES.map((mode) => ({ key: mode.key, label: mode.label, icon: VIEW_MODE_ICON[mode.key] })), selectedKeys: [viewMode], onItemClick: (key) => onViewModeChange(key === 'list' ? 'list' : 'grid') } }] };`; the More dropdown's items become the Bulk entry alone; `return bulkMode ? [search, view, exitBulk] : [search, view, more];` (More would be empty in bulk mode); update the memo deps. Sizing comes from Toolbar.tsx (TOOLBAR_CONTROL.HEIGHT, icon-only width), no new size values.

(9) pages/main/Success.tsx: `const viewMode = useSelector((s: RootState) => s.applications.viewMode);` replaces layoutMode; `const coverageOf = useApplicationCoverage();` (import from '../../hooks/useApplicationCoverage'); grid style: display 'grid'; gridTemplateColumns '1fr' when viewMode === 'list', otherwise the template literal repeat(${CARD_LAYOUT.CARDS_PER_ROW}, minmax(0, 1fr)); gap CARD_LAYOUT.GRID_GAP_PX; alignItems 'stretch' (CARD_LAYOUT added to the existing root constants import); pass `coverage={coverageOf(application)}` to ApplicationCard; in the content memo deps viewMode and coverageOf replace layoutMode; toolbar props `viewMode={viewMode} onViewModeChange={(mode) => dispatch(setViewMode(mode))}`; import setViewMode in place of setLayoutMode. Leave the pre-existing inline 'No applications match…' / 'Clear all filters' literals alone.

(10) components/cards/view/ApplicationCard.tsx, rewritten on the shared parts. Props: the existing ones plus `coverage?: ApplicationCoverage`. Keep isActivateKey and the role=button, tabIndex, bulk className, hover state, onClick and onKeyDown behaviour exactly. Root style `{ ...getCardShellStyle(hovered), height: '100%', display: 'flex', flexDirection: 'column' }` (stretched rows stay even; the shell brings the hover border, CARD_LAYOUT padding and radius). Children: `<div style={{ flex: 1 }}>` holding (a) `<ApplicationCardHeader application={application} onEditApplication={onEditApplication} bulkMode={bulkMode} selected={selected} onToggleSelect={onToggleSelect} />`; (b) `<div style={CARD_STATS_GRID_STYLE}>` with four StatCell: APPLICATIONS_UI.CARD.LABELS.RESOURCES / `application.resourceCount ?? 0`; APPLICATION_CARD.STATS.INCIDENTS / `application.metrics?.derived?.totalIncidents ?? 0`; APPLICATION_CARD.STATS.RECOVERIES / `application.metrics?.derived?.totalRecoveries ?? 0`; APPLICATIONS_UI.CARD.LABELS.MANAGED_BY / `application.managed?.by || APPLICATIONS_UI.FALLBACKS.EMPTY`; (c) when coverage is defined, `<CardChipSection label={APPLICATION_CARD.COVERAGE.LABEL} emptyText={coverage.known ? APPLICATION_CARD.COVERAGE.NONE : APPLICATIONS_UI.FALLBACKS.EMPTY} moreLabel={APPLICATION_CARD.MORE} items={[...coverage.active.map((name) => coverageChip(name, 'active')), ...coverage.upcoming.map((name) => coverageChip(name, 'upcoming'))]} />`, with module-level `const COVERAGE_ICON: Record<ApplicationCoverageState, React.ReactNode> = { active: <SafetyCertificateOutlined />, upcoming: <ClockCircleOutlined /> };` and `const coverageChip = (name: string, state: ApplicationCoverageState): CardChipItem => ({ key: name, label: name, icon: COVERAGE_ICON[state], accent: APPLICATION_COVERAGE_ACCENT[state], title: APPLICATION_CARD.COVERAGE.CHIP_TITLE(name, state) });` (icon, accent and title differ, so the two states never differ by colour alone). After that wrapper, the footer `<div style={CARD_FOOTER_STYLE}>`: `<span style={TRUNCATE_STYLE}>{APPLICATION_CARD.CREATED_PREFIX} {createdAt ? <TimeAgo date={createdAt} /> : APPLICATIONS_UI.FALLBACKS.EMPTY}</span>` and `<span style={{ flexShrink: 0 }}>{APPLICATION_CARD.UPDATED_PREFIX} {lastUpdated ? <TimeAgo date={lastUpdated} /> : APPLICATIONS_UI.FALLBACKS.EMPTY}</span>`. Delete METRICS_ROW_STYLE, MetricMini, the 'Total incidents' / 'Total recoveries' literals, primaryNamespace and the APPLICATION_SECTION_LAYOUT import.

(11) components/cards/view/ApplicationCardHeader.tsx: keep every hook, selector, permission, handler, menu item, labelWithTooltip, SYNC_TAG_CONFIG and the ApplicationResetModal exactly; only markup and styles change. Remove the primaryNamespace prop and bulkTextIndent. `const accent = APPLICATION_HEALTH_ACCENT[(application.health?.status ?? '').toLowerCase()] ?? DEFAULT_COLORS.DEFAULT;` (getApplicationHealthAccentColor stays for its other callers). Markup: `<div style={CARD_HEADER_STYLE}>`, left `<div style={CARD_IDENTITY_STYLE}>`: the existing bulk Checkbox span when bulkMode; `<CardIconChip icon={<ApplicationIcon />} accent={accent} />` with module-level `const ApplicationIcon = Icons.Application;` (the sidebar's application icon, Icons from the root constants); `<span style={CARD_TITLE_COLUMN_STYLE}>` holding the title `<span title={title} style={CARD_TITLE_STYLE}>{title}</span>` (title = displayName || name), the description only when non-empty as `<span title={descriptionText} style={{ ...TRUNCATE_STYLE, fontSize: CARD_LAYOUT.META_FONT_SIZE_PX, color: DEFAULT_COLORS.TEXT_MUTED }}>`, and `<span style={CARD_TAG_ROW_STYLE}>` with one RowTag per `application.namespaces?.items` name sliced to CARD_LAYOUT.MAX_TARGET_TAGS (props as the plan card's target tags: capitalize={false}, background DEFAULT_COLORS.CHIP_CUSTOM_BG, color DEFAULT_COLORS.TEXT_SECONDARY, fontSize CARD_LAYOUT.TAG_FONT_SIZE_PX), a RowTag `APPLICATION_CARD.MORE(hidden)` in DEFAULT_COLORS.TEXT_MUTED when more namespaces exist, then the existing sync pill (Tooltip + span, unchanged). Right `<div style={CARD_ASIDE_STYLE}>`: `<CardStatusPill label={statusText} accent={accent} />`, then the existing Dropdown whose Button style becomes `getCardMenuButtonStyle(menuOpen)` (replaces the local menuButtonStyle useMemo; radius 10 becomes the shared 8). Remove the h3 and the dot plus status text.

**Test first:** `/Users/houssem/Desktop/dashboard-ui/src/features/resources/applications/models/application.ts`

- `npm run check-all`
- `grep -rnE "layoutMode|ApplicationLayoutMode|setLayoutMode|TOOLBAR_LAYOUT_" src`

- red: After the type rename in (1), check-all reports errors in applicationsSlice.ts, ApplicationsToolbar.tsx and Success.tsx and the grep lists every old name.
- green: grep prints nothing; check-all zero errors.

**Verify:** cd /Users/houssem/Desktop/dashboard-ui && npm run check-all (zero errors; an error in a file this step did not touch is reported to the orchestrator, not fixed here); the AP2 node check still prints 'coverage check ok'; `grep -rn "Total incidents\|Total recoveries" src` prints nothing; `grep -nE "#[0-9a-fA-F]{3,8}\b|console\.|: any\b|as any" src/features/resources/applications/components/cards/view/ApplicationCard.tsx src/features/resources/applications/components/cards/view/ApplicationCardHeader.tsx src/features/resources/applications/constants/cards.ts src/features/resources/applications/components/layout/ApplicationsToolbar.tsx` prints nothing; against AP3's pre copies (D16), `diff -u` of constants/texts.ts, models/application.ts and models/index.ts shows only the hunks listed in (1) and (6), with AP2's and the Local AI Analyzer's hunks untouched; npx prettier --write <this step's 14 files only>; then save the post copies.

### V1 — Verify dashboard-ui: check-all, coverage check, UI-rule greps, screenshots

- **Phase:** P2 · **Part:** APPS · **Repo:** dashboard-ui · **dependsOn:** AP3 · **crossPlanDependsOn:** PERM:U2, PERM:U3, PERM:U4, PERM:U5, EXCL:U2 · **parallelGroup:** verify
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Machine-checked gate plus a screenshot request.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui`

**Change:**

Runs after AP3 and, through crossPlanDependsOn, after PERM U2-U5 and EXCL U2 (PERM U1 and EXCL U1 precede those), so check-all judges the integrated tree. Run in /Users/houssem/Desktop/dashboard-ui: `npm run check-all`; fix only errors in files AP1-AP3 created or edited, and only inside this plan's hunks (D16 post copies), without eslint-disable, any, console.* or hex; report every other error as file:line to the orchestrator and leave that file alone; format only this plan's files with `npx prettier --write <file>`; the AP2 node check; `grep -rnE "layoutMode|ApplicationLayoutMode|TOOLBAR_LAYOUT_" src` (empty); `grep -rn "APPLICATION_SECTION_LAYOUT" src/features/plans` (empty). Do NOT run `npm run build` or `check-all-and-build`: generate:licenses rewrites the staged public/licenses.json. Never open a browser: ask the user for screenshots of /applications in grid (sidebar expanded and collapsed), list and bulk mode, of the toolbar just above and below 676px, and of /protection-plans (must match the pre-change look).

**Test first:** `/Users/houssem/Desktop/dashboard-ui/package.json`

- `npm run check-all`
- `AP2 node check`

- red: n/a
- green: zero errors in this plan's files, every other error reported; 'coverage check ok'

**Verify:** zero errors in this plan's files and every other error reported to the orchestrator; screenshots received and matching the acceptance criteria of P1.

### R1 — Review the applications-cards-coverage change set

- **Phase:** P2 · **Part:** APPS · **Repo:** dashboard-ui · **dependsOn:** V1 · **parallelGroup:** review
- **Route:** claude-opus-5-5 / effort high / P4 — criteria none — Read-only review of a UI change set; the orchestrator folds it into its integrated verification.
- **Files:**
  - `/Users/houssem/Desktop/dashboard-ui`

**Change:**

Read-only review against this plan, findings as file:line: (1) plan card identity: the ProtectionPlanCard.tsx diff between AP1's pre and post copies (D16; never `git diff`, which also shows PERM U2's hunks) only swaps inline styles and elements for shared ones with equal values, and every shared piece is a verbatim move; (2) coverage semantics per D2-D4: phase map, applications scope by name, namespaces scope by intersection, only the list matching scope.type, exclusions never read, dedupe; (3) the read gate is ACTION_PERMISSIONS.protectionPlans.view (scope, level and deny; no bare usePermission('protection-plans', …) and no viewViolations in useApplicationCoverage.ts), no plan fetch is dispatched without it, the section is hidden when denied and shows the placeholder until the list is known, and known reads only the plan list (never loading or error) so an empty list keeps its text across polls; (4) UI rules: no any, no console.*, no hex, no vendor names, strings in constants files, DEFAULT_COLORS only, toolbar sizing through Toolbar.tsx; (5) persisted state: only viewMode is whitelisted and nothing reads layoutMode; (6) scope discipline: for each AP step, the diff between its pre and post copies (D16) of every file shared with the Permissions, Exclusions and Local AI Analyzer plans is limited to what that step lists; (7) cognitive complexity <= 12 for every new function. Fix trivial findings in place; report the rest.

**Test first:** `/Users/houssem/Desktop/Github/telark/.claude/plans/applications-cards-coverage.plan.md`

- `review checklist items 1-7`

- red: n/a
- green: no high-severity finding open

**Verify:** Every finding closed or listed under user-owned follow-ups.

## Docs updated in the same change set

- None. No user doc describes the applications list or card (checked: telark docs/, INSTALL.md, charts READMEs, dashboard-ui README.md). No backend, CRD, chart or values change, so INSTALL.md, the chart READMEs, VALUES.md, CRDS.md and the service READMEs stay as they are.

## User-owned follow-ups

- Screenshots (agents never open a browser): /applications in grid with the sidebar expanded and collapsed, list, bulk; the toolbar just above and below 676px to confirm TOOLBAR_COMPACT_WIDTH.DEFAULT; /protection-plans to confirm the plan card is unchanged.
- Product call: keep scheduled and pending_approval plans visible as 'upcoming' (default here) or show active plans only (one-line change in APPLICATION_COVERAGE_BY_PHASE).
- Commit when satisfied (no agent commits).

## Out of scope

- A backend coverage endpoint or plan-list summary view (upgrade path if plan payloads grow: exporter informer + ?view=summary for plans).
- A coverage filter or sort in the applications toolbar (for example 'uncovered only').
- Coverage on the application details page (the Local AI Analyzer is editing that surface).
- A view-mode dropdown on the protection plans list.
- Clickable plan chips that navigate to the plan.
- A UI unit-test runner (none exists; the coverage logic is checked by the node command in AP2).
- Noticed, not changed: the inline 'No applications match the current filters.' / 'Clear all filters' literals in Success.tsx; hex values in PHASE_DOT_COLOR and HEALTH_DOT_COLOR; unused CARD_LAYOUT keys (STRIPE_*, SPINE_WIDTH_PX, REFUSAL_FONT_SIZE_PX, EDGE_TRACK_HEIGHT_PX, MIN_HEIGHT_PX, CHIP_RADIUS_PX, ROW_GAP_PX); getApplicationHealthAccentColor's non-DEFAULT_COLORS warning colour.
- Any version bump, build, push, tag, chart publish or deploy.

## Risks

- Shared files: ProtectionPlanCard.tsx and protectionPlans.ts are also edited by the Permissions and Exclusions plans; applications models/application.ts, models/index.ts and constants/texts.ts by the Local AI Analyzer. Mitigation: the hunk limits in AP1-AP3 and D15, one dashboard-ui step at a time (D15), and diff checks against per-step copies (D16).
- check-all is project-wide, so a gate run while any other dashboard-ui step is mid-edit fails on that step's files. Mitigation: the serialized dashboard-ui lane across APPS, PERM and EXCL (D15), and gates that fix only this plan's files and report the rest.
- Each open applications tab now lists every ProtectionPlan CR once per fetchIntervalSeconds through an uncached exporter handler (process-global mutex, direct apiserver list). The home dashboard already pays the same cost; upgrade path in Out of scope.
- 3 cards per row next to the expanded sidebar was never measured (the plans grid uses the same fixed 3); the header may wrap tags on narrow cards. Screenshot follow-up.
- TOOLBAR_COMPACT_WIDTH.DEFAULT = 676 is derived from one added control, not measured. Screenshot follow-up.
- The coverage section makes cards taller, so fewer fit above the fold in list mode.
- The coverage check lives in this plan, not in the repo: it guards the implementation now, not future regressions.

## Revision log

| Id | Severity | Finding | Resolution |
|---|---|---|---|
| R0 | n/a | Initial plan (single author, terrain-backed; load-bearing claims re-verified in code on 2026-09-23: card anatomy and line anchors, CARD_LAYOUT/ACCENT_TINT consumers, persisted whitelist, toolbar dropdown lacking selectedKeys, Dashboard plan read gate, plans slice state, pagination clamp; coverage util prototyped and the node check run green in a scratch copy). | n/a |
| R1 | critical | AP2 (5), D5, dependsOn: AP2 had dependsOn [] and hard-coded usePermission('protection-plans', 'ReadOnly'), switching to 'an entry for viewing' only if one existed when it applied; viewViolations (permissionEngine.tsx:212-216) already fits that wording, and nothing ordered AP2 after PERM U1, which adds protectionPlans.view that PERM D14 requires for coverage. | Folded in. AP2 crossPlanDependsOn PERM:U1 (the PERM plan's field; dependsOn keeps in-plan ids only). AP2 (5) gates unconditionally on module-level `const { view: viewPlans } = ACTION_PERMISSIONS.protectionPlans;` and usePermission(viewPlans.scope, viewPlans.level, viewPlans.deny), both imported from '../../../auth/hooks' (index.ts exports both). The conditional sentence and the follow-up bullet are deleted; D5, the assumption and the edge row are updated; AP2 verify greps for a bare level gate or viewViolations. PERM U5 (crossPlanDependsOn APPS:AP2) becomes a checked no-op, because its replacement text equals this gate. |
| R2 | high | AP2 (5) known flag, D6: every poll's pending sets loading = true and error = null (protectionPlansSlice.ts:50-53), so with no plans all 12 cards flipped between 'No protection plan covers this application' and '—' every tick (PLAN_LIST_POLL.MIN_SECONDS 5). | Folded in, with a different mechanism from the suggested latched useState. known = plans.length > 0 or plans !== mountedPlans, with `const [mountedPlans] = useState(plans)`. Only a fulfilled fetch (or a plan action) replaces state.plans (:54-57, :69-113), so polls never flip it and the cold-visit 'none' frame is gone as well. A flag latched on 'plans.length > 0 or (!loading and !error)' latches on the first render (initialState :16-27), so it would claim 'none' for the whole first fetch. Setting it in an effect fails react-hooks/set-state-in-effect: both variants were linted in memory against eslint.config.mjs. D6, the cold-visit and fetch-fails rows, a new 'empty plan list while polling' row and R1 item 3 are updated. |
| R3 | high | AP2 feeding AP3: the gate ignored PERM's view key unless AP2 happened to run after U1. A role with Owner/ALL plus viewprotectionplans.deny would poll into 403s (logErrorOnce) and render chips from the persisted list (persistConfig.ts:42-46). | Folded in through R1: crossPlanDependsOn PERM:U1 plus the unconditional view-key gate. D5, the 'Viewer can read applications but not plans' row (now naming the deny-rule role), R1 item 3 and the follow-ups are updated. The optional PERM R1 addition already exists in PERM R1 item 12, which is PERM-owned. |
| R4 | high | AP2, AP3, V1 vs PERM U1-U4 and EXCL U1-U2: check-all is project-wide, and AP2/AP3 share no file with those steps, so file-level serialization let an AP gate run inside another plan's red window. V1's 'fix every error' could also edit another plan's in-flight files. | Folded in. D15: every dashboard-ui step of APPS, PERM and EXCL runs in one serialized lane (lane key: repo dashboard-ui). The AP1-AP3 verify steps and the P0/P2 acceptance criteria report errors outside the step's files to the orchestrator. V1 gets crossPlanDependsOn PERM:U2, PERM:U3, PERM:U4, PERM:U5 and EXCL:U2 (U5 because it edits an APPS file); it fixes only AP1-AP3 files and hunks and reports the rest. The Approach, the P0 title, the step-order heading and the risks are updated. |
| R5 | high | AP1 verify, R1 item 1, AP3 verify: the git-diff checks ran against the fully staged baseline. That diff also shows PERM U2's usePermission/menuItems hunks in ProtectionPlanCard.tsx and the Local AI Analyzer's hunks in the models and texts.ts. D15's 'AP1 first' was a recommendation only. | Folded in. New D16: each AP step saves pre and post copies of its existing files to the run scratchpad. AP1-AP3 verify diff against the pre copies ('this step's hunks are limited to X; other hunks untouched'), and R1 items 1 and 6 diff pre against post. The PERM U2 to APPS:AP1 edge would be a PERM-file edit, outside this plan's edit scope. D15 now makes the AP1/PERM U2 order free, since both re-read anchors and D16 judges only each step's own hunks. If a fixed order is wanted, D15 names that edge as PERM-owned. |
