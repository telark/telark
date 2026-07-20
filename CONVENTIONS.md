# Conventions

The authoritative rulebook for working in this repo. Where a rule is machine-enforced,
the enforcing file is named — that file wins if this doc ever drifts.

## Repository layout

```
services/
  auth  discovery  exporter  notifier    Go services (github.com/telark/<svc>), one go.work
  enrichment                             Python / FastAPI service
charts/
  telark                                 application chart (services + subcharts + telark-crds)
  telark-crds                            CRDs, bundled as a subchart of telark
docs/                                    architecture, install, CRDs, ADRs, this + dev docs
.github/                                 CI, release, deploy workflows + composite actions
Makefile  go.work  .golangci.yml  cliff.toml  codecov.yml
```

Each Go service: `services/<svc>/main.go` + `internal/` split into single-purpose
packages — observed set: `constants config routes handlers clients helpers
controllers coordination authz cmd tests`. Module path is `github.com/telark/<svc>`.

**Shared Go packages** (`data`, `rest`, `x-ware`, `kcore`) live **outside this repo**
as private `github.com/telark/*` modules, pinned in each service's `go.mod` — ordinary
dependencies, **no `replace` directives**. The **UI** is a **separate repo**; only its
built image is referenced here (`services.ui`).

## Go

Enforced by the shared root [`.golangci.yml`](.golangci.yml) (golangci-lint v2, discovered
by walking up from `services/<svc>`; **never** pass `--no-config` or `//nolint` as a
workaround). Hard limits: **cyclomatic complexity ≤ 12** (`gocyclo`), **function length
≤ 60 lines** (`funlen`), `revive` all-rules, plus `errcheck gosec dupl unparam prealloc
bodyclose staticcheck` and more. `run.tests: false` — lint targets non-test code.

Beyond the linter (from `services/*/CLAUDE.md`):

- **Constants, not literals.** Every static string, config value, log message, and error
  string goes in the package's `constants/` folder — never inline in logic. No bare `""`,
  `0`, `1` in service code; use named constants.
- **Outbound calls go through `rest`-pkg clients.** No direct HTTP that bypasses the client
  layer. Discovery-service wraps those clients under `clients/<domain>.go`.
- **Kubernetes logic lives in `kcore`-pkg** (informers, dynamic watchers, listers). Read it
  before writing any new k8s client logic — it likely already has what you need.
- **Shared packages change upstream first.** Edit `data/rest/x-ware/kcore` in their own repo,
  release, then bump the consumer — never the other way. No service-specific logic in a
  shared package.
- **Domain logic stays out of handlers.** Core logic in a top-level `<service>/<domain>/`
  package; handlers stay thin and delegate.
- Prefer `slices`/`maps` stdlib over hand-rolled loops. Match the surrounding file's style.
- **Comments:** only for the non-obvious *why*, never restating the code (no header comments
  above funcs/types). Keep a comment block to **≤ 2 lines** — go beyond two lines only when
  the logic is genuinely hard to follow; if you need more, the code is probably too complex.
  This applies everywhere: Go, Python, Helm templates, and `values.yaml`.

## Naming

| Thing | Rule | Source |
|---|---|---|
| Packages / files | lowercase, single purpose; follow the package you're editing | layout |
| CRD kinds | PascalCase (`ApplicationAsResource`, `GlobalConfig`) | `docs/CRDS.md` |
| CRD groups | `erpi.<app.name>`, `auth.<app.name>`, `classification.<app.name>` | `docs/CRDS.md` |
| API routes | `/api/v1/<domain>/…`; health at `/api/v1/status/{live,ready}` | services |
| Env vars (containers) | `SCREAMING_SNAKE_CASE` | `charts/telark/values.yaml` |
| Helm value keys | `camelCase` | `charts/telark/values.yaml` |

## Errors & logging

- `errcheck` is on — never drop an error. Wrap with context on the way up; return, don't
  panic, in request/reconcile paths.
