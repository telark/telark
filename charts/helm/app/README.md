# telark-core-chart

Helm chart for the telark app: 6 first-class services + shared infra (redis, nats, kyverno, metrics-server, ollama).

## Values reference

### `app`

| Key | Default | Description |
|---|---|---|
| `app.name` | `telark` | Prefix for all resource names |
| `app.namespace` | `telark` | Install namespace; bootstrap CRs land here |
| `app.image.registry` | `botriack` | Container image registry |
| `app.image.repository` | `telark` | Container image repository |
| `app.kyverno.enabled` | `true` | Install kyverno subchart |
| `app.ollama.enabled` | `false` | Install ollama subchart |
| `app.persistence.enabled` | `true` | Provision exporter snapshot PVC |
| `app.persistence.storageClass` | `""` | `""` = cluster default; `"-"` = disable dynamic provisioning; `"<name>"` = explicit class |
| `app.persistence.size` | `1Gi` | PVC size |
| `app.persistence.accessMode` | `ReadWriteOnce` | PVC access mode |

#### `app.auth.bootstrap`

| Key | Default | Description |
|---|---|---|
| `app.auth.bootstrap.admins` | `["houssem.kraoua@gmail.com"]` | Admin email list. Each entry receives the Admin role on first OIDC login. Joined with commas → `BOOTSTRAP_ADMINS` env. Required when `app.auth.passkey.selfRegistration` is `"false"`. |

#### `app.auth.oidc`

Google OIDC config. Consumed by both the auth-service (env) and the bootstrap chart's `GlobalConfig` CR.

| Key | Default | Description |
|---|---|---|
| `app.auth.oidc.enabled` | `true` | Feature flag exposed via `GlobalConfig.spec.oidc.enabled`. |
| `app.auth.oidc.googleClientID` | `"286046819175-..."` | Google OAuth2 client id. Injected as `GOOGLE_CLIENT_ID` env and published in `GlobalConfig.spec.oidc.googleClientID`. |
| `app.auth.oidc.egressAllowed` | `"true"` | `"true"` = backend fetches Google JWKS dynamically (needs egress to `googleapis.com`). `"false"` = offline mode using `googleJwkJson`. |
| `app.auth.oidc.googleJwkJson` | `""` | Pasted content of `https://www.googleapis.com/oauth2/v3/certs`. Used only when `egressAllowed: "false"`. Rotate every 24–48h. |

#### `app.auth.passkey`

WebAuthn relying-party identity + passkey-flow policy. **`selfRegistration` gates only the passkey path** — OIDC users are always auto-provisioned.

| Key | Default | Description |
|---|---|---|
| `app.auth.passkey.id` | `"localhost"` | Relying Party identifier (eTLD+1 of the origin). Injected as `RP_ID`. |
| `app.auth.passkey.name` | `"Dashboard App"` | Display name shown by the authenticator (Touch ID prompt, etc.). Injected as `RP_NAME`. |
| `app.auth.passkey.origin` | `"http://localhost:3000"` | Origin accepted by the server for WebAuthn assertions. Browser enforces strictly. Injected as `RP_ORIGIN`. |
| `app.auth.passkey.selfRegistration` | `"true"` | `"false"` blocks new passkey registration. Requires at least one `app.auth.bootstrap.admins` entry when disabled. Injected as `SELF_REGISTRATION_ENABLED`. |

### `app.serviceDefaults`

Fallbacks for any `services.<svc>.*` key omitted.

| Key | Default | Description |
|---|---|---|
| `app.serviceDefaults.port` | `8080` | Container + Service port |
| `app.serviceDefaults.serviceType` | `ClusterIP` | K8s Service type |
| `app.serviceDefaults.replicas` | `1` | Deployment replicas |
| `app.serviceDefaults.terminationGracePeriodSec` | `60` | Pod termination grace period |

### `app.shared`

Pod-level config selectively applied via per-service gates.

**`app.shared.redis`** — mounted as `envFrom: configMapRef: <app.name>-redis-cm` when `useRedis: true` (default).

