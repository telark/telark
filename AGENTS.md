# Agent instructions for telark

Rules and workflows for coding agents (Claude Code, Codex, Cursor, Copilot, Gemini and others) in this repository. This file is canonical: `CLAUDE.md` only imports it, and the skills under `.claude/skills/` hold the step-by-step procedures. [CONVENTIONS.md](CONVENTIONS.md) is the human rulebook for naming, errors, logging, test layout, commits and PRs. Where a rule is machine-enforced (`.golangci.yml`, `.github/workflows/ci.yaml`, `Makefile`), the enforcing file wins; report drift instead of working around it.

## Repository map

| Path | Contents |
|---|---|
| `services/auth`, `services/discovery`, `services/exporter`, `services/notifier` | Go services (packages of the root module `github.com/telark/telark`): `main.go` plus single-purpose packages under `internal/`; tests under `internal/tests/<area>/` |
| `internal/data`, `internal/rest`, `internal/kcore`, `internal/x-ware` | Shared Go packages the services import: `data` CRD types, constants and Kyverno policy rendering; `rest` the HTTP clients, endpoints and router; `kcore` Kubernetes client helpers; `x-ware` the authorization middleware, CORS, Redis and NATS helpers. Tests under `tests/<area>/` |
| `services/analyzer` | The analyzer: a Python/FastAPI service that runs local open-weight models through Ollama |
| `charts/telark` | Application chart: services, subcharts (Redis, NATS JetStream, Kyverno, metrics-server, optional VPA, Ollama), sizing presets in `modes/`, and `values.dev.yaml` (local port-forward settings that Helm never loads) |
| `charts/telark-crds` | CRDs, a standalone chart published to OCI; `charts/telark` depends on it by exact version (re-pinned by `release-charts` on `chart=both`) |
| `docs/` | `INSTALL.md`, `DEVELOPMENT.md` (every make target), `PUBLISHING.md`, `CRDS.md`, ADRs; `architecture/`, `security/` and `testing/` for agents and contributors |
| `.github/` | CI (`workflows/ci.yaml`), build and release workflows, composite actions in `actions/` and their scripts in `scripts/` |
| `.claude/` | Agent skills (`skills/`) |

`make help` lists the Makefile targets; `docs/DEVELOPMENT.md` says when to use each.

## System map

What telark is, where things live, and the facts to know before touching code. The linked docs hold the detail, and the code wins over any doc.

### What telark is

A control plane for **protection plans** over Kubernetes workloads. It groups workloads into applications, binds policy templates to a scope (applications or namespaces) and a time window, enforces them at admission through Kyverno, verifies them against live cluster state, and records change history with rollback. It installs as one Helm chart (`charts/telark`) with its CRDs (`charts/telark-crds`).

### Services

| Service | Lang | Role | Reads first |
|---|---|---|---|
| `exporter` | Go | Owns every telark CRD and the snapshot and report volumes; seeds built-in access roles, categories and the `TelarkConfig`; the only stateful service | `services/exporter/README.md` |
| `discovery` | Go | Groups workloads into applications, leader-elected reconcile loop, change history and rollback, protection-plan lifecycle, approvals, violations, reports, Kyverno policies | `services/discovery/README.md` |
| `auth` | Go | Passkey (WebAuthn) and Google OIDC login, sessions, user/group/role deletion cleanup | `services/auth/README.md`, `OIDC.md` |
| `notifier` | Go | Consumes the `telark.applications.update` JetStream events discovery publishes and upserts `Application` through the exporter. Nothing publishes deletes: application deletes go through discovery's reset. Also delivers webhooks: the events the exporter queues on the Redis stream `notif:events` go, signed, to the `app.webhooks` targets | `services/notifier/README.md` |
| `analyzer` | Python/FastAPI | Local incident analysis with an open-weight model (Ollama) over read-only cluster tools; insights in Redis; SSE to the UI | `services/analyzer/README.md`, `ARCHITECTURE.md` |
| `ui` | (separate repo) | Dashboard SPA, `telark/dashboard-ui` (stable branch `main`); this repo ships only its image reference (`services.ui`) | none |

Service relationships and flows: [docs/architecture/](docs/architecture/README.md).

