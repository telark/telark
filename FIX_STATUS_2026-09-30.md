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
