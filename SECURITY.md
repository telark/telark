# Security policy

## Reporting a vulnerability

Report security issues privately. Do not open a public issue for anything exploitable.

- Preferred: [GitHub private vulnerability reporting](https://github.com/telark/telark/security/advisories/new).
- Or email: **contact@telark.io** with subject `SECURITY: <summary>`.

Include the affected component and version, a description, reproduction steps or a proof of concept, and the impact. We aim to acknowledge reports within 3 business days and to agree on a disclosure timeline once the issue is confirmed.

## Supported versions

Telark is pre-1.0. Only the latest released chart/app version receives security fixes.

## Credential storage

- **Session tokens are never stored.** A token is 32 bytes from `crypto/rand`; only its SHA-256 digest reaches the cluster, as the `Session` resource name (`session-<digest>`). An etcd snapshot therefore yields no usable session credential.
- **Encrypt Secrets at rest.** Kubernetes does not encrypt etcd by default. No cloud KMS is needed: an `EncryptionConfiguration` with the local `aesgcm` provider and a key file on the control plane covers every Secret in the cluster. It is the most valuable hardening step for a Telark install and needs no Telark configuration.
- **Redis holds no credentials**, but the exporter caches authorization grants there. Redis requires a password, generated into `telark-redis-secret` (or a Secret you name in `redis.auth.existingSecret`) and given to every Telark pod that uses Redis as `REDIS_PASSWORD`; the chart refuses to render with `redis.auth.enabled=false`. A NetworkPolicy (`redis.networkPolicy.allowExternal: false`) admits only pods labeled `<release>-redis-client` in Redis's namespace, if your CNI enforces NetworkPolicy (Calico and Cilium do; some setups ignore it without warning). Every Telark service holds the password, so Redis is still treated as untrusted: every cache entry that drives an authorization decision is HMAC-signed with the service token (which never enters Redis), so a planted entry is rejected. Traffic to Redis is not encrypted.

## Scope notes

- Images are public and pull anonymously; the chart ships no registry credentials. For a private registry or mirror, set `app.image.pullSecrets` or `global.imagePullSecrets`.
- The CRD write guard (`app.crdGuard`) is on and enforcing by default. Telark's custom resources are its authorization data, so the guard is the boundary between namespace edit rights and telark Admin: keep it enforcing, and put break-glass identities in `app.crdGuard.extraAllowedUsers` rather than turning it off.
- The chart renders ingress NetworkPolicies for the Telark services and NATS (`app.networkPolicy.enabled`); they too need a CNI that enforces NetworkPolicy.
- The chart ships no bootstrap admin and no self-registration: set `app.auth.bootstrap.admin` to your own address at install and enroll it right away ([First admin](docs/INSTALL.md#2-first-admin)).
