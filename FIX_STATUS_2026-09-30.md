# Fix status 2026-09-30 (FIX_BRIEF_2026-09-27 → now)

## Repo state
- Nothing committed or pushed except rest `e9d3380` (ProtectionPlanDrift fields): pushed, **not tagged**; services still pin rest `v0.15.0`.
- Staged, uncommitted: telark 192 files (branch `fix/fix-critical-security-issues`), dashboard-ui 111, landing-page 16, rest 1 (notifications `Delete`). infra: 1 unstaged (vpc-cni NetworkPolicy).
- Nothing deployed since 13:08 (cluster runs the pre-fix images).

## Fixed
### Exporter
- E1 non-admins could strip Admin from admins (role/group PATCH/DELETE capped, auth delete path included)
- E2 group PATCH without `userRefs` emptied the group
- E3 user PATCH without `groupRefs` dropped memberships
- E4 audit actors taken from the request body
- E5 / E5-boot group-side membership bypassed admin visibility, protection and bootstrap
- E6 rollbacks never ran; force-sync progress never stored
- E7 any user read the whole TelarkConfig (OIDC trust)
- E8 snapshot-manifest deny rule not enforced
- E9 oversized PATCH body → 413
- E10 role notification always said "revoked"
- E11 finalizer routes → Internal only
- E12 deleted user left Passkey CRs
- E13 passkey PATCH/DELETE unknown id → 404
- E14 hidden-admin existence oracle on PATCH users
- E15 role create without `type` → 422 raw text; PATCH blanked `type`
- E16 config PATCH unknown key → error (config half)
- E17 config schema 400 named `<nil>`
- E18 mark-read of unknown/foreign notification → 404
- E19 YAML manifest download named `.json`
- E21 `roleRefs:[null]` accepted
- D4 role protection locks (lockName, lockCategory, softDelete) enforced; creator/Admin-on-ALL only
- D5 list 503s under load: shared renders, bounded concurrency (`EXPORTER_LIST_RENDER_CONCURRENCY`)
- Notifications: per-row `DELETE notifications/{id}`; WATCH-based updates fix two proven races (double unread decrement; re-emit reviving a deleted item)

### Auth
- D1 bootstrap admin is passkey-only (Google refused, 403)
- A1 OIDC trust set could not be cleared
- A2 passkey PATCH/DELETE flattened to 500
- A3 Google login onto an account that signs in another way → 409
- A4 unbounded pre-auth request bodies
- A5 deleted passkey kept its identity
- A6 invalid cleanup env silently became 0
- A7 self-registration created the account at start
- A8 dead error mapping; A9 self-registration defaulted to on
- D4-c deleter stamped on cleanup

### Discovery
- D1 app down when first seen got no incident
- D2 park dropped the run's violation history; C-32 non-Owner duplicate relaxed approval mode
- D3 withdrawn policies lost recent denials
- D4 plan health trusted the annotation, not the policy body
- D5 invalid scope touched the cluster, then 500
- D6 plan referencing a vanished app could not be edited
- D7 insights environment/tags taken from the wrong namespace
- D8 `Published` condition never on the CR
- D9 status drift: `mismatched`/`stale`/`added` (needs rest release)
- D10 self-monitoring: release namespace hidden unless `app.selfMonitoring.enabled`
- D11 delete+recreate racing a flush lost its change entry
- D12 rollback notification `applicationId`; D13 `restoredGeneration` never written; D14 stale sweep sent no notification; D15 stale comment

### Analyzer
- N1 a change made after an incident began was reported as its cause (+ D1-N1)

### Chart
- C1 CRD write guard covers every namespace
- C2 uninstall leaves no Kyverno webhooks / finalizers (pre-delete hooks)
- C3 ui pod no longer gets the service token
- C4 dead nats securityContext; C5 duplicated metrics-server args
- C7 NATS/Ollama NetworkPolicy namespace
- `.dockerignore` for every service and the UI; `helm test` removed

### Dashboard UI
- U1 single mark-read never worked
- U2 error-boundary text invisible
- U3 roles without all 7 scope levels uneditable
- U4 "Back to login" invisible in light theme
- U5 audit fields no longer sent by the client
- U6 Insights claimed "All clear" with auto-analysis off
- U7 GET config fired without settings access; cluster card no-access state
- U8 missing CURRENT_USER purged and reloaded every load
- U9 lists flashed "No users yet" while loading
- U10 / U11 roles fetched without permission (403s, "No roles available")
- U12 compare said "identical" when only a Secret changed
- U13 / U14 / U15 contrast failures (impact note, notification text, sidebar labels)
- U16 register form flash; U17 dark auth CTA; U18 enroll link lost on reload (and dead token kept after a failed enrollment)
- U19 SSO form enabled without Admin on ALL
- U20 nginx `/healthz` headers
- U21 "No access" coverage chip; U22 non-Telark actors looked up as users
- IAM-15 admin-lock tooltip removed; role search on create user; toolbar wrap; avatar shrink
- D3 zero-role users no longer blocked app-wide; D8 one primary CTA style (green, dark text, AA)
- Notifications: per-row mark-read/delete, icon toolbar; bell no longer shifts on panel open/close
- F13 notifications infinite loading with 2+ tabs (cross-tab storage ping-pong)
- SSO JWK field no longer shows the stale stored set; placeholder + Google JWKS link
- Confirm dialogs unified on `ActionConfirmModal` (role, passkey, rollback abort, revoke session)
- Change log unreadable at 375 px

### Docs, rules
- T1–T5, C6, C8, D6, D9, Q3 docs drift; upgrade sections removed; env/API/helm references updated
- F1 test seams: `CoalesceForTest`, `SetOwnNamespaceForTest` removed (`POD_NAMESPACE` Downward API); AGENTS.md forbids `*ForTest` seams
- F4 drift-comparison wording; F7 zero constant; F3 gofmt

## Live retest (13:08 build, before the afternoon fixes)
- PASS: chart 23, auth 10, apps 20, exporter 10, IAM 24, plans 22, UI 25; `rbac_matrix.py` 0 mismatches.
- FAIL then fixed in code, not yet retested: D5, D11, D4-c, IAM-15, N1, D1-N1, U22.

## Remaining (this round)
- ~~Actor names~~ code done 2026-09-30 (Task 2): exporter `GET users/names?ids=` + UI `useUsernamesByIds`/`ActorDisplay` at every actor field; gates green; not deployed or live-tested
- ~~Modals: content modals (session expired, enroll link, orphaned passkeys, avatar picker) on the shared chrome; premium restyle within the theme~~ done 2026-10-01 (every dialog on BaseModal; Task 6: migrated dialogs PASS)
- ~~Notification rows restyle~~ done 2026-10-01 (shipped with the dialog polish)
- ~~UI items: group tooltip avatar initials, Manage Roles at 900×900, rollback meta ellipsis; F12 long app title overflows the page~~ done 2026-10-01 (Task 5 items 1–4; long page titles wrap since the actor-names change)
- ~~Visual check: D8 CTAs, dialogs, bell, SSO JWK, change log~~ done 2026-10-01 (Task 6; the bell fix in "Done 2026-10-01 afternoon")
- ~~Rebuild + redeploy; retest FAIL/BLOCKED rows, regressions, `rbac_matrix.py`~~ done 2026-10-01 (Task 8 retest: 29 PASS, 0 FAIL)
- ~~Clean up test data (e2e-* namespaces, personas, sessions); discovery replicas back to 1~~ done (every lane deleted its data; no e2e namespaces and discovery at 1 replica, checked 2026-10-02)
- ~~Commit messages per repo; remove `go.work` replaces first~~ done (no `go.work` since the monorepo merge; messages written each round)

