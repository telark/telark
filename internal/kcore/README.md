# kcore

Kubernetes building blocks for the [telark](https://telark.io) platform. Dynamic informers, CRD helpers, metrics, and health — the k8s plumbing services share instead of re-implementing.

## Packages

| Package | What it provides |
|---|---|
| `informers` | Dynamic informers for watching and reacting to k8s resources |
| `crds` | Custom-resource helpers and utilities |
| `resources` | Resource adapters and typed access |
| `metrics` | Metrics collection over the k8s metrics APIs |
| `health` | Health and reachability checks |
| `manifest` | Manifest parsing and helpers |
| `shared` | Shared helpers |
| `constants` | Constants and formatting helpers |

## Install

```sh
export GOPRIVATE=github.com/telark/*   # private until public release
go get github.com/telark/kcore
```

Consumed by the telark services (notably `discovery` and `exporter`).
