# Architecture: components and flows

How the services depend on each other and how data and control move between them. The one-page overview with the system diagram is [../architecture.md](../architecture.md); each service README has its internals; this page is the cross-service map. Paths starting `data/`, `rest/`, `kcore/` or `x-ware/` are in the shared Go packages under `internal/`.

## Components

| Component | Runtime | State it owns | Calls |
|---|---|---|---|
| exporter | Go | every Telark CR (through the Kubernetes API), snapshot and report volumes, in-app notifications and the authz cache in Redis | Kubernetes API only; no other service |
| discovery | Go | Redis coordination keys, insights row index (in memory) | exporter; Kubernetes API (informers, rollback writes, Kyverno policies); NATS publish; Redis |
| auth | Go | WebAuthn challenges, OIDC nonces and JWKS, enrollment tokens, cleanup streams (Redis DB 1) | exporter; Google (optional); Redis |
| notifier | Go | none | exporter, discovery (reset); NATS consume; Redis heartbeat |
| analyzer | Python | insights in Redis (`analyzer:*`) | auth (permissions), exporter (read), Kubernetes API (read), Ollama, Redis |
| ui | nginx + SPA (separate repo) | none | the service APIs |
| Redis | subchart, no auth | coordination, queues, caches | |
| NATS JetStream | subchart, password auth | stream `telark_applications` | |
| Kyverno | subchart | admission webhooks | |
| Ollama | subchart | model files | |

Shared modules: `data` (CRD types, constants, errors, plan templates and Kyverno policy rendering in `data/policies`), `rest` (HTTP clients, endpoint paths, router, server), `kcore` (informers, dynamic client, workload kinds), `x-ware` (authz middleware, CORS, Redis streams, election and locks, NATS).

## Service-to-service calls

