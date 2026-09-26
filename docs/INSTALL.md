# Install guide

## Prerequisites

- Kubernetes ≥ 1.30 (1.33+ recommended) — enforced by the chart's `kubeVersion`; see [Kubernetes compatibility](../README.md#kubernetes-compatibility).
- Helm ≥ 3.
- A StorageClass for the exporter's two PVCs (snapshots and protection plan reports). `standard` and `performance` run two exporter replicas sharing both volumes, so the class must be **ReadWriteMany** for both claims (`efs-sc` on EKS with the EFS CSI driver). A one-node cluster (`--set app.singleNode=true`) and `minimal` run one replica on any default class.

## 1. Install

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.persistence.storageClass=<rwx-class> \
  --set 'app.auth.bootstrap.admins={jane.doe@example.com}'
```

Replace `jane.doe@example.com` with your own admin email (see [First admin](#2-first-admin)): the chart ships none and refuses to render without one. The install also fails early if the class is missing in `standard`/`performance` — a ReadWriteMany claim against block storage never binds. On a one-node cluster pass `--set app.singleNode=true` instead of the class; the examples below omit the class and admin flags for brevity, keep yours on every command.

One command installs everything — CRDs, NATS config, and default `standard` sizing all ship in the chart. The CRDs are cluster-scoped and kept on uninstall (`resource-policy: keep`). Managing CRDs out of band (e.g. GitOps applies them first)? Add `--set crds.enabled=false`. From a checkout, `./charts/telark` works in place of the OCI ref.

### Sizing modes

`app.mode` sizes every telark service — replicas, resources, client rate limits and disruption budgets — from a single flag. The default is `standard`; pick another with `--set`:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance
```

| Mode | For | Capacity (measured 2026-09-17) |
|---|---|---|
| `minimal` | dev, demos, evaluation — single replica, no autoscaling, no PDBs | a few hundred applications |
| `standard` (default) | small–mid production — every service starts at 1 replica and scales on CPU up to 3 (HPA); the exporter runs 2 replicas sharing a ReadWriteMany snapshot volume; add `--set vpa.enabled=true` for vertical scaling | verified at 2 000 applications |
| `performance` | large clusters — same, HPA ceiling 5, disruption budgets keep one pod through drains; larger requests/limits and a 50 GiB volume | beyond 1 000 applications |

