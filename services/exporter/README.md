# exporter service

The exporter is Telark's system of record. It owns every Telark custom resource, seeds the
built-in roles, categories and `TelarkConfig`, stores application snapshots and protection plan
reports on its volumes, and serves the REST API the other services use to read and write them.
It is the only stateful service and the only writer of Telark CRs (discovery also patches
`applications/status`).

## Architecture

```mermaid
%%{init: {"theme":"base","themeVariables":{"fontFamily":"ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif","fontSize":"13px","lineColor":"#94a3b8","primaryColor":"#eef2ff","primaryBorderColor":"#6366f1","primaryTextColor":"#312e81","edgeLabelBackground":"#ffffff","clusterBkg":"#f8fafc","clusterBorder":"#e2e8f0"},"flowchart":{"curve":"basis","htmlLabels":true,"nodeSpacing":48,"rankSpacing":62,"padding":12}}}%%
flowchart LR
  subgraph peers["Callers"]
    AUTH(auth)
    DISC(discovery)
    ANA(analyzer)
    NTF(notifier)
  end

  subgraph exporter["exporter"]
    RT(routes) --> H(handlers)
    H --> EX("exporters<br/>snapshot · generics · auth")
    H --> SEED(SeedBuiltins)
    EX --> KC(kcore client)
  end

  K8S[("Kubernetes API<br/>Telark CRDs")]
  PVC[("Snapshots PVC")]
  REDIS[("Redis")]

  AUTH & DISC & ANA & NTF -->|REST| RT
  KC -->|list · get · patch| K8S
  EX -->|manifests| PVC
  H -->|change events| REDIS

  classDef svc fill:#eef2ff,stroke:#6366f1,stroke-width:1.5px,color:#312e81;
  classDef store fill:#fff7ed,stroke:#f59e0b,stroke-width:1.5px,color:#92400e;
  classDef peer fill:#f1f5f9,stroke:#94a3b8,stroke-width:1.5px,color:#334155;
  class RT,H,EX,SEED,KC svc;
  class K8S,PVC,REDIS store;
  class AUTH,DISC,ANA,NTF peer;
```

## Responsibilities

- Own the Telark CRDs (group `telark.io`, see [CRDS.md](../../docs/CRDS.md)) and the OIDC trust Secret, the single service that reads and writes them on the cluster. Reads return a view with `id` = `metadata.name` and `.status` flattened into the top level; writes strip `id` and send status fields to the `/status` subresource of `Application`, `ProtectionPlan` and `TelarkConfig`.
- Seed built-in resources (access roles, categories, the `TelarkConfig` named `default`) on startup.
- Store and serve **snapshots** of workload manifests for audit, comparison, and rollback targets, on a PersistentVolume.
- Store and serve **protection plan reports** (rendered by discovery) and each plan's report ledger on a second PersistentVolume; a reports GC goroutine (own Redis lock key, one replica per tick, shares `SNAPSHOT_GC_INTERVAL_SEC`) sweeps report directories whose plan CR no longer exists.
- Expose the REST surface every other service consumes for CRD operations.
- Store per-user in-app notifications in Redis.

## Layout

| Package | Role |
|---|---|
| `internal/routes` | HTTP route registration (rest router) |
| `internal/handlers/{resources,config,categories,plans,auth,notifications}` | Request handlers per CRD domain |
| `internal/exporters/{snapshot,generics,auth,shared}` | CRD read/write + snapshot serialization against the cluster |
| `internal/exporters/reports` · `internal/handlers/reports` | Report create/list/download, ledger get/put, reports orphan sweep |
| `internal/utils/artifact` | Generic on-volume primitives shared by snapshots and reports: atomic write, path containment, `TickAllowed` (Redis-gated GC tick) |
| `internal/utils/reports` | Reports store on the reports volume (per-plan directory, ledger, retention of 10 on-demand reports) |
| `internal/startup` | `SeedBuiltins` and boot wiring |
| `internal/managers/{envs,certs}` | Env resolution, CA-bundle / TLS material |
| `internal/redis/notifications` | Per-user in-app notifications |
| `internal/cache` · `internal/utils/*` | Compute, concurrency, snapshot helpers |
| `internal/authz` | Per-route authorization requirements |

