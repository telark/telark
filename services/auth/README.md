# auth service

The auth service signs people in to Telark. It handles passwordless login with WebAuthn
passkeys and Google OIDC, creates sessions, runs the break-glass admin enrollment, and cleans
up references when a user, group or role is deleted. It keeps no database: users, passkeys
and sessions are CRs written through exporter, and the digests of WebAuthn challenges live in Redis.

## Architecture

```mermaid
%%{init: {"theme":"base","themeVariables":{"fontFamily":"ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif","fontSize":"13px","lineColor":"#94a3b8","primaryColor":"#eef2ff","primaryBorderColor":"#6366f1","primaryTextColor":"#312e81","edgeLabelBackground":"#ffffff","clusterBkg":"#f8fafc","clusterBorder":"#e2e8f0"},"flowchart":{"curve":"basis","htmlLabels":true,"nodeSpacing":48,"rankSpacing":62,"padding":12}}}%%
flowchart LR
  UI(["ui / operator"])

  subgraph auth["auth"]
    RT(routes) --> H("handlers<br/>auth · passkey · oidc")
    H --> WA(webauthn)
    H --> OI(oidc)
    CL(cleanup controller)
  end

  REDIS[("Redis<br/>challenges · enroll tokens · cleanup streams")]
  EXP(exporter)
  GOOG{{"Google OIDC<br/>JWKS"}}

  UI -->|login / passkey| RT
  H -->|challenges| REDIS
  CL -->|cleanup jobs| REDIS
  H -->|User · Session · Passkey CRDs| EXP
  OI -.egress.-> GOOG

  classDef svc fill:#eef2ff,stroke:#6366f1,stroke-width:1.5px,color:#312e81;
  classDef peer fill:#f1f5f9,stroke:#94a3b8,stroke-width:1.5px,color:#334155;
  classDef store fill:#fff7ed,stroke:#f59e0b,stroke-width:1.5px,color:#92400e;
  classDef ext fill:#faf5ff,stroke:#a855f7,stroke-width:1.5px,color:#6b21a8;
  class RT,H,WA,OI,CL svc;
  class UI,EXP peer;
  class REDIS store;
  class GOOG ext;
```

## Responsibilities

