# Install guide

## Prerequisites

- Kubernetes ≥ 1.33 (clears ValidatingAdmissionPolicy GA; see `Chart.yaml` `kubeVersion`).
- Helm ≥ 3.
- A default StorageClass (the exporter needs a PVC for snapshots).

## 1. Install

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace
```

One command installs everything — CRDs, NATS config, and default `standard` sizing all ship in the chart. The CRDs are cluster-scoped and kept on uninstall (`resource-policy: keep`). Managing CRDs out of band (e.g. GitOps applies them first)? Add `--set crds.enabled=false`. From a checkout, `./charts/telark` works in place of the OCI ref.

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

## 2. First admin

Set `app.auth.bootstrap.admins` before install (or upgrade after). Those emails receive the Admin role on first OIDC login; with passkey self-registration off, this is the only path to a first admin.

## 3. Verify

```sh
kubectl get pods -n telark -l app.kubernetes.io/instance=telark
helm test telark -n telark      # readiness probe against the auth service
```

## Install-time flags

Everything is set on the one command line with `--set key=value`. Re-pass the same flags on `helm upgrade` — Helm does not remember them across upgrades.

### App

| Flag | Default | Description |
|---|---|---|
| `app.mode` | `standard` | Size every telark service: `minimal` \| `standard` \| `performance` (see [Sizing modes](#sizing-modes)) |
| `app.name` | `telark` | App identity / resource-name prefix (also the CRD group, `erpi.<name>`) |
| `app.namespace` | `telark` | Install namespace |
| `app.image.registry` | `telark` | Registry / org hosting the service images |
| `app.image.pullPolicy` | `Always` | Image pull policy |
| `app.image.pullSecrets` | `[]` | Image pull secrets for a private registry |
| `app.persistence.size` | `10Gi` | Exporter snapshot PVC size |
| `app.persistence.storageClass` | `""` | PVC class (`""` = cluster default; a ReadWriteMany class is required for `performance`) |
| `app.persistence.accessMode` | `ReadWriteOnce` | Exporter PVC access mode |
| `app.crdGuard.enabled` | `false` | Admission guard: only owning service accounts may write telark CRs |
| `app.crdGuard.enforce` | `false` | With the guard on, `false` audits and `true` rejects |
| `app.auth.bootstrap.admins[0]` | `contact@telark.io` | Emails granted Admin on first login (indexed: `[0]`, `[1]`, …) |
| `app.auth.passkey.selfRegistration` | `"true"` | `"false"` blocks new passkey registration (needs a bootstrap admin) |

### Subcharts

Bundled dependencies ship production-grade defaults sized for every mode, so you rarely touch these. On/off toggles and the values telark pins — any other upstream key works the same way (`--set <subchart>.<path>`):

| Flag | Default | Description |
|---|---|---|
| `crds.enabled` | `true` | Install CRDs (the telark-crds subchart); `false` to manage them out of band |
| `app.kyverno.enabled` | `true` | Install the policy engine (kyverno) |
| `app.ollama.enabled` | `false` | Install the local LLM (ollama) for on-cluster enrichment |
| `metrics-server.enabled` | `true` | Install metrics-server; `false` if the cluster already ships one |
| `redis.architecture` | `standalone` | `replication` for a replicated redis |
| `redis.master.persistence.size` | `4Gi` | Redis PVC size |
| `nats.persistence.size` | `4Gi` | NATS JetStream PVC size |
| `kyverno.admissionController.replicas` | `2` | Policy-engine admission replicas |
| `kyverno.admissionController.container.extraArgs.clientRateLimitQPS` | `50` | Policy-engine API QPS |
| `metrics-server.resources.limits.memory` | `400Mi` | metrics-server memory limit |
| `ollama.persistentVolume.size` | `10Gi` | ollama model storage (when enabled) |

The **complete** field list — every telark value and every pinned subchart value — is the auto-generated [`charts/telark/VALUES.md`](../charts/telark/VALUES.md); full upstream options live in each dependency's own chart (redis/nats = Bitnami, plus kyverno, metrics-server, ollama).

Example:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance \
  --set app.persistence.storageClass=efs \
  --set app.auth.bootstrap.admins[0]=you@corp.com
```

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

CRDs and existing custom resources are **not** removed (they carry `helm.sh/resource-policy: keep`). For a full teardown, delete the CRDs explicitly — this also deletes every telark custom resource:

```sh
kubectl delete crd -l app.kubernetes.io/part-of=telark
```
