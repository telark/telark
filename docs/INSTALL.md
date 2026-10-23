# Install and configure

This guide installs Telark with the defaults, then covers exposure, sizing, the bundled subcharts and the settings most installs never change. For a guided first run, see [Getting started](getting-started.md). Every value is listed in [`charts/telark/VALUES.md`](../charts/telark/VALUES.md).

## Prerequisites

- Kubernetes 1.30 or newer; 1.33+ is the tested target. The chart's `kubeVersion` enforces the floor ([compatibility](../README.md#compatibility)).
- Helm 3.
- A default StorageClass (managed clusters, kind, minikube and k3d have one) for the exporter's two claims ([Exporter storage](#exporter-storage)).

## 1. Install

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com
```

- Replace `test@example.com` with your own email. The chart ships no admin and requires one ([First admin](#2-first-admin)).
- By default the chart installs everything, CRDs included. The CRDs are cluster-scoped and kept on uninstall (`resource-policy: keep`). If you manage CRDs out of band (a GitOps tool applies them first), add `--set crds.enabled=false`.
- From a checkout, `./charts/telark` works in place of the OCI reference.

**Already running Kyverno?** Install with the bundled Kyverno disabled and Telark uses yours:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com \
  --set kyverno.enabled=false
```

Telark is tested with Kyverno v1.19.1, the version the chart bundles. Your own Kyverno must be 1.18 or later, serve the `policies.kyverno.io` CRDs (plan policies are `NamespacedValidatingPolicy` objects; without them every active plan reads Degraded and discovery logs why) and keep its `PolicyViolation` Events, as its default `omitEvents` does: the plan's admission decisions come from them. Upgrade Telark to 0.2.0 before you move Kyverno to 1.20 ([Upgrade notes](#upgrade-notes)). With your own Kyverno, change history names no author, because the chart installs the policy that records authors only with the bundled Kyverno.

The examples below leave out the admin flag. Keep it on every command.

## 2. First admin

Telark has exactly one bootstrap admin: the email in `app.auth.bootstrap.admin`. Only the bootstrap admin can turn on SSO or passkey self-registration, so the value is required: without it the chart fails the render and the auth service refuses to start.

The bootstrap admin holds the built-in Admin role (Admin on `ALL`) and nothing more. Only the auth service's break-glass command creates or recovers it. It prints an [enrollment link](#enrollment-links) for that account, in the form the Members dialog builds, and one step: open it in a browser that is not signed in to Telark, such as a private window. Register a passkey with it within 10 minutes: a break-glass link always lives 10 minutes, whatever the enrollment link lifetime is set to.

```sh
kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email test@example.com --enroll
# prints https://<dashboard-host>/register#enroll=<token>&email=test%40example.com and one step
```

