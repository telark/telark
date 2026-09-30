# Contributing

Thanks for contributing to Telark. Start with these three documents:

- [CONVENTIONS.md](CONVENTIONS.md): naming, errors, logging, tests and commits.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): every make target and when to use it.
- [GOVERNANCE.md](GOVERNANCE.md): roles and the contributor ladder.

## Repository layout

```
services/
  auth  discovery  exporter  notifier   Go services
  analyzer                              Python / FastAPI service
internal/
  data  rest  kcore  x-ware             shared Go packages
charts/
  telark        application chart
  telark-crds   custom resource definitions (bundled as a subchart of telark)
.github/        CI and release workflows
docs/           user guides, reference, architecture, security, testing, ADRs
```

All Go code is one module, `github.com/telark/telark` (the root `go.mod`): the services import the shared packages in `internal/` directly, so a change to one lands together with its callers.

## Prerequisites

- Go 1.27+ (the exact version is the `go` line of the root `go.mod`), `golangci-lint`, `helm` 3.
- Python 3.13+ and `venv` for the analyzer.
- Docker and a Kubernetes cluster for end-to-end work.

Exact tool versions and install commands: [docs/testing](docs/testing/README.md#fresh-environment).

## Build, lint, test

The `Makefile` wraps the common flows; the full reference is [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

```sh
make lint        # golangci-lint per Go service and package (shared root .golangci.yml) + helm lint
make test        # go test for the whole module
make helm-lint   # lint both charts
make fmt         # gofmt
```

- Run golangci-lint with the shared root `.golangci.yml`; never pass `--no-config` or override flags. Errors must be zero, and `//nolint` is not a fix.
- The analyzer uses `pytest` in a virtualenv. Its commands are in [docs/testing](docs/testing/README.md#analyzer-servicesanalyzer).

## Charts

Subchart packages (`charts/*/charts/*.tgz`) are git-ignored build artifacts; `Chart.lock` is committed. After any chart change:

```sh
helm repo add vpa https://charts.fairwinds.com/stable   # once; make deps doesn't add it
make deps            # once, or after editing dependencies
make helm-lint
make helm-validate   # renders every mode and schema-validates with kubeconform
helm template t ./charts/telark --set app.auth.bootstrap.admin=test@example.com   # optionally: --set app.mode=<mode>
```

- After changing `values.yaml`, run `make values-docs` to refresh each chart's `VALUES.md`; CI fails if it drifts.
- Don't bump chart or module versions, and don't commit `replace` directives. Releases handle versioning, and the services must build against the published modules.
- `CHANGELOG.md` is generated from Conventional Commits by the release workflow; preview it with `make changelog`.
- Packaging, pushing and signing the charts: [docs/PUBLISHING.md](docs/PUBLISHING.md).

## Commits and pull requests

Full rules: [CONVENTIONS.md](CONVENTIONS.md#commits-branches-prs). Who can approve and merge: [GOVERNANCE.md](GOVERNANCE.md).

- Conventional Commit subjects (`feat:`, `fix:`, `chore:`, `docs:`), at most 50 characters, imperative.
- Keep changes surgical: every changed line should trace to the stated goal.
- Open the pull request against `main`, fill in the template, and make sure CI is green.

## License

By contributing, you agree that your contributions are licensed under the [Elastic License 2.0](LICENSE.md).
