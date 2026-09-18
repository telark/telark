# Feature Implementation Guidelines

Reusable checklist for implementing any feature across telark services, charts and the
dashboard UI. [CONVENTIONS.md](CONVENTIONS.md) owns code style, naming and linting; this
file owns *how to design and ship a feature so it survives production*. Where they
overlap, CONVENTIONS.md wins.

Use it top to bottom. Every item is a question you should be able to answer with a
specific line of code, config key, or test.

---

## 0. Before writing code

- [ ] **State the goal as a verifiable outcome**, not an activity ("exactly one record per
      event at N replicas", not "fix duplicates").
- [ ] **Trace the real flow end to end** before choosing a design: producer → transport →
      consumer → storage → API → UI. Bugs live in the hop nobody read.
- [ ] **Check what already exists**: a shared-package helper, a rest client, a store key, a
      UI hook, a Helm helper. Re-implementing something three files over is the most
      common waste.
- [ ] **List your assumptions** about ordering, uniqueness, freshness and failure. Each one
      becomes an edge case (§9) or a test (§8).
- [ ] **Decide the scope boundary**: which service owns the logic (§2), which repo changes
      first (shared package → release → consumer), what stays untouched.
- [ ] If two interpretations are plausible, write both down and pick one *explicitly*.

---

## 1. Engineering discipline

- **Root cause over symptom.** A report names a symptom. Grep every caller of the function
  you are about to touch; fix once, where all callers route through.
- **Surgical diffs.** Every changed line traces to the goal. No drive-by refactors, no
  "while I'm here" formatting, no speculative flexibility.
- **Constants, not literals**, for every string, key, threshold and message. Repeated
  literals across files are hoisted to a shared constants file; a feature-scoped constant
  used elsewhere moves to the service root `constants/`.
- **Types are centralized** (one `types.go` per package). No doc-only or empty files.
- **No `any` in TypeScript**, no `_ param` placeholders in Go (remove the parameter and
  update callers), no `console.*` (use the project logger).
- **Comments only for the non-obvious why**, ≤ 2 lines. If you need more, the code is too
  complex.
- **Tunables are Helm values**, not hardcoded defaults: rates, worker counts, cache TTLs,
  concurrency limits, refresh intervals. Expose them as env vars sized by `app.mode`
  presets (`minimal | standard | performance`); keep the service `Default*` constants as
  safe fallbacks.
- **Never bump versions, build, push, tag or deploy unless asked.** Releases own versioning.
- **Docs are part of "done"**: any user-facing change updates the relevant README / INSTALL
  / chart docs in the same change. Generated docs are regenerated, never hand-edited.

---

## 2. Architecture and ownership

- **One owner per resource type.** The service that owns a resource is the only one that
  reads or writes it directly; every other service goes through its rest-pkg HTTP client.
  Client wrappers live in `clients/<domain>.go`; handlers never import rest clients.
- **Handlers are thin.** Domain logic lives in a top-level `<service>/<domain>/` package.
  Build the receiving service's handler first, then the client wrapper, then the caller.
- **Shared packages change upstream first**, get released, then the consumer bumps its
  pin. No `replace` directives, no service-specific logic in a shared package.
- **Kubernetes access goes through kcore** (informers, listers, dynamic watchers, manifest
  cleaning). Read it before adding K8s logic; it usually already has the primitive.
- **Cluster-singleton work is leader-gated** through the generic leader gate. The leader
  advertises itself by **pod IP, not pod name** (pod DNS fails during rollouts), and every
  singleton loop re-runs a catch-up on leadership acquisition.
- **Per-concern client budgets.** A hot path that shares a client or rate limiter with
  another concern starves it. Give expensive or latency-sensitive paths their own client,
  QPS budget or informer mirror.
- **Trust boundaries are named.** Anything crossing a service, user or cluster boundary is
  validated at the edge. A raw secret is never accepted where a hash or `self` would do.

---

## 3. API and data-flow design

- **Routes:** `/api/v1/<domain>/…`; every route declares an authz requirement and is
  covered by the "no scopeless routes" test. Self-service routes are `Authenticated` + an
  ownership guard, not a write-role requirement.
- **Status codes carry meaning.** `404` only for a real NotFound; limiter, deadline,
  transport and backend errors are `503` (+ `Retry-After` when useful); invalid input is
  `400`; a lost race is `409`. Callers branch on these, so a wrong mapping cascades
  (a `404`-on-throttle re-armed retries; an auth-backend error mapped to `401` logged every
  user out).
- **Payload views.** Large collections offer a summary view and a full view; the UI defaults
  to summary. Polled responses carry an `ETag` and honour `If-None-Match` → `304`.
- **Idempotent writes.** Any write that can be retried, replayed or raced is guarded by a
  lock or a fingerprint check and returns a stable result on replay.
- **Reject no-ops that would still mutate state** (`400`), instead of accepting them and
  bumping a counter or version.
- **Secrets never travel in URLs**, path segments, query strings or log lines. Tokens go in
  headers; references are hashed or `self`. Log formats use `$uri` without the query
  string; rest clients redact path and query values.
- **Pagination and bounds everywhere** a collection can grow. An unbounded read per request
  is an OOM waiting for load.
- **Slow-storage reads are cached** and refreshed on an interval, never computed per
  request.

---

## 4. State, events and consistency

- **Choose the source of truth per datum and write it down.** Every "which one wins?" bug
  came from two candidates.
- **Fingerprint what you record.** Keep a content fingerprint of the last recorded state
  and drop events whose input was already recorded. Status-only or controller-write events
  are artifacts, not changes.
- **Stamp the post-state on every write.** Any fallback that reconstructs from stored data
  runs only when the stamp matches; otherwise baseline silently. Fabricating a record is
  worse than skipping one.
- **Compare like with like.** Normalize both sides with the same function before diffing:
  strip the same server-added defaults and system annotations on both sides; treat empty
  vs. absent maps as equal.
- **Dedup keys are content-based, not name-based.** Name-keyed time-window dedup drops
  legitimate consecutive changes.
- **Every buffered pipeline has a cap, a backoff, a rate, and ACK-on-fail.** An unacked
  failure is requeued, not lost; a full buffer sheds with a metric, not a panic.
- **Coalesce on a key, flush on a schedule** — and the schedule must never publish a record
  whose prerequisite data is missing. Defer the outcome until it exists.
- **Catch-up reconcile is periodic and on leadership change.** State absorbed into an
  initial LIST is otherwise invisible; unknown items are baselined without publishing.
- **Readiness reflects own health only** (own store, own sync). A throttled dependency is
  reported as `degraded`, never as `503`, or rollouts stall.
- **TTLs are deliberate.** A too-short buffer TTL loses data; a bounded record TTL limits
  what a leader gap can reconstruct. Document what happens when each TTL expires.
- **Numeric decoding is explicit.** Generic JSON decode into `float64` silently breaks
  integer comparisons; decode into typed structs.

---

## 5. UI patterns

- **Polling is cheap or it is not polling.** `ETag`/`304` revalidation, windowed reads,
  summary views, one poller per tab.
- **The client flags a backend gap only on a definite `404`**, never on a transient blip.
- **Sizing and colour come from one source.** Control height/radius/font from
  `controls.ts` via theme tokens; status colours from `DEFAULT_COLORS`; never hex, `size=`
  props, inline heights or nested ConfigProviders. Fonts are forced globally; inline
  `fontFamily` is dead code.
- **Pick the palette by surface.** Token names can lie; choose colours for the surface they
  render on. Screenshot before fixing a colour bug you cannot see.
- **Measure, do not guess, responsive thresholds.** ResizeObserver on the container, not
  viewport media queries.
- **Sequential loaders render pixel-identical** or the switch reads as a glitch. One
  canonical full-page loader.
- **Layout owns chrome; pages own content.** Transitions use `useOutlet()`.
- **Never name internal vendors** in user-facing strings; centralize all validation, error
  and UI strings in a constants file.
- **Insecure contexts degrade with a themed banner**, not a console error.
- **Done in the UI** = `npm run check-all` with zero errors (warnings OK, no
  `eslint-disable`) + live smoke of every affected page with zero console errors.

---

## 6. Performance

- **Know the ceiling before shipping**: pick the scale target, measure the hot path at that
  scale, record the number.
- **Never a full LIST per request.** Serve from an informer-backed store; render once per
  (view, version), pre-compressed; bound concurrent renders with a semaphore that returns
  `503 + Retry-After` on overflow.
- **Rate-limit outbound bursts with per-key exponential backoff**, exposed as a tunable.
- **Worker pools, not single workers**, sized by profile, with a lifecycle tied to the
  leadership term so nothing leaks per term.
- **In-flight de-duplication** on cache miss (per-key lock): a miss storm produces one
  upstream call, not N.
- **Cache negative results deliberately.** Retrying every `404` is a self-inflicted flood;
  take the drop path on a real NotFound.
- **Cleanup is part of the feature**: orphan files, empty directories, stale keys, expired
  entries. Ship the GC with the write path, size its grace period, measure its runtime at
  scale on the real storage class.
- **Keep transport cheap**: gzip at the edge, keepalive to upstreams, `304` on unchanged.

---

## 7. Resilience and failure handling

- **Design for three replicas and a leader change mid-operation.** What happens if the
  leader dies after the write but before the ACK? After the ACK but before the stamp?
- **Backoff, cap and jitter every retry loop.** Unbounded retries saturate shared limiters
  and take down neighbours.
- **Locks for cross-replica races** (TTL-bound); one winner, explicit loser status.
- **Fail closed at trust boundaries, fail open on observability.** A missing metric never
  blocks a request; a malformed reference always does.
- **Degrade, do not cascade.** A throttled dependency downgrades a feature or a readiness
  detail, never the whole pod or the user's session.
- **Behaviour changes are documented as such** in the report and the docs.
- **Rollouts are verified live**: pods ready, leader elected, no reconcile storms, zero
  secrets in access logs, the affected API matrix returning the expected codes.

---

## 8. Testing and verification

- **Tests live in `internal/tests/<area>/`**, table-driven, using the shared `testutil`
  fakes. Never beside production code; expose a helper rather than writing an
  `_internal_test.go`.
- **Every bug fix ships a reproducing test first.** Every non-trivial branch, loop, parser
  or security path leaves one runnable check behind.
- **Cross-package coverage** (`go test -coverpkg=./... ./...`); CI ratchet floor never
  lowered. Python: `pytest` + `compileall`, stub suites in a separate process.
- **Never weaken an existing test to pass.** If it asserts wrong behaviour, flag it.
- **Lint with the project config**, zero errors, no `--no-config`, no `//nolint`.
  Complexity ≤ 12, functions ≤ 60 lines — extract a helper instead of relaxing the gate.
- **Verify in the real system, not only in tests**: build, roll out, then exercise the
  exact scenario that motivated the change. Each re-run of a fixed scenario tends to
  expose the next root cause the previous fix was hiding.
- **Stress before declaring capacity.** A feature touching list, flush, notify or storage
  paths runs at the scale target with a scripted change burst, a delete/recreate and a
  leader failover; duplicates, drops and OOMs are counted, not eyeballed.
- **Heavy command output is truncated by the `rtk` wrapper** — write it to a file or use
  the raw proxy before drawing conclusions.

---

## 9. Edge-case checklist

Walk this list for every feature; each item has bitten us at least once.

**Ordering and identity**
- [ ] Same key updated twice inside one dedup window.
- [ ] Event arrives before its prerequisite data exists (first tick after a restart or
      recreate).
- [ ] Event for an item the owning service does not know yet (`404` from the owner).
- [ ] Status-only / controller-only writes on a watched object.
- [ ] Empty map vs. absent map; server-added defaults; system annotations on one side only.
- [ ] Version equals current, is missing, or goes backwards.

**Concurrency and topology**
- [ ] Three replicas, one leader; leader changes mid-operation; leader gap longer than TTL.
- [ ] Two replicas trigger the same write in the same second.
- [ ] A worker leaks across leadership terms.
- [ ] Rollout restarts: pod DNS unavailable, pod-template annotation churn.

**Scale**
- [ ] Thousands of items, a hundred changed in one second, a namespace deleted and
      recreated.
- [ ] Cache miss storm; a full LIST behind every miss.
- [ ] Buffer full; consumer slower than producer; TTL expiry mid-pipeline.
- [ ] Thousands of orphan files/directories after a burst; GC runtime on network storage.

**Failure**
- [ ] Owner service throttled, slow or down — each maps to a distinct status and a distinct
      caller behaviour.
- [ ] Authz backend unavailable: users stay logged in, request gets `503`.
- [ ] Readiness with a degraded dependency.

**Security**
- [ ] Any token, secret or one-time link in a URL, log line, path parameter or query.
- [ ] Raw secret accepted where a hash / `self` should be.
- [ ] Self-service route blocked for a read-only role; admin-only route open to anyone.
- [ ] Insecure context (http) for browser crypto features.

**UI**
- [ ] Referenced backend object missing (definite `404`) vs. transient failure.
- [ ] Narrow container with sidebar open; loaders switching; light and dark surfaces.
- [ ] Multiple tabs polling; stale bundle after a rollout (calls get `400`, then reload).

---

## 10. Definition of done

1. Goal restated as an outcome and met, with evidence in the PR or report.
2. Diff is surgical; constants, types and strings centralized; no version bumps.
3. Lint and tests green with the project config; new behaviour has a reproducing test.
4. Tunables exposed as Helm values sized per `app.mode`; docs and generated docs updated.
5. Status codes, authz requirements and secret handling reviewed against §3.
6. Failure modes (§7) and edge cases (§9) walked and either handled or explicitly
   documented as accepted.
7. Deployed and verified live at the scale target for the paths it touches; follow-ups
   listed with an owner, not left implicit.
