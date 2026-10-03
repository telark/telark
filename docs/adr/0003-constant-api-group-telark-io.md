# 3. One constant API group, telark.io

Date: 2026-09-26

## Status

Accepted. Supersedes [ADR 0002](0002-app-name-is-the-identity-source-of-truth.md) for API groups, label, annotation and finalizer domains.

## Context

Under ADR 0002 the CRD groups were derived from `app.name` (three groups: `erpi.<name>`, `auth.<name>`, `classification.<name>`) while the Go services hardcoded `telark`, so the value was never free to change. The kinds carried suffixes (`ApplicationAsResource`, `UserAsResource`, `CategoryAsClassification`), status fields lived in `spec` with no status subresource, every object repeated its name in `spec.id`, and Kyverno labels used a domain (`telark.erpi/`) that no group owned. The REST API mirrored this with verb suffixes (`.../get`, `.../patch`) and `findby*` routes.

## Decision

- Every telark CRD is in one group, **`telark.io`**, version **`v1alpha1`**, namespaced in the release namespace. The group is a constant in the charts and in the Go code; it does not follow `app.name`.
- Kinds: `Application`, `ProtectionPlan`, `TelarkConfig`, `Category`, `User`, `Group`, `AccessRole`, `Passkey`, `Session`. Each CRD has short names and the `telark` (or `telark-auth`) category, so `kubectl get telark` lists them all.
- `ProtectionPlan`, `Application` and `TelarkConfig` have a status subresource; observed state lives in `.status`. `User` and `AccessRole` keep their lifecycle in `spec.status`, because it is admin intent.
- The object name is the identity: `spec.id` is gone and the REST view returns `id = metadata.name`.
- Labels, annotations and finalizers use the `telark.io/` domain (`telark.io/protection-plan`, `telark.io/template-id`, `telark.io/plan-name`, …); the managed-by label is the standard `app.kubernetes.io/managed-by=telark`.
- `app.name` only prefixes object names (Deployments, Services, Secrets, `telark-exporter-service`, …) through the `telark.name` and `telark.fullname` helpers. Nothing derives identity from `.Chart.Name`.
- The REST API is resource-oriented under `/api/v1/` (`applications`, `users`, `groups`, `accessroles`, `config`, `categories`, `protectionplans`, …) with HTTP verbs instead of path suffixes; service-only routes live under `internal/`.

## Consequences

- Renaming the app no longer touches API groups, so a second install with a different `app.name` shares the same CRDs (one install per cluster remains the supported layout).
- One group removes group-level isolation; the CRD write guard therefore matches every resource and subresource of `telark.io` (`*` and `*/*`) and admits discovery only for `applications/status`.
- The plurals `applications`, `users`, `groups`, `sessions` and `categories` collide with other CRDs (Argo CD's `applications.argoproj.io`, for example). Docs and scripts use fully qualified names (`applications.telark.io`) or the short names (`tapp`, `tuser`, …).
- The change is not upgradable in place: old CRDs, objects and `telark.erpi/*`-labeled Kyverno policies must be removed and the chart reinstalled. Users re-enroll passkeys and roles and groups are re-created.
- The Google JWK set moves out of the config CR into the Secret `telark-oidc-trust-secret`, guarded by its own admission policy.
