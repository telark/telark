# CRD reference

Telark's custom resources are defined by the `telark-crds` chart. Every CRD is in the group **`telark.io`**, version **`v1alpha1`**, namespaced in the release namespace, and kept on uninstall (`helm.sh/resource-policy: keep`); to remove them, follow the [full teardown](INSTALL.md#full-teardown). The group is constant ([ADR 0003](adr/0003-constant-api-group-telark-io.md)).

`kubectl get telark -n telark` lists every Telark object except passkeys and sessions (`kubectl get telark-auth -n telark`). Several plurals collide with other CRDs (Argo CD installs `applications.argoproj.io`), so use the fully qualified name (`applications.telark.io`) or the short name (`tapp`).

## Kinds

| Kind | Plural (FQ name) | Short | Category | `/status` | Purpose |
|---|---|---|---|---|---|
| `Application` | `applications.telark.io` | `tapp` | `telark` | yes | A discovered application: workloads grouped into one unit. Scope target for protection plans. |
| `ProtectionPlan` | `protectionplans.telark.io` | `tplan` | `telark` | yes | Policy templates bound to a scope and a time window. |
| `TelarkConfig` | `telarkconfigs.telark.io` | `tcfg` | `telark` | yes | Cluster-wide settings. Singleton: the schema accepts only the name `default`. |
| `Category` | `categories.telark.io` | `tcat` | `telark` | no | Classification categories. Singleton list: one object named `categories`. |
| `User` | `users.telark.io` | `tuser` | `telark` | no | A user in the role model. |
| `Group` | `groups.telark.io` | `tgroup` | `telark` | no | An access group in the role model. |
| `AccessRole` | `accessroles.telark.io` | `trole` | `telark` | no | A role: scopes granted and denials, at a level. |
| `Passkey` | `passkeys.telark.io` | `tpk` | `telark-auth` | no | A registered WebAuthn passkey credential. |
| `Session` | `sessions.telark.io` | `tsess` | `telark-auth` | no | An authenticated session. |

## Identity

The object name (`metadata.name`) is the identity; there is no `spec.id`. The exporter's REST view adds `id` from `metadata.name` on every read and strips `id` on write. References between objects hold names: `roleRefs`, `groupRefs`, `userRefs`, `participantRefs` and `scope.applicationRefs` name objects, while `categoryRef`, `environmentRef` and `tagRefs` name items in the `categories` object (items keep their own `id`, pattern `cat-…`).

## Spec and status

| Kind | `spec` | `.status` |
|---|---|---|
| `Application` | `name`, `displayName`, `description`, `managed` | `health`, `resourceCount`, `namespaces`, `resourceSummary`, `resources`, `images`, `ports`, `envVarKeys`, `configMapRefs`, `secretRefs`, `serviceMappings`, `ingressRules`, `metrics`, `snapshots`, `rollbacks`, `history`, `lastForceSync`, `createdAt`, `lastUpdated`, `conditions` (type `Published`: status `True`/`False`, reason `Pending`, `Created` or `Failed`) |
| `ProtectionPlan` | `name`, `description`, `severity`, `priority`, `scope` (`type`, `namespaces` or `applicationRefs`, `exclusions`), `policies`, `mode` (`audit` \| `enforce`), `timeMode`, `timeRange`, `approvalMode` (`automatic` \| `required`; absent = automatic; Production defaults to `required`), `participantRefs`, `environmentRef`, `tagRefs` (at most 20), `createdAt/By`, `lastUpdatedAt/By` | `phase`, `reason`, `conditions` (`Ready`, `Approved`, `PoliciesHealthy`), `observedGeneration`, `renderedPolicies`, `health`, `healthCheckedAt`, `healthDetail`, `startedAt/By`, `terminatedAt/By`, `approval` (state, requester, decider, comment, bounded history; written only by discovery) |
| `TelarkConfig` | `excludedNamespaces`, `ai` (`enabled`, `model`: a local analyzer model tag, default `granite4:350m`; `autoAnalyze`), `snapshots`, `oidc` (`enabled`, `googleClientID`, `egressAllowed`), `selfRegistration` (`enabled`, default off). `oidc` and `selfRegistration` are written only by auth, for the bootstrap admin | `cluster.version` |
| `Category` | `categories[]`: `id`, `name`, `scope` (`groups`, `roles`, `plan-environments`, `plan-tags`), `type`, `creationDate`, … | none |
| `User` | `username`, `fullname`, `email`, `roleRefs`, `groupRefs`, `bootstrap`, `identities`, `avatar`, `settings`, `status` (`phase`: `active`, `inactive` or `suspended`; `lastLoginAt`; `invite`: `issuedAt`, `expiresAt`, `issuedBy` of a pending enrollment link, written only by auth and display only, since the link itself expires in Redis; `inviteAcceptedAt`: when a passkey last closed a pending link, also written only by auth) | none (lifecycle stays in `spec.status`) |
| `Group` | `name`, `description`, `userRefs`, `roleRefs`, `categoryRef`, `createdBy`, `lastUpdatedBy` | none |
| `AccessRole` | `name`, `version`, `priority`, `categoryRef`, `scopesAndPermissions`, `protection`, `status` (lifecycle), `validity`, `createdBy`, `lastUpdatedBy`, `deprecatedAt`, `deletedAt` | none (lifecycle stays in `spec.status`) |
| `Passkey` | `userId`, `credentialId` (at most 2 048 characters), `publicKey` (at most 4 096), `deviceName`, `deviceType`, `backupEligible`, `backupState`, timestamps | none |
| `Session` | `userId`, `createdTimestamp`, `expiresTimestamp`, `ipAddress`, `deviceMetadata`. Named `session-<sha256(token)>` (the schema rejects any other name); the token itself is never stored. | none |

The status subresource means a write to `spec` never changes `.status` and the reverse: writers send status fields to `/status`. The REST view flattens `.status` into the top level of the object, so API clients see one shape.

## OIDC trust anchor

The optional Google JWK set is not part of `TelarkConfig`: it lives in the Secret `telark-oidc-trust-secret` (key `googleJwkJson`), which the exporter writes when the bootstrap admin saves it through auth and the auth service reads as a mounted file. `GET /api/v1/config` merges it back into `oidc.googleJwkJson`. For cluster-less renders, point `app.auth.oidc.existingSecret` at a Secret you manage ([INSTALL.md](INSTALL.md#gitops-cluster-less-renders)).

## Labels and finalizers

- Kyverno policies rendered for a protection plan carry `telark.io/protection-plan=<plan id>`, `telark.io/template-id` and `app.kubernetes.io/managed-by=telark`, and the annotations `telark.io/plan-name`, `telark.io/created-by` and `telark.io/render-hash`.
- Users, groups and access roles carry the finalizers `telark.io/user-cleanup`, `telark.io/group-cleanup` and `telark.io/role-cleanup`, which the auth service clears.

## Ownership

The `exporter` service owns every `telark.io` resource and the OIDC trust Secret; `discovery` additionally patches `applications/status` (rollback records). The CRD write guard (`app.crdGuard`, on and enforcing by default) restricts direct writes to every `telark.io` resource and subresource, and to the OIDC trust Secret, to the owning service accounts; see [INSTALL.md](INSTALL.md#crd-write-guard).
