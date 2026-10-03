# Contributing

Thanks for contributing to Telark. Start with these three documents:

- [CONVENTIONS.md](CONVENTIONS.md): naming, errors, logging, tests, branches and commits.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): every make target and when to use it.
- [GOVERNANCE.md](GOVERNANCE.md): roles and the contributor ladder.

## Repositories

| Repository | Contents |
|---|---|
| [`telark/telark`](https://github.com/telark/telark) (this one) | Services, Helm charts, CRDs and docs |
| [`telark/dashboard-ui`](https://github.com/telark/dashboard-ui) | The operator dashboard SPA |

Dashboard-specific issues and pull requests go to `telark/dashboard-ui`. The branch, commit and pull request rules below apply to every Telark repository.

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

Go (the version on the `go` line of the root `go.mod`), `golangci-lint`, `helm` 3, Python 3.13 with `venv` for the analyzer, and Docker plus a Kubernetes cluster for end-to-end work. Pinned versions and install commands: [docs/testing](docs/testing/README.md#fresh-environment).

## Build, lint, test

```sh
make lint        # golangci-lint per Go service and package (shared root .golangci.yml) + helm lint
make test        # go test for the whole module
make helm-lint   # lint both charts
make fmt         # gofmt
```

- Never pass `--no-config` or override flags to golangci-lint. Errors must be zero, and `//nolint` is not a fix.
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
- Don't bump chart or service versions (the build and release workflows do), and don't add `replace` directives.
- `CHANGELOG.md` is generated from Conventional Commits by the release workflow; preview it with `make changelog`.
- Packaging, pushing and signing the charts: [docs/PUBLISHING.md](docs/PUBLISHING.md).

## Branches, commits and pull requests

Full rules: [CONVENTIONS.md](CONVENTIONS.md#commits-branches-prs). Who can approve and merge: [GOVERNANCE.md](GOVERNANCE.md).

- Branch from `main` as `<type>/<kebab-case-description>`, for example `fix/apps-and-plans-sync`.
- Conventional Commit subjects, `<type>(<optional-scope>): <description>`, with an imperative description of at most 50 characters.
- Keep changes surgical: every changed line should trace to the stated goal.
- Open the pull request against `main`, fill in the template, and make sure CI is green.

## License

By contributing, you agree that your contributions are licensed under the [Elastic License 2.0](LICENSE.md).
