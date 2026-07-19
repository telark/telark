# Install guide

## Prerequisites

- Kubernetes ≥ 1.33 (clears ValidatingAdmissionPolicy GA; see `Chart.yaml` `kubeVersion`).
- Helm ≥ 3.
- A default StorageClass (the exporter needs a PVC for snapshots).

## 1. Install the CRDs

CRDs are a separate chart, published to the registry, and must be installed first. They are cluster-scoped and kept on uninstall.

```sh
helm install telark-crds oci://ghcr.io/telark/charts/telark-crds
```

Pulls the latest published CRD chart. Uses the chart defaults (`app.name: telark`); add `--set app.name=<name> --set app.namespace=<ns>` if you customize the app identity. From a checkout, `./charts/telark-crds` works in place of the OCI ref.

## 2. Install the app

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace
```

One command — everything needed to run (NATS config, default `standard` sizing) ships in the chart. From a checkout, `./charts/telark` works in place of the OCI ref.

### Sizing modes

`app.mode` sizes every telark service — replicas, resources, client rate limits and disruption budgets — from a single flag. The default is `standard`; pick another with `--set`:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance
```

| Mode | For |
|---|---|
| `minimal` | dev, demos, evaluation — single replica, no PDBs |
| `standard` (default) | small–mid production — 2 replicas, disruption budgets |
| `performance` | large clusters — 3 replicas; needs a ReadWriteMany class for the exporter (add `--set app.persistence.storageClass=<rwx-class>`) |

`app.mode` sizes telark's own services only — Helm resolves a subchart's values before the mode is known, so redis, NATS, the policy engine and metrics-server ship fixed production-grade defaults owned by the chart, identical in every mode. Nothing to tune.

## 3. First admin

Set `app.auth.bootstrap.admins` before install (or upgrade after). Those emails receive the Admin role on first OIDC login; with passkey self-registration off, this is the only path to a first admin.

## 4. Verify

```sh
kubectl get pods -n telark -l app.kubernetes.io/instance=telark
helm test telark -n telark      # readiness probe against the auth service
```

## Configuration

Every value is documented in [`charts/telark/README.md`](../charts/telark/README.md). Common ones:

- `app.image.*` — registry/repository/pullPolicy/pullSecrets.
- `app.persistence.*` — exporter snapshot storage (size, class, access mode).
- `app.crdGuard.*` — restrict who may write telark CRs directly (audit → enforce).
- `app.podSecurityContext` / `app.containerSecurityContext` — pod hardening.

## Upgrade

```sh
helm upgrade telark oci://ghcr.io/telark/charts/telark -n telark \
  --set app.mode=<mode>
```

Re-pass the same `--set` / `-f` flags used at install: Helm does not remember them across upgrades.

## Uninstall

```sh
helm uninstall telark -n telark
```

CRDs and existing custom resources are **not** removed (they carry `helm.sh/resource-policy: keep`). Delete `telark-crds` and the CRDs explicitly if you want a full teardown.
