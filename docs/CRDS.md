# CRD reference

telark's custom resources are defined by the `telark-crds` chart. The group suffix is the app identity `app.name` (default `telark`); below it is shown as `<name>`. All are kept on uninstall (`helm.sh/resource-policy: keep`); to remove them, follow the [full teardown](INSTALL.md#full-teardown).

## Group `erpi.<name>`

| Kind | Plural | Purpose |
|---|---|---|
| `ApplicationAsResource` | `applicationsasresources` | A discovered application — a group of workloads treated as one unit. Scope target for protection plans. |
| `GroupAsResource` | `groupsasresources` | An access group in the role model. |
| `RoleAsResource` | `rolesasresources` | A role: scopes granted and denials, at a level. |
| `UserAsResource` | `usersasresources` | A user in the role model. |
| `GlobalConfig` | `globalconfigs` | Cluster-wide settings (AI provider/keys, OIDC) set from the UI at runtime. |
| `ProtectionPlan` | `protectionplans` | Policy templates bound to a scope and a time window; transitions scheduled → active → terminated. |

## Group `auth.<name>`

| Kind | Plural | Purpose |
|---|---|---|
| `UserPasskey` | `userpasskeys` | A registered WebAuthn passkey credential. |
| `UserSession` | `usersessions` | An active authenticated session. Named `session-<sha256(token)>`; the token itself is never stored. |

## Group `classification.<name>`

| Kind | Plural | Purpose |
|---|---|---|
| `CategoryAsClassification` | `categoriesasclassifications` (short: `cat`) | A classification category applied to applications. |

## Ownership

The `exporter` service owns every resource in these groups; `discovery` additionally patches its own `applicationsasresources`. The optional CRD write guard (`app.crdGuard`) restricts direct writes to these groups to the owning service accounts — see [INSTALL.md](INSTALL.md).