## User actions
- ~~rest: tag a release with `e9d3380` + the staged `Delete`, then bump the rest pin~~ obsolete: rest is `telark/internal/rest` since the monorepo merge
- Live checks needing you: D2a/D2c, A1 + D1 OIDC callback, U2/U18 screenshots, D10 fixture, IAM-14b/E5-boot, U12/U19 fixtures

## Remaining, part 2 (former follow-ups, this round too)
- ~~Remove the pre-existing seams `UseCacheForTest`, `SetExcludedForTest`; review test-only exports (`PublishedPayload`, `HiddenPlatformApp`, `FailStaleInProgress`, `FinalizeRollbackSuccess`)~~ done (seams gone; the two rollback-controller exports stay with their one-line why, as AGENTS.md allows)
- ~~D11: recreate later than one window or across leader failover~~ done 2026-10-01 (`coalesce:held` for 5 min; Task 8 PASS)
- ~~Role PATCH/DELETE last-admin guard~~ done 2026-10-01 (409 when no active Admin on ALL would remain)
- ~~E16 finalizer-body half; E9 sibling error paths; CORS `Retry-After` (x-ware)~~ done 2026-10-01 (413 on every write route; CORS exposes `Retry-After`)
- ~~Passkey onboarding (decided, next after this round; plan as a feature):~~ done 2026-10-02 (see "Done 2026-10-02")
  1. ~~Invites: admin creates the user, then Members → "Create enroll link": one-time URL shown once with copy button and expiry (~24 h), single-use, bound to the user, re-issuable; "Invite pending / expired" in Members. Owner on users; Admin targets need Admin on ALL. No SMTP in MVP.~~
  2. ~~Self-registration moves from the chart (`app.auth.passkey.selfRegistration`, `SELF_REGISTRATION_ENABLED`) to a TelarkConfig field (CRD first), default off, toggle in Settings next to Single Sign-On, read live by auth; chart guard becomes "bootstrap admin required"; docs updated.~~
  3. ~~Bootstrap-only control: SSO config and the self-registration toggle editable only by the bootstrap user (backend + UI, today Admin on ALL); others see them read-only with a tooltip.~~
- ~~Monorepo: shared modules into `telark/internal/`~~ DONE 2026-09-30 (history merges + uncommitted move on `chore/import-module-history`; merge its PR with a merge commit)
- ~~data `internal/data/shared/config.go` dead map~~ done 2026-10-02 (no map was left; its 18 unused constants and the `Status` and `Action` types they used are removed); ~~gofmt `app_namespaces_test.go`~~ done in the monorepo gofmt pass

## Done 2026-09-30 evening (details: session-data e2e-2026-09-25/NEXT_SESSION_2026-09-30g.md)
- Task 2b bootstrap profile edit 403 (guard + UI changed-fields-only); bootstrap email locked to the chart value; chart rev 4 `app.auth.bootstrap.admin=contact@telark.io`
- UI: login stuck loading + no-passkey redirect; dashboard/feature no-access redesign; Members "No roles"; locked-input tooltip; profile real-time uniqueness; settings icon/badge/order
- Headless "Couldn't load this content" on no-access pages: not a UI bug, the ui-norole test session had been deleted (401); re-minted, pages render the no-access state
- Settings no-access position: gate moved to `SettingsPage` around `SettingsLayout`, so SSO / Insights / Governance lock states sit where the feature pages put theirs (per-section checks removed; SSO keeps its level-only check)
- Plan details Violations/Reports no-access: box-style lock state kept (user approved)
- Login "I lost my passkey": no more jump to /register (which refuses existing accounts); shows "Contact your administrator to restore access to this account." inline; dead browser orphan-cleanup removed (auth answers 401 to it by design)
- Login unknown email with self-registration off: "No account for this email. Check the address, or contact your administrator." (was "Register first…")
- Role pickers (Add/Edit member roles, group Manage Roles): new lock state ("No access / Requires ReadOnly on roles") in light-panel colors
- All of the above: UI gate green, live-checked in the user's Chrome (22:30)
- Group ↔ member consistency retest (22:40, API as admin, CRD+API+list count after every step): create user with group, create group with users, add/remove from either side, multi-group, multi-user, PATCH without refs, role inheritance via group: all PASS; cluster scan 0 one-sided memberships. Open: deleting a user/group leaves the other side's ref for ~45–60 s until auth's cleanup sweep → fix queued (exporter strips refs on DELETE); restart scenario pending
- Suspended account login (user item, 23:05): auth answers 403 "user account is suspended" only after the passkey/Google credential is verified (login start unchanged, no enumeration); login card shows "Your Telark account is suspended. Contact your administrator." inline (passkey and Google, no toast, no lost-passkey popup). Live: real UI passkey login suspended → message, activate → immediate login (2 cycles); Google path confirmed live by the user (refused with the message, then immediate login after reactivation)
- Suspend → activate stale state (user item): root cause = every login's last-login write re-sent the phase it had read (stale by-email cache, 15 s) and the exporter defaulted a missing phase to active, so a login could re-suspend/re-activate an account; fixed (stamp sends only lastLoginAt, exporter keeps the stored phase, fresh by-email reads, stamp only after the session exists). Live: 6 API cycles 104/104 + 2 UI cycles
- Bootstrap Google login (user item): restriction unchanged; 403 "the bootstrap administrator signs in with a passkey" → login card: "Google sign-in isn't available for the bootstrap administrator. Sign in with its passkey." Unit-tested; needs the user's live Google check
- My Permissions (user items): bootstrap account shows "Full access to Telark" (display only); inherited roles show "Inherited from <group name>" (auth sends `groupName`), no group IDs in text or tooltip. Live: admin-grp shows "Inherited from e2e-r-admin-grp", 0 group IDs in the page
- Deployed: auth + exporter (pinned 0.0.1); UI via Vite. Gates: go test -race (2347), golangci-lint 0 issues, UI check-all-and-build 0 errors
- Bootstrap Google login, live (23:18): confirmed by the user, Google sign-in with the bootstrap mailbox is refused with "Google sign-in isn't available for the bootstrap administrator. Sign in with its passkey." Test setup on the cluster only: bootstrap mailbox switched to the user's Google address (chart rev 5 `app.auth.bootstrap.admin`, bootstrap account email patched to match, the old Google JIT test account deleted); repo values unchanged (contact@telark.io)
- Group ↔ member delete window (user item, fixed): deleting a user now removes it from its groups' member lists at once, and deleting a group removes it from its members at once (the exporter's delete strips the other side right after the delete, outside its own lock; auth's cleanup sweep stays the backstop). Regression test `TestDeleteStripsTheOtherSide` (fails on the old handlers). Live: memb.py 16/16 PASS incl. both deletes checked 1.5 s later; exporter restart → boot reconcile aligned 0/0, 0 one-sided memberships, API = CRD for 125 users / 15 groups; Groups page "2 Members" → member deleted through the UI's auth route → "1 Member" 1.6 s later; group deleted through auth → member's groupRefs empty 0.06 s later; sweep still drops the finalizers (~65 s); exporter logs clean. Gates: go test -race (2351), golangci-lint 0 issues. Timing note: the 45–60 s window came from deletes sent straight to the exporter API, which only auth's 60 s sweeper caught; the UI's auth route already queued the cleanup within seconds, and both paths are now immediate for membership
- Role delete timing in the UI (user item): the single and bulk role-delete modals now say "They lose the access it grants immediately. The role disappears from them within a few seconds." (bulk: same for "these roles"). Measured through the UI's auth route: refs gone from the user and group at +0.2 s, role gone from the list at +1.3 s; grants skip a deleted role at once. The role texts' plurals use a new `pluralize` helper (`src/utils/helpers/format.ts`). Live: both modals checked headless as admin; UI gate green

