# Telark documentation

Telark is a protection gate for your Kubernetes applications: you decide what can change an app, and when.

## Get started

- [Getting started](getting-started.md): install, sign in, protect a first application and see a violation, in about 10 minutes.

## Concepts

- [Concepts](concepts.md): applications, protection plans, change history and rollback, Insights, and access control.

## Install and configure

- [Install and configure](INSTALL.md): prerequisites, sizing modes, exposing the dashboard, first admin, networking, GitOps, upgrades and uninstall.
- [Chart README](../charts/telark/README.md): chart values explained, analyzer runtime profiles and air-gapped models.
- [CRD chart](../charts/telark-crds/README.md): installing the CRDs on their own.
- [Login and SSO](../services/auth/OIDC.md): turning on Google sign-in.

## Reference

- [Chart values](../charts/telark/VALUES.md): every value, generated from `values.yaml`.
- [Custom resources](CRDS.md): the `telark.io/v1alpha1` kinds, fields, labels and finalizers.
- [Security policy](../SECURITY.md): reporting vulnerabilities, supported versions, hardening notes.

## Internals

For contributors and reviewers. Start with [CONTRIBUTING.md](../CONTRIBUTING.md).

- [Architecture](architecture.md): services, data flow and subcharts on one page.
  - [Components and flows](architecture/README.md): call graph, Redis and NATS usage, startup and cross-service flows.
  - [Protection plans](architecture/protection-plans.md): lifecycle, Kyverno policies, health, violations and reports.
- [Security model](security/README.md): authentication, authorization, RBAC, trust boundaries and invariants.
- [Architecture decisions](adr/).
- [Testing and validation](testing/README.md): tool versions, build and test commands, CI parity and cluster checks.
- [Development](DEVELOPMENT.md): make targets.
- [Publishing](PUBLISHING.md): packaging, signing and pushing the Helm charts.
- Service READMEs: [discovery](../services/discovery/README.md), [exporter](../services/exporter/README.md), [auth](../services/auth/README.md), [notifier](../services/notifier/README.md), [analyzer](../services/analyzer/README.md) and [how the analyzer works](../services/analyzer/ARCHITECTURE.md).
