# exporter-service k6 stress plan

Single reference. Route inventory, wire format, thresholds, business 4xx, stage definitions.

## Service basics

- Path prefix: `/api/v1/` (`rest/router/def.go:34`, `rest/base/def.go` `V1="api/v1"`).
- In-cluster DNS: `telark-exporter-service.telark.svc.cluster.local:8080`.
- Server timeouts: `ReadTimeout=30s`, `WriteTimeout=30s`, `IdleTimeout=120s`. Body cap 1MB. Port 8080.
- No auth middleware. CORS only.
- All bodies parsed as `map[string]any` via `json.Unmarshal`. Send `Content-Type: application/json`.
- Service-side global backpressure: `k8sCreateSem = chan struct{}, 10` (`exporters/generics/create.go:25,29`). Caps concurrent CRD creates.

## Scope

**In-scope** (51 routes — see inventory below).

**Out of scope**:
- Notifications (R8–R12), challenges (R50–R52).
- Routes needing `X-User-ID` or a passkey `{credentialId}` (R3, R6, R58–R62).
- R18 rollback-by-id, R27 OIDC identity, R64/R65 snapshot get/manifest — need seeds not in code.
- soak activation (skeleton).

---

## Route inventory

`Class` codes drive threshold buckets below.

| ID | Method | Path | Handler | Class |
|---|---|---|---|---|
| R1 | GET | `/status/live` | `status/health.go:14` | probe |
| R2 | GET | `/status/ready` | `status/health.go:14` | probe |
| R4 | GET | `/protectionplans` | `plans/protection/handler.go` | list |
| R5 | GET | `/protectionplans/{id}` | `plans/protection/handler.go` | get-cached |
| R7 | DELETE | `/protectionplans/{id}` | `plans/protection/handler.go` | delete |
| R13 | GET | `/config` | `config/handler.go` | get |
| R14 | PATCH | `/config` | `config/handler.go` | patch |
| R15 | POST | `/applications` | `resources/application/handler.go` | create |
| R16 | GET | `/applications` | `resources/application/handler.go` | list |
| R17 | GET | `/applications/{name}/rollbacks` | `resources/application/handler.go` | get |
| R18 | GET | `/applications/{name}/rollbacks/{rollbackId}` | `resources/application/handler.go` | get |
| R19 | GET | `/applications/{name}` | `resources/application/handler.go` | get-cached |
| R20 | PATCH | `/applications/{name}` | `resources/application/handler.go` | patch |
| R21 | DELETE | `/applications/{name}` | `resources/application/handler.go` | delete |
| R22 | POST | `/users` | `resources/user/handler.go` | create |
| R23 | GET | `/users` | `resources/user/handler.go` | list |
| R24 | GET | `/internal/users/by-username/{username}` | `resources/user/handler.go` | get-cached |
| R25 | GET | `/internal/users/by-email/{email}` | `resources/user/handler.go` | get-cached |
| R26 | GET | `/users/{id}` | `resources/user/handler.go` | get-cached |
| R27 | GET | `/internal/users/by-identity` | `resources/user/handler.go` | get |
| R28 | PATCH | `/users/{id}` | `resources/user/handler.go` | patch |
| R29 | DELETE | `/users/{id}` | `resources/user/handler.go` | delete |
| R30 | POST | `/groups` | `resources/group/handler.go` | create |
| R31 | GET | `/groups` | `resources/group/handler.go` | list |
| R32 | GET | `/groups/{id}` | `resources/group/handler.go` | get-cached |
| R33 | PATCH | `/groups/{id}` | `resources/group/handler.go` | patch |
| R34 | DELETE | `/groups/{id}` | `resources/group/handler.go` | delete |
| R35 | POST | `/accessroles` | `resources/role/handler.go` | create |
| R36 | GET | `/accessroles` | `resources/role/handler.go` | list |
| R37 | GET | `/accessroles/{id}` | `resources/role/handler.go` | get-cached |
| R42 | PATCH | `/accessroles/{id}` | `resources/role/handler.go` | patch |
| R43 | DELETE | `/accessroles/{id}` | `resources/role/handler.go` | delete |
| R44 | POST | `/categories` | `categories/handler.go` | create |
| R45 | GET | `/categories` | `categories/handler.go` | list |
| R46 | GET | `/categories/{id}` | `categories/handler.go` | get-cached |
| R47 | GET | `/categories?scope={scope}` | `categories/handler.go` | list |
| R48 | PATCH | `/categories/{id}` | `categories/handler.go` | patch |
| R49 | DELETE | `/categories/{id}` | `categories/handler.go` | delete |
| R53 | POST | `/internal/auth/users/{userId}/sessions` | `auth/session/handler.go` | create |
| R54 | GET | `/auth/sessions?user={userId}` | `auth/session/handler.go` | list |
| R55 | GET | `/auth/sessions/self` | `auth/session/handler.go` | get |
| R56 | PATCH | `/auth/sessions/self` | `auth/session/handler.go` | patch |
| R57 | DELETE | `/auth/sessions/self` | `auth/session/handler.go` | delete |
| R63 | POST | `/internal/snapshots` | `snapshot/handler.go` | create-fs |
| R64 | GET | `/snapshots/{id}` | `snapshot/handler.go` | get |
| R65 | GET | `/snapshots/{id}/manifest` | `snapshot/handler.go` | get |
| R66 | GET | `/snapshots` | `snapshot/handler.go` | list-fs |