## Done 2026-10-01 night (dashboard-ui, UI gate: check-all-and-build 0 errors)
- Task 3 modals: every dialog now uses one chrome, `BaseModal` (ActionConfirmModal builds on it): SessionExpired, EnrollLink, OrphanedPasskeys and AvatarPicker migrated from their own antd Modals. Chrome per the user's live calls: compact (360 px, 16/20 padding), title and text left-aligned, no icon, X in the top-right corner, footer right-aligned with the muted Cancel and a solid primary (green) or danger button, warning consequences in a note callout, blurred backdrop, light-surface theme set by the modal itself (a disabled button no longer vanishes, modals look the same from pages, panels and auth). Names in modal text are bold without quotes (role and plan deletes; plan delete's report warning moved to the note); the app reset modal names the application. Removed: the old close icon, actionConfirmModal.css and the global modal CSS overrides (one of them zeroed the bottom padding). Live (headless): role delete single and bulk, app reset, avatar picker (disabled Update visible), session expired. Not live-checked: EnrollLink, OrphanedPasskeys (user check)
- Role delete timing note: plurals via `pluralize`, no semicolons in UI text (user rule)
- Task 4 notification rows (sub-agent, reviewed): tinted type badge, unread dot, one-line title and 2-line message with full text on hover, hover background, timestamp bottom-right under the row icons (user call). Per-row mark-read/delete unchanged. Live: mocked list (0/1/7 rows) and real exporter data
- Task 5 UI items (sub-agent, reviewed): (1) group View panel "+N" tooltip now shows member initials and readable usernames (shared ViewPanelHeader, the only overflow tooltip); (2) Manage Roles double scrollbar at 900x900 fixed at the root: the inner list scroller is gone from all four lists (user and group Manage Roles, Manage Groups, Manage Members and their Assigned views), the panel body is the one scroller, the scroll chevron and its dead code removed (user approved); (3) Manage Rollbacks rows: metadata on one line with ellipsis and the full text on hover, long namespaces no longer widen the panel; (4) change log at 375 px checked, no change needed. Live: before/after screenshots at 1440, 900 and 375
- Pills (user item): pills use the declared colors (SUCCESS, DANGER, WARNING, INFO_STRONG for info, NEUTRAL, TEXT_MUTED) with white text (user decision; white on SUCCESS 2.1:1 and WARNING 1.9:1 is below AA, accepted); `getPillSurface` returns background and text color, callers no longer set the color
- Softer DANGER `#E05252` (was `#FF4D4F`) and WARNING `#E2A336` (was `#faad14`), tints updated; antd danger and warning (danger buttons, form errors) now use them too via the root theme (user item). Live: modal Delete renders the new red