| Variable | Default | Description |
|---|---|---|
| `REDIS_HOST` | `telark-release-redis-master` | Redis service DNS name |
| `REDIS_PORT` | `6379` | Redis port |

**`app.shared.resources`** — applied when `includeResources: true` (default).

| Key | Default | Description |
|---|---|---|
| `requests.cpu` / `requests.memory` | `100m` / `128Mi` | Resource requests |
| `limits.cpu` / `limits.memory` | `500m` / `512Mi` | Resource limits |

**`app.shared.healthCheck`** — HTTP probes applied when `includeHealthCheck: true` (default). `port` is the named container port.

| Key | Default | Description |
|---|---|---|
| `port` | `http` | Named port for probe targets |
| `livenessProbe.path` | `/api/v1/status/live` | Liveness HTTP path |
| `readinessProbe.path` | `/api/v1/status/ready` | Readiness HTTP path |
| `*.initialDelaySeconds` / `periodSeconds` / `timeoutSeconds` | `15` | Probe timings |
| `*.failureThreshold` | `3` | Consecutive failures before unhealthy |

**`app.shared.security`** — pod + container securityContext, applied when `includeSecurity: true` (default).

| Key | Default | Description |
|---|---|---|
| `runAsUser` / `runAsGroup` / `fsGroup` | `1001` | Non-root identity |
| `allowPrivilegeEscalation` | `false` | Block setuid-style escalation |
| `readOnlyRootFilesystem` | `true` | Mount root FS read-only |
| `dropCapabilities` | `["ALL"]` | Linux capabilities to drop |

**`app.shared.natsEnvFromSecret`** — mounted when `useNatsCreds: true` (default false). Secret: `<app.name>-nats-secret`.

| Variable | Secret key | Description |
|---|---|---|
| `NATS_USER` | `username` | NATS auth user |
| `NATS_PASSWORD` | `password` | NATS auth password |

### `services.<svc>`

Per-service block. Gates default to `true` unless noted.

| Key | Default | Description |
|---|---|---|
| `enabled` | varies | Render this service's manifests |
| `name` | varies | K8s resource suffix |
| `imageTagPrefix` | varies | Image tag prefix; full tag = `<imageTagPrefix><version>` |
| `version` | varies | Image version segment |
| `category` | varies | Semantic label (pod label `type:<category>`) |
| `replicas` | `app.serviceDefaults.replicas` | Override |
| `port` | `app.serviceDefaults.port` | Override |
| `serviceType` | `app.serviceDefaults.serviceType` | Override |
| `terminationGracePeriodSec` | `app.serviceDefaults.terminationGracePeriodSec` | Override |
| `includeResources` | `true` | Apply `app.shared.resources` |
| `includeHealthCheck` | `true` | Apply `app.shared.healthCheck` |
| `includeSecurity` | `true` | Apply `app.shared.security` |
| `useRedis` | `true` | Mount `app.shared.redis` configmap |
| `useNatsCreds` | `false` | Mount `app.shared.natsEnvFromSecret` |
| `env` | `{}` | Inline env map; values pass through `tpl` against `.Values` |
| `envFromConfigMap` | `{}` | `valueFrom: configMapKeyRef` map (external configmap) |
| `envFromSecret` | `{}` | `valueFrom: secretKeyRef` map (secret `<app.name>-<name>-secret`) |
| `volumes` / `volumeMounts` | `[]` | Pod volumes + mounts |
| `pdb.enabled` | varies | PodDisruptionBudget |
| `topologySpread.*` | unset | TopologySpreadConstraints |

#### Service identities

| Service | `name` | `imageTagPrefix` | `version` | `category` | `enabled` |
|---|---|---|---|---|---|
| `exporter` | `exporter-service` | `exp-` | `3.3.1` | `export` | `true` |
| `discovery` | `discovery-service` | `discovery-` | `1.8.4` | `sync` | `true` |
| `enrichment` | `enrichment-service` | `enrich-` | `0.1.0` | `ai-enrichment` | `true` |
| `notifier` | `notifier-service` | `not-` | `0.2.1` | `notification` | `true` |
| `auth` | `auth-service` | `auth-` | `0.3.1` | `auth` | `true` |
| `ui` | `ui-service` | `ui-` | `0.0.1` | `ui` | `false` |

