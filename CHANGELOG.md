# Changelog

All notable changes to telark are documented here.
Commits follow [Conventional Commits](https://www.conventionalcommits.org).

## [0.3.0] - 2026-09-17

### [MAJOR]
- Fix all bugs for last storm test

## [0.2.1] - 2026-09-17

### Bug Fixes
- Fix login with passkeys with the proper error handling and remove the login ceremonu lock, adjust the modes config values

## [0.2.0] - 2026-09-17

### [MAJOR]
- Fix apps discovery by design by improving  applications snapshots persistance, fix redis missed dead cache logic on exporter single-get, improve concurrency on notifier and remove the silent apps patch drops, drop apis latency

## [0.1.3] - 2026-09-14

### Bug Fixes
- Fix ListCleanupViews parsing by setting views in nested items

### [MAJOR]
- Fix Issues in some services like Discovery in where when the exporter-service is not available and unreachable the discovery-service keeps sending requests on a broken REST CALLs, Remove Noisy & Sensitive Logs from all the backend services and keep only the ones that are necessary for users to view, Fix notifier nats connection failing  maxAttended retries

## [0.0.1] - 2026-07-20

### Bug Fixes
- Adjust ui config and remove leftovers in docs
- Remove unused inputs from GHA
- Update services versions manually due to broken workflows (cosign issues)

### Features
- Add missing tests for all services to cover at least 30% for coverage and 50% for auth and exporte, add new contributions and conventions rules and optimize the starter kit docs

### [MAJOR]
- Adjsut OIDC pipeline flow to relay on ui actions instead of injected envs,migrate enrichment shape to async to store data in redis and add new stable routes, migrate built-in CRDs instances to new seeder logic managed by exporter, enhance services quality and remove dead code with deep cleanup.
- Re-structure helm charts org and apply community standards