- Every `rest` client targets `http://telark-<service>-service:8080/api/v1/<endpoint>` (`rest/base/def.go`); the analyzer's defaults are the same host names (`services/analyzer/constants.py`). The chart passes no service URLs, so the service names are fixed by `app.name`.
- All `rest` clients call the exporter, except `ResetApplicationByName`, which calls discovery (`rest/clients/applications/client.go`).
- Every client sends the service token (`X-Service-Token`), so service calls are Internal ([security](../security/README.md#service-token)).

```
auth      -> exporter   sessions, passkeys, users, groups, access roles, config
discovery -> exporter   applications, snapshots, plans, reports and ledgers, notifications, config, sessions/users/groups/access roles (authz)
notifier  -> exporter   application patch / create
notifier  -> discovery  application reset
analyzer  -> auth       GET /api/v1/auth/permissions (every user request)
analyzer  -> exporter   config, applications, plans, categories (read)
```

discovery reaches the exporter through the wrappers in `services/discovery/internal/clients/*.go` (timeouts, circuit breaker).

## Messaging and shared state

**NATS JetStream.** discovery publishes `telark.applications.update` (`services/discovery/internal/publisher/base.go`). notifier consumes `telark.applications.{create,update,delete}` from stream `telark_applications` (work-queue retention, file storage, 24 h max age) with durable pull consumers (`x-ware/nats/streams`). No code publishes `create` or `delete` today; application deletes go through discovery's reset (auto-cleanup calls it in process).

**Redis** (auth uses DB 1 via `REDIS_DB`; the others the default DB):

| Owner | Keys | Purpose |
|---|---|---|
| discovery | `election:prewarm` | leader lease, `SET NX` with 15 s TTL renewed every 5 s (`x-ware/redis/stream/election.go`) |
| discovery | `streams:discovery:operations` (group `telark-discovery-consumer-group`), `lock:app:*`, `dedup:*`, `replica:*` | leader enqueues every application each cycle; workers take a per-app lock ([discovery README](../../services/discovery/README.md#distributed-coordination)) |
| discovery | `forcesync:*`, `lock:plan-name:*`, `lock:plan-decision:*`, `lock:reports:ledger:*`, `lock:rollback:*` | force sync queue and per-object locks |
| discovery | `coalesce:buf:<app>` (5 min TTL) | the app's coalesced informer pre-images and flush deadline, so a new leader finishes the flush; the prewarm tick defers while it exists |
| discovery | `coalesce:held:<app>` (5 min TTL) | pre-images of a flush that found no inputs (every workload deleted), taken once by the app's next event, on this leader or the next |
| discovery | `history:floor:<app>` (10 min TTL) | last generation the leader published; a stored copy behind it is stale |
| discovery | `history:recorded:<app>` (hash, 24 h TTL, refreshed by the reconcile tick) | per resource, the fingerprint of the object the last flush diffed against |
| discovery | `history:post:<app>` (hash, same TTL as `history:recorded:`) | compared roots of each resource the last flush changed, and the `generation` whose flush wrote them |
| discovery | `snap:pending:<app>` (10 min TTL) | pre-image set written for a generation the store has not recorded yet; a retried flush reuses or reclaims it |
| discovery | `history:deferred:<app>` (10 min TTL) | `<fingerprint>:<ticks>` of a change the tick saw without an informer pre-image; on the second consecutive tick it is recorded against the live state |
| discovery → analyzer | `insights:jobs` | analysis jobs (consumer group `analyzer`) |
| analyzer → discovery | `analyzer:<ns>:<name>` (7-day TTL), `analyzer:index` (ZSET) | insight documents and the index discovery reads for its insight lists |
| exporter | `notif:user:*`, `notif:item:*` | per-user in-app notifications |
| exporter | authz generation and signed grant entries | grants cache ([security](../security/README.md#where-each-service-resolves-sessions-and-grants)) |
| exporter | `exporter:snapshot:gc`, `exporter:reports:gc`, list-cache generations | GC tick locks, list cache |
| auth | `auth:webauthn:challenge:*`, `auth:oidc:nonce:*`, `auth:oidc:jwks:google`, `auth:passkey:enroll-token:*`, `auth:passkey:invite:*`, `auth:passkey:invite-of:*`, `auth:cleanup:<users\|groups\|accessroles>` | login ceremonies, enrollment links (keyed by the token's SHA-256 digest, never the token) and the deletion cleanup streams |

Constants: `services/<svc>/internal/constants/` (discovery `coordination.go`, `forcesync.go`, `informers.go`; auth `config.go`; exporter `config.go`, `authz.go`), `services/analyzer/constants.py`.

## Kubernetes access

- discovery runs `kcore` informers over the application workload kinds on every replica; the coalesced flush and the leader loops (plan controller, report checkpoint, rollback controller) run on the leader only (`services/discovery/internal/coordination/leadergate/`).
- The exporter mirrors `Application` and `Session` CRs with informers and writes every Telark CR. discovery also patches `applications/status` directly, from the rollback controller (`services/discovery/internal/handlers/rollback/controller.go`).
- The analyzer issues read-only `GET`s with its ServiceAccount token.
- auth and notifier never call the Kubernetes API.
- RBAC per service: [security](../security/README.md#kubernetes-privileges).

## Flows

**Application discovery.** discovery derives applications from workload labels (`services/discovery/internal/discovery/derivation/`), merges them across namespaces, diffs against the stored application, and publishes `telark.applications.update`. notifier patches the `Application` through the exporter and creates it on 404. Authored incident and recovery changes also `XADD` a job to `insights:jobs`.

**Change history, snapshots and rollback.** On a material change discovery stores sanitized pre-change manifests through the exporter, which writes them to the snapshots volume (`/snapshots/apps/<app>/<ns>/V<generation>.json`, `services/exporter/internal/utils/snapshot/paths.go`). A rollback request appends an intent through the exporter; the leader's rollback controller fetches the snapshot, writes the objects back (create, or update to replace; `services/discovery/internal/handlers/rollback/replace.go`) and patches the rollback status on the CR ([discovery README](../../services/discovery/README.md#rollback)).

**Application delete.** discovery's reset (`POST .../applications/{name}/reset`, from auto-cleanup or the API) clears the app's Redis state and deletes the CR through the exporter, which deletes its snapshot files.

**Insights.** The analyzer consumes `insights:jobs`, investigates with the local model and read-only tools, writes `analyzer:<ns>:<name>` and indexes it, and streams updates over SSE (`GET /api/v1/insights/events`, in-process broadcaster). The UI reads insight lists through discovery (`insights/applications`, `insights`) and asks the analyzer for analyze, triage, runtime and events. Details: [analyzer ARCHITECTURE.md](../../services/analyzer/ARCHITECTURE.md).

**Protection plans.** [protection-plans.md](protection-plans.md).

**Sign-in.** The UI calls auth; auth verifies the passkey assertion or Google ID token, then creates the `Session` through the exporter and returns the token. Every later API call carries `X-Session-Token`; each service resolves it as described in [security](../security/README.md#authentication).

**User, group and access-role deletion.** auth's `DELETE auth/{users,groups,accessroles}/{id}` route runs the deletion guard, deletes through the exporter (a finalizer keeps the record), and queues a job on `auth:cleanup:<kind>`. The cleanup reconciler deletes a deleted user's sessions, strips back-references from other records, then removes the finalizer (`services/auth/internal/controllers/cleanup/`, [auth README](../../services/auth/README.md)). The exporter's delete already strips group membership on the other side, so for membership the reconciler is the backstop.

## Startup and health

| Service | Order in `main` | Probes (`/api/v1/status/...`) |
|---|---|---|
| exporter | snapshot and report directories → Redis → seed built-ins (access roles upserted, categories merged, `TelarkConfig` `default` only if absent) → GC loops → application and session informers → authz → HTTP | `live`, `ready` (Redis and the application mirror synced) |
| discovery | authz → HTTP server with a self-supervisor → async bootstrap: Redis, consumer group, informers, insights index, leader loop, consumer, force sync, rollback, plans, auto-cleanup | `live`, `ready` (fails only on Redis; bootstrap, exporter breaker and Kubernetes report degraded) |
| auth | subcommand dispatch → config and bootstrap-admin check → WebAuthn → Redis → cleanup system → authz → HTTP | `health`, `ready`, `live` |
| notifier | Redis → NATS (creates the streams) → status server | `live`, `ready` (NATS connected) |
| analyzer | uvicorn; lifespan starts the config poll, the job worker and the review loop | `live`, `ready` (Redis ping) |

All listen on 8080 in the cluster.

## Where older docs differ from the code

Checked against the code; the service READMEs are fixed separately:

- The discovery and notifier READMEs describe a `telark.applications.delete` NATS flow; nothing publishes it.
