# exporter-service k6 stress suite

Six scenarios. One shared library. Runs as in-cluster K8s Job in `telark` namespace.

Full spec: [`TEST_PLAN.md`](./TEST_PLAN.md). Runner usage: [`cluster/USAGE.md`](./cluster/USAGE.md).

## Layout

```
k6/
├── TEST_PLAN.md         route inventory, wire format, business 4xx, thresholds
├── README.md            this file
├── config/
│   ├── data.js          seed pools + payload generators
│   └── thresholds.js    per-route + per-scenario budgets
├── lib/
│   ├── routes.js        single source of truth for paths
│   ├── http.js          tagged request, real_failures Rate, 4xx classifier
│   ├── checks.js        response checks, JSON capture
│   ├── summary.js       handleSummary, table + JSON output
│   └── vu.js            VU pool helpers
├── scenarios/           smoke|load|stress|spike|journey|soak
├── cluster/
│   ├── USAGE.md         per-scenario run instructions
│   ├── job.yaml         Job manifest template
│   └── run.sh           per-run: build CM → apply Job → stream logs → copy results → delete
└── results/             handleSummary JSON + per-run logs (gitignored)
```

## Run

```sh
cd Desktop/Github/exporter-service

k6/cluster/run.sh smoke      # ~30s    — pre-flight wire-format check
k6/cluster/run.sh load       # ~7 min  — baseline @ 500 RPS
k6/cluster/run.sh stress     # ≤10 min — ramp to MAX_RPS, find ceiling
k6/cluster/run.sh spike      # ~4.5 min — burst + recovery
k6/cluster/run.sh journey    # 5 min   — end-to-end flows @ 50 RPS
```

Each invocation: builds per-run ConfigMap → applies Job → streams logs to `k6/results/<run>.log` → copies result JSON to `k6/results/<run>-json/` → deletes Job and ConfigMap. Nothing persists in the cluster between runs.

Direct local invocation (not recommended; port-forward chokes under load):
```sh
BASE_URL=http://localhost:8002 k6 run k6/scenarios/smoke.js
```

## Env vars

| Var | Default | Effect |
|---|---|---|
| `BASE_URL` | `http://telark-exporter-service.telark.svc.cluster.local:8080` | service URL |
| `BASELINE_RPS` | `500` | load + soak target |
| `MAX_RPS` | `2000` | stress peak (raise when CEILING NOT REACHED fires) |
| `SEED_POOL_SIZE` | `1000` | SharedArray pool per resource |
| `SEED_PREFIX` | auto: `k6-<run>` | ID prefix |
| `WAIT_TIMEOUT` | `30m` | runner timeout per Job |
| `IMAGE` | `grafana/k6:latest` | k6 container image |

Example:
```sh
MAX_RPS=10000 k6/cluster/run.sh stress
```

## Scope

In-scope: 51 routes (full inventory in [`TEST_PLAN.md`](./TEST_PLAN.md)).

Out of scope: notifications (R8–R12), challenges (R50–R52), routes needing `X-User-ID` or a passkey credential id (R3, R6, R58–R62), R18/R27/R64/R65 (need external seeds), soak activation.

## Watch for in stdout

- `*** LOAD GENERATOR SATURATED ***` — achieved RPS < target × 0.95.
- `*** BREACHED THRESHOLDS: N ***` — N route p95/p99 breaches; check sorted table.
- `*** CEILING NOT REACHED — raise MAX_RPS ***` — stress only; bump `MAX_RPS` and rerun.
- 503 spikes on creates during stress = `k8sCreateSem=10` saturation (documented business 4xx).

## Validation

Each scenario validated via `k6 inspect`. Adding `k6/` does not break `go build ./...` or `golangci-lint run` for exporter-service.

## Cleanup CRD residue after stress

Every synthesized resource starts with `k6-`:
```sh
for kind in protectionplans users groups accessroles categories applications sessions; do
  kubectl -n telark get "$kind.telark.io" -o name 2>/dev/null | grep '/k6-' \
    | xargs -r kubectl -n telark delete
done
```
