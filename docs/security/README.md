# Security model

How Telark authenticates people and services, authorizes each request, and what it may do in the cluster. Every statement points at the code that implements it; when the code changes, this page changes in the same diff. Vulnerability reporting is in [SECURITY.md](../../SECURITY.md).

Paths starting `x-ware/`, `data/` or `rest/` are in the shared Go packages under `internal/` (`internal/x-ware/`, `internal/data/`, `internal/rest/`).

## Trust boundaries

| Boundary | Credential | Enforced by |
|---|---|---|
| Browser → auth, discovery, exporter, analyzer | session token in the `X-Session-Token` header | `x-ware/authz/handler.go` in every Go service; `services/analyzer/authz.py` |
| Service → service | shared service token in the `X-Service-Token` header | the same middleware (constant-time compare) |
| Service → Kubernetes API | the service's own ServiceAccount | chart RBAC, see [Kubernetes privileges](#kubernetes-privileges) |
| Anyone → Telark CRDs directly | Kubernetes identity | CRD write guard (`app.crdGuard`), on and enforcing by default |
| Workload changes → cluster | Kubernetes identity | Kyverno policies rendered from active protection plans |
| Services → Redis | none (Redis runs without auth) | treated as untrusted: authorization cache entries are HMAC-signed |
| discovery, notifier → NATS | user and password from a chart-generated Secret | NATS server authorization in `charts/telark/values.yaml` (`nats:`) |
| analyzer → Ollama | none | `templates/shared/ollama-networkpolicy.yaml`: only analyzer pods reach port 11434 |
| auth → Google (optional) | none; Google's JWKS verifies ID tokens | only when `TelarkConfig.oidc.egressAllowed` is set |

The chart exposes only the UI through Ingress or HTTPRoute (`templates/shared/ingress.yaml`, `httproute.yaml`: path `/` to `services.ui`). How the UI reaches the service APIs is defined in the separate dashboard-ui repository.

## Authentication

Unprefixed paths in this section are under `services/auth/internal/`. Every auth route refuses a request body over 1 MiB with 413 (`helpers/shared/http.go`).

### Sessions

- Created only by `CreateUserSession` (`services/auth/internal/helpers/auth/session.go`), after a passkey login (`handlers/auth/login.go`) or an OIDC callback (`handlers/oidc/callback.go`). Registration alone doesn't create one, and a non-active account gets none.
- Token: 32 bytes from `crypto/rand`, base64url (`services/auth/internal/helpers/shared/token.go`). Lifetime `SESSION_EXPIRY` hours (default 24, `services/auth/internal/constants/config.go`), with no sliding refresh.
- Stored as a `Session` CR named `session-<sha256(token)>` (`data/auth/session.go`); the exporter blanks the raw token before persisting (`services/exporter/internal/utils/auth/session/extract.go`). The raw token is never stored and never appears in a URL: clients address a session by that name or by `self`, and the exporter rejects a raw token in the path (`services/exporter/internal/utils/auth/session/name.go`).
- Sent only in the `X-Session-Token` header (`data/constants/headers.go`); no cookies, no query parameter.
- Logout deletes the session and always answers 200 (`handlers/auth/logout.go`). Expired sessions are purged per user whenever that user gets a new session (`services/exporter/internal/utils/auth/session/cleanup.go`); there is no global sweeper.

### Passkeys (WebAuthn)

