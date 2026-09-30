# Development

Which command to run, and when. The root `Makefile` is the entry point: `make help`
lists every target. Conventions are in [CONVENTIONS.md](../CONVENTIONS.md).

## Prerequisites

| Tool | Needed for | Notes |
|---|---|---|
| Go **1.27.1** | all Go targets | the `go` line of the root `go.mod` |
| `golangci-lint` **v2.14.0** | `make lint` | the version CI pins; uses the shared root `.golangci.yml` (never pass `--no-config`) |
| `helm` **≥ 3** (OCI) | all `helm-*` targets | |
| `kubeconform` **v0.8.0** | `make helm-validate` | schema-validates rendered manifests |
| Python **3.13** + `venv` | analyzer tests | `services/analyzer`; the version CI uses |
| Docker + a cluster | end-to-end work | |
| `git-cliff` | `make changelog` | changelog generation |

## Make targets

| Target | What it does | When to use | Prereqs |
|---|---|---|---|
| `make help` | Lists targets with their `##` descriptions | discovering commands | — |
| `make build` | `go build ./...` across the module | after any Go change | Go |
| `make vet` | `go vet ./...` | quick static check | Go |
| `make fmt` | `gofmt -w services internal` | before committing Go | Go |
| `make test` | `go test ./...` across the module (services and shared packages) | after any Go change | Go |
| `make lint` | `golangci-lint run` per service and shared package (`SERVICE=<name>` for one) **+** `make helm-lint` | before every PR | golangci-lint, helm |
| `make helm-lint` | `helm lint` on `telark-crds` (with app values) and `telark` | after any chart change | helm |
| `make helm-template` | Render the app chart; `MODE=minimal\|standard\|performance` optional | inspect rendered manifests | helm, `make deps` |
| `make deps` | `helm repo add` + `helm dependency build`; fetches the git-ignored subchart `.tgz` | once, or after editing chart deps | helm |
| `make helm-validate` | Renders every mode and pipes to `kubeconform` (schema validation) | after any chart change | kubeconform, `make deps` |
| `make values-docs` | Regenerates each chart's `VALUES.md` via `helm-docs` | after editing any `values.yaml` (CI fails on drift) | Go (runs helm-docs via `go run`) |
| `make changelog` | Regenerates `CHANGELOG.md` from Conventional Commits (`git-cliff`, format in `cliff.toml`); PR links and contributors need `GITHUB_TOKEN`, otherwise it runs offline | preview release notes | git-cliff |
| `make publish-charts` | `make deps` + package + push both charts to the OCI registry | manual chart publish (run `helm registry login ghcr.io` first) | helm, registry auth |
| `make check` | `make lint` + `make test` | full local gate before a PR | all Go + helm |

## Notes

- **One Go module, no per-service Makefiles.** Go commands run from the repository root;
  to work on one service directly: `go test ./services/<svc>/...`.
- **Coverage** is measured cross-package (`go test -coverpkg=./services/<svc>/...`) because
  tests live in `internal/tests/*`; CI enforces a per-service floor (see `.github/workflows/ci.yaml`).
- **Chart deps are git-ignored** (`charts/*/charts/*.tgz`). Run `make deps` after a
  fresh clone before `helm-lint`/`helm-template`/`helm-validate`. `make deps` doesn't add
  the `vpa` repository; run `helm repo add vpa https://charts.fairwinds.com/stable` first.
- **Analyzer (Python)** isn't in the Makefile; its CI-equivalent commands are in
  [testing/README.md](testing/README.md#analyzer-servicesanalyzer).
- Chart packaging, signing, and registry publishing: [PUBLISHING.md](PUBLISHING.md).
