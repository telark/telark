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

The Go services depend on shared packages published under `github.com/telark/*`. They resolve locally through the workspace during development.

## Prerequisites

- Go (see `go.work` for the version), `golangci-lint`, `helm` ≥ 3.
- Python 3.12 + `venv` for the enrichment service.
- Docker + a Kubernetes cluster for end-to-end work.

## Go workspace

The four Go services live in a single `go.work`. Shared internal packages resolve via workspace-wide `replace` directives.

**Set `GOPRIVATE` before any module operation** — the internal packages are private:

```sh
export GOPRIVATE=github.com/telark/*
```

Build the whole workspace rather than tidying a single service — a standalone `go mod tidy` re-resolves shared packages to published versions and can fail:

```sh
go build ./...        # from the repo root, uses the workspace
go work sync
```

## Build, lint, test

A `Makefile` wraps the common flows:

```sh
make build       # go build across the workspace
make lint        # golangci-lint per Go service (each service's own config) + helm lint
make test        # go test per module
make helm-lint   # lint both charts
make fmt         # gofmt
```

- **Never pass `--no-config` or override linter flags.** Each Go service ships its own `.golangci.yml`; use it. Fix all errors (warnings are acceptable, errors must be zero, no `//nolint` as a workaround).
- The enrichment service uses `pytest` inside a virtualenv (`services/enrichment`).

## Charts

After any chart change:

```sh
make helm-lint
helm template t ./charts/telark -f charts/telark/values.mode.standard.yaml \
  --set-file nats.configuration=charts/telark/config/nats.conf
```

Do not bump chart/module versions or remove `replace` directives — releases handle versioning.

## Commits & PRs

- Conventional-commit style subjects (`feat:`, `fix:`, `chore:`, `docs:`…), ≤ 50 chars, imperative.
- Keep changes surgical; every changed line should trace to the stated goal.
- Open a PR against `main`; fill in the template; ensure CI is green.

## License

By contributing you agree your contributions are licensed under the [Elastic License 2.0](LICENSE.md).
