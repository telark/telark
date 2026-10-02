# telark

Helm chart for [Telark](https://telark.io), a protection gate for Kubernetes applications. It installs Telark's services (exporter, discovery, auth, notifier, analyzer and the dashboard) with Kyverno, Redis, NATS, metrics-server and Ollama.

## Install

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com
```

Set your own admin email; the chart ships none and refuses to render without one. The exporter's volumes come from the cluster's default StorageClass ([Exporter storage](../../docs/INSTALL.md#exporter-storage)). Prerequisites, first sign-in, exposure, sizing modes, upgrades and uninstall are in the [install guide](../../docs/INSTALL.md); a guided first run is in [Getting started](../../docs/getting-started.md).

Measured capacity (2026-09-18): `minimal` handles a few hundred applications; `standard` was verified at 2 000 applications (three discovery replicas; rediscovery of a deleted 100-app namespace took about 3.5 minutes); `performance` is for larger clusters.

### Upgrade order

When CRDs are managed out of band (`crds.enabled=false`), upgrade `telark-crds` before `telark`. An older CRD rejects the protection plan `pending_approval` phase and approval fields, and silently prunes `scope.exclusions`, `environmentRef` and `tagRefs`. Older dashboard bundles show pending plans with a raw label and no Cancel (Owners can Clear).

## Values reference

The tables below explain the values that matter. The generated index of every key, type and default is [VALUES.md](VALUES.md) (`make values-docs`, drift-checked in CI).

### Top-level

| Key | Default | Description |
|---|---|---|
| `nameOverride` | `""` | Override the base name (defaults to `app.name`) |
| `fullnameOverride` | `""` | Override the resource-name prefix |
| `commonLabels` | `{}` | Labels added to every resource |
| `commonAnnotations` | `{}` | Annotations added to every resource |
| `global.imagePullSecrets` | `[]` | Pull secrets merged into every pod |
| `crds.enabled` | `true` | Install CRDs (the telark-crds subchart) with the app; `false` to manage them out of band |
| `monitoring.serviceMonitor.enabled` | `false` | Emit a Prometheus-Operator ServiceMonitor for the services' `/metrics`. No service serves `/metrics` yet; keep it off. See [docs/INSTALL.md](../../docs/INSTALL.md#monitoring-prometheus) |
| `monitoring.serviceMonitor.labels` | `{}` | Labels matching Prometheus's `serviceMonitorSelector` (usually `release: <name>`) |
| `monitoring.serviceMonitor.path` / `interval` | `/metrics` / `30s` | Scrape path / interval |
| `ingress.enabled` | `false` | Ingress for the dashboard (routes to `ingress.service`, default `ui`). See [docs/INSTALL.md](../../docs/INSTALL.md#access-the-dashboard) |
| `ingress.className` / `host` / `path` / `pathType` / `tls` / `annotations` | see values | Ingress routing + TLS |
| `gateway.enabled` | `false` | Gateway API `HTTPRoute` for the dashboard, the alternative to the Ingress (routes to `gateway.service`, default `ui`). See [docs/INSTALL.md](../../docs/INSTALL.md#access-the-dashboard) |
| `gateway.parentRefs` / `hostnames` / `annotations` | `[]` / `[]` / `{}` | Gateways to attach to (entries take `name`, `namespace`, `sectionName`), hostnames the route matches, HTTPRoute annotations |
| `redis.image.digest` | `sha256:33a5a129…` | Pins the Redis image (`bitnami/redis`, which publishes only `latest`) to one build. See [docs/INSTALL.md](../../docs/INSTALL.md#subcharts) |
| `redis.master.resources` | requests `100m` / `128Mi`, limits `150m` / `512Mi` | Redis sizing, identical in every mode; replaces the subchart's `nano` preset. Redis never evicts, so raise the memory limit beyond 2 000 applications. See [docs/INSTALL.md](../../docs/INSTALL.md#subcharts) |

### `app`

| Key | Default | Description |
|---|---|---|
| `app.name` | `telark` | Source of truth for the app identity / resource-name prefix |
| `app.namespace` | `telark` | Install namespace; bootstrap CRs land here. Must match the release namespace (`-n`): the subcharts follow `-n`, so a mismatch splits redis/nats away from the services |
| `app.mode` | `standard` | Sizes every Telark service (replicas, resources, rate limits, PDBs). `minimal` \| `standard` \| `performance`. Subcharts keep production-grade defaults across all modes. |
| `app.image.registry` | `ghcr.io/telark` | Registry and namespace hosting the per-service repos |
| `app.image.pullPolicy` | `Always` | Image pull policy for every service container |
| `app.image.pullSecrets` | `[]` | Pull secrets (public images need none; set for a private registry) |
| `app.kubectlImage` | `registry.k8s.io/kubectl:v1.37.1@sha256:…` | kubectl image of the uninstall hooks that stop auth and Kyverno and clear their finalizers and webhooks ([Uninstall](../../docs/INSTALL.md#uninstall)); mirror it for air-gapped installs |
| `app.kyverno.enabled` | `true` | Install kyverno subchart |
| `app.kyverno.failOpen` | `true` | Kyverno webhooks fail open (`failurePolicy: Ignore`), so enforce plans are best-effort while Kyverno is down. Must equal `kyverno.features.forceFailurePolicyIgnore.enabled`; the render fails otherwise. See [Policy engine fail-open](../../docs/INSTALL.md#policy-engine-fail-open) |
| `app.crdGuard.enabled` / `enforce` | `true` / `true` | ValidatingAdmissionPolicy: only the owning service accounts may write Telark CRs (`telark.io`, including `/status`), and only the exporter may change the key in the OIDC trust Secret (`enforce: false` audits). See [CRD write guard](../../docs/INSTALL.md#crd-write-guard) |
| `app.crdGuard.extraAllowedUsers` | `[]` | Break-glass usernames also allowed to write Telark CRs and the OIDC trust Secret |
| `app.networkPolicy.enabled` | `true` | Ingress NetworkPolicies: default deny for Telark pods, APIs only from Telark pods, dashboard from anywhere, NATS 4222 only from discovery and notifier. Needs an enforcing CNI. See [Network policies](../../docs/INSTALL.md#network-policies) |
| `app.selfMonitoring.enabled` | `false` | `false`: discovery skips Telark's own namespace (its services, Redis, NATS, Ollama, Kyverno, metrics-server), so none of it shows under Applications, and Application CRs already created there are cleaned up. `true`: they are discovered and shown like any application. Plans can never target that namespace either way. → `SELF_MONITORING_ENABLED` on discovery. See [Self-monitoring](../../docs/INSTALL.md#self-monitoring) |
| `app.serviceToken.value` | `""` | Service token; empty = generated on install, read back on upgrade |
| `app.serviceToken.existingSecret` | `""` | Secret (key `token`) you manage instead of the generated one, for cluster-less renders. See [GitOps](../../docs/INSTALL.md#gitops-cluster-less-renders) |
| `nats.existingSecrets.publisher` / `consumer` | `""` | Secrets (keys `username`, `password`) for the NATS publisher (discovery) and consumer (notifier) users instead of the generated `<app.name>-nats-{publisher,consumer}-secret`. See [GitOps](../../docs/INSTALL.md#gitops-cluster-less-renders) |
| `redis.auth.existingSecret` | `<release>-redis-secret` | Secret with the Redis password (key `redis-password`), generated on install and read back on upgrade; name a Secret you manage for cluster-less renders. Redis always requires the password: `redis.auth.enabled=false` fails the render. See [GitOps](../../docs/INSTALL.md#gitops-cluster-less-renders) |
| `metrics-server.args` | `[]` | Flags added after the subchart's `defaultArgs` (which already set `--kubelet-preferred-address-types`). Kubelet certificates are verified; add `--kubelet-insecure-tls` only where they are self-signed. See [metrics-server kubelet TLS](../../docs/INSTALL.md#metrics-server-kubelet-tls) |
| `app.ollama.enabled` | `true` | Install the ollama subchart, the local model runtime the analyzer needs; `false` skips it (for example with `app.ollama.runtimeUrl`). Sized once for every mode. See [Analyzer runtime (ollama)](#analyzer-runtime-ollama) |
| `app.ollama.autoPull` | `true` | Let the analyzer pull a missing model (and allow ollama HTTPS egress); `false` for air-gapped installs |
| `app.ollama.runtimeUrl` | `""` | Ollama-API endpoint you run yourself (URL only, no key); empty = the subchart. See [Analyzer runtime (ollama)](#analyzer-runtime-ollama) |
| `app.persistence.enabled` | `true` | Provision the exporter snapshot and report PVCs (kept on uninstall) |
| `app.persistence.storageClass` | `""` | `"<name>"` = explicit class; `"-"` = disable dynamic provisioning; `""` = cluster default on install, the claims' current class on upgrade. A different class moves the data to new claims; more than one exporter replica needs a ReadWriteMany class. See [Exporter storage](../../docs/INSTALL.md#exporter-storage) |
| `app.persistence.snapshotsSize` | `10Gi` | Exporter snapshots PVC size (`minimal` 1Gi, `performance` 50Gi); only grows, where the class allows expansion |
| `app.persistence.reportsSize` | `2Gi` | Exporter reports PVC size (`minimal` 512Mi, `performance` 10Gi); only grows |

#### Protection plan reports

The exporter mounts two PVCs rendered from one template (snapshots and reports); both follow `app.persistence.storageClass` and the access mode its replica count needs ([Exporter storage](../../docs/INSTALL.md#exporter-storage)). A final report is captured asynchronously right after a plan ends (after its policies are removed and its phase is recorded). Periodic checkpoints keep records past the 1 h Event retention; an on-demand report merges what the cluster still holds. Reports are deleted with the plan and swept if the plan CR disappears; at most 10 on-demand reports are kept per plan. Formats: HTML, Markdown, JSON, CSV (PDF via the browser's print). The document shows user ids, not names. When two on-demand renders are already running the generate call returns HTTP 429 with `Retry-After`.

#### `app.auth.bootstrap`

| Key | Default | Description |
|---|---|---|
| `app.auth.bootstrap.admin` | `""` | The one bootstrap admin's email. The account is created and recovered only with `./main break-glass --email <email> --enroll` (passkey) and holds the built-in Admin role; Google sign-in and passkey self-registration never grant Admin, even to this email. → `BOOTSTRAP_ADMIN` env. Required: the render fails without it, since only this account can turn on SSO or self-registration. See [First admin](../../docs/INSTALL.md#2-first-admin) |

#### `app.auth.oidc`

The Google client id, the OIDC flag and the egress switch are runtime settings on the TelarkConfig CR (Settings in the dashboard). The pinned signing keys used when egress is not allowed live in a Secret (`<fullname>-oidc-trust-secret`, key `googleJwkJson`): the exporter writes it when an admin saves the keys, auth reads it from a read-only volume and picks up a change within about a minute. The chart renders it empty and reads it back on upgrade; only the exporter (and `app.crdGuard.extraAllowedUsers`) may change the key while `app.crdGuard.enabled` is on.

| Key | Default | Description |
|---|---|---|
| `app.auth.oidc.existingSecret` | `""` | Secret (key `googleJwkJson`) you manage instead of the chart-rendered one, for cluster-less renders; the exporter needs it to exist and the guard lets others create it only with an empty key. See [GitOps](../../docs/INSTALL.md#gitops-cluster-less-renders) |

#### `app.auth.passkey`

WebAuthn relying-party identity. Passkey self-registration is not a chart value: it is a Settings toggle (TelarkConfig `selfRegistration.enabled`), off by default and changed only by the bootstrap admin ([First admin](../../docs/INSTALL.md#2-first-admin)).

| Key | Default | Description |
|---|---|---|
| `app.auth.passkey.id` | `""` | Relying Party identifier. Empty follows the request host (`X-Forwarded-Host`, else `Host`, port stripped), so passkeys work on whichever hostname you open the dashboard on. Required (the render fails) with `ingress.enabled` or `gateway.enabled`; pin it in production. Injected as `RP_ID`. |
| `app.auth.passkey.name` | `"Dashboard App"` | Display name shown by the authenticator (Touch ID prompt, etc.). Injected as `RP_NAME`. |
| `app.auth.passkey.origin` | `""` | Origin(s) accepted for WebAuthn ceremonies, comma-separated. Empty follows the request `Origin` header, whose host must be the relying party or one of its subdomains. Required (the render fails) with `ingress.enabled` or `gateway.enabled`; pin to `https://<domain>` in production. Injected as `RP_ORIGIN`. |

### `app.serviceDefaults`

Fallbacks for any `services.<svc>.*` key omitted.

| Key | Default | Description |
|---|---|---|
| `app.serviceDefaults.port` | `8080` | Container + Service port |
| `app.serviceDefaults.serviceType` | `ClusterIP` | K8s Service type |
| `app.serviceDefaults.replicas` | `1` | Starting replicas; the HPA scales from here in `standard`/`performance` (`minimal` stays fixed at 1) |
| `app.serviceDefaults.terminationGracePeriodSec` | `60` | Pod termination grace period |
| `app.serviceDefaults.autoscaling.enabled` | `true` | Fleet-wide HPA (1 → 3 replicas on CPU; `performance` 1 → 5; `minimal` off; never the exporter). Vertical scaling: `vpa.enabled=true` installs the operator and `app.serviceDefaults.vpa.updateMode` (`Auto`) drives a VPA per service. See [docs/INSTALL.md](../../docs/INSTALL.md#autoscaling-hpa) |
| `app.serviceDefaults.autoscaling.minReplicas` / `maxReplicas` | `1` / `3` | HPA replica floor / ceiling |
| `app.serviceDefaults.autoscaling.targetCPUUtilizationPercentage` | `80` | HPA scale-up CPU target |

### `app.shared`

Pod-level config selectively applied via per-service gates.

**`app.shared.redis`**: mounted as `envFrom: configMapRef: <app.name>-redis-cm` when `useRedis: true` (default), together with `REDIS_PASSWORD` from the Secret named by `redis.auth.existingSecret`.

| Variable | Default | Description |
|---|---|---|
| `REDIS_HOST` | `<release>-redis-master` | Redis service DNS name (templated on the release name) |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | from `<release>-redis-secret` | `secretKeyRef`, never in the ConfigMap |

**`app.shared.resources`**: applied when `includeResources: true` (default).

| Key | Default | Description |
|---|---|---|
| `requests.cpu` / `requests.memory` | `100m` / `128Mi` | Resource requests |
| `limits.cpu` / `limits.memory` | `500m` / `512Mi` | Resource limits |

**`app.shared.healthCheck`**: HTTP probes applied when `includeHealthCheck: true` (default). `port` is the named container port.

| Key | Default | Description |
|---|---|---|
| `port` | `http` | Named port for probe targets |
| `livenessProbe.path` | `/api/v1/status/live` | Liveness HTTP path |
| `readinessProbe.path` | `/api/v1/status/ready` | Readiness HTTP path |
| `*.initialDelaySeconds` / `periodSeconds` / `timeoutSeconds` | liveness `15`; readiness `5` / `5` / `15` | Probe timings; readiness only checks the pod's own Redis link, so it turns Ready seconds after start and rollouts do not wait on other services |
| `*.failureThreshold` | `3` | Consecutive failures before unhealthy |

**`app.shared.podSecurityContext`** / **`app.shared.containerSecurityContext`**: pod- and container-level securityContext, applied when a service has `includeSecurity: true` (default). Set `enabled: false` on either to omit it.

| Key | Default | Description |
|---|---|---|
| `app.shared.podSecurityContext.enabled` | `true` | Render the pod securityContext |
| `app.shared.podSecurityContext.runAsUser` / `runAsGroup` / `fsGroup` | `1001` | Non-root identity |
| `app.shared.podSecurityContext.seccompProfile` / `app.shared.containerSecurityContext.seccompProfile` | `{type: RuntimeDefault}` | Seccomp profile, so the namespace can carry Pod Security `restricted` |
| `app.shared.containerSecurityContext.enabled` | `true` | Render the container securityContext |
| `app.shared.containerSecurityContext.allowPrivilegeEscalation` | `false` | Block setuid-style escalation |
| `app.shared.containerSecurityContext.readOnlyRootFilesystem` | `true` | Mount root FS read-only |
| `app.shared.containerSecurityContext.capabilities.drop` | `["ALL"]` | Linux capabilities to drop |

**`app.serviceDefaults`** scheduling: `nodeSelector` (`{}`), `tolerations` (`[]`), `affinity` (`{}`); overridable per service.

**`app.shared.nats`** + NATS credentials: mounted when `useNatsCreds: true` (default false). The Secret follows the service's `natsUser`: `<app.name>-nats-publisher-secret` (discovery, may only publish `telark.applications.*`) or `<app.name>-nats-consumer-secret` (notifier, JetStream API and acks), or `nats.existingSecrets.<user>`. The NATS server serves no monitoring port; its probes use the client port.

| Variable | Secret key / default | Description |
|---|---|---|
| `NATS_HOST` | `<release>-nats` | NATS service DNS name (templated on the release name) |
| `NATS_USER` | `username` | NATS auth user |
| `NATS_PASSWORD` | `password` | NATS auth password |

### `services.<svc>`

Per-service block. Gates default to `true` unless noted.

| Key | Default | Description |
|---|---|---|
| `enabled` | varies | Render this service's manifests |
| `name` | varies | K8s resource suffix |
| `repository` | varies | Image repo name; image = `<app.image.registry>/<repository>:<version>` |
| `version` | varies | Image tag (semver) |
| `category` | varies | Semantic grouping, rendered as the label `<name>.io/category:<category>` |
| `replicas` | `app.serviceDefaults.replicas` | Override |
| `port` | `app.serviceDefaults.port` | Override |
| `serviceType` | `app.serviceDefaults.serviceType` | Override |
| `nodePort` | unset | Fixed port when `serviceType: NodePort` (used on `ui`, the dashboard; unset = allocated by Kubernetes). See [docs/INSTALL.md](../../docs/INSTALL.md#access-the-dashboard) |
| `terminationGracePeriodSec` | `app.serviceDefaults.terminationGracePeriodSec` | Override |
| `includeResources` | `true` | Apply `app.shared.resources` |
| `includeHealthCheck` | `true` | Apply `app.shared.healthCheck` |
| `includeSecurity` | `true` | Apply `app.shared.podSecurityContext` + `app.shared.containerSecurityContext` (`ui` sets `false`: nginx runs as uid 101 and gets its own contexts below) |
| `podSecurityContext` / `containerSecurityContext` | unset | Rendered verbatim instead of the shared contexts; `ui` uses them (uid 101, no privilege escalation, all capabilities dropped, read-only root with `emptyDir` on `/var/cache/nginx` and `/tmp`) |
| `automountServiceAccountToken` | unset (Kubernetes default: mounted) | `false` on auth, notifier and ui, which never call the Kubernetes API; set on both the pod and its ServiceAccount |
| `serviceToken` | `true` | Inject the shared service token as `TELARK_SERVICE_TOKEN`; `false` on ui, whose nginx only proxies browser calls and never calls a service itself |
| `serviceAccount.create` / `serviceAccount.name` / `serviceAccount.annotations` | `create: true` | Per-service ServiceAccount control |
| `nodeSelector` / `tolerations` / `affinity` | `app.serviceDefaults.*` | Scheduling overrides |
| `useRedis` | `true` | Mount `app.shared.redis` and `REDIS_PASSWORD` |
| `useNatsCreds` | `false` | Mount `app.shared.nats` and the NATS credentials of `natsUser` |
| `natsUser` | unset | `publisher` (discovery) or `consumer` (notifier); required with `useNatsCreds` |
| `env` | `{}` | Inline env map; values pass through `tpl` against `.Values` |
| `envFromConfigMap` | `{}` | `valueFrom: configMapKeyRef` map (external configmap) |
| `envFromSecret` | `{}` | `valueFrom: secretKeyRef` map (secret `<app.name>-<name>-secret`) |
| `volumes` / `volumeMounts` | `[]` | Pod volumes (`configMap`, `secret`, `persistentVolumeClaim`, `emptyDir`) + mounts |
| `pdb.enabled` | varies | PodDisruptionBudget |
| `autoscaling.enabled` | mode | Per-service HPA (auth/discovery/notifier/ui; never exporter or analyzer). Inherits `app.serviceDefaults.autoscaling.*`; on in `standard` and `performance`, off in `minimal` |
| `topologySpread.*` | unset | TopologySpreadConstraints |

#### Service identities

Image tags are `services.<svc>.version` in `values.yaml`, bumped by the release workflows.

| Service | `name` | `repository` | `category` | `enabled` |
|---|---|---|---|---|
| `exporter` | `exporter-service` | `exporter` | `persistence-manager` | `true` |
| `discovery` | `discovery-service` | `discovery` | `discovery` | `true` |
| `analyzer` | `analyzer-service` | `analyzer` | `ai-insights` | `true` |
| `notifier` | `notifier-service` | `notifier` | `event-streaming` | `true` |
| `auth` | `auth-service` | `auth` | `auth` | `true` |
| `ui` | `ui-service` | `ui` | `ui` | `true` |

`analyzer` (Python) runs one replica with `autoscaling.enabled: false`: one worker bound to one runtime slot.

#### `services.exporter.env`

| Variable | Default | Description |
|---|---|---|
| `CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins answered with CORS headers; empty sends none (the dashboard proxies every API on its own origin). See [CORS](../../docs/INSTALL.md#cors) |
| `SNAPSHOTS_PATH` | `/snapshots` | Filesystem mount path for snapshot files |
| `SNAPSHOTS_PVC_NAME` | `{{ .Values.app.name }}-exporter-snapshots-pvc` (tpl), plus a storage suffix after a class change | PVC backing snapshot storage |
| `SNAPSHOTS_PVC_NAMESPACE` | `{{ .Values.app.namespace }}` (tpl) | Namespace of the snapshots PVC |
| `EXPORTER_K8S_CLIENT_QPS` | `50` | K8s client QPS; sized for CRD-write fanout (10× client-go default) |
| `EXPORTER_K8S_CLIENT_BURST` | `100` | K8s client burst |
| `EXPORTER_LIST_RENDER_CONCURRENCY` | `2` | List renders running at once per list route. Concurrent requests for the same list share one render; a request waits for a slot up to its 20 s list deadline, then gets 503 with `Retry-After` |
| `SNAPSHOT_GC_INTERVAL_SEC` | `3600` | Sweep the snapshot PVC for files no Application CR references and older than 1 h (`minimal` 7200, `performance` 900; `0` = off). One replica sweeps per interval. Also drives the reports orphan sweep; `0` disables both |
| `REPORTS_PATH` | `/reports` | Filesystem mount path for protection plan report files |
| `BOOTSTRAP_ADMIN` | `{{ .Values.app.auth.bootstrap.admin }}` (tpl) | Same email as auth; a session may not create or edit a user with it (403), see [First admin](../../docs/INSTALL.md#2-first-admin) |
| `OIDC_TRUST_SECRET_NAME` | `{{ include "telark.oidcTrustSecretName" . }}` (tpl) | Secret the exporter writes the pinned OIDC keys to; follows `app.auth.oidc.existingSecret` |

`services.exporter.envFromConfigMap.CA_BUNDLE` → configmap `telark-ca-bundle`, key `ca.crt` (trusted CA bundle).

#### `services.discovery.env`

K8s client + informers:

| Variable | Default | Description |
|---|---|---|
| `CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins answered with CORS headers; empty sends none (the dashboard proxies every API on its own origin). See [CORS](../../docs/INSTALL.md#cors) |
| `DISCOVERY_K8S_CLIENT_QPS` | `100` | K8s client QPS; above 50/100 default to absorb snapshot LIST fanout on busy clusters |
| `DISCOVERY_K8S_CLIENT_BURST` | `200` | K8s client burst |
| `DISCOVERY_ROLLBACK_K8S_CLIENT_QPS` | `100` | Rollback controller's own K8s client QPS (its own bucket, sized like the shared one), so informer/prewarm traffic never queues a rollback's apply calls |
| `DISCOVERY_ROLLBACK_K8S_CLIENT_BURST` | `200` | Rollback controller's own K8s client burst |
| `DISCOVERY_INFORMER_RESYNC_SEC` | `600` | Base informer resync interval |
| `DISCOVERY_ROLLBACK_INFORMER_RESYNC_SEC` | `600` | Rollback informer resync interval |
| `DISCOVERY_INFORMER_RESYNC_JITTER_FRACTION` | `0.2` | ±jitter applied per replica to spread resyncs and avoid LIST stampedes |
| `DISCOVERY_INFORMER_COALESCING_WINDOW_SEC` | `5` | Event coalescing window |
| `DISCOVERY_INFORMER_COALESCING_MAX_WAIT_SEC` | `10` | Max wait before a forced flush |
| `DISCOVERY_COALESCE_BUFFER_MAX_ENTRIES` | `500` | Coalescing buffer cap |
| `DISCOVERY_SNAPSHOT_FETCH_TIMEOUT_MS` | `5000` | Per-resource K8s GET deadline during snapshot assembly. Tune up on clusters with high apiserver tail latency. |

Coordination (leader election + claim queue):

| Variable | Default | Description |
|---|---|---|
| `COORDINATION_BATCH_SIZE` | `10` | Items processed per coordination batch |
| `COORDINATION_BATCH_BLOCK_SEC` | `2` | Max time to wait while filling a batch |
| `COORDINATION_MAX_RETRY_ATTEMPTS` | `5` | Max retries per claim |
| `COORDINATION_LOCK_TTL_SEC` | `120` | Distributed lock TTL |
| `COORDINATION_LOCK_HEARTBEAT_SEC` | `30` | Lock heartbeat interval |
| `COORDINATION_ELECTION_TTL_SEC` | `15` | Leader election lease TTL |
| `COORDINATION_ELECTION_RENEW_SEC` | `5` | Leader renewal interval |
| `COORDINATION_DEDUP_TTL_SEC` | `60` | Dedup key TTL |
| `COORDINATION_STALE_CLAIM_MIN_IDLE_SEC` | `300` | Min idle period before a claim is eligible for the stale sweep |
| `COORDINATION_STALE_CLAIM_INTERVAL_SEC` | `60` | Stale-claim sweep interval |
| `COORDINATION_SHUTDOWN_CLEANUP_TIMEOUT_SEC` | `45` | Graceful shutdown cleanup deadline |

Redis client (service-specific override of the shared defaults):

| Variable | Default | Description |
|---|---|---|
| `REDIS_RETRY_INTERVAL_SEC` | `5` | Retry backoff base |
| `REDIS_MAX_WAIT_SEC` | `180` | Total wait cap before failing the Redis op |
| `REDIS_PING_TIMEOUT_SEC` | `2` | Per-ping timeout |

Snapshot writer + protection plan:

| Variable | Default | Description |
|---|---|---|
| `SNAPSHOT_WRITE_MAX_ATTEMPTS` | `5` | Retries on snapshot write failure |
| `SNAPSHOT_WRITE_RETRY_INTERVAL_SEC` | `2` | Retry interval |
| `PROTECTION_PLAN_TICK_INTERVAL_SEC` | `31` | Protection-plan evaluator tick cadence |
| `PROTECTION_PLAN_REPORT_MAX_VIOLATIONS` | `5000` | Bounds the per-plan report ledger (`minimal` 2000); rows beyond the report's size budget are dropped oldest-first and flagged |
| `PROTECTION_PLAN_REPORT_CHECKPOINT_SEC` | `900` | Report checkpoint cadence (`minimal` 1800); clamped to 60–1800 so it always stays below the 1 h Event retention |

Force-sync queue:

| Variable | Default | Description |
|---|---|---|
| `FORCE_SYNC_WORKERS` | `6` | Concurrent force-sync worker count on the leader (`minimal` 2, `performance` 12) |
| `FORCE_SYNC_STREAM_MAX_LEN` | `5000` | Redis stream length cap |
| `FORCE_SYNC_DEDUP_TTL_SEC` | `600` | Dedup key TTL |
| `FORCE_SYNC_JOB_TIMEOUT_SEC` | `300` | Per-job deadline |
| `FORCE_SYNC_MAINTENANCE_INTERVAL_SEC` | `60` | Maintenance loop interval |
| `FORCE_SYNC_PEL_IDLE_RECLAIM_SEC` | `60` | PEL idle threshold for reclaim |
| `FORCE_SYNC_ACK_RETENTION_SEC` | `3600` | Ack-id retention window |

Auto-cleanup of empty application CRDs:

| Variable | Default | Description |
|---|---|---|
| `DISCOVERY_AUTO_CLEANUP_ENABLED` | `"true"` | Master switch. `false` = detector goroutine never runs (zero overhead). |
| `DISCOVERY_AUTO_CLEANUP_DELETE_ENABLED` | `"true"` | Action gate. `false` = dry-run (logs intent only). `true` = destructive cleanup (Redis purge, snapshot dir removal, CRD delete). |
| `DISCOVERY_AUTO_CLEANUP_CYCLE_INTERVAL_SEC` | `60` | Detector wake interval; each cycle lists every application once (`performance` 120) |
| `DISCOVERY_AUTO_CLEANUP_EMPTY_CYCLES_REQUIRED` | `2` | Consecutive empty cycles before a CRD becomes cleanup-eligible |
| `DISCOVERY_AUTO_CLEANUP_GRACE_PERIOD_SEC` | `0` | Wall-clock floor from first-empty observation to eligibility; `0` = none |

Misc:

| Variable | Default | Description |
|---|---|---|
| `REST_EXPORTER_DURATION_LOG_ENABLED` | `"true"` | Log REST → exporter call durations |
| `REST_EXPORTER_DURATION_LOG_DEDUP_SEC` | `10` | Dedup window for the duration logs |
| `SELF_MONITORING_ENABLED` | `{{ .Values.app.selfMonitoring.enabled }}` (tpl) | Set from `app.selfMonitoring.enabled`; `false` excludes Telark's own namespace from discovery. See [Self-monitoring](../../docs/INSTALL.md#self-monitoring) |

Insights page index (see [Insights page](#insights-page)):

| Variable | Default | Description |
|---|---|---|
| `INSIGHTS_INDEX_REFRESH_SEC` | `15` | Incremental index refresh interval (documents written since the last refresh). Keep it well under 60 s: the dashboard asks for fresh reads for 60 s after a user's triage, then relies on this refresh |
| `INSIGHTS_INDEX_RESYNC_SEC` | `300` | Full membership resync; drops expired or deleted documents |
| `INSIGHTS_STALE_AFTER_SEC` | `86400` | An active insight not seen for this long shows as `stale` |

#### `services.analyzer.env`

The analyzer's on/off switch, model and auto-analyze setting live on the TelarkConfig CR (set in Settings; a fresh install seeds it on, with `granite4:350m` and auto-analyze off, and an existing CR is never rewritten); only runtime limits live here. Probes use the shared `/api/v1/status/{live,ready}` paths; readiness fails while Redis is unreachable.

| Variable | Default | Description |
|---|---|---|
| `REDIS_POOL_SIZE` | `10` | Redis client connection pool size |
| `OLLAMA_HOST` | `app.ollama.runtimeUrl`, else `http://<release>-ollama:11434` (tpl) | Model runtime URL |
| `CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins answered with CORS headers; empty sends none (the dashboard proxies the analyzer on its own origin). See [CORS](../../docs/INSTALL.md#cors) |
| `OLLAMA_AUTO_PULL` | `{{ .Values.app.ollama.autoPull }}` (tpl) | Pull a missing model on demand |
| `ANALYZER_MODE` | `fast` | `fast` = rules + one short narration; `deep` = the multi-step tool loop (4+ vCPU or GPU) |
| `ANALYZER_NUM_THREAD` | `2` | Threads per model request; = `ollama.resources.limits.cpu`, never above the node's vCPU (oversubscription makes a run minutes long) |
| `ANALYZER_NARRATE_TIMEOUT_SEC` | `45` | Fast mode: timeout of the narration call; on timeout the rule text stays |
| `ANALYZER_MAX_STEPS` | `8` | Deep mode: model turns per analysis |
| `ANALYZER_MAX_TOOL_CALLS` | `8` | Deep mode: read-only tool calls per analysis |
| `ANALYZER_TOOL_RESULT_MAX_BYTES` | `2048` | Tool output cap; keeps the context small |
| `ANALYZER_WALL_SEC` | `480` | Deep mode: hard time limit per analysis |
| `ANALYZER_LOOP_TIMEOUT_SEC` | `120` | Deep mode: per model call; covers a cold load |
| `ANALYZER_EMIT_TIMEOUT_SEC` | `180` | Deep mode: timeout of the final structured answer |
| `ANALYZER_CHARS_PER_TOKEN` | `3.5` | Token estimate for the context fit check |
| `ANALYZER_CONTEXT_TOKENS` | `4096` | Must match `OLLAMA_CONTEXT_LENGTH`; deep: `8192` |
| `ANALYZER_QUEUE_MAX` | `100` | Pending jobs before Analyze answers 429 |
| `ANALYZER_AUTO_COOLDOWN_SEC` | `600` | Per-app gap between incident analyses |
| `ANALYZER_MANUAL_COOLDOWN_SEC` | `60` | Per-app gap between manual analyses |
| `ANALYZER_CONFIG_POLL_SEC` | `30` | How often Settings are re-read |
| `ANALYZER_REVIEW_INTERVAL_SEC` | `7200` | Each app is re-reviewed at most this often by the sweep; `0` = no sweep (Analyze still reviews) |
| `ANALYZER_REVIEW_TICK_SEC` | `120` | Sweep wake interval |
| `ANALYZER_REVIEW_APPS_PER_MIN` | `20` | Sweep rate limit (`minimal` 10, `performance` 60) |
| `ANALYZER_REVIEW_WORKLOADS_MAX` | `10` | Workloads reviewed per app; the rest keep their cards untouched |
| `ANALYZER_USAGE_MIN_SAMPLES` | `12` | Usage samples required before usage rules fire |
| `ANALYZER_USAGE_MIN_SPAN_SEC` | `43200` | Time those samples must span |
| `ANALYZER_CHANGE_VELOCITY_PER_DAY` | `20` | 7-day average changes per day that flags an app as changing very often |
| `ANALYZER_CHANGE_RISK_MIN_SPAN_SEC` | `259200` | Change history an app needs before change-rate rules apply |
| `ANALYZER_PRODUCTION_PATTERN` | `(^\|[-_.])(prod\|production\|prd)($\|[-_.])` | Case-insensitive regex; an app is production when a namespace or a covering plan's environment matches it; per-workload rules (replicas, disruption budget, digest pinning) use the workload's own namespace. An invalid regex fails the pod at start |

RBAC: the analyzer ClusterRole is read-only (`get`, `list`). Besides pods, events and workloads (incident analysis), setup reviews list four kinds per app namespace: `services` (selectors that match no pod, exposure), `policy/poddisruptionbudgets` (missing or blocking budgets), `autoscaling/horizontalpodautoscalers` (autoscaling limits and conflicts) and `networking.k8s.io/networkpolicies` (namespaces without a policy). It never reads ConfigMaps, Secrets, nodes, metrics or RBAC objects, and never writes. With an older chart these lists answer 403: those rule families are skipped (no card created or resolved) and a warning is logged.

#### Analyzer runtime (ollama)

`app.ollama.enabled=true` (default) installs the ollama subchart as the analyzer's model runtime. Nothing else talks to it: a NetworkPolicy admits only the analyzer pods on port 11434 and allows HTTPS egress only while `app.ollama.autoPull=true`. The chart pulls no model at start (`ollama.ollama.models.pull` stays empty: the subchart pulls in a `postStart` hook that ignores `app.ollama.autoPull`, and a failed pull there restarts the container in a loop). With `app.ollama.autoPull=true` and the analyzer enabled (the fresh-install default), the analyzer itself pulls the model chosen in Settings (default `granite4:350m`, 708 MB) at its first config poll after start when the runtime lacks it, and retries every `ANALYZER_CONFIG_POLL_SEC` while the pull fails; until the pull finishes, a fast-mode analysis keeps the rule text and skips the narration. Models live on a volume (10Gi on the cluster's default class; 20Gi to trial 8B models) that carries `helm.sh/resource-policy: keep`, so they survive pod restarts, disabling and uninstalling.

Sizing does not follow `app.mode`: Helm resolves a subchart's values before the mode preset is applied, so `ollama.resources` is one value for every mode. The default (requests `250m` / `1536Mi`, limit cpu `2`) is the CPU tiny profile below; measured with `granite4:350m` loaded at a 4k context, ollama holds about 1.1Gi and idles near 0 CPU, and a narration bursts to the 2-core limit for a few seconds. The request is kept low so `minimal` still fits one 2 vCPU / 8 GiB node; raise it with the profile values on bigger nodes.

How a run works (`ANALYZER_MODE=fast`, default): the analyzer reads the app's overview, change history, workload status and warning events, and deterministic rules turn them into insight cards, shown within about 2 s. One short schema-constrained model call then rewrites only their title and summary; if it fails or times out, the rule text stays and the run still completes ("rules only"). `ANALYZER_MODE=deep` runs the multi-step tool loop instead.

Profiles (one slot):

| Profile | Model | Requests | Limits | Values | Latency |
|---|---|---|---|---|---|
| CPU tiny (default) | `granite4:350m` | cpu 250m, memory 1536Mi | cpu 2 | chart defaults | about 8 s per insight |
| CPU 4 vCPU | `qwen3:1.7b` | cpu 1, memory 4Gi | cpu 4 | `ollama.resources.requests.cpu=1`, `ollama.resources.requests.memory=4Gi`, `ollama.resources.limits.cpu=4`, `services.analyzer.env.ANALYZER_NUM_THREAD=4` | 20–45 s |
| GPU / deep | `qwen3:4b` | cpu 1, memory 3Gi, `nvidia.com/gpu` 1 | cpu 2, `nvidia.com/gpu` 1 | `ollama.resources.requests.cpu=1`, `ollama.resources.requests.memory=3Gi`, `ollama.ollama.gpu.enabled=true`, `ANALYZER_MODE=deep`, `ANALYZER_CONTEXT_TOKENS=8192` and `OLLAMA_CONTEXT_LENGTH=8192`, `ANALYZER_WALL_SEC=480` | minutes on CPU; seconds on GPU |

Models (all Apache-2.0; Settings shows the license of any tag you type and warns on non-commercial ones such as `qwen2.5:3b`). Fast mode accepts any installed model; deep mode requires the `tools` capability:

| Model | License | Note |
|---|---|---|
| `granite4:350m` | Apache-2.0 | Default; CPU tiny |
| `qwen3:1.7b` | Apache-2.0 | CPU 4 vCPU; better prose |
| `qwen3:4b` | Apache-2.0 | GPU / deep |
| `qwen2.5:7b` | Apache-2.0 | GPU; 4.7 GB, raise memory |

Modes:

- **Connected** (`app.ollama.autoPull=true`): the analyzer pulls the chosen model; the ollama pod gets port-443 egress. On Cilium or Calico you can narrow it with an FQDN policy (e.g. `CiliumNetworkPolicy` `toFQDNs: registry.ollama.ai`).
- **Air-gapped** (`app.ollama.autoPull=false`): no pulls, no HTTPS egress; provide the model one of two ways:
  - **Pre-seeded volume:** `ollama.persistentVolume.existingClaim=<pvc>`, a claim whose root holds `models/` (blobs and manifests) copied from `~/.ollama/models` on a connected machine.
  - **Baked image:** build `FROM ollama/ollama:0.17.7` with `COPY models /models`, then set `ollama.image.repository` / `ollama.image.tag`, `ollama.persistentVolume.enabled=false` and add `{name: OLLAMA_MODELS, value: /models}` to `ollama.extraEnv` (a values file replaces the whole list, so copy the chart's entries too). Never bake under `/root/.ollama`: the subchart always mounts a volume there, which hides the model.
- **Self-hosted endpoint** (`app.ollama.runtimeUrl=http://<host>:11434`, `app.ollama.enabled=false`): the analyzer talks to an Ollama you run (for example on a GPU host). It must speak the Ollama API; no key, no Secret. No ollama NetworkPolicy is rendered; the analyzer pod's egress follows your cluster's policies. Set `ANALYZER_NUM_THREAD` to that host's cores.

Memory: `ollama.resources` has no memory limit on purpose. Ollama checks free memory as the cgroup limit minus current usage, and usage counts the page cache of pulled model files, so any limit eventually refuses model loads. Without a limit it reads the node's available memory, which is cache-aware; `OLLAMA_KEEP_ALIVE=-1` keeps the loaded model resident. The pod is Burstable and the 1536Mi request still reserves memory. For Guaranteed QoS set requests = limits with memory ≥ 3 × model size + 1Gi and re-check after every pull. Deleting unused models (`DELETE /api/delete`) frees the cache.

#### Recommendations

Besides incidents, the analyzer reviews each app's setup with deterministic rules (reliability, resources, scaling, security, images, config, networking, change risk, protection, multi-namespace consistency) and shows the findings as recommendation cards on the Insights page (per app: `/insights?app=<namespace>/<name>`). Reviews never call the model.

- **When:** after every analysis run (skipped while jobs are queued), and from a sweep every `ANALYZER_REVIEW_TICK_SEC`: apps whose generation changed first, then any app not reviewed for `ANALYZER_REVIEW_INTERVAL_SEC`, at most `ANALYZER_REVIEW_APPS_PER_MIN` (about 2 000 apps in 100 min at the default). The sweep yields to queued analyses and runs only while the analyzer is enabled in Settings.
- **Budget per mode:** `minimal` 10 apps/min, `standard` 20, `performance` 60.
- **Production:** `ANALYZER_PRODUCTION_PATTERN` marks an app as production (namespace or plan environment name); production raises single-replica and missing-budget findings to warning and enables the protection-plan rules.
- **Usage:** usage rules (near limit, over/under-provisioned) need `ANALYZER_USAGE_MIN_SAMPLES` samples over `ANALYZER_USAGE_MIN_SPAN_SEC`, taken from the Application metrics; without metrics-server they stay silent.
- **Disable:** `ANALYZER_REVIEW_INTERVAL_SEC=0` stops the sweep; Analyze still reviews its app.

A rule fires only when its reads were complete; a failed or truncated read neither creates nor resolves cards. Cards can be dismissed (recommendations) or acknowledged (any card) by Contributors on insights, unless a role denies `insights.triageinsights.deny`. Liveness/startup failures with restarts are now reported as `crashloop` instead of `probe_failure`.

#### Insights page

The dashboard's Insights page lists incidents and recommendations of every app: discovery filters and pages them in a fixed order (`GET /api/v1/insights/get`, Read on insights, excluded namespaces hidden) and the dashboard sorts and groups them. Each discovery replica keeps an in-memory index refreshed every `INSIGHTS_INDEX_REFRESH_SEC` from the analyzer's index key, so requests never touch Redis; the page stays readable while the analyzer is off. Until the first load the route answers 503. Environments come from the protection plans that cover each app.

**Rollout:** release `internal/data` and `internal/rest`, then deploy discovery with the bumped pins (an older discovery drops the new card fields and shows recommendations as incidents), then the analyzer image, then the dashboard. Upgrade `telark-crds` before `telark` (an older CRD prunes `ai.model` and `ai.autoAnalyze`), and upgrade the chart and the analyzer image together: an older analyzer ignores the new variables but runs its tool loop in the 4k context. A pre-upgrade hook Job deletes the old AI provider key Secret `telark-ai-provider-key`; no manual step. Existing TelarkConfig CRs keep their model (e.g. `qwen3:4b`) until changed in Settings; switch to `granite4:350m` on CPU nodes. Dashboard bundles older than this release show an empty insights panel until upgraded.

#### `services.notifier.env`

| Variable | Default | Description |
|---|---|---|
| `NOTIFIER_APPLY_WORKERS` | `8` | Concurrent apply workers; an application always maps to the same worker, so its updates stay ordered (`minimal` 2, `performance` 32; max 62) |

Also inherits `app.shared.redis`, `app.shared.nats` and the NATS consumer credentials (`natsUser: consumer`).

#### `services.auth.env`

Redis:

| Variable | Default | Description |
|---|---|---|
| `CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins answered with CORS headers; empty sends none (the dashboard proxies every API on its own origin). See [CORS](../../docs/INSTALL.md#cors) |
| `REDIS_DB` | `1` | Redis DB index |
| `REDIS_RETRY_INTERVAL_SEC` | `2` | Retry backoff base |
| `REDIS_MAX_WAIT_SEC` | `30` | Total wait cap before failing the Redis op |
| `REDIS_PING_TIMEOUT_SEC` | `3` | Per-ping timeout |

Bootstrap (templated from `app.auth.bootstrap`):

| Variable | Source | Description |
|---|---|---|
| `BOOTSTRAP_ADMIN` | `{{ .Values.app.auth.bootstrap.admin }}` | The bootstrap admin's email; `break-glass --enroll` marks it `bootstrap: true`, and it is refused on bare-email passkey registration. OIDC and self-registration never grant it Admin |

WebAuthn / passkey (templated from `app.auth.passkey`):

| Variable | Source | Description |
|---|---|---|
| `RP_ID` | `{{ .Values.app.auth.passkey.id }}` | Relying Party identifier; empty follows the request host |
| `RP_NAME` | `{{ .Values.app.auth.passkey.name }}` | Display name shown to the user |
| `RP_ORIGIN` | `{{ .Values.app.auth.passkey.origin }}` | Allowed origin(s), comma-separated; empty follows the request `Origin` |
| `ENROLL_INVITE_TTL_SEC` | inline (`"3600"`) | Lifetime (seconds) of an enrollment link created from Members; a new link revokes the previous one. See [Enrollment links](../../docs/INSTALL.md#enrollment-links) |
| `CHALLENGE_TIMEOUT` | inline (`"60"`) | Challenge TTL (seconds) |
| `SESSION_EXPIRY` | inline (`"24"`) | Session TTL (hours) |

OIDC trust keys (mounted from the Secret named by `app.auth.oidc.existingSecret`, else `<fullname>-oidc-trust-secret`):

| Variable | Default | Description |
|---|---|---|
| `OIDC_TRUST_FILE` | `/etc/telark/oidc/googleJwkJson` | Pinned Google JWK set, re-read when it changes; used when the TelarkConfig disallows egress |

Cleanup controllers + queue:

| Variable | Default | Description |
|---|---|---|
| `RECONCILE_TICK_SECONDS` | `5` | Reconciler tick interval |
| `RECONCILE_PASS_DEADLINE_SECONDS` | `30` | Per-pass deadline |
| `CLEANUP_WORKERS_PER_TYPE` | `2` | Workers per cleanup type |
| `CLEANUP_STREAM_MAXLEN` | `10000` | Cleanup stream length cap |
| `CLEANUP_LAG_ALERT_THRESHOLD` | `500` | Lag threshold for alerts |
| `CLEANUP_SWEEPER_INTERVAL_SECONDS` | `60` | Sweeper interval |
| `CLEANUP_JOB_MAX_ATTEMPTS` | `5` | Max retries per cleanup job |
| `CLEANUP_DEDUP_TTL_SECONDS` | `600` | Dedup key TTL |
| `CLEANUP_XCLAIM_MIN_IDLE_SECONDS` | `60` | XCLAIM min idle threshold |
| `CLEANUP_LIST_TIMEOUT_SECONDS` | `10` | List op timeout |
| `CLEANUP_PATCH_TIMEOUT_SECONDS` | `5` | Patch op timeout |
| `CLEANUP_MAX_CONCURRENT_PATCHES` | `4` | Concurrent patch cap |
| `RECONCILE_BACKOFF_INITIAL_SECONDS` | `5` | Initial backoff for failed reconciles |
| `RECONCILE_BACKOFF_MAX_SECONDS` | `300` | Max backoff cap |

#### `services.ui.env`

No env.
