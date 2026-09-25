---
name: go-service-change-gate
description: Use before reporting any change to a Go service under services/ (auth, discovery, exporter, notifier) as done, or when asked whether a Go change is green. Runs build, vet, tests and golangci-lint per service the way CI does, including the GOWORK=off check that catches code relying on unreleased shared modules.
---

# Go service change gate

Goal: every Go service you touched passes what CI runs for it, and the tree you leave behind is the tree you checked. CI's definition lives in `.github/workflows/ci.yaml` (the `go` job), `.github/actions/go-ci/action.yaml` and `.github/scripts/go-ci-test.sh`; when anything below disagrees with them, they win.

## Commands

From `services/<svc>`, for each service you changed:

```sh
go build ./...
go vet ./...
go test ./...

# CI mode: shared modules resolve from the go.mod pins, not from go.work
GOWORK=off go build ./...
GOWORK=off go test -race ./...

golangci-lint run   # last, after every edit and after deleting scratch files
```

`make check` from the repo root runs lint and tests for all four services, plus `helm lint` (which needs `make deps` once).

## What to check

- **CI parity.** `GOWORK=off` needs read access to the private `github.com/telark/*` modules (`GOPRIVATE` covering them, plus git credentials). A failure there on a symbol that exists in a local shared-module checkout means the service depends on an unreleased change. Releasing that module and bumping the pin is the user's job, so report it; don't add `replace` directives or edit `go.work`.
- **Coverage.** Tests live in `internal/tests/<area>/`, so only cross-package coverage means anything. When a change could move coverage, run what CI runs and compare with the service's floor in `ci.yaml`:

  ```sh
  GOWORK=off go test -race -coverpkg=./... -coverprofile=coverage.out ./...
  go tool cover -func=coverage.out | tail -1
  ```

  Raise the floor when coverage clearly improves; never lower it.
- **Test placement.** No `*_test.go` or `*_internal_test.go` beside production code. Move any test you or a sub-agent created outside `internal/tests/` before reporting done, and expose a small exported helper when a test needs an unexported symbol.
- **Lint result.** Zero errors, with the repository's `.golangci.yml` (golangci-lint finds it by walking up from the service directory). No `--no-config`, config-override flags or `//nolint`; when a finding is unclear, read `.golangci.yml`. Read the linter's summary instead of grepping for `file:line:col`, since the output can group issues by file. Pre-existing errors in files you didn't touch: flag them, don't silently fix them.
- **Linter and Go version.** golangci-lint only handles Go versions up to the one it was built with. If it reports that its Go version is lower than the targeted one, upgrade golangci-lint (the version CI pins is in `.github/actions/go-ci/action.yaml`); don't pin `GOTOOLCHAIN` to an older Go.
- **A passing build proves nothing about lint.** `revive` flags design issues the compiler accepts, so show the lint result in your report.
- **Workspace files.** `go.work.sum` can change during workspace builds and lint runs. It's the user's file; leave it.

## Report

Name the services you gated and give the lint result. If the change needs a new image or a shared-module release before it takes effect, say so once; don't bump versions.
