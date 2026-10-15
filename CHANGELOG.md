# Changelog

All notable changes to Telark are documented here.

## [0.1.2](https://github.com/telark/telark/compare/v0.1.1...v0.1.2) - 2026-10-15

### 🐛 Bug Fixes

- Adjsut CHANGELOG ([#80](https://github.com/telark/telark/pull/80)) ([5c6e11c](https://github.com/telark/telark/commit/5c6e11caf5a3d1cdbb32b99f40c9bd7e7f123872))
- Name a suspended account in its 403 ([#84](https://github.com/telark/telark/pull/84)) ([f372d85](https://github.com/telark/telark/commit/f372d85a51a7fdf7205e53e0d40b0c3d40d05d1a))
- Let go-webauthn verify every passkey registration ([#84](https://github.com/telark/telark/pull/84)) ([11254dd](https://github.com/telark/telark/commit/11254dd75777d5439f69e86f2c87124ba545149c))
- Spend an enrollment link only when a passkey is saved ([#84](https://github.com/telark/telark/pull/84)) ([74e35ab](https://github.com/telark/telark/commit/74e35ab403d1f42eecc8caf6cbdb3be5732f1ae5))
- Print clear break-glass enroll steps ([#84](https://github.com/telark/telark/pull/84)) ([29f9d15](https://github.com/telark/telark/commit/29f9d159f4af5d75bd2142ecb700e0b4e70b68fa))
- Fix auth lint issue ([#84](https://github.com/telark/telark/pull/84)) ([cc4926e](https://github.com/telark/telark/commit/cc4926ef238e404e8fae2df3ad3f80653416cd9c))

### 🔐 Security

- Build on Go 1.27.2 for its standard library fixes ([#84](https://github.com/telark/telark/pull/84)) ([01a0637](https://github.com/telark/telark/commit/01a06370e57cfb23ec5f481cba0669c7b8f7dd3e))
- Bump golang.org/x/net to v0.61.0 ([#84](https://github.com/telark/telark/pull/84)) ([f38463e](https://github.com/telark/telark/commit/f38463e2bdcb564bca3ab4d2d3ddfe4f9548c001))

### 📚 Documentation

- Recommend a second passkey for the bootstrap admin ([#84](https://github.com/telark/telark/pull/84)) ([e3050d3](https://github.com/telark/telark/commit/e3050d3c8b45c95050ede2620ce59ab83dfb8a13))
- Add new passkeys requirements ([#84](https://github.com/telark/telark/pull/84)) ([9089d84](https://github.com/telark/telark/commit/9089d84e78b3849cd1083a2a62a045623fb62b98))
- Lint with the golangci-lint version CI pins ([#84](https://github.com/telark/telark/pull/84)) ([79b748b](https://github.com/telark/telark/commit/79b748b3fec08bb04c8e0b173375dc8502eb2b74))

### 🖥️ Dashboard UI

v0.1.1 → [v0.1.2](https://github.com/telark/dashboard-ui/releases/tag/v0.1.2)

#### [0.1.2](https://github.com/telark/dashboard-ui/compare/v0.1.1...v0.1.2) - 2026-10-15

##### 🐛 Bug Fixes

- End the session when the account is suspended ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([a131018](https://github.com/telark/dashboard-ui/commit/a131018e5fa7b53cb944177ec5f6ed8cd1b5ca16))
- Show the model runtime and the model details as one list ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([14195e7](https://github.com/telark/dashboard-ui/commit/14195e7250d323ca421c965b694ad9a1c82614de))
- Name Google in sign-in settings and color links from the theme ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([66a08c8](https://github.com/telark/dashboard-ui/commit/66a08c8d6572a70ff5a6571a6a77ee9f8abf9966))
- Give panels and dialogs one footer from the theme ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([3440411](https://github.com/telark/dashboard-ui/commit/344041178271237fa3963d6a06cb85e8eb441a49))
- Show a user-switch icon for the account-state action ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([26e2a7e](https://github.com/telark/dashboard-ui/commit/26e2a7ebf13720f323d4d51122a14be08353f594))
- Point the auth footer's terms link at the license page ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([b0534b8](https://github.com/telark/dashboard-ui/commit/b0534b8aeb5d39b476796efe834f2e3af8f312f2))
- Name a next step in every no-passkey message ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([ee77775](https://github.com/telark/dashboard-ui/commit/ee777753cadbf4fa47b96c883eb39a5c5a7b07ba))
- Show each login error once, in the card ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([d310771](https://github.com/telark/dashboard-ui/commit/d310771d7004215c8ac0e67963dc67fc1c7cbd53))
- Keep the enrollment link through a failed passkey prompt ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([a9cd0db](https://github.com/telark/dashboard-ui/commit/a9cd0db66aec8dcbbc0f57fe107a8d177f507302))
- Drop the green glow behind the login brand panel ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([9d851bd](https://github.com/telark/dashboard-ui/commit/9d851bd411c73d27eedc4f5c5e8bc9f4d7d1de7d))
- Measure element width when the node attaches ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([60e6d98](https://github.com/telark/dashboard-ui/commit/60e6d9884536325182e2cbe22e62452ca821e81d))
- Show absolute times on focus and to screen readers ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([9473eb2](https://github.com/telark/dashboard-ui/commit/9473eb20b422dc1392efefb0635c21e7b833cc13))
- Align colors, labels, confirms and home icons ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([a8e825f](https://github.com/telark/dashboard-ui/commit/a8e825f4c6c7082fd85881c856def16024f733d1))
- Say what an audit plan does and download reports from one button ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([f775396](https://github.com/telark/dashboard-ui/commit/f7753963bf07279c724c1ff5a5cae9e82300a0ee))
- Open at once, keep input on a failed save and manage focus ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([0fa83d3](https://github.com/telark/dashboard-ui/commit/0fa83d33a44057cc10ecfc7163eda0667798332c))
- Pin row actions, keep tables still and open rows from the keyboard ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([70124bf](https://github.com/telark/dashboard-ui/commit/70124bf8a27ed514407f942dd457dd30a2415ead))
- Load the right values, enable Save only on a change, clear warnings ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([b8751e9](https://github.com/telark/dashboard-ui/commit/b8751e930f7621735b75916505557dc01473a0b2))
- Split the member, group and role view panels into tabs ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([089a82b](https://github.com/telark/dashboard-ui/commit/089a82b1b4e3477a743361f36eb20a6d86481836))

##### ⚡ Performance

- Load less up front, poll less and re-render less ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([5cc4881](https://github.com/telark/dashboard-ui/commit/5cc4881c180620dbceb7b219a5caa1636ff81a1a))
- Start the sidebar slide at once and keep navigation light ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([99e88ef](https://github.com/telark/dashboard-ui/commit/99e88efbfd9b7812a79716a5e3befc83fec55053))

##### 📚 Documentation

- Describe the view panel tabs, stacked panels and role rows ([#46](https://github.com/telark/dashboard-ui/pull/46)) ([27b8f76](https://github.com/telark/dashboard-ui/commit/27b8f76c1b4150849440cc39add2ee808c69ede4))

##### Contributors

- [@hourki](https://github.com/hourki)


### Contributors

- [@hourki](https://github.com/hourki)

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

