# Install and configure

This guide installs Telark with the defaults first, then covers exposure, sizing, the bundled subcharts and the settings most installs never change. For a guided first run, see [Getting started](getting-started.md). Every value is also listed in [`charts/telark/VALUES.md`](../charts/telark/VALUES.md).

## Prerequisites

- Kubernetes 1.30 or newer; 1.33+ is the tested target. The chart's `kubeVersion` enforces the floor ([compatibility](../README.md#compatibility)).
- Helm 3.
- A StorageClass for the exporter's two PVCs (snapshots and protection plan reports). The `standard` and `performance` modes run two exporter replicas that share both volumes, so the class must be **ReadWriteMany** (`efs-sc` on EKS with the EFS CSI driver). A one-node cluster (`--set app.singleNode=true`) and `minimal` mode run one replica on any default class.

## 1. Install

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.persistence.storageClass=<rwx-class> \
  --set 'app.auth.bootstrap.admins={jane.doe@example.com}'
```

- Replace `jane.doe@example.com` with your own email ([First admin](#2-first-admin)). The chart ships no admin and refuses to render without one.
- In `standard` and `performance`, the install fails early without a storage class, because a ReadWriteMany claim against block storage never binds. On a one-node cluster pass `--set app.singleNode=true` instead of the class.
- The command installs everything: the CRDs, NATS configuration and `standard` sizing all ship in the chart. The CRDs are cluster-scoped and kept on uninstall (`resource-policy: keep`). If you manage CRDs out of band (a GitOps tool applies them first), add `--set crds.enabled=false`.
- From a checkout, `./charts/telark` works in place of the OCI reference.

The examples below leave out the class and admin flags. Keep yours on every command.

## 2. First admin

`app.auth.bootstrap.admins` is empty by default and passkey self-registration (`app.auth.passkey.selfRegistration`) is `"false"`. With neither, nobody could sign in, so the chart fails the render and the auth service refuses to start until you set at least one admin email:

```sh
--set 'app.auth.bootstrap.admins={jane.doe@example.com,john.doe@example.com}'
```

These emails get the Admin role only from a verified identity: their first Google SSO sign-in (the identity provider verified the email), or an enrolment you start yourself. Registering a passkey from the login page never grants Admin, and a bootstrap email cannot be registered there at all.

To enrol the first admin with a passkey (no SSO yet), run the auth service's break-glass command and open the link within 10 minutes:

```sh
kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email jane.doe@example.com --enroll
# open https://<dashboard-host>/register?enroll=<token>
```

With the default port-forward, the host is `http://localhost:3000`. The admin then sends enrolment links to other passkey users.

`app.auth.passkey.selfRegistration="true"` lets anyone who reaches the dashboard create a ReadOnly account. Leave it off unless only people you trust can reach the dashboard.

Bootstrap accounts belong to the chart. The API refuses to delete them, only they may edit their own record, and no dashboard user can create or rename a user to one of these emails (the exporter receives the same list as `BOOTSTRAP_ADMINS`). Only a bootstrap account may delete or suspend another administrator, and non-administrators never see administrator accounts.

## 3. Verify

```sh
kubectl get pods -n telark -l app.kubernetes.io/instance=telark
helm test telark -n telark      # readiness probe against the auth service
```

## Access the dashboard

The dashboard (the `ui` service) is deployed by default behind a ClusterIP Service on port 8080, reachable only inside the cluster. The chart bundles no ingress or gateway controller; it uses the one your cluster already runs.

