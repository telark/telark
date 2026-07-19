# telark-crds

Custom resource definitions for [telark](https://telark.io). These ship **with** the `telark` chart as a subchart, so a normal `helm install telark` already includes them. Install this chart on its own only when managing CRDs out of band (e.g. GitOps, with `--set crds.enabled=false` on the app).

CRDs are cluster-scoped and carry `helm.sh/resource-policy: keep`, so they survive an uninstall of this release. The `telark` chart reconciles the resources these definitions describe.

## Standalone install

Only needed when the app is installed with `crds.enabled=false`. From the registry (uses the chart defaults, `app.name: telark`):

```sh
helm install telark-crds oci://ghcr.io/telark/charts/telark-crds
```

Pulls the latest published version. Override to match a customized app release: `--set app.name=<name> --set app.namespace=<ns>`. From a checkout, `./charts/telark-crds` works in place of the OCI ref.

## Values

This chart holds no config of its own. It reads two keys from the telark chart's values so CRD groups and namespaces match the app release (full index: **[VALUES.md](VALUES.md)**):

| Key | Description |
|---|---|
| `app.name` | App identity; forms the API-group suffix (e.g. `erpi.<name>`) |
| `app.namespace` | Namespace the built-in custom resources target |

## Definitions

| Group | Kinds |
|---|---|
| `erpi.<name>` | ApplicationAsResource, GroupAsResource, RoleAsResource, UserAsResource, GlobalConfig, ProtectionPlan |
| `auth.<name>` | UserPasskey, UserSession |
| `classification.<name>` | CategoryAsClassification |
