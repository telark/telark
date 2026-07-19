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

## Scope notes

- Images are public and pull anonymously; the chart ships no registry credentials. For a private registry/mirror, supply pull secrets via `app.image.pullSecrets` / `global.imagePullSecrets`.
- The CRD write guard (`app.crdGuard`) is off by default; enable and enforce it to restrict who may write telark custom resources directly.
