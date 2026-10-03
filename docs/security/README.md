# Security model

How Telark authenticates people and services, authorizes each request, and what it may do in the cluster. Every statement points at the code that implements it; when the code changes, this page changes in the same diff. Vulnerability reporting is in [SECURITY.md](../../SECURITY.md). Paths starting `x-ware/`, `data/` or `rest/` are in the shared Go packages under `internal/`.

## Trust boundaries

| Boundary | Credential | Enforced by |
|---|---|---|
| Browser → auth, discovery, exporter, analyzer | session token in the `X-Session-Token` header | `x-ware/authz/handler.go` in every Go service; `services/analyzer/authz.py` |
| Service → service | shared service token in the `X-Service-Token` header | the same middleware (constant-time compare) |
| Service → Kubernetes API | the service's own ServiceAccount | chart RBAC, see [Kubernetes privileges](#kubernetes-privileges) |
| Anyone → Telark CRDs directly | Kubernetes identity | CRD write guard (`app.crdGuard`), on and enforcing by default |
| Workload changes → cluster | Kubernetes identity | Kyverno policies rendered from active protection plans |
| Services → Redis | password from a chart-generated Secret (`REDIS_PASSWORD`) | Redis itself (the chart refuses `redis.auth.enabled=false`) and its NetworkPolicy; still treated as untrusted: authorization cache entries are HMAC-signed |
| discovery, notifier → NATS | user and password from a chart-generated Secret | NATS server authorization in `charts/telark/values.yaml` (`nats:`) |
| analyzer → Ollama | none | `templates/shared/ollama-networkpolicy.yaml`: only analyzer pods reach port 11434 |
| auth → Google (optional) | none; Google's JWKS verifies ID tokens | only when `TelarkConfig.oidc.egressAllowed` is set |

The chart exposes only the UI through Ingress or HTTPRoute (`templates/shared/ingress.yaml`, `httproute.yaml`: path `/` to `services.ui`). How the UI reaches the service APIs is defined in the separate dashboard-ui repository.

## Authentication

Unprefixed paths in this section are under `services/auth/internal/`. Every auth route refuses a request body over 1 MiB with 413 (`helpers/shared/http.go`).

### Sessions

- Created only by `CreateUserSession` (`helpers/auth/session.go`), after a passkey login (`handlers/auth/login.go`) or an OIDC callback (`handlers/oidc/callback.go`). Registration alone creates none, and a non-active account gets none.
- Token: 32 bytes from `crypto/rand`, base64url (`helpers/shared/token.go`), valid `SESSION_EXPIRY` hours (default 24, `constants/config.go`) with no sliding refresh. Sent only in the `X-Session-Token` header (`data/constants/headers.go`): no cookies, no query parameter.
- Stored as a `Session` CR named `session-<sha256(token)>` (`data/auth/session.go`). The exporter blanks the raw token before persisting (`services/exporter/internal/utils/auth/session/extract.go`) and rejects a raw token in a path (`name.go` there): clients address a session by that name or by `self`.
- Logout deletes the session and always answers 200 (`handlers/auth/logout.go`). Expired sessions are purged per user whenever that user gets a new session (`services/exporter/internal/utils/auth/session/cleanup.go`); there is no global sweeper.

### Passkeys (WebAuthn)

