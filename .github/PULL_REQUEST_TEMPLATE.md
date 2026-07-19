## What

<!-- What does this change do, and why? Link the issue it closes. -->

Closes #

## Type

- [ ] Feature
- [ ] Fix
- [ ] Refactor / chore
- [ ] Docs
- [ ] Chart / infra

## Checklist

- [ ] Change is scoped — every changed line traces to the goal above.
- [ ] `make lint` passes (golangci per service, zero errors; no `//nolint` workarounds).
- [ ] `make test` passes.
- [ ] Chart changes: `make helm-lint` clean; `helm template` renders.
- [ ] No chart/module version bumps; no committed `replace` directives.
- [ ] Docs updated if behavior or values changed.

## Notes for reviewers

<!-- Anything non-obvious: tradeoffs, follow-ups, migration/upgrade impact. -->
