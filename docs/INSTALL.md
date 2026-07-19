# Install guide

## Prerequisites

- Kubernetes ≥ 1.33 (clears ValidatingAdmissionPolicy GA; see `Chart.yaml` `kubeVersion`).
- Helm ≥ 3.
- A default StorageClass (the exporter needs a PVC for snapshots).

## 1. Install the CRDs

CRDs are a separate chart and must be installed first. They are cluster-scoped and kept on uninstall.

```sh
helm install telark-crds ./charts/telark-crds -f charts/telark/values.yaml
```

## 2. Install the app

```sh
helm install telark ./charts/telark \
  --set-file nats.configuration=charts/telark/config/nats.conf
```

`--set-file` is required: the NATS config is loaded from `charts/telark/config/nats.conf` and cannot be a values default.

### Sizing modes (optional)

Layer an overlay for cluster size. Without one, the chart defaults suit a single-node / evaluation cluster.

```sh
helm install telark ./charts/telark \
  -f charts/telark/values.mode.<mode>.yaml \
  --set-file nats.configuration=charts/telark/config/nats.conf
```

| Mode | For |
|---|---|
| `minimal` | dev, demos, evaluation — single replica, no PDBs |
| `standard` | small–mid production — 2 replicas, disruption budgets |
| `performance` | large clusters — 3 replicas; needs a ReadWriteMany storage class for the exporter |

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
helm upgrade telark ./charts/telark \
  -f charts/telark/values.mode.<mode>.yaml \
  --set-file nats.configuration=charts/telark/config/nats.conf
```

## Uninstall

```sh
helm uninstall telark -n telark
```

CRDs and existing custom resources are **not** removed (they carry `helm.sh/resource-policy: keep`). Delete `telark-crds` and the CRDs explicitly if you want a full teardown.
