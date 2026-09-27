# telark-crds

Custom resource definitions for [Telark](https://telark.io), a protection gate for Kubernetes applications. The `telark` chart already bundles this chart as a subchart, so a normal `helm install telark` includes the CRDs. Install it on its own only when you manage CRDs out of band, for example with GitOps, and set `crds.enabled=false` on the `telark` chart.

The CRDs carry `helm.sh/resource-policy: keep`, so they survive an uninstall. Removing them is part of the [full teardown](https://github.com/telark/telark/blob/main/docs/INSTALL.md#full-teardown). Field-level reference: [CRD reference](https://github.com/telark/telark/blob/main/docs/CRDS.md).

## Standalone install

From the registry, with the chart defaults (`app.name: telark`):

```sh
helm install telark-crds oci://ghcr.io/telark/charts/telark-crds
```

This pulls the latest published version. To match a customized app release, pass `--set app.name=<name> --set app.namespace=<ns>` (the API group stays `telark.io`). From a checkout, `./charts/telark-crds` works in place of the OCI ref.

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
