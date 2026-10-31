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
| `ProtectionPlan` | `name`, `description`, `severity`, `priority`, `scope` (`type`, `namespaces` or `applicationRefs`, `exclusions`), `policies`, `mode` (`audit` \| `enforce`), `timeMode` (`permanent` \| `time_range` \| `repeating`), `timeRange`, `repeat` (`days`, `startTime`, `endTime`, `timeZone`; required when `timeMode` is `repeating`), `approvalMode` (`automatic` \| `required`; absent = automatic; Production defaults to `required`), `participantRefs`, `environmentRef`, `kindRef`, `tagRefs` (at most 20), `createdAt/By`, `lastUpdatedAt/By` | `phase`, `reason`, `conditions` (`Ready`, `Approved`, `PoliciesHealthy`), `observedGeneration`, `renderedPolicies`, `health`, `healthCheckedAt`, `healthDetail`, `startedAt/By`, `terminatedAt/By`, `approval` (state, requester, decider, comment, bounded history; written only by discovery), `window` (a repeating plan's open or next window, written only by discovery) |
| `TelarkConfig` | `excludedNamespaces`, `ai` (`enabled`, `model`: a local analyzer model tag, default `granite4:350m`; `autoAnalyze`), `snapshots`, `oidc` (`enabled`, `googleClientID`, `egressAllowed`), `selfRegistration` (`enabled`, default off), `breakGlassAlerts` (`notifyAdmins`, default on: whether Admins on `ALL` hear of break-glass requests and read them). `oidc`, `selfRegistration` and `breakGlassAlerts` are written only by auth, for the bootstrap admin | `cluster.version`, `lastMigration` and `rollbackMigration` (the exporter's [migration record](#migrations), written only at its startup and left out of the REST view), `bootstrap` (`userID`, `email`, `pinnedAt` of the pinned bootstrap account; `requests`, the newest 20 break-glass requests: `id`, `kind` `recover` \| `replace`, `targetEmail`, `state` `pending` \| `approved` \| `canceled` \| `completed`, `reason` `signed-in` \| `denied` \| `expired`, `openedAt`, `approvedAt`, `canceledAt`, `completedAt`; written only by the exporter, with a compare-and-swap on `resourceVersion`, and left out of the config view, which refuses the key) |
| `Category` | `categories[]`: `id`, `name`, `scope` (`groups`, `roles`, `plan-environments`, `plan-tags`, `plan-kinds`), `type`, `creationDate`, … | none |
| `User` | `username`, `fullname`, `email`, `roleRefs`, `groupRefs`, `bootstrap`, `identities`, `avatar`, `settings`, `status` (`phase`: `active`, `inactive` or `suspended`; `lastLoginAt`; `invite`: `issuedAt`, `expiresAt`, `issuedBy` of a pending enrollment link, written only by auth and display only, since the link itself expires in Redis; `inviteAcceptedAt`: when a passkey last closed a pending link, also written only by auth) | none (lifecycle stays in `spec.status`) |
| `Group` | `name`, `description`, `userRefs`, `roleRefs`, `categoryRef`, `createdBy`, `lastUpdatedBy` | none |
| `AccessRole` | `name`, `version`, `priority`, `categoryRef`, `scopesAndPermissions`, `protection`, `status` (lifecycle), `validity`, `createdBy`, `lastUpdatedBy`, `deprecatedAt`, `deletedAt` | none (lifecycle stays in `spec.status`) |
| `Passkey` | `userId`, `credentialId` (at most 2 048 characters), `publicKey` (at most 4 096), `deviceName`, `deviceType`, `backupEligible`, `backupState`, timestamps | none |
| `Session` | `userId`, `createdTimestamp`, `expiresTimestamp`, `ipAddress`, `deviceMetadata`. Named `session-<sha256(token)>` (the schema rejects any other name); the token itself is never stored. | none |

The schemas cap the fields people type (lengths, item counts, character sets) with the same limits the exporter checks before a write ([Input validation](security/README.md#input-validation)); the limits added in 0.2.1 are listed in the [upgrade notes](INSTALL.md#upgrade-notes).

The exporter's Role (`charts/telark/templates/exporter/rbac/role.yaml`) names each resource, subresource and verb it may use. A new CRD or subresource goes into it too, or the exporter gets a 403.

The status subresource means a write to `spec` never changes `.status` and the reverse: writers send status fields to `/status`. The REST view flattens `.status` into the top level of the object, so API clients see one shape.

## OIDC trust anchor

The optional Google JWK set is not part of `TelarkConfig`: it lives in the Secret `telark-oidc-trust-secret` (key `googleJwkJson`), which the exporter writes when the bootstrap admin saves it through auth and the auth service reads as a mounted file. `GET /api/v1/config` merges it back into `oidc.googleJwkJson`. For cluster-less renders, point `app.auth.oidc.existingSecret` at a Secret you manage ([INSTALL.md](INSTALL.md#gitops-cluster-less-renders)).

## Labels and finalizers

- Kyverno policies rendered for a protection plan carry `telark.io/protection-plan=<plan id>`, `telark.io/template-id` and `app.kubernetes.io/managed-by=telark`, and the annotations `telark.io/plan-name`, `telark.io/created-by` and `telark.io/render-hash`.
- Users, groups and access roles carry the finalizers `telark.io/user-cleanup`, `telark.io/group-cleanup` and `telark.io/role-cleanup`, which the auth service clears.

## Ownership

The `exporter` service owns every `telark.io` resource and the OIDC trust Secret; `discovery` additionally patches `applications/status` (rollback records). The CRD write guard (`app.security.crdGuard`, on and enforcing by default) restricts direct writes to every `telark.io` resource and subresource, and to the OIDC trust Secret, to the owning service accounts; see [INSTALL.md](INSTALL.md#crd-write-guard).

## Schema changes across releases

Stored objects outlive releases. An upgrade changes the schema and the code, not the objects already stored, and a rollback runs the previous release's code on them. So a schema change follows three rules:

- **New fields are optional.** Give a new field a CRD `default:` when its value is fixed, and have code read a missing field as that default. The API server fills a `default:` whenever it reads an object, so objects stored before the field existed show it too, but only inside a parent that exists: a new object field needs `default: {}` for the defaults under it to apply.
- **Expand, then contract.** Never remove or rename a field in one release. Release N stops writing the old field and migrates the data, then release N+1 drops it from the schema. That way, rolling back one release still works.
- **Clients send only the fields they change**, never a whole stored object. Request bodies are decoded strictly, so a stored field the service no longer knows gets the request refused.

### Migrations

Data a `default:` cannot fill goes into a migration step of the exporter, the only writer of these objects (`services/exporter/internal/migrations`): a value computed from other data, a renamed field to convert, or objects to rewrite so that a field dropped from the schema leaves storage.

- Steps are numbered and append-only: a released step keeps its number and is never removed or reordered. Each step names the version that ships it in `Release`. While the newest step names none, no older exporter starts on the data it migrated.
- At startup, before it serves and before it reconciles the built-ins, the exporter runs every step above the one recorded in `TelarkConfig` `.status.lastMigration`, in order, and records each step as it completes. With each step it records `.status.rollbackMigration`: the last step of the release before the newest step's. A failed step stops the exporter, and the restarted pod resumes at that step.
- Steps are idempotent: a crash between a step and its record runs it again, every replica that starts runs the pending steps, and the upgrade after a rollback runs the newer steps again.
- Steps follow "expand, then contract" (above), which is what makes one release back safe: the release before the one that ran a step still reads the data. So an exporter whose last step is below `lastMigration` but not below `rollbackMigration` starts: it lowers `lastMigration` to its own last step and logs it. It leaves `rollbackMigration` as it is, so rolling back once more still refuses.
- An exporter whose last step is below `rollbackMigration`, two or more releases back, refuses to start and says so in its log, because the stored data is newer than its code. Run a newer release, or restore the backup taken before the upgrade ([Upgrade](INSTALL.md#upgrade)).
- Each minor version lists its schema changes and steps in the [upgrade notes](INSTALL.md#upgrade-notes).
