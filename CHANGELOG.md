# Changelog

All notable changes to telark are documented here.
Commits follow [Conventional Commits](https://www.conventionalcommits.org).

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

