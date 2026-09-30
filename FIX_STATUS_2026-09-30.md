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

### Chart, infra
- C1 CRD write guard covers every namespace
- C2 uninstall leaves no Kyverno webhooks / finalizers (pre-delete hooks)
- C3 ui pod no longer gets the service token
- C4 dead nats securityContext; C5 duplicated metrics-server args
- C7 NATS/Ollama NetworkPolicy namespace
- I1 vpc-cni `enableNetworkPolicy` (Terraform, not applied)
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
- U16 register form flash; U17 dark auth CTA; U18 enrol link lost on reload (and dead token kept after a failed enrolment)
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
- Modals: content modals (session expired, enrol link, orphaned passkeys, avatar picker) on the shared chrome; premium restyle within the theme
- Notification rows restyle
- UI items: group tooltip avatar initials, Manage Roles at 900×900, rollback meta ellipsis; F12 long app title overflows the page
- Visual check: D8 CTAs, dialogs, bell, SSO JWK, change log
- Rebuild + redeploy; retest FAIL/BLOCKED rows, regressions, `rbac_matrix.py`
- Clean up test data (e2e-* namespaces, personas, sessions); discovery replicas back to 1
- Commit messages per repo; remove `go.work` replaces first

## User actions
- ~~rest: tag a release with `e9d3380` + the staged `Delete`, then bump the rest pin~~ obsolete: rest is `telark/internal/rest` since the monorepo merge
- infra: `terraform apply` (I1), then NetworkPolicy retests NP-2…NP-11
- Live checks needing you: D2a/D2c, A1 + D1 OIDC callback, U2/U18 screenshots, D10 fixture, IAM-14b/E5-boot, U12/U19 fixtures

## Remaining, part 2 (former follow-ups, this round too)
- Remove the pre-existing seams `UseCacheForTest`, `SetExcludedForTest`; review test-only exports (`PublishedPayload`, `HiddenPlatformApp`, `FailStaleInProgress`, `FinalizeRollbackSuccess`)
- D11: recreate later than one window or across leader failover
- Role PATCH/DELETE last-admin guard
- E16 finalizer-body half; E9 sibling error paths; CORS `Retry-After` (x-ware)
- Passkey onboarding (decided, next after this round; plan as a feature):
  1. Invites: admin creates the user, then Members → "Create enrol link": one-time URL shown once with copy button and expiry (~24 h), single-use, bound to the user, re-issuable; "Invite pending / expired" in Members. Owner on users; Admin targets need Admin on ALL. No SMTP in MVP.
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
- Role pickers (Add/Edit member roles, group Manage Roles): new lock state ("No access / Requires ReadOnly on roles") in light-panel colours
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