## Dependencies

- **Internal modules:** `data` (CRD types), `kcore` (dynamic informers / client), `rest` (router + server), `x-ware` (Redis, authz, CORS).
- **Infrastructure:** Kubernetes API (CRD storage), a snapshots **PVC**, Redis.
- **Peers:** none upstream; exporter is the backend the other services depend on.

## Configuration

Full reference: [chart README](../../charts/telark/README.md#servicesexporterenv). Key vars:

| Variable | Default | Description |
|---|---|---|
| `SNAPSHOTS_PATH` | `/snapshots` | Mount path for snapshot files |
| `SNAPSHOTS_PVC_NAME` | `<app.name>-exporter-snapshots-pvc` | PVC backing snapshot storage |
| `SNAPSHOTS_PVC_NAMESPACE` | `<app.namespace>` | Namespace of the PVC |
| `REPORTS_PATH` | `/reports` | Mount path for protection plan report files (the reports PVC) |
| `SNAPSHOT_GC_INTERVAL_SEC` | `3600` | Snapshot sweep interval; also drives the reports orphan sweep (`0` disables both) |
| `EXPORTER_K8S_CLIENT_QPS` / `_BURST` | `50` / `100` | K8s client rate limits, sized for CRD-write fan-out |
| `EXPORTER_LIST_RENDER_CONCURRENCY` | `2` | List renders at once per list route; requests for the same list share one render |
| `CA_BUNDLE` | configmap `<app.name>-ca-bundle` | Trusted CA bundle (`ca.crt`) |
| `OIDC_TRUST_SECRET_NAME` | `telark-oidc-trust-secret` | Secret whose `googleJwkJson` key holds the Google JWK set (chart: `app.auth.oidc.existingSecret` or the chart-managed one) |

## API

REST under `/api/v1/`, one collection per resource with HTTP verbs (no `/get`, `/patch` suffixes):
`applications`, `users`, `groups`, `accessroles`, `config`, `categories` (`?scope=` filters),
`protectionplans`, `reports`, `snapshots`, `notifications` and `auth/sessions`. Service-only routes
live under `internal/` (`internal/users/by-{username,email,identity}`, `internal/reports`,
`internal/protectionplans/{id}/ledger`, `internal/snapshots`, `internal/notifications`,
`internal/auth/users/{userId}/sessions`, `internal/auth/passkeys`), and cleanup views and
finalizers under `cleanup/{type}` (`{type}` is `users`, `groups` or `accessroles`; finalizer writes are
service-only).
`auth/sessions/self` (GET, PATCH internal, DELETE) is the session named by `X-Session-Token` (a raw
token from the UI or a session name from a peer); `DELETE auth/sessions/{name}` revokes another of the
caller's own sessions and answers 404 for anyone else's.
Liveness/readiness at `/api/v1/status/{live,ready}`.

Protection plan create/patch refuse (403) lifecycle, approval and material keys from
session identities; only Internal callers (discovery) may write them: `approvalMode`,
`approval`, `phase`, `renderedPolicies`, `startedAt`, `startedBy`, `terminatedAt`,
`terminatedBy`, `reason`, `health`, `healthCheckedAt`, `healthDetail`, `policies`, `scope` (including its nested
`exclusions`), `mode`, `timeMode`, `timeRange`, `name` (renames go through discovery's `revise`, which keeps
names unique). This keeps approval (`pending_approval`) and phase changes
on discovery's gated routes.

Protection plan routes (`{id}` = plan name). Deny rules are `protection-plans.<action>.deny`
entries a custom role lists; built-in roles list none.

| Route | Authz | Purpose |
|---|---|---|
| `GET protectionplans` | Read, deny `viewprotectionplans` | List plans |
| `GET protectionplans/{id}` | Read, deny `viewprotectionplans` | Read one plan |
| `POST protectionplans` | Write, deny `createprotectionplan` | Create (discovery; sessions cannot send lifecycle or material keys) |
| `PATCH protectionplans/{id}` | Internal | Patch the CR; users edit through discovery, which owns the lifecycle |
| `DELETE protectionplans/{id}` | Internal | Delete the CR; users delete through discovery's `clear`, which removes deployed policies first |

Protection plan reports (`{id}` = plan name):

| Route | Authz | Purpose |
|---|---|---|
| `POST internal/reports` | Internal | Store a rendered report (called by discovery) |
| `PUT internal/protectionplans/{id}/ledger` | Internal | Replace the plan's report ledger |
| `GET internal/protectionplans/{id}/ledger` | Internal | Read the plan's report ledger |
| `GET reports?planId=&trigger=&from=&to=&limit=` | Read (protection plans), deny `viewprotectionplanreports` | List report metadata across all plans, newest first. Optional filters: `planId` (repeatable or comma list), `trigger` (`manual`, `cancel`, `end`), `from`/`to` (RFC3339, on `generatedAt`). `limit` defaults to 200, max 1000; `X-Total-Count` holds the match count before the limit |
| `GET protectionplans/{id}/reports` | Read (protection plans), deny `viewprotectionplanreports` | List the plan's reports |
| `GET protectionplans/{id}/reports/download?report=&format=` | Read (protection plans), deny `downloadprotectionplanreport` | Download one report as `html`, `md`, `json` or `csv` |

Downloads carry `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox`, and are served inline (no `Content-Disposition`). Reports are removed when their plan is deleted.

Environment and tag categories (`plan-environments`, `plan-tags`) follow the `protection-plans`
scope: create needs Contributor (deny `addprotectionplancategory`), edit and delete need Owner
(deny `editprotectionplancategory` / `deleteprotectionplancategory`). For every category scope,
built-in categories cannot be patched or deleted and `type: built-in` cannot be set (400); a name
already used in the scope, compared trimmed and case-insensitively, is refused (409).

Applications and snapshots: `POST applications`, `DELETE applications/{name}`,
`POST internal/snapshots` and `DELETE snapshots/{id}` are Internal (the notifier and discovery; users
delete an application through discovery's `reset`, which also purges its Redis state). Session callers of
`PATCH applications/{name}` may set only `displayName` (at most 200 characters) and
`description` (at most 1000); any other spec key answers 403 and an over-long value 400. Internal callers
keep full access.

Config (`PATCH config`, the `TelarkConfig` named `default`) is checked per field: `excludedNamespaces` and
`userSettings` need settings Contributor (deny `editdiscoveryconfig`), `snapshots` Contributor
(deny `editsnapshotstorage`), `ai` Owner (deny `controlaiinsights`), `oidc` Admin on `ALL` (deny `settings.editoidcconfig`),
and `cluster` is Internal (written by discovery, stored in `.status`). `oidc.googleJwkJson` is written to the
Secret `telark-oidc-trust-secret` (or `app.auth.oidc.existingSecret`), not the CR, and `GET config` merges it
back. A patch naming none of these fields needs settings ReadOnly, like `GET config`, since it answers
with the whole config. A value the CRD schema rejects answers 400 naming the field, for example `spec.ai.model`.

Users, groups and access roles: creating a user with roles, groups or a status applies the same rules as
patching them (users Owner + `attachroletouser`, groups Owner + `addusertogroup`, users Admin +
`suspenduser`); attaching or removing a group's roles needs groups Owner plus `attachroletogroup` /
`removerolefromgroup` (a group create carrying roles is gated like an attach); a role create or patch may not grant a scope level above the caller's own on that
scope (an `ALL` grant counts for every scope). Role protection flags answer 403: `preventModification`
refuses any patch beyond `protection`, and `preventScopeChanges`, `lockName` and `lockCategory` refuse a
change to `scopesAndPermissions`, `name` and `categoryRef`, unless the same patch lifts that flag. The same cap applies to assigning a role: a user create or patch and a group create or
patch answer 403 naming the role and scope when a role being added grants a level above the caller's own
(deny rules on that role do not count; roles already held are not checked). Taking a role away is
capped the same way: removing it from a user, detaching it from a group, and deleting it or a group
carrying it. Adding or removing a group member, from the user or the group side, is capped by every
role the group carries. A role patch touching `status`, `validity` or `scopesAndPermissions` is capped
against the stored and the merged role. Sessions may not set `type: built-in` or change a built-in role's
`protection` (403); a custom role's `protection` is set by its creator at create and changed afterwards
only by its creator or an Admin on ALL (403 otherwise). A role created without `type` is `custom`.
`createdBy` and `lastUpdatedBy` of roles and groups are stamped from the caller; body values are
ignored. Deleting a role with `protection.softDelete` keeps it with `status: Deleted` and `deletedAt`,
so it grants nothing; `preventDeletion` refuses the delete (403). Deny rules are stored lower-cased. `identities` on a user is Internal only, and `email` / `username` are changed
only by the account owner (403 otherwise; resending the stored value is allowed). Internal callers
are exempt.

Request bodies on the user, group, role, category and protection-plan create and patch routes must use
the exact JSON field names: an unknown or differently cased key (`RoleRefs`, `status.Phase`)
answers 400 before any guard or write runs.

Roles and groups on a user are diffed against the stored lists: an addition needs `attachroletouser` /
`addusertogroup`, a removal `removerolefromuser` / `removeuserfromgroup`, each with users or groups Owner.
A group's `userRefs` is gated the same way (groups Owner plus the add or remove rule, never on
oneself). Ids that do not resolve to a live user, group or role answer 400 naming them, and lists are
deduplicated on write.

Group membership is stored on both sides and grants read only the user side, so the exporter keeps them
consistent: a group create or patch that changes `userRefs` updates each affected user's
`groupRefs`, and a user create or patch that changes `groupRefs` updates each group's
member list. The counterparts are written first, one at a time under their own lock, and the caller's
own record last; a failure answers 500 with the record that could not be updated and the caller's
record unchanged, so the same request can be retried (every mirror write is a no-op once applied).
Affected users' cached grants are dropped. On boot, one pass over all users and groups aligns the
group side to the user side, which is what grants: a group gains the users that name it and loses the
ones that don't, and user lists lose duplicates and groups that no longer exist. Nobody's access changes.

Deleting a user (`DELETE users/{id}`) purges the user's sessions at once and, while the
cleanup finalizer still holds the record, the user grants nothing and `GET users/{id}` answers
410, so peers resolving grants over HTTP deny as well. A user, group or role whose `deletionTimestamp`
is set grants nothing anywhere (`deletionTimestamp` is projected into the typed reads and the GET
responses) and refuses every PATCH from a session with 410; the cleanup cascade (service token) still
patches it. Finalizer removal and cleanup views of a record that is already gone answer 404.

A user's `email` is unique like the username (case-insensitive, trimmed): a duplicate answers 409.

Administrators (Admin on `ALL` through an active role, directly or via a live group) and bootstrap
accounts (`spec.bootstrap: true`, written only with the service token; a session sending the field gets
403) are hidden from every caller who is not an administrator or a service: the users list omits them,
`GET` by id, username, email or identity answers 404, group member lists and cleanup views omit their
ids. A group patch keeps the hidden members, and naming one answers 400 like an unknown id; a user
patch of one answers 404 before its body is read. Such callers share one cached, coalesced users list and one
groups list (`RestrictedListKey`, keyed also by the users, groups and roles list generations); their single-record
GETs stay uncached. Bootstrap accounts cannot be
deleted through the API, only they may edit their own record (any other caller gets 403, a group
create or patch adding or removing one included), and a session
may not create or rename a user to the `BOOTSTRAP_ADMIN` email (403). Any Admin on `ALL` may delete or
suspend another administrator, but a user delete, suspension or `roleRefs`/`groupRefs` change, and a
group delete, `roleRefs` change or member removal, that would leave no active user holding Admin on
`ALL` answers 409, whoever calls. Nobody may delete their own account (403).

Snapshot reads (`GET snapshots/{id}` and `/manifest`) mask every `Secret` `data` and `stringData`
value with `[redacted]` for session callers; the stored file and Internal callers (the rollback
controller) keep the real values. Both routes are withheld by `viewapplicationssnapshots` and by
`viewapplicationsnapshotmanifest`.

## Build & run

```sh
go build ./...                 # from the repo root (uses go.work)
docker build -t ghcr.io/telark/exporter:<version> .
```

Runs in-cluster via the [telark chart](../../charts/telark); see [INSTALL](../../docs/INSTALL.md)
and [CONTRIBUTING](../../CONTRIBUTING.md).