#### `services.exporter.env`

| Variable | Default | Description |
|---|---|---|
| `SNAPSHOTS_PATH` | `/snapshots` | Filesystem mount path for snapshot files |
| `SNAPSHOTS_PVC_NAME` | `{{ .Values.app.name }}-exporter-snapshots-pvc` (tpl) | PVC backing snapshot storage |
| `SNAPSHOTS_PVC_NAMESPACE` | `{{ .Values.app.namespace }}` (tpl) | Namespace of the snapshots PVC |
| `EXPORTER_K8S_CLIENT_QPS` | `50` | K8s client QPS; sized for CRD-write fanout (10× client-go default) |
| `EXPORTER_K8S_CLIENT_BURST` | `100` | K8s client burst |

`services.exporter.envFromConfigMap.CA_BUNDLE` → configmap `telark-ca-bundle`, key `ca.crt` (trusted CA bundle).

#### `services.discovery.env`

K8s client + informers:

| Variable | Default | Description |
|---|---|---|
| `DISCOVERY_K8S_CLIENT_QPS` | `100` | K8s client QPS; above 50/100 default to absorb snapshot LIST fanout on busy clusters |
| `DISCOVERY_K8S_CLIENT_BURST` | `200` | K8s client burst |
| `DISCOVERY_INFORMER_RESYNC_SEC` | `600` | Base informer resync interval |
| `DISCOVERY_ROLLBACK_INFORMER_RESYNC_SEC` | `600` | Rollback informer resync interval |
| `DISCOVERY_INFORMER_RESYNC_JITTER_FRACTION` | `0.2` | ±jitter applied per replica to spread resyncs and avoid LIST stampedes |
| `DISCOVERY_INFORMER_COALESCING_WINDOW_SEC` | `5` | Event coalescing window |
| `DISCOVERY_INFORMER_COALESCING_MAX_WAIT_SEC` | `10` | Max wait before a forced flush |
| `DISCOVERY_COALESCE_BUFFER_MAX_ENTRIES` | `500` | Coalescing buffer cap |
| `DISCOVERY_SNAPSHOT_FETCH_TIMEOUT_MS` | `5000` | Per-resource K8s GET deadline during snapshot assembly. Tune up on clusters with high apiserver tail latency. |

Coordination (leader election + claim queue):

| Variable | Default | Description |
|---|---|---|
| `COORDINATION_BATCH_SIZE` | `10` | Items processed per coordination batch |
| `COORDINATION_BATCH_BLOCK_SEC` | `2` | Max time to wait while filling a batch |
| `COORDINATION_MAX_RETRY_ATTEMPTS` | `5` | Max retries per claim |
| `COORDINATION_LOCK_TTL_SEC` | `120` | Distributed lock TTL |
| `COORDINATION_LOCK_HEARTBEAT_SEC` | `30` | Lock heartbeat interval |
| `COORDINATION_ELECTION_TTL_SEC` | `15` | Leader election lease TTL |
| `COORDINATION_ELECTION_RENEW_SEC` | `5` | Leader renewal interval |
| `COORDINATION_DEDUP_TTL_SEC` | `60` | Dedup key TTL |
| `COORDINATION_STALE_CLAIM_MIN_IDLE_SEC` | `300` | Min idle period before a claim is eligible for the stale sweep |
| `COORDINATION_STALE_CLAIM_INTERVAL_SEC` | `60` | Stale-claim sweep interval |
| `COORDINATION_SHUTDOWN_CLEANUP_TIMEOUT_SEC` | `45` | Graceful shutdown cleanup deadline |

Redis client (service-specific override of the shared defaults):

