# Security model

How telark authenticates people and services, authorizes each request, and what it may do in the cluster. Every statement points at the code that implements it; when the code changes, this page changes in the same diff. Vulnerability reporting is in [SECURITY.md](../../SECURITY.md).

Paths starting `x-ware/`, `data/` or `rest/` are in the shared Go modules `github.com/telark/{x-ware,data,rest}`, which live in their own repositories (pinned in each service's `go.mod`).

## Trust boundaries

| Boundary | Credential | Enforced by |
|---|---|---|
| Browser → auth, discovery, exporter, analyzer | session token in the `X-Session-Token` header | `x-ware/authz/handler.go` in every Go service; `services/analyzer/authz.py` |
| Service → service | shared service token in the `X-Service-Token` header | the same middleware (constant-time compare) |
| Service → Kubernetes API | the service's own ServiceAccount | chart RBAC, see [Kubernetes privileges](#kubernetes-privileges) |
| Anyone → telark CRDs directly | Kubernetes identity | optional CRD write guard (`app.crdGuard`), off by default |
| Workload changes → cluster | Kubernetes identity | Kyverno policies rendered from active protection plans |
| Services → Redis | none (Redis runs without auth) | treated as untrusted: authorization cache entries are HMAC-signed |
| discovery, notifier → NATS | user and password from a chart-generated Secret | NATS server authorization in `charts/telark/values.yaml` (`nats:`) |
| analyzer → Ollama | none | `templates/shared/ollama-networkpolicy.yaml`: only analyzer pods reach port 11434 |
| auth → Google (optional) | none; Google's JWKS verifies ID tokens | only when `GlobalConfig.oidc.egressAllowed` is set |

The chart exposes only the UI through Ingress or HTTPRoute (`templates/shared/ingress.yaml`, `httproute.yaml`: path `/` to `services.ui`). How the UI reaches the service APIs is defined in the separate dashboard-ui repository.

## Authentication

Unprefixed paths in this section are under `services/auth/internal/`.

### Sessions

- Created only by `CreateUserSession` (`services/auth/internal/helpers/auth/session.go`), after a passkey login (`handlers/auth/login.go`) or an OIDC callback (`handlers/oidc/callback.go`). Registration alone doesn't create one, and a non-active account gets none.
- Token: 32 bytes from `crypto/rand`, base64url (`services/auth/internal/helpers/shared/token.go`). Lifetime `SESSION_EXPIRY` hours (default 24, `services/auth/internal/constants/config.go`), with no sliding refresh.
- Stored as a `UserSession` CR named `session-<sha256(token)>` (`data/auth/session.go`); the exporter blanks the raw token before persisting (`services/exporter/internal/utils/auth/session/extract.go`). The raw token is never stored and never appears in a URL: clients address a session by that name or by `self`, and the exporter rejects a raw token in the path (`services/exporter/internal/utils/auth/session/name.go`).
- Sent only in the `X-Session-Token` header (`data/constants/headers.go`); no cookies, no query parameter.
- Logout deletes the session and always answers 200 (`handlers/auth/logout.go`). Expired sessions are purged per user whenever that user gets a new session (`services/exporter/internal/utils/auth/session/cleanup.go`); there is no global sweeper.

### Passkeys (WebAuthn)

- Library `github.com/go-webauthn/webauthn` (`services/auth/go.mod`). Relying-party settings `RP_ID`, `RP_ORIGIN` (comma-separated), `RP_NAME` and `CHALLENGE_TIMEOUT` (`services/auth/internal/config/config.go`). When `RP_ID` or `RP_ORIGIN` is empty, they are derived from the request's forwarded host, host or origin headers (`helpers/webauthn/webauthn.go`); production installs should set both (`app.auth.passkey.id` and `origin` in `charts/telark/values.yaml`).
- Challenges live in Redis with a short TTL; enrolment links are single-use (Redis `GETDEL`, `helpers/auth/enroll.go`). Credentials are `UserPasskey` CRs written through the exporter's Internal passkey routes.
- Browsers enable WebAuthn only on `https://` or `http://localhost` origins ([INSTALL.md](../INSTALL.md#access-the-dashboard)).
- Self-registration is controlled by `SELF_REGISTRATION_ENABLED`; the auth service refuses to start when it is off and no bootstrap admin is configured (`services/auth/internal/config/bootstrap.go`).

### OIDC (Google)

- Configured at runtime in the `GlobalConfig` CR (`oidc`: enabled, client ID, egress switch, optional JWK set), re-read on every callback. See [services/auth/OIDC.md](../../services/auth/OIDC.md).
- The UI posts a Google ID token. auth verifies the RSA signature locally against Google's keys (fetched and cached in Redis when egress is allowed, otherwise the configured JWK set), `aud` = client ID, `iss` = `https://accounts.google.com`, `exp`, a single-use nonce from Redis, and `email_verified` (`helpers/oidc/google.go`, `helpers/oidc/nonce.go`, `handlers/oidc/callback.go`). There is no code exchange and no client secret.
- Users are matched by provider, issuer and subject, then by a unique email; an ambiguous email is refused with 409, and an unknown one creates the user (`handlers/oidc/provision.go`). OIDC provisioning is not gated by `SELF_REGISTRATION_ENABLED`.

### Service token

- One shared secret per install: Secret `telark-service-token-secret`, key `token`, from `app.serviceToken.value`, else the existing Secret (Helm `lookup`), else 48 random characters (`charts/telark/templates/shared/service-token-secret.yaml`). Every deployment receives it as `TELARK_SERVICE_TOKEN` (`templates/_deployment.tpl`).
- A service refuses to start without it (`x-ware/authz/bootstrap.go`). Outgoing `rest` clients attach it as `X-Service-Token` (`rest/clients/shared/servicetoken.go`).
- A request with a valid service token is Internal: it passes every route, scoped ones included, without a grant check, and the caller-supplied `X-User-ID` is trusted as the acting user (`x-ware/authz/handler.go`, `identify`). A wrong token is 401. The token is therefore equivalent to full API access.
- A `helm template` render without cluster access (GitOps) generates a new token each time unless `app.serviceToken.value` is set (`values.yaml`, `app.serviceToken`).

## Authorization

### Middleware (`x-ware/authz`)

Every Go service with an API wraps its router with the same middleware (`main.go` of auth, discovery and exporter; notifier serves only unauthenticated probes). Per request, in `handler.go`:

1. Read the claimed `X-User-ID`, then strip `X-User-ID`, `X-Username` and `X-Email` from the request (`constants.go`), so a client can't inject an identity.
2. Look up the route (method + mux path template, `rest/router`). A route without an entry in the service's `internal/authz/requirements.go` is **403**.
3. `Public` routes pass.
4. A present `X-Service-Token` must match (else 401) and makes the caller Internal (see above). An `Internal` route without it is 401.
5. Otherwise `X-Session-Token` → user id → grants. `Authenticated` routes stop here; scoped routes then need `Allows`.
6. Only a resolver verdict (not found, gone, session expired, user not active) produces 401/403; any other resolver error is 503, so an outage never looks like a logout (`denialFor`).

Requirement helpers (`x-ware/authz/requirement.go`): `Read` = ReadOnly, `Write` = Contributor, `Own` = Owner, `Administer` = Admin on a scope; `Denyable(req, action)` adds a deny-rule key. Each API service's `internal/tests/authz/requirements_test.go` fails when a route has no requirement (`TestRequirementsCoverEveryRoute`) or when a route becomes Public unexpectedly (`TestOnlyProbesArePublic` in discovery and exporter, `TestPublicRoutesArePinned` in auth).

### Grants (`x-ware/authz/grants.go`, `allow.go`)

- Levels (`data/resources/role/def.go`): ReadOnly 1 < Contributor 2 < Owner 3 < Admin 4. A level covers every lower one; an unknown level covers nothing.
- A role is a list of `{scope, level, rules}`; scopes are names such as `applications`, `users`, `settings`, `insights`, or `ALL` (`roledata.ScopeAll`).
- `CollectGrants`: the user must be in the active phase (else `ErrUserNotActive`). Roles come from the user's own role ids plus the roles of each group the user belongs to. A role counts only if its status is `Active` and it isn't expired; a temporary role whose expiry doesn't parse counts as expired. Levels merge by maximum per scope; deny rules accumulate per scope. Users, groups and roles that are being deleted (deletion timestamp set) grant nothing. A missing group or role is skipped with a warning; a backend error fails the whole request.
- `Allows`: **deny first.** If the route's rule key is denied on the route's scope or on `ALL`, the request is refused whatever level any role grants. Then the granted level must cover the required one, taking the stronger of the scope's entry and the `ALL` entry.
- Deny-rule keys are `<scope>.<action>.deny` in lower case (`x-ware/authz/rule.go`); the action names are in `data/resources/role/rules.go`.

### Where each service resolves sessions and grants

| Service | Session lookup | Grants cache |
|---|---|---|
| exporter | `UserSession` informer mirror, API-server fallback (`services/exporter/internal/authz/resolver.go`) | Redis, 60 s, HMAC-SHA256 signed with the service token and bound to the user; role and group changes bump a generation counter that invalidates every entry (`cache.go`, `signing.go`) |
| discovery | exporter session route | in process, 30 s for sessions and grants, errors never cached (`services/discovery/internal/constants/authz.go`); revocation can take up to 30 s to reach discovery |
| auth | exporter, no cache (`services/auth/internal/authz/resolver.go`) | none |
| analyzer | calls auth `GET /api/v1/auth/permissions` on every request, no cache (`services/analyzer/authz.py`) | none |

The analyzer re-implements the same evaluation in Python (`authz.py`: active and unexpired roles only, deny rule first, stronger of scope and `ALL`). Keep it in step with `x-ware/authz` when either changes.

### Handler-level guards

Route requirements decide who may call a route; guards decide what the caller may change. The exporter's are in `services/exporter/internal/authz/guard.go` and `visibility.go`, auth's in `services/auth/internal/authz/guard.go`. Examples:

- `GuardApplicationPatch`: users may change only an application's display name and description.
- `GuardGlobalConfigPatch`: a level per field; OIDC settings need Admin; `cluster` is Internal only.
- `GuardSelfUser`, `GuardSelfSessionToken`: a user reads and deletes only their own sessions and passkeys.
- `GuardReferencedIDs`: referenced users, groups, roles must exist and not be terminating; a failed lookup refuses.
- `GuardRoleDeletion` (`preventDeletion`, whoever calls) and `services/exporter/internal/utils/resources/role/protection.go` (`preventModification`, `preventScopeChanges`): the built-in roles set all three.
- Most guards let Internal callers through, which is why some deny rules can only be enforced at the route (see the comment above `addIdentityProvider` and `addCleanup` in `services/auth/internal/authz/requirements.go`).

### Users, groups, roles and bootstrap admins

- **Records.** `UserAsResource` (role ids, group ids, account phase, `bootstrap`), `GroupAsResource` (role ids, members), `RoleAsResource` (scope entries, status, validity, protection flags); types in `data/resources/{user,group,role}`. Group membership is stored on both the user and the group; grants read the user side. The exporter writes the other side first, so a failed request can be retried, and once at boot drops memberships that only one side records (`services/exporter/internal/membership/`).
- **Built-in roles** are re-applied on every exporter start (`services/exporter/internal/startup/seed.go`, `data/resources/role/builtin.go`): Admin (Admin on `ALL`), Owner, Contributor and ReadOnly (that level on every built-in scope), each protected against deletion, modification and scope changes. There are no built-in groups.
- **New users** get the ReadOnly role, unless their email is in `BOOTSTRAP_ADMINS` (`app.auth.bootstrap.admins`): then the Admin role and `bootstrap: true` (`services/auth/internal/helpers/auth/role.go` and `jit.go`, `services/auth/internal/handlers/oidc/provision.go`). Each OIDC login re-applies both when missing, decided on the email the identity provider verified (`EnsureBootstrapAdmin`). The `auth break-glass --email <email>` subcommand is the operator's emergency path.
- **Privilege edits** (`GuardUserPatch`, `GuardUserCreate`, `GuardGroupRolesPatch`, `GuardGroupMembersPatch`, `GuardRoleLevels` in `services/exporter/internal/authz/guard.go`):
  - A patch that touches no privileged field is a profile edit, allowed on your own account only.
  - Roles need Owner on `users`, groups Owner on `groups`, account status Admin on `users` plus the `suspenduser` rule; adding and removing each check their own deny rule.
  - Nobody changes their own roles, groups or status, or adds or removes themselves as a group member.
  - A role you assign or author may not exceed your own level on any scope (`ALL` counts for every scope).
  - Creating a user with roles, groups or a status needs the same rights, and the `bootstrap` field can't be set through the API.
- **Administrators and bootstrap accounts** (`services/exporter/internal/authz/visibility.go`, `services/auth/internal/authz/guard.go`):
  - A caller that is neither Internal nor Admin on `ALL` gets 404 for administrator and bootstrap users and doesn't see them in lists; a role or group that can't be read counts as administrative.
  - Bootstrap accounts are never deleted through the API, only they may edit their own record, and no user may take a `BOOTSTRAP_ADMINS` email (`GuardReservedEmail`).
  - Only a bootstrap account may delete or suspend another administrator, and nobody deletes their own account.
- **Deletion** goes through auth's cleanup routes (Owner on the scope plus the `deleteuser`, `deletegroup` or `deleterole` rule). A record being deleted grants nothing and refuses edits (`GuardNotTerminating`, 410); the cleanup cascade purges a deleted user's sessions and strips references to the record before its finalizer is removed (`services/auth/internal/controllers/cleanup/`).

## Kubernetes privileges

Each service runs under its own ServiceAccount `telark-<service name>-sa`, for example `telark-discovery-service-sa` (`templates/_helpers.tpl`); the rules below come from `charts/telark/templates/<service>/rbac/`.

| Service | Cluster-wide | In the release namespace |
|---|---|---|
| discovery | read core workloads, config and Secrets, events, namespaces, apps, batch, networking, HPA/VPA, CRD definitions, metrics; **create/update/patch** Services, ConfigMaps, Secrets, PVCs, ServiceAccounts, Deployments, StatefulSets, DaemonSets, CronJobs, Ingresses, NetworkPolicies, HPAs, VPAs (rollback writes snapshots back with create or update); patch/update `applicationsasresources`; create/get/list/patch/delete Kyverno `policies` | none |
| exporter | all verbs on `admissionregistration.k8s.io` (no code in the exporter or the shared modules uses it today); create/get/list/delete Kyverno `policies` | all verbs on every resource of the `erpi.`, `auth.` and `classification.` telark groups; PVC get/list/watch/update/patch |
| analyzer | get/list pods, events, services, apps workloads, PDBs, HPAs, NetworkPolicies (read-only) | none |
| auth, notifier, ui | none (ServiceAccount only) | none |

No service can create RBAC objects, escalate, bind or impersonate. ServiceAccount tokens are mounted in every pod (the chart doesn't set `automountServiceAccountToken`).

- **Pod security** (`templates/_deployment.tpl`, `app.shared.podSecurityContext` and `containerSecurityContext` in `values.yaml`): non-root uid/gid 1001, no privilege escalation, read-only root filesystem, all capabilities dropped. discovery and ui set `includeSecurity: false` and run without these settings.
- **Kyverno policies from protection plans** are created only by discovery: namespaced Kyverno `Policy` objects, labelled `telark.erpi/protection-plan=<plan id>`, applied with server-side apply, `Enforce` or `Audit` from the plan (`data/policies/shared.go`, `services/discovery/internal/core/plans/protection/policies/applier.go`). See [protection plans](../architecture/protection-plans.md).
- **Chart-installed Kyverno policy** `telark-inject-modifier` (`templates/policies/clusterpolicy-image-inject.yaml`): a mutate policy, `failurePolicy: Ignore`, that stamps `telark.io/last-modified-by`, `-at` and `-operation` annotations on namespaced objects outside the telark and system namespaces. It renders only when the Kyverno `ClusterPolicy` API already exists.
- **Kyverno fails open**: the chart sets `kyverno.features.forceFailurePolicyIgnore.enabled: true`, so when Kyverno's webhook is unavailable, admission (including plan policies in `Enforce`) lets requests through.
- **CRD write guard** (`templates/policies/crd-admission-policy.yaml`, `app.crdGuard`, off by default): a ValidatingAdmissionPolicy that allows writes to the three telark groups only from the exporter ServiceAccount (and `extraAllowedUsers`), plus discovery for `applicationsasresources`; `enforce: true` denies, otherwise it audits.
- **Network**: the only NetworkPolicies are Ollama's (ingress from analyzer pods only; egress DNS, plus 443 when `app.ollama.autoPull` is on) and the Redis subchart's (clients labelled `telark-redis-client`). The telark services have none.

## Invariants

Never bypass, weaken or work around these. A change that needs to is a design change: raise it with the user.

1. **Every route has an explicit requirement.** Unmapped routes are denied (`x-ware/authz/handler.go`); the per-service requirement tests must keep failing on a missing or newly Public route.
2. **Deny rules win over levels**, on the route's scope and on `ALL` (`x-ware/authz/allow.go`). Don't add a code path that checks a level without the rule.
3. **Only active users and active, unexpired roles grant anything**, and an unparsable expiry is expired (`x-ware/authz/grants.go`). The analyzer's copy (`services/analyzer/authz.py`) follows the same rules.
4. **Identity comes from the validated session or the service token, never from client headers.** The middleware strips `X-User-ID`, `X-Username`, `X-Email`; handlers take the user from the request context, not from the body or path.
5. **The service token never leaves the cluster Secret**: not in logs, responses, Redis or the UI. It is full API access.
6. **Session tokens are never stored or put in a URL**; only `session-<sha256>` names and `self` are (`data/auth/session.go`, `services/exporter/internal/utils/auth/session/name.go`).
7. **Redis is untrusted.** Any cached value that drives an authorization decision is signed and verified (`services/exporter/internal/authz/signing.go`); an unsigned or mismatched entry is a miss, never a grant.
8. **An outage is not a denial and not a grant.** Resolver failures return 503 (`denialFor`); discovery never caches errors.
9. **The exporter is the only writer of telark CRDs** (discovery may patch `applicationsasresources`); other services go through the exporter's HTTP API with `rest` clients ([AGENTS.md](../../AGENTS.md#go-services)).
10. **Guards run for user callers on every write path**: role levels you can grant are capped at your own, role protection flags hold for everyone, referenced ids must exist.
11. **The analyzer stays local**: open-weight models through Ollama only, no provider API keys, egress only in connected mode ([AGENTS.md](../../AGENTS.md#analyzer-python)).
12. **CRD schema first.** A field added to a Go type before the CRD schema is silently pruned, which can drop security-relevant data such as role rules ([AGENTS.md](../../AGENTS.md#go-services)).

## Not provided by the code today

Facts an operator or reviewer should not assume otherwise:

- No request rate limiting or login lockout in any service.
- Redis runs without authentication; don't enable `redis.auth.enabled` (the services get no password, see SECURITY.md).
- The CRD write guard is off by default, and the exporter's `admissionregistration.k8s.io` grant would let its ServiceAccount remove it.
- Kyverno is configured to fail open.
- No NetworkPolicy isolates the telark services from other pods.
- CORS in `x-ware/cors` allows only `http://localhost:3000` with credentials.
