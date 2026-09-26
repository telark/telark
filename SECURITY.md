# Security Policy

## Reporting a vulnerability

Please report security issues **privately**. Do not open a public issue for anything exploitable.

- Preferred: [GitHub private vulnerability reporting](https://github.com/telark/telark/security/advisories/new).
- Or email: **contact@telark.io** with subject `SECURITY: <summary>`.

Include: affected component and version, a description, reproduction steps or a PoC, and impact. We aim to acknowledge within 3 business days and to agree on a disclosure timeline once the issue is confirmed.

## Supported versions

telark is pre-1.0. Only the latest released chart/app version receives security fixes.

| Version | Supported |
|---|---|
| latest | ✅ |
| older | ❌ |

## Credential storage

- **Session tokens are never stored.** A token is 32 bytes from `crypto/rand`; only its SHA-256 digest reaches the cluster, as the `UserSession` resource name (`session-<digest>`). An etcd snapshot therefore yields no usable session credential.
- **Encrypt Secrets at rest.** Kubernetes does not encrypt etcd by default. Self-hosted clusters do not need a cloud KMS for this — an `EncryptionConfiguration` with the local `aesgcm` provider and a key file on the control plane covers every Secret in the cluster. This is the highest-value hardening step for a telark install and requires no telark configuration.
- **Redis holds no credentials**, but the exporter caches authorization grants there. It ships with authentication disabled and a NetworkPolicy (`redis.networkPolicy.allowExternal: false`) admitting only pods labelled `<release>-redis-client`, in the same namespace as Redis. That policy is enforced only if your CNI implements NetworkPolicy — Calico and Cilium do, some setups silently ignore it. Redis is therefore treated as untrusted either way: every cache entry that drives an authorization decision is HMAC-signed with the service token (which never enters Redis), so a planted entry is rejected rather than obeyed. Do **not** set `redis.auth.enabled: true` — the chart wires no Redis password into the services, so enabling auth locks every service out of the cache.

## Scope notes

- Images are public and pull anonymously; the chart ships no registry credentials. For a private registry/mirror, supply pull secrets via `app.image.pullSecrets` / `global.imagePullSecrets`.
- The CRD write guard (`app.crdGuard`) is on and enforcing by default. telark's custom resources are its authorization data, so the guard is the boundary between namespace edit rights and telark Admin: keep it enforcing, and put break-glass identities in `app.crdGuard.extraAllowedUsers` rather than turning it off.
- The chart renders ingress NetworkPolicies for the telark services and NATS (`app.networkPolicy.enabled`); like Redis's, they need a CNI that enforces NetworkPolicy.
- The chart ships no bootstrap admin and no self-registration: set `app.auth.bootstrap.admins` to your own address at install and enrol it right away (docs/INSTALL.md, First admin).
