# Conventions

The authoritative rulebook for working in this repo. Where a rule is machine-enforced,
the enforcing file is named, and that file wins if this doc drifts.

## Repository layout

The directory map is in [CONTRIBUTING.md](CONTRIBUTING.md#repository-layout).

Each Go service: `services/<svc>/main.go` + `internal/` split into single-purpose
packages, such as `constants config routes handlers clients helpers controllers
coordination authz tests`. Import path is `github.com/telark/telark/services/<svc>`.

**Shared Go packages** (`data`, `rest`, `x-ware`, `kcore`) live in `internal/` of the
same module (`github.com/telark/telark`, one root `go.mod`, **no `go.work`, no `replace`
directives**), so a change to one lands together with its callers. No service-specific
logic in a shared package.

The dashboard SPA lives in the separate
[`telark/dashboard-ui`](https://github.com/telark/dashboard-ui) repository, which holds
the UI conventions; this repo only references its built image (`services.ui`).

## Go

Enforced by the shared root [`.golangci.yml`](.golangci.yml) (golangci-lint v2, run from
the repo root per service and package; **never** pass `--no-config` or use `//nolint` as a
workaround). Hard limits: **cyclomatic complexity ≤ 12** (`gocyclo`), **function length
≤ 60 lines** (`funlen`), `revive` all-rules, plus `errcheck gosec dupl unparam prealloc
bodyclose staticcheck` and more. `run.tests: true`, so lint covers test code too.

Beyond the linter:

- **Constants, not literals.** Every static string, config value, log message, and error
  string goes in the package's `constants/` folder, never inline in logic. No bare `""`,
  `0`, `1` in service code; use named constants.
- **Outbound calls go through `rest` clients.** No direct HTTP that bypasses the client
  layer. Discovery wraps those clients in `services/discovery/internal/clients/<domain>.go`.
- **Kubernetes logic lives in `kcore`** (informers, dynamic watchers, listers). Read it
  before writing new k8s client logic; it likely already has what you need.
- **Domain logic stays out of handlers.** Core logic goes in its own
  `services/<svc>/internal/<domain>/` package; handlers stay thin and delegate.
- Prefer `slices`/`maps` stdlib over hand-rolled loops. Match the surrounding file's style.
- **Comments:** only for the non-obvious *why*, never restating the code (no header comments
  above funcs/types). Keep a comment block to **≤ 2 lines** unless the logic is genuinely
  hard to follow; needing more usually means the code is too complex. This applies
  everywhere: Go, Python, Helm templates, and `values.yaml`.

## Naming

| Thing | Rule | Source |
|---|---|---|
| Packages / files | lowercase, single purpose; follow the package you're editing | layout |
| CRD kinds | PascalCase (`ProtectionPlan`, `TelarkConfig`) | `docs/CRDS.md` |
| CRD group | `telark.io` (constant), version `v1alpha1` | `docs/CRDS.md` |
| API routes | `/api/v1/<domain>/…`; health at `/api/v1/status/{live,ready}` | services |
| Env vars (containers) | `SCREAMING_SNAKE_CASE` | `charts/telark/values.yaml` |
| Helm value keys | `camelCase` | `charts/telark/values.yaml` |

## Errors & logging

- `errcheck` is on: never drop an error. Wrap with context on the way up; return, don't
  panic, in request/reconcile paths.
- Use the service's structured logger (e.g. analyzer's `app_logger.py`); don't `fmt.Print`
  / `print()` for diagnostics in service code.

## Testing

- **Layout:** tests live in `internal/tests/*` as separate packages that exercise the code
  packages. Measure coverage **cross-package**: `go test -coverpkg=./... ./...` (plain
  `go test ./...` reports ~0% for this layout).
- **Style:** table-driven; reuse the `testutil` helpers (miniredis, embedded NATS, fakes).
  Don't weaken an existing test to make a change pass; if it asserts wrong behavior, flag it.
- **Gate:** CI enforces a coverage **ratchet floor** per service and shared package
  (`.github/workflows/ci.yaml`); raise it as coverage improves, never lower it.
- **Python (analyzer):** `pytest`; stub-based suites (`test_*_cov.py`) run in a separate
  process from real-dependency suites; `python -m compileall` is the syntax gate. No
  ruff/black/flake8 config exists, so formatting is by convention.

## Commits, branches, PRs

These rules apply to every Telark repository. The short version is in
[CONTRIBUTING.md](CONTRIBUTING.md#branches-commits-and-pull-requests).

Branch and commit types:

| Type | For |
|---|---|
| `feat` | new functionality |
| `fix` | bug fixes |
| `refactor` | code restructuring without changing behavior |
| `perf` | performance improvements |
| `docs` | documentation-only changes |
| `test` | adding or updating tests |
| `build` | build system or dependency changes |
| `ci` | CI/CD workflow changes |
| `chore` | maintenance and tooling |
| `hotfix` | urgent production fixes (branches only) |
| `revert` | reverting a previous commit (commits only) |

**Branches:** `<type>/<kebab-case-description>`: lowercase, descriptive and concise, for
example `feat/add-auth-layer-using-webauthn` or `fix/apps-and-plans-sync`.

**Commits:** [Conventional Commits](https://www.conventionalcommits.org/),
`<type>(<optional-scope>): <description>` (`cliff.toml` builds the changelog from these).

- The description is imperative, lowercase after the prefix, ≤ 50 chars, free of
  unnecessary punctuation, and covers one logical change: `feat(auth): add WebAuthn
  authentication`, `build(deps): update TypeScript`.
- Mark a breaking change with `!` after the type or scope
  (`feat(api)!: change dashboard API response handling`) or a `BREAKING CHANGE:` footer.
- No non-standard tags such as `[Major]` or `[Minor]`.
- Never commit secrets.

**Pull requests:**

- **Trunk-based:** open PRs against `main`; there are no long-lived release branches
  (releases are `v*` tags).
- Fill in the PR template, keep changes surgical (every changed line traces to the goal),
  and land with green CI + the required `CODEOWNERS`/maintainer review
  (see [GOVERNANCE.md](GOVERNANCE.md)).
- Do **not** bump chart or service versions in a feature PR, and do not add `replace`
  directives or a `go.work`: releases own versioning.

## Helm charts

- **One flag sizes everything:** `app.mode` (`minimal|standard|performance`) deep-merges a
  preset over `values.yaml` via `_mode.tpl`, for telark's own services only (subcharts
  can't be reached from a parent template, so they ship fixed production-grade defaults).
- **Generic over per-service boilerplate:** workload templates are thin wrappers over shared
  helpers (`_deployment.tpl` etc.).
- **Subcharts:** declared in `Chart.yaml`, locked in `Chart.lock` (committed); the vendored
  `charts/*/charts/*.tgz` are **git-ignored** build artifacts that `make deps` rebuilds.
- **Docs are generated:** never hand-edit `VALUES.md`; run `make values-docs` (CI fails on
  drift). CRDs carry `helm.sh/resource-policy: keep`.
- Keep `values.yaml` lean; per-key prose belongs in the chart README, not inline. No
  per-service Helm `resources:` overrides (common `includeResources` handles it).

## AI coding assistants

You may use coding assistants. [AGENTS.md](AGENTS.md) holds the rules they must follow
here (Claude Code reads it through `CLAUDE.md`). You stay responsible for what you submit:
review and test it yourself. Assisted pull requests meet the same expectations as any other.
