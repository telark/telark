# 2. app.name is the identity source of truth

Date: 2026-07-19

## Status

Accepted

## Context

The app identity (`telark`) appears in three places that must agree:

- Resource names rendered by the `telark` chart (`telark-exporter-service`, …).
- CRD API groups rendered by the `telark-crds` chart (`erpi.telark`, `auth.telark`, `classification.telark`).
- The Go services, which **hardcode** those API groups (e.g. `discovery` watches `erpi.telark/applicationsasresources`).

Helm's idiomatic source for a chart name is `.Chart.Name`. That does not work here: the two charts have different chart names (`telark` and `telark-crds`), so `.Chart.Name` would make the CRD groups `erpi.telark-crds` and break the contract the Go code depends on. The identity is also not free to vary at deploy time, because the Go groups are fixed.

## Decision

`.Values.app.name` (default `telark`) is the single source of truth for the app identity, in **both** charts. Helper templates (`telark.name`, `telark.fullname`) default to it, overridable via `nameOverride` / `fullnameOverride`. Nothing derives identity from `.Chart.Name`, and the literal string `telark` is never hardcoded in templates — names, API groups, policy names, and annotation domains all derive from `app.name`. Helper *identifier* names (`"telark.labels"`, etc.) remain literal, as Helm requires; they never render into output.

## Consequences

- Both charts render a consistent identity even though their chart names differ.
- The CRD chart carries its own `app.name: telark` default so it renders valid standalone; at the real install the `telark` chart's values are authoritative.
- Changing the identity is a coordinated change across both charts *and* the Go source — it is not a per-chart values tweak.
- A repo-wide `grep telark` over rendered output should return zero static literals; only helper identifiers remain in source.
