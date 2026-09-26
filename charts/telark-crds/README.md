# telark-crds

Custom resource definitions for [telark](https://telark.io). These ship **with** the `telark` chart as a subchart, so a normal `helm install telark` already includes them. Install this chart on its own only when managing CRDs out of band (e.g. GitOps, with `--set crds.enabled=false` on the app).

CRDs are cluster-scoped and carry `helm.sh/resource-policy: keep`, so they survive an uninstall of this release — removing them is the [full teardown](https://github.com/telark/telark/blob/main/docs/INSTALL.md#full-teardown). The `telark` chart reconciles the resources these definitions describe.

## Standalone install

Only needed when the app is installed with `crds.enabled=false`. From the registry (uses the chart defaults, `app.name: telark`):

```sh
helm install telark-crds oci://ghcr.io/telark/charts/telark-crds
```

Pulls the latest published version. Override to match a customized app release: `--set app.name=<name> --set app.namespace=<ns>` (the API group stays `telark.io`). From a checkout, `./charts/telark-crds` works in place of the OCI ref.

## Values

This chart holds no config of its own. It reads two keys from the telark chart's values so labels and namespaces match the app release (full index: **[VALUES.md](VALUES.md)**):

| Key | Description |
|---|---|
| `app.name` | App identity for labels; the API group is always `telark.io` |
| `app.namespace` | Namespace the built-in custom resources target |

## Definitions

Every CRD is in group `telark.io`, version `v1alpha1`, namespaced. `kubectl get telark` lists the `telark` category; use the full names (`applications.telark.io`) where plurals collide with other CRDs.

| Kind | Plural | Short name | Category | `/status` |
|---|---|---|---|---|
| Application | `applications` | `tapp` | `telark` | yes |
| ProtectionPlan | `protectionplans` | `tplan` | `telark` | yes |
| TelarkConfig (singleton `default`) | `telarkconfigs` | `tcfg` | `telark` | yes |
| Category (singleton `categories`) | `categories` | `tcat` | `telark` | no |
| User | `users` | `tuser` | `telark` | no |
| Group | `groups` | `tgroup` | `telark` | no |
| AccessRole | `accessroles` | `trole` | `telark` | no |
| Passkey | `passkeys` | `tpk` | `telark-auth` | no |
| Session | `sessions` | `tsess` | `telark-auth` | no |

ProtectionPlan phases (`status.phase`): `draft`, `pending_approval`, `scheduled`, `active`, `terminated`, `canceled`, `failed`. Optional metadata `environmentRef` and `tagRefs` (category ids, at most 20 tags) classify a plan. Optional spec field `approvalMode` (`automatic` | `required`, absent = automatic) and `status.approval` (state, requester, decider, comment, history of at most 20 events) drive plan approval. Optional `scope.exclusions` {kinds[], resources[]{kind,name,namespace}} excludes kinds (any scope) or named resources (applications scope only) from enforcement.
