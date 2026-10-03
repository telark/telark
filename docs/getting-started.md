# Getting started

In this tutorial you install Telark, sign in as the first admin, watch a demo application appear, and protect it with a protection plan in audit mode, so you see Telark record a change the plan would have blocked. It takes about 10 minutes, most of it waiting for pods to start.

## Before you start

You need:

- A Kubernetes cluster, version 1.30 or newer, and `kubectl` pointed at it.
- Helm 3.
- A default StorageClass. Managed clusters, kind, minikube, k3d and Docker Desktop have one.
- A browser that supports passkeys, and a way to create one: Touch ID, Windows Hello, a phone or a security key.

## 1. Install Telark

Replace `test@example.com` with your email:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com
```

The chart installs Telark's services together with Kyverno, Redis, NATS, metrics-server and Ollama. Wait until every pod is running:

```sh
kubectl get pods -n telark -l app.kubernetes.io/instance=telark
```

You should see every pod `Running` and `READY`. The first start pulls several images, so allow a few minutes.

## 2. Sign in as the first admin

Create a one-time enrollment token with the auth service's break-glass command, using the email you passed at install:

```sh
kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email test@example.com --enroll
```

You should see a line like `enrollment token for test@example.com (expires …): <token>`. The token is valid for 10 minutes.

Forward the dashboard to your machine and leave the command running:

```sh
kubectl port-forward -n telark svc/telark-ui-service 3000:8080
```

Open `http://localhost:3000/register?enroll=<token>` and create your passkey. You are now signed in as Admin.

Passkeys are bound to the host name you register them on. If you later expose the dashboard on another host, see [Access the dashboard](INSTALL.md#access-the-dashboard).

## 3. See your applications

Create a small demo application:

```sh
kubectl create namespace demo
kubectl create deployment web --image=nginx:1.27 --replicas=2 -n demo
```

`kubectl create deployment` labels the Deployment `app=web`, so Telark sees it as an application named `web` ([how applications are grouped](concepts.md#applications)).

Open **Applications** in the dashboard. You should see `web` in the `demo` namespace within a few seconds, next to the applications already running in your cluster. Open it to see its workloads, health and change history.

## 4. Create a protection plan in audit mode

Audit mode records what a [protection plan](concepts.md#protection-plans) would block without blocking it, so it is the safe way to start.

1. Open **Protection plans** and select **Create Plan**.
2. Under **Details**, name the plan `web freeze` and pick a severity.
3. Leave **Environment** empty. A plan in the Production environment requires approval.
4. Under **Execution & approval**, keep **Automatic**.
5. Under **Scope**, choose **Applications** and select `web`.
6. Under **Schedule**, choose **Time range**, start now and end in one hour.
7. Under **Policies**, set the mode to **Audit** and add the template **Block Replica Scaling**.
8. Select **Create Plan**.

You should see the plan as **Active** and, after the next health check (about 30 seconds), its health as **Healthy**. Telark has created a Kyverno policy in the `demo` namespace:

```sh
kubectl get policies.kyverno.io -n demo -l telark.io/protection-plan
```

## 5. See a violation

Scale the protected Deployment:

```sh
kubectl scale deployment web -n demo --replicas=3
```

The command succeeds, because the plan only audits. Open the plan and look at **Violations**. Within a minute (select **Refresh** if needed) you should see the scale request on `web`, with a message saying replica scaling would be blocked by the plan.

To block the change for real, edit or duplicate the plan and set the mode to **Enforce**. The same `kubectl scale` command is then refused by the admission webhook with the message "Replica scaling is blocked by protection plan …".

When you are done, cancel the plan from its page. Telark removes its Kyverno policies. A report of what happened while the plan ran is available under **Reports** in HTML, Markdown, JSON or CSV.

## 6. Optional: turn on automatic analysis

[Insights](concepts.md#insights) is on by default. Its model (708 MB) downloads in the background after install; until it finishes, cards show the rule text without the model's rewording. To have incidents analyzed as they happen:

1. Open **Settings**, then **Insights**, and turn on **Analyze automatically on incidents and recoveries**.
2. Break the demo application:

   ```sh
   kubectl set image deployment/web nginx=nginx:does-not-exist -n demo
   ```

3. Open **Insights**. You should see a card for `web` naming an image pull failure, the evidence it used and the change it followed. You can also run an analysis yourself with **Analyze**.

## Clean up

Delete the demo application with `kubectl delete namespace demo`. To remove Telark, see [Uninstall](INSTALL.md#uninstall).

## Next steps

- [Concepts](concepts.md): how applications, protection plans, change history, Insights and access control fit together.
- [Install and configure](INSTALL.md): sizing modes, exposing the dashboard over HTTPS, Google SSO, GitOps and upgrades.
- [Protection plans in depth](architecture/protection-plans.md): lifecycle, approvals, health checks, violations and reports.