| Variable | Default | Description |
|---|---|---|
| `REDIS_RETRY_INTERVAL_SEC` | `5` | Retry backoff base |
| `REDIS_MAX_WAIT_SEC` | `180` | Total wait cap before failing the Redis op |
| `REDIS_PING_TIMEOUT_SEC` | `2` | Per-ping timeout |

Snapshot writer + protection plan:

| Variable | Default | Description |
|---|---|---|
| `SNAPSHOT_WRITE_MAX_ATTEMPTS` | `5` | Retries on snapshot write failure |
| `SNAPSHOT_WRITE_RETRY_INTERVAL_SEC` | `2` | Retry interval |
| `PROTECTION_PLAN_TICK_INTERVAL_SEC` | `31` | Protection-plan evaluator tick cadence |

Force-sync queue:

| Variable | Default | Description |
|---|---|---|
| `FORCE_SYNC_WORKERS` | `4` | Concurrent force-sync worker count |
| `FORCE_SYNC_STREAM_MAX_LEN` | `5000` | Redis stream length cap |
| `FORCE_SYNC_DEDUP_TTL_SEC` | `600` | Dedup key TTL |
| `FORCE_SYNC_JOB_TIMEOUT_SEC` | `300` | Per-job deadline |
| `FORCE_SYNC_MAINTENANCE_INTERVAL_SEC` | `60` | Maintenance loop interval |
| `FORCE_SYNC_PEL_IDLE_RECLAIM_SEC` | `60` | PEL idle threshold for reclaim |
| `FORCE_SYNC_ACK_RETENTION_SEC` | `3600` | Ack-id retention window |

Auto-cleanup of empty application CRDs:

| Variable | Default | Description |
|---|---|---|
| `DISCOVERY_AUTO_CLEANUP_ENABLED` | `"true"` | Master switch. `false` = detector goroutine never runs (zero overhead). |
| `DISCOVERY_AUTO_CLEANUP_DELETE_ENABLED` | `"false"` | Action gate. `false` = dry-run (logs intent only). `true` = destructive cleanup (Redis purge, snapshot dir removal, CRD delete). |
| `DISCOVERY_AUTO_CLEANUP_CYCLE_INTERVAL_SEC` | `300` | Detector wake interval |
| `DISCOVERY_AUTO_CLEANUP_EMPTY_CYCLES_REQUIRED` | `3` | Consecutive empty cycles before a CRD becomes cleanup-eligible |
| `DISCOVERY_AUTO_CLEANUP_GRACE_PERIOD_SEC` | `900` | Wall-clock floor from first-empty observation to eligibility |

Misc:

| Variable | Default | Description |
|---|---|---|
| `REST_EXPORTER_DURATION_LOG_ENABLED` | `"true"` | Log REST → exporter call durations |
| `REST_EXPORTER_DURATION_LOG_DEDUP_SEC` | `10` | Dedup window for the duration logs |

#### `services.enrichment.env`

The AI **provider** and **API key** are not env vars: an admin sets them at runtime from the UI, and they are stored in the GlobalConfig CR. Only the non-secret model names live here.

| Variable | Default | Description |
|---|---|---|
| `REDIS_POOL_SIZE` | `10` | Redis client connection pool size |
| `OLLAMA_HOST` | `http://telark-release-ollama:11434` | Ollama base URL (used when provider = `ollama`) |
| `OLLAMA_MODEL` | `qwen2.5:3b` | Ollama model tag |
| `ANTHROPIC_MODEL` | `claude-haiku-4-5-20251001` | Anthropic model id |
| `GROQ_MODEL` | `llama-3.3-70b-versatile` | Groq model id |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Gemini model id |
| `NUM_WORKERS` | `3` | Concurrent worker goroutines (1 for ollama, 3 for cloud providers) |
| `METRICS_INTERVAL_S` | `60` | Metrics emit cadence (seconds) |
| `WORKER_SHUTDOWN_TIMEOUT_S` | `30` | Graceful worker shutdown deadline (seconds) |

#### `services.notifier.env`