The link starts with `app.auth.passkey.origin`. When that is empty, as with the default port-forward, it starts with `<dashboard-url>`: replace that with the address you open the dashboard at, `http://localhost:3000` with the port-forward. The token is the link's `enroll=` value, after the `#`, so the browser never sends it to a server or an access log, and the registration page has no field for it. Whoever opens the link first gets the passkey: run the command from your own terminal, not from CI or a recorded session, and open the link only in your own browser. Until the account's first passkey is stored, running the command again prints a new token, which replaces the previous one, and an existing account with that email becomes the bootstrap admin. That first passkey pins the account: after it, break-glass opens a request instead of acting ([Recovery requests](#recovery-requests)). Any other email is refused without creating or changing an account; other users get an [enrollment link](#enrollment-links).

A second passkey for the bootstrap admin is recommended, not required: kept on another device or in a password manager, it signs you in when one device is lost, without break-glass. Once signed in, open Settings → Security → Passkeys and choose "Add on another device". Passkeys are bound to the relying-party ID (`app.auth.passkey.id`, the auth service's `RP_ID`), so changing it means registering every passkey again, the bootstrap admin's through break-glass ([Passkeys and HTTPS](#passkeys-and-https)).

Google SSO and passkey self-registration never grant Admin: a new account always starts as ReadOnly, even with the bootstrap email, and the login page refuses to register the bootstrap email.

Next, give at least two regular users the Admin role (directly or through a group), so the platform never depends on one person. Create their accounts on the Members page and send each an [enrollment link](#enrollment-links), or let them sign up through SSO or self-registration, then grant them Admin there. For Google SSO, register `https://<dashboard-host>/auth/google/callback` as an authorized redirect URI of your Google OAuth client ([Login and SSO](../services/auth/OIDC.md)). Any Admin on `ALL` can suspend, demote or delete a regular admin, but the API refuses a change that would leave no active Admin (409), and nobody can delete their own account.

Self-registration is a Settings toggle next to Single Sign-On, off by default; everyone but the bootstrap admin, Admins on `ALL` included, sees both read-only. With self-registration on, anyone who reaches the dashboard can create a ReadOnly account with a passkey, so leave it off unless only people you trust can reach the dashboard. It is a `TelarkConfig` setting, not a chart value, and applies within a few seconds without a restart.

The bootstrap account belongs to the chart. The API refuses to delete or suspend it, only it may edit its own record, and no dashboard user can give its email to another account or change it (the exporter receives the same email as `BOOTSTRAP_ADMIN`). Non-administrators never see administrator accounts. Administrators see the bootstrap account only as "Bootstrap account", with no username, full name or email, in member pickers, group lists, history, notifications and Settings → Authentication, and the Members page doesn't list it. Only the account itself sees its own details. Whether Admins on `ALL` hear of break-glass requests is its choice too ([Recovery requests](#recovery-requests)).

### Recovery requests

Once the bootstrap account is pinned (`TelarkConfig` `status.bootstrap`), only a completed request moves the bootstrap role, and changing `app.auth.bootstrap.admin` makes no second bootstrap account:

- `break-glass --enroll` opens a request, or prints the open one: its id, the request's page (`https://<dashboard-host>/break-glass#request=<id>&kind=<recover|replace>`, where `kind` only picks what the page shows first) and when it completes on its own. The bootstrap account is notified, and so is every Admin on `ALL` while break-glass alerts are on. The request stays in `TelarkConfig`, which no account can clear, and Settings → Authentication lists it.
- A recovery, for the pinned account's own email, can't be approved: it completes on its own 72 hours after it opened, unless the bootstrap account signs in first or denies it on that page, signed in or not. A replacement completes when the bootstrap account approves it on that page with a passkey, or after the same 72 hours. A sign-in by the bootstrap account cancels either, so whoever ran break-glass can't wait out an owner who is around.
- Run the command again after the 72 hours, or a replacement's approval, to complete the request. One run works at a time: another `--enroll` run stops at once and says so.
- A recovery goes link first, wipe after. The run freezes the account, removes its sessions, enrollment links and open sign-ins and registrations, waits about 45 seconds for a request in flight when it began, removes any passkey a stopped run's link saved unless the account has no other, and prints a 10-minute link. The account keeps its passkeys. The run then waits for a passkey registered with that link, up to about 12 minutes, with a line every 30 seconds, so keep it running. Meanwhile the account can't sign in or deny the request, and only the link's registration gets through. Once that passkey is saved, the run removes every session, passkey sign-in and other passkey of the account, the passkeys last, so a run that fails before them leaves them all. It then completes the request and thaws the account. If the link expires unused, the account keeps its passkeys, the request stays open, and the run exits 1: run it again for a new link. If the request ends while the run waits (a sign-in that began before the run cancels it), the run stops waiting, removes the passkey the link registered if one did, and exits 1. Either way it first waits about 45 seconds more, so a registration still in flight counts.
- A run that stops while it waits leaves the account frozen until the link's window ends. A passkey the link saved can't sign in while the request is open, and the next run removes it before it prints a new link. A sign-in, a deny or the expiry removes it too, unless the account has no other passkey, before the request's end is recorded: if that fails, the request stays open, and the next cancel or expiry tries again.
- A replacement, after `app.auth.bootstrap.admin` changed, creates or promotes the account with the new email and, once completed, moves the bootstrap role to it. The previous account keeps its roles, so the new bootstrap admin can demote it, and neither account loses a passkey. For 24 hours after it completed, until the new account has a passkey, each run prints a new token without a new request, and each one is notified.
- Without `--enroll`, the command prints the pinned account and the newest request, and changes nothing.

Break-glass alerts are a switch in Settings → Authentication, on by default, that only the bootstrap account sees and can change (`PATCH auth/break-glass-alerts` with `{"notifyAdmins": false}`; 403 for everyone else, Admins on `ALL` included). Admins on `ALL` see only a Break-glass requests card there, without the pinned account, the switch or the Approve and Deny buttons, and only while the switch is on. Turned off, the opened and canceled notices go to the bootstrap account alone, and so do the requests. A link break-glass is issuing and a completed request still reach every active Admin on `ALL`, since either can mean the account changed hands. Turned back on, they see the requests again, with no notices for what happened meanwhile. It is a `TelarkConfig` setting (`spec.breakGlassAlerts.notifyAdmins`), not a chart value.

Someone else holding a passkey of the bootstrap account cancels each request with every sign-in, so the owner can't recover through Telark. A cluster admin then resets the pin, an audited write to the CRs. Add your identity to `app.security.crdGuard.extraAllowedUsers` first ([CRD write guard](#crd-write-guard)), and take it off afterwards:

```sh
ID=$(kubectl get tcfg default -n telark -o jsonpath='{.status.bootstrap.userID}')
for kind in passkeys sessions; do
  kubectl delete "$kind" -n telark $(kubectl get "$kind" -n telark -o jsonpath="{.items[?(@.spec.userId=='$ID')].metadata.name}")
done
# Removes the pin and keeps the requests; the next break-glass --enroll acts at once, and the new passkey pins the account again and ends a request left open.
kubectl patch tcfg default -n telark --subresource=status --type=merge -p '{"status":{"bootstrap":{"pinnedAt":null}}}'
```

### Enrollment links

An enrollment link lets someone register a passkey on an account an admin created for them (Members, row action "Create enrollment link"). The dashboard shows the link once. It registers one passkey, for that account only, fills in the account's email on the register page, and expires after an hour by default. A canceled or failed passkey prompt doesn't use it up: the link stays valid until a passkey is saved with it or it expires. A new link revokes the previous one, and the same row can revoke it. Members shows the account as "Invite pending", "Invite expired" or, once a passkey closed the link, "Enrolled". Break-glass links always live 10 minutes. Change the lifetime of the others, in seconds, with:

```sh
--set services.auth.env.ENROLL_INVITE_TTL_SEC=86400
```

Who may create one: a user with Owner on users, for an account whose permissions, direct and through its groups, are all within their own on every scope. Nobody creates one for themselves (use "Add on another device"), for the bootstrap admin (break-glass only) or for a suspended or deleted account. A link for an account that already has a passkey is an account recovery: only the bootstrap admin or an Admin on `ALL` can create it, and the account owner is notified when it is created and when it is used.

## 3. Verify

```sh
kubectl get pods -n telark -l app.kubernetes.io/instance=telark
```

## Access the dashboard

The dashboard (the `ui` service) runs behind a ClusterIP Service on port 8080, reachable only inside the cluster. The chart bundles no ingress or gateway controller; it uses the one your cluster already runs.

| Method | Use it for | Needs |
|---|---|---|
| [Port-forward](#port-forward) (default) | Trying Telark, development | Nothing |
| [NodePort](#nodeport) | Lab clusters | Firewall access to a node port, TLS in front |
| [LoadBalancer](#loadbalancer) | Cloud clusters without an ingress controller | A cloud load balancer or MetalLB, TLS in front |
| [Ingress](#ingress) | Production | An ingress controller |
| [Gateway API](#gateway-api) | Production on Gateway API | A Gateway and a controller that implements `HTTPRoute` v1 |

### Passkeys and HTTPS

- **Passkeys need HTTPS.** Browsers enable WebAuthn only on `https://` or `http://localhost`, so port-forward works without TLS. On a NodePort, LoadBalancer or Ingress host, the dashboard shows a warning and passkey sign-in and registration stay disabled until the host serves a certificate the browser trusts. NodePort and LoadBalancer expose the plain-HTTP `ui` service, so terminate TLS in front of them; the Ingress can get a certificate from [cert-manager](#tls-with-cert-manager).
- **Passkeys are bound to a host.** By default the WebAuthn relying party follows the request host, so a passkey registered on `localhost:3000` is not accepted on the NodePort or Ingress hostname; register again there.
- **Pin the relying party in production.** Set `app.auth.passkey.id=<domain>` and `app.auth.passkey.origin=https://<domain>` so it never follows client-supplied `Host` or `Origin` headers. With `ingress.enabled` or `gateway.enabled` the chart refuses to render until both are set; pin them yourself for NodePort and LoadBalancer.
- **The session is a `Secure` cookie.** Sign-in sets `telark_session` (HttpOnly, Secure, SameSite=Strict), so the dashboard needs `https://`, or `http://localhost` in Chrome, Edge or Firefox. When every `app.auth.passkey.origin` entry is `https://`, the cookie is `__Host-telark_session` instead, which the browser takes only from the dashboard's own host, so a sibling subdomain can't plant one. Changing the origins between http and https later signs everyone out once. Safari drops `Secure` cookies on `http://localhost`: there, use one of those browsers or https. Open a port-forwarded dashboard at `localhost`, not `127.0.0.1`. A cookie isn't scoped to a port, so every server on `localhost`, on any port, receives it while you are signed in through a port-forward.
- **Signing in on a second host.** While signed in, open Settings → Security → Passkeys, choose "Add on another device", and open the one-time link it shows (valid for 10 minutes) on the other host.

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

The notification bell and the insights pages hold server-sent event streams (`text/event-stream`) open. A controller or load balancer in front must not buffer them and must allow more than 15 s of idle time, since the services ping every 15 s. ingress-nginx honors the `X-Accel-Buffering: no` header the services send, and its default 60 s read timeout is enough. Behind a proxy that buffers, the bell falls back to polling every 30 s.

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

| Mode | For |
|---|---|
| `minimal` | Development, demos, evaluation. Single replica, no autoscaling, no PDBs |
| `standard` (default) | Small and mid-size production. Every service starts at 1 replica and scales on CPU up to 3 (HPA), except the exporter ([Exporter storage](#exporter-storage)). Add `--set vpa.enabled=true` for vertical scaling |
| `performance` | Large clusters. As `standard`, with an HPA ceiling of 5, disruption budgets that let a drain take one pod at a time, larger requests and limits, and 2 GiB exporter volumes |

The mode sizes Telark's own services only. Helm resolves subchart values before the mode is known, so Redis, NATS, Kyverno, metrics-server and Ollama have the same defaults in every mode. On large installs, size Redis ([Subcharts](#subcharts)).

Ephemeral storage is the same in every mode. Each Telark service container requests `50Mi` for its logs (the kubelet keeps five 10Mi files by default) and may use up to `2Gi`, which leaves room for the disk-backed emptyDir volumes: the dashboard's nginx temp files and the exporter's data [without persistence](#exporter-storage). The kubelet evicts a pod that goes above its limit. To raise it in any mode, pass `--set app.shared.resources.limits.ephemeral-storage=4Gi`.

## Exporter storage

The exporter keeps snapshots and protection plan reports on two claims, `telark-exporter-snapshots-pvc` and `telark-exporter-reports-pvc`. By default it runs one replica on ReadWriteOnce claims from the cluster's default StorageClass, so an install needs no storage flag.

A second exporter replica keeps it available through node failures and rolling upgrades. Both replicas share the claims, so they need a ReadWriteMany class (`efs-sc` on EKS with the EFS CSI driver):

```sh
--set services.exporter.replicas=2 \
--set app.persistence.storageClass=efs-sc
```

More than one replica without a class fails the render, and so does a class the cluster doesn't have. `minimal` always runs one replica. Break-glass serializes its [recovery requests](#recovery-requests) inside one exporter, so scale back to one replica before a recovery: with two, a rare race can leave the bootstrap account with no passkey until the next break-glass run.

Kubernetes never changes a claim's class or access mode, so upgrades keep the claims as they are:

- An empty `app.persistence.storageClass` keeps the class the claims already use, so forgetting the flag on an upgrade changes nothing.
- Sizes (`app.persistence.snapshotsSize`, `app.persistence.reportsSize`, also set by the mode) only grow, and only when the class allows volume expansion; a smaller value keeps the current size.

**Changing the class.** Name the new class (and the replica count) on `helm upgrade`. The chart creates new claims named after it (`telark-exporter-snapshots-pvc-efs-sc-rwx`), and the exporter copies the snapshots and reports into them before it starts again, so it restarts once. The upgrade's notes list the old claims, which are kept; delete them after the next upgrade:

```sh
kubectl delete pvc -n telark telark-exporter-snapshots-pvc telark-exporter-reports-pvc
```

- `helm rollback telark -n telark` returns the exporter to the old claims as they were at the change, for example when the new claims stay `Pending` (a class that cannot serve the access mode, such as ReadWriteMany on block storage). A later upgrade back to the new class copies over what the exporter wrote in between; for each file, the newer copy wins.
- Uninstall keeps the claims, and a reinstall picks up the ones named after its storage settings, else the original ones (copied like an upgrade when the class differs). After a class change, reinstall with the same settings, or the original claims come back as they were at the change.
- Cluster-less renders ([GitOps](#gitops-cluster-less-renders)) cannot read the live claims, so they always render the original names and the requested class: keep the class you installed with.

**Without persistence.** `--set app.persistence.enabled=false` renders no claims, and the exporter keeps snapshots and reports in emptyDir volumes that are lost whenever its pod goes and that count against its `2Gi` ephemeral-storage limit ([Sizing modes](#sizing-modes)). A claim that already exists (kept by an earlier install, or created by hand) stays mounted instead. Cluster-less renders cannot see it, so they always render the emptyDir.

## Install-time flags

Set values with `--set key=value`, and pass the same flags on every [upgrade](#upgrade).

### App

| Flag | Default | Description |
|---|---|---|
| `app.mode` | `standard` | Size every Telark service: `minimal` \| `standard` \| `performance` ([Sizing modes](#sizing-modes)) |
| `app.name` | `telark` | Resource-name prefix. The CRD group is always `telark.io` ([ADR 0003](adr/0003-constant-api-group-telark-io.md)) |
| `app.namespace` | `telark` | Install namespace. Must match the release namespace (`-n`): the subcharts follow `-n`, so a mismatch splits Redis and NATS away from the services that address them by bare name |
| `app.image.registry` | `ghcr.io/telark` | Registry and organization hosting the service images |
| `app.image.pullPolicy` | `Always` | Image pull policy |
| `app.image.pullSecrets` | `[]` | Image pull secrets for a private registry |
| `app.persistence.snapshotsSize` | `512Mi` | Exporter snapshots PVC size (`performance` 2Gi); only grows |
| `app.persistence.reportsSize` | `512Mi` | Exporter reports PVC size (`performance` 2Gi); only grows |
| `app.persistence.storageClass` | `""` | Class for both exporter PVCs ([Exporter storage](#exporter-storage)) |
| `services.exporter.replicas` | `1` | More than one needs a ReadWriteMany `app.persistence.storageClass` |
| `app.security.crdGuard.enabled` | `true` | Admission guard: only the owning service accounts may write Telark CRs ([CRD write guard](#crd-write-guard)) |
| `app.security.crdGuard.enforce` | `true` | With the guard on, `false` audits and `true` rejects |
| `app.auth.bootstrap.admin` | `""` | The one bootstrap admin's email, enrolled with break-glass; required. Once that account is pinned, a change only lets break-glass open a replacement request ([First admin](#2-first-admin)) |
| `app.auth.passkey.id` / `origin` | `""` | WebAuthn relying party; required with `ingress.enabled` or `gateway.enabled` ([Passkeys and HTTPS](#passkeys-and-https)) |
| `app.selfMonitoring.enabled` | `false` | `true` shows Telark's own namespace under Applications ([Self-monitoring](#self-monitoring)) |
| `app.security.networkPolicy.enabled` | `true` | Ingress NetworkPolicies for the Telark pods and NATS ([Network policies](#network-policies)) |
| `app.security.serviceToken.existingSecret` | `""` | Secret (key `token`) you manage instead of the generated service token ([GitOps](#gitops-cluster-less-renders)) |
| `app.webhooks.targets` / `existingSecret` | `[]` / `""` | Endpoints that receive signed events, or a Secret you manage holding them ([Webhooks](#webhooks)) |
| `services.<svc>.env.CORS_ALLOWED_ORIGINS` | `""` | Comma-separated browser origins the exporter, discovery, auth and analyzer answer with CORS headers ([CORS](#cors)) |
| `services.auth.env.ENROLL_INVITE_TTL_SEC` | `"3600"` | Lifetime in seconds of an enrollment link created from Members; break-glass links always live 10 minutes ([Enrollment links](#enrollment-links)) |

### Subcharts

You rarely touch these. The table lists the on/off switches and the values the chart pins; any other upstream key works the same way (`--set <subchart>.<path>`), with the full options in each dependency's own chart (Bitnami for Redis and NATS, plus Kyverno, metrics-server and Ollama).

| Flag | Default | Description |
|---|---|---|
| `crds.enabled` | `true` | Install the CRDs (the telark-crds subchart); `false` to manage them out of band |
| `kyverno.enabled` | `true` | Install Kyverno, the policy engine; `false` uses a Kyverno you already run ([Install](#1-install)) |
| `kyverno.features.forceFailurePolicyIgnore.enabled` | `true` | Admission fails open while Kyverno is down ([Policy engine fail-open](#policy-engine-fail-open)) |
| `ollama.enabled` | `true` | Install Ollama, the local model runtime the analyzer needs ([Analyzer runtime](#analyzer-runtime)) |
| `app.ollama.autoPull` | `true` | Let the analyzer pull a missing model; `false` for air-gapped installs |
| `app.ollama.runtimeUrl` | `""` | Ollama-API endpoint you run yourself (URL only, no key); empty uses the bundled runtime |
| `metrics-server.enabled` | `true` | Install metrics-server; `false` if the cluster already has one |
| `redis.architecture` | `standalone` | `replication` for a replicated Redis |
| `redis.image.digest` | pinned | Bitnami publishes only `bitnami/redis:latest`, so the chart pins one build by digest. For a newer build, resolve its digest (`docker buildx imagetools inspect bitnami/redis:latest`) and pass `--set redis.image.digest=sha256:<digest>` |
| `redis.master.persistence.size` | `2Gi` | Redis PVC size; fixed once installed ([Upgrade](#upgrade)) |
| `redis.master.resources.limits.memory` | `512Mi` | Redis memory limit (requests `100m` / `128Mi`); see the note below |
| `redis.auth.existingSecret` | `<release>-redis-secret` | Secret with the Redis password (key `redis-password`), generated on install and read back on upgrade; name your own for cluster-less renders ([GitOps](#gitops-cluster-less-renders)). Redis always requires the password: `redis.auth.enabled=false` or `redis.auth.password` fails the render |
| `nats.persistence.size` | `1Gi` | NATS JetStream PVC size; fixed once installed ([Upgrade](#upgrade)) |
| `nats.existingSecrets.publisher` / `consumer` | `""` | Secrets (keys `username`, `password`) you manage instead of the generated NATS users ([GitOps](#gitops-cluster-less-renders)) |
| `kyverno.admissionController.replicas` | `2` | Kyverno admission replicas |
| `kyverno.admissionController.container.extraArgs.clientRateLimitQPS` | `50` | Kyverno API QPS |
| `metrics-server.resources.limits.memory` | `400Mi` | metrics-server memory limit |
| `metrics-server.args` | kubelet TLS verified | Add `--kubelet-insecure-tls` only where kubelet certificates are self-signed ([metrics-server kubelet TLS](#metrics-server-kubelet-tls)) |
| `ollama.persistentVolume.size` | `6Gi` | Model storage (when enabled), kept on uninstall: the default model plus any one catalog model, since a model switch deletes the other catalog models. Never shrinks ([Upgrade](#upgrade)) |

**Sizing Redis for large installs.** Redis keeps everything in memory and never evicts, so at its memory limit the pod is OOM-killed. The analyzer's documents are the largest part and grow with the number of applications, so raise the limit in proportion, for example `--set redis.master.resources.limits.memory=1Gi`. The chart sets `redis.master.resources`, so `redis.master.resourcesPreset` has no effect.

## Analyzer runtime

Insights runs on an in-cluster model runtime (Ollama, installed by default), so analysis data stays in the cluster. A fresh install turns the analyzer on with `granite4:350m`, which answers in seconds on 2 vCPU, and leaves automatic analysis off. Change either, or turn the analyzer off, in Settings; upgrades keep the existing settings.

The runtime has the same size in every mode ([Sizing modes](#sizing-modes)): requests `250m` CPU and `1536Mi` memory, a 2-CPU limit and no memory limit. That fits `granite4:350m`, and `minimal` still fits one 2 vCPU / 8 GiB node. Models live on a 6Gi volume kept on uninstall, so they survive restarts and reinstalls.

| Setup | Flags | Behavior |
|---|---|---|
| Connected (default) | `app.ollama.autoPull=true` | The analyzer pulls the chosen model right after start when the runtime lacks it (708 MB for `granite4:350m`), and the Ollama pod gets HTTPS egress for it. Until the pull finishes, an analysis shows the rule text without the model's rewording; **Install model** in Settings starts the pull right away |
| Air-gapped | `app.ollama.autoPull=false` | Nothing is pulled and the Ollama pod gets no HTTPS egress. Pre-load the model on a seeded volume or a baked image |
| Your own runtime | `ollama.enabled=false`, `app.ollama.runtimeUrl=http://<host>:11434` | The analyzer uses an Ollama-API endpoint you run (URL only, no key) |

Larger profiles (4 vCPU CPU, GPU and deep mode), model licenses and the air-gapped procedure are in the chart README, [Analyzer runtime (ollama)](../charts/telark/README.md#analyzer-runtime-ollama).

**Recommendations.** The analyzer also reviews each app's setup and shows [recommendation cards](concepts.md#setup-review) on the [Insights page](../charts/telark/README.md#insights-page) ([details](../charts/telark/README.md#recommendations)).

- A sweep re-reviews every app every 2 hours (`services.analyzer.env.ANALYZER_REVIEW_INTERVAL_SEC`, `0` disables it) at 20 apps per minute (10 in `minimal`, 60 in `performance`).
- The chart grants the analyzer ClusterRole the read-only access the reviews need (`get`, `list` on Services, PodDisruptionBudgets, HorizontalPodAutoscalers and NetworkPolicies).
- Apps count as production when a namespace or a covering plan's environment matches `services.analyzer.env.ANALYZER_PRODUCTION_PATTERN` (default `(^|[-_.])(prod|production|prd)($|[-_.])`, case-insensitive), for example `--set-string 'services.analyzer.env.ANALYZER_PRODUCTION_PATTERN=^live-'`.
- Per-workload rules (single replica, missing disruption budget, image digest pinning) judge each workload by its own namespace, so the dev namespace of a multi-namespace app is not held to production rules.

## Autoscaling (HPA)

The stateless services (auth, discovery, notifier, ui) can run behind a HorizontalPodAutoscaler (`autoscaling/v2`, CPU-based). HPAs need metrics-server, which ships with the chart.

- The exporter ([Exporter storage](#exporter-storage)) and the analyzer (one worker bound to one runtime slot) never autoscale.
- `standard` and `performance` turn autoscaling on (start at 1, maximum 3 and 5); `minimal` keeps it off.
- With `vpa.enabled=true`, the VPA sizes only memory on the services the HPA scales, because the HPA measures CPU against the request; on the exporter and the analyzer it sizes both. Turning it on with an upgrade needs its CRDs first ([Upgrade](#upgrade)).

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
| `services.<svc>.autoscaling.enabled` | mode | Turn the HPA on or off for that service |
| `services.<svc>.autoscaling.minReplicas` | `1` | Replica floor |
| `services.<svc>.autoscaling.maxReplicas` | `3` | Replica ceiling |
| `services.<svc>.autoscaling.targetCPUUtilizationPercentage` | `80` | Scale-up CPU target |
| `services.<svc>.autoscaling.targetMemoryUtilizationPercentage` | _(unset)_ | Optional memory target |

Set the same keys under `app.serviceDefaults.autoscaling` to change the default for every service at once. When a service autoscales, the chart omits `spec.replicas` so Helm and the HPA don't conflict.

## Self-monitoring

By default, discovery skips the release namespace, so Telark's own components (the Telark services, Redis, NATS, Ollama, Kyverno and metrics-server) never show under Applications. To discover and show them like any application:

```sh
--set app.selfMonitoring.enabled=true
```

- The value maps to discovery's `SELF_MONITORING_ENABLED` env var.
- While it is off, auto-cleanup removes Application CRs already created in that namespace (for example while it was on). That needs `DISCOVERY_AUTO_CLEANUP_ENABLED` and `DISCOVERY_AUTO_CLEANUP_DELETE_ENABLED` (both `"true"` in the chart by default) and takes about two cleanup cycles.
- Protection plans can never target the release namespace, whatever the value.

## Monitoring (Prometheus)

No Telark service serves a Prometheus `/metrics` endpoint, so keep the ServiceMonitor off: it would scrape nothing. The chart can create one (Prometheus Operator) if the Prometheus Operator CRDs are already in the cluster (for example from kube-prometheus-stack):

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set monitoring.serviceMonitor.enabled=true \
  --set monitoring.serviceMonitor.labels.release=kube-prometheus-stack
```

Prometheus only picks up a ServiceMonitor whose labels match its `serviceMonitorSelector`, which is `release: <your-release>` for kube-prometheus-stack. Set `monitoring.serviceMonitor.labels.release` to your Prometheus release name, or Prometheus ignores the monitor without any error.

| Key | Default | Description |
|---|---|---|
| `monitoring.serviceMonitor.enabled` | `false` | Create the ServiceMonitor |
| `monitoring.serviceMonitor.labels` | `{}` | Labels matching Prometheus's `serviceMonitorSelector` (usually `release: <name>`) |
| `monitoring.serviceMonitor.path` | `/metrics` | Scrape path |
| `monitoring.serviceMonitor.interval` | `30s` | Scrape interval |

The monitor selects every Telark service (`app.kubernetes.io/part-of: telark`) on the `http` port. With [network policies](#network-policies) on, a Prometheus outside the namespace also needs an allow rule.

## Webhooks

The notifier sends events to endpoints you name, signed the [Standard Webhooks](https://www.standardwebhooks.com/) way: `rollback.completed`, `plan.approval.requested`, `plan.approval.decided` and `incident.critical` (a new Critical incident). Each event goes once to every target that lists its type, however many users see it in the bell. Role, group and enroll-link notifications stay in the bell, because they speak to one person and show account activity: a target that lists them fails the render.

```yaml
# webhooks.yaml, passed with -f on install and on every upgrade
app:
  webhooks:
    targets:
      - name: ops                        # [a-z0-9-], unique, shown in the notifier log
        url: https://hooks.example.com/telark
        secret: whsec_REPLACE_ME         # echo "whsec_$(openssl rand -base64 32)"
        events: [rollback.completed, plan.approval.requested, plan.approval.decided, incident.critical]
```

| Key | Description |
|---|---|
| `name` | Lowercase letters, digits and `-`, unique |
| `url` | `https` only, with no user or password in it |
| `secret` | `whsec_` and at least 32 base64 characters (24 random bytes); signs every delivery |
| `previousSecret` | Optional, while receivers move to a new `secret`: each delivery then carries a second signature |
| `events` | One or more of the four types above |
| `allowPrivate` | `true` lets the target be a private or cluster address (`*.svc`, `*.cluster.local`, single-label names, the private ranges below) and use `http` |

The chart renders the list into the Secret `telark-webhooks`. The notifier mounts it and re-reads it about a minute after a change, with no restart. A URL like Slack's is itself a credential, so keep targets out of values files in Git. For Argo CD, Flux or any cluster-less render, create the Secret yourself, key `targets.json` holding the same list as JSON:

```sh
cat > targets.json <<'EOF'
[{"name": "ops", "url": "https://hooks.example.com/telark", "secret": "whsec_REPLACE_ME", "events": ["incident.critical"]}]
EOF
kubectl create secret generic telark-webhook-targets -n telark --from-file=targets.json
```

```sh
--set app.webhooks.existingSecret=telark-webhook-targets
```

The notifier checks a Secret it didn't render the same way. One bad target refuses the whole file: the refusal is logged and the targets already running keep going.

**Payload.** A `POST` with `Content-Type: application/json` and `User-Agent: telark-webhooks/1`:

```json
{"version": 1, "id": "1792143000123-0", "type": "plan.approval.requested", "timestamp": "2026-10-16T09:30:00.123Z",
 "data": {"severity": "info", "title": "Protection plan awaiting approval", "message": "**Data freeze** was submitted for approval by admin.",
          "metadata": {"planId": "p-1", "planName": "Data freeze", "targetId": "p-1"}}}
```

`id` stays the same on every retry, and `timestamp` is when the event happened. `message` is the bell's text, with `**` around names. The `metadata` keys: `rollback.completed` has `applicationId`, `applicationName`, `status` (`success`, `failure` or `aborted`) and `targetId`; `plan.approval.requested` has `planId`, `planName` and `targetId`, and `plan.approval.decided` adds `decision`; `incident.critical` has `insightId` and `targetId`. Ignore fields you don't know: new fields and new types stay in version 1.

**Verify.** `webhook-id` carries `id`, `webhook-timestamp` the Unix seconds of the attempt, and `webhook-signature` `v1,` followed by the base64 HMAC-SHA256 of `<id>.<timestamp>.<body>`, keyed by the base64-decoded part of the secret. While `previousSecret` is set, a second `v1,` signature follows after a space. Refuse a timestamp more than 5 minutes off, and an `id` you already processed within those 5 minutes. Record an `id` only once you have processed its event: a retry that follows your own 5xx carries the same `id`, and must not be refused. The Standard Webhooks libraries check the signature and the timestamp. By hand, in Python:

```python
import base64, hashlib, hmac, time

def verified(secret: str, headers: dict, body: bytes) -> bool:
    msg_id, stamp = headers["webhook-id"], headers["webhook-timestamp"]
    if abs(time.time() - int(stamp)) > 300:
        return False
    key = base64.b64decode(secret.removeprefix("whsec_"))
    digest = hmac.new(key, f"{msg_id}.{stamp}.".encode() + body, hashlib.sha256).digest()
    expected = "v1," + base64.b64encode(digest).decode()
    return any(hmac.compare_digest(expected, sig) for sig in headers["webhook-signature"].split(" "))
```

**Retries.** A 2xx counts as delivered. A network error, a timeout (10 s per attempt), 408, 429 or a 5xx is retried, for 8 attempts in all: after 30 s, 2 min, 10 min, 30 min, 1 h, 2 h and 4 h (each ±10 %), about 8 h, and then the event is given up. `Retry-After` can push an attempt later, by 4 h at most. Any other status fails at once, a 3xx included: redirects are never followed. A delivery owed survives a notifier restart. The order isn't guaranteed, so sort by `timestamp`. The queue keeps the last 1,000 events, so a receiver that stays down that long loses the oldest ones it was owed.

**Log.** One line per attempt in the notifier log, with the target, event id and type, attempt, status, duration and outcome (`delivered`, `retry`, `failed`, `gave_up` or `dropped`), never the URL or a secret:

```sh
kubectl logs -n telark deploy/telark-notifier-service | grep 'webhook target='
```

**Addresses.** The notifier checks the address it actually dials, after DNS, so a name that resolves to a refused address is refused too. Loopback, link-local (169.254.0.0/16, with the cloud metadata address, and fe80::/10), multicast, broadcast and unspecified addresses are always refused. Private addresses (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10 and fc00::/7) need `allowPrivate: true`. No proxy from the environment is used. Egress from Telark pods is otherwise open ([Network policies](#network-policies)).

## Security and cluster integration

These settings have safe defaults. Read them before a production install.

### CRD write guard

Telark's custom resources (users, roles, sessions, passkeys) are its authorization data. The guard (`app.security.crdGuard`, a ValidatingAdmissionPolicy, on and enforcing by default) rejects writes to every `telark.io` resource and subresource, in every namespace, from anything but the owning service accounts (discovery may write only `applications/status`). A second policy does the same for the OIDC trust Secret `telark-oidc-trust-secret`. Edit rights on another namespace therefore don't turn into Telark Admin. Rights on `telark` itself can: `helm upgrade`, `pods/exec`, Secret read and an allowlist entry each let their holder act as any Telark user ([Trust boundaries](security/README.md#trust-boundaries)).

- Break-glass identities go in `app.security.crdGuard.extraAllowedUsers`, for example `--set 'app.security.crdGuard.extraAllowedUsers={system:serviceaccount:ops:breakglass}'`.
- `--set app.security.crdGuard.enforce=false` only audits (logs and allows).
- `--set app.security.crdGuard.enabled=false` removes the guard; the install notes warn when it is off.

### Network policies

With `app.security.networkPolicy.enabled=true` (default) the chart renders ingress NetworkPolicies in the release namespace:

- A default deny for every Telark pod.
- The exporter, discovery, auth, analyzer and notifier APIs accept traffic only from Telark pods of the same release.
- The dashboard (`ui`) accepts traffic on its port from anywhere, because the ingress controller, Gateway or port-forward reaches it from outside the release.
- NATS accepts clients only from discovery and notifier on 4222, and serves no monitoring port (the NATS Services still list 8222).
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

`kyverno.features.forceFailurePolicyIgnore.enabled=true` (default) makes every Kyverno webhook `failurePolicy: Ignore`. While the admission controller is down, rolling out or timing out, the API server admits every request, including ones an `enforce` protection plan would reject. Workloads stay deployable when Kyverno is unhealthy, and enforcement is best-effort.

For strict enforcement turn it off:

```sh
--set kyverno.features.forceFailurePolicyIgnore.enabled=false
```

Fail-closed means an unavailable Kyverno blocks writes to everything its webhooks match, so keep its two admission replicas and its disruption budget.

With your own Kyverno (`kyverno.enabled=false`), this key doesn't apply: plan policies (`NamespacedValidatingPolicy`) set no `failurePolicy`, so they fail closed unless your Kyverno runs with `forceFailurePolicyIgnore`.

### Last-modified annotations

A Kyverno `MutatingPolicy` stamps `telark.io/last-modified-{by,at,operation}` on the kinds discovery tracks (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, ConfigMaps, Services, PVCs, ServiceAccounts, Ingresses, NetworkPolicies, HPAs, VPAs), outside `telark`, `kyverno` and the system namespaces. Change history uses them to name who made a change.

- Secrets are not stamped, so history does not name who changed a Secret.
- The policy renders only when the Kyverno `MutatingPolicy` API (`policies.kyverno.io/v1`) already exists, which on a first install it does not. Run `helm upgrade` once after the first install, with the same flags, to add it. Changes made before that carry no author.
- 0.1.x installed it as a `ClusterPolicy` of the same name, which the upgrade leaves in place: delete it yourself ([0.2.0 upgrade notes](#upgrade-notes)).
- With your own Kyverno (`kyverno.enabled=false`) the chart does not install the policy, so history names no author.

### GitOps (cluster-less renders)

The chart generates the service token, the two NATS users' passwords and the Redis password on install and reads them back on upgrade with `lookup`. Argo CD, Flux and other `helm template` pipelines render without a cluster, so `lookup` returns nothing and every sync would produce new values: pods that restart on the new token can no longer reach their peers. Create the Secrets yourself and point the chart at them:

```sh
kubectl create secret generic telark-service-token -n telark \
  --from-literal=token="$(openssl rand -hex 32)"
kubectl create secret generic telark-nats-publisher -n telark \
  --from-literal=username=publisher --from-literal=password="$(openssl rand -hex 24)"
kubectl create secret generic telark-nats-consumer -n telark \
  --from-literal=username=consumer --from-literal=password="$(openssl rand -hex 24)"
kubectl create secret generic telark-redis-password -n telark \
  --from-literal=redis-password="$(openssl rand -hex 24)"
```

```sh
--set app.security.serviceToken.existingSecret=telark-service-token \
--set nats.existingSecrets.publisher=telark-nats-publisher \
--set nats.existingSecrets.consumer=telark-nats-consumer \
--set redis.auth.existingSecret=telark-redis-password
```

The OIDC trust Secret (the optional Google JWK set, key `googleJwkJson`) is rendered the same way, so every sync would empty it. Create it yourself and point the chart at it. The exporter updates it when the bootstrap admin saves the JWK set in the dashboard (saving `googleJwkJson: ""` clears it), and the CRD write guard and the exporter's RBAC cover the name you pass:

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
--set 'metrics-server.args={--kubelet-insecure-tls}'
```

### CORS

No API service sends CORS headers unless its `CORS_ALLOWED_ORIGINS` env var (`services.{exporter,discovery,auth,analyzer}.env`) lists the browser origins allowed to call it directly (comma-separated exact origins; an entry holding `*` is ignored, because credentials are allowed). The dashboard reaches every API through its own nginx proxy on the same origin, so installs leave them empty. For UI development against a port-forwarded cluster, set `http://localhost:3000` on all four (`charts/telark/values.dev.yaml` does): the services then allow credentials, which carry the session cookie across the ports, and the `X-Requested-With` header the dashboard sends.

### Sign-in rate limits

Auth limits the four sign-in starts anyone can call without a session: passkey sign-in, passkey registration, the break-glass approval and the Google sign-in nonce. In each one-minute window, a client gets `RATE_LIMIT_PER_CLIENT` starts on each of them (default 30), and all clients together `RATE_LIMIT_GLOBAL` (default 3000). Past either limit, auth answers 429 with `Retry-After: 60` and the message "too many attempts right now: wait a minute, then try again". The finishes, logout and every other route are not limited, and while Redis is down the starts go through.

Auth finds the client in `X-Forwarded-For`. The dashboard's nginx appends the address it got the request from, and every proxy in front of it appends one more, so auth takes the entry `RATE_LIMIT_TRUSTED_HOPS` from the right and ignores what the client wrote to the left of it; an IPv6 client counts per /64. The chart sets 1 with `ingress.enabled` or `gateway.enabled`, else 0. Set it yourself when your path differs:

- Another L7 proxy or cloud load balancer in front of the ingress controller or gateway adds a hop: `--set services.auth.env.RATE_LIMIT_TRUSTED_HOPS=2`.
- NodePort or LoadBalancer: keep 0, and keep the client's address on the `ui` Service, or every client counts as the node that forwarded it. The same goes for your ingress controller's own Service.

  ```sh
  kubectl patch svc -n telark telark-ui-service -p '{"spec":{"externalTrafficPolicy":"Local"}}'
  ```

A wrong hop count weakens only the per-client limit: too high trusts an address the client wrote, too low counts a proxy as the client. The per-route limit holds either way, and a flood from many addresses can hold a start at it, so sign-in waits for the next minute while the flood lasts.

## Upgrade

Back up first. After an upgrade that ran a migration step, `helm rollback` to the previous release still works, but an exporter two or more releases older refuses to start on the data, so the way back that far is the backup. Take an etcd snapshot (`etcdctl snapshot save`) where you run the control plane yourself, or a [Velero](https://velero.io/) backup of the namespace:

```sh
velero backup create telark-before-upgrade --include-namespaces telark --wait
```

Telark keeps state in `.status` (plan phases and approvals, application history, the migration record), and a Velero restore leaves it out unless it names those resources: `--status-include-resources applications.telark.io,protectionplans.telark.io,telarkconfigs.telark.io`.

Read the [upgrade notes](#upgrade-notes) of every version between yours and the target, then upgrade:

```sh
helm upgrade telark oci://ghcr.io/telark/charts/telark -n telark \
  --set app.mode=<mode>
```

Pass the same `--set` and `-f` flags you used at install: Helm does not remember them across upgrades.

- **CRDs.** The chart upgrades the CRDs with the app. CRDs managed out of band (`crds.enabled=false`): upgrade `telark-crds` before `telark` ([chart README](../charts/telark/README.md#upgrade-order)).
- **Migrations.** Before it serves, the new exporter runs the migration steps the upgrade notes list and records the last one in the `TelarkConfig` status ([Migrations](CRDS.md#migrations)). A failed step stops the exporter, which retries it on every restart; its log names the step (`kubectl logs -n telark deploy/telark-exporter-service`). After an upgrade that ran a step, `helm rollback` to the previous release still works: its exporter lowers the record to its own last step and logs `lowered the record`, and the next upgrade runs the newer steps again. To go back further, restore the backup.
- **Redis and NATS volume sizes.** Kubernetes cannot change a StatefulSet's volume size, so an upgrade that changes `redis.master.persistence.size` or `nats.persistence.size` stops and prints a command to run once, for example `kubectl delete statefulset -n telark telark-redis-master telark-nats --cascade=orphan`. The pods, claims and data stay, the claims keep their size, and the next upgrade re-creates the StatefulSets. To skip the step, keep the installed size on every upgrade. Cluster-less renders cannot check the live size, so run the command before the sync ([GitOps](#gitops-cluster-less-renders)).
- **Ollama volume size.** Kubernetes never shrinks a claim, so an upgrade that asks for a smaller `ollama.persistentVolume.size` stops and names the size to keep on every upgrade.
- **Turning on `vpa.enabled` later.** Helm installs a subchart's CRDs only on `helm install`, so an upgrade that sets `vpa.enabled=true` starts the VPA operator without its CRDs, and no service gets a VerticalPodAutoscaler. Apply the CRDs of the bundled VPA chart (the `vpa` version in `charts/telark/Chart.yaml`) first; the upgrade then creates the VPAs too:

  ```sh
  helm pull vpa --repo https://charts.fairwinds.com/stable --version 5.1.0 --untar --untardir /tmp/telark-vpa
  kubectl apply --server-side -f /tmp/telark-vpa/vpa/crds/
  ```

### Upgrade notes

Each minor version lists here the CRD schema changes and the exporter migration steps it brings ([schema rules](CRDS.md#schema-changes-across-releases)). A version not listed changes neither.

- **0.2.0**: `TelarkConfig` gets `status.lastMigration` and `status.rollbackMigration`, the migration record. No migration step.
  - Upgrade from 0.1.x with `--reset-then-reuse-values` (or your full `-f` and `--set` flags), not `--reuse-values`. That flag keeps 0.1.x's defaults, which lack the keys 0.2.0 adds, so the render fails (`nil pointer evaluating interface {}.existingSecret`).
  - Some values moved. The old paths still work in 0.2.0, and win when set, but a later release removes them; the install notes list any that your values still use. An old path an earlier release set keeps winning under `--reset-then-reuse-values`: set the value on the old path, or upgrade with `--reset-values` and your flags on the new paths.

    | Old path | New path |
    |---|---|
    | `app.crdGuard.*` | `app.security.crdGuard.*` |
    | `app.networkPolicy.enabled` | `app.security.networkPolicy.enabled` |
    | `app.serviceToken.*` | `app.security.serviceToken.*` |
    | `app.kubectlImage` | `app.image.kubectl` |
    | `app.kyverno.enabled` | `kyverno.enabled` |
    | `app.ollama.enabled` | `ollama.enabled` |
    | `app.kyverno.failOpen` | removed: set `kyverno.features.forceFailurePolicyIgnore.enabled`, the one Kyverno reads (a different `failOpen` still fails the render) |

  - Everyone signs in once after the upgrade: the session moves from the dashboard's `localStorage` into an HttpOnly cookie (`__Host-telark_session` when every `app.auth.passkey.origin` entry is https, else `telark_session`), and the dashboard revokes the old token at its first load. Upgrade the dashboard and the services together (one chart release), since a new dashboard on an old backend gets no cookie. The passkey and Google sign-in answers no longer carry `sessionToken`; API clients keep sending `X-Session-Token`, and a change that relies on the cookie needs `X-Requested-With`.
  - Auth limits the public sign-in starts per client and per route, then answers 429 ([Sign-in rate limits](#sign-in-rate-limits)). With another proxy in front of your ingress or gateway, or with a NodePort or LoadBalancer, check `RATE_LIMIT_TRUSTED_HOPS` and the `ui` Service's `externalTrafficPolicy` there.
  - Enrollment links issued before the upgrade stop working, because auth now signs the Redis entries behind them. Create new links from Members, and run break-glass again for the bootstrap admin. A passkey registration open during the upgrade starts again.
  - `TelarkConfig` gets `status.bootstrap`, the pinned bootstrap account and its break-glass requests ([Recovery requests](#recovery-requests)). At each start, before it serves, the exporter pins the bootstrap account that has a passkey (the `app.auth.bootstrap.admin` one first, else the oldest) and clears `bootstrap` on every other account, which keeps its roles: check Members afterwards. On a pinned install, break-glass opens a request instead of acting, and a changed `app.auth.bootstrap.admin` no longer makes a second bootstrap account. A rollback to 0.1.x drops `status.bootstrap`, which its CRD doesn't keep, so the pin and any open break-glass request are gone, and the next upgrade pins again.
  - `TelarkConfig` gets `spec.breakGlassAlerts.notifyAdmins`, the bootstrap account's [break-glass alerts](#recovery-requests) switch. The CRD defaults it to `true`, and the exporter reads a missing value as `true`, so Admins on `ALL` keep hearing of requests and no migration step is needed. A rollback to 0.1.x drops the field, so after the next upgrade the switch is on again until the bootstrap account turns it off.
  - Administrators see the bootstrap account as "Bootstrap account", with no username, full name or email ([First admin](#2-first-admin)). Notices sent before the upgrade keep the names they were sent with.
  - The reports volume gets a `kept/` directory: a plan delete that keeps the plan's reports moves them there, with a `plan.json` holding the plan's name. Reports already stored stay under `plans/` and are not kept, so nothing migrates. A rollback to 0.1.x leaves `kept/` alone (its sweep reads only `plans/`) but cannot list or download those reports; they come back with the next upgrade. Kept reports are never swept, so they count against `app.persistence.reportsSize` until someone deletes them.
  - Plan policies move from Kyverno's deprecated `kyverno.io/v1` `Policy` to `policies.kyverno.io/v1` `NamespacedValidatingPolicy`, under the same names, so nothing migrates. Within a controller tick of the upgrade, discovery deploys each active plan's new policies; it no longer removes 0.1.x's `Policy` objects. The chart's last-modified policy becomes a `MutatingPolicy` ([Last-modified annotations](#last-modified-annotations)), and the upgrade also leaves 0.1.x's `ClusterPolicy` of the same name. Once every active plan reads Healthy, delete both:

    ```sh
    kubectl delete policies.kyverno.io -A -l app.kubernetes.io/managed-by=telark
    kubectl delete clusterpolicies.kyverno.io telark-inject-modifier
    ```

    Until then the old and the new plan policies both refuse, so a refusal can show twice in a plan's admission decisions, and both last-modified policies stamp the same annotations, which does no harm. The new policy writes `telark.io/last-modified-at` with fractional seconds (RFC 3339, such as `2026-10-20T09:15:02.123456789Z`) where 0.1.x wrote whole seconds, so a script that reads the annotation must accept a fraction. Refusal messages now name the plan by its name instead of its ID and end with ` (requested by "<username>")`, and the plan's admission decisions and reports show that username. Before you move Kyverno to 1.20, check that these return nothing: `kubectl get policies.kyverno.io -A -l app.kubernetes.io/managed-by=telark` and `kubectl get clusterpolicies.kyverno.io telark-inject-modifier --ignore-not-found`. After a rollback to 0.1.x, once its controller has redeployed the old policies (about a minute), delete the new ones, which 0.1.x leaves enforcing:

    ```sh
    kubectl delete namespacedvalidatingpolicies.policies.kyverno.io -A -l app.kubernetes.io/managed-by=telark
    kubectl delete mutatingpolicies.policies.kyverno.io telark-inject-modifier
    ```

    `helm rollback` runs no post-upgrade hook, so 0.1.x's last-modified `ClusterPolicy` doesn't come back, and nothing stamps last-modified annotations. Then upgrade once to the same 0.1.x chart, with the flags you installed with, and its hook recreates the policy: `helm upgrade telark oci://ghcr.io/telark/charts/telark --version <0.1.x> -n telark --reset-then-reuse-values`.

  - `ProtectionPlan` gets `spec.kindRef`, `spec.repeat` (with the new `timeMode: repeating`) and `status.window`, and categories get the `plan-kinds` scope, with four built-in kinds (QA, Testing, Release, Maintenance window) added at the exporter's first start. Existing plans get no kind and keep their schedule, so nothing migrates. Upgrade CRDs you manage yourself first: an older schema refuses `repeating` and silently drops `kindRef`. Before a rollback to 0.1.x, cancel repeating plans or give them another schedule: 0.1.x treats a repeating plan as permanent and never opens or closes its windows. The rollback brings back 0.1.x's CRDs, whoever manages them, and they keep neither `spec.kindRef`, `spec.repeat` and `status.window` on plans nor the migration record on the `TelarkConfig`: plans show no kind, and an object 0.1.x writes in between loses those fields for good, so note each plan's kind first. Each repeating plan keeps its newest 100 window reports, a cap on the count, not the size: one report can reach 16 MiB, so a busy Enforce series can hold up to about 1.6 GiB of `app.persistence.reportsSize`, like kept reports.
  - Role create and patch answer 400 for a deny rule that isn't `<scope>.<action>.deny` with a known scope and action, such as a bare `rollbackapplication` instead of `applications.rollbackapplication.deny`. Stored roles are not migrated and keep loading. Such a rule never denied anything, because authorization matches only the full key: replace it the next time you edit the role's scopes, since that edit answers 400 while it stays.

## Uninstall

Let in-progress rollbacks finish.

```sh
helm uninstall telark -n telark
```

This removes every Telark service and the bundled policy engine with its webhooks and plan policies. The pre-delete hooks run `app.image.kubectl`, so air-gapped clusters must mirror it.

Your data stays: the CRDs and custom resources (users, roles, plans and the rest), the service-token, NATS, Redis and OIDC Secrets, the exporter's snapshot and report volumes, the Redis and NATS volumes, and the Ollama model volume. A reinstall picks up where you left off.

With an external policy engine (`kyverno.enabled=false`), also remove the plan policies, which would otherwise keep enforcing:

```sh
kubectl delete namespacedvalidatingpolicies.policies.kyverno.io -A -l telark.io/protection-plan
```

### Full teardown

To also delete all Telark data, run after `helm uninstall` (about 15 seconds):

```sh
kubectl delete crd -l app.kubernetes.io/part-of=telark
kubectl delete namespace telark
```

`helm uninstall` already cleared the cleanup finalizers on users, groups and roles; a reinstall that keeps the data restores them within a minute. If the uninstall ran with `--no-hooks`, the bundled policy engine's webhooks stay behind and keep intercepting API writes; delete them:

```sh
kubectl delete validatingwebhookconfigurations,mutatingwebhookconfigurations -l webhook.kyverno.io/managed-by=kyverno
```

A `--no-hooks` uninstall also leaves the finalizers, so deletions hang in `Terminating`; clear them by hand and they finish on their own:

```sh
kubectl get users.telark.io,groups.telark.io,accessroles.telark.io -n telark -o name |
  xargs -r -P 16 -I{} kubectl patch -n telark {} --type merge -p '{"metadata":{"finalizers":null}}'
```

## Known limitations

- The API is `telark.io/v1alpha1` and may change before 1.0. Pin the chart version.
- Single cluster per install.
- By default the chart bundles Kyverno, Redis, NATS, metrics-server and Ollama. The bundled Kyverno can be turned off to use one you already run ([Install](#1-install)).
- The bundled Kyverno stays on 1.19. Kyverno 1.20 is untested.
