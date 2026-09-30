# Testing and validation

How to build, test and validate Telark from a fresh clone, locally or in a Claude Code cloud session. `.github/workflows/ci.yaml` is the merge gate and the source of truth for versions, test lists and coverage floors; when this page and CI disagree, CI wins.

## What runs where

| Check | Needs | Without a cluster | CI job |
|---|---|---|---|
| Go build, vet, unit tests (per service) | Go | yes | `go`, test leg |
| `golangci-lint` (per service) | golangci-lint | yes | `go`, lint leg |
| Helm lint, `VALUES.md` drift, kubeconform | Helm, Go (for helm-docs), kubeconform, network access to chart repos and schemas | yes | `helm` |
| Analyzer syntax check, pytest, coverage | Python 3.13 | yes | `analyzer` |
| Pod readiness, API calls, UI flows, admission behaviour | a cluster with the chart installed | no | none |

Cloud sessions have no cluster, no registry login and no dashboard UI checkout. Every row except the last runs there; the last row is manual work on a cluster (see [Cluster validation](#cluster-validation)).

## Fresh environment

| Tool | Version | Pinned in |
|---|---|---|
| Go | 1.27.1 | the `go` line of every `services/<svc>/go.mod` (CI: `setup-go` with `go-version-file`) |
| golangci-lint | v2.13.2 | `.github/actions/go-ci/action.yaml` |
| kubeconform | v0.8.0 | `.github/actions/helm-kubeconform-validate/action.yaml` |
| helm-docs | v1.14.2 | `Makefile` (`HELM_DOCS`, run with `go run`; nothing to install) |
| Helm | not pinned; CI installs the latest release (`azure/setup-helm`) | any Helm ≥ 3 with OCI support; checked with v4.3.0 |
| Python | 3.13 | the `analyzer` job in `ci.yaml` |

Linux x86-64:

```sh
# Go. A Go >= 1.21 already on PATH also works: it downloads the go.mod toolchain unless GOTOOLCHAIN=local.
curl -fsSL https://go.dev/dl/go1.27.1.linux-amd64.tar.gz | sudo tar -C /usr/local -xz
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH

# golangci-lint, the version CI pins
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.13.2

# kubeconform, as CI installs it (or: go install github.com/yannh/kubeconform/cmd/kubeconform@v0.8.0)
curl -fsSL https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-linux-amd64.tar.gz | sudo tar -xz -C /usr/local/bin kubeconform

# Helm
curl -fsSL https://get.helm.sh/helm-v4.3.0-linux-amd64.tar.gz | sudo tar -xz -C /usr/local/bin --strip-components=1 linux-amd64/helm
```

Python 3.13: use the image's `python3.13` if there is one, otherwise `uv venv --python 3.13 --seed <dir>` downloads it.

## Shared Go modules

- The shared modules `github.com/telark/{data,rest,kcore,x-ware}` are separate repositories, pinned in each service's `go.mod`. Their repositories are public, and the default `GOPROXY` (`proxy.golang.org`) serves the pinned versions, so fetching them needs no `GOPRIVATE`, token or git credential.
- The committed `go.work` replaces those modules with absolute paths on the maintainer's machine. Anywhere else, every workspace-mode Go command fails with `reading .../go.mod: no such file or directory`.
- So outside the maintainer's machine, run Go commands from `services/<svc>` with `GOWORK=off`, as CI does. `make test` and `make lint` work with `GOWORK=off` exported because they `cd` into each service. `make build`, `make vet` and `make sync` run at the repository root, where only the workspace provides a module, so they fail there.
- Don't edit `go.work`, add `replace` directives or bump module pins ([AGENTS.md](../../AGENTS.md#releases-and-versions)).

**Expected failure.** Service code can use a shared-module symbol that exists only in the maintainer's unreleased local checkout. The workspace build is then green, and `GOWORK=off` fails with `undefined: ...` or `... has no field or method ...` on a shared-module type. The service is not where that gets fixed: the module has to be released and the pin bumped, which is the user's job. Report it and continue with the services that build.

## Commands

### Go services (`auth`, `discovery`, `exporter`, `notifier`)

From `services/<svc>`:

```sh
export GOWORK=off
go build ./...
go vet ./...
go test -race ./...
golangci-lint run          # finds the root .golangci.yml by walking up; never --no-config
```

CI's test leg, with the floor from the `go` job matrix (auth 55, discovery 35, exporter 52, notifier 75):

```sh
GOWORK=off go test -race -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

- Tests live in `internal/tests/<area>/` as separate packages, so only `-coverpkg=./...` coverage means anything.
- The test helpers (`internal/tests/testutil`) use miniredis and an embedded NATS server; no Redis, NATS or cluster is needed.
- golangci-lint refuses to start while another golangci-lint holds its lock (`parallel golangci-lint is running`); lint services one after another.
- The full procedure and what to report: the `go-service-change-gate` skill.

### Helm charts

From the repository root, with `kubeconform` on `PATH`:

```sh
helm repo add vpa https://charts.fairwinds.com/stable   # make deps doesn't add this one; CI does
make deps              # adds the other chart repos, then helm dependency build charts/telark
make helm-lint         # telark-crds (with the app values) and telark
make helm-validate     # renders minimal, standard and performance; kubeconform for Kubernetes 1.30-1.34
make values-docs       # regenerates both VALUES.md; CI fails on drift, so check git diff afterwards
```

- In a fresh Helm config, `make deps` fails with `no repository definition for https://charts.fairwinds.com/stable` until the `vpa` repository is added. CI's list is `.github/actions/helm-add-dependency-repos/action.yaml`.
- `make helm-validate` downloads Kubernetes and CRD schemas (cached in `/tmp/kubeconform-cache`). A pass prints `Invalid: 0, Errors: 0` for every mode and version; the skipped resources are kinds without a published schema.
- Rendering `standard` or `performance` without `app.persistence.storageClass` fails by design (`templates/_storage_guard.tpl`); the make target passes the placeholder `validate`.
- The full procedure and the docs that change with a value: the `helm-chart-change` skill.

### Analyzer (`services/analyzer`)

```sh
cd services/analyzer
VENV="${TMPDIR:-/tmp}/analyzer-venv"      # outside the service: compileall walks every subdirectory
python3.13 -m venv "$VENV"
"$VENV/bin/pip" install -r requirements.txt pytest pytest-cov
"$VENV/bin/python" -m compileall -q .
"$VENV/bin/python" -m pytest --cov=. --cov-report= tests/test_*_cov.py -q
"$VENV/bin/python" -m pytest --cov=. --cov-append --cov-report= tests/test_authz.py tests/test_insights.py tests/test_exporter.py -q
"$VENV/bin/python" -m coverage report --fail-under=95
```

The stub-based `test_*_cov.py` suites replace `sys.modules` entries, so they run in their own process, apart from the real-dependency suites listed explicitly in `ci.yaml`. No Ollama, Redis or cluster is needed. The procedure: the `analyzer-ci-gate` skill.

### Everything at once

```sh
export GOWORK=off PATH="$HOME/go/bin:$PATH"
make check             # golangci-lint and go test for the four services, plus make helm-lint (after make deps)
make helm-validate
```

`make check` skips the race detector, the coverage floors, the analyzer and `values-docs`; use the per-area commands for CI parity.

## Cluster validation

This needs a Kubernetes cluster (≥ 1.30) with the chart installed, which a cloud session doesn't have. Installing and reaching the dashboard: [INSTALL.md](../INSTALL.md). The maintainer's build-and-deploy loop is the `deploy-dev-cluster` skill; the `scripts/local-*.sh` helpers it calls are git-ignored and exist only on the maintainer's machine.

### Smoke test

```sh
kubectl -n telark get pods
```

Every pod should be `Running` and `READY`: every service with a health check (all but the UI) is ready only once `/api/v1/status/ready` answers.

### Calling the APIs

In the cluster every service listens on port 8080 (`app.serviceDefaults.port`) under `/api/v1/`. The local port-forward ports are in `charts/telark/values.dev.yaml` (exporter 8002, discovery 8004, auth 8006, analyzer 8007):

```sh
kubectl -n telark port-forward svc/telark-discovery-service 8004:8080 &
curl -fsS localhost:8004/api/v1/status/ready
curl -fsS -H "X-Session-Token: $TOKEN" localhost:8004/api/v1/<route>
```

- A user request carries its session token in the `X-Session-Token` header, never in a cookie or the query string. Sign in through the UI (passkeys need `https://` or `http://localhost`) and copy the header from any API request in the browser's network panel.
- Route paths: `services/{auth,discovery,exporter}/internal/routes/routes.go` with the path constants in the `rest` module's `endpoints/` packages, and `services/analyzer/api_server.py`. Each Go service's access rule per route is in `internal/authz/requirements.go`. Send the opaque ids the UI sends, not display names.
- A 403 for a user who looks authorized usually means a deny rule or a missing grant, not a bug; see [Authorization](../security/README.md#authorization).

### End-to-end: UI → API → backend → Kubernetes

A protection plan crosses every layer. After each step, check the layer below instead of trusting the status code:

1. **UI → discovery.** Create and start a plan from the UI, or through the discovery plan routes with a session token. Expect a 2xx and the plan in the list.
2. **discovery → exporter → CRD.** `kubectl -n telark get protectionplans.telark.io -o yaml` shows the plan, its fields under `spec` and its phase under `.status`. A field accepted by the API but missing here was pruned by the CRD schema ([AGENTS.md](../../AGENTS.md#go-services), CRDs).
3. **discovery → Kyverno.** While the plan is active, `kubectl get policies.kyverno.io -A -l telark.io/protection-plan=<plan-id>` lists its namespaced Kyverno `Policy` objects.
4. **Admission.** Make a change one of the plan's templates blocks and expect the API server to reject it when the plan enforces (audit mode only records it). Violations are read from Kubernetes Events, which expire; see [Protection plans](../architecture/protection-plans.md).
5. **Terminate or cancel.** The label query from step 3 returns nothing, and the finished plan reports zero violations, by design.

Other flows follow the same pattern: applications land in `applications.telark.io`; users, groups and access roles in `users.telark.io`, `groups.telark.io` and `accessroles.telark.io` (`kubectl get telark -n telark` lists them all); analyzer insights in Redis ([architecture](../architecture/README.md)). Judge latency from inside the cluster, because `kubectl` exec-auth from a laptop adds seconds to every call.

### Dashboard UI

The UI is the separate `telark/dashboard-ui` repository (stable branch `main`); this repository only references its image (`services.ui` in `charts/telark/values.yaml`). UI checks run in that repository.
