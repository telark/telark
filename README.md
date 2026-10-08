<p align="center">
  <img src="docs/assets/telark-banner.svg" alt="Telark" width="320">
</p>

<h3 align="center">A protection gate for your Kubernetes applications</h3>

<p align="center">
  Decide what can change an application, and when.<br>
  Telark holds the line during releases and maintenance windows, shows every change that got through,<br>
  and tells you why an app broke. Self-hosted, in your cluster.
</p>

<p align="center">
  <a href="docs/getting-started.md"><b>Get started</b></a> ·
  <a href="docs/">Docs</a> ·
  <a href="https://telark.io">Website</a> ·
  <a href="https://github.com/telark/telark/discussions">Discussions</a>
</p>

<p align="center">
  <a href="https://github.com/telark/telark/actions/workflows/ci.yaml"><img src="https://github.com/telark/telark/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/telark/telark"><img src="https://codecov.io/gh/telark/telark/graph/badge.svg" alt="Coverage"></a>
  <a href="https://github.com/telark/telark/releases"><img src="https://img.shields.io/github/v/release/telark/telark?sort=semver&color=2f6feb" alt="Release"></a>
  <a href="https://artifacthub.io/packages/search?repo=telark"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/telark" alt="Artifact Hub"></a>
  <img src="https://img.shields.io/badge/Kubernetes-%E2%89%A51.30-326ce5?logo=kubernetes&logoColor=white" alt="Kubernetes 1.30+">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-Elastic--2.0-2f6feb.svg" alt="License: Elastic-2.0"></a>
</p>

<p align="center">
  <a href="https://telark.io"><img src="https://www.telark.io/readme/hero.gif" alt="Telark gate: an approved change lands with its snapshot and insights, a blocked one is pushed back" width="840"></a>
</p>

> [!NOTE]
> Early access. The API is `v1alpha1` and may change before 1.0. Pin a chart version.

## Why Telark

It is 23:40, the release is half out, and someone scales `checkout` to zero from a terminal they forgot was pointed at production. Kubernetes allowed it, because RBAC decides *who* may change a workload, never *when*. Then the pages start, and the first question is always the same: what changed?

Telark turns that night into a non-event:

| Today | With Telark |
|---|---|
| A change freeze is a message in a chat channel | A **protection plan** blocks the changes you name (deletion, scaling, image, config and Secret edits, storage) for exactly the window you set. It arms and disarms itself. |
| You hope nobody edited or removed the safeguards | Telark flags drift in the live cluster and keeps a report when the window closes |
| "What changed?" costs an hour of `kubectl` | Every change is recorded field by field, deletions included, with a snapshot you can roll back to in one click |
| You piece the incident together from six workloads | **Insights** names the affected workload and the likely cause |
| Access is all or nothing per namespace | Roles per area with per-action rules, such as "may edit plans, may not approve them" |

## How it works

1. **Discover.** Telark groups your workloads into applications on its own. You protect `checkout`, not seventeen Deployments.
2. **Plan.** Pick the changes to block from ready-made templates, choose the app or namespace and the window, and run it in audit mode first. Add an approval step when it matters; Production plans always require one.
3. **Enforce.** While the window is open, Kyverno (bundled with the chart by default, or one you already run) refuses those changes at admission.
4. **Verify.** Telark checks the live cluster for the policies it expects and reports what was blocked, audited or tampered with.

You work in a dashboard, not in policy YAML.

## Insights: decision support, not autopilot

When an application degrades, Telark reads its events, pod status and recent changes and writes one card per affected workload: crash loop, out of memory, image pull failure, scheduling, failing probes, stuck rollout, or a regression right after a config change. Each card cites the evidence and the change it followed, and resolves itself when the workload recovers. Telark also reviews every application's setup against 60 rules and lists what to fix, such as a single replica with no disruption budget in production.

How it stays trustworthy:

- By default, deterministic rules decide every finding. A small open-weight model only rewrites the wording, and Telark discards any rewrite that drops or invents a fact. Opt-in deep mode, for bigger nodes or a GPU, lets the model investigate with the same read-only tools.
- It is read-only and runs in your cluster through Ollama: no data leaves it, no API key is needed, and it works air-gapped.
- It is on by default, you can switch it off in Settings, and the rest of Telark works without it. Details: [Concepts](docs/concepts.md#insights).

## Quick start

You need Kubernetes 1.30+, Helm 3, and a default StorageClass (managed clusters and kind, minikube or k3d have one). Replace `test@example.com` with your email.

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com
```

**Already running Kyverno?** Install with the bundled Kyverno disabled and Telark uses yours:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com \
  --set app.kyverno.enabled=false
```

Telark is tested with Kyverno v1.19.1, the version the chart bundles. With your own Kyverno, change history names no author, because the chart installs the policy that records authors only with the bundled Kyverno.

When the pods are Running, enroll yourself as the first admin and open the dashboard:

```sh
kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email test@example.com --enroll
kubectl port-forward -n telark svc/telark-ui-service 3000:8080
# open http://localhost:3000/register?enroll=<token printed above>
```

Your applications appear on their own. Create a protection plan in audit mode to see what it would block; the [getting started guide](docs/getting-started.md) walks through it in about ten minutes.

## Documentation

| | |
|---|---|
| [Getting started](docs/getting-started.md) | Install, first login, first protection plan |
| [Concepts](docs/concepts.md) | Applications, protection plans, insights, access control |
| [Install and configure](docs/INSTALL.md) | Sizing, exposure, SSO, networking, upgrades, uninstall |
| [Chart values](charts/telark/VALUES.md) | Every configurable value |
| [Custom resources](docs/CRDS.md) | The `telark.io` API |
| [Architecture](docs/architecture.md) | Services and data flow |
| [Security model](docs/security/README.md) | Authentication, authorization, trust boundaries |

## Compatibility

Kubernetes 1.30 or newer; 1.33+ is the tested target. Only GA Kubernetes APIs are used. Runs on EKS, GKE, AKS and OpenShift. Tested with Kyverno v1.19.1, the version the chart bundles.

## Known limitations

- The API is `telark.io/v1alpha1` and may change before 1.0. Pin the chart version.
- Single cluster per install.
- By default the chart bundles Kyverno, Redis, NATS, metrics-server and Ollama. The bundled Kyverno can be turned off to use one you already run ([Quick start](#quick-start)), and the bundled metrics-server with `metrics-server.enabled=false`.

## Community

Questions and ideas go to [Discussions](https://github.com/telark/telark/discussions), bugs to [Issues](https://github.com/telark/telark/issues). To contribute, start with [CONTRIBUTING.md](CONTRIBUTING.md); the dashboard lives in [telark/dashboard-ui](https://github.com/telark/dashboard-ui). Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

Source-available under the [Elastic License 2.0](LICENSE.md). You may use, modify and self-host Telark; you may not offer it to others as a managed service. Third-party software in Telark's images and chart stays under its own license: see [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).
