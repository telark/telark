# telark

**A protection gate for your Kubernetes workloads.** Discover your applications, then decide what can change them, and when.

telark puts a policy shield around your Kubernetes applications during maintenance windows, test runs, and risky changes. Define policies, open a protection window scoped to discovered applications, and nothing touches those workloads without your say — decided at admission and verified against live cluster state.

Website: [telark.io](https://telark.io)

## How it works

- **Discovery** groups raw workloads into *applications* in real time.
- **Protection plans** bind policy templates to a scope and a window; a plan runs only while active.
- **Cluster-truth health** reads the cluster to confirm the plan that should be running is the one actually enforced — drift, missing policies, and tampering are surfaced.

## Architecture

Six first-class services plus shared infrastructure, all shipped by the Helm chart:

| Service | Role |
|---|---|
| `exporter` | Owns the CRDs/storage; seeds built-ins; snapshots cluster state |
| `discovery` | Groups workloads into applications; dispatches enrichment; reconciles plans |
| `enrichment` | AI insights over applications (Python/FastAPI) |
| `auth` | Passkey + OIDC authentication, session and role reconciliation |
| `notifier` | Notifications |
| `ui` | Dashboard (separate repo, image only) |

Shared infra (subcharts): redis, nats, kyverno, metrics-server, ollama.

See [`docs/`](docs/) for the full architecture, install guide and CRD reference.

## Quick start

Install the CRDs, then the app:

```sh
helm install telark-crds ./charts/telark-crds -f charts/telark/values.yaml
helm install telark ./charts/telark \
  --set-file nats.configuration=charts/telark/config/nats.conf
```

Optional sizing overlay (`minimal` | `standard` | `performance`):

```sh
helm install telark ./charts/telark \
  -f charts/telark/values.mode.standard.yaml \
  --set-file nats.configuration=charts/telark/config/nats.conf
```

Full instructions: [`docs/INSTALL.md`](docs/INSTALL.md).

## Development

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the Go workspace, `GOPRIVATE`, build/lint/test flow.

## Security

Report vulnerabilities privately — see [`SECURITY.md`](SECURITY.md).

## License

[Elastic License 2.0](LICENSE.md).
