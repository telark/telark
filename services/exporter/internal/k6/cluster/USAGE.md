# k6 in-cluster runner — usage

## One command

```sh
cd /Users/houssem/Desktop/Github/exporter-service
k6/cluster/run.sh <scenario>
```

Each invocation: builds a per-run ConfigMap → creates Job → streams logs → saves results → **deletes both Job and ConfigMap**. Nothing persists in the cluster between runs.

Results land in `k6/results/<run>.log` + `k6/results/<run>-json/`.

## Scenarios

### `smoke`
Hits every in-scope route once, with the real wire format. Use as a pre-flight check before any other scenario. Fails fast (`abortOnFail: true`) on any threshold breach. ~30 seconds.

```sh
k6/cluster/run.sh smoke
```

### `load`
Holds 500 RPS for 5 minutes (1 min ramp up, 5 min hold, 1 min ramp down). Mix of cached reads + writes. Use to confirm steady-state behavior at expected production load. ~7 minutes.

```sh
k6/cluster/run.sh load
```

### `stress`
Climbs through 500 → 2000 → 5000 → MAX_RPS. Finds the service ceiling. Use to discover which route breaks first under sustained pressure. ≤10 minutes.

```sh
k6/cluster/run.sh stress
MAX_RPS=10000 k6/cluster/run.sh stress   # raise peak when defaults don't break anything
```

If logs say `CEILING NOT REACHED` → bump `MAX_RPS` and rerun.

### `spike`
Hold 500 RPS, slam to 5000 in 10s, hold 1 min, drop back to 500. Measures burst behavior and recovery time. ~4.5 minutes.

```sh
k6/cluster/run.sh spike
```

### `journey`
Runs end-to-end user flows (J2 create-user-group-role, J3 session lifecycle) at 50 RPS for 5 minutes. Stricter per-step p95 budget (500ms). Use to confirm multi-step flows hold under modest load.

```sh
k6/cluster/run.sh journey
```

### `soak`
30-minute sustained 500 RPS reads. Watches for memory leaks and lock-map growth. **Skeleton only** — open `k6/scenarios/soak.js` and uncomment the `scenarios.soak` block before first run. Off-hours.

```sh
WAIT_TIMEOUT=45m k6/cluster/run.sh soak
```

## Recommended order

```sh
k6/cluster/run.sh smoke    # 1. confirm everything works
k6/cluster/run.sh load     # 2. baseline
k6/cluster/run.sh stress   # 3. ceiling
k6/cluster/run.sh spike    # 4. burst
k6/cluster/run.sh journey  # 5. flows
```

## Env overrides

| Var | Default | When to override |
|---|---|---|
| `MAX_RPS` | `2000` | Stress shows `CEILING NOT REACHED` |
| `BASELINE_RPS` | `500` | Want different sustained load |
| `BASE_URL` | `http://telark-exporter-service.telark.svc.cluster.local:8080` | Different namespace or service |
| `SEED_POOL_SIZE` | `1000` | Larger pool for less ID rotation |
| `SEED_PREFIX` | auto: `k6-<run>` | Group artifacts across runs |
| `WAIT_TIMEOUT` | `30m` | Soak needs longer |
| `IMAGE` | `grafana/k6:latest` | Private registry mirror |

## Aborting

Ctrl-C in the terminal. Job is deleted automatically via `trap`.

## Cleanup CRDs after stress runs

Every synthesized resource starts with `k6-`. Bulk delete:

```sh
for kind in protectionplans users groups accessroles categories applications sessions; do
  kubectl -n telark get "$kind.telark.io" -o name 2>/dev/null | grep '/k6-' \
    | xargs -r kubectl -n telark delete
done
```
