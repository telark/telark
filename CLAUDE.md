# telark: Claude Code entry point

The rules for every coding agent are in AGENTS.md, imported below; follow them. This file is only the map: what telark is, where things live, and the few facts you need before touching code. The linked docs hold the detail, and the code wins over any doc.

@AGENTS.md

## What telark is

A control plane for **protection plans** over Kubernetes workloads. It groups workloads into applications, binds policy templates to a scope (applications or namespaces) and a time window, enforces them at admission through Kyverno, verifies them against live cluster state, and records change history with rollback. It installs as one Helm chart (`charts/telark`) with its CRDs (`charts/telark-crds`).

## Services

| Service | Lang | Role | Reads first |
|---|---|---|---|
| `exporter` | Go | Owns every telark CRD and the snapshot and report volumes; seeds built-in roles, categories and global config; the only stateful service | `services/exporter/README.md` |
| `discovery` | Go | Groups workloads into applications, leader-elected reconcile loop, change history and rollback, protection-plan lifecycle, approvals, violations, reports, Kyverno policies | `services/discovery/README.md` |
| `auth` | Go | Passkey (WebAuthn) and Google OIDC login, sessions, user/group/role deletion cleanup | `services/auth/README.md`, `OIDC.md` |
| `notifier` | Go | Consumes the `telark.applications.*` JetStream events discovery publishes: upserts `ApplicationAsResource` through the exporter, and on a delete calls discovery's application reset | `services/notifier/README.md` |
| `analyzer` | Python/FastAPI | Local incident analysis with an open-weight model (Ollama) over read-only cluster tools; insights in Redis; SSE to the UI | `services/analyzer/README.md`, `ARCHITECTURE.md` |
| `ui` | (separate repo) | Dashboard SPA, `telark/dashboard-ui`; this repo ships only its image reference | none |

Shared Go modules `github.com/telark/{data,rest,kcore,x-ware}` are separate repositories pinned in each `go.mod`: `data` holds CRD types, constants and Kyverno policy rendering, `rest` the HTTP clients, endpoints and router, `kcore` Kubernetes client helpers, `x-ware` the authorization middleware, CORS, Redis and NATS helpers. Subcharts: Redis, NATS JetStream, Kyverno, metrics-server, VPA (optional), Ollama.

Detail: [docs/architecture/](docs/architecture/README.md).

## How it fits together

- Everything that persists goes through the **exporter's HTTP API** (`rest` clients, `/api/v1/...`, port 8080). Only the exporter writes telark CRDs; discovery also patches `applicationsasresources`.
- **discovery** watches the cluster, publishes application events to NATS, stores snapshots and history through the exporter, pushes analysis jobs to the Redis stream `insights:jobs`, and creates and deletes the Kyverno `Policy` objects of active plans.
- **auth** produces sessions; every other API service validates them through the same `x-ware` middleware (the analyzer asks auth).
- Redis is shared coordination and cache (leader locks, queues, dedup, insights, authz cache) and is **untrusted**: no auth, so authorization cache entries are HMAC-signed.

## Security model (essentials)

Full model, RBAC table and invariants: [docs/security/](docs/security/README.md).

- **Users** send `X-Session-Token`. The token is 32 random bytes; only `session-<sha256>` is stored (a `UserSession` CR), and it never appears in a URL.
- **Services** send `X-Service-Token` (`TELARK_SERVICE_TOKEN`, one chart-generated Secret). A valid service token is Internal and skips grant checks, so it is equivalent to full API access.
- **Every route** has an explicit requirement in `services/<svc>/internal/authz/requirements.go` (Public, Authenticated, Internal, or a level on a scope, optionally with a deny rule). Unmapped routes are denied.
- **Grants** (`x-ware/authz`): levels ReadOnly < Contributor < Owner < Admin per scope, `ALL` covers every scope; roles come from the user and the user's groups; only active users and active, unexpired roles count; **deny rules beat any level**.
- **Guards** in handlers limit what a caller may change (for example, you can't grant a role above your own level or edit your own roles), in `services/exporter/internal/authz/` and `services/auth/internal/authz/guard.go`.
- **Kubernetes**: each service has its own ServiceAccount and ClusterRole; discovery has wide write access for rollback and plan policies, the analyzer is read-only, auth, notifier and ui have none. The optional CRD write guard (`app.crdGuard`) restricts direct CRD writes.

## Kubernetes objects

- CRD groups `erpi.telark` (applications, users, groups, roles, global config, protection plans), `auth.telark` (passkeys, sessions), `classification.telark` (categories); all namespaced in the release namespace. Reference: [docs/CRDS.md](docs/CRDS.md).
- `app.name` (default `telark`) is the identity in names and API groups ([ADR 0002](docs/adr/0002-app-name-is-the-identity-source-of-truth.md)); the Go services hardcode the `telark` groups.
- **CRD schema first**: a Go field not declared in `charts/telark-crds/templates/crds/` is pruned silently on write.

## Protection plans

Phases `draft`, `pending_approval`, `scheduled`, `active`, `terminated`, `canceled`, `failed` (`data/plans/protectionplan.go`). discovery drives the lifecycle; approval is `automatic` or `required` (Production defaults to required). While active, discovery renders the plan's templates into namespaced Kyverno `Policy` objects labelled `telark.erpi/protection-plan=<id>`, in `audit` or `enforce` mode, minus scope exclusions, and checks their health against the cluster. Violations come from Kubernetes Events; terminate and cancel delete the policies, and a finished plan then reports zero violations. Detail: [docs/architecture/protection-plans.md](docs/architecture/protection-plans.md).

## Build, test, validate

Cloud sessions and any machine other than the maintainer's: **`export GOWORK=off`** first. The committed `go.work` points the shared modules at the maintainer's local paths, so workspace-mode Go commands fail elsewhere. The shared modules download from the public Go proxy without credentials.

```sh
export GOWORK=off
cd services/<svc> && go build ./... && go vet ./... && go test -race ./... && golangci-lint run
helm repo add vpa https://charts.fairwinds.com/stable && make deps && make helm-lint && make helm-validate && make values-docs
# analyzer: see docs/testing (Python 3.13 venv, two pytest runs, coverage >= 95)
```

Tool versions (Go 1.27.1, golangci-lint v2.13.2, kubeconform v0.8.0, Python 3.13), install commands, CI coverage floors and cluster validation (UI → API → backend → Kubernetes): [docs/testing/](docs/testing/README.md). No cluster is available in a cloud session; unit tests, lint, Helm lint and render, kubeconform and the analyzer suite all run without one.

`GOWORK=off` can fail with `undefined` shared-module symbols when a service uses an unreleased local change of a shared module. That is expected until the module is released and the pin bumped (the user's job); report it, don't add `replace` directives or edit `go.work`.

## Before you call something done

- Run the skill for what you touched: `go-service-change-gate`, `helm-chart-change`, `analyzer-ci-gate` (`.claude/skills/`).
- Docs change in the same diff as the behaviour, including `docs/security/` when you touch authentication, authorization, RBAC or the service token.
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
| In-flight feature plans | `.claude/plans/` |
