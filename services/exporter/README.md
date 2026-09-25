# exporter service

The platform's system-of-record. Exporter owns every telark custom resource, seeds
the built-ins, snapshots live cluster state to disk, and serves the REST API that all
other services call to read and write CRDs. It is the **only stateful service** — it
holds the snapshots PVC and is the single writer of Telark CRs on the cluster.

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

- Own the Telark CRDs (groups `erpi.*`, `auth.*`, `classification.*`) — the single service that reads and writes them on the cluster.
- Seed built-in resources (roles, categories, default `GlobalConfig`) on startup.
- Store and serve **snapshots** of workload manifests for audit, comparison, and rollback targets, on a PersistentVolume.
- Store and serve **protection plan reports** (rendered by discovery) and each plan's report ledger on a second PersistentVolume; a reports GC goroutine (own Redis lock key, one replica per tick, shares `SNAPSHOT_GC_INTERVAL_SEC`) sweeps report directories whose plan CR no longer exists.
- Expose the REST surface every other service consumes for CRD operations.
- Emit change notifications on Redis for downstream consumers.

## Layout

| Package | Role |
|---|---|
| `internal/routes` | HTTP route registration (rest router) |
| `internal/handlers/{resources,plans,classification,auth,notifications}` | Request handlers per CRD domain |
| `internal/exporters/{snapshot,generics,auth,shared}` | CRD read/write + snapshot serialization against the cluster |
| `internal/exporters/reports` · `internal/handlers/reports` | Report create/list/download, ledger get/put, reports orphan sweep |
| `internal/utils/artifact` | Generic on-volume primitives shared by snapshots and reports: atomic write, path containment, `TickAllowed` (Redis-gated GC tick) |
| `internal/utils/reports` | Reports store on the reports volume (per-plan directory, ledger, retention of 10 on-demand reports) |
| `internal/startup` | `SeedBuiltins` and boot wiring |
| `internal/managers/{envs,certs}` | Env resolution, CA-bundle / TLS material |
| `internal/redis/notifications` | Change-event publishing |
| `internal/cache` · `internal/utils/*` | Compute, concurrency, snapshot helpers |
| `internal/authz` | Per-route authorization requirements |

## Dependencies

- **Internal modules:** `data` (CRD types), `kcore` (dynamic informers / client), `rest` (router + server), `x-ware` (Redis, authz, CORS).
- **Infrastructure:** Kubernetes API (CRD storage), a snapshots **PVC**, Redis.
- **Peers:** none upstream — exporter is the backend the other services depend on.

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
| `CA_BUNDLE` | configmap `<app.name>-ca-bundle` | Trusted CA bundle (`ca.crt`) |

## API

REST under `/api/v1/` — CRD operations grouped by domain: `resources/*` (applications,
groups, roles, users, globalconfig), `plans/*` (protection plans), `classification/*`,
`auth/*` (sessions, passkeys), `snapshots/*`, and `notifications/*`. Liveness/readiness
at `/api/v1/status/{live,ready}`.

Protection plan create/patch refuse (403) lifecycle, approval and material keys from
session identities; only Internal callers (discovery) may write them: `approvalMode`,
`approval`, `phase`, `renderedPolicies`, `startedAt`, `startedBy`, `terminatedAt`,
`terminatedBy`, `reason`, `health`, `healthCheckedAt`, `healthDetail`, `policies`, `scope` (including its nested
`exclusions`), `mode`, `timeMode`, `timeRange`. This keeps approval (`pending_approval`) and phase changes
on discovery's gated routes.

Protection plan routes (`{id}` = plan name). Deny rules are `protection-plans.<action>.deny`
entries a custom role lists; built-in roles list none.

| Route | Authz | Purpose |
|---|---|---|
| `GET plans/protection/get` | Read, deny `viewprotectionplans` | List plans |
| `GET plans/protection/{id}/get` | Read, deny `viewprotectionplans` | Read one plan |
| `POST plans/protection/create` | Write, deny `createprotectionplan` | Create (discovery; sessions cannot send lifecycle or material keys) |
| `PATCH plans/protection/{id}/patch` | Write, deny `editprotectionplan` | Patch (sessions: metadata keys only) |
| `DELETE plans/protection/{id}/delete` | Internal | Delete the CR; users delete through discovery's `clear`, which removes deployed policies first |

Protection plan reports (`{id}` = plan name):

| Route | Authz | Purpose |
|---|---|---|
| `POST reports/plans/create` | Internal | Store a rendered report (called by discovery) |
| `POST reports/plans/{id}/ledger/put` | Internal | Replace the plan's report ledger |
| `GET reports/plans/{id}/ledger/get` | Internal | Read the plan's report ledger |
| `GET reports/get?planId=&trigger=&from=&to=&limit=` | Read (protection plans), deny `viewprotectionplanreports` | List report metadata across all plans, newest first. Optional filters: `planId` (repeatable or comma list), `trigger` (`manual`, `cancel`, `end`), `from`/`to` (RFC3339, on `generatedAt`). `limit` defaults to 200, max 1000; `X-Total-Count` holds the match count before the limit |
| `GET reports/plans/{id}/get` | Read (protection plans), deny `viewprotectionplanreports` | List the plan's reports |
| `GET reports/plans/{id}/download?report=&format=` | Read (protection plans), deny `downloadprotectionplanreport` | Download one report as `html`, `md`, `json` or `csv` |

Downloads carry `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox`, and are served inline (no `Content-Disposition`). Reports are removed when their plan is deleted.

Environment and tag categories (`plan-environments`, `plan-tags`) follow the `protection-plans`
scope: create needs Contributor (deny `addprotectionplancategory`), edit and delete need Owner
(deny `editprotectionplancategory` / `deleteprotectionplancategory`).

## Build & run

```sh
go build ./...                 # from the repo root (uses go.work)
docker build -t telark/exporter:<version> .
```

Runs in-cluster via the [telark chart](../../charts/telark); see [INSTALL](../../docs/INSTALL.md)
and [CONTRIBUTING](../../CONTRIBUTING.md).
