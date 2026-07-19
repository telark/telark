# Contributing

Thanks for contributing to telark.

## Repository layout

```
services/
  auth  discovery  exporter  notifier   Go services (in the go.work workspace)
  enrichment                            Python / FastAPI service
charts/
  telark        application chart
  telark-crds   custom resource definitions (install first)
.github/        CI and release workflows
docs/           architecture, install guide, CRD reference, ADRs
```

The Go services depend on shared packages published under `github.com/telark/*` (`data`, `rest`, `x-ware`, `kcore`) — ordinary module dependencies, pinned in each service's `go.mod`.

## Prerequisites

- Go 1.26+ (see `go.work` for the exact version), `golangci-lint`, `helm` ≥ 3.
- Python 3.13+ + `venv` for the enrichment service.
- Docker + a Kubernetes cluster for end-to-end work.

## Go workspace

The four Go services live in a single `go.work`; it holds only those four. They build against the published `github.com/telark/*` modules at the versions pinned in each `go.mod` — there are no `replace` directives.

**Set `GOPRIVATE` before any module operation** — the internal packages are private until release:

```sh
export GOPRIVATE=github.com/telark/*
```

Build the whole workspace rather than tidying a single service — a standalone `go mod tidy` re-resolves the shared `github.com/telark/*` modules and can fail against the private registry:

```sh
go build ./...        # from the repo root, uses the workspace
go work sync
```

## Build, lint, test

A `Makefile` wraps the common flows:

```sh
make build       # go build across the workspace
make lint        # golangci-lint per Go service (shared root .golangci.yml) + helm lint
make test        # go test per module
make helm-lint   # lint both charts
make fmt         # gofmt
```

- **Never pass `--no-config` or override linter flags.** A shared root `.golangci.yml` covers every Go service (golangci-lint discovers it by walking up from `services/<svc>`). Fix all errors (warnings are acceptable, errors must be zero, no `//nolint` as a workaround).
- The enrichment service uses `pytest` inside a virtualenv (`services/enrichment`).

## Charts

After any chart change:

```sh
make helm-lint
helm template t ./charts/telark               # optionally: --set app.mode=<mode>
```

Do not bump chart or module versions, and do not commit local `replace` directives — releases handle versioning, and the services must build against the published modules.

After changing `values.yaml`, run `make values-docs` to refresh each chart's `VALUES.md` (CI fails if it drifts). Packaging, pushing, and signing the charts is covered in [docs/PUBLISHING.md](docs/PUBLISHING.md).

## Commits & PRs

- Conventional-commit style subjects (`feat:`, `fix:`, `chore:`, `docs:`…), ≤ 50 chars, imperative.
- Keep changes surgical; every changed line should trace to the stated goal.
- Open a PR against `main`; fill in the template; ensure CI is green.

## License

By contributing you agree your contributions are licensed under the [Elastic License 2.0](LICENSE.md).