No service-specific env. Inherits `app.shared.redis` and `app.shared.natsEnvFromSecret`.

#### `services.auth.env`

Redis:

| Variable | Default | Description |
|---|---|---|
| `REDIS_DB` | `1` | Redis DB index |
| `REDIS_RETRY_INTERVAL_SEC` | `2` | Retry backoff base |
| `REDIS_MAX_WAIT_SEC` | `30` | Total wait cap before failing the Redis op |
| `REDIS_PING_TIMEOUT_SEC` | `3` | Per-ping timeout |

Bootstrap (templated from `app.auth.bootstrap`):

| Variable | Source | Description |
|---|---|---|
| `BOOTSTRAP_ADMINS` | `{{ join "," .Values.app.auth.bootstrap.admins }}` | Comma-joined admin email list; recipients get Admin role on first OIDC login |

WebAuthn / passkey (templated from `app.auth.passkey`):

| Variable | Source | Description |
|---|---|---|
| `RP_ID` | `{{ .Values.app.auth.passkey.id }}` | Relying Party identifier (origin host) |
| `RP_NAME` | `{{ .Values.app.auth.passkey.name }}` | Display name shown to the user |
| `RP_ORIGIN` | `{{ .Values.app.auth.passkey.origin }}` | Allowed origin |
| `SELF_REGISTRATION_ENABLED` | `{{ .Values.app.auth.passkey.selfRegistration }}` | `"false"` blocks new passkey registration. OIDC self-provisioning is always on. |
| `CHALLENGE_TIMEOUT` | inline (`"60"`) | Challenge TTL (seconds) |
| `SESSION_EXPIRY` | inline (`"24"`) | Session TTL (hours) |

Google OIDC (templated from `app.auth.oidc`):

| Variable | Source | Description |
|---|---|---|
| `GOOGLE_CLIENT_ID` | `{{ .Values.app.auth.oidc.googleClientID }}` | Google OAuth client id |
| `EGRESS_ALLOWED` | `{{ .Values.app.auth.oidc.egressAllowed }}` | `"true"` = backend fetches JWKS dynamically (needs egress to googleapis.com). `"false"` = offline mode using `GOOGLE_OIDC_JWK_JSON`. |
| `GOOGLE_OIDC_JWK_JSON` | `{{ .Values.app.auth.oidc.googleJwkJson }}` | Pasted JWK content for offline mode (rotate every 24–48h) |

Cleanup controllers + queue:

| Variable | Default | Description |
|---|---|---|
| `RECONCILE_TICK_SECONDS` | `5` | Reconciler tick interval |
| `RECONCILE_PASS_DEADLINE_SECONDS` | `30` | Per-pass deadline |
| `CLEANUP_WORKERS_PER_TYPE` | `2` | Workers per cleanup type |
| `CLEANUP_STREAM_MAXLEN` | `10000` | Cleanup stream length cap |
| `CLEANUP_LAG_ALERT_THRESHOLD` | `500` | Lag threshold for alerts |
| `CLEANUP_SWEEPER_INTERVAL_SECONDS` | `60` | Sweeper interval |
| `CLEANUP_JOB_MAX_ATTEMPTS` | `5` | Max retries per cleanup job |
| `CLEANUP_DEDUP_TTL_SECONDS` | `600` | Dedup key TTL |
| `CLEANUP_XCLAIM_MIN_IDLE_SECONDS` | `60` | XCLAIM min idle threshold |
| `CLEANUP_LIST_TIMEOUT_SECONDS` | `10` | List op timeout |
| `CLEANUP_PATCH_TIMEOUT_SECONDS` | `5` | Patch op timeout |
| `CLEANUP_MAX_CONCURRENT_PATCHES` | `4` | Concurrent patch cap |
| `RECONCILE_BACKOFF_INITIAL_SECONDS` | `5` | Initial backoff for failed reconciles |
| `RECONCILE_BACKOFF_MAX_SECONDS` | `300` | Max backoff cap |

#### `services.ui.env`

No env. Disabled by default.