## Path / query / header inputs

| Param | Routes | Source constant |
|---|---|---|
| `{id}` | R5, R7, R26, R28, R29, R32, R33, R34, R37, R42, R43, R46, R48, R49, R64, R65 | `constants.IDParam` |
| `{name}` | R17–R21 | `constants.NameParam` |
| `{username}` | R24 | `constants.UsernameParam` |
| `{email}` | R25 | `constants.EmailParam` |
| `{userId}` | R53 | `constants.UserIDParam` |
| query `user` | R54 | `authendpoints.QuerySessionUser` |
| header `X-Session-Token` | R55, R56, R57 | raw token or session name |
| query `scope` | R47 | `categoryendpoints.QueryScope` |
| `{rollbackId}` | R18 | literal |
| query `provider/issuer/subject` | R27 | – |
| query `scope/namespace/generation` | R64, R65 | – |
| header `Accept` | R65 | – |

No `X-User-ID` / passkey `{credentialId}` routes in scope.

## Body shapes (write routes)

All sent with `Content-Type: application/json`. Examples reflect struct + hand-rolled validation (no `validate:"..."` tags exist).

```js
// R14 PatchConfig
{ userSettings: { fetchIntervalSeconds:60 } }

// R15 CreateApplication
{ name:'app-k6-<uuid>', displayName:'app-k6', namespaces:{ primary:'default' }, managed:{ by:'k6' } }

// R20 PatchApplication
{ displayName:'app-k6-updated' }

// R22 CreateUser (UserStatus.Phase = active|inactive|suspended)
{ id, username, fullname:'k6 user', email, status:{ phase:'active' }, creationDate }

// R28 PatchUser
{ fullname:'updated' }

// R30 CreateGroup
{ id, name, description:'', categoryRef:'', userRefs:[], roleRefs:[], creationDate }

// R33 PatchGroup
{ description:'updated' }

// R35 CreateRole (RoleType = built-in|custom; RoleStatus.Phase = Active|Inactive|Deprecated|Deleted)
{ id, name, description:'', version:'1.0', type:'custom', priority:1, categoryRef:'',
  scopesAndPermissions:[], status:{ phase:'Active' }, creationDate }

// R42 PatchRole
{ description:'updated' }

// R44 CreateCategory (CategoryType = built-in|custom)
{ id, name, scope:'global', type:'custom', creationDate }

// R48 PatchCategory
{ name:'renamed' }

// R53 CreateSession
{ userId, sessionToken:'sess-<uuid>', createdTimestamp, expiresTimestamp, deviceMetadata:{} }

// R56 PatchSession
{ expiresTimestamp:'<iso+2h>' }

// R63 CreateSnapshot (manifest is interface{} — default stub)
{ id, scope:'apps', namespace:'default', generation:1,
  manifest:{ apiVersion:'v1', kind:'ConfigMap', metadata:{ name:'k6-stub', namespace:'default' }, data:{} } }
```

---

## Business 4xx (not counted as failures)

| Code | Routes | Reason |
|---|---|---|
| 404 | all reads with seed IDs (R5, R7, R17–R19, R24–R27, R32, R37, R46, R55, R64, R65) | seed miss — expected |
| 409 | all patches (R14, R20, R28, R33, R42, R48, R56) | K8s `resourceVersion` conflict; `ConflictStatus=409` in `constants/config.go` |
| 503 | all creates (R15, R22, R30, R35, R44, R53, R63) — **stress only** | `k8sCreateSem=10` saturation |

All other 4xx + unexpected 5xx = `real_failures` Rate.

---

## Per-class threshold table

Tagged `{route, scenario}`. Journey applies an additional `p95 ≤ 500ms` cap.

| Class | p95 | p99 |
|---|---|---|
| probe | 50ms | 100ms |
| list | 500ms | 1s |
| list-fs | 1s | 2s |
| get-cached | 200ms | 400ms |
| get | 400ms | 800ms |
| create | 800ms | 1.5s |
| create-fs | 3s | 6s |
| patch | 600ms | 1.2s |
| delete | 600ms | 1.2s |

Aggregate error rate: < 0.5% load, < 5% stress, < 0.1% journey/soak.

---

## Coverage matrix

✓ = exercised. – = skipped. J# = journey step.