## Done 2026-10-01 (Task 6 visual checks, headless on the Vite server, no code change)
- D8 primary CTAs: PASS. Login "Authenticate" and enroll "Register Passkey" in light and dark, roles toolbar "Add Role", Create Role panel submit, session-expired "Go to Login": green `#20C997`, dark text, weight 600, no shadow, 8.3:1 contrast. Danger confirms (Delete, Revoke, Reset) use `#E05252` with white text (3.8:1, user's softer red)
- Migrated dialogs: PASS. Role delete, app reset (note callout), revoke session, session expired: 360 px, left-aligned, bold names without quotes, X top-right, muted Cancel with a solid button on the right; avatar picker 460 px (avatar grid), disabled Update visible. EnrollLink and OrphanedPasskeys: user check in Chrome
- SSO JWK field: PASS. Empty with placeholder, Google JWKS link and "Leave it empty to keep the pinned keys" (switch flipped client-side only, nothing saved; cluster `egressAllowed` still true)
- Notifications with 2 tabs (F13): PASS. Both tabs load 4 rows, no spinner; mark-read in one tab updates both bells within 15 s; 2 GETs per tab at load, 1 POST after the mark, no cross-tab ping-pong
- Bell: FAIL (not fixed, reported). With always-visible scrollbars it moves 15 px right while any slide-out panel is open (notifications, Create Role) and snaps back on close: the scroll lock measures the gutter as `innerWidth - clientWidth`, which is 0 while `scrollbar-gutter: stable` reserves 15 px, so the header widens. Overlay scrollbars (macOS default) don't show it
- Found, not fixed: settings Save buttons (SSO, Insights, Governance, Timezone) use the default variant, not the D8 primary; the SSO hint lacks a period between the link and "Leave it empty…"; bell aria-label says "1 unread notifications"; an add then remove to a group 3 s apart delivered only "Removed from group" (same family as the 3-of-4 drop)
- Service logs: 0 error lines in 50 min across the 18 telark pods

## Done 2026-10-01 afternoon (telark + dashboard-ui, deployed 13:01: exporter, auth, discovery; UI on the Vite dev server)
- Test seams (Task 7, T1): `UseCacheForTest`, `SetExcludedForTest`, kcore `ResetAllClients` and exporter `authz.UseGrantSource` removed; `PublishedPayload`, `HiddenPlatformApp` and rollback `StaleSweepErrorMsg` unexported. Tests now go through production paths (CRs in a fake apiserver read by the real CRD source, real informers, embedded JetStream, `FailStaleInProgress` for the stale sweep). Still exported, each with a one-line why in the code because only an in-cluster client reaches them: `SetDynamicClient`, rollback `RunWorkers`, `RecordWithRetry`, `ReplaceUnstructured`, `FailStaleInProgress`, `FinalizeRollbackSuccess`
- D11 lost pre-image (Task 7): when a flush finds no workloads (all deleted), its pre-images are kept in Redis `coalesce:held:<app>` for 5 min (user decision, was 24 h) and taken once by the app's next event, on this leader or the next; force sync and app reset clear them. Live E-17 (delete, then recreate with a new image after 0 s, 10 s, 60 s): 3/3 apps at generation 2 with one "Image updated pause:3.9 → pause:3.10" each; hold TTL 300 s right after a delete, gone once the app was recreated
- T10 E-17 "drift flush recorded N -> N": not seen live after the deploy (above), closed. Root cause on record: images, ports, env keys, ConfigMap/Secret refs, service mappings and ingress rules are compared only with the stored CR, and a snapshot-scheduled flush seeds only replicas from its pre-image; the known triggers are fixed (an undone image change followed by a no-change publish; the lost pre-image, now held). If it ever shows again: seed those fields from the flush's pre-image objects
- Role last-admin guard (Task 7): a role edit (scopes, status, validity) or delete, soft delete included, that would leave no active Admin on ALL answers 409. T6: the level cap and this guard judge one merged role whose scopes, status and validity are exactly what the PATCH stores (an emptied scope list, an omitted validity). Live: a custom Admin-on-ALL role was created, deactivated and deleted (200, 200, 202), with no false 409 while admins remain
- Oversized and malformed bodies (Task 7, T5, T8): finalizer bodies, snapshot create, passkey create/patch/delete, session create/patch, notification emit and the discovery rollback trigger answer 413 above the cap. Live: rollback trigger 413 (was 422, no rollback recorded), passkey DELETE 413 (was 404)
- Expected refusals no longer logged as errors (B4, T5): guard 403s, unknown references, role protection, case-variant keys and malformed or oversized bodies answer 4xx without an `[ERROR]` line, while 5xx are still logged. Live: 400, 403, 400, 422, 413 with 0 exporter `[ERROR]` lines (old build: 5)
- CORS (Task 7, T4): the browser can now read `Retry-After` and `ETag` cross-origin, and `If-None-Match` passes the preflight, so the UI can wait out a shed request and revalidate insights. Live on 8002, 8004 and 8006 (old build: refused, nothing exposed)
- Notifications (B1, B2): "Added to group" then "Removed from group" 3 s apart now both arrive (only identical re-published events merge); role changes read "Granted 1 role. Revoked 2 roles.". Live PASS
- Auth challenge cleanup (B3): the extra `CleanupChallenge` call is gone; a retried passkey finish keeps its challenge
- Small fixes: Kubernetes client errors now carry their cause (T3); dead `ManagedFields` and `ExcludedNamespaces` lists removed (T2); a 30 s discovery test now runs in 0 s (T7); Redis key table in `docs/architecture` lists the 7 informer keys with their TTLs (T9)
- CI coverage (user request): every Go test leg uploads its profile to Codecov, with flags for the 4 shared packages and new floors data 73, rest 54, kcore 22, x-ware 47 (measured 74.5, 55.4, 23.7, 48.4)
- UI monospace (U1): fingerprints, plan template and policy names, the passkey popover and both error boundaries render in Geist Mono (they fell back to Geist). Live PASS
- UI texts (U2, U3, U4): remaining inline strings moved to constants with the copy unchanged; email placeholders "e.g. test@example.com"; an unused constant deleted. Live PASS
- Users page without groups access (U5): no more 2 failed group requests per load; the Groups page shows its no-access card. Live: 0 group requests for users-c/users-o, admin unchanged
- Manage Roles without roles access (U6): only the no-access card shows (no search, toggles or filter) and "Update roles" stays disabled, because submitting would have removed every role the user has. Live PASS for users-o and groups-o, admin unchanged
- Task 6 findings, fixed in the reviewed UI changes: the bell no longer moves under open panels, settings Save buttons use the green primary, the SSO hint has its period, the bell says "6 unread notifications", interval options read "1 minute / 2 minutes", and settings-only personas get no 403 on Governance. Live PASS (`t6-fixcheck`, `t6-bell`, `t6-gov403`, `t6-snapinfo`)
- Gates: Go build and vet clean, `go test -race` green on all 8 legs (coverage auth 70.3, discovery 60.4, exporter 69.3, notifier 78.1, all above their floors), golangci-lint v2.14.0 0 issues on the 7 touched packages; UI `check-all-and-build` 0 errors
- Service logs 13:01–13:16: 0 error lines in 18 pods, except one discovery `[ERROR]` for the deliberately oversized rollback body (follow-up FT7)

## Task 8 retest 2026-10-01 (4 Opus lanes on the 13:01 build)
- 29 PASS, 0 FAIL. Apps 5: D10 auto-clean of a pre-existing CR, D11 (the E-17 run above), REG-apps-view, REG-apps-patch, REG-apps-internal. Plans 6: D5 no-503 retry (API side), D1-N1, N1 L-6, F8-c, R3-1. IAM 11: D4-c, IAM-15 bulk delete, REG-roles, REG-memb, REG-vis, REG-prot-boot, REG-prot-self, REG-prot-adm, REG-lastadmin (no false 409), and the full permission matrix (115 personas × 144 routes, 16,704 checks, 0 mismatches, 0 server errors). UI 7: U12, U19 (UI half), U22, D5 (UI retry), REG-hide, REG-name, REG-color
- BLOCKED 1: REG-lastadmin's 409 side can't be reached live without risking a lockout (real admins hold the built-in Admin role); covered by `TestRoleEditsKeepAnAdministrator` and `TestLastAdminGuard`
- NEEDS-USER 1: U19's bootstrap-only side belongs to the approved passkey onboarding plan (next session)
- Every lane deleted the test data it created

## Done 2026-10-02 (passkey onboarding; deployed: exporter + auth 0.0.1, UI 0.0.2, chart rev 9, the two edited CRDs applied with kubectl)
- Members → "Create enroll link" gives a one-time link, shown once with copy and expiry (1 h, `ENROLL_INVITE_TTL_SEC`). A new link kills the old one, "Revoke enroll link" kills it, and the row shows "Invite pending" / "Invite expired". Live: issue 201; replaced, used, revoked and expired links (TTL 60 test) all get one identical 401
- Who may create a link: Owner on users, capped like role assignment (403 when the member holds more, directly or through a group); own account 403, bootstrap 403 (404 below Admin on ALL, like any hidden admin), suspended 409, being deleted 410. Live: every refusal as expected; the UI disables the row button with the reason
- Recovery links (member already has a passkey): bootstrap or Admin on ALL only, and the member gets a bell notice on creation and on use. Live: Owner on users 403, Admin on ALL 201, both notices within 1 s
- Tokens are stored only as SHA-256 digests (Redis dump: 0 raw tokens) and never logged (0 hits in auth and exporter logs); the register page strips the token from the address bar and posts it in the body; `/register` answers `Referrer-Policy: no-referrer` and the UI access log never records the query
- Self-registration moved from the chart to Settings (TelarkConfig `selfRegistration.enabled`, off by default), read live by auth: a flip shows within 4.4 s, no restart. `app.auth.passkey.selfRegistration` and `SELF_REGISTRATION_ENABLED` are gone; the chart and auth require `app.auth.bootstrap.admin`
- SSO and self-registration are bootstrap-only: auth answers 403 to every other account (Admin on ALL included), the exporter refuses `oidc` / `selfRegistration` writes from any session, and Settings shows both read-only with a tooltip. Live and headless: as expected
- Settings → Single Sign-On shows the Google client ID as `***` to anyone who can't edit it (everyone but the bootstrap account); an empty ID stays empty (user request 2026-10-02; checked by the user in Chrome)
- Google sign-in refused with 409 (the email belongs to an account that signs in another way, e.g. a passkey from an enroll link, or several accounts share it) now explains why in the login card instead of "Google login failed" (user report 2026-10-02; headless check with the 409 faked: both messages shown, no generic toast; checked by the user in Chrome)
- Fixed during the round: revoking a link while its member is being deleted answered 500 (now 200); a link whose member was deleted answered 404 instead of the shared 401 (now identical); suspending a member with a pending link would have failed (the panel now sends only the phase); a disabled menu item's tooltip covered "Create enroll link"
- Gates: Go build, vet, test -race (124 packages), golangci-lint 2.14.0 0 issues (auth, exporter, data, rest), helm lint, render without the bootstrap admin fails as intended, VALUES.md no drift; UI check-all-and-build 0 errors; landing-page check-all + build. Logs after the fixes: 0 `[ERROR]` / `[WARNING]` in auth and exporter
- Needs the user: the bootstrap-side Chrome check (edit SSO, flip self-registration); the telark-crds release and re-pin (until then every `helm upgrade` on telark-dev reverts the two CRDs: run it with `--force-conflicts`, then re-apply the two CRDs)

### Cleanup 2026-10-02b (no behavior change)
- Test comments cut to the two-line why rule (`TestGuardEnrollLink`, the self-registration test, the bootstrap config test); the header comments on the `enrollDirectory` and `inviteDirectory` fixtures removed; the `FakeExporter` comment now states only its usage rule
- auth constants: the TelarkConfig read warning moved to "Configuration Errors", and the messages group is titled "Enrollment Link and Self-Registration Messages", since it holds both
- American spelling everywhere (user rule): about 260 text fixes in docs, UI copy, comments, messages and CRD descriptions across telark, dashboard-ui and landing-page ("enrolment" → "enrollment", Members "Enrol link" → "Enroll link", "cancelled" → "canceled"…); identifiers renamed too: the auth package `handlers/authorisation` → `handlers/authorization`, discovery `filterCanceledScalarChanges` and `InfoHistoryReplicaChangeCanceled`, 13 Go and Python test names, the UI `canceled`/`canceling` variables and 5 constants. The three `AGENTS.md` files now state the rule
- UI: prettier rewrapped the client-ID line in `OIDCSection.tsx` (the build's only warning)
- Checked, no change needed: `FakeExporter` has no unused member; `telarkConfigLg` stays (the package's `lg` logs under the cleanup prefix); `EnrollLinkAction` and `useUserEnrollLink` stay under cognitive complexity 12; no dead anchors and no unused exports; the Lua script comments document the KEYS/ARGV contract
- Consistency: the chart rendered with the live values sets the same env names as the 15 running containers, and the 9 CRDs are structurally identical to the live ones (only the protectionplans `terminatedAt` description now says "canceled"; it goes live with the telark-crds release)
- Gates: Go build, vet, test -race (124 packages), golangci-lint 2.14.0 0 issues (4 services, 4 shared packages); analyzer 770 tests, coverage 99 %; helm lint, kubeconform 0 invalid, VALUES.md no drift; UI check-all-and-build 0 errors, 0 warnings; landing-page check-all + build
- Deployed at 19:00: auth, discovery, UI. Live without a session: `auth/config` 200, a bogus enroll token 401, `enroll-link` and `auth/permissions` 401; 0 `[ERROR]` / `[WARNING]` in auth, discovery, notifier, exporter (both replicas) and UI since the rollout
- Live with a session (a throwaway Admin and target, both deleted after): `auth/permissions` 200; issue 201, a second link 201 and the first one then 401; revoke 200 with `status.invite` cleared. Headless: Members shows "Invite pending" and the "Create enroll link" / "Revoke enroll link" menu; Settings → Single Sign-On is read-only with the bootstrap-only tooltip; `/register?enroll=bogus` shows the invalid-link message; no console errors besides that expected 401; 0 `[ERROR]` / `[WARNING]` in every pod

### Done 2026-10-02c (Redis password, FT16, FU13, FD2, the "Enrolled" pill, the link email prefill; deployed: chart rev 10, analyzer, auth and exporter 0.0.1, UI 0.0.2)
- Redis requires a password: the chart generates `telark-redis-secret` (key `redis-password`, kept on uninstall, read back on upgrade; `redis.auth.existingSecret` names your own for GitOps), and every Telark pod that uses Redis gets it as `REDIS_PASSWORD` through a required Secret reference. The render refuses `redis.auth.enabled=false`, an empty `existingSecret` and any `redis.auth.password`. Live: no password gets NOAUTH, a wrong one WRONGPASS, the pod's own PONG; the 6 client pods carry the reference, ui none
- CONFIG and ACL are disabled next to FLUSHDB and FLUSHALL, so no client can turn the password off at runtime. Live: all four answer "unknown command"
- The analyzer sends `REDIS_PASSWORD` (the Go services already read it). Live: authenticated connections from analyzer, auth, discovery, both exporters and notifier, with the stream consumers active
- No data lost through the Redis restart (AOF copied to scratch first, copy deleted after the checks): db0 443 keys (444 before, volatile churn), db1 3, every persistent key-prefix count identical, `analyzer:index` 15, `insights:jobs` lag 0; the Redis NetworkPolicy is unchanged
- Switch window 23:15–23:16 UTC: auth restarted once (Redis not ready within its 30 s, as planned), then every pod Ready, `status/ready` 200 on 8002, 8004, 8006 and 8007, and 0 NOAUTH, WRONGPASS or `[ERROR]` lines in any pod since; a no-session bogus `register/start` still answers 401
- Docs: SECURITY.md, CLAUDE.md, the architecture and security READMEs, INSTALL (subcharts row, GitOps commands, version note, uninstall), chart README, VALUES.md, analyzer README and `.env.example`; landing-page installing, upgrading, troubleshooting (authenticated `redis-cli`, render-error row), multi-replica discovery, uninstalling, helm-values and environment-variables. The three exporter test comments now say "untrusted"
- FT16: an invalid `ENROLL_INVITE_TTL_SEC` now warns "ENROLL_INVITE_TTL_SEC must be a positive integer, using the default 3600" under the `auth-service` logger, without the "[cleanup]" tag; behavior unchanged (positive values only, default 3600, `TestEnrollInviteTTL` green)
- FU13: the toolbar key `'save'` (six sections) and the hint font size (OIDC twice, self-registration, the discovery namespaces preview) now come from `SETTINGS_CONSTANTS` (`TOOLBAR.SAVE_KEY`, `CONTENT.HINT_FONT_SIZE`); the other 12px texts in Settings are table headers and row labels, not hints, and stay
- FD2: the `build-service.yaml` comment says "canceled" (`cancelled()` is GitHub's function and stays); action audit: all 18 external actions already pin their latest release by SHA. `CHANGELOG.md` keeps its British words on purpose: git-cliff generates it from old commit subjects
- Gates: Go build, vet, test -race (124 packages), golangci-lint 2.14.0 0 issues (auth, exporter), auth coverage 72.2 %; analyzer 770 tests, coverage 99 %; helm lint, kubeconform 0 invalid (3 modes × 5 Kubernetes versions), one more document per mode, the three refused renders fail, VALUES.md regenerated; UI check-all-and-build 0 errors, 0 warnings; landing-page check-all + build
- Live with a session (a throwaway Admin and target, both deleted after): link issued 201 and revoked 200 (Redis key and `status.invite` set, then cleared), notifications and `sessions/self` 200. Headless on the deployed UI: the 7 settings Save buttons look and behave as before (30 px, 13 px text; disabled until a draft change; Timezone saved and kept after reload; SSO and self-registration read-only with the bootstrap-only tooltip), the reachable hints render at 12 px, 0 failed calls, 0 console errors. Pod scan: 0 NOAUTH/WRONGPASS; one `[ERROR]` from a Google sign-in refused for the bootstrap account (FT15), not from the test
- Members shows a green "Enrolled" pill once a passkey closes a member's enrollment link (user request 2026-10-02): auth now records `status.inviteAcceptedAt` in the same write that clears the invite; a later link shows "Invite pending" again, and revoking it brings "Enrolled" back; the users CRD declares the field and the exporter refuses it from sessions like `status.invite`. Deployed: the users CRD applied with kubectl (goes out with the telark-crds release), exporter and auth 0.0.1, UI 0.0.2. Live (throwaway accounts, deleted after): link 201, passkey through the link 201, the stamp within 1 s, the pill and its "Enrolled <time>" tooltip shown with Revoke disabled, a recovery link "Invite pending" then revoked back to "Enrolled", a session PATCH of the field 403, 0 `[ERROR]` lines. The member enrolled before this change keeps an empty cell (no stamp was recorded then)
- Enrollment links carry the account's email (`&email=`), and the register page shows it prefilled and read-only, also after a reload (user request 2026-10-02); links without it (break-glass) keep an editable field, and auth still checks the email against the link's account. UI only, deployed (UI 0.0.2). Live headless (throwaway accounts, deleted after): the Members link carries the encoded email; a fresh browser with a virtual authenticator opened it, the address bar lost the query, the field stayed prefilled and read-only after a reload, the passkey registered and the account showed "Enrolled"; a link without the email kept an editable field; the "add on another device" link carries the user's own email; 0 `[ERROR]` lines, and the UI access log holds no `enroll=`. An empty `email=` in a hand-made link now leaves the field editable, and hovering the locked field explains that the email comes from the enroll link
- Gates for both: Go build, vet, test -race (124 packages), golangci-lint 0 issues (auth, exporter, data), coverage auth 72.2 % and exporter 69.4 %; helm lint and validate, the users CRD passes a server-side dry run; UI check-all-and-build 0 errors, 0 warnings; landing-page check-all + build
- Artifact Hub: `charts/artifacthub-repo.yml` and PUBLISHING.md lose the stale "private registry" notes (both chart packages pull anonymously), and the metadata push now follows Artifact Hub's OCI rule: one repository per chart, file pushed to `ghcr.io/telark/charts/telark:artifacthub.io`. Registered on Artifact Hub as `telark` (the ID in the file comes from its control panel) and the metadata pushed (anonymous read OK; the chart versions untouched)

### Done 2026-10-02d (SonarCloud findings; analyzer 0.0.1; live-checked 2026-10-02 on the fresh install)
- Analyzer: CORSMiddleware is added last, so its headers also reach the early 413 refusals. Live: a preflight from `http://localhost:3000` gets the CORS headers; a 65 537-byte body with that Origin gets 413 with `Access-Control-Allow-Origin`, also chunked; without an Origin, 413 and no CORS headers; a disallowed origin's preflight 400 without them
- Analyzer: the Kubernetes API client binds its service-account CA context first (same verification). Live: a manual analyze of a throwaway app completed (`status=done`, 5 tool calls, not truncated), with no TLS error and no `k8s_unavailable` in the result or the logs
- Analyzer: `run_analysis`, `_analyze`, `merge_recommendations` and the evaluate loop delegate to small helpers, and named constants replace repeated literals; outputs unchanged. Live: the review loop wrote 24 and 12 recommendation cards for two throwaway apps with the usual titles ("… runs a single replica", "… has no disruption budget", "… has no resource requests", …)
- Chart: the pre-delete hook containers take memory requests and limits from the mode's shared sizing, and delete-webhooks gets a 64Mi ephemeral-storage limit. Live: `helm get hooks` shows 128Mi/512Mi on both and 64Mi ephemeral storage on delete-webhooks
- CI and build: pip installs wheels only, `npm ci --ignore-scripts` in the UI gate, HTTPS-only kubeconform download, `[[ ]]` tests and an explicit default case in the CI scripts; discovery explains the no-op rollback-lock release; the exporter status-patch and notifier log-hygiene tests use helpers (assertions unchanged)
- Log scan: 0 NOAUTH or WRONGPASS anywhere; 185 `[ERROR]` lines in auth, all between 11:05 and 11:07 UTC while AWS replaced a node (Redis and the exporter unreachable for a minute), and 0 in every container since. Throwaway namespace, apps, user, session and analyzer keys deleted
- Dropped, as decided in round d: S8188 (discovery's package-level cancel must outlive `startMainService()`) and S8544 (needs a hash-locked dependency file)

### Done 2026-10-02e (exporter storage redesign; deployed: exporter 0.0.1 rebuilt, chart from the checkout, fresh EFS install as the final state)
- A default install needs no storage flag: the exporter runs 1 replica on ReadWriteOnce claims from the cluster's default StorageClass, in every mode; `services.exporter.replicas=2` with a ReadWriteMany `app.persistence.storageClass` runs two; `app.singleNode` is gone (ignored if passed). Live: fresh install with no storage flag, claims Bound on gp3, all 15 pods Ready in 34 s
- Upgrades never touch a claim's immutable fields: the chart reads the live claims and keeps their class and access mode, an empty `storageClass` keeps the current class, and sizes only grow, only where the class allows expansion. Live: an upgrade without changes left the claims and the exporter pod untouched; standard → minimal kept 10Gi (no shrink error); → performance expanded the claims to 50Gi/10Gi (its pods then did not fit the 2-vCPU nodes); back to standard kept 50Gi and the disk grew on mount
- A class change moves the data: new claims named after the requested storage, and the exporter's new `migrate-storage` init container copies the snapshots and reports into them (newer file wins, atomic writes) before the exporter starts; the old claims are kept and the upgrade notes list them. Live: gp3 ReadWriteOnce → efs-sc ReadWriteMany with 2 replicas in one upgrade (the original failing case), every seeded snapshot and report served with the same SHA-256; the next upgrade released the old claims; 2 replicas on 2 nodes both served a snapshot written through one of them; efs-sc → gp3 with 1 replica also passed
- Rollback across a move returns the exporter to the old claims as they were; upgrading forward again merges what was written on both sides. Live: rollback served generations 1–3 and the report (generation 4, written on EFS after the move, 404 as documented); generation 5 written during the rollback; roll forward 9/9. A move that cannot bind (ReadWriteMany on gp3) leaves the old claims intact, a re-run still copies from them, and `helm rollback` recovered 9/9
- Uninstall keeps the exporter's claims (`resource-policy: keep`) and a reinstall picks them up: the ones named after its settings, else the original ones; new claims always take the original names, as cluster-less renders do. Live: uninstall + reinstall with the same EFS settings, claims adopted, 9/9
- `app.persistence.size` is renamed `app.persistence.snapshotsSize`, so it reads as the pair of `reportsSize` (user request); the schema rejects the old key. Same sizes in every mode, and a server-side dry run against the live release renders a manifest identical to the installed one
- The render fails on more than one replica without a class, and on a class the cluster does not have (with cluster access). The exporter renders `rollingUpdate` explicitly so a switch to Recreate applies (live through an upgrade and two rollbacks), and its disruption budget is off at one replica (performance mode would otherwise block every node drain)
- Final state: fresh install from the release values (efs-sc, now also `services.exporter.replicas: 2`): original claim names on efs-sc ReadWriteMany, 2 exporter replicas on 2 nodes, Ready in 45 s; users and telarkconfigs CRDs re-applied from the checkout
- Docs: INSTALL (prerequisites, install, new "Exporter storage" section, flags, autoscaling, version note, uninstall), chart README, VALUES.md, README, getting started, PUBLISHING, testing notes, exporter README (the subcommand), helm-chart-change skill; landing-page quickstart, system requirements, installing, uninstalling, upgrading, troubleshooting, backups, metrics, Helm values and the home page install box
- Gates: Go build, vet, test -race (124 packages), golangci-lint 0 issues (exporter), new `CopyTree` tests; helm lint, kubeconform 0 invalid (3 modes × 5 Kubernetes versions), VALUES.md regenerated; landing type-check, lint, build, 101 internal links OK; public-repo grep clean

### Done 2026-10-02f (sizing, probes, hardening, storage; chart only, no image; fresh installs of all three modes on telark-dev)
- CPU limits: `app.shared.resources.limits.cpu` 500m → 1000m (`minimal` 250m → 500m) and the `performance` CPU request 500m → 250m. Live: at 500m discovery's leader was throttled in 20–23% of CFS periods with 150 apps; at 1000m, same 150 apps and two change rounds, 1.6% / 0.1% / 0.9% on its three replicas and 0% everywhere else
- Startup probe for every service (`app.shared.healthCheck.startupProbe`: `/api/v1/status/live`, period 5 s, timeout 5 s, 60 failures = 5 minutes), because the exporter, notifier and auth open their port only once Redis answers. Live (standard): Redis scaled to 0 for 106 s and the exporter (×2) and notifier pods deleted; the new pods failed only their startup probe (25 times each), running pods only readiness, 0 restarts, all Ready 45 s after Redis returned (the old 60 s liveness budget would have restarted them)
- `services.<svc>.healthCheck` merges over the shared block and `_deployment.tpl` renders the three probes from one `range`; ui gets startup, liveness and readiness probes on nginx's `/healthz` (it had none). Live: ui Ready with all three probes in every mode
- Exporter volumes 10Gi/2Gi → 512Mi/512Mi in `standard` and `minimal` (its own block removed), `performance` 50Gi/10Gi → 2Gi/2Gi (user). Snapshots peaked at 179 MB in the 2,000-app storm. Live: fresh claims at those sizes in each mode (gp3, then efs-sc ReadWriteMany with 2 replicas in standard)
- Redis: volume 4Gi → 2Gi and CPU limit 150m → 500m. Live: AOF 41 MiB at 150 apps, its exec probes throttled it in ~6% of periods at 150m and 0% at 500m
- NATS: volume 4Gi → 1Gi and JetStream `max_file` 5G → 700M (the cap was larger than the volume). Live: 0.2–0.4 MiB used under load
- Kyverno admission requests 200m/256Mi → 100m/128Mi. Measured peak 37m/58Mi under a 150-app audit plan
- Ollama: volume 10Gi → 6Gi (the default model plus any one catalog model, since a switch never deletes the old one) and `readOnlyRootFilesystem: true` (it stays root; with the bundled runtime the namespace meets Pod Security `baseline`, documented). Live: model pulls with the read-only root (`minimal`, `performance`), narration on a crash-looping app `narrated=True` in 10.3 s (`standard`)
- Fixed a pre-existing `performance` bug with `REDIS_POOL_SIZE: "24"` on discovery and auth: 12 force-sync workers (discovery) and 3 × 4 cleanup workers (auth) held all 10 pooled connections in blocking `XREADGROUP` (`CLIENT LIST`: 10/10), so discovery logged "service not ready: exporter" every second and never created Applications. Live: after the fix all Ready, 0 restarts, seed and verify 7/7
- VPA: `controlledResources: [memory]` where the service's HPA is on (the HPA measures CPU against the request). Live (performance + `vpa.enabled=true`): all 6 VPAs gave recommendations, memory only on the 4 HPA services
- `validateSubchartVolumes`: an upgrade whose Redis or NATS size differs from the live StatefulSet's claim template fails before anything applies and prints `kubectl delete statefulset -n <ns> <names> --cascade=orphan`; an Ollama size below the live claim fails and names `--set ollama.persistentVolume.size=<live>`. Live (release with 4Gi volumes): the guard stopped the server dry run with that command; after the orphan delete and the upgrade, Redis kept its keys, NATS its 5 streams and 3 consumers, the claims kept 4Gi, verify 7/7. Helm 4 `--dry-run=server` alone does not catch the claim-template change (`kubectl diff` does)
- Live E2E per mode, each a fresh install after all PVCs were deleted: `minimal` (gp3) all Ready, 0 restarts, 0 `[ERROR]`, session, discovery, snapshots 7/7, plan report, model pull, analyze; `performance` with VPA (did not fit the 2 × 2 vCPU nodes by 10m, Ollama scaled down for that run), then without: all Ready, 0 restarts, 0 `[ERROR]`, seed and verify 7/7, model pull, analyze; `standard` (release values): seed and verify 7/7, passkey enroll, register, login and permissions 201/201/200/200, rollback to generation 1 with `rollback.completed`, narration, HPA discovery 1 → 3 at 81%/80% under 150 apps and back to 1 five minutes after they were deleted (while they existed, discovery's per-minute pass averaged ~200m over its pods, twice its request, so it held 3)
- Final scan (standard): 0 NOAUTH or WRONGPASS; every `[ERROR]` line falls inside the deliberate Redis outage (auth's cleanup workers 600, FT20; discovery's force-sync reads and leader election), 0 before or after; Warning events only from probes during the outage and the install and from HPA metrics at install; 0 restarts, no OOMKilled; all 16 pods Ready, every HPA at 1 replica
- Cleanup: the 150-app load, the crash namespace, the pvc-test seed (plan, namespace, user with 0 sessions and passkeys left, notifications) and the passkey user (finalizer done, no keys) are gone, 0 apps left; the analyzer swept the 151 deleted apps' keys itself, pvc-web's (the last app, FT21) were removed by hand; the release stays installed
- Docs: INSTALL (sizing table, flags, subcharts, analyzer runtime, VPA, version note "Smaller Redis, NATS and model volumes"), chart README (resources, probes, per-service `healthCheck`, VPA, Redis, Ollama security, `REDIS_POOL_SIZE`), VALUES.md; landing-page system requirements, Helm values, environment variables, health checks, upgrading, troubleshooting
- Gates: helm lint, kubeconform 15/15 (3 modes × Kubernetes 1.30–1.34), VALUES.md regenerated; landing lint and build; no Go or Python change; public-repo grep clean

### New follow-ups (post-MVP, found 2026-10-02)
- FT18: `topologySpread` counts the old pods during a rolling update, so the new pods can all land on one node (seen with the exporter); `matchLabelKeys: [pod-template-hash]` in `_deployment.tpl` would fix it
- FT19 (before this round too): `app.persistence.enabled=false` renders no exporter claims, but the exporter still mounts them, so it stays Pending
- FT20: while Redis or the exporter is unreachable, auth's cleanup workers log about 4 `[ERROR]` lines per second with no backoff (`[cleanup] stream read failed`, `sweeper list failed`; seen during a node replacement)
- FT21: the analyzer never removes the keys of the last deleted app (its cleanup ignores an empty app list): documents, `analyzer:index`, `analyzer:usage` and `analyzer:review` entries stay
- FT22: `standard` Redis pool margin: discovery holds 7 and auth 6 of their 10 pooled connections in blocking stream reads, which leaves 3–4 for everything else; a pool well above the worker count, or a separate client for blocking reads, would remove the risk
- FT23: `performance` PDBs (`minAvailable: 1`) block every node drain while their service runs one replica (the analyzer always; auth, discovery and notifier at the HPA floor). Design before this round; the exporter's is already off at one replica
- FT24: Kyverno's background and reports controllers have no probes, and no Kyverno container has a CPU limit (upstream chart choices, kept)
- FT25: telark-crds 0.0.1 lacks three fields, so after the checkout CRDs are applied by hand every `helm upgrade` of the dev release conflicts on the users and telarkconfigs CRDs (the release is marked failed, everything else applies). Upgrade it with `--set crds.enabled=false`, never `--force-conflicts` (that restores the 0.0.1 CRDs and prunes the newer fields), until telark-crds is released and re-pinned
- FT26: enabling `vpa.enabled` on an upgrade does not install the VPA CRDs (Helm applies a subchart's `crds/` only on install)
- FT27: `minimal` was live-checked before the NATS 1Gi and Ollama 6Gi change; both values are mode-independent and passed in `standard` and `performance`
- FT11: deleting a user leaves its pending link keys in Redis until they expire (at most 1 h); the link already answers like any dead link
- FT17 (FT11 family): deleting a user leaves its notifications in Redis for good (`notif:user:<id>:items`, `:unread_count` and the notices, no expiry)
- FT12: a link that was opened but not finished keeps "Invite pending" until it expires (the invite clears only when a passkey is stored)
- FT13: auth's 403s carry no error code, so the UI matches the recovery refusal on its text
- FT14: an existing TelarkConfig without `selfRegistration` shows no such key in `GET config` until it is first saved (auth and the UI read it as off)
- FU9: the UI treats an unreadable public auth config as "self-registration on" (`selectSelfRegistrationEnabled`); the server still refuses
- FU10: an enroll link opened in a browser that already has a session redirects home and loses the token
- FU11: after a self-registration flip, a tab's cached auth config (5 min) still drives its login and register pages
- FU12: `BOOTSTRAP_PILL.TOOLTIP` contains a semicolon; the Members row action buttons hide keyboard focus (`outline: none`); `UsersTable` is exported but unused
- FD1: `services/exporter/README.md` spells the deny rule `controlaiinsights` (code: `controlainsights`)
- FT15 (FT1 family): auth's Google callback logs `[ERROR]` for its expected 409 refusals and for the 403 that refuses Google sign-in for the bootstrap account (`handlers/oidc/callback.go:136`, `shared.HandleError`)
- FU14: a disabled toolbar button (the settings Save buttons, for example) jumps to full opacity when the mouse leaves it: `onMouseLeave` lacks the disabled check `onMouseEnter` has (`components/display/toolbar/Toolbar.tsx`)
- ~~FT16: an invalid `ENROLL_INVITE_TTL_SEC` warns under the cleanup logger with a "[cleanup]" tag, because `envSeconds` is shared with the cleanup tunables~~ done 2026-10-02c
- ~~FU13: six settings sections repeat the toolbar key `'save'`, and three hints repeat `fontSize: 12`, inline; move both to `SETTINGS_CONSTANTS`~~ done 2026-10-02c
- ~~FD2: `.github/workflows/build-service.yaml` keeps one "cancelled" comment (any edit under `.github/` takes the action-pin audit); `CHANGELOG.md` keeps British words from old commit subjects (git-cliff generates it)~~ done 2026-10-02c (`CHANGELOG.md` left as generated)

## Done 2026-10-02h (navigation and settings cleanup; deployed discovery + exporter 0.0.1 and UI 0.0.2, telarkconfigs CRD patched live)
- Settings > Governance is now Discovery & Storage (database icon) at /settings/discovery, without the fetch interval field.
- Fetch interval removed end to end: TelarkConfig `userSettings` (CRD, Go types, defaults, exporter guard, docs). A PATCH carrying it now gets 400. UI lists and details refresh every 60 s, and discovery's rediscovery runs every 60 s.
- Settings > Single Sign-On is now Authentication (key icon) at /settings/authentication, first under Platform.
- Applications and Protection plans cards lost their top-left icon chip.
- The header border reaches the screen edge: pages scroll in a content pane below the header (with its own stable scrollbar gutter) instead of the window.
- `cluster/namespaces` no longer returns Telark's own namespace while self-monitoring is off (excluded-namespaces picker, plan scopes, Insights filter); the self-monitoring test covers on and off.
- Sidebar: Operations (Applications, Protection plans, Insights) and Administration (Members, Groups, Roles, Settings). Settings left the account menu.
- Icons: Protection plans shield with check, Roles person with key, Settings gear.
- Routes: /applications/:name, /protection-plans(/:name), /members, /groups, /roles, /settings/insights, /settings/discovery, /settings/authentication, /settings/security/passkeys (was /passkeys plus an in-page view).
- Selectable dropdowns (Applications view, Insights group by, compact quick filters) no longer show the green selected highlight, like Select options.
- Roles: creating the first role no longer loops forever (the create panel refetched roles, which flipped the empty page and remounted the panel).
- FT28: release telark-crds and re-pin it; until then a `helm upgrade` puts `userSettings` back into the live CRD (unused, harmless).
- FU15: `useRoleActions` and `useEditRoleSubmit` still navigate to `${APP_ROUTES.ROLES}/${id}/view`, a route that never existed (every caller passes `skipNavigate`).
- FD3: the landing site's marketing copy (nav, feature tile, pricing) still says "Access & permissions".
- Not live-checked: self-monitoring on (turning it on would register Telark's own components as applications); the unit test covers it.

## Follow-ups (post-MVP)
- FT1: exporter body-validation 400s still log `[ERROR]`: `handlers/categories/handler.go:33`, `handlers/plans/protection/handler.go:174`, `handlers/resources/{group/handler.go:50, role/handler.go:42, user/handler.go:51}`, `utils/classification/category/validate.go:84`, `utils/resources/group/{patch.go:45, validate.go:17}`, `utils/resources/user/patch.go:46`
- FT2: a role PATCH with `scopesAndPermissions: []` still writes the priority and version computed from the old scopes
- FT3: exporter `authz.ResetGenerationFloor` (`cache.go:109`) is a test-only export
- FT4: while NATS is unreachable, `BuildPrewarmApplicationOptions` blocks up to 30 s per call under the x-ware NATS client lock, including from the informer flush path, and it retries even when `NATS_HOST` is unset (fix without changing the healthy path)
- FT5: the Redis key table still lacks `rollback:applying:`, `cleanup:empty_streak:`, `cleanup:auto:inflight:`, `incident:state:`, `analyzer:inflight:`, `lock:gen:`, `ops:`, `grace:scale:`, `reset:cooldown:`
- FT6: `TestRecordWithRetryOutlivesCanceledCallerAndReturnsLastError` takes 10 s (waits the real retry interval); speed it up only through production paths
- FT7 (FT1 family): discovery's rollback trigger logs its body errors (413, 422, 400) as `[ERROR]` (`decodeTriggerRollbackBody`, `applications/rollback.go:281`)
- FT7b: discovery's protection-plan handlers also log expected 4xx refusals as `[ERROR]`, and so do some not-found lookups (`applications.telark.io … not found`, `accessroles.telark.io … not found`)
- FT8: the exporter's `LogByStatusAndSend` logs body-validation 4xx (app PATCH 400s) at `[WARNING]`
- FT9: the analyzer's guard card cites an entry's first non-health change, so a changed ConfigMap reference reads `configMapRef none→<new>` instead of `<old>→<new>`
- FT10: app reset and auto-cleanup delete only discovery's Redis keys; the analyzer's `analyzer:<ns>:<app>` cache (7 days) and its auto cooldown outlive the app, so a recreated app inherits them
- FU1: inline UI strings: `PasskeyCard.tsx` 'Failed to copy'; plans `HealthSection.tsx` 'No policy details available.' and 'yes'/'no'; `ApplicationChangeLogSection.tsx` `severity: ${…}`; `CategoryColumns.tsx` 'Built-in'/'Custom'; `useViewGroupPanelData.tsx` 'Creation Date'; `DiscoveryBehaviorSection.tsx` card title and description; the `'custom'` option value repeated in two files
- FU2: users list `Columns.tsx` builds the role-count plural inline instead of `pluralize`
- FU3: profile `EMAIL_PLACEHOLDER` still says 'e.g. john@example.com'
- FU4: plans `HealthSection.tsx` imports the applications feature's `APPLICATION_SECTION_LAYOUT` (hoist it to `src/constants`)
- FU5: drop the `APPLICATION_MANIFEST_VIEW.CODE_CLASS` alias once its 2 callers import `MONOSPACE_CLASS`
- FU6: with roles read but no groups read, Manage Roles "From Groups" is empty instead of the groups no-access card, and the users-list role count leaves out group-inherited roles (pre-existing)
- FU7: the '—' empty value is defined in 7 feature constants files plus `ACTORS.NONE`, and inline in `SessionsTable.tsx:159` and `ViolationsSection.tsx:138, :232`: one root constant
- FU8: when the users list fails to load, the Members toolbar still says "0 members" next to the error card
- Redis TLS in transit: the subchart's generated certificate never rotates and its SANs miss the short service name; needs cert-manager or an operator-supplied `tls.existingSecret`, plus TLS in `x-ware/redis/core` and the analyzer
- Redis per-service ACL users: the subchart renders them from `lookup` (no password on a fresh or cluster-less install) and services share keyspaces without a per-service prefix map (FT5)
- Kyverno API types: `internal/data/policies` and 6 discovery plan files import Kyverno's v1 API, which pulls the cosign and cloud-provider SDKs (data tests build 1,846 packages instead of 339, discovery binary 2,169 instead of 1,009). Proposal: local JSON-identical types for the 18 types used, the real types only in discovery's `kyverno_accepts_test.go`, plus a rendered-policies-unchanged test

## Next (MVP)
- Passkey onboarding (enroll invitations, self-registration in TelarkConfig, bootstrap-only identity settings): implemented and deployed 2026-10-02 (see "Done 2026-10-02"); left: the user's bootstrap-side Chrome check and the telark-crds release
