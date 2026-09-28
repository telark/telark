# auth service

The auth service signs people in to Telark. It handles passwordless login with WebAuthn
passkeys and Google OIDC, creates sessions, runs the break-glass admin enrolment, and cleans
up references when a user, group or role is deleted. It keeps no database: users, passkeys
and sessions are CRs written through exporter, and login challenges live in Redis.

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

  REDIS[("Redis<br/>challenges · enrol tokens · cleanup streams")]
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
- **Google OIDC:** token verification against Google's JWKS, fetched live (`EGRESS_ALLOWED=true`) or from a pasted key set for air-gapped clusters. The token's subject picks the user; a first login with no stored subject attaches the identity by email only when exactly one user carries that email (409 otherwise). Full admin-config + login flow: **[OIDC.md](OIDC.md)**.
- **Sessions:** issue and validate (`X-Session-Token`); sessions are `Session` CRs written through exporter. An expired session is refused on use, and a user's expired sessions are deleted when that user gets a new session; there is no global sweeper. A session the exporter reports as expired (410) answers 401, never 503; only an unreachable exporter is an outage. No login path mints a session for a user being deleted or whose account is not active (403).
- **Cleanup cascade:** deleting a user, group or role through the cleanup API clears every back-reference before the finalizer is dropped; a user's sessions are deleted first, so revoked access does not outlive the deletion. The route requires Owner on the scope and honours the `deleteuser` / `deletegroup` / `deleterole` deny rules; an id the exporter no longer has answers 404. A failed pass is retried with exponential backoff (`RECONCILE_BACKOFF_INITIAL_SECONDS` doubling up to `RECONCILE_BACKOFF_MAX_SECONDS`, capped by `CLEANUP_XCLAIM_MIN_IDLE_SECONDS`), a job whose worker died is reclaimed once it has idled past that threshold, and the dead-letter stream is capped at 1000 entries.
- **User deletion rules:** a user never deletes their own account; bootstrap users (`bootstrap: true` on the user record) are never deleted through the API (403); an administrator (Admin on ALL, direct or via a group, suspended or not) is deleted only by a bootstrap user (403 otherwise); a caller below Admin targeting either is answered 404. Internal callers are not gated.
- **Role model:** reconcile authorization resources; grant the Admin role to `BOOTSTRAP_ADMINS` on first OIDC login and mark the record `bootstrap: true` (both re-applied on every login when missing). Passkey self-registration always creates a ReadOnly account and refuses a `BOOTSTRAP_ADMINS` email.
- **Provisioning policy:** `SELF_REGISTRATION_ENABLED` gates the passkey path only; OIDC users are always auto-provisioned. A bare email opens a registration only for an account that does not exist yet; an existing account adds a passkey through its session or a one-time enrolment link.
- **Ops subcommands:** `backfill-finalizers` (migrate/seed auth data) and `break-glass --email <email> [--enroll]` (emergency admin access; `--enroll` creates the account when missing and prints a one-time enrolment token, the operator-run way to enrol the first administrator on a passkey-only install) via the binary's `cmd` dispatch.

## Layout

| Package | Role |
|---|---|
| `internal/routes` | HTTP route registration |
| `internal/handlers/{auth,passkey,oidc,authorisation,config,cleanup,status}` | Request handlers per domain |
| `internal/helpers/{webauthn,oidc,auth,redis,shared}` | WebAuthn/OIDC logic, Redis access, shared helpers |
| `internal/controllers/cleanup` · `internal/coordination/cleanup` | Deletion cleanup reconciler (sessions and back-references of deleted users, groups and roles) + its coordination |
| `internal/clients` | Exporter REST client wrappers |
| `internal/cmd/{backfill,breakglass}` | CLI subcommands |
| `internal/authz` | Per-route authorization requirements |

## Dependencies

- **Internal modules:** `data` (auth types, errors), `rest` (exporter client, router, server), `x-ware` (Redis, authz, CORS). **No `kcore`**: auth never touches the K8s API directly; all CRD access is through exporter.
- **Infrastructure:** Redis (WebAuthn challenges, OIDC nonces, enrolment tokens, cleanup streams).
- **Peers:** persists CRs through **exporter**; verifies tokens against **Google OIDC** (external, optional egress).

## Configuration

Full reference: [chart README](../../charts/telark/README.md#servicesauthenv). Key vars:

| Variable | Default | Description |
|---|---|---|
| `RP_ID` / `RP_NAME` / `RP_ORIGIN` | `localhost` / `Dashboard App` / `http://localhost:3000` | WebAuthn relying-party identity |
| `SELF_REGISTRATION_ENABLED` | `true` (chart: `false`) | `false` blocks new passkey registration; a self-registered account is always ReadOnly |
| `CHALLENGE_TIMEOUT` / `SESSION_EXPIRY` | `60` (s) / `24` (h) | Challenge / session TTLs |
| `BOOTSTRAP_ADMINS` | — | Comma-joined admin emails granted Admin on first login |
| `GOOGLE_CLIENT_ID` | — | Google OAuth client id |
| `EGRESS_ALLOWED` | `true` | `false` = offline JWKS from `GOOGLE_OIDC_JWK_JSON` |
| `OIDC_TRUST_FILE` | `/etc/telark/oidc/googleJwkJson` | Pasted Google JWK set, mounted from the Secret `telark-oidc-trust-secret`; re-read when it changes |
| `REDIS_DB` | `1` | Redis DB index |

## API

REST under `/api/v1/auth/`: login `start`/`finish`, `logout`, passkeys (`GET`/`POST auth/passkeys`,
`GET`/`PATCH`/`DELETE auth/passkeys/{credentialId}`, `auth/passkeys/enroll-link`), OIDC login and
config, permissions, and the deletion cascade (`DELETE auth/{users,groups,accessroles}/{id}`); status
probes at `/api/v1/status/{live,ready}`. All passkey and session-scoped calls require the
`X-Session-Token` header. auth stores passkeys and sessions (`Passkey`, `Session` CRs) through the
exporter's `internal/auth/*` routes.

## Build & run

```sh
go build ./...
docker build -t ghcr.io/telark/auth:<version> .
```

Runs in-cluster via the [telark chart](../../charts/telark); see [INSTALL](../../docs/INSTALL.md)
and [CONTRIBUTING](../../CONTRIBUTING.md).
