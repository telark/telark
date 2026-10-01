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
- Modals: content modals (session expired, enroll link, orphaned passkeys, avatar picker) on the shared chrome; premium restyle within the theme
- Notification rows restyle
- UI items: group tooltip avatar initials, Manage Roles at 900×900, rollback meta ellipsis; F12 long app title overflows the page
- Visual check: D8 CTAs, dialogs, bell, SSO JWK, change log
- Rebuild + redeploy; retest FAIL/BLOCKED rows, regressions, `rbac_matrix.py`
- Clean up test data (e2e-* namespaces, personas, sessions); discovery replicas back to 1
- Commit messages per repo; remove `go.work` replaces first

## User actions
- ~~rest: tag a release with `e9d3380` + the staged `Delete`, then bump the rest pin~~ obsolete: rest is `telark/internal/rest` since the monorepo merge
- Live checks needing you: D2a/D2c, A1 + D1 OIDC callback, U2/U18 screenshots, D10 fixture, IAM-14b/E5-boot, U12/U19 fixtures

## Remaining, part 2 (former follow-ups, this round too)
- Remove the pre-existing seams `UseCacheForTest`, `SetExcludedForTest`; review test-only exports (`PublishedPayload`, `HiddenPlatformApp`, `FailStaleInProgress`, `FinalizeRollbackSuccess`)
- D11: recreate later than one window or across leader failover
- Role PATCH/DELETE last-admin guard
- E16 finalizer-body half; E9 sibling error paths; CORS `Retry-After` (x-ware)
- Passkey onboarding (decided, next after this round; plan as a feature):
  1. Invites: admin creates the user, then Members → "Create enroll link": one-time URL shown once with copy button and expiry (~24 h), single-use, bound to the user, re-issuable; "Invite pending / expired" in Members. Owner on users; Admin targets need Admin on ALL. No SMTP in MVP.
  2. Self-registration moves from the chart (`app.auth.passkey.selfRegistration`, `SELF_REGISTRATION_ENABLED`) to a TelarkConfig field (CRD first), default off, toggle in Settings next to Single Sign-On, read live by auth; chart guard becomes "bootstrap admin required"; docs updated.
  3. Bootstrap-only control: SSO config and the self-registration toggle editable only by the bootstrap user (backend + UI, today Admin on ALL); others see them read-only with a tooltip.
- ~~Monorepo: shared modules into `telark/internal/`~~ DONE 2026-09-30 (history merges + uncommitted move on `chore/import-module-history`; merge its PR with a merge commit)
- data `internal/data/shared/config.go` dead map; ~~gofmt `app_namespaces_test.go`~~ done in the monorepo gofmt pass

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

### New follow-ups (post-MVP, found 2026-10-02)
- FT11: deleting a user leaves its pending link keys in Redis until they expire (at most 1 h); the link already answers like any dead link
- FT12: a link that was opened but not finished keeps "Invite pending" until it expires (the invite clears only when a passkey is stored)
- FT13: auth's 403s carry no error code, so the UI matches the recovery refusal on its text
- FT14: an existing TelarkConfig without `selfRegistration` shows no such key in `GET config` until it is first saved (auth and the UI read it as off)
- FU9: the UI treats an unreadable public auth config as "self-registration on" (`selectSelfRegistrationEnabled`); the server still refuses
- FU10: an enroll link opened in a browser that already has a session redirects home and loses the token
- FU11: after a self-registration flip, a tab's cached auth config (5 min) still drives its login and register pages
- FU12: `BOOTSTRAP_PILL.TOOLTIP` contains a semicolon; the Members row action buttons hide keyboard focus (`outline: none`); `UsersTable` is exported but unused
- FD1: `services/exporter/README.md` spells the deny rule `controlaiinsights` (code: `controlainsights`)
- FT15 (FT1 family): auth's Google callback logs `[ERROR]` for its expected 409 refusals (`handlers/oidc/callback.go`, `shared.HandleError`)
- FT16: an invalid `ENROLL_INVITE_TTL_SEC` warns under the cleanup logger with a "[cleanup]" tag, because `envSeconds` is shared with the cleanup tunables
- FU13: six settings sections repeat the toolbar key `'save'`, and three hints repeat `fontSize: 12`, inline; move both to `SETTINGS_CONSTANTS`
- FD2: `.github/workflows/build-service.yaml` keeps one "cancelled" comment (any edit under `.github/` takes the action-pin audit); `CHANGELOG.md` keeps British words from old commit subjects (git-cliff generates it)

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
- Kyverno API types: `internal/data/policies` and 6 discovery plan files import Kyverno's v1 API, which pulls the cosign and cloud-provider SDKs (data tests build 1,846 packages instead of 339, discovery binary 2,169 instead of 1,009). Proposal: local JSON-identical types for the 18 types used, the real types only in discovery's `kyverno_accepts_test.go`, plus a rendered-policies-unchanged test

## Next (MVP)
- Passkey onboarding (enroll invitations, self-registration in TelarkConfig, bootstrap-only identity settings): implemented and deployed 2026-10-02 (see "Done 2026-10-02"); left: the user's bootstrap-side Chrome check and the telark-crds release
