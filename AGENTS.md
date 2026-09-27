# Agent instructions for telark

Rules and workflows for coding agents (Claude Code, Codex, Cursor, Copilot, Gemini and others) working in this repository. This file is canonical for rules: `CLAUDE.md` imports it and adds only a short map of the system for Claude Code, and the skills under `.claude/skills/` hold the step-by-step procedures. [CONVENTIONS.md](CONVENTIONS.md) is the human rulebook for naming, errors, logging and commits. Where a rule is machine-enforced (`.golangci.yml`, `.github/workflows/ci.yaml`, `Makefile`), the enforcing file wins; report drift instead of working around it.

## Repository map

| Path | Contents |
|---|---|
| `services/auth`, `services/discovery`, `services/exporter`, `services/notifier` | Go services (module `github.com/telark/<svc>`): `main.go` plus single-purpose packages under `internal/`; tests under `internal/tests/<area>/` |
| `services/analyzer` | The analyzer: a Python/FastAPI service that runs local open-weight models through Ollama |
| `charts/telark` | Application chart: services, subcharts, sizing presets in `modes/`, and `values.dev.yaml` (local port-forward settings that Helm never loads) |
| `charts/telark-crds` | CRDs, vendored into `charts/telark` as a `file://` subchart |
| `docs/` | `INSTALL.md`, `DEVELOPMENT.md` (every make target), `PUBLISHING.md`, `CRDS.md`, ADRs; `architecture/`, `security/` and `testing/` for agents and contributors |
| `scripts/` | Dev-cluster loop: `local-build-push.sh`, `local-port-forward.sh`; git-ignored, so they exist only on the maintainer's machine |
| `.github/` | CI (`workflows/ci.yaml`), build and release workflows, composite actions in `actions/` and their scripts in `scripts/` |
| `.claude/` | Feature guidelines, model routing, feature plans (`plans/`), skills (`skills/`) |

`make help` lists the Makefile targets; `docs/DEVELOPMENT.md` says when to use each.

Outside this repository:

