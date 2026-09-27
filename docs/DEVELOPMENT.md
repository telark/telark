# Development

Local development commands. The root `Makefile` is the entry point — `make help`
lists every target. Conventions live in [CONVENTIONS.md](../CONVENTIONS.md); this
file is only "what command, when".

## Prerequisites

| Tool | Needed for | Notes |
|---|---|---|
| Go **1.27.1** | all Go targets | the `go` line of each `services/<svc>/go.mod`; the committed `go.work` is the maintainer's local workspace (see the `GOWORK=off` note below) |
| `golangci-lint` **v2.13.2** | `make lint` | the version CI pins; uses the shared root `.golangci.yml` (never pass `--no-config`) |
| `helm` **≥ 3** (OCI) | all `helm-*` targets | |
| `kubeconform` **v0.8.0** | `make helm-validate` | schema-validates rendered manifests |
| Python **3.13** + `venv` | analyzer tests | `services/analyzer`; the version CI uses |
| Docker + a cluster | end-to-end work | |
| `git-cliff` | `make changelog` | changelog generation |

## Make targets

| Target | What it does | When to use | Prereqs |
|---|---|---|---|
| `make help` | Lists targets with their `##` descriptions | discovering commands | — |
| `make build` | `go build ./...` across the workspace | after any Go change | Go |
| `make vet` | `go vet ./...` | quick static check | Go |
| `make fmt` | `gofmt -w services` | before committing Go | Go |
| `make test` | `go test ./...` per Go service (`auth discovery exporter notifier`) | after any Go change | Go |
| `make lint` | `golangci-lint run` per service **+** `make helm-lint` | before every PR | golangci-lint, helm |
| `make helm-lint` | `helm lint` on `telark-crds` (with app values) and `telark` | after any chart change | helm |
| `make helm-template` | Render the app chart; `MODE=minimal\|standard\|performance` optional | inspect rendered manifests | helm, `make deps` |
| `make deps` | `helm repo add` + `helm dependency build` — fetches the git-ignored subchart `.tgz` | once, or after editing chart deps | helm |
| `make helm-validate` | Renders every mode and pipes to `kubeconform` (schema validation) | after any chart change | kubeconform, `make deps` |
| `make values-docs` | Regenerates each chart's `VALUES.md` via `helm-docs` | after editing any `values.yaml` (CI fails on drift) | Go (runs helm-docs via `go run`) |
| `make changelog` | Regenerates `CHANGELOG.md` from Conventional Commits (`git-cliff`) | preview release notes | git-cliff |
| `make sync` | `go work sync` | after changing workspace deps | Go |
| `make publish-charts` | `make deps` + package + push both charts to the OCI registry | manual chart publish (run `helm registry login ghcr.io` first) | helm, registry auth |
| `make check` | `make lint` + `make test` | full local gate before a PR | all Go + helm |

## Notes

- **No per-service Makefiles.** Targets loop over the Go services; to work on one
  service directly: `cd services/<svc> && go test ./...`.
- **Coverage** is measured cross-package (`go test -coverpkg=./...`) because tests
  live in `internal/tests/*`; CI enforces a per-service floor (see `.github/workflows/ci.yaml`).
- **`GOWORK=off` outside the maintainer's machine.** The committed `go.work` replaces the
  shared modules with local absolute paths, so in a fresh clone every workspace-mode Go
  command fails. Export `GOWORK=off` (what CI does): `make test`, `make lint` and
  `make check` then work per service; `make build`, `make vet` and `make sync` need the
  workspace. Details: [testing/README.md](testing/README.md#shared-go-modules).
- **Chart deps are git-ignored** (`charts/*/charts/*.tgz`) — run `make deps` after a
  fresh clone before `helm-lint`/`helm-template`/`helm-validate`. `make deps` doesn't add
  the `vpa` repository; run `helm repo add vpa https://charts.fairwinds.com/stable` first.
- **Analyzer (Python)** isn't in the Makefile; its CI-equivalent commands are in
  [testing/README.md](testing/README.md#analyzer-servicesanalyzer).
- Chart packaging, signing, and registry publishing: [PUBLISHING.md](PUBLISHING.md).