### How it fits together

- Everything that persists goes through the **exporter's HTTP API** (`rest` clients, `/api/v1/...`, port 8080). Only the exporter writes telark CRDs; discovery also patches `applications/status`.
- **discovery** watches the cluster, publishes application events to NATS, stores snapshots and history through the exporter, pushes analysis jobs to the Redis stream `insights:jobs`, and creates and deletes the Kyverno `NamespacedValidatingPolicy` objects of active plans.
- **auth** produces sessions; every other API service validates them through the same `x-ware` middleware (the analyzer asks auth).
- Redis is shared coordination and cache (leader locks, queues, dedup, insights, authz cache). It requires a password (chart-generated Secret, `REDIS_PASSWORD`) but stays **untrusted**, since every service holds it: authorization cache entries are HMAC-signed.

### Security model (essentials)

Full model, RBAC table and invariants: [docs/security/](docs/security/README.md).

- **Browsers** send the session as the HttpOnly `telark_session` cookie (`__Host-telark_session` when every passkey origin is https, one name per install), which x-ware turns into `X-Session-Token`; API clients send that header. A change that relies on the cookie alone also needs `X-Requested-With`. The token is 32 random bytes; only `session-<sha256>` is stored (a `Session` CR), it never appears in a URL, and that name is no credential.
- **Services** send `X-Service-Token` (`TELARK_SERVICE_TOKEN`, one chart-generated Secret). A valid service token is Internal and skips grant checks, so it is equivalent to full API access.
- **Every route** has an explicit requirement in `services/<svc>/internal/authz/requirements.go` (Public, Authenticated, Internal, or a level on a scope, optionally with a deny rule). Unmapped routes are denied.
- **Grants** (`x-ware/authz`): levels ReadOnly < Contributor < Owner < Admin per scope, `ALL` covers every scope; roles come from the user and the user's groups; only active users and active, unexpired roles count; **deny rules beat any level**.
- **Guards** in handlers limit what a caller may change (for example, you can't grant a role above your own level or edit your own roles), in `services/exporter/internal/authz/` and `services/auth/internal/authz/guard.go`.
- **Kubernetes**: each service has its own ServiceAccount. discovery and the analyzer have a ClusterRole (discovery has wide write access for rollback and plan policies, the analyzer is read-only), the exporter has a namespaced Role that names each `telark.io` resource, and auth, notifier and ui have none. The optional CRD write guard (`app.security.crdGuard`) restricts direct CRD writes.

### Kubernetes objects

- One CRD group, `telark.io` `v1alpha1`: `Application`, `ProtectionPlan`, `TelarkConfig` (these three with a status subresource), `Category`, `User`, `Group`, `AccessRole`, `Passkey`, `Session`; all namespaced in the release namespace. The object name is the identity (no `spec.id`). Use FQ names (`applications.telark.io`) or short names (`tapp`). Reference: [docs/CRDS.md](docs/CRDS.md).

### Protection plans

Phases `draft`, `pending_approval`, `scheduled`, `active`, `terminated`, `canceled`, `failed` (`data/plans/protectionplan.go`). discovery drives the lifecycle; approval is `automatic` or `required` (Production is always required). While active, discovery renders the plan's templates into Kyverno `NamespacedValidatingPolicy` objects labeled `telark.io/protection-plan=<id>`, in `audit` or `enforce` mode, minus scope exclusions, and checks their health against the cluster. Detail: [docs/architecture/protection-plans.md](docs/architecture/protection-plans.md).

### Build, test, validate

All Go code is one module with one root `go.mod`, and every Go command runs from the repository root. Don't add a `go.work`, a nested `go.mod` or `replace` directives, so every machine and CI build the same way.

```sh
go build ./... && go vet ./... && go test -race ./... && golangci-lint run ./services/<svc>/...   # lint one service or ./internal/<pkg>/... at a time
helm repo add vpa https://charts.fairwinds.com/stable && make deps && make helm-lint && make helm-validate && make values-docs
# analyzer: see docs/testing (Python 3.14 venv, two pytest runs, coverage >= 95)
```

Tool versions (Go 1.27.3, golangci-lint v2.14.0, kubeconform v0.8.0, Python 3.14), install commands, CI coverage floors and cluster validation (UI → API → backend → Kubernetes): [docs/testing/](docs/testing/README.md). No cluster is available in a cloud session; unit tests, lint, Helm lint and render, kubeconform and the analyzer suite all run without one.

## Working agreements

### 1. Surface assumptions

Don't assume. Don't hide confusion. Surface tradeoffs. Before implementing:

- State your assumptions explicitly. Minor choices (naming, formatting, default values, which approach among equivalents): pick a reasonable option and note it rather than asking. Scope changes, destructive or irreversible actions, or requirements you can't infer from the request or the code: ask first.
- If multiple interpretations exist, present them; don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.

### 2. Simplicity first

Minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked, no abstractions for single-use code, no "flexibility" or "configurability" that wasn't requested, no error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it. Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical changes

Touch only what you must. Clean up only your own mess. The test: every changed line should trace directly to the user's request.

- When editing existing code, don't "improve" adjacent code, comments, or formatting, don't refactor things that aren't broken, and match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it; don't remove pre-existing dead code unless asked.
- Remove imports, variables and functions that your changes made unused.

### 4. Goal-driven execution

Define success criteria. Loop until verified. Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

Verify each step against its success criterion before moving on. Strong criteria let you loop independently; weak ones ("make it work") need constant clarification.

### In this repository

- **Comments** explain only a non-obvious why (a hidden constraint, an invariant, a workaround), in at most two lines, in Go, Python, templates and `values.yaml` alike. No header comments above functions, types or fields: well-named identifiers already say what the code does, and a comment that needs more than two lines usually means the code needs simplifying.
- **New code ships tested.** Every new or changed line of Go or Python comes with tests that cover it, in the same change, so Codecov's patch check passes. A line no test can reach is named in your report with the reason, never left uncovered silently.
- **Gates stay strict.** Run linters with the repository's checked-in config: no `--no-config`, override flags or `//nolint`, because they hide real issues and diverge from CI; when a finding is unclear, read the config. Don't weaken a test to make a change pass (if it asserts wrong behavior, flag it), and don't relax a CI check.
- **Sub-agents** start without your context. Brief them with the rules that apply to their task (test location, constants, no version bumps, the lint gate) and check their output against those rules before reporting done.
- **Truncated output.** Some agent shells filter long or multi-document command output, sometimes even through a `>` redirect. When completeness matters (`helm template`, multi-document YAML, long greps, file dumps), compare counts with what you expect and re-run unfiltered before concluding that something is broken.
- **Code navigation.** If the code-review-graph MCP server is available and `list_graph_stats` shows this repository indexed, prefer it for callers, impact-radius and test-coverage questions (see the `graph-*` skills); otherwise use normal search.

## Go services

**Layout and ownership**

- Domain logic (orchestration, validation, Kubernetes operations, business rules) lives in its own package under `services/<svc>/internal/<domain>/`. Handlers only decode the request, call the domain package and write the response; controllers and background loops reuse the same package. The logic stays reusable, and handlers stay thin enough to test in isolation.
- The exporter owns CRD operations and the storage behind them; other services reach them over HTTP through `rest` clients. The exporter implements those handlers itself and never calls a `rest` client (reusing the module's request and response types is fine), since calling itself over HTTP would be a circular dependency.
- For a new cross-service domain, build in this order: endpoint constants and types in `rest` (`endpoints/<domain>/`), handlers in the owning service wired to its routes, the `rest` client (`clients/<domain>/`), then the callers. A client without a server is dead code, and the handler defines the wire contract.
- Discovery uses `rest` clients only through wrappers in `services/discovery/internal/clients/<domain>.go` (see `applications.go`), which hold its timeouts, circuit breaker and error classification. A few older call sites construct clients directly; don't extend that pattern.
- The shared packages `internal/{data,rest,kcore,x-ware}` belong to the same module as the services, so a change to one lands in the same diff as its callers. No service-specific logic in a shared package. New Kubernetes client logic (informers, dynamic watchers, listers, informer factories) belongs in `kcore`; read it first, it likely has what you need.

**Code**

- Every static string, config value, log message and error string goes in the package's `constants/` folder, not inline in logic. Write no bare `""`, `0` or `1`: use the named constants (`constants.EmptyString`, `constants.DefaultInitValue`, `constants.DefaultIncrementValue`, …; check the service's `constants/` first). `revive`'s `add-constant` rule is on, so magic numbers and repeated strings fail lint.
- The messages the auth commands print (results, errors, usage and flag texts, link formats) live in `services/auth/cmd/outputs.go`, not in `constants/`.
- A literal repeated in two or more files of a package (operation or kind names, JMESPath expressions, operators, label keys, regexes, format strings) moves to a shared constants file in that package, so there is one place to audit and edit.
- When another feature needs a constant defined in a feature's own `constants.go`, move it to the service's root `constants/` package, remove the original and update the callers; don't import one feature's constants from another, because that creates false coupling. If the constant really belongs to that feature, keep the boundary and define your own.
- All `type` declarations of a package go in one `types.go`; methods and functions stay in their domain files. Don't create empty or doc-only files.
- Remove an unused parameter from the signature and update every caller rather than renaming it `_`. `_ Type` is only for signatures fixed by an interface you don't own.
- Prefer the `slices` and `maps` packages over hand-written loops for membership, index, sort, clone and delete.
- Match the naming, structure and style of the file you're editing. Complexity and function-length limits come from `.golangci.yml`; extract a helper instead of relaxing them. Naming, error and logging rules are in [CONVENTIONS.md](CONVENTIONS.md).
- Spelling is American English everywhere: code, comments, docs, UI copy and CRD descriptions ("enrollment", "behavior", "canceled"). `misspell` enforces it in Go (`locale: US`).
- An expected 4xx (a refusal, a bad body) is an answer, not a failure: send it with `SendResponse`, because `LogAndSendResponse` logs `[ERROR]` for any non-nil error whatever the status. Exporter handlers decode bodies through `GetSpec`/`GetSpecFor` (`utils/shared/extract.go`), the one place that maps body errors to 413, 422 and 400; calling `ParseRequestBody` directly bypasses it.
- Kyverno policies are `policies.kyverno.io/v1` CEL policies built with Kyverno's own types from `github.com/kyverno/api`, the light API module; the protection health check decodes live specs into the same types, so a hand-added field still counts as drift. Every string a user or the cluster controls reaches CEL through `CELString`/`CELList` (`internal/data/policies/cel.go`). Production code never imports `github.com/kyverno/kyverno`: it links cosign and three cloud SDKs into every binary that imports it. Tests may: discovery's `kyverno_accepts` and behavior tests compile and judge every template with Kyverno's own CEL compiler and matcher (its vpol engine needs Kyverno's Pod Security fork, which this module can't carry).

**Configuration**

- A new configurable value (timeout, interval, size, rate, feature flag) is an env var read at runtime: an `Env*` name and a `Default*` fallback in the service's `constants/`, a loader in `config/`, and the cluster-facing value under `services.<svc>.env` in `charts/telark/values.yaml` (plus `charts/telark/modes/*.yaml` when it varies by install mode). Add it to the chart even when the Go default is off, so operators can see and flip it.
- To tune an existing value, override it in the chart and leave `Env*` and `Default*` untouched; name the env var and the recommended value in your report. `Default*` constants are the conservative baseline every environment inherits. Per-cluster tuning, staged rollouts and emergency switches belong to the deployment layer, so a cluster can change them without a new image.

**CRDs**

- **CRD schema first.** The CRDs are strict structural schemas. A field added to a Go type before the CRD schema declares it is pruned by the API server: the write returns 200, the data is gone, and nothing is logged. Edit the CRD YAML in `charts/telark-crds/templates/crds/` first, then the Go type, and verify with a read-back instead of trusting the status code.
- Stored objects outlive releases, and rolling back one release must still work ([CRDS.md](docs/CRDS.md#schema-changes-across-releases)):
  - New fields are optional. Give one a CRD `default:` when its value is fixed, and have code read a missing field as that default.
  - Never remove or rename a field in one release ("expand, then contract"): release N stops writing the old field and migrates the data, then release N+1 drops it from the schema.
  - Clients send only the fields they change, never a whole stored object.
  - Data a `default:` cannot fill (a value computed from other data, a renamed field, objects to rewrite so a dropped field leaves storage) gets an idempotent step appended to `Steps` in `services/exporter/internal/migrations`, numbered one past the last, with the version that ships it in `Release` (a newest step without one allows no rollback). List every schema change and step in the version's upgrade notes in `docs/INSTALL.md`.
- A new CRD or subresource also goes into the exporter's Role (`charts/telark/templates/exporter/rbac/role.yaml`), which names each `telark.io` resource and verb instead of a wildcard, so anything missing from it fails with a 403 at runtime.

**Tests and verification**

- Tests live under `services/<svc>/internal/tests/<area>/` (`internal/<pkg>/tests/<area>/` for a shared package) as separate packages, table-driven, reusing the `testutil` helpers. Don't add `*_test.go` or `*_internal_test.go` files beside production code. Production packages stay test-free, and each service's tests sit in one tree. Never add test seams to production code: no `*ForTest` functions and no exported setters or constructors that exist only for tests or mutate package state. Test through the exported API production already uses; export an unexported function only when production calls it the same way. The one in-package test file is discovery's `coalesce_internal_test.go`.
- CRD reads in tests go to a fake apiserver: seed `dynamicfake.NewSimpleDynamicClient` and inject it with `k8sclient.SetDynamicClient` (reset with `SetDynamicClient(nil)`). `ConvertToCRDTemplate` drops `id` and `deletionTimestamp` from the spec, so set those in metadata. Clients built with `NewDynamicClientWithRateLimit` (rollback controller, informers) ignore the injected one and `rest.InClusterConfig` reads a fixed token path, so the few functions only those paths reach stay exported with a one-line why, and informer-filling tests live in `coalesce_internal_test.go`. Discovery keeps the first excluded-namespace list per process (one list per test package, through `testutil.ExcludedNamespaces`); stdout-capture tests see only per-call loggers (rest `base.GetLogger()`), not cached ones.
- Process-wide state outlives a test run: the cached Redis clients, the excluded-namespace list, the snapshot stats cache and the logger's command-output switch. A test package that binds the cached Redis client starts one embedded server in `TestMain`, and a test that needs such state unset gets its own package, so `go test -count=2 -shuffle=on ./...` stays green.
- Before calling a Go change done, run the `go-service-change-gate` skill: build, vet and test across the module, and `golangci-lint run` per service and shared package last, with zero errors.

**Known pitfall**

- Protection-plan violations come from Kubernetes Events filtered against the plan's rendered policies. `Terminate` and `Cancel` call `CleanupByPlanID`, which deletes the policies, and then blank the rendered-policy list, so a finished plan reports zero violations; Events also expire after an hour (`RetentionWindow` in the violations package). Anything that needs a finished plan's history has to capture it durably before `CleanupByPlanID` runs, and state the window it covers; the run ledger behind reports is that record (it keeps the whole run), and the violations route serves it with `window=run`.

## Helm chart

- The CRD API group is the constant `telark.io` (version `v1alpha1`), and labels, annotations and finalizers use the `telark.io/` domain, in both charts and in the Go code ([ADR 0003](docs/adr/0003-constant-api-group-telark-io.md)). `.Values.app.name` (default `telark`) only prefixes object names: derive resource names with the `telark.name` and `telark.fullname` helpers instead of hardcoding `telark`, and don't source names from `.Chart.Name` (the CRD chart is named `telark-crds`).
- Wire subchart values with the least mechanism that works: nothing when the subchart's own `values.yaml` default is right, a `<subchart>:` block in the parent only to override a default, and `global.*` only when many subcharts need the same value.
- Sizing is shared: `includeResources: true` applies `app.shared.resources`, and the `app.mode` presets (`modes/minimal.yaml`, `modes/performance.yaml`; `standard` is `values.yaml` itself) resize it. Don't add per-service `resources:` blocks or template branches for them, because per-service overrides drift from the shared block and hide where values come from. If one service truly needs different sizing, change the shared value or ask.
- Reuse the existing chart directories and subcharts (for example, install an existing subchart as its own release) instead of creating parallel chart folders.
- Keep `values.yaml` lean, like a product release: a header of about a dozen lines, a one-line inline comment only for a non-obvious why, no dividers and no per-key narration. Per-key documentation goes in the chart's `README.md`.
- `values.schema.json` rejects unknown keys in several blocks, so a new key there needs a schema entry; `make helm-lint` catches a missing one.
- Vendored subcharts (`charts/*/charts/`) are git-ignored and rebuilt by `make deps`; `Chart.lock` is committed.
- Follow the `helm-chart-change` skill for the lint, render and validate gate and for the docs that change with a value.

## Analyzer (Python)

The analyzer is `services/analyzer` (chart key `services.analyzer`, image `analyzer`). Its `README.md` and `ARCHITECTURE.md` describe the pipeline.

- It runs only free, open-weight models on open-source runtimes (Ollama). No commercial AI providers and no API keys, not even as an optional or bring-your-own-key tier: the product promise is a free, source-available, privacy-first analyzer, and provider keys were removed on purpose.
- The only deployment switch is air-gapped versus connected. Air-gapped: no egress, models pre-loaded (`app.ollama.autoPull=false`). Connected: egress only to fetch open models or to reach the customer's own self-hosted Ollama-API endpoint (`app.ollama.runtimeUrl`, a URL, no key).
- It has to answer in seconds on small CPU-only nodes. Designs that need minutes, or a GPU by default, don't fit.
- `main.py` is the only root module. The rest sits in domain packages: `api/` (HTTP app, session checks), `analysis/` (incident runs, rules, messages, the setup review, recommendations, the insight document), `core/` (config, constants, models, helpers, logging, events, the exporter client), `providers/` (Ollama client, model runtime), `prompts/` and `tools/`. A new module goes into its domain's package, and a new package needs its own `COPY` line in the Dockerfile (`tests/test_dockerfile_cov.py` fails without it).
- Log through `core/app_logger.py`, not `print()`, and keep strings in the existing `core/constants.py` and `analysis/messages.py` split. There is no formatter or linter config, so match the surrounding code.
- No regex with nested or overlapping quantifiers on the same characters (`(a+)+`, `\s+\d.*$`): SonarCloud rates it a critical vulnerability (python:S5852, catastrophic backtracking). Prefer `str` methods or a linear pattern. Go's `regexp` is linear and safe.
- Before calling a change done, run the `analyzer-ci-gate` skill.

## Images

- Each service `Dockerfile` copies its files root-owned and read-only (`COPY` without `--chown`) and runs as a non-root `USER`, and the analyzer's `pip install` reads only the hash lock (`--require-hashes`). SonarCloud rates a copied file the runtime user can write (docker:S6504) and an install without hashes (S8544) as vulnerabilities.

## Docs

- Docs change in the same change as the behavior they describe; stale docs mislead users and the drift compounds. For a new or changed chart value or flag: usage and non-obvious gotchas in `docs/INSTALL.md`; a value-reference row in `charts/<chart>/README.md` linking to that INSTALL section (check that the anchor resolves); a regenerated `VALUES.md`; then a sweep of `README.md`, `docs/PUBLISHING.md`, `CONTRIBUTING.md` and the service READMEs for references the change makes stale.
- User docs use the literal `telark` names and labels; don't add "if you changed app.name, replace…" caveats. Operational procedures (uninstall teardown, for example) are copy-paste command blocks in the docs, not scripts.
- Shipped files (docs, code comments, docstrings, test fixtures) never contain a personal email address or a real person's name. Contact, maintainer, disclosure and bootstrap-admin fields use `contact@telark.io`; examples and tests use `test@example.com`, never a person-like placeholder. The repositories and charts are public, and a comment ships as publicly as a README.
- Reusable guideline and checklist docs stay feature-agnostic: describe the general mechanism, not one feature's keys, names or scenarios, so the doc applies to the next feature.
- Generated files are regenerated, never edited: `VALUES.md` (`make values-docs`) and `CHANGELOG.md` (the release workflow prepends each version's section with git-cliff; `make changelog` previews it).
- `docs/security/` changes in the same diff as any change to authentication, authorization, RBAC or the service token.

## CI and GitHub Actions

- `.github/workflows/ci.yaml` is the merge gate: per Go service and shared package, a `test` leg (`go test -race` with a coverage floor, its profile uploaded to Codecov) and a `lint` leg, both run from the module root; the `analyzer` job (syntax check, pytest, coverage floor); and the Helm job (dependency build, lint of both charts, `VALUES.md` drift check, kubeconform across modes). Build and release workflows run on demand (`workflow_dispatch`; chart publishing also runs on a `v*` tag).
- Coverage floors are a ratchet: raise them as coverage improves, never lower them.
- Any edit under `.github/`, or to a pinned tool version, follows the `github-actions-edit` skill: pin every action to its latest released tag and audit all `uses:` lines, not only the ones you touched. Stale action majors run on deprecated Node runtimes and fill every run with warnings.

## Releases and versions

- `main` is the stable trunk, and PRs target it. Check the current branch before editing or building: an old feature branch can lag main's version bumps, so a build from it pushes to a stale tag while the cluster keeps running the newer one, and the deploy silently does nothing. If you're not on `main` and the task doesn't say which branch to use, ask. Don't merge, rebase or switch branches on the user's behalf.
- Humans own git: don't commit, push or tag unless asked.
- Don't edit `services.<svc>.version` in `charts/telark/values.yaml` or the versions in `Chart.yaml`; the build and release workflows bump them. Don't build or push images, publish charts or apply changes to a cluster unless asked: "fix the bug" means code, chart values and docs. When a change needs a new image to take effect, or live verification you weren't asked to deploy for, say so in your report.
- Third-party dependency updates (for example `go-redis` or `nats.go`) are ordinary work when the user asks for them.
- Commit messages follow Conventional Commits ([CONVENTIONS.md](CONVENTIONS.md#commits-branches-prs)).

## Skills index

Claude Code discovers these automatically; other agents can open the files directly.

| Skill | When to use | Path |
|---|---|---|
| `go-service-change-gate` | Before calling any change to a Go service done | [`.claude/skills/go-service-change-gate/SKILL.md`](.claude/skills/go-service-change-gate/SKILL.md) |
| `helm-chart-change` | Changing anything under `charts/`, or adding or changing a service env var | [`.claude/skills/helm-chart-change/SKILL.md`](.claude/skills/helm-chart-change/SKILL.md) |
| `analyzer-ci-gate` | Before calling any change to `services/analyzer` done | [`.claude/skills/analyzer-ci-gate/SKILL.md`](.claude/skills/analyzer-ci-gate/SKILL.md) |
| `github-actions-edit` | Editing anything under `.github/`, or bumping a pinned CI tool version | [`.claude/skills/github-actions-edit/SKILL.md`](.claude/skills/github-actions-edit/SKILL.md) |
| `graph-explore-codebase` | Mapping structure with the code-review-graph MCP server, when it has this repo indexed | [`.claude/skills/graph-explore-codebase/SKILL.md`](.claude/skills/graph-explore-codebase/SKILL.md) |
| `graph-debug-issue` | Tracing a bug through call chains, flows and recent changes with the graph | [`.claude/skills/graph-debug-issue/SKILL.md`](.claude/skills/graph-debug-issue/SKILL.md) |
| `graph-review-changes` | Risk-ranked review of a diff with the graph | [`.claude/skills/graph-review-changes/SKILL.md`](.claude/skills/graph-review-changes/SKILL.md) |
| `graph-refactor-safely` | Renames, dead-code checks and refactors with graph previews | [`.claude/skills/graph-refactor-safely/SKILL.md`](.claude/skills/graph-refactor-safely/SKILL.md) |

## Related docs

- [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): setup and every make target.
- [docs/INSTALL.md](docs/INSTALL.md), [charts/telark/README.md](charts/telark/README.md), [docs/CRDS.md](docs/CRDS.md), [docs/PUBLISHING.md](docs/PUBLISHING.md).
- [docs/architecture.md](docs/architecture.md) (services, flows, Redis and NATS, startup) and [SECURITY.md](SECURITY.md).