- **Shared Go modules** `github.com/telark/{data,rest,kcore,x-ware}` live in their own repositories and are pinned in each service's `go.mod`; the Go module proxy serves the pinned versions without credentials. `go.work` is maintained by the user and points them at checkouts on the user's machine; CI ignores it (`GOWORK=off`), and so must any other environment ([docs/testing](docs/testing/README.md#shared-go-modules)).
- **Dashboard UI**: the `telark/dashboard-ui` repository (stable branch `master`). This repo only references its image (`services.ui`).
- **Cluster infrastructure**: the `telark/infra` repository (Terraform).
- **Legacy**: the `release-manager` repository is no longer used. Values, modes, tunables and sizing live in `charts/telark`; ignore older notes that point elsewhere.

## Working agreements

### 1. Surface assumptions

Don't assume. Don't hide confusion. Surface tradeoffs.

Before implementing:

- State your assumptions explicitly.
- If multiple interpretations exist, present them; don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- For minor choices (naming, formatting, default values, which approach among equivalents), pick a reasonable option and note it rather than asking. For scope changes, destructive or irreversible actions, or requirements you can't infer from the request or the code, ask first.

### 2. Simplicity first

Minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.
- Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical changes

Touch only what you must. Clean up only your own mess.

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it; don't delete it.

When your changes create orphans:

- Remove imports, variables and functions that your changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: every changed line should trace directly to the user's request.

### 4. Goal-driven execution

Define success criteria. Loop until verified.

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

Verify each step against its success criterion before moving on. Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

### In this repository

- **Comments** explain only a non-obvious why (a hidden constraint, an invariant, a workaround), in at most two lines, in Go, Python, templates and `values.yaml` alike. No header comments above functions, types or fields: well-named identifiers already say what the code does, and a comment that needs more than two lines usually means the code needs simplifying.
- **Gates stay strict.** Run linters with the repository's checked-in config: no `--no-config`, override flags or `//nolint`, because they hide real issues and diverge from CI; when a finding is unclear, read the config. Don't weaken a test to make a change pass (if it asserts wrong behavior, flag it), and don't lower a coverage floor or relax a CI check.
- **Infrastructure** changes (node groups, instance types, capacity, EKS version, IAM) are Terraform edits in the infra repository that the user reviews and applies. Read-only `aws` and `kubectl` queries are fine for facts; don't change the cluster directly with `aws`, `eksctl` or similar, because it has to stay reproducible from code.
- **Sub-agents** start without your context. Brief them with the rules that apply to their task (test location, constants, no version bumps, the lint gate) and check their output against those rules before reporting done.
- **Truncated output.** Some agent shells filter long or multi-document command output, sometimes even through a `>` redirect. When completeness matters (`helm template`, multi-document YAML, long greps, file dumps), compare counts with what you expect and re-run unfiltered before concluding that something is broken.
- **Code navigation.** If the code-review-graph MCP server is available and `list_graph_stats` shows this repository indexed, prefer it for callers, impact-radius and test-coverage questions (see the `graph-*` skills); otherwise use normal search.
- **Feature work** follows [.claude/FEATURE_IMPLEMENTATION_GUIDELINES.md](.claude/FEATURE_IMPLEMENTATION_GUIDELINES.md): design checklist, edge cases, definition of done.

## Go services

**Layout and ownership**

- Domain logic (orchestration, validation, Kubernetes operations, business rules) lives in its own package under `services/<svc>/internal/<domain>/`. Handlers only decode the request, call the domain package and write the response; controllers and background loops reuse the same package. The logic stays reusable, and handlers stay thin enough to test in isolation.
- The exporter owns CRD operations and the storage behind them; other services reach them over HTTP through `rest` clients. The exporter implements those handlers itself and never calls a `rest` client (reusing the module's request and response types is fine), since calling itself over HTTP would be a circular dependency.
- For a new cross-service domain, build in this order: endpoint constants and types in `rest` (`endpoints/<domain>/`), handlers in the owning service wired to its routes, the `rest` client (`clients/<domain>/`), then the callers. A client without a server is dead code, and the handler defines the wire contract.
- Discovery uses `rest` clients only through wrappers in `services/discovery/internal/clients/<domain>.go` (see `applications.go`), which hold its timeouts, circuit breaker and error classification. A few older call sites construct clients directly; don't extend that pattern.
- Shared modules change upstream first: edit `data`, `rest`, `kcore` or `x-ware` in its own repository, release it, then bump the consumer. No service-specific logic in a shared module. New Kubernetes client logic (informers, dynamic watchers, listers, informer factories) belongs in `kcore`; read it first, it likely has what you need.

**Code**

- Every static string, config value, log message and error string goes in the package's `constants/` folder, not inline in logic. Write no bare `""`, `0` or `1`: use the named constants (`constants.EmptyString`, `constants.DefaultInitValue`, `constants.DefaultIncrementValue`, …; check the service's `constants/` first). `revive`'s `add-constant` rule is on, so magic numbers and repeated strings fail lint.
- A literal repeated in two or more files of a package (operation or kind names, JMESPath expressions, operators, label keys, regexes, format strings) moves to a shared constants file in that package, so there is one place to audit and edit.
- When another feature needs a constant defined in a feature's own `constants.go`, move it to the service's root `constants/` package, remove the original and update the callers; don't import one feature's constants from another, because that creates false coupling. If the constant really belongs to that feature, keep the boundary and define your own.
- All `type` declarations of a package go in one `types.go`; methods and functions stay in their domain files. Don't create empty or doc-only files.
- Remove an unused parameter from the signature and update every caller rather than renaming it `_`. `_ Type` is only for signatures fixed by an interface you don't own.
- Prefer the `slices` and `maps` packages over hand-written loops for membership, index, sort, clone and delete.
- Match the naming, structure and style of the file you're editing. Complexity and function-length limits come from `.golangci.yml`; extract a helper instead of relaxing them. Naming, error and logging rules are in [CONVENTIONS.md](CONVENTIONS.md).

**Configuration**

- A new configurable value (timeout, interval, size, rate, feature flag) is an env var read at runtime: an `Env*` name and a `Default*` fallback in the service's `constants/`, a loader in `config/`, and the cluster-facing value under `services.<svc>.env` in `charts/telark/values.yaml` (plus `charts/telark/modes/*.yaml` when it varies by install mode). Add it to the chart even when the Go default is off, so operators can see and flip it.
- To tune an existing value, override it in the chart and leave `Env*` and `Default*` untouched; name the env var and the recommended value in your report. `Default*` constants are the conservative baseline every environment inherits. Per-cluster tuning, staged rollouts and emergency switches belong to the deployment layer, so a cluster can change them without a new image.

**CRDs**

- The CRDs are strict structural schemas. A field added to a Go type before the CRD schema declares it is pruned by the API server: the write returns 200, the data is gone, and nothing is logged. Edit the CRD YAML in `charts/telark-crds/templates/crds/` first, then the Go type, and verify with a read-back instead of trusting the status code.

**Tests and verification**

- Tests live under `services/<svc>/internal/tests/<area>/` as separate packages, table-driven, reusing the `testutil` helpers. Don't add `*_test.go` or `*_internal_test.go` files beside production code; when a test needs an unexported symbol, expose a small exported helper or test through the exported surface. Production packages stay test-free, and each service's tests sit in one tree.
- Before calling a Go change done, run the `go-service-change-gate` skill: build, vet and test, the `GOWORK=off` check CI runs, and `golangci-lint run` last with zero errors. A green workspace build is not a green CI build, because CI resolves shared modules from the `go.mod` pins.

**Known pitfall**

- Protection-plan violations come from Kubernetes Events filtered against the plan's rendered policies. `Terminate` and `Cancel` call `CleanupByPlanID` and then blank the rendered-policy list, so a finished plan reports zero violations; Events also expire (see `RetentionWindow` in the violations package). Anything that needs a finished plan's history has to capture it durably before `CleanupByPlanID` runs, and state the window it covers.

## Helm chart

- The CRD API group is the constant `telark.io` (version `v1alpha1`), and labels, annotations and finalizers use the `telark.io/` domain, in both charts and in the Go code ([ADR 0003](docs/adr/0003-constant-api-group-telark-io.md)). `.Values.app.name` only prefixes object names: derive resource names with the `telark.name` and `telark.fullname` helpers instead of hardcoding `telark`, and don't source names from `.Chart.Name` (the CRD chart is named `telark-crds`).
- Wire subchart values with the least mechanism that works: nothing when the subchart's own `values.yaml` default is right, a `<subchart>:` block in the parent only to override a default, and `global.*` only when many subcharts need the same value.
- Sizing is shared: `includeResources: true` applies `app.shared.resources`, and the `app.mode` presets (`modes/minimal.yaml`, `modes/performance.yaml`; `standard` is `values.yaml` itself) resize it. Don't add per-service `resources:` blocks or template branches for them, because per-service overrides drift from the shared block and hide where values come from. If one service truly needs different sizing, change the shared value or ask.
- Reuse the existing chart directories and subcharts (for example, install an existing subchart as its own release) instead of creating parallel chart folders.
- Keep `values.yaml` lean, like a product release: a header of about a dozen lines, a one-line inline comment only for a non-obvious why, no dividers and no per-key narration. Per-key documentation goes in the chart's `README.md`.
- `values.schema.json` rejects unknown keys in several blocks, so a new key there needs a schema entry; `make helm-lint` catches a missing one.
- `VALUES.md` is generated (`make values-docs`, drift-checked in CI); don't edit it by hand. Vendored subcharts (`charts/*/charts/`) are git-ignored and rebuilt by `make deps`; `Chart.lock` is committed.
- Follow the `helm-chart-change` skill for the lint, render and validate gate and for the docs that change with a value.

## Analyzer (Python)

The analyzer is `services/analyzer` (chart key `services.analyzer`, image `analyzer`). Its `README.md` and `ARCHITECTURE.md` describe the pipeline.

- It runs only free, open-weight models on open-source runtimes (Ollama). No commercial AI providers and no API keys, not even as an optional or bring-your-own-key tier: the product promise is a free, open-source, privacy-first analyzer, and provider keys were removed on purpose.
- The only deployment switch is air-gapped versus connected. Air-gapped: no egress, models pre-loaded (`app.ollama.autoPull=false`). Connected: egress only to fetch open models or to reach the customer's own self-hosted Ollama-API endpoint (`app.ollama.runtimeUrl`, a URL, no key).
- It has to answer in seconds on small CPU-only nodes. Designs that need minutes, or a GPU by default, don't fit.
- Log through `app_logger`, not `print()`, and keep strings in the existing `constants.py` and `messages.py` split. There is no formatter or linter config, so match the surrounding code.
- Before calling a change done, run the `analyzer-ci-gate` skill. Its image is not built by `scripts/local-build-push.sh`; see the `deploy-dev-cluster` skill.

## Docs

- Docs change in the same change as the behavior they describe; stale docs mislead users and the drift compounds. For a new or changed chart value or flag: usage and non-obvious gotchas in `docs/INSTALL.md`; a value-reference row in `charts/<chart>/README.md` linking to that INSTALL section (check that the anchor resolves); a regenerated `VALUES.md`; then a sweep of `README.md`, `docs/PUBLISHING.md`, `CONTRIBUTING.md` and the service READMEs for references the change makes stale.
- User docs use the literal `telark` names and labels; don't add "if you changed app.name, replace…" caveats. Operational procedures (uninstall teardown, for example) are copy-paste command blocks in the docs, not scripts.
- Shipped files (docs, code comments, docstrings, test fixtures) never contain a personal email address or a real person's name. Contact, maintainer, disclosure and bootstrap-admin fields use `contact@telark.io`; examples use a placeholder such as `jane.doe@example.com`. The repositories and charts are public, and a comment ships as publicly as a README.
- Reusable guideline and checklist docs stay feature-agnostic: describe the general mechanism, not one feature's keys, names or scenarios, so the doc applies to the next feature.
- Generated files are regenerated, never edited: `VALUES.md` (`make values-docs`) and `CHANGELOG.md` (git-cliff, from Conventional Commits).

## CI and GitHub Actions

- `.github/workflows/ci.yaml` is the merge gate: per Go service, a `test` leg (`go test -race` with a coverage floor) and a `lint` leg, both with `GOWORK=off`; the `analyzer` job (syntax check, pytest, coverage floor); and the Helm job (dependency build, lint of both charts, `VALUES.md` drift check, kubeconform across modes). Build and release workflows run on demand (`workflow_dispatch`; chart publishing also runs on a `v*` tag).
- Coverage floors are a ratchet: raise them as coverage improves, never lower them.
- Service code that uses a symbol present only in an unreleased local checkout of a shared module builds in the workspace and fails in CI. Releasing that module and bumping the pin is a prerequisite, and it is the user's job; say so in your report.
- Any edit under `.github/`, or to a pinned tool version, follows the `github-actions-edit` skill: pin every action to its latest released tag and audit all `uses:` lines, not only the ones you touched. Stale action majors run on deprecated Node runtimes and fill every run with warnings.

## Releases and versions

- `main` is the stable trunk, and PRs target it. Check the current branch before editing or building: an old feature branch can lag main's version bumps, so a build from it pushes to a stale tag while the cluster keeps running the newer one, and the deploy silently does nothing. If you're not on `main` and the task doesn't say which branch to use, ask. Don't merge, rebase or switch branches on the user's behalf.
- Humans own git: don't commit, push or tag unless asked.
- Don't edit `services.<svc>.version` in `charts/telark/values.yaml` or the versions in `Chart.yaml`; the build and release workflows bump them. Don't build or push images, publish charts or apply changes to a cluster unless asked: "fix the bug" means code, chart values and docs. When a change needs a new image to take effect, or live verification you weren't asked to deploy for, say so in your report.
- `go.work` (including its `replace` directives), releasing shared modules and bumping their pins in `go.mod`, and `git push` are the user's work. Don't do them, offer them or report them as blockers. `go.work.sum` changes on its own during workspace builds and lint runs; leave it as it is. Don't commit `replace` directives in a `go.mod`.
- Third-party dependency updates (for example `go-redis` or `nats.go`) are ordinary work when the user asks for them.
- Commit messages follow Conventional Commits ([CONVENTIONS.md](CONVENTIONS.md#commits-branches-prs)).

## Skills index

Claude Code discovers these automatically; other agents can open the files directly.

| Skill | When to use | Path |
|---|---|---|
| `go-service-change-gate` | Before calling any change to a Go service done | [`.claude/skills/go-service-change-gate/SKILL.md`](.claude/skills/go-service-change-gate/SKILL.md) |
| `helm-chart-change` | Changing anything under `charts/`, or adding or changing a service env var | [`.claude/skills/helm-chart-change/SKILL.md`](.claude/skills/helm-chart-change/SKILL.md) |
| `analyzer-ci-gate` | Before calling any change to `services/analyzer` done | [`.claude/skills/analyzer-ci-gate/SKILL.md`](.claude/skills/analyzer-ci-gate/SKILL.md) |
| `deploy-dev-cluster` | The user asks for a deploy to the dev cluster, or has started a build-deploy-verify loop | [`.claude/skills/deploy-dev-cluster/SKILL.md`](.claude/skills/deploy-dev-cluster/SKILL.md) |
| `github-actions-edit` | Editing anything under `.github/`, or bumping a pinned CI tool version | [`.claude/skills/github-actions-edit/SKILL.md`](.claude/skills/github-actions-edit/SKILL.md) |
| `graph-explore-codebase` | Mapping structure with the code-review-graph MCP server, when it has this repo indexed | [`.claude/skills/graph-explore-codebase/SKILL.md`](.claude/skills/graph-explore-codebase/SKILL.md) |
| `graph-debug-issue` | Tracing a bug through call chains, flows and recent changes with the graph | [`.claude/skills/graph-debug-issue/SKILL.md`](.claude/skills/graph-debug-issue/SKILL.md) |
| `graph-review-changes` | Risk-ranked review of a diff with the graph | [`.claude/skills/graph-review-changes/SKILL.md`](.claude/skills/graph-review-changes/SKILL.md) |
| `graph-refactor-safely` | Renames, dead-code checks and refactors with graph previews | [`.claude/skills/graph-refactor-safely/SKILL.md`](.claude/skills/graph-refactor-safely/SKILL.md) |

## Related docs

- [CONVENTIONS.md](CONVENTIONS.md): naming, errors, logging, test layout, commits and PRs.
- [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): setup and every make target.
- [docs/INSTALL.md](docs/INSTALL.md), [charts/telark/README.md](charts/telark/README.md), [docs/CRDS.md](docs/CRDS.md), [docs/PUBLISHING.md](docs/PUBLISHING.md).
- [docs/architecture/](docs/architecture/README.md), [docs/security/](docs/security/README.md) and [docs/testing/](docs/testing/README.md): service relationships and flows, the security model and its invariants, and how to build, test and validate from a fresh clone.
- [.claude/FEATURE_IMPLEMENTATION_GUIDELINES.md](.claude/FEATURE_IMPLEMENTATION_GUIDELINES.md): how to design and ship a feature so it survives production, with the edge-case checklist and definition of done.
- [.claude/WORKFLOWS_MODEL_ROUTING.md](.claude/WORKFLOWS_MODEL_ROUTING.md): model and effort routing for sub-agents and workflow scripts, and the lean workflow shape. Two newer rules apply on top of it:
  - A sub-agent may run on Fable for a genuinely hard fix: a cross-service root cause, a concurrency, locking or data-integrity bug, or a fix that already failed once on Opus. Routine checks, API tests and simple fixes stay on Opus. State the reason when you choose Fable.
  - Resume a workflow run only when its agent call order is deterministic (sequential, or `parallel()` over a fixed array). Resuming a promise- or DAG-scheduled script reorders the cached calls and re-runs finished steps; run the remaining steps as plain agents or as a new sequential workflow instead.
- `.claude/plans/`: design and plan documents for in-flight features.
