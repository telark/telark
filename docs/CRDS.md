# CRD reference

telark's custom resources are defined by the `telark-crds` chart. The group suffix is the app identity `app.name` (default `telark`); below it is shown as `<name>`. All are kept on uninstall (`helm.sh/resource-policy: keep`); to remove them, follow the [full teardown](INSTALL.md#full-teardown).

## Group `erpi.<name>`

| Kind | Plural | Purpose |
|---|---|---|
| `ApplicationAsResource` | `applicationsasresources` | A discovered application — a group of workloads treated as one unit. Scope target for protection plans. |
| `GroupAsResource` | `groupsasresources` | An access group in the role model. |
| `RoleAsResource` | `rolesasresources` | A role: scopes granted and denials, at a level. |
| `UserAsResource` | `usersasresources` | A user in the role model. `spec.status.phase` is `active`, `inactive` or `suspended`. |
| `GlobalConfig` | `globalconfigs` | Cluster-wide settings set from the UI at runtime: local analyzer `spec.ai` = {enabled, model, autoAnalyze} (written by Owners via Settings; `model` is a local analyzer model tag, default `granite4:350m`; deep mode additionally requires the tools capability), OIDC (`oidc.googleJwkJson` at most 64 KiB). |
| `ProtectionPlan` | `protectionplans` | Policy templates bound to a scope and a time window; transitions pending_approval → scheduled → active → terminated; carries optional metadata environmentID / tagIDs (category ids, at most 20 tags); `approvalMode` (`automatic` \| `required`, absent = automatic; defaults to `required` for the Production environment) and `approval` (state, requester, decider, comment, bounded history; written only by discovery); `scope.exclusions` (kinds for both scope types, named resources for the applications scope only) narrows enforcement without changing policy names. |

## Group `auth.<name>`

| Kind | Plural | Purpose |
|---|---|---|
| `UserPasskey` | `userpasskeys` | A registered WebAuthn passkey credential (`credentialId` at most 2 048 characters, `publicKey` at most 4 096). |
| `UserSession` | `usersessions` | An active authenticated session. Named `session-<sha256(token)>` (the schema rejects any other name); the token itself is never stored. |

## Group `classification.<name>`

| Kind | Plural | Purpose |
|---|---|---|
| `CategoryAsClassification` | `categoriesasclassifications` (short: `cat`) | A classification category; scopes: groups, roles, plan-environments (protection-plan environments), plan-tags (protection-plan tags). |

## Ownership

The `exporter` service owns every resource in these groups; `discovery` additionally patches its own `applicationsasresources`. The CRD write guard (`app.crdGuard`, on and enforcing by default) restricts direct writes to these groups to the owning service accounts — see [INSTALL.md](INSTALL.md#crd-write-guard).
