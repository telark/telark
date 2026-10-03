# Changelog

All notable changes to Telark are documented here.

## [0.1.0] - 2026-10-03

### 🐛 Bug Fixes

- Remove legacy k6 code ([#60](https://github.com/telark/telark/pull/60))
- Support one single bootstrap admin instead of a list ([#64](https://github.com/telark/telark/pull/64))
- Harden authz, discovery and chart for MVP ([#65](https://github.com/telark/telark/pull/65))
- Bootstrap user should no longer be able to change his email as its locked with the chart config.

### 📚 Documentation

- Add go linters to make-lint command ([#62](https://github.com/telark/telark/pull/62))

### Contributors

- [@hourki](https://github.com/hourki)

## [0.0.2] - 2026-09-28

### ✨ Features

- Add missing tests for all services to cover at least 30% for coverage and 50% for auth and exporte, add new contributions and conventions rules and optimize the starter kit docs

### 🐛 Bug Fixes

- Adjust ui config and remove leftovers in docs
- Remove unused inputs from GHA
- Update services versions manually due to broken workflows (cosign issues)
- Fix ListCleanupViews parsing by setting views in nested items

### 📚 Documentation

- Product-led README, getting started and concepts; tighten every… ([#56](https://github.com/telark/telark/pull/56))

### Contributors

- [@actions-user](https://github.com/actions-user)
- [@hourki](https://github.com/hourki)

[0.1.0]: https://github.com/telark/telark/compare/v0.0.2...v0.1.0
[0.0.2]: https://github.com/telark/telark/compare/v0.0.1...v0.0.2

