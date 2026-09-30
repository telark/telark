# kcore

Kubernetes building blocks shared by the [Telark](https://github.com/telark/telark) services: client setup, dynamic informers, custom-resource reads and writes (including the status subresource), manifests for snapshots and rollback, and metrics.

An internal package of the Telark Go module (`github.com/telark/telark/internal/kcore`): only the Telark services import it, and it changes together with them.

| Package | Contents |
|---|---|
| `k8sclient` | In-cluster clients with rate limits |
| `informers` | Dynamic informers and listers |
| `crds` | Custom-resource CRUD, status subresource writes, and `view` (API view of a CR: spec, projected status, `id` from `metadata.name`) |
| `manifest` | Fetch and clean manifests for snapshots and rollback |
| `resources`, `ops` | Typed access to core Kubernetes resources and write operations |
| `metrics` | Pod metrics from metrics-server |
| `resilience`, `health`, `shared`, `constants` | Retries, worker pools, health checks, helpers |

Depends on [`data`](../data).