- Library `github.com/go-webauthn/webauthn`; relying-party settings `RP_ID`, `RP_ORIGIN` (comma-separated), `RP_NAME` and `CHALLENGE_TIMEOUT` (`config/config.go`). An empty `RP_ID` or `RP_ORIGIN` is derived from the request's forwarded host, host or origin headers (`helpers/webauthn/webauthn.go`), so production installs set both (`app.auth.passkey.id` and `origin`). The relying-party instance cache is bounded (`MaxWebAuthnInstances`) and rebuilt when a burst of distinct hosts fills it. WebAuthn needs `https://` or `http://localhost` ([INSTALL.md](../INSTALL.md#access-the-dashboard)).
- Challenges and enrollment tokens are single-use: Redis keys with a short TTL, consumed on first read (`GETDEL`), so a failed or replayed assertion restarts the ceremony (`helpers/auth/enroll.go`). Redis is untrusted, so an enrollment token is stored only as its SHA-256 digest: the raw token exists only in the answer that created it (and in break-glass output), and no log line carries it.
- Credentials are `Passkey` CRs written through the exporter's Internal passkey routes (`internal/auth/passkeys`); users reach their own through auth's `auth/passkeys/{credentialId}`. When the library declines a registration, the manual attestation path (`helpers/webauthn/attestation.go`) still checks the ceremony type, the challenge, an allowed origin, the relying-party hash and the user-present flag.
- **Who may open a registration** (`helpers/auth/passkey.go`): a session (own account); a one-time enrollment token (the user's own link for another device, or an invite); or, while self-registration is on, a bare email naming no existing account, created only when the registration finishes. A bare email never enrolls an existing account, with or without passkeys (a failed passkey lookup counts as "has passkeys"), nor the `BOOTSTRAP_ADMIN` email, which only [break-glass](#users-groups-roles-and-the-bootstrap-admin) enrolls.
- **Self-registration** (`TelarkConfig` `selfRegistration.enabled`, off by default): auth caches it, asking the exporter at most every 5 seconds; a failed read keeps the last value (initially off), and the public `GET auth/config` serves only the cached value (`config/telarkconfig.go`). Only the bootstrap admin can turn it or SSO on, and auth refuses to start without one (`config/bootstrap.go`).
- **Invites**, enrollment links for another user (`POST`/`DELETE auth/users/{id}/enroll-link`; `GuardEnrollLinkIssue`/`Revoke` in `authz/guard.go`; `helpers/auth/enroll.go`). The holder can sign in as the target, so issuing is capped like a role assignment: Owner on `users`, and every level the target holds, directly or through a group, within the caller's own on that scope (403). Refused: the caller's own account (403); an administrator or bootstrap account for a caller below Admin on `ALL` (404, like a missing id); the bootstrap account for everyone else (403, break-glass only); a terminating account (410); a suspended one (409).
  - A link for an account that already has a passkey is an account recovery: it needs the bootstrap account or Admin on `ALL`, and the account is notified when the link is created and when a passkey is added through it.
  - One link per account is live: a new one revokes the previous in the same Redis script, and revoking deletes it. Any stored passkey closes a pending invite, so a link issued while the account had no passkey cannot add a second one later. An unknown, replaced, used, revoked or expired link, or one whose account is deleted or being deleted, gets the same answer.
  - The user record's `status.invite` (issued, expires, issuer) is display only and written by auth alone (the exporter refuses it from a session); the Redis TTL (`ENROLL_INVITE_TTL_SEC`, default one hour) is the authority.

### OIDC (Google)

- Configured at runtime in the `TelarkConfig` CR `default` (`oidc`: enabled, client ID, egress switch), re-read on every callback. The optional JWK set is the trust anchor, so it lives outside the CR in the Secret `telark-oidc-trust-secret` (key `googleJwkJson`): the exporter writes it, auth reads it as a mounted file (`OIDC_TRUST_FILE`, re-read on change; other replicas see a new value after the kubelet sync, about a minute), and a dedicated admission policy lets only the exporter (and `extraAllowedUsers`) write it ([OIDC.md](../../services/auth/OIDC.md)).
- The UI posts a Google ID token, which auth verifies locally: the RSA signature against Google's keys (fetched and cached only in process memory when egress is allowed, else the configured JWK set; never in Redis), `aud` = client ID, `iss` = `https://accounts.google.com`, `exp`, a single-use nonce from Redis, and `email_verified` (`helpers/oidc/google.go`, `helpers/oidc/nonce.go`, `handlers/oidc/callback.go`). No code exchange, no client secret.
- Changing the OIDC settings (`PATCH auth/oidc/config`, Admin on `settings` plus the `settings.editoidcconfig.deny` rule) or the self-registration toggle (`PATCH auth/self-registration`) also takes the bootstrap account, read from the caller's own user record rather than its grants, because whoever controls the trust anchor can mint a login for any user (`handlers/oidc/config.go`, `GuardBootstrapCaller`). The exporter accepts both settings only from a service (Internal), so a session cannot write them past auth.
- Users are matched by provider, issuer and subject, then by a unique email of an account with no identity yet; an ambiguous email, or one whose account already signs in another way, gets 409, and an unknown one creates the user whatever the self-registration setting (`handlers/oidc/provision.go`). Stored emails are not verified, so a Google subject never attaches to an account that already has a passkey or another provider. The bootstrap account is passkey only, even before its first enrollment: a login that resolves to it, by email or by a Google subject bound before its promotion, gets 403 and the reserved-email message (`handlers/oidc/callback.go`).

### Service token

- One shared secret per install, key `token`: the Secret named by `app.serviceToken.existingSecret`, else the chart's `telark-service-token-secret`, filled from `app.serviceToken.value`, else the existing Secret (Helm `lookup`), else 48 random characters (`charts/telark/templates/shared/service-token-secret.yaml`). Every deployment except ui (`services.ui.serviceToken: false`; nginx only proxies) receives it as `TELARK_SERVICE_TOKEN` (`templates/_deployment.tpl`).
- A service refuses to start without it (`x-ware/authz/bootstrap.go`); outgoing `rest` clients attach it as `X-Service-Token` (`rest/clients/shared/servicetoken.go`).
- A request with a valid service token is Internal: it passes every route, scoped ones included, without a grant check, and the caller-supplied `X-User-ID` is trusted as the acting user (`x-ware/authz/handler.go`, `identify`). A wrong token is 401. The token is full API access.

## Authorization

### Middleware (`x-ware/authz`)

auth, discovery and exporter wrap their router with the same middleware in `main.go` (notifier serves only unauthenticated probes). Per request, in `handler.go`:

1. Read the claimed `X-User-ID`, then strip `X-User-ID`, `X-Username` and `X-Email` from the request (`constants.go`), so a client can't inject an identity.
2. Look up the route (method + mux path template, `rest/router`). A route without an entry in the service's `internal/authz/requirements.go` is **403**.
3. `Public` routes pass.
4. A present `X-Service-Token` must match (else 401) and makes the caller Internal. An `Internal` route without it is 401.
5. Otherwise `X-Session-Token` → user id → grants. `Authenticated` routes stop here; scoped routes then need `Allows`.
6. Only a resolver verdict (not found, gone, session expired, user not active) produces 401/403; any other resolver error is 503, so an outage never looks like a logout (`denialFor`).

Requirement helpers (`x-ware/authz/requirement.go`): `Read` = ReadOnly, `Write` = Contributor, `Own` = Owner, `Administer` = Admin on a scope; `Denyable(req, action)` adds a deny-rule key. Each API service's `internal/tests/authz/requirements_test.go` fails when a route has no requirement (`TestRequirementsCoverEveryRoute`) or becomes Public unexpectedly (`TestOnlyProbesArePublic` in discovery and exporter, `TestPublicRoutesArePinned` in auth).

### Grants (`x-ware/authz/grants.go`, `allow.go`)

- Levels (`data/resources/role/def.go`): ReadOnly 1 < Contributor 2 < Owner 3 < Admin 4. A level covers every lower one; an unknown level covers nothing.
- A role is a list of `{scope, level, rules}`; scopes are names such as `applications`, `users`, `settings`, `insights`, or `ALL` (`roledata.ScopeAll`).
- `CollectGrants`: the user must be active (else `ErrUserNotActive`). Roles come from the user's own role ids plus those of each of its groups. A role counts only if its status is `Active` and it isn't expired; an expiry that doesn't parse counts as expired. Levels merge by maximum per scope; deny rules accumulate per scope. Users, groups and roles being deleted (deletion timestamp set) grant nothing. A missing group or role is skipped with a warning; a backend error fails the whole request.
- `Allows`: **deny first.** If the route's rule key is denied on the route's scope or on `ALL`, the request is refused whatever level any role grants. Then the granted level must cover the required one, taking the stronger of the scope's entry and the `ALL` entry.
- Deny-rule keys are `<scope>.<action>.deny` in lower case (`x-ware/authz/rule.go`); the action names are in `data/resources/role/rules.go`.

### Peer calls (`rest/clients`)

- Path parameters are `url.PathEscape`d before substitution and the shared client never follows redirects (`rest/clients/shared/{utils,client}.go`), so a crafted name or email cannot steer a service-token call to another route.
- The user, group, role and session lookups that feed authorization send `Cache-Control: no-cache`, so a planted exporter response-cache entry in Redis cannot change a peer's grants (`rest/clients/shared/headers.go`).
- Peer 5xx bodies are not relayed to users (`rest/utils/response/def.go`); request bodies over 1 MiB are refused with 413, not truncated (`rest/utils/request/def.go`).

### Where each service resolves sessions and grants

| Service | Session lookup | Grants cache |
|---|---|---|
| exporter | `Session` informer mirror, API-server fallback (`services/exporter/internal/authz/resolver.go`) | Redis, 60 s, HMAC-SHA256 signed with the service token and bound to the user, the generation and the issue time. Role and group changes bump a generation counter that invalidates every entry. An entry older than 60 s is a miss even if Redis kept it; a generation lower than one the process has seen counts as a rollback and bypasses the cache (`cache.go`, `signing.go`) |
| discovery | exporter session route | in process, 30 s for sessions and grants, errors never cached (`services/discovery/internal/constants/authz.go`); a revocation can take up to 30 s to reach discovery |
| auth | exporter, no cache (`services/auth/internal/authz/resolver.go`) | none |
| analyzer | calls auth `GET /api/v1/auth/permissions` on every request, no cache (`services/analyzer/authz.py`) | none |

The analyzer re-implements the same evaluation in Python (`authz.py`: active and unexpired roles only, deny rule first, stronger of scope and `ALL`). Keep it in step with `x-ware/authz` when either changes.

### Handler-level guards

Route requirements decide who may call a route; guards decide what the caller may change (exporter: `services/exporter/internal/authz/guard.go`, `visibility.go`; auth: `services/auth/internal/authz/guard.go`). Examples:

- `GuardApplicationPatch`: users may change only an application's display name and description.
- Protection plans: the exporter's plan `PATCH` and `DELETE` routes are Internal, and on create `GuardPlanLifecycle` keeps lifecycle, approval and policy fields away from sessions, so users edit, approve and delete through discovery. discovery's approval, requester, `enforce` and `{{`/`}}` rules are in [protection plans](../architecture/protection-plans.md#lifecycle).
- Config guard (`PATCH config`): a level per field; `oidc`, `selfRegistration` and `cluster` are Internal only (auth writes the first two for the bootstrap account); a patch naming no governed field needs Read on `settings`, like `GET config`.
- Snapshot manifests need both the `viewapplicationssnapshots` and the `viewapplicationsnapshotmanifest` rules (`GuardSnapshotView` in the manifest handler).
- The cleanup finalizer routes (`PUT` and `DELETE cleanup/{type}/{id}/finalizer`) are Internal: only auth's cleanup cascade and the uninstall hook set or remove finalizers.
- `GuardSelfUser`, `GuardSelfSessionToken`: a user reads and deletes only their own sessions and passkeys. The per-user session list (`GET auth/sessions?user={userId}`) is not response-cached, because a cache hit would be served before the owner check.
- `GuardReferencedIDs`: referenced users, groups and roles must exist and not be terminating; a failed lookup refuses.
- Role protection (`GuardRoleDeletion` and `services/exporter/internal/utils/resources/role/protection.go`; 403 whoever calls): `preventDeletion` refuses DELETE; `preventModification` refuses every key besides `protection`; `preventScopeChanges`, `lockName` and `lockCategory` refuse a change to `scopesAndPermissions`, `name` and `categoryRef` (resending the stored value is allowed). A flag the same patch lifts no longer applies, so unlocking and editing can be one save. `softDelete` turns DELETE into `status: Deleted` plus `deletedAt`; the role stays stored and grants nothing. The built-in roles set `preventDeletion`, `preventModification` and `preventScopeChanges`.
- Who may change `protection` (`GuardRoleReservedFields`): on create, the caller, who becomes the creator; on a custom role, only its creator (`createdBy`) or an Admin on `ALL`, on top of the edit rule; on a built-in role, nobody (403). `createdBy` and `lastUpdatedBy` of roles and groups are stamped from the caller; body values are ignored.
- Most guards let Internal callers through, so some deny rules can only be enforced at the route (see the comment above `addIdentityProvider` and `addCleanup` in `services/auth/internal/authz/requirements.go`).

### Users, groups, roles and the bootstrap admin

- **Records.** `User` (`roleRefs`, `groupRefs`, account phase in `spec.status`, `bootstrap`), `Group` (`roleRefs`, `userRefs`), `AccessRole` (scope entries, lifecycle `status`, validity, protection flags); types in `data/resources/{user,group,role}`, identity in `metadata.name` ([CRD identity](../CRDS.md#identity)). Group membership is stored on both the user and the group; grants read the user side. The exporter writes the other side first, so a failed request can be retried, and strips the other side right after a user or group delete (auth's cleanup sweep is the backstop). At boot it reconciles both sides from the user side: it adds user-only memberships to the groups and drops group-only ones, references to missing groups and duplicates (`services/exporter/internal/membership/`).
- **Built-in roles** are re-applied on every exporter start (`services/exporter/internal/startup/seed.go`, `data/resources/role/builtin.go`): Admin (Admin on `ALL`), Owner, Contributor and ReadOnly (that level on every built-in scope), each protected against deletion, modification and scope changes. There are no built-in groups.
- **New users** from OIDC (`handlers/oidc/provision.go`) or passkey self-registration (`helpers/auth/jit.go`) get the ReadOnly role, even with the bootstrap email; neither path sets `bootstrap: true`.
- **The bootstrap admin** is one email, `BOOTSTRAP_ADMIN` (`app.auth.bootstrap.admin`). Only the operator's `./main break-glass --email <email> [--enroll]` subcommand (`services/auth/cmd/breakglass.go`) grants it the built-in Admin role and `bootstrap: true`; it creates and recovers the account, and `--enroll` prints an enrollment token. `--email` only selects that account ([invariant 13](#invariants)), so break-glass never creates or promotes another account. It also removes every non-passkey identity from the promoted account, so no OIDC binding survives the promotion. Regular admins are ordinary users holding Admin on `ALL`, directly or through a group.
- **Privilege edits** (`GuardUserPatch`, `GuardUserCreate`, `GuardGroupRolesPatch`, `GuardGroupMembersPatch`, `GuardRoleLevels` in `services/exporter/internal/authz/guard.go`):
  - A patch that touches no privileged field is a profile edit, allowed on your own account only.
  - Roles need Owner on `users`, groups Owner on `groups`, account status Admin on `users` plus the `suspenduser` rule; adding and removing each check their own deny rule.
  - Nobody changes their own roles, groups, status or group membership.
  - Granting is capped at your own level on every scope (`ALL` counts for every scope): a role you assign or author, every role a group carries when you add a member to it (from either side), and a role patch that changes `status`, `validity` or `scopesAndPermissions`, checked against the role as it will be stored, so an inactive or expired role above you can't be switched back on.
  - Taking a role away is capped the same way: removing it from a user, detaching it from a group, removing a member from a group that carries it (from either side), editing `status`, `validity` or `scopesAndPermissions` of a stored role above you, and deleting a role or a group through the exporter's `DELETE accessroles/{id}` and `groups/{id}` (auth's cleanup DELETE routes call these as Internal, so they get the cap once auth checks it itself).
  - Creating a user with roles, groups or a status needs the same rights. The API never sets `bootstrap`. Only services (Internal) write `identities`, `status.invite` and `status.inviteAcceptedAt`, on create and patch alike (403 from a session), and only the account owner changes `email` and `username`. Resending a stored value unchanged is allowed.
  - Sessions can't create or patch a role with `type: built-in`; `protection` follows the role protection rules above. Deny rules are stored in lower case.
  - Bodies on the user, group, role, category, protection-plan and cleanup-finalizer write routes are refused (400) when a key is not exactly a JSON field name of the record (`CheckCanonicalKeys` in `services/exporter/internal/utils/shared/keys.go`): `encoding/json` matches keys case-insensitively but the guards read exact keys, so a case variant must never reach the decoder.
- **Administrators and bootstrap accounts** (`services/exporter/internal/authz/visibility.go`, `services/auth/internal/authz/guard.go`):
  - A caller that is neither Internal nor Admin on `ALL` gets 404 for administrator and bootstrap users and doesn't see them in lists; a role or group it can't read counts as administrative. Its user PATCH of a hidden account answers 404 before the body is parsed; its group patch keeps the hidden members, and naming a hidden id answers 400 like an unknown id.
  - Bootstrap accounts are never deleted through the API, and only they may edit their own record. A group create or patch that adds or removes one answers 403 to an Admin on `ALL` (`GuardGroupMembersPatch`). No session caller may give the `BOOTSTRAP_ADMIN` email to another account (by create or move) or move the bootstrap account off it, since break-glass finds the account by that email; an unchanged email is always accepted (`GuardReservedEmail`). Internal callers are exempt, so auth itself refuses the bootstrap email on the bare-email registration path.
  - Any Admin on `ALL` may delete or suspend another (non-bootstrap) administrator, and nobody deletes their own account.
  - A change that would leave no active user holding Admin on `ALL` (the bootstrap account counts) answers 409 for every caller, Internal included, so deletes routed through auth are covered: a user delete, suspension or `roleRefs`/`groupRefs` change; a group delete, `roleRefs` change or member removal; a role delete (soft delete included) or `scopesAndPermissions`, `status` or `validity` change (`GuardUserPatchLastAdmin`, `GuardUserDeleteLastAdmin`, `GuardGroupPatchLastAdmin`, `GuardGroupDeleteLastAdmin`, `GuardRolePatchLastAdmin`, `GuardRoleDeleteLastAdmin`). The check reads the user list and then writes, so two concurrent removals of the last two admins can both pass. The built-in Admin role can't be edited or deleted.
- **Deletion** goes through auth's `DELETE auth/{users,groups,accessroles}/{id}` routes (Owner on the scope plus the `deleteuser`, `deletegroup` or `deleterole` rule). A record being deleted grants nothing and refuses edits (`GuardNotTerminating`, 410); the cleanup cascade purges a deleted user's sessions and strips references to the record before its finalizer is removed (`services/auth/internal/controllers/cleanup/`).

## Kubernetes privileges

Each service runs under its own ServiceAccount `telark-<service name>-sa`, for example `telark-discovery-service-sa` (`templates/_helpers.tpl`); the rules below come from `charts/telark/templates/<service>/rbac/`.

| Service | Cluster-wide | In the release namespace |
|---|---|---|
| discovery | read core workloads, config and Secrets, events, namespaces, apps, batch, networking, HPA/VPA, pod metrics; **create/update** Services, ConfigMaps, Secrets, PVCs, ServiceAccounts, Deployments, StatefulSets, DaemonSets, CronJobs, Ingresses, NetworkPolicies, HPAs, VPAs (rollback writes snapshots back with create or update, never patch); get/list/watch `applications` (`telark.io`); patch `applications/status` (`telark.io`); create/get/list/patch/delete Kyverno `policies` (server-side apply) | none |
| exporter | none (no ClusterRole) | all verbs on every resource and subresource of `telark.io`; get, update and patch on the OIDC trust Secret only (by name); PVC get/list/watch (reads the snapshot volume's capacity) |
| analyzer | get/list pods, events, services, apps workloads, PDBs, HPAs, NetworkPolicies (read-only) | none |
| auth, notifier, ui | none (ServiceAccount only, token not mounted) | none |

No service can create RBAC objects, escalate, bind or impersonate. Discovery's write rules still cover every namespace, Secrets and ServiceAccounts included (RBAC cannot exclude namespaces), so a compromised discovery pod is close to cluster-admin; it is the one service to protect first. auth, notifier and ui set `automountServiceAccountToken: false` on the pod and the ServiceAccount; the other services mount their token.

- **Pod security** (`templates/_deployment.tpl`, `app.shared.podSecurityContext` and `containerSecurityContext` in `values.yaml`): every service but ui runs as non-root uid/gid 1001, with no privilege escalation, a read-only root filesystem, all capabilities dropped and seccomp `RuntimeDefault`. ui (nginx) sets `includeSecurity: false` and its own contexts in `services.ui`: the same settings as non-root uid 101, with `emptyDir` on `/var/cache/nginx` and `/tmp`.
- **Plan policies** are created only by discovery: namespaced Kyverno `Policy` objects labeled `telark.io/protection-plan=<plan id>`, applied with server-side apply, `Enforce` or `Audit` from the plan (`data/policies/shared.go`, `services/discovery/internal/core/plans/protection/policies/applier.go`; [protection plans](../architecture/protection-plans.md)).
- **Chart-installed Kyverno policy** `telark-inject-modifier` (`templates/policies/clusterpolicy-image-inject.yaml`): a mutate policy, `failurePolicy: Ignore`, that stamps the `telark.io/last-modified-{by,at,operation}` annotations on the kinds discovery snapshots, outside the telark and system namespaces. Secrets are excluded, so Kyverno never mutates Secret writes for it. A first install gets it on its first upgrade ([INSTALL.md](../INSTALL.md#last-modified-annotations)).
- **CRD write guard** (`templates/policies/crd-admission-policy.yaml`, `app.crdGuard`, on and enforcing by default): a ValidatingAdmissionPolicy that matches every resource and subresource of `telark.io` (`*/*`, status writes included) in every namespace (the binding has no namespace selector) and allows writes only from the exporter ServiceAccount (and `extraAllowedUsers`), plus discovery for `applications/status`. A second policy guards the Secret `telark-oidc-trust-secret` by name, so namespace `edit` rights (which include Secrets) cannot plant a JWK set and mint OIDC logins. `enforce: false` only audits. The CRD schemas also pin `Session` names to `session-<64 hex>` and `User` `spec.status.phase` to `active`, `inactive` or `suspended`.
- **Network** (`templates/shared/networkpolicy.yaml`, `app.networkPolicy.enabled`, on by default): ingress default deny for every Telark pod; the service APIs accept only Telark pods of the same release; ui accepts any source on its port; NATS accepts discovery and notifier on 4222 only (6222 between NATS pods) and runs no monitoring listener (the NATS Services still list 8222). Ollama's policy admits analyzer pods only (egress DNS, plus 443 when `app.ollama.autoPull` is on); the Redis subchart's admits clients labeled `<release>-redis-client`. None of it applies without a CNI that enforces NetworkPolicy.
- **NATS users**: discovery connects as the publisher (publish `telark.applications.*`, subscribe `_INBOX.>` for JetStream acks), notifier as the consumer (publish `$JS.API.>` and `$JS.ACK.>`, subscribe `telark.applications.*` and `_INBOX.>`), each from its own Secret.
- **Chart-generated secrets** (service token, NATS users, Redis password) carry `helm.sh/resource-policy: keep`. A render without cluster access (GitOps) cannot `lookup` them and generates new values each time, so cluster-less renders use `app.serviceToken.existingSecret`, `nats.existingSecrets` and `redis.auth.existingSecret` ([INSTALL.md](../INSTALL.md#gitops-cluster-less-renders)).

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
9. **The exporter is the only writer of Telark CRDs** (discovery may patch `applications/status`); other services go through the exporter's HTTP API with `rest` clients ([AGENTS.md](../../AGENTS.md#go-services)).
10. **Guards run for user callers on every write path**: role levels you can grant are capped at your own, role protection flags hold for everyone, referenced ids must exist.
11. **The analyzer stays local**: open-weight models through Ollama only, no provider API keys, egress only in connected mode ([AGENTS.md](../../AGENTS.md#analyzer-python)).
12. **CRD schema first.** A field added to a Go type before the CRD schema is silently pruned, which can drop security-relevant data such as role rules ([AGENTS.md](../../AGENTS.md#go-services)).
13. **Break-glass serves the bootstrap admin only.** Its `--email` selects an account and grants nothing: any address but `BOOTSTRAP_ADMIN`, or none configured, is refused before any read or write (`services/auth/cmd/breakglass.go`).

## Not provided by the code

Facts an operator or reviewer should not assume otherwise:

- No request rate limiting or login lockout in any service.
- Redis traffic is not encrypted (no TLS); it stays inside the release namespace, behind Redis's password and NetworkPolicy.
- A cluster admin, or anyone who may delete ValidatingAdmissionPolicies, can remove the CRD write guard; no Telark ServiceAccount can.
- Anyone who may `kubectl exec` into the auth pod, or read Secrets in the release namespace, can obtain the service token (full API access); break-glass refusing other emails does not change that, so keep `pods/exec` and Secret reads there to cluster operators.
- Kyverno fails open unless the operator sets `app.kyverno.failOpen=false` (which must equal `kyverno.features.forceFailurePolicyIgnore.enabled`): while its webhook is unavailable, admission lets requests through, plan policies in `Enforce` included ([INSTALL.md](../INSTALL.md#policy-engine-fail-open)).
- NetworkPolicies restrict ingress only; egress from every Telark pod is open.
- CORS in `x-ware/cors` sends no headers unless `CORS_ALLOWED_ORIGINS` lists origins (never `*`); the dashboard proxies every API on its own origin.
