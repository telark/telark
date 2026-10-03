# telark: Claude Code entry point

The rules for every coding agent are in AGENTS.md, imported below; follow them. This file is only the map: what telark is, where things live, and the few facts you need before touching code. The linked docs hold the detail, and the code wins over any doc.

@AGENTS.md

## What telark is

A control plane for **protection plans** over Kubernetes workloads. It groups workloads into applications, binds policy templates to a scope (applications or namespaces) and a time window, enforces them at admission through Kyverno, verifies them against live cluster state, and records change history with rollback. It installs as one Helm chart (`charts/telark`) with its CRDs (`charts/telark-crds`).

## Services

| Service | Lang | Role | Reads first |
|---|---|---|---|
| `exporter` | Go | Owns every telark CRD and the snapshot and report volumes; seeds built-in access roles, categories and the `TelarkConfig`; the only stateful service | `services/exporter/README.md` |
| `discovery` | Go | Groups workloads into applications, leader-elected reconcile loop, change history and rollback, protection-plan lifecycle, approvals, violations, reports, Kyverno policies | `services/discovery/README.md` |
| `auth` | Go | Passkey (WebAuthn) and Google OIDC login, sessions, user/group/role deletion cleanup | `services/auth/README.md`, `OIDC.md` |
| `notifier` | Go | Consumes the `telark.applications.*` JetStream events discovery publishes: upserts `Application` through the exporter, and on a delete calls discovery's application reset | `services/notifier/README.md` |
| `analyzer` | Python/FastAPI | Local incident analysis with an open-weight model (Ollama) over read-only cluster tools; insights in Redis; SSE to the UI | `services/analyzer/README.md`, `ARCHITECTURE.md` |
| `ui` | (separate repo) | Dashboard SPA, `telark/dashboard-ui`; this repo ships only its image reference | none |

Shared Go packages `internal/{data,rest,kcore,x-ware}` live in the same module as the services (`github.com/telark/telark`, one root `go.mod`): `data` holds CRD types, constants and Kyverno policy rendering, `rest` the HTTP clients, endpoints and router, `kcore` Kubernetes client helpers, `x-ware` the authorization middleware, CORS, Redis and NATS helpers. Subcharts: Redis, NATS JetStream, Kyverno, metrics-server, VPA (optional), Ollama.

Detail: [docs/architecture/](docs/architecture/README.md).

## How it fits together

- Everything that persists goes through the **exporter's HTTP API** (`rest` clients, `/api/v1/...`, port 8080). Only the exporter writes telark CRDs; discovery also patches `applications/status`.
- **discovery** watches the cluster, publishes application events to NATS, stores snapshots and history through the exporter, pushes analysis jobs to the Redis stream `insights:jobs`, and creates and deletes the Kyverno `Policy` objects of active plans.
- **auth** produces sessions; every other API service validates them through the same `x-ware` middleware (the analyzer asks auth).
- Redis is shared coordination and cache (leader locks, queues, dedup, insights, authz cache). It requires a password (chart-generated Secret, `REDIS_PASSWORD`) but stays **untrusted**, since every service holds it: authorization cache entries are HMAC-signed.

## Security model (essentials)

Full model, RBAC table and invariants: [docs/security/](docs/security/README.md).