| Method | Use it for | Needs |
|---|---|---|
| [Port-forward](#port-forward) (default) | Trying Telark, development | Nothing |
| [NodePort](#nodeport) | Lab clusters | Firewall access to a node port, TLS in front |
| [LoadBalancer](#loadbalancer) | Cloud clusters without an ingress controller | A cloud load balancer or MetalLB, TLS in front |
| [Ingress](#ingress) | Production | An ingress controller |
| [Gateway API](#gateway-api) | Production on Gateway API | A Gateway and a controller that implements `HTTPRoute` v1 |

### Passkeys and HTTPS

- **Passkeys need HTTPS.** Browsers enable WebAuthn only on `https://` or `http://localhost`. Port-forward works without TLS. On a NodePort, LoadBalancer or Ingress host, the dashboard shows a warning and passkey sign-in and registration stay disabled until the host serves a certificate the browser trusts. NodePort and LoadBalancer expose the plain-HTTP `ui` service, so terminate TLS in front of them; the Ingress can get a certificate from cert-manager ([below](#tls-with-cert-manager)).
- **Passkeys are bound to a host.** By default the WebAuthn relying party follows the request host, so a passkey registered on `localhost:3000` is not accepted on the NodePort or Ingress hostname; register again there.
- **Pin the relying party in production.** Set `app.auth.passkey.id=<domain>` and `app.auth.passkey.origin=https://<domain>` so it never follows client-supplied `Host` or `Origin` headers. With `ingress.enabled` or `gateway.enabled` the chart refuses to render until both are set; pin them yourself for NodePort and LoadBalancer.
- **Signing in on a second host.** A passkey is always created for the host the browser is open on. While signed in, open Settings → Security → Passkeys, choose "Add on another device", and open the one-time link it shows (valid for 10 minutes) on the other host.

### Port-forward

Maps local port 3000 to the service's 8080:

```sh
kubectl port-forward -n telark svc/telark-ui-service 3000:8080
# open http://localhost:3000
```

### NodePort

Opens the same port on every node:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set services.ui.serviceType=NodePort \
  --set services.ui.nodePort=30080
# open http://<any-node-ip>:30080
```

Leave `services.ui.nodePort` unset and Kubernetes allocates one from 30000–32767 (`kubectl get svc -n telark telark-ui-service`). Allow the port inbound in the nodes' firewall or cloud security group.

### LoadBalancer

Needs a cloud load balancer (EKS, GKE, AKS) or MetalLB on bare metal:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set services.ui.serviceType=LoadBalancer
kubectl get svc -n telark telark-ui-service   # EXTERNAL-IP
```

### Ingress

Needs an ingress controller. If you have none, install ingress-nginx:

```sh
helm upgrade --install ingress-nginx ingress-nginx \
  --repo https://kubernetes.github.io/ingress-nginx \
  --namespace ingress-nginx --create-namespace
```

Then enable the chart's Ingress for your hostname:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.host=telark.example.com \
  --set app.auth.passkey.id=telark.example.com \
  --set app.auth.passkey.origin=https://telark.example.com
```

Without `ingress.tls` the Ingress serves plain HTTP, session tokens included, and the install notes print a warning. Add TLS as below or terminate it in front of the controller.

| Flag | Default | Description |
|---|---|---|
| `ingress.enabled` | `false` | Create an Ingress for the dashboard |
| `ingress.className` | `""` | IngressClass (for example `nginx`) |
| `ingress.host` | `""` | Hostname (`""` matches any host) |
| `ingress.service` | `ui` | Which `services.<key>` to route to |
| `ingress.path` / `ingress.pathType` | `/` / `Prefix` | Route path and match type |
| `ingress.tls` | `[]` | TLS blocks, for example `[{secretName: telark-tls, hosts: [telark.example.com]}]` |
| `ingress.annotations` | `{}` | Controller annotations (cert-manager and others) |

#### TLS with cert-manager

Install cert-manager with its CRDs, create a Let's Encrypt `ClusterIssuer` that solves HTTP-01 challenges through the nginx class, then point the chart's Ingress at it.

- The DNS name must already resolve to the ingress controller's load balancer (`kubectl get svc -n ingress-nginx ingress-nginx-controller` shows its `EXTERNAL-IP`), or the challenge cannot pass.
- A self-signed certificate is not enough: Chrome also disables WebAuthn on pages with certificate errors.
- Replace `admin@example.com` with the address Let's Encrypt should notify about expiring certificates.

```sh
helm upgrade --install cert-manager cert-manager \
  --repo https://charts.jetstack.io \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true
```

```sh
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt
    solvers:
      - http01:
          ingress:
            ingressClassName: nginx
EOF
```

```sh
helm upgrade --install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.host=telark.example.com \
  --set ingress.tls[0].secretName=telark-tls \
  --set ingress.tls[0].hosts[0]=telark.example.com \
  --set ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt \
  --set app.auth.passkey.id=telark.example.com \
  --set app.auth.passkey.origin=https://telark.example.com
# open https://telark.example.com
```

### Gateway API

Instead of an Ingress, the chart can render an `HTTPRoute` (`gateway.networking.k8s.io/v1`) attached to a Gateway you already run. It needs Gateway API v1.0+ CRDs and a controller that implements `HTTPRoute` v1: Envoy Gateway, NGINX Gateway Fabric, Cilium or Istio. ingress-nginx does **not** implement Gateway API.

Install the CRDs (standard channel) if the cluster has none:

```sh
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.6.2/standard-install.yaml
```

Then attach the route to your Gateway:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set gateway.enabled=true \
  --set gateway.parentRefs[0].name=<gateway> \
  --set gateway.hostnames[0]=telark.example.com \
  --set app.auth.passkey.id=telark.example.com \
  --set app.auth.passkey.origin=https://telark.example.com
```

| Flag | Default | Description |
|---|---|---|
| `gateway.enabled` | `false` | Create an HTTPRoute for the dashboard |
| `gateway.parentRefs` | `[]` | Gateways to attach to; entries take `name`, `namespace`, `sectionName` |
| `gateway.hostnames` | `[]` | Hostnames the route matches (`[]` matches any host) |
| `gateway.service` | `ui` | Which `services.<key>` to route to |
| `gateway.annotations` | `{}` | HTTPRoute annotations |

## Sizing modes

`app.mode` sizes every Telark service (replicas, resources, client rate limits and disruption budgets) from one flag. The default is `standard`:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance
```

| Mode | For | Capacity (measured 2026-09-17) |
|---|---|---|
| `minimal` | Development, demos, evaluation. Single replica, no autoscaling, no PDBs | A few hundred applications |
| `standard` (default) | Small and mid-size production. Every service starts at 1 replica and scales on CPU up to 3 (HPA); the exporter runs 2 replicas sharing a ReadWriteMany snapshot volume. Add `--set vpa.enabled=true` for vertical scaling | Verified at 2 000 applications |
| `performance` | Large clusters. As `standard`, with an HPA ceiling of 5, disruption budgets that keep one pod through drains, larger requests and limits, and a 50 GiB volume | Beyond 1 000 applications |

`app.mode` sizes Telark's own services only. Helm resolves a subchart's values before the mode is known, so Redis, NATS, Kyverno, metrics-server and Ollama ship fixed production-grade defaults, identical in every mode. There is nothing to tune up to 2 000 applications; beyond that, size Redis ([Subcharts](#subcharts)).

## Install-time flags

Set values with `--set key=value`. Helm does not remember them across upgrades, so pass the same flags on every `helm upgrade`.

### App

| Flag | Default | Description |
|---|---|---|
| `app.mode` | `standard` | Size every Telark service: `minimal` \| `standard` \| `performance` ([Sizing modes](#sizing-modes)) |
| `app.name` | `telark` | Resource-name prefix. The CRD group is always `telark.io` ([ADR 0003](adr/0003-constant-api-group-telark-io.md)) |
| `app.namespace` | `telark` | Install namespace. Must match the release namespace (`-n`): the subcharts follow `-n`, so a mismatch splits Redis and NATS away from the services that address them by bare name |
| `app.image.registry` | `ghcr.io/telark` | Registry and organization hosting the service images |
| `app.image.pullPolicy` | `Always` | Image pull policy |
| `app.image.pullSecrets` | `[]` | Image pull secrets for a private registry |
| `app.persistence.size` | `10Gi` | Exporter snapshot PVC size (`minimal` 1Gi, `performance` 50Gi) |
| `app.persistence.reportsSize` | `2Gi` | Exporter reports PVC size (`minimal` 512Mi, `performance` 10Gi) |
| `app.persistence.storageClass` | `""` | Class for both exporter PVCs. Must be ReadWriteMany in `standard` and `performance` (two exporter replicas); `""` means the cluster default, valid only with `app.singleNode=true` or `minimal` |
| `app.singleNode` | `false` | One-node cluster: the exporter runs 1 replica on ReadWriteOnce, so no ReadWriteMany class is needed. Access mode and update strategy follow the replica count automatically |
| `app.crdGuard.enabled` | `true` | Admission guard: only the owning service accounts may write Telark CRs ([CRD write guard](#crd-write-guard)) |
| `app.crdGuard.enforce` | `true` | With the guard on, `false` audits and `true` rejects |
| `app.auth.bootstrap.admins` | `[]` | Emails granted Admin on first verified sign-in; required while self-registration is off ([First admin](#2-first-admin)) |
| `app.auth.passkey.selfRegistration` | `"false"` | `"true"` lets anyone who reaches the dashboard register a passkey account |
| `app.auth.passkey.id` / `origin` | `""` | WebAuthn relying party; required with `ingress.enabled` or `gateway.enabled` ([Passkeys and HTTPS](#passkeys-and-https)) |
| `app.networkPolicy.enabled` | `true` | Ingress NetworkPolicies for the Telark pods and NATS ([Network policies](#network-policies)) |
| `app.serviceToken.existingSecret` | `""` | Secret (key `token`) you manage instead of the generated service token ([GitOps](#gitops-cluster-less-renders)) |
| `services.<svc>.env.CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins the exporter, discovery, auth and analyzer answer with CORS headers ([CORS](#cors)) |

### Subcharts

The bundled dependencies ship production-grade defaults for every mode, so you rarely touch these. The table lists the on/off switches and the values the chart pins. Any other upstream key works the same way (`--set <subchart>.<path>`).

| Flag | Default | Description |
|---|---|---|
| `crds.enabled` | `true` | Install the CRDs (the telark-crds subchart); `false` to manage them out of band |
| `app.kyverno.enabled` | `true` | Install Kyverno, the policy engine |
| `app.kyverno.failOpen` | `true` | Admission fails open while Kyverno is down ([Policy engine fail-open](#policy-engine-fail-open)) |
| `app.ollama.enabled` | `true` | Install Ollama, the local model runtime the analyzer needs ([Analyzer runtime](#analyzer-runtime)) |
| `app.ollama.autoPull` | `true` | Let the analyzer pull a missing model; `false` for air-gapped installs |
| `app.ollama.runtimeUrl` | `""` | Ollama-API endpoint you run yourself (URL only, no key); empty uses the bundled runtime |
| `metrics-server.enabled` | `true` | Install metrics-server; `false` if the cluster already has one |
| `redis.architecture` | `standalone` | `replication` for a replicated Redis |
| `redis.image.digest` | pinned | Bitnami publishes only `bitnami/redis:latest`, so the chart pins one build by digest. To take a newer build, resolve the current digest (`docker buildx imagetools inspect bitnami/redis:latest`) and pass `--set redis.image.digest=sha256:<digest>` |
| `redis.master.persistence.size` | `4Gi` | Redis PVC size |
| `redis.master.resources.limits.memory` | `512Mi` | Redis memory limit (requests `100m` / `128Mi`); see the note below |
| `nats.persistence.size` | `4Gi` | NATS JetStream PVC size |
| `nats.existingSecrets.publisher` / `consumer` | `""` | Secrets (keys `username`, `password`) you manage instead of the generated NATS users ([GitOps](#gitops-cluster-less-renders)) |
| `kyverno.admissionController.replicas` | `2` | Kyverno admission replicas |
| `kyverno.admissionController.container.extraArgs.clientRateLimitQPS` | `50` | Kyverno API QPS |
| `metrics-server.resources.limits.memory` | `400Mi` | metrics-server memory limit |
| `metrics-server.args` | kubelet TLS verified | Add `--kubelet-insecure-tls` only where kubelet certificates are self-signed ([metrics-server kubelet TLS](#metrics-server-kubelet-tls)) |
| `ollama.persistentVolume.size` | `10Gi` | Model storage (when enabled); kept on uninstall |

**Sizing Redis for large installs.** Redis keeps everything in memory and never evicts, so at its memory limit the pod is OOM-killed. The analyzer's documents are the largest part: in the worst case about 210 MiB for 2 000 applications, which the `512Mi` limit covers. For more applications, raise the limit in proportion, for example `--set redis.master.resources.limits.memory=1Gi`. The chart sets `redis.master.resources`, so `redis.master.resourcesPreset` has no effect.

The complete list of Telark values and pinned subchart values is the generated [`charts/telark/VALUES.md`](../charts/telark/VALUES.md). Full upstream options are in each dependency's own chart (Bitnami for Redis and NATS, plus Kyverno, metrics-server and Ollama).

Example:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance \
  --set app.persistence.storageClass=efs \
  --set 'app.auth.bootstrap.admins={jane.doe@example.com}'
```

## Analyzer runtime

Insights runs on an in-cluster model runtime (Ollama, installed by default), so analysis data stays in the cluster. A fresh install turns the analyzer on with `granite4:350m`, which answers in seconds on 2 vCPU, and leaves automatic analysis off. Change the model, turn on automatic analysis or turn the analyzer off in Settings; upgrades keep the existing settings.

The runtime has one size for every mode, because Helm resolves subchart values before `app.mode` applies: requests `250m` CPU and `1536Mi` memory, a 2-CPU limit and no memory limit. That is enough for `granite4:350m` while `minimal` still fits one 2 vCPU / 8 GiB node. Models live on a 10Gi volume that is kept on uninstall, so they survive restarts and reinstalls.

| Setup | Flags | Behaviour |
|---|---|---|
| Connected (default) | `app.ollama.autoPull=true` | The analyzer pulls the chosen model right after start when the runtime lacks it (708 MB for `granite4:350m`), and the Ollama pod gets HTTPS egress for it. Until the pull finishes, an analysis shows the rule text without the model's rewording; **Install model** in Settings starts the pull right away |
| Air-gapped | `app.ollama.autoPull=false` | Nothing is pulled and the Ollama pod gets no HTTPS egress. Pre-load the model on a seeded volume or a baked image, as described in the chart README |
| Your own runtime | `app.ollama.enabled=false`, `app.ollama.runtimeUrl=http://<host>:11434` | The analyzer uses an Ollama-API endpoint you run (URL only, no key) |

Larger profiles (4 vCPU CPU, GPU and deep mode), model licences and the air-gapped procedure are in the chart README, [Analyzer runtime (ollama)](../charts/telark/README.md#analyzer-runtime-ollama).

**Recommendations.** The analyzer also reviews each app's setup (replicas, disruption budgets, resources, autoscaling, images, network policies, protection plans) and shows recommendation cards on the Insights page ([Insights page](../charts/telark/README.md#insights-page)).

- A sweep re-reviews every app every 2 hours (`services.analyzer.env.ANALYZER_REVIEW_INTERVAL_SEC`, `0` disables it) at 20 apps per minute (10 in `minimal`, 60 in `performance`).
- The reviews need read-only access to Services, PodDisruptionBudgets, HorizontalPodAutoscalers and NetworkPolicies, which the chart grants the analyzer ClusterRole (`get`, `list`; no writes).
- Apps count as production when a namespace or a covering plan's environment matches `services.analyzer.env.ANALYZER_PRODUCTION_PATTERN` (default `(^|[-_.])(prod|production|prd)($|[-_.])`, case-insensitive). Set it to your own naming, for example `--set-string 'services.analyzer.env.ANALYZER_PRODUCTION_PATTERN=^live-'`.
- Per-workload rules (single replica, missing disruption budget, image digest pinning) judge each workload by its own namespace, so the dev namespace of a multi-namespace app is not held to production rules.

Details: [Recommendations](../charts/telark/README.md#recommendations) in the chart README.

## Autoscaling (HPA)

The stateless services (auth, discovery, notifier, ui) can run behind a HorizontalPodAutoscaler (`autoscaling/v2`, CPU-based). HPAs need metrics-server, which ships with the chart.

- The exporter never autoscales: its replica count is fixed by the mode (2 in `standard` and `performance`, 1 in `minimal` or with `app.singleNode=true`).
- The analyzer never autoscales: one worker is bound to one runtime slot.
- `standard` and `performance` turn autoscaling on (start at 1, maximum 3 and 5); `minimal` keeps it off.

In any mode you can enable, disable or tune it per service:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set services.discovery.autoscaling.enabled=true \
  --set services.discovery.autoscaling.minReplicas=2 \
  --set services.discovery.autoscaling.maxReplicas=8 \
  --set services.discovery.autoscaling.targetCPUUtilizationPercentage=70
```

| Key | Default | Description |
|---|---|---|
| `services.<svc>.autoscaling.enabled` | mode | Turn the HPA on or off for that service (on in `standard` and `performance`, off in `minimal`) |
| `services.<svc>.autoscaling.minReplicas` | `1` | Replica floor |
| `services.<svc>.autoscaling.maxReplicas` | `3` | Replica ceiling |
| `services.<svc>.autoscaling.targetCPUUtilizationPercentage` | `80` | Scale-up CPU target |
| `services.<svc>.autoscaling.targetMemoryUtilizationPercentage` | _(unset)_ | Optional memory target |

Set the same keys under `app.serviceDefaults.autoscaling` to change the default for every service at once. When a service autoscales, Helm stops managing its replica count (`spec.replicas` is omitted) so the HPA and Helm don't conflict.

## Monitoring (Prometheus)

The chart can create a ServiceMonitor (Prometheus Operator) that scrapes every service's `/metrics`. It is off by default because it needs the Prometheus Operator CRDs already in the cluster (for example from kube-prometheus-stack).

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set monitoring.serviceMonitor.enabled=true \
  --set monitoring.serviceMonitor.labels.release=kube-prometheus-stack
```

Prometheus only picks up a ServiceMonitor whose labels match its `serviceMonitorSelector`. For kube-prometheus-stack that selector is `release: <your-release>`, so set `monitoring.serviceMonitor.labels.release` to your Prometheus release name. Without it, Prometheus ignores the monitor without any error.

| Key | Default | Description |
|---|---|---|
| `monitoring.serviceMonitor.enabled` | `false` | Create the ServiceMonitor |
| `monitoring.serviceMonitor.labels` | `{}` | Labels matching Prometheus's `serviceMonitorSelector` (usually `release: <name>`) |
| `monitoring.serviceMonitor.path` | `/metrics` | Scrape path |
| `monitoring.serviceMonitor.interval` | `30s` | Scrape interval |

The monitor selects every Telark service (`app.kubernetes.io/part-of: telark`) on the `http` port; the services must expose `/metrics` there for scraping to return data. With [network policies](#network-policies) on, a Prometheus outside the namespace also needs an allow rule.

## Security and cluster integration

These settings have safe defaults. Read them before a production install, or when something below applies to your cluster.

### CRD write guard

Telark's custom resources (users, roles, sessions, passkeys) are its authorization data. The guard (`app.crdGuard`, a ValidatingAdmissionPolicy, on and enforcing by default) rejects writes to every `telark.io` resource and subresource from anything but the owning service accounts (discovery may write only `applications/status`). A second policy does the same for the OIDC trust Secret `telark-oidc-trust-secret`. As a result, edit rights on the `telark` namespace do not turn into Telark Admin.

- Break-glass identities go in `app.crdGuard.extraAllowedUsers`, for example `--set 'app.crdGuard.extraAllowedUsers={system:serviceaccount:ops:breakglass}'`.
- `--set app.crdGuard.enforce=false` only audits (logs and allows).
- `--set app.crdGuard.enabled=false` removes the guard; the install notes warn when it is off.

### Network policies

With `app.networkPolicy.enabled=true` (default) the chart renders ingress NetworkPolicies in the release namespace:

- A default deny for every Telark pod.
- The exporter, discovery, auth, analyzer and notifier APIs accept traffic only from Telark pods of the same release.
- The dashboard (`ui`) accepts traffic on its port from anywhere, because the ingress controller, Gateway or port-forward reaches it from outside the release.
- NATS accepts clients only from discovery and notifier on 4222, and serves no monitoring port.
- Redis and Ollama keep their own policies. Egress is not restricted: the services talk to the Kubernetes API, the OIDC issuer and each other.

NetworkPolicies do nothing without a CNI that enforces them (Calico, Cilium, a cloud provider's policy add-on). Check yours before relying on them.

A Prometheus outside the namespace needs its own allow rule to scrape `/metrics` (policies add up):

```sh
kubectl apply -n telark -f - <<'YAML'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: telark-allow-prometheus
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: telark
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - port: http
YAML
```

### Policy engine fail-open

`app.kyverno.failOpen=true` (default) makes every Kyverno webhook `failurePolicy: Ignore`. While the admission controller is down, rolling out or timing out, the API server admits every request, including ones an `enforce` protection plan would reject. Workloads stay deployable when Kyverno is unhealthy, and enforcement is best-effort.

For strict enforcement set both keys. Helm cannot pass a parent value to a subchart, so the render fails when they differ:

```sh
--set app.kyverno.failOpen=false \
--set kyverno.features.forceFailurePolicyIgnore.enabled=false
```

Fail-closed means an unavailable Kyverno blocks writes to everything its webhooks match, so keep its two admission replicas and its disruption budget.

### Last-modified annotations

A Kyverno ClusterPolicy stamps `telark.io/last-modified-{by,at,operation}` on the kinds discovery tracks (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, ConfigMaps, Services, PVCs, ServiceAccounts, Ingresses, NetworkPolicies, HPAs, VPAs), outside `telark`, `kyverno` and the system namespaces. Change history uses them to name who made a change.

- Secrets are not stamped, so history does not name who changed a Secret.
- The policy renders only when the Kyverno `ClusterPolicy` API already exists, which on a first install it does not. Run `helm upgrade` once after the first install, with the same flags, to add it. Changes made before that carry no author.

### GitOps (cluster-less renders)

The chart generates the service token and the two NATS users' passwords on install and reads them back on upgrade with `lookup`. Argo CD, Flux and other `helm template` pipelines render without a cluster, so `lookup` returns nothing and every sync would produce new values: pods that restart on the new token can no longer reach their peers. Create the Secrets yourself and point the chart at them:

```sh
kubectl create secret generic telark-service-token -n telark \
  --from-literal=token="$(openssl rand -hex 32)"
kubectl create secret generic telark-nats-publisher -n telark \
  --from-literal=username=publisher --from-literal=password="$(openssl rand -hex 24)"
kubectl create secret generic telark-nats-consumer -n telark \
  --from-literal=username=consumer --from-literal=password="$(openssl rand -hex 24)"
```

```sh
--set app.serviceToken.existingSecret=telark-service-token \
--set nats.existingSecrets.publisher=telark-nats-publisher \
--set nats.existingSecrets.consumer=telark-nats-consumer
```

The OIDC trust Secret (the optional Google JWK set, key `googleJwkJson`) is rendered the same way, so every sync would empty it. Create it yourself and point the chart at it. The exporter updates it when an Admin saves the JWK set in the dashboard, and the CRD write guard and the exporter's RBAC cover the name you pass:

```sh
kubectl create secret generic telark-oidc-trust -n telark --from-literal=googleJwkJson=''
```

```sh
--set app.auth.oidc.existingSecret=telark-oidc-trust
```

The generated Secrets carry `helm.sh/resource-policy: keep`, so an uninstall never drops them.

### metrics-server kubelet TLS

The bundled metrics-server verifies kubelet serving certificates. Where kubelets use self-signed certificates (kind, minikube, some bare-metal installs) it cannot scrape, and HPAs stay at `<unknown>`. There, and only there, add the flag:

```sh
--set 'metrics-server.args={--kubelet-insecure-tls,--kubelet-preferred-address-types=InternalIP\,ExternalIP\,Hostname}'
```

### CORS

No API service sends CORS headers unless its `CORS_ALLOWED_ORIGINS` env var (`services.{exporter,discovery,auth,analyzer}.env`) lists the browser origins allowed to call it directly (comma-separated; `*` is ignored because credentials are allowed). The dashboard reaches every API through its own nginx proxy on the same origin, so installs leave them empty. For UI development against a port-forwarded cluster, set `http://localhost:3000` on all four (`charts/telark/values.dev.yaml` does).

## Upgrade

```sh
helm upgrade telark oci://ghcr.io/telark/charts/telark -n telark \
  --set app.mode=<mode>
```

Pass the same `--set` and `-f` flags you used at install: Helm does not remember them across upgrades.

### Version notes

- **Chart with the security defaults.** `app.auth.bootstrap.admins` no longer defaults to a vendor address and self-registration is off, so pass your admin email (the render fails without one). The CRD write guard now enforces, NetworkPolicies restrict ingress to the Telark APIs and NATS, and NATS moves from one shared user to a publisher (discovery) and a consumer (notifier) with new Secrets, so the NATS server and both clients restart during the rollout. With an Ingress or Gateway, pin `app.auth.passkey.id` and `origin` too.
- **From chart 0.2.1 or older, or when switching modes.** Those releases run one exporter replica on a ReadWriteOnce claim, and Kubernetes cannot change a bound claim's access mode or class. Add `--set app.singleNode=true` to keep that claim (one replica, Recreate). To move to two replicas on ReadWriteMany, uninstall, delete the `telark-exporter-snapshots-pvc` claim (snapshots are lost; copy `/snapshots` off the pod first if you need them), then reinstall with `--set app.persistence.storageClass=<rwx-class>`. The same applies when switching between `minimal` and `standard`/`performance`, or toggling `app.singleNode`.
- **Chart that adds protection plan reports.** The exporter gains a second claim, `telark-exporter-reports-pvc`, which binds on rollout with the same class and access mode as the snapshot claim. Do not upgrade with `--reuse-values`: the reports volume, mount and the `REPORTS_PATH` / `PROTECTION_PLAN_REPORT_*` entries arrive only with the new chart defaults; with `--reuse-values` the exporter logs a reports-root error at start and every report write fails. The exporter volumes render even when `app.persistence.enabled=false`, so the pods then wait on claims nobody provisions.
- **Chart that pulls from GHCR.** The service images move from Docker Hub (`telark/<service>`) to `ghcr.io/telark/<service>`, with the same names and tags. Do not upgrade with `--reuse-values`, which keeps the old `app.image.registry`. Nodes behind an egress allowlist need `ghcr.io` and `pkg-containers.githubusercontent.com`; a mirror re-syncs from `ghcr.io/telark`.
- **CRDs managed out of band** (`crds.enabled=false`): upgrade `telark-crds` before `telark` ([chart README](../charts/telark/README.md#upgrade-order)).

### Upgrading from 0.4 or older

Chart 0.5 moves every CRD to the single group `telark.io` with new kinds, field names and the Kyverno label `telark.io/protection-plan` ([ADR 0003](adr/0003-constant-api-group-telark-io.md), [CRD reference](CRDS.md)). There is no in-place migration: tear the old install down and reinstall. Every Telark object is deleted, so everyone signs in again, passkey users re-enrol, and custom roles, groups, categories, protection plans and settings are re-created by hand. OIDC users are provisioned again on first sign-in but lose their role and group assignments.

```sh
# 1. Remove the admission policies of the old protection plans (the new discovery does not see the old label)
kubectl delete policies.kyverno.io -A -l telark.erpi/protection-plan

# 2. Optional: export the old objects for reference while re-creating them
for r in applicationsasresources.erpi.telark protectionplans.erpi.telark globalconfigs.erpi.telark \
         usersasresources.erpi.telark groupsasresources.erpi.telark rolesasresources.erpi.telark \
         categoriesasclassifications.classification.telark; do
  kubectl get "$r" -n telark -o yaml > "old-$r.yaml"
done

# 3. Uninstall the release
helm uninstall telark -n telark

# 4. Clear the cleanup finalizers, then delete the nine old CRDs and their objects
for r in usersasresources.erpi.telark groupsasresources.erpi.telark rolesasresources.erpi.telark; do
  kubectl get "$r" -n telark -o name | xargs -r -P 16 -I{} kubectl patch {} -n telark --type merge -p '{"metadata":{"finalizers":null}}'
done
kubectl delete crd \
  applicationsasresources.erpi.telark protectionplans.erpi.telark globalconfigs.erpi.telark \
  usersasresources.erpi.telark groupsasresources.erpi.telark rolesasresources.erpi.telark \
  userpasskeys.auth.telark usersessions.auth.telark \
  categoriesasclassifications.classification.telark
```

Then install as in [1. Install](#1-install), with the same flags as before, and enrol the first admin with break-glass as in [2. First admin](#2-first-admin):

```sh
kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email jane.doe@example.com --enroll
# open https://<dashboard-host>/register?enroll=<token>
```

The admin then sends enrolment links to the other passkey users and re-creates roles, groups and categories. With a cluster-less render, also create the OIDC trust Secret ([GitOps](#gitops-cluster-less-renders)). Afterwards, address Telark objects by their fully qualified names (`kubectl get applications.telark.io -n telark`) or short names (`tapp`, `tplan`, `tuser`, …), especially when Argo CD is installed: its `applications.argoproj.io` answers to a plain `kubectl get applications`.

## Uninstall

Let in-progress rollbacks finish, and back up the exporter's volumes if you need the snapshots or reports.

```sh
helm uninstall telark -n telark
```

This removes every Telark service, the bundled policy engine with its webhooks and plan policies, and the exporter's snapshot and report PVCs. The pre-delete hooks run `app.kubectlImage`, so air-gapped clusters must mirror it.

Your data stays: the CRDs and custom resources (users, roles, plans and the rest), the service-token, NATS and OIDC Secrets, the Redis and NATS volumes, and the Ollama model volume. A reinstall picks up where you left off.

With an external policy engine (`app.kyverno.enabled=false`), also remove the plan policies, which would otherwise keep enforcing:

```sh
kubectl delete policies.kyverno.io -A -l telark.io/protection-plan
```

### Full teardown

To also delete all Telark data, run after `helm uninstall` (about 15 seconds):

```sh
kubectl delete crd -l app.kubernetes.io/part-of=telark
kubectl delete namespace telark
```

`helm uninstall` already cleared the cleanup finalizers on users, groups and roles; a reinstall that keeps the data restores them within a minute. If the uninstall ran with `--no-hooks` and deletions hang in `Terminating`, clear the finalizers by hand and they finish on their own:

```sh
kubectl get users.telark.io,groups.telark.io,accessroles.telark.io -n telark -o name |
  xargs -r -P 16 -I{} kubectl patch -n telark {} --type merge -p '{"metadata":{"finalizers":null}}'
```
