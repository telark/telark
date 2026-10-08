# Changelog

All notable changes to Telark are documented here.

## [0.1.1](https://github.com/telark/telark/compare/v0.1.0...v0.1.1) - 2026-10-08

### 🐛 Bug Fixes

- Delete the old model on a model switch
- Issue break-glass tokens as invites
- Never block on NATS, fully reset apps
- Add missing shared types
- Compute role priority from saved scopes
- Unblock drains and run without persistence

### 📚 Documentation

- Note metrics-server can be disabled when the cluster runs one
- Document trust boundaries and the VPA upgrade
- Add the copyright line to every license and publish Alpine sources with complete notices

### 🖥️ Dashboard UI

v0.1.0 → [v0.1.1](https://github.com/telark/dashboard-ui/releases/tag/v0.1.1)

#### [0.1.1](https://github.com/telark/dashboard-ui/compare/v0.1.0...v0.1.1) - 2026-10-08

##### 🐛 Bug Fixes

- Read refusal codes, keep enroll links
- Merge the Insights cards, refresh on save
- Show group-role access, focus row actions
- Keep disabled buttons dimmed

##### 📚 Documentation

- Add copyright, carve-out and notices
- List the port-forward commands

##### Contributors

- [@hourki](https://github.com/hourki)

#### [0.1.2](https://github.com/telark/dashboard-ui/compare/v0.1.1...v0.1.2) - 2026-10-08

No user-facing changes.

#### [0.1.1](https://github.com/telark/dashboard-ui/compare/v0.1.0...v0.1.1) - 2026-10-08

##### 🐛 Bug Fixes

- Read refusal codes, keep enroll links
- Merge the Insights cards, refresh on save
- Show group-role access, focus row actions
- Keep disabled buttons dimmed

##### 📚 Documentation

- Add copyright, carve-out and notices
- List the port-forward commands

##### Contributors

- [@hourki](https://github.com/hourki)


### Contributors

- [@hourki](https://github.com/hourki)

## [0.1.0] - 2026-10-04

### ✨ Features

- Add missing tests for all services to cover at least 30% for coverage and 50% for auth and exporte, add new contributions and conventions rules and optimize the starter kit docs
- Resolve actor names and let users read their own profile
- Add passkey enrollment links
- Redis password and the Enrolled state
- Install on any default storage class

### 🐛 Bug Fixes

- Adjust ui config and remove leftovers in docs
- Remove unused inputs from GHA
- Update services versions manually due to broken workflows (cosign issues)
- Fix ListCleanupViews parsing by setting views in nested items
- Remove legacy k6 code ([#60](https://github.com/telark/telark/pull/60))
- Support one single bootstrap admin instead of a list ([#64](https://github.com/telark/telark/pull/64))
- Harden authz, discovery and chart for MVP ([#65](https://github.com/telark/telark/pull/65))
- Bootstrap user should no longer be able to change his email as its locked with the chart config.
- Harden login states and membership deletes
- Harden guards, rollbacks and CI for MVP
- Size from measured load, add probes
- Hide the own namespace from pickers, drop the fetch interval
- Break-glass serves the bootstrap admin only
- Use stashed deps on build ([#75](https://github.com/telark/telark/pull/75))

### 📚 Documentation

- Product-led README, getting started and concepts; tighten every… ([#56](https://github.com/telark/telark/pull/56))
- Add go linters to make-lint command ([#62](https://github.com/telark/telark/pull/62))
- Simplify the docs and fix stale claims
- Add third-party notices and known limitations

### Contributors

- [@actions-user](https://github.com/actions-user)
- [@hourki](https://github.com/hourki)

[0.1.0]: https://github.com/telark/telark/releases/tag/v0.1.0

