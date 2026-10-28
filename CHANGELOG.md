# Changelog

All notable changes to Telark are documented here.

## [0.2.0](https://github.com/telark/telark/compare/v0.1.2...v0.2.0) - 2026-10-27

### ✨ Features

- Keep stored data valid across upgrades ([#87](https://github.com/telark/telark/pull/87)) ([0acc3f8](https://github.com/telark/telark/commit/0acc3f8c68b5ce3c065cee65a4b7a424310d4cd3))
- Send new critical incidents to every Insights reader ([#87](https://github.com/telark/telark/pull/87)) ([3775863](https://github.com/telark/telark/commit/377586385fe8301128a2903c1225f0ab5fc1d078))
- Keep a plan's reports on delete and delete any report ([#87](https://github.com/telark/telark/pull/87)) ([1c6e491](https://github.com/telark/telark/commit/1c6e4914837ae3f609ed782a006d9409ad1e1a01))
- Add plan kinds and repeating windows ([#87](https://github.com/telark/telark/pull/87)) ([c3acb36](https://github.com/telark/telark/commit/c3acb3666286c36815641e68c6bd6379d2509302))
- Push list changes over SSE and deliver webhooks ([#87](https://github.com/telark/telark/pull/87)) ([6c6d08e](https://github.com/telark/telark/commit/6c6d08e832ce4970e68a221a3a483308b33cb68c))
- Move plan policies to Kyverno's CEL ValidatingPolicy ([#87](https://github.com/telark/telark/pull/87)) ([53aa48c](https://github.com/telark/telark/commit/53aa48c3ef16ee4b97846e21ea3599ee37a0bde3))
- Pin the bootstrap account and approve break-glass requests ([#87](https://github.com/telark/telark/pull/87)) ([3da00e2](https://github.com/telark/telark/commit/3da00e238facefdb7e3ea87c1885e57948fd280c))
- Keep the browser session in an HttpOnly cookie ([#87](https://github.com/telark/telark/pull/87)) ([aed085b](https://github.com/telark/telark/commit/aed085b517904464326c0b24cfad5743217685aa))
- Rate limit the public sign-in starts ([#87](https://github.com/telark/telark/pull/87)) ([c870c82](https://github.com/telark/telark/commit/c870c82531373f1013b99d3ed8a9908fbf9c5c4c))
- Keep the bootstrap account private ([#87](https://github.com/telark/telark/pull/87)) ([be49807](https://github.com/telark/telark/commit/be49807da82bb61ac2191565241c04cb7e900bf2))
- Group the security values under app.security and shorten the notes ([#87](https://github.com/telark/telark/pull/87)) ([d605994](https://github.com/telark/telark/commit/d605994fb2b060fbe96420a9890a44607980cb9a))
- Make the break-glass wait a chart value and cover the patch lines ([#87](https://github.com/telark/telark/pull/87)) ([765340e](https://github.com/telark/telark/commit/765340ed540bc4a1b6c5e25752cf2f2f388173c0))

### 🐛 Bug Fixes

- Sign registration entries and harden break-glass links ([#87](https://github.com/telark/telark/pull/87)) ([c40aefb](https://github.com/telark/telark/commit/c40aefb5bc842fcf4abcf4bf82ad01077030a26d))
- Size ephemeral storage, and let auth wait 180 s for Redis ([#87](https://github.com/telark/telark/pull/87)) ([77936cf](https://github.com/telark/telark/commit/77936cf370eae179c2bc292c3602df9b288adc39))
- List the exporter Role's resources instead of wildcards ([#87](https://github.com/telark/telark/pull/87)) ([72adab4](https://github.com/telark/telark/commit/72adab47fa0785e8abacaa255524b005864c8f59))
- Release the force-sync lock with WithoutCancel ([#87](https://github.com/telark/telark/pull/87)) ([fedfd3b](https://github.com/telark/telark/commit/fedfd3b9372de5c7b45531f9e8739940ca1e8693))
- Refuse unknown deny rules on role writes ([#87](https://github.com/telark/telark/pull/87)) ([baa1107](https://github.com/telark/telark/commit/baa110795afcdba3a0f57a082b64c2e7a474ca0f))
- Keep an app's images, ports and env keys in order ([#87](https://github.com/telark/telark/pull/87)) ([4909ce3](https://github.com/telark/telark/commit/4909ce36f7c2ee37a28590a238853c6a95b21c05))
- Make break-glass recovery deny-only, link first and wipe after ([#87](https://github.com/telark/telark/pull/87)) ([f20fe9b](https://github.com/telark/telark/commit/f20fe9b6868cb35c4c727ed71a7a11665398d3a9))
- Retry break-glass's last passkey look and state the one-replica limit ([#87](https://github.com/telark/telark/pull/87)) ([ca3b647](https://github.com/telark/telark/commit/ca3b64761b25971dbf10f36490771b5e492da65e))

### 🖥️ Dashboard UI

v0.1.2 → [v0.2.0](https://github.com/telark/dashboard-ui/releases/tag/v0.2.0)

#### [0.2.0](https://github.com/telark/dashboard-ui/compare/v0.1.2...v0.2.0) - 2026-10-27

##### ✨ Features

- Make the plan page prove the plan works ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([47bb137](https://github.com/telark/dashboard-ui/commit/47bb1376ad18a6c845927be6b3becbc478a01ffd))
- Add Get started, app protection status and the create review ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([2ee1b4c](https://github.com/telark/dashboard-ui/commit/2ee1b4ca94a32c9d434f03175abaaade7978a8b1))
- Open critical incidents from the bell ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([3d50e8b](https://github.com/telark/dashboard-ui/commit/3d50e8bf11d5144a2c3b80d9ea1a2eda3dcdceb9))
- Add Needs attention, docs links and help on empty Applications ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([14687be](https://github.com/telark/dashboard-ui/commit/14687be35b056c1a1b0bc640a3f2b219d592f80b))
- Keep a plan's reports on delete and delete any report ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([701af3d](https://github.com/telark/dashboard-ui/commit/701af3d513bfa88f392aabcfd9916dd624121c0a))
- Add plan kinds and repeating windows ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([fd23a06](https://github.com/telark/dashboard-ui/commit/fd23a06e21a88525bc9f21c2d41b7fbe6f80600f))
- Update the bell from the live stream ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([54c2363](https://github.com/telark/dashboard-ui/commit/54c23636bfe26673cb45936099e8511129a19941))
- Approve break-glass requests from the dashboard ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([8e2b617](https://github.com/telark/dashboard-ui/commit/8e2b617234ffa4e9e3b18d7c135b630536fa4c6f))
- Keep the session in an HttpOnly cookie ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([b66d401](https://github.com/telark/dashboard-ui/commit/b66d4018157fba78340dbacbad749d3214594b0c))
- Rework the cards and the application pages ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([af3a62c](https://github.com/telark/dashboard-ui/commit/af3a62cc1b3130a4457f6b82f7621cda5b9de1d8))
- Let the bootstrap account choose break-glass alerts ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([0e59809](https://github.com/telark/dashboard-ui/commit/0e59809bbf1a318fd9ef6f9dc251da5a96c6891a))
- Restyle protection plans like applications ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([54cebd4](https://github.com/telark/dashboard-ui/commit/54cebd44924673526e962f0c6945ddc9d5581ba1))
- Restyle the home boxes like the cards ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([f8e1316](https://github.com/telark/dashboard-ui/commit/f8e1316c457e5742332a5b2d03a8537b9e592a63))
- Make recovery deny-only and show Admins only the requests ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([b8843f3](https://github.com/telark/dashboard-ui/commit/b8843f326347b36756e49fd1f4ba6f6c3353da16))
- Show the configured break-glass wait ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([d126d71](https://github.com/telark/dashboard-ui/commit/d126d7129a97c4033110459f44a1dde7ab806c45))

##### 🐛 Bug Fixes

- Read enrollment links from the URL fragment ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([fbf107a](https://github.com/telark/dashboard-ui/commit/fbf107a9b24ecf24514376802c445b3e4f323aca))
- Show facts as text, confirm wide grants and fix pill contrast ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([685fa85](https://github.com/telark/dashboard-ui/commit/685fa85c5716a3d3838b8cc71ac47b5be6281358))
- Fix zero-plan protection status, empty-state copy and headings ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([8b4aa07](https://github.com/telark/dashboard-ui/commit/8b4aa07dc104e5a7a84b44a32984fb955c786a6b))
- Use gold warnings, a readable red and no blue pills ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([a1053e3](https://github.com/telark/dashboard-ui/commit/a1053e371873ab2bbde2c181fcdaa33b31eabac0))
- Show an error state with Retry on the detail pages ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([03264a8](https://github.com/telark/dashboard-ui/commit/03264a8457ed8bb35bbe888179cb03fb6f0ad78c))
- Drop the Members bootstrap pill and fix Home and group copy ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([e4e450d](https://github.com/telark/dashboard-ui/commit/e4e450d22903326d6a92e5d30b6754f8ae32eff6))
- Give each color one job ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([eeb1d0d](https://github.com/telark/dashboard-ui/commit/eeb1d0d343c41456883693cbe784f9251fdc65dc))
- Keep a toolbar button readable when it disables itself ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([3bbcbda](https://github.com/telark/dashboard-ui/commit/3bbcbda1629dfb8f122d0672f4eab7737c0d7b12))
- Center page empty states ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([3d4c0db](https://github.com/telark/dashboard-ui/commit/3d4c0db93e07deb4273e25a48e0910aa023f310e))
- Make dialog notes neutral ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([281c824](https://github.com/telark/dashboard-ui/commit/281c824402651b087e7a58d612321888a9e8cde0))
- Show a check in the empty notifications panel ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([d833fd8](https://github.com/telark/dashboard-ui/commit/d833fd83e9451d329a26414b5b68dc17934fe224))
- Clear the noticed UI issues ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([2d979fd](https://github.com/telark/dashboard-ui/commit/2d979fdec85ded88eb90714bed17c94ec30b88d3))
- Clear the SonarCloud findings and the Firefox empty-state gap ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([ee455f0](https://github.com/telark/dashboard-ui/commit/ee455f0a2cb13bf840d4ff533e998e054b3dbf6b))

##### 📚 Documentation

- State the chart's CPU limit per mode ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([2ba1409](https://github.com/telark/dashboard-ui/commit/2ba1409460f60cc7bb27a7f085ce251e6890a2f9))
- State CI, the new red's contrast and the color rules in the agent rules ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([cca65b9](https://github.com/telark/dashboard-ui/commit/cca65b90cfbaa3f3ce389fa7a46c128fb6906985))
- State the plan card, picker, loader, empty-state and home box rules ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([9797f55](https://github.com/telark/dashboard-ui/commit/9797f55a405dfdaa07ce776c4af06a3c7705386f))
- Point CLAUDE.md at AGENTS.md and add the SonarCloud rules ([#53](https://github.com/telark/dashboard-ui/pull/53)) ([64606c0](https://github.com/telark/dashboard-ui/commit/64606c0181b19d9a590af3295cacd7bb1f9d342f))

##### Contributors

- [@hourki](https://github.com/hourki)


### Contributors

- [@hourki](https://github.com/hourki)

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

