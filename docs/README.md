# telark docs

- [Architecture](architecture.md) — services, data flow, subcharts.
  - [Components and flows](architecture/README.md) — call graph, Redis/NATS, startup, cross-service flows.
  - [Protection plans](architecture/protection-plans.md) — lifecycle, Kyverno policies, violations, reports.
- [Security model](security/README.md) — authentication, authorization, RBAC, trust boundaries, invariants.
- [Testing and validation](testing/README.md) — tool versions, build and test commands, CI parity, cluster checks.
- [Development](DEVELOPMENT.md) — make targets.
- [Install guide](INSTALL.md) — prerequisites, CRDs, app, sizing modes, verification.
- [CRD reference](CRDS.md) — custom resource groups and kinds.
- [Architecture decisions](adr/) — ADRs.
- [Publishing](PUBLISHING.md) — package, push, sign, and register the Helm charts (OCI / GHCR).

Service deep dives:

- [Analyzer architecture](../services/analyzer/ARCHITECTURE.md) — AI insights pipeline.
- [OIDC / SSO architecture](../services/auth/OIDC.md) — Google SSO admin config and login flow.
