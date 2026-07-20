<p align="center">
  <img src="docs/assets/telark-banner.svg" alt="telark" width="320">
</p>

<p align="center">
  <b>A protection gate for your Kubernetes workloads.</b><br>
  Discover your applications, then decide what can change them — and when.
</p>

<!-- TODO: add a hero screenshot / demo GIF of the dashboard here, e.g.
<p align="center"><img src="docs/assets/dashboard.png" alt="telark dashboard" width="820"></p>
-->

<p align="center">
  <a href="https://telark.io">Website</a> ·
  <a href="docs/INSTALL.md">Install</a> ·
  <a href="docs/">Docs</a> ·
  <a href="https://github.com/telark/telark/discussions">Discussions</a> ·
  <a href="https://github.com/telark/telark/issues">Issues</a>
</p>

<p align="center">
  <a href="https://github.com/telark/telark/actions/workflows/ci.yaml"><img src="https://github.com/telark/telark/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
  <!-- Replace CODECOV_BADGE_TOKEN with the graph token from codecov.io → repo Settings → Badges & Graphs. Private repos need it; if you make the badge public, drop the ?token=… part. -->
  <a href="https://codecov.io/gh/telark/telark"><img src="https://codecov.io/gh/telark/telark/graph/badge.svg?token=CODECOV_BADGE_TOKEN" alt="Coverage"></a>
  <a href="https://github.com/telark/telark/releases"><img src="https://img.shields.io/github/v/release/telark/telark?sort=semver&color=2f6feb" alt="Release"></a>
  <img src="https://img.shields.io/badge/Kubernetes-%E2%89%A51.30-326ce5?logo=kubernetes&logoColor=white" alt="Kubernetes >= 1.30">
  <img src="https://img.shields.io/badge/Helm-OCI-0f1689?logo=helm&logoColor=white" alt="Helm OCI chart">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-Elastic--2.0-2f6feb.svg" alt="License: Elastic-2.0"></a>
</p>

> [!NOTE]
> **Early access.** telark is under active development toward its first stable release — chart and APIs may still change. Pin a chart version for production installs.

---

## Why telark

Kubernetes gives everyone with cluster access the power to change anything, at any time. During a maintenance window, a risky deploy, or a test run, that is exactly what you do **not** want.

telark puts a **time-scoped protection gate** around the workloads that matter. A stray `kubectl delete`, an accidental scale-down, or an unreviewed image swap simply does not go through until you say so — decided at admission, and verified against the live cluster, not assumed from "the write succeeded".

No new mental model, no YAML archaeology: discover your apps, pick what to protect, open a window.

## What you get

- **Application-aware protection** — telark groups raw workloads into *applications* automatically. Protect "checkout", not seventeen Deployments.
- **Time-bounded windows** — freeze a scope from 22:00 to 02:00 tonight; it arms and disarms itself on schedule.
- **Ready-made policy templates** — block deletion, replica scaling, image patterns/tags, storage changes, ConfigMap/Secret edits, and more. Run in **audit** first, flip to **enforce** when you trust it.
- **Cluster-truth health** — telark reads the cluster to confirm the protection you asked for is the protection actually running. Drift, missing policies, and tampering are surfaced.
- **AI insights** — optional per-application summaries and risk signals.
- **Modern auth** — passkeys and Google SSO, with a built-in role model.

## How it works

1. **Discover** — workloads are grouped into applications in real time.
2. **Plan** — bind policy templates to a scope and a window; run now or schedule.
3. **Enforce** — while a plan is active, admission decisions protect its scope.
4. **Verify** — health is computed from live cluster state, not from the API's word for it.

## Quick start

> **Prerequisites:** Kubernetes ≥ 1.30 (1.33+ recommended), Helm 3, and a default StorageClass.

One command from the registry — CRDs, dashboard, and everything else ship with the chart:

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace
```

Size it for the cluster with one flag (`minimal` · `standard` · `performance`):

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance
```

Then reach the dashboard:

```sh
kubectl port-forward -n telark svc/telark-ui-service 3000:8080
# open http://localhost:3000
```

Full walkthrough, sizing, autoscaling, monitoring and ingress: **[docs/INSTALL.md](docs/INSTALL.md)**.

## Architecture

Six services plus shared infrastructure, all shipped by one Helm chart:

| Service | Role |
|---|---|
| `exporter` | Owns the CRDs/storage; seeds built-ins; snapshots cluster state |
| `discovery` | Groups workloads into applications; drives protection-plan reconciliation |
| `enrichment` | AI insights over applications (Python/FastAPI) |
| `auth` | Passkey + Google OIDC login, sessions, roles |
| `notifier` | Notifications |
| `ui` | Dashboard SPA, deployed by default (image built from a separate repo) |

Bundled subcharts: redis, nats, kyverno, metrics-server, ollama. Deep dive in **[docs/architecture.md](docs/architecture.md)**.

## Kubernetes compatibility

telark installs on **Kubernetes 1.30 or newer** (enforced by the chart's `kubeVersion`) and emits only **GA** APIs (`apps/v1`, `autoscaling/v2`, `networking.k8s.io/v1`, `policy/v1`, `apiextensions.k8s.io/v1`, `admissionregistration.k8s.io/v1`), so newer releases work as they ship.

| Tier | Kubernetes | Meaning |
|---|---|---|
| **Supported** | **1.33+** — the minors under [upstream support](https://kubernetes.io/releases/version-skew-policy/) | tested target; bugs fixed here |
| **Best-effort** | **1.30 – 1.32** | fully functional (every API is GA), but older / less-tested and possibly EOL upstream |

Managed distros: EKS · GKE · AKS · OpenShift. The floor is 1.30 because the optional `crdGuard` uses ValidatingAdmissionPolicy (GA in 1.30); the bundled policy engine (kyverno) only needs 1.25.

## Documentation

| | |
|---|---|
| 🚀 [Install guide](docs/INSTALL.md) | Install, sizing modes, autoscaling, monitoring, ingress |
| 🏗️ [Architecture](docs/architecture.md) | Services, data flow, admission model |
| 📦 [CRD reference](docs/CRDS.md) | Custom resources telark installs |
| 🧭 [ADRs](docs/adr/) | Architecture decision records |
| ⚙️ [Chart values](charts/telark/VALUES.md) | Every configurable value |
| 🛠️ [Development](docs/DEVELOPMENT.md) | Make targets, local build/test |
| 📐 [Conventions](CONVENTIONS.md) | Code, naming, testing, AI-agent workflow |
| 🏛️ [Governance](GOVERNANCE.md) | Roles & the contributor ladder |

## Community & support

- 💬 **Questions & ideas** — [GitHub Discussions](https://github.com/telark/telark/discussions)
- 🐛 **Bugs & feature requests** — [Issues](https://github.com/telark/telark/issues)
- 🤝 **Code of Conduct** — [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- 🌐 **Website** — [telark.io](https://telark.io) · reach us at `contact@telark.io`

## Contributing

Contributions are welcome. The repo is a Go workspace (services + shared internal packages) plus a Python service and the Helm charts. Start with **[CONTRIBUTING.md](CONTRIBUTING.md)**, the rulebook in **[CONVENTIONS.md](CONVENTIONS.md)**, and roles/promotion in **[GOVERNANCE.md](GOVERNANCE.md)**.

## Security

Found a vulnerability? Please report it **privately** — see **[SECURITY.md](SECURITY.md)**. Do not open a public issue for security reports.

## License

telark is **source-available** under the [Elastic License 2.0](LICENSE.md): free to use, copy, modify, and self-host — you may not provide it to others as a managed service, or remove the license/keys. See the license for the exact terms.
