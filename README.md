<p align="center">
  <img src="docs/assets/telark-banner.svg" alt="telark" width="320">
</p>

<p align="center">
  <b>A protection gate for your Kubernetes workloads.</b><br>
  Discover your applications, then decide what can change them — and when.
</p>

<p align="center">
  <a href="https://telark.io">telark.io</a> ·
  <a href="docs/INSTALL.md">Install</a> ·
  <a href="docs/">Docs</a> ·
  <a href="https://github.com/telark/telark/issues">Issues</a>
</p>

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-Elastic--2.0-2f6feb.svg" alt="License: Elastic-2.0"></a>
</p>

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

Install the CRDs, then the app — two one-line commands, both from the registry:

```sh
helm install telark-crds oci://ghcr.io/telark/charts/telark-crds
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace
```

Size it for the cluster with one flag (`minimal` | `standard` | `performance`):

```sh
helm install telark oci://ghcr.io/telark/charts/telark -n telark --create-namespace \
  --set app.mode=performance
```

Full walkthrough: [docs/INSTALL.md](docs/INSTALL.md).

## Architecture

Six services plus shared infrastructure, all shipped by the Helm chart:

| Service | Role |
|---|---|
| `exporter` | Owns the CRDs/storage; seeds built-ins; snapshots cluster state |
| `discovery` | Groups workloads into applications; drives protection-plan reconciliation |
| `enrichment` | AI insights over applications (Python/FastAPI) |
| `auth` | Passkey + Google OIDC login, sessions, roles |
| `notifier` | Notifications |
| `ui` | Dashboard (separate repo; image only) |

Subcharts: redis, nats, kyverno, metrics-server, ollama. Details in [docs/architecture.md](docs/architecture.md).

## Documentation

- [Install guide](docs/INSTALL.md) · [Architecture](docs/architecture.md) · [CRD reference](docs/CRDS.md) · [ADRs](docs/adr/)

## Contributing

Go workspace, `GOPRIVATE`, build/lint/test — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities privately — [SECURITY.md](SECURITY.md).

## License

[Elastic License 2.0](LICENSE.md).