`app.mode` sizes telark's own services only — Helm resolves a subchart's values before the mode is known, so redis, NATS, the policy engine, metrics-server and the model runtime (ollama, see [Analyzer runtime](#analyzer-runtime)) ship fixed production-grade defaults owned by the chart, identical in every mode. Nothing to tune up to 2 000 applications; beyond that, size Redis (see [Subcharts](#subcharts)).

## 2. First admin

`app.auth.bootstrap.admins` is empty by default and passkey self-registration (`app.auth.passkey.selfRegistration`) is `"false"`, so the chart fails the render until you set at least one admin email: with neither, nobody could ever log in and the auth service refuses to start.

```sh
--set 'app.auth.bootstrap.admins={jane.doe@example.com,john.doe@example.com}'
```

Those emails receive the Admin role only from a verified identity: their first OIDC sign-in (the identity provider verified the email), or an enrolment you start yourself. Registering a passkey from the login page never grants Admin, and a bootstrap email cannot be registered there at all. To enrol the first admin with a passkey (no SSO yet), run the auth service's break-glass command and open the link it prints within 10 minutes:

```sh
kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email jane.doe@example.com --enroll
# open https://<dashboard-host>/register?enroll=<token>
```

`app.auth.passkey.selfRegistration="true"` lets anyone who reaches the dashboard create a ReadOnly account; leave it off unless only people you trust can reach the dashboard.

Bootstrap accounts belong to the chart: the API refuses to delete them, only they may edit their own record, and no dashboard user can create or rename a user to one of these emails (the exporter receives the same list as `BOOTSTRAP_ADMINS`). Only a bootstrap account may delete or suspend another administrator, and non-administrators never see administrator accounts at all.

## 3. Verify

```sh
kubectl get pods -n telark -l app.kubernetes.io/instance=telark
helm test telark -n telark      # readiness probe against the auth service
```

## Access the dashboard

The dashboard (`ui` service) is **deployed by default** behind a ClusterIP Service on port 8080 — reachable inside the cluster only. Expose it one of four ways. The chart bundles no ingress or gateway controller: like Argo CD, Grafana, Vault and Longhorn it ships ClusterIP plus the knobs, and uses whichever controller your cluster already runs.

Passkeys are bound to the host you open the dashboard on. By default the WebAuthn relying party follows the request host, so a passkey registered on `localhost:3000` is not accepted on the NodePort or Ingress hostname — register again there. For production, pin `app.auth.passkey.id=<domain>` and `app.auth.passkey.origin=https://<domain>` so the relying party stays fixed and never follows client-supplied `Host` or `Origin` headers. With `ingress.enabled` or `gateway.enabled` the chart refuses to render until both are set; pin them yourself for NodePort and LoadBalancer. A passkey is always created for the host the browser is open on, so to sign in on a second host open Settings → Security → Passkeys while signed in, choose "Add on another device", and open the one-time link it shows (valid for 10 minutes) on the other host to register a passkey there.

**HTTPS is required for passkeys.** Browsers only enable WebAuthn on secure origins — `https://` or `http://localhost` — so the port-forward tier works without TLS, but on a NodePort, LoadBalancer or Ingress host the dashboard shows a warning and passkey sign-in and registration stay disabled until the host serves a certificate the browser trusts. NodePort and LoadBalancer expose the plain-HTTP `ui` service, so terminate TLS in front of them; the Ingress tier can get a certificate from cert-manager (below).

### Port-forward (default, dev)

Maps local `3000` to the service's `8080`. Nothing to install:

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

Needs a cloud load balancer (EKS, GKE, AKS, …) or MetalLB on bare metal:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set services.ui.serviceType=LoadBalancer
kubectl get svc -n telark telark-ui-service   # EXTERNAL-IP
```

### Ingress

Needs an ingress controller. Without one, install ingress-nginx:

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

Without `ingress.tls` the Ingress serves plain HTTP, session tokens included, and the install notes print a warning; add TLS as below or terminate it in front of the controller.

| Flag | Default | Description |
|---|---|---|
| `ingress.enabled` | `false` | Create an Ingress for the dashboard |
| `ingress.className` | `""` | IngressClass (e.g. `nginx`) |
| `ingress.host` | `""` | Hostname (`""` = match any host) |
| `ingress.service` | `ui` | Which `services.<key>` to route to |
| `ingress.path` / `ingress.pathType` | `/` / `Prefix` | Route path + match type |
| `ingress.tls` | `[]` | TLS blocks, e.g. `[{secretName: telark-tls, hosts: [telark.example.com]}]` |
| `ingress.annotations` | `{}` | Controller annotations (cert-manager, etc.) |

**TLS with cert-manager** — install cert-manager with its CRDs, create a Let's Encrypt `ClusterIssuer` that solves HTTP-01 challenges through the nginx class, then point the chart's Ingress at it. The DNS name must already resolve to the ingress controller's load balancer (`kubectl get svc -n ingress-nginx ingress-nginx-controller` shows its `EXTERNAL-IP`) or the challenge cannot pass. A self-signed certificate is not enough: Chrome also disables WebAuthn on pages with certificate errors, so the certificate must be one the browser trusts. Replace `admin@example.com` with the address Let's Encrypt should notify about expiring certificates.

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

**Gateway API** — instead of an Ingress, the chart can render an `HTTPRoute` (`gateway.networking.k8s.io/v1`) attached to a Gateway you already run. Requirements: Kubernetes ≥ 1.30 (the chart's floor), Gateway API v1.0+ CRDs (`HTTPRoute` v1) and a controller that implements `HTTPRoute` v1 — Envoy Gateway, NGINX Gateway Fabric, Cilium or Istio; ingress-nginx does **not** implement Gateway API. Install the CRDs (standard channel) if the cluster has none:

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
| `gateway.hostnames` | `[]` | Hostnames the route matches (`[]` = any host) |
| `gateway.service` | `ui` | Which `services.<key>` to route to |
| `gateway.annotations` | `{}` | HTTPRoute annotations |

## Install-time flags

Everything is set on the one command line with `--set key=value`. Re-pass the same flags on `helm upgrade` — Helm does not remember them across upgrades.

### App

| Flag | Default | Description |
|---|---|---|
| `app.mode` | `standard` | Size every telark service: `minimal` \| `standard` \| `performance` (see [Sizing modes](#sizing-modes)) |
| `app.name` | `telark` | App identity / resource-name prefix (also the CRD group, `erpi.<name>`) |
| `app.namespace` | `telark` | Install namespace. Must match the release namespace (`-n`): the subcharts follow `-n`, so a mismatch splits redis/nats away from the services that address them by bare name |
| `app.image.registry` | `telark` | Registry / org hosting the service images |
| `app.image.pullPolicy` | `Always` | Image pull policy |
| `app.image.pullSecrets` | `[]` | Image pull secrets for a private registry |
| `app.persistence.size` | `10Gi` | Exporter snapshot PVC size |
| `app.persistence.reportsSize` | `2Gi` | Exporter reports PVC size (`minimal` 512Mi, `performance` 10Gi) |
| `app.persistence.storageClass` | `""` | Class for both exporter PVCs (snapshots and reports). Must be a ReadWriteMany class in `standard`/`performance` (two exporter replicas); `""` = cluster default, valid only with `app.singleNode=true` or `minimal` |
| `app.singleNode` | `false` | One-node cluster: the exporter runs 1 replica on ReadWriteOnce, no ReadWriteMany class needed. Access mode and update strategy follow the replica count automatically |
| `app.crdGuard.enabled` | `true` | Admission guard: only owning service accounts may write telark CRs; see [CRD write guard](#crd-write-guard) |
| `app.crdGuard.enforce` | `true` | With the guard on, `false` audits and `true` rejects |
| `app.auth.bootstrap.admins` | `[]` | Emails granted Admin on first sign-in; required while self-registration is off, see [First admin](#2-first-admin) |
| `app.auth.passkey.selfRegistration` | `"false"` | `"true"` lets anyone who reaches the dashboard register a passkey account |
| `app.auth.passkey.id` / `origin` | `""` | WebAuthn relying party; required with `ingress.enabled` or `gateway.enabled`, see [Access the dashboard](#access-the-dashboard) |
| `app.networkPolicy.enabled` | `true` | Ingress NetworkPolicies for the telark pods and NATS; see [Network policies](#network-policies) |
| `app.serviceToken.existingSecret` | `""` | Secret (key `token`) you manage instead of the generated service token; see [GitOps](#gitops-cluster-less-renders) |
| `services.<svc>.env.CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins the exporter, discovery, auth and analyzer answer with CORS headers; see [CORS](#cors) |

### Subcharts

Bundled dependencies ship production-grade defaults sized for every mode, so you rarely touch these. On/off toggles and the values telark pins — any other upstream key works the same way (`--set <subchart>.<path>`):

| Flag | Default | Description |
|---|---|---|
| `crds.enabled` | `true` | Install CRDs (the telark-crds subchart); `false` to manage them out of band |
| `app.kyverno.enabled` | `true` | Install the policy engine (kyverno) |
| `app.kyverno.failOpen` | `true` | Admission fails open while the policy engine is down; see [Policy engine fail-open](#policy-engine-fail-open) |
| `app.ollama.enabled` | `true` | Install the local model runtime (ollama) the analyzer needs; see [Analyzer runtime](#analyzer-runtime) |
| `app.ollama.autoPull` | `true` | Let the analyzer pull a missing model; `false` for air-gapped installs |
| `app.ollama.runtimeUrl` | `""` | Ollama-API endpoint you run yourself (URL only, no key); empty = the bundled runtime |
| `metrics-server.enabled` | `true` | Install metrics-server; `false` if the cluster already ships one |
| `redis.architecture` | `standalone` | `replication` for a replicated redis |
| `redis.master.persistence.size` | `4Gi` | Redis PVC size |
| `redis.master.resources.limits.memory` | `512Mi` | Redis memory limit (requests `100m` / `128Mi`); see the sizing note below |
| `nats.persistence.size` | `4Gi` | NATS JetStream PVC size |
| `nats.existingSecrets.publisher` / `consumer` | `""` | Secrets (keys `username`, `password`) you manage instead of the generated NATS users; see [GitOps](#gitops-cluster-less-renders) |
| `kyverno.admissionController.replicas` | `2` | Policy-engine admission replicas |
| `kyverno.admissionController.container.extraArgs.clientRateLimitQPS` | `50` | Policy-engine API QPS |
| `metrics-server.resources.limits.memory` | `400Mi` | metrics-server memory limit |
| `metrics-server.args` | kubelet TLS verified | Add `--kubelet-insecure-tls` only where kubelet certificates are self-signed; see [metrics-server kubelet TLS](#metrics-server-kubelet-tls) |
| `ollama.persistentVolume.size` | `10Gi` | Model storage (when enabled); kept on uninstall |

**Sizing Redis for large installs.** Redis keeps everything in memory and never evicts, so when it reaches its memory limit the pod is OOM-killed. The analyzer's documents are the largest part: in the worst case about 210 MiB for 2 000 applications, which the `512Mi` limit covers. For more applications, raise the limit in proportion, e.g. `--set redis.master.resources.limits.memory=1Gi`. The chart sets `redis.master.resources`, so `redis.master.resourcesPreset` has no effect.

The **complete** field list — every telark value and every pinned subchart value — is the auto-generated [`charts/telark/VALUES.md`](../charts/telark/VALUES.md); full upstream options live in each dependency's own chart (redis/nats = Bitnami, plus kyverno, metrics-server, ollama).

Example:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance \
  --set app.persistence.storageClass=efs \
  --set 'app.auth.bootstrap.admins={jane.doe@example.com}'
```

### CRD write guard

telark's custom resources (users, roles, sessions, passkeys) are its authorization data. The guard (`app.crdGuard`, a ValidatingAdmissionPolicy, on and enforcing by default) rejects writes to them from anything but the owning service accounts, so edit rights on the `telark` namespace do not turn into telark Admin. Break-glass identities go in `app.crdGuard.extraAllowedUsers`, for example `--set 'app.crdGuard.extraAllowedUsers={system:serviceaccount:ops:breakglass}'`. `--set app.crdGuard.enforce=false` only audits (logs and allows) and `--set app.crdGuard.enabled=false` removes the guard; the install notes warn when it is off.

### Network policies

With `app.networkPolicy.enabled=true` (default) the chart renders ingress NetworkPolicies in the release namespace: a default deny for every telark pod; the exporter, discovery, auth, analyzer and notifier APIs accept traffic only from telark pods of the same release; the dashboard (`ui`) accepts traffic on its port from anywhere, because the Ingress controller, Gateway or port-forward reaches it from outside the release; NATS accepts clients only from discovery and notifier on 4222, and serves no monitoring port. Redis and the model runtime keep their own policies. Egress is not restricted: the services talk to the Kubernetes API, the OIDC issuer and each other.

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

### GitOps (cluster-less renders)

The chart generates the service token and the two NATS users' passwords on install and reads them back on upgrade with `lookup`. Argo CD, Flux and any other `helm template` pipeline render without a cluster, so `lookup` returns nothing and every sync would produce new values: pods that restart on the new token can no longer reach their peers. Create the Secrets yourself and point the chart at them:

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

The generated Secrets carry `helm.sh/resource-policy: keep`, so an uninstall never drops them.

### Policy engine fail-open

`app.kyverno.failOpen=true` (default) makes every Kyverno webhook `failurePolicy: Ignore`: while the admission controller is down, rolling out or timing out, the API server admits every request, including ones an `enforce` protection plan would reject. Workloads stay deployable when Kyverno is unhealthy, at the cost of enforcement being best-effort. For strict enforcement set both keys (Helm cannot pass a parent value to a subchart, so the render fails when they differ):

```sh
--set app.kyverno.failOpen=false \
--set kyverno.features.forceFailurePolicyIgnore.enabled=false
```

Fail-closed means an unavailable Kyverno blocks writes to everything its webhooks match, so keep its two admission replicas and disruption budget.

### Last-modified annotations

A Kyverno ClusterPolicy stamps `telark.io/last-modified-{by,at,operation}` on the kinds discovery tracks (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, ConfigMaps, Services, PVCs, ServiceAccounts, Ingresses, NetworkPolicies, HPAs, VPAs), outside `telark`, `kyverno` and the system namespaces. Secrets are not stamped, so history does not name who changed a Secret. The policy renders only when the Kyverno `ClusterPolicy` API already exists, which on a first install it does not yet: run `helm upgrade` once after the first install, with the same flags, to add it. Changes made before that carry no author.

### metrics-server kubelet TLS

The bundled metrics-server verifies kubelet serving certificates. Where kubelets use self-signed certificates (kind, minikube, some bare-metal installs) it cannot scrape and HPAs stay at `<unknown>`; there, and only there, add the flag:

```sh
--set 'metrics-server.args={--kubelet-insecure-tls,--kubelet-preferred-address-types=InternalIP\,ExternalIP\,Hostname}'
```

### CORS

No API service sends CORS headers unless its `CORS_ALLOWED_ORIGINS` env var (`services.{exporter,discovery,auth,analyzer}.env`) lists the browser origins allowed to call it directly (comma-separated; `*` is ignored because credentials are allowed). The dashboard reaches every API through its own nginx proxy on the same origin, so installs leave them empty. For UI development against a port-forwarded cluster set `http://localhost:3000` on all four (`charts/telark/values.dev.yaml` does).

## Analyzer runtime

AI insights run on an in-cluster model runtime (ollama, installed by default), so analysis data stays in the cluster. A fresh install turns the analyzer on with `granite4:350m` (which answers in seconds on 2 vCPU) and automatic analysis off; change the model, turn on automatic analysis or turn the analyzer off in Settings. Upgrades keep the existing settings. The runtime has one size for every mode, because Helm resolves subchart values before `app.mode` applies: requests `250m` CPU and `1536Mi` memory, a 2-CPU limit and no memory limit, enough for `granite4:350m` while `minimal` still fits one 2 vCPU / 8 GiB node. Models live on a 10Gi volume that is kept on uninstall, so they survive restarts and reinstalls.

- **Connected** (default, `app.ollama.autoPull=true`): the analyzer pulls the chosen model right after start when the runtime lacks it (708 MB for `granite4:350m`), and the ollama pod gets HTTPS egress for it. Until the pull finishes, an analysis shows the rule text without the model's narration; **Install model** in Settings starts the pull right away.
- **Air-gapped** (`--set app.ollama.autoPull=false`): nothing is pulled and the ollama pod gets no HTTPS egress. Pre-load the model on a seeded volume or a baked image, as described in the chart README.
- **Your own runtime**: `--set app.ollama.enabled=false --set app.ollama.runtimeUrl=http://<host>:11434` points the analyzer at an Ollama-API endpoint you run (URL only, no key).

Larger profiles (CPU 4 vCPU, GPU / deep), model licences and the air-gapped procedure are in the chart README, [Analyzer runtime (ollama)](../charts/telark/README.md#analyzer-runtime-ollama).

**Recommendations:** the analyzer also reviews each app's setup (replicas, disruption budgets, resources, autoscaling, images, network policies, protection plans) and shows recommendation cards; a sweep re-reviews every app every 2 hours (`services.analyzer.env.ANALYZER_REVIEW_INTERVAL_SEC`, `0` disables it) at 20 apps/min (10 in `minimal`, 60 in `performance`). The reviews need read-only access to Services, PodDisruptionBudgets, HorizontalPodAutoscalers and NetworkPolicies, which the chart grants the analyzer ClusterRole (`get`, `list`; nothing else, no writes). Apps count as production when a namespace or a covering plan's environment matches `services.analyzer.env.ANALYZER_PRODUCTION_PATTERN` (default `(^|[-_.])(prod|production|prd)($|[-_.])`, case-insensitive); set it to your own naming, for example `--set-string 'services.analyzer.env.ANALYZER_PRODUCTION_PATTERN=^live-'`. Per-workload rules (single replica, missing disruption budget, image digest pinning) judge each workload by its own namespace, so the dev namespace of a multi-namespace app is not held to production rules. Details in the chart README, [Recommendations](../charts/telark/README.md#recommendations); the dashboard lists every app's cards on its Insights page, see [Insights page](../charts/telark/README.md#insights-page).

## Autoscaling (HPA)

The stateless services — auth, discovery, notifier, ui — can run behind a HorizontalPodAutoscaler (`autoscaling/v2`, CPU-based). The exporter never autoscales — its replica count is fixed by the mode (2 in `standard`/`performance`, 1 in `minimal` or with `app.singleNode=true`). Nor does the analyzer: one worker bound to one runtime slot. HPAs need metrics-server, which ships with the chart.

**`standard` and `performance` turn autoscaling on** (start at 1, max 3 and 5); `minimal` keeps it off. In any mode you can enable, disable or tune it per service:

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

**Upgrading to the chart with the security defaults:** `app.auth.bootstrap.admins` no longer defaults to a vendor address and self-registration is off, so pass your admin email (the render fails without one). The CRD write guard now enforces, NetworkPolicies restrict ingress to the telark APIs and NATS, and NATS moves from one shared user to a publisher (discovery) and a consumer (notifier) with new Secrets, so the NATS server and both clients restart during the rollout. With an Ingress or Gateway, pin `app.auth.passkey.id` and `origin` too.

**From chart 0.2.1 or older, or when switching modes:** those releases run one exporter replica on a ReadWriteOnce claim, and Kubernetes cannot change a bound claim's access mode or class. Add `--set app.singleNode=true` to keep that claim (one replica, Recreate). To move to two replicas on ReadWriteMany, uninstall, delete the `telark-exporter-snapshots-pvc` claim (snapshots are lost — copy `/snapshots` off the pod first if you need them), then reinstall with `--set app.persistence.storageClass=<rwx-class>`. The same applies when switching between `minimal` and `standard`/`performance`, or toggling `app.singleNode`.

**Upgrading to the chart that adds protection plan reports:** the exporter gains a second claim, `telark-exporter-reports-pvc`, which binds on rollout with the same class and access mode as the snapshot claim. Do not upgrade with `--reuse-values`: the reports volume, mount and the `REPORTS_PATH` / `PROTECTION_PLAN_REPORT_*` entries arrive only with the new chart defaults; with `--reuse-values` the exporter logs a reports-root error at start and every report write fails. Note that the exporter volumes render even when `app.persistence.enabled=false` (pre-existing behaviour), so the pods then wait on claims nobody provisions.

## Uninstall

```sh
helm uninstall telark -n telark
```

This removes every telark service **and the exporter's snapshot and report PVCs** — back them up first if you need them. CRDs and custom resources are **not** removed (they carry `helm.sh/resource-policy: keep`), nor are the service-token and NATS Secrets, the redis/NATS volumes or the analyzer model volume (ollama), so a reinstall picks up where you left off.

Before uninstalling, cancel active protection plans (so their admission policies are removed) and let in-progress rollbacks finish.

### Full teardown

Run after `helm uninstall`, **in this order**. Deleting the CRDs or namespace first hangs in `Terminating`: users, groups and roles carry `telark.io/*-cleanup` finalizers that only the (now removed) auth service clears. Those finalizers only tidy references between telark resources, which this teardown deletes anyway, so clearing them is safe.

```sh
# 1. Clear the cleanup finalizers
for crd in $(kubectl get crd -l app.kubernetes.io/part-of=telark -o name | cut -d/ -f2); do
  kubectl get "$crd" -A -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name}{"\n"}{end}' |
    while read -r ns name; do
      kubectl patch "$crd" "$name" -n "$ns" --type merge -p '{"metadata":{"finalizers":null}}'
    done
done

# 2. Delete the CRDs — this deletes every telark custom resource
kubectl delete crd -l app.kubernetes.io/part-of=telark

# 3. Only with an external policy engine (app.kyverno.enabled=false): leftover plan policies
kubectl delete policies.kyverno.io -A -l telark.erpi/protection-plan

# 4. Remaining volumes and the kept Secrets
kubectl delete namespace telark
```

Already stuck in `Terminating`? Run step 1; the pending deletions complete on their own.