- **Users** send `X-Session-Token`. The token is 32 random bytes; only `session-<sha256>` is stored (a `Session` CR), and it never appears in a URL.
- **Services** send `X-Service-Token` (`TELARK_SERVICE_TOKEN`, one chart-generated Secret). A valid service token is Internal and skips grant checks, so it is equivalent to full API access.
- **Every route** has an explicit requirement in `services/<svc>/internal/authz/requirements.go` (Public, Authenticated, Internal, or a level on a scope, optionally with a deny rule). Unmapped routes are denied.
- **Grants** (`x-ware/authz`): levels ReadOnly < Contributor < Owner < Admin per scope, `ALL` covers every scope; roles come from the user and the user's groups; only active users and active, unexpired roles count; **deny rules beat any level**.
- **Guards** in handlers limit what a caller may change (for example, you can't grant a role above your own level or edit your own roles), in `services/exporter/internal/authz/` and `services/auth/internal/authz/guard.go`.
- **Kubernetes**: each service has its own ServiceAccount and ClusterRole; discovery has wide write access for rollback and plan policies, the analyzer is read-only, auth, notifier and ui have none. The optional CRD write guard (`app.crdGuard`) restricts direct CRD writes.

## Kubernetes objects

- One CRD group, `telark.io` `v1alpha1`: `Application`, `ProtectionPlan`, `TelarkConfig` (these three with a status subresource), `Category`, `User`, `Group`, `AccessRole`, `Passkey`, `Session`; all namespaced in the release namespace. The object name is the identity (no `spec.id`). Use FQ names (`applications.telark.io`) or short names (`tapp`). Reference: [docs/CRDS.md](docs/CRDS.md).
- The group `telark.io` is constant; `app.name` (default `telark`) only prefixes object names ([ADR 0003](docs/adr/0003-constant-api-group-telark-io.md)).
- **CRD schema first**: a Go field not declared in `charts/telark-crds/templates/crds/` is pruned silently on write.

## Protection plans

Phases `draft`, `pending_approval`, `scheduled`, `active`, `terminated`, `canceled`, `failed` (`data/plans/protectionplan.go`). discovery drives the lifecycle; approval is `automatic` or `required` (Production defaults to required). While active, discovery renders the plan's templates into namespaced Kyverno `Policy` objects labeled `telark.io/protection-plan=<id>`, in `audit` or `enforce` mode, minus scope exclusions, and checks their health against the cluster. Violations come from Kubernetes Events; terminate and cancel delete the policies, and a finished plan then reports zero violations. Detail: [docs/architecture/protection-plans.md](docs/architecture/protection-plans.md).

## Build, test, validate

All Go commands run from the repository root; there is no `go.work` and no `replace` directive, so every machine and CI build the same way.

```sh
go build ./... && go vet ./... && go test -race ./... && golangci-lint run ./services/<svc>/...   # lint one service or ./internal/<pkg>/... at a time
helm repo add vpa https://charts.fairwinds.com/stable && make deps && make helm-lint && make helm-validate && make values-docs
# analyzer: see docs/testing (Python 3.13 venv, two pytest runs, coverage >= 95)
```

Tool versions (Go 1.27.1, golangci-lint v2.14.0, kubeconform v0.8.0, Python 3.13), install commands, CI coverage floors and cluster validation (UI → API → backend → Kubernetes): [docs/testing/](docs/testing/README.md). No cluster is available in a cloud session; unit tests, lint, Helm lint and render, kubeconform and the analyzer suite all run without one.

## Before you call something done

- Run the skill for what you touched: `go-service-change-gate`, `helm-chart-change`, `analyzer-ci-gate` (`.claude/skills/`).
- Docs change in the same diff as the behavior, including `docs/security/` when you touch authentication, authorization, RBAC or the service token.
- No commits, pushes, tags, version bumps, image builds or cluster changes unless asked (AGENTS.md, Releases and versions).

## Docs map

| Need | Doc |
|---|---|
| Rules, conventions, skills index | [AGENTS.md](AGENTS.md), [CONVENTIONS.md](CONVENTIONS.md) |
| Services, flows, Redis/NATS, startup | [docs/architecture/](docs/architecture/README.md), [docs/architecture.md](docs/architecture.md) |
| Protection plans end to end | [docs/architecture/protection-plans.md](docs/architecture/protection-plans.md) |
| Auth, authz, RBAC, invariants | [docs/security/](docs/security/README.md), [SECURITY.md](SECURITY.md) |
| Build, test, CI parity, cluster checks | [docs/testing/](docs/testing/README.md), [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) |
| Install and chart values | [docs/INSTALL.md](docs/INSTALL.md), [charts/telark/README.md](charts/telark/README.md) |
| CRDs | [docs/CRDS.md](docs/CRDS.md) |