- **Passkey auth:** WebAuthn registration and assertion (login start/finish), passkey CRUD.
- **Sign-in rate limits:** the four public starts (passkey sign-in and registration, break-glass approval, Google nonce) take `RATE_LIMIT_PER_CLIENT` starts per client and `RATE_LIMIT_GLOBAL` per route a minute, counted in Redis, then 429 with `Retry-After: 60`; the client comes from `X-Forwarded-For` past `RATE_LIMIT_TRUSTED_HOPS` proxies, and a Redis outage lets the starts through ([Sign-in rate limits](../../docs/INSTALL.md#sign-in-rate-limits)).
- **Google OIDC:** token verification against Google's JWKS, fetched live (`egressAllowed` in the TelarkConfig `oidc` block) or from a pasted key set for air-gapped clusters (`OIDC_TRUST_FILE`). The token's subject picks the user; a first login with no stored subject attaches the identity by email only when exactly one user carries that email and that account has no identity yet (409 otherwise). A login that resolves to the bootstrap admin, by subject or by email, is refused (403, `the bootstrap administrator signs in with a passkey`). Full admin-config + login flow: **[OIDC.md](OIDC.md)**.
- **Sessions:** issue and validate; a sign-in answers the session only as the HttpOnly session cookie, `__Host-telark_session` when every passkey origin is https and `telark_session` otherwise (`SESSION_COOKIE_HOST_PREFIX`, set by the chart), which x-ware turns into `X-Session-Token` (the header API clients send), and logout clears it. Sessions are `Session` CRs written through exporter. An expired session is refused on use, and a user's expired sessions are deleted when that user gets a new session (there is no global sweeper). A session the exporter reports as expired (410) answers 401, never 503; only an unreachable exporter is an outage. No login path mints a session for a user being deleted or whose account is not active (403); a suspended account is told `user account is suspended`, only once its passkey or Google token has been verified. A login's last-login stamp never writes the account phase.
- **Cleanup cascade:** deleting a user, group or role through the cleanup API clears every back-reference before the finalizer is dropped; a user's sessions, live enrollment link and passkeys are deleted first, so revoked access does not outlive the deletion, and its notifications are cleared once no back-reference is left (removing it from a group notifies it one last time). The route requires Owner on the scope and honors the `deleteuser` / `deletegroup` / `deleterole` deny rules; an id the exporter no longer has answers 404. A failed pass, and a worker whose stream read fails (Redis down), is retried with exponential backoff (`RECONCILE_BACKOFF_INITIAL_SECONDS` doubling up to `RECONCILE_BACKOFF_MAX_SECONDS`, capped by `CLEANUP_XCLAIM_MIN_IDLE_SECONDS`), a job whose worker died is reclaimed once it has idled past that threshold, and the dead-letter stream is capped at 1000 entries.
- **User deletion rules:** a user never deletes their own account; bootstrap users (`bootstrap: true` on the user record) are never deleted through the API (403); a caller below Admin targeting either a bootstrap user or an administrator (Admin on ALL, direct or via a group, suspended or not) is answered 404; any other Admin may delete an administrator. Internal callers are not gated here; the exporter refuses (409) a delete that would leave no active Admin.
- **Role and group deletion rules:** deleting a role, or a group, takes its levels from every holder, so each level of the role (or of every role the group assigns) must be within the caller's own on that scope or on ALL; otherwise 403 (`role <name> cannot be changed, removed or deleted: it grants <level> on <scope>, above your own level on that scope`). An Admin on ALL may delete any role or group. A role or group already gone passes to the exporter (404); internal callers are not gated here.
- **Role model:** reconcile authorization resources. OIDC and passkey self-registration always create a ReadOnly account without the `bootstrap` marker, even for the `BOOTSTRAP_ADMIN` email; bare-email passkey registration refuses that email. Only `break-glass` grants the bootstrap admin its Admin role and marker, and it drops any non-passkey identity from that account, so the bootstrap admin signs in with a passkey only.
- **Provisioning policy:** self-registration (the TelarkConfig `selfRegistration.enabled`, off by default, read through a cache that asks the exporter at most once every 5 seconds and keeps the last value when a read fails) gates the passkey path only; OIDC users are always auto-provisioned. A bare email opens a registration only for an account that does not exist yet, and that account is created only when the ceremony finishes with a verified passkey (an abandoned start leaves nothing behind); an existing account adds a passkey through its session or a one-time enrollment link.
- **Enrollment links for another user:** `POST auth/users/{id}/enroll-link` (201, `{token, expiresAt}`, the token shown once) and `DELETE` (revoke). Owner on users, capped like a role assignment: every level the target holds, directly or through a group, must be within the caller's own (403). Oneself (403), a hidden administrator or bootstrap account (404 below Admin on ALL, 403 for the bootstrap account otherwise), a terminating (410) or suspended (409) account are refused; revoking skips the last two. An account that already has a passkey needs the bootstrap account or Admin on ALL, and its owner is notified when the link is created and when it is used. Redis keeps only the token's SHA-256 digest (`auth:passkey:invite:<digest>`, whose account is signed with the service token, and `auth:passkey:invite-of:<user>`, both with `ENROLL_INVITE_TTL_SEC`), one live link per user; the user's `status.invite` records issuer and expiry for display and is cleared on revoke and once a passkey is stored; opening the link only reads it, so a canceled or failed ceremony leaves it usable, and the finish that saves a passkey spends it atomically and stamps `status.inviteAcceptedAt` (Members shows "Enrolled").
- **Break-glass requests:** once the bootstrap account is pinned (its first passkey), break-glass opens a request on the exporter instead of acting. `POST auth/bootstrap/requests/{id}/assert/start` and `.../finish` (public, `action` `approve` or `deny`) let the pinned account decide it with a user-verified passkey assertion bound to the request and the action, and no sign-in; they create no session. A recovery can only be denied: `approve` on one answers 409 before any challenge is stored. A passkey sign-in by the bootstrap account cancels its pending request (`signed-in`); a failed cancel never fails the sign-in. The account's own enrollment links are indexed (`auth:passkey:enroll-tokens-of:<user>`) so a recovery drops them, with its open ceremonies, while the account is frozen (`auth:webauthn:frozen:<user>`: sign-in, registration, link creation and a deny answer 409, and a passkey, identity or session write checks it again right before it is sent). While a recovery waits for its link, the freeze holds that link's digest, and only the registration the link opened (its start, finish and both writes) gets past it. A bootstrap sign-in with a passkey saved after the wait (24 hours by default, `app.security.breakGlass.waitHours`) of a recovery that is still open is refused (409) before any session, and cancels nothing. One break-glass run works at a time (`auth:breakglass:run`).
- **Sign-in settings:** `PATCH auth/oidc/config`, `PATCH auth/self-registration` and `PATCH auth/break-glass-alerts` are refused (403) to everyone but the bootstrap account, read from the caller's own user record (503 when it cannot be read).
- **Ops subcommands:** `break-glass --email <email> [--enroll]` (the bootstrap admin's access: any email but `BOOTSTRAP_ADMIN` is refused; `--enroll` creates the account when missing and prints a one-time enrollment link, issued like an enrollment link for another user but with no issuer and a fixed 10-minute lifetime, the operator-run way to enroll the first administrator; once the account is pinned, it opens a request first), `backfill-finalizers`, and `remove-finalizers` (run by the chart's pre-delete hook on `helm uninstall`, with auth stopped, to clear the cleanup finalizers), via the binary's `cmd` dispatch ([cmd/README.md](cmd/README.md)).

## Layout

| Package | Role |
|---|---|
| `internal/routes` | HTTP route registration |
| `internal/handlers/{auth,passkey,oidc,authorization,config,cleanup,status,bootstrap}` | Request handlers per domain |
| `internal/bootstrap` | Break-glass request approval ceremony, the sign-in cancel and the refusal of a stopped recovery's passkey |
| `internal/ratelimit` | Rate limit middleware on the public sign-in starts |
| `internal/helpers/{webauthn,oidc,auth,redis,shared}` | WebAuthn/OIDC logic, Redis access, shared helpers |
| `internal/controllers/cleanup` · `internal/coordination/cleanup` | Deletion cleanup reconciler (sessions, passkeys, enrollment links, notifications and back-references of deleted users, groups and roles) + its coordination |
| `internal/clients` | Exporter REST client wrappers |
| `cmd` | CLI subcommands ([cmd/README.md](cmd/README.md)) |
| `internal/authz` | Per-route authorization requirements |

## Dependencies

- **Shared packages:** `data` (auth types, errors), `rest` (exporter client, router, server), `x-ware` (Redis, authz, CORS). **No `kcore`**: auth never touches the K8s API directly; all CRD access is through exporter.
- **Infrastructure:** Redis (WebAuthn challenge digests, OIDC nonces, enrollment token digests, sign-in rate limit counts, cleanup streams).
- **Peers:** persists CRs through **exporter**; verifies tokens against **Google OIDC** (external, optional egress).

## Configuration

Full reference: [chart README](../../charts/telark/README.md#servicesauthenv). Key vars:

| Variable | Default | Description |
|---|---|---|
| `RP_NAME` | required | WebAuthn relying-party display name |
| `RP_ID` / `RP_ORIGIN` | empty | WebAuthn relying-party id / allowed origins; when empty, both are resolved from the request host |
| `ENROLL_INVITE_TTL_SEC` | `3600` | Lifetime of an enrollment link created for another user (a `break-glass --enroll` link always lives 10 minutes); a zero, negative or malformed value keeps the default |
| `CHALLENGE_TIMEOUT` / `SESSION_EXPIRY` | `60` (s) / `24` (h) | Challenge / session TTLs |
| `BOOTSTRAP_ADMIN` | required | The bootstrap admin's email, enrolled and recovered with `break-glass --enroll`; auth refuses to start without it |
| `BREAK_GLASS_WAIT_HOURS` | `24` | Hours a break-glass request waits with no bootstrap sign-in before it completes on its own (`app.security.breakGlass.waitHours`); anything but a whole number of hours, at least 1, stops auth at start |
| `OIDC_TRUST_FILE` | `/etc/telark/oidc/googleJwkJson` | Pasted Google JWK set, mounted from the Secret `telark-oidc-trust-secret`; re-read when it changes |
| `REDIS_DB` | `0` (chart: `1`) | Redis DB index |

The Google client id, the egress mode, self-registration and break-glass alerts are not env vars: they live in the TelarkConfig (`oidc`, `selfRegistration`, `breakGlassAlerts`), set through `PATCH auth/oidc/config` ([OIDC.md](OIDC.md)), `PATCH auth/self-registration` (`{"enabled": true}`) and `PATCH auth/break-glass-alerts` (`{"notifyAdmins": false}`). The cleanup tunables (`RECONCILE_*`, `CLEANUP_*`, `BACKFILL_*`) take positive integers; a malformed, zero or negative value falls back to its default with a warning.

## API

REST under `/api/v1/auth/`: login `start`/`finish`, `logout`, passkeys (`GET`/`POST auth/passkeys`,
`GET`/`PATCH`/`DELETE auth/passkeys/{credentialId}`, `auth/passkeys/enroll-link`), enrollment links
for another user (`POST`/`DELETE auth/users/{id}/enroll-link`), OIDC login and config, the
self-registration toggle (`PATCH auth/self-registration`), the break-glass alerts switch
(`PATCH auth/break-glass-alerts`), permissions, and the deletion cascade
(`DELETE auth/{users,groups,accessroles}/{id}`); status
probes at `/api/v1/status/{live,ready}`. Passkey and session-scoped calls require the session
(the cookie, or the `X-Session-Token` header), except `POST auth/passkeys`, which also finishes a registration opened
by `register/start` (with an enrollment token or a bare email) and requires `X-Device-Name` and
`X-Device-Type`. `PATCH`/`DELETE auth/passkeys/{credentialId}` relay the exporter's refusal: 404
for an unknown credential or another account's (indistinguishable), 400 for the last passkey. A request body over 1 MiB answers 413. Every 403 auth itself refuses carries a stable `code` beside its `message` (for example `enroll_link_recovery`, `account_suspended`, `self_delete`; the list is `Refusal*` in `internal/constants/strings.go`), so clients branch on the code, not the wording. auth
stores passkeys and sessions (`Passkey`, `Session` CRs) through the exporter's `internal/auth/*` routes.

## Build & run

```sh
# from the repo root: the image builds the service together with internal/
go build ./services/auth
docker build -f services/auth/Dockerfile -t ghcr.io/telark/auth:<version> .
```

Runs in-cluster via the [telark chart](../../charts/telark); see [INSTALL](../../docs/INSTALL.md)
and [CONTRIBUTING](../../CONTRIBUTING.md).