- Library `github.com/go-webauthn/webauthn` (root `go.mod`). Relying-party settings `RP_ID`, `RP_ORIGIN` (comma-separated), `RP_NAME` and `CHALLENGE_TIMEOUT` (`services/auth/internal/config/config.go`). When `RP_ID` or `RP_ORIGIN` is empty, they are derived from the request's forwarded host, host or origin headers (`helpers/webauthn/webauthn.go`); production installs should set both (`app.auth.passkey.id` and `origin` in `charts/telark/values.yaml`).
- Challenges live in Redis with a short TTL and are consumed on first read (Redis `GETDEL`), so a failed or replayed assertion has to restart the ceremony; enrolment links are single-use the same way (`helpers/auth/enroll.go`). Credentials are `Passkey` CRs written through the exporter's Internal passkey routes (`internal/auth/passkeys`); users reach their own through auth's `auth/passkeys/{credentialId}`. When the library declines a registration, the manual attestation path (`helpers/webauthn/attestation.go`) still checks the ceremony type, the challenge, an allowed origin, the relying-party hash and the user-present flag.
- **Who may open a registration** (`helpers/auth/passkey.go`): a session (own account), a one-time enrolment token, or a bare email that names no existing account; that account is created only when the registration finishes. An existing account is never enrolled from a bare email, whether or not it has passkeys, and a passkey lookup that fails counts as "has passkeys". The `BOOTSTRAP_ADMIN` email is refused on the bare-email path: the operator enrols it with `./main break-glass --email <email> --enroll`, which creates the account with the Admin role and prints the enrolment token.
- The relying-party instance cache is bounded (`MaxWebAuthnInstances`); it is dropped and rebuilt when a burst of distinct hosts fills it.
- Browsers enable WebAuthn only on `https://` or `http://localhost` origins ([INSTALL.md](../INSTALL.md#access-the-dashboard)).
- Self-registration is controlled by `SELF_REGISTRATION_ENABLED` (chart default off); the auth service refuses to start when it is off and no bootstrap admin is configured (`services/auth/internal/config/bootstrap.go`). A self-registered account always starts with the ReadOnly role and without the `bootstrap` marker (`helpers/auth/jit.go`).

### OIDC (Google)

- Configured at runtime in the `TelarkConfig` CR `default` (`oidc`: enabled, client ID, egress switch), re-read on every callback. The optional JWK set is the trust anchor, so it is kept out of the CR in the Secret `telark-oidc-trust-secret` (key `googleJwkJson`): the exporter writes it, auth reads it as a mounted file (`OIDC_TRUST_FILE`, re-read when it changes; other replicas see a new value after the kubelet sync, about a minute), and a dedicated admission policy lets only the exporter (and `extraAllowedUsers`) write it. See [services/auth/OIDC.md](../../services/auth/OIDC.md).
- The UI posts a Google ID token. auth verifies the RSA signature locally against Google's keys (fetched from Google and cached only in process memory when egress is allowed, otherwise the configured JWK set; Redis never holds the trust anchor), `aud` = client ID, `iss` = `https://accounts.google.com`, `exp`, a single-use nonce from Redis, and `email_verified` (`helpers/oidc/google.go`, `helpers/oidc/nonce.go`, `handlers/oidc/callback.go`). There is no code exchange and no client secret. Changing the OIDC settings (`PATCH auth/oidc/config`) needs Admin on `ALL`, not only on `settings`, because whoever controls the trust anchor can mint a login for any user (`handlers/oidc/config.go`).
- Users are matched by provider, issuer and subject, then by a unique email that belongs to an account with no identity yet; an ambiguous email or one whose account already signs in another way is refused with 409, and an unknown one creates the user (`handlers/oidc/provision.go`). The bootstrap account is passkey only, even before its first enrolment: a login that resolves to it, by email or by a Google subject bound before its promotion, is refused with 403 and the reserved-email message (`handlers/oidc/callback.go`). Stored emails are not verified, so a Google subject never attaches to an account that already has a passkey or another provider. OIDC provisioning is not gated by `SELF_REGISTRATION_ENABLED`.

### Service token

- One shared secret per install: Secret `telark-service-token-secret`, key `token`, from `app.serviceToken.value`, else the existing Secret (Helm `lookup`), else 48 random characters (`charts/telark/templates/shared/service-token-secret.yaml`). Every deployment except ui (`services.ui.serviceToken: false`; nginx only proxies) receives it as `TELARK_SERVICE_TOKEN` (`templates/_deployment.tpl`).
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

### Peer calls (`rest/clients`)

- Path parameters are `url.PathEscape`d before substitution and the shared client never follows redirects (`rest/clients/shared/{utils,client}.go`), so a crafted name or email cannot steer a service-token call to another route.
- The user, group, role and session lookups that feed authorization send `Cache-Control: no-cache`, so a planted exporter response-cache entry in Redis cannot change a peer's grants (`rest/clients/shared/headers.go`).
- Peer 5xx bodies are not relayed to users (`rest/utils/response/def.go`); request bodies over 1 MiB are refused with 413 rather than truncated (`rest/utils/request/def.go`).

### Where each service resolves sessions and grants

| Service | Session lookup | Grants cache |
|---|---|---|
| exporter | `Session` informer mirror, API-server fallback (`services/exporter/internal/authz/resolver.go`) | Redis, 60 s, HMAC-SHA256 signed with the service token and bound to the user, the generation and the issue time; role and group changes bump a generation counter that invalidates every entry. An entry older than 60 s is a miss even if Redis kept it, and a generation lower than one the process has seen is treated as a rollback: the cache is bypassed (`cache.go`, `signing.go`) |
| discovery | exporter session route | in process, 30 s for sessions and grants, errors never cached (`services/discovery/internal/constants/authz.go`); revocation can take up to 30 s to reach discovery |
| auth | exporter, no cache (`services/auth/internal/authz/resolver.go`) | none |
| analyzer | calls auth `GET /api/v1/auth/permissions` on every request, no cache (`services/analyzer/authz.py`) | none |

The analyzer re-implements the same evaluation in Python (`authz.py`: active and unexpired roles only, deny rule first, stronger of scope and `ALL`). Keep it in step with `x-ware/authz` when either changes.

### Handler-level guards

Route requirements decide who may call a route; guards decide what the caller may change. The exporter's are in `services/exporter/internal/authz/guard.go` and `visibility.go`, auth's in `services/auth/internal/authz/guard.go`. Examples:

- `GuardApplicationPatch`: users may change only an application's display name and description.
- Protection plans: the exporter's plan `PATCH` and `DELETE` routes are Internal; users edit, approve and delete through discovery, which owns the lifecycle. On create, `GuardPlanLifecycle` keeps lifecycle, approval and policy fields away from sessions. In discovery, the approval mode is derived server-side (Production always `required`; a client-sent `automatic` counts only from an Owner), nobody who requested, reactivated or materially edited a plan since its last approval may decide it, `enforce` on a `namespaces` scope needs Owner, and a plan name or description containing `{{` or `}}` is refused; the rendered Kyverno message carries only the plan id, because Kyverno substitutes variables there ([protection plans](../architecture/protection-plans.md#lifecycle)).
- The config guard (`PATCH config`): a level per field; OIDC settings need Admin on `ALL` (not only on `settings`) and still honour the `settings.editoidcconfig.deny` rule; `cluster` is Internal only; a patch naming no governed field needs Read on `settings`, like `GET config`.
- Snapshot manifests need both the `viewapplicationssnapshots` and the `viewapplicationsnapshotmanifest` rules (`GuardSnapshotView` in the manifest handler).
- The cleanup finalizer routes (`PUT` and `DELETE cleanup/{type}/{id}/finalizer`) are Internal: only auth's cleanup cascade and the uninstall hook set or remove finalizers.
- `GuardSelfUser`, `GuardSelfSessionToken`: a user reads and deletes only their own sessions and passkeys. The per-user session list (`GET auth/sessions?user={userId}`) is not response-cached, because a cache hit would be served before the owner check.
- `GuardReferencedIDs`: referenced users, groups, roles must exist and not be terminating; a failed lookup refuses.
- Role protection (`GuardRoleDeletion` and `services/exporter/internal/utils/resources/role/protection.go`, 403, whoever calls): `preventDeletion` refuses DELETE; `preventModification` refuses every key besides `protection`; `preventScopeChanges`, `lockName` and `lockCategory` refuse a change to `scopesAndPermissions`, `name` and `categoryRef` (resending the stored value is allowed). A flag that the same patch lifts no longer applies, so unlocking and editing can be one save. `softDelete` turns DELETE into `status: Deleted` plus `deletedAt`; the role stays stored and grants nothing. The built-in roles set `preventDeletion`, `preventModification` and `preventScopeChanges`.
- Who may change `protection` (`GuardRoleReservedFields`): on create, the caller, who becomes the creator; on a custom role, only its creator (`createdBy`) or an Admin on `ALL`, on top of the edit rule; on a built-in role, nobody (403). `createdBy` and `lastUpdatedBy` of roles and groups are stamped from the caller; body values are ignored.
- Most guards let Internal callers through, which is why some deny rules can only be enforced at the route (see the comment above `addIdentityProvider` and `addCleanup` in `services/auth/internal/authz/requirements.go`).

### Users, groups, roles and the bootstrap admin

- **Records.** `User` (`roleRefs`, `groupRefs`, account phase in `spec.status`, `bootstrap`), `Group` (`roleRefs`, `userRefs`), `AccessRole` (scope entries, lifecycle `status`, validity, protection flags); types in `data/resources/{user,group,role}`. A record's identity is its `metadata.name`; the API returns it as `id`, and there is no `spec.id` to disagree with it. Group membership is stored on both the user and the group; grants read the user side. The exporter writes the other side first, so a failed request can be retried. A user or group delete strips the other side right after the delete; auth's cleanup sweep is the backstop. Once at boot it reconciles the two sides from the user side: user-only memberships are added to the groups, group-only ones are dropped, and so are references to missing groups and duplicates (`services/exporter/internal/membership/`).
- **Built-in roles** are re-applied on every exporter start (`services/exporter/internal/startup/seed.go`, `data/resources/role/builtin.go`): Admin (Admin on `ALL`), Owner, Contributor and ReadOnly (that level on every built-in scope), each protected against deletion, modification and scope changes. There are no built-in groups.
- **New users** get the ReadOnly role, whether they arrive through OIDC (`handlers/oidc/provision.go`) or passkey self-registration (`helpers/auth/jit.go`), even when their email is the bootstrap email; neither path sets `bootstrap: true`.
- **The bootstrap admin** is one email, `BOOTSTRAP_ADMIN` (`app.auth.bootstrap.admin`). Only the operator's `./main break-glass --email <email> [--enroll]` subcommand (`services/auth/cmd/breakglass.go`) grants it the built-in Admin role and `bootstrap: true`; it creates the account and recovers it. Break-glass on any other email grants Admin without the marker. Either way it removes every non-passkey identity from the promoted account, so no OIDC binding survives the promotion. Regular admins are ordinary users holding Admin on `ALL`, directly or through a group.
- **Privilege edits** (`GuardUserPatch`, `GuardUserCreate`, `GuardGroupRolesPatch`, `GuardGroupMembersPatch`, `GuardRoleLevels` in `services/exporter/internal/authz/guard.go`):
  - A patch that touches no privileged field is a profile edit, allowed on your own account only.
  - Roles need Owner on `users`, groups Owner on `groups`, account status Admin on `users` plus the `suspenduser` rule; adding and removing each check their own deny rule.
  - Nobody changes their own roles, groups or status, or adds or removes themselves as a group member.
  - A role you assign or author may not exceed your own level on any scope (`ALL` counts for every scope). Adding a member to a group, from either side, is capped the same way by every role the group carries. A role patch that changes `status`, `validity` or `scopesAndPermissions` is capped against the role as it will be stored, so an inactive or expired role above you can't be switched back on.
  - Taking a role away is capped like granting it: removing it from a user, detaching it from a group, removing a member from a group that carries it (from either side), editing `status`, `validity` or `scopesAndPermissions` of a stored role above you, and deleting a role or a group through the exporter's `DELETE accessroles/{id}` and `groups/{id}` (auth's cleanup DELETE routes call these as Internal, so they get the cap once auth checks it itself).
  - Creating a user with roles, groups or a status needs the same rights, and the `bootstrap` field can't be set through the API.
  - Identity fields: `identities` is set only by services (Internal), never from a session; `email` and `username` are changed only by the account owner. Resending the stored value unchanged is allowed.
  - Sessions can't create or patch a role with `type: built-in`; `protection` follows the role protection rules above. Deny rules are stored in lower case on write.
  - Request bodies on the user, group, role, category and protection-plan write routes are refused (400) when a key is not exactly a JSON field name of the record (`CheckCanonicalKeys` in `services/exporter/internal/utils/shared/keys.go`): `encoding/json` matches keys case-insensitively, the guards read exact keys, so a case variant must never reach the decoder.
- **Administrators and bootstrap accounts** (`services/exporter/internal/authz/visibility.go`, `services/auth/internal/authz/guard.go`):
  - A caller that is neither Internal nor Admin on `ALL` gets 404 for administrator and bootstrap users and doesn't see them in lists; a role or group that can't be read counts as administrative. A user PATCH of such a hidden account answers 404 before the body is parsed. A group patch from such a caller keeps the hidden members, and naming a hidden id answers 400 like an unknown id.
  - Bootstrap accounts are never deleted through the API, only they may edit their own record, a group create or patch that adds or removes one answers 403 to an Admin on `ALL` (`GuardGroupMembersPatch`), and no session caller may create a user with the `BOOTSTRAP_ADMIN` email, move another account onto it, or move the bootstrap account off it, since break-glass finds the account by that email; an unchanged email is always accepted (`GuardReservedEmail`; Internal callers such as auth are exempt, so auth itself refuses the bootstrap email on the bare-email passkey registration path and leaves it to `break-glass --enroll`).
  - Any Admin on `ALL` may delete or suspend another (non-bootstrap) administrator, and nobody deletes their own account.
  - A user delete, suspension or `roleRefs`/`groupRefs` change, and a group delete, `roleRefs` change or member removal, that would leave no active user holding Admin on `ALL` (the bootstrap account counts) answers 409 for every caller, Internal included, so the deletes routed through auth are covered (`GuardUserPatchLastAdmin`, `GuardUserDeleteLastAdmin`, `GuardGroupPatchLastAdmin`, `GuardGroupDeleteLastAdmin`). The check reads the user list and then writes, so two concurrent removals of the last two admins can both pass. The built-in Admin role can't be edited or deleted; a custom role granting Admin on `ALL` is not covered.
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

- **Pod security** (`templates/_deployment.tpl`, `app.shared.podSecurityContext` and `containerSecurityContext` in `values.yaml`): non-root uid/gid 1001, no privilege escalation, read-only root filesystem, all capabilities dropped, seccomp `RuntimeDefault`, for every service but ui. ui (nginx, uid 101) sets `includeSecurity: false` and its own contexts in `services.ui`: non-root uid 101, no privilege escalation, all capabilities dropped, read-only root filesystem with `emptyDir` on `/var/cache/nginx` and `/tmp`, seccomp `RuntimeDefault`.
- **Kyverno policies from protection plans** are created only by discovery: namespaced Kyverno `Policy` objects, labelled `telark.io/protection-plan=<plan id>`, applied with server-side apply, `Enforce` or `Audit` from the plan (`data/policies/shared.go`, `services/discovery/internal/core/plans/protection/policies/applier.go`). See [protection plans](../architecture/protection-plans.md).
- **Chart-installed Kyverno policy** `telark-inject-modifier` (`templates/policies/clusterpolicy-image-inject.yaml`): a mutate policy, `failurePolicy: Ignore`, that stamps `telark.io/last-modified-by`, `-at` and `-operation` annotations on the kinds discovery snapshots (Secrets excluded, so Kyverno never mutates Secret writes for it), outside the telark and system namespaces. It renders only when the Kyverno `ClusterPolicy` API already exists, so a first install gets it on its first upgrade ([INSTALL.md](../INSTALL.md#last-modified-annotations)).
- **Kyverno fails open by default** (`app.kyverno.failOpen`, which must equal `kyverno.features.forceFailurePolicyIgnore.enabled`): when Kyverno's webhook is unavailable, admission (including plan policies in `Enforce`) lets requests through. Setting both to `false` makes enforcement fail closed ([INSTALL.md](../INSTALL.md#policy-engine-fail-open)).
- **CRD write guard** (`templates/policies/crd-admission-policy.yaml`, `app.crdGuard`, on and enforcing by default): a ValidatingAdmissionPolicy that matches every resource and subresource of `telark.io` (`*/*`, which covers status writes too) in every namespace (the binding has no namespace selector) and allows writes only from the exporter ServiceAccount (and `extraAllowedUsers`), plus discovery for `applications` with subresource `status`. A second policy guards the Secret `telark-oidc-trust-secret` by name, so namespace `edit` rights (which include Secrets) cannot plant a JWK set and mint OIDC logins. `enforce: false` only audits. The CRD schemas also pin `Session` names to `session-<64 hex>` and `User` `spec.status.phase` to `active`, `inactive` or `suspended`.
- **Network** (`templates/shared/networkpolicy.yaml`, `app.networkPolicy.enabled`, on by default): ingress default deny for every Telark pod; the service APIs accept only telark pods of the same release; ui accepts any source on its port; NATS accepts discovery and notifier on 4222 only (6222 between NATS pods) and runs no monitoring listener (the NATS Services still list 8222). Ollama's policy admits analyzer pods only (egress DNS, plus 443 when `app.ollama.autoPull` is on), the Redis subchart's admits clients labelled `<release>-redis-client`. Egress is open. None of it applies without a CNI that enforces NetworkPolicy.
- **NATS users**: discovery connects as the publisher (publish `telark.applications.*`, subscribe `_INBOX.>` for JetStream acks), notifier as the consumer (publish `$JS.API.>` and `$JS.ACK.>`, subscribe `telark.applications.*` and `_INBOX.>`), each from its own Secret.
- **Chart-generated secrets** (service token, NATS users) carry `helm.sh/resource-policy: keep`; cluster-less renders use `app.serviceToken.existingSecret` and `nats.existingSecrets` instead of `lookup` ([INSTALL.md](../INSTALL.md#gitops-cluster-less-renders)).

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

## Not provided by the code today

Facts an operator or reviewer should not assume otherwise:

- No request rate limiting or login lockout in any service.
- Redis runs without authentication; don't enable `redis.auth.enabled` (the services get no password, see SECURITY.md).
- A cluster admin, or anyone who may delete ValidatingAdmissionPolicies, can remove the CRD write guard; no Telark ServiceAccount can.
- Kyverno fails open unless the operator sets `app.kyverno.failOpen=false`.
- NetworkPolicies restrict ingress only; egress from every Telark pod is open.
- CORS in `x-ware/cors` sends no headers unless `CORS_ALLOWED_ORIGINS` lists origins (never `*`); the dashboard proxies every API on its own origin.
