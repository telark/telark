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

## Access the dashboard

The dashboard (`ui` service) is optional and **off by default** — enable it with `--set services.ui.enabled=true`, then reach it either way.

**Port-forward** (no ingress needed):

```sh
kubectl port-forward -n telark svc/telark-ui-service 8080:3000
# open http://localhost:3000
```

**Ingress** (off by default; needs an ingress controller in the cluster):

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set services.ui.enabled=true \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.host=telark.example.com
```

| Flag | Default | Description |
|---|---|---|
| `ingress.enabled` | `false` | Create an Ingress for the dashboard |
| `ingress.className` | `""` | IngressClass (e.g. `nginx`) |
| `ingress.host` | `""` | Hostname (`""` = match any host) |
| `ingress.service` | `ui` | Which `services.<key>` to route to |
| `ingress.path` / `ingress.pathType` | `/` / `Prefix` | Route path + match type |
| `ingress.tls` | `[]` | TLS blocks, e.g. `[{secretName: telark-tls, hosts: [telark.example.com]}]` |
| `ingress.annotations` | `{}` | Controller annotations (cert-manager, etc.) |

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

## Autoscaling (HPA)

The stateless services — auth, discovery, enrichment, notifier, ui — can run behind a HorizontalPodAutoscaler (`autoscaling/v2`, CPU-based). The exporter never autoscales (it holds a ReadWriteOnce volume). HPAs need metrics-server, which ships with the chart.

**`performance` mode turns autoscaling on automatically** (min 3, max 5). In any mode you can enable or tune it per service:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set services.discovery.autoscaling.enabled=true \
  --set services.discovery.autoscaling.minReplicas=2 \
  --set services.discovery.autoscaling.maxReplicas=8 \
  --set services.discovery.autoscaling.targetCPUUtilizationPercentage=70
```

| Key | Default | Description |
|---|---|---|
| `services.<svc>.autoscaling.enabled` | `false` | Turn the HPA on for that service (on in `performance`) |
| `services.<svc>.autoscaling.minReplicas` | `1` | Replica floor |
| `services.<svc>.autoscaling.maxReplicas` | `3` | Replica ceiling |
| `services.<svc>.autoscaling.targetCPUUtilizationPercentage` | `80` | Scale-up CPU target |
| `services.<svc>.autoscaling.targetMemoryUtilizationPercentage` | _(unset)_ | Optional memory target |

Set the same keys under `app.serviceDefaults.autoscaling` to change the default for **every** service at once. When a service autoscales, Helm stops managing its replica count (`spec.replicas` is omitted) so the HPA and Helm don't fight.

## Monitoring (Prometheus)

telark can emit a **ServiceMonitor** (Prometheus Operator) that scrapes every service's `/metrics`. It is **off by default** — it needs the Prometheus Operator CRDs already in the cluster (e.g. from kube-prometheus-stack).

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set monitoring.serviceMonitor.enabled=true \
  --set monitoring.serviceMonitor.labels.release=kube-prometheus-stack
```

**The label is the part people miss:** Prometheus only picks up a ServiceMonitor whose labels match its `serviceMonitorSelector`. For kube-prometheus-stack that selector is `release: <your-release>`, so set `monitoring.serviceMonitor.labels.release` to your Prometheus release name. Omit it and Prometheus silently ignores the monitor.

| Key | Default | Description |
|---|---|---|
| `monitoring.serviceMonitor.enabled` | `false` | Create the ServiceMonitor |
| `monitoring.serviceMonitor.labels` | `{}` | Labels matching Prometheus's `serviceMonitorSelector` (usually `release: <name>`) |
| `monitoring.serviceMonitor.path` | `/metrics` | Scrape path |
| `monitoring.serviceMonitor.interval` | `30s` | Scrape interval |

The monitor selects every telark service (`app.kubernetes.io/part-of: telark`) on the `http` port — the services must expose `/metrics` there for scraping to return data.

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