| Route(s) | smoke | load | stress | spike | journey | soak |
|---|---|---|---|---|---|---|
| R1, R2 | ✓ | ✓ | – | – | – | ✓ |
| R4 | ✓ | ✓ | ✓ | ✓ | – | ✓ |
| R5 | ✓ | ✓ | ✓ | – | – | ✓ |
| R7 | ✓ | – | – | – | – | – |
| R13 | ✓ | ✓ | – | – | – | ✓ |
| R14 | ✓ | – | – | – | – | – |
| R15 | ✓ | ✓ | ✓ | ✓ | – | – |
| R16, R19 | ✓ | ✓ | ✓ | ✓ | – | ✓ |
| R17 | ✓ | – | – | – | – | – |
| R18 | – | – | – | – | – | – |
| R20, R21 | ✓ | – | – | – | – | – |
| R22 | ✓ | ✓ | ✓ | – | J2 | – |
| R23 | ✓ | ✓ | ✓ | ✓ | – | ✓ |
| R24, R25 | ✓ | ✓ | – | – | – | ✓ |
| R26 | ✓ | ✓ | ✓ | – | J2 | ✓ |
| R27 | – | – | – | – | – | – |
| R28 | ✓ | – | – | – | J2 | – |
| R29 | ✓ | – | – | – | – | – |
| R30 | ✓ | ✓ | – | – | J2 | – |
| R31 | ✓ | ✓ | ✓ | ✓ | – | ✓ |
| R32 | ✓ | ✓ | – | – | – | ✓ |
| R33, R34 | ✓ | – | – | – | – | – |
| R35 | ✓ | ✓ | – | – | J2 | – |
| R36 | ✓ | ✓ | ✓ | – | – | ✓ |
| R37 | ✓ | ✓ | – | – | – | ✓ |
| R42, R43 | ✓ | – | – | – | – | – |
| R44 | ✓ | ✓ | – | – | – | – |
| R45 | ✓ | ✓ | ✓ | – | – | ✓ |
| R46 | ✓ | ✓ | – | – | – | ✓ |
| R47–R49 | ✓ | – | – | – | – | – |
| R53 | ✓ | – | – | – | J3 | – |
| R54 | ✓ | – | – | – | – | – |
| R55 | ✓ | – | – | – | J3 | – |
| R56 | ✓ | – | – | – | – | – |
| R57 | ✓ | – | – | – | J3 | – |
| R63 | ✓ | – | – | – | – | – |
| R64, R65 | – | – | – | – | – | – |
| R66 | ✓ | ✓ | – | – | – | – |

---

## Journeys

| ID | Flow |
|---|---|
| J2 | R22 → R30 → R35 → R28 → R26 (user + group + role assignment) |
| J3 | R53 → R55 → R57 (session lifecycle; capture `sessionToken` from R53 response and send it as `X-Session-Token`) |

---

## Stage definitions (locked per task spec)

**smoke** (`per-vu-iterations`, 1 VU × 1 iter, `abortOnFail:true`).

**load** (`ramping-arrival-rate`):
- ramp 0 → BASELINE_RPS (1m), hold BASELINE_RPS (5m), ramp → 0 (1m).

**stress** (`ramping-arrival-rate`, no `sleep()`):
- ramp 0 → 500 (1m), hold 500 (1m), ramp → 2000 (2m), ramp → 5000 (2m), ramp → MAX_RPS (2m), ramp → 0 (2m).
- If thresholds hold at MAX_RPS → log `CEILING NOT REACHED`.

**spike** (`ramping-arrival-rate`, no `sleep()`):
- hold 500 (1m), ramp → 5000 (10s), hold 5000 (1m), drop → 500 (10s), hold 500 (2m).
- Recovery = time from drop until p95 ≤ 1s at baseline.

**journey** (`constant-arrival-rate`):
- 50 RPS × 5 min. Per-step p95 < 500ms.

**soak** (`constant-arrival-rate`, **skeleton**):
- 500 RPS × 30 min. Commented; enable for off-hours.

VU pool: `preAllocatedVUs = ceil(target_rps × 1.5)`, `maxVUs = 2 × pre` (`lib/vu.js`).

---

## Env vars

| Var | Default | Effect |
|---|---|---|
| `BASE_URL` | `http://telark-exporter-service.telark.svc.cluster.local:8080` | service URL |
| `BASELINE_RPS` | `500` | load + soak target |
| `MAX_RPS` | `10000` | stress peak |
| `SEED_POOL_SIZE` | `1000` | SharedArray pool per resource |
| `SEED_PREFIX` | `k6-<run>` | ID prefix; auto-unique per run |
| `RESULTS_DIR` | `k6/results` | handleSummary JSON output |

---

## Saturation + ceiling

- `LOAD GENERATOR SATURATED` — achieved RPS < target × 0.95.
- `CEILING NOT REACHED — raise MAX_RPS above N and rerun` — stress only, thresholds held + no saturation at MAX_RPS.
- Service-side hard cap on creates: `k8sCreateSem=10` → ~100 creates/sec at 100ms K8s latency.