- Use the service's structured logger (e.g. enrichment's `app_logger.py`); don't `fmt.Print`
  / `print()` for diagnostics in service code.

## Testing

- **Layout:** tests live in `internal/tests/*` as separate packages that exercise the code
  packages. Measure coverage **cross-package**: `go test -coverpkg=./... ./...` (plain
  `go test ./...` reports ~0% for this layout).
- **Style:** table-driven; reuse the `testutil` helpers (miniredis, embedded NATS, fakes).
  Don't weaken an existing test to make a change pass — if it asserts wrong behavior, flag it.
- **Gate:** CI enforces a per-service coverage **ratchet floor** (`.github/workflows/ci.yaml`);
  raise it as coverage improves, never lower it.
- **Python (enrichment):** `pytest`; stub-based suites (`test_*_cov.py`) run in a separate
  process from real-dependency suites; `python -m compileall` is the syntax gate. *(No
  ruff/black/flake8 config exists today — formatting is by convention.)*

## Commits, branches, PRs

- **Conventional Commits** subjects (`feat: fix: chore: docs: …`), ≤ 50 chars, imperative
  (`cliff.toml` builds the changelog from these).
- **Trunk-based:** open PRs against `main`; there are no long-lived release branches
  (releases are `v*` tags).
- Fill in the PR template, keep changes surgical (every changed line traces to the goal),
  and land with green CI + the required `CODEOWNERS`/maintainer review
  (see [GOVERNANCE.md](GOVERNANCE.md)).
- Do **not** bump chart or module versions in a feature PR, and do not commit `replace`
  directives — releases own versioning.

## Helm charts

- **One flag sizes everything:** `app.mode` (`minimal|standard|performance`) deep-merges a
  preset over `values.yaml` via `_mode.tpl` — for telark's own services only (subcharts
  can't be reached from a parent template, so they ship fixed production-grade defaults).
- **Generic over per-service boilerplate:** workload templates are thin wrappers over shared
  helpers (`_deployment.tpl` etc.).
- **Subcharts:** declared in `Chart.yaml`, locked in `Chart.lock` (committed); the vendored
  `charts/*/charts/*.tgz` are **git-ignored** build artifacts — `make deps` rebuilds them.
- **Docs are generated:** never hand-edit `VALUES.md` — run `make values-docs` (CI fails on
  drift). CRDs carry `helm.sh/resource-policy: keep`.
- Keep `values.yaml` lean (SaaS-MVP ready); per-key prose belongs in the chart README, not
  inline. No per-service Helm `resources:` overrides (common `includeResources` handles it).

## UI

The dashboard SPA lives in a **separate repository**; this repo only references the built
image (`services.ui`). UI-specific conventions belong in that repo.

## AI-agent workflow

Development here is often driven by LLM coding agents. An agent must:

**Read first (in order):** the root [README](README.md) → this file → the
`services/<svc>/CLAUDE.md` for the service it's touching → [CONTRIBUTING.md](CONTRIBUTING.md).
`CLAUDE.md` files are binding and override defaults.

**Always:**
- Scope work to one service/chart; keep the diff surgical.
- Put strings/config in `constants/`; obey the linter limits (≤12 complexity, ≤60-line funcs).
- Verify before "done": `make lint` (0 errors) + `make test`, and for charts
  `make helm-lint && make helm-validate`; regenerate `VALUES.md` if `values.yaml` changed.
- Heavy command output is truncated by the `rtk` wrapper — write it to a file (or use the
  raw proxy) so results aren't silently cut.

**Never:**
- Edit a shared `github.com/telark/*` package from a consuming service (change it upstream
  first), or add service-specific logic to a shared package.
- Bump chart/module versions, add/remove `replace` directives, or hand-edit `VALUES.md`.
- Commit, push, or tag — humans own git.
- Touch another service's directory when scoped to one, or weaken existing tests to pass.
- Commit secrets, or relax a linter/gate to make something pass.
