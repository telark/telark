# Login and single sign-on

How Google sign-in works in Telark, for admins setting it up and for contributors. There
are two flows: an admin turns SSO on (letters A–C), then a user signs in (numbers 1–5).
Passkey sign-in and the first admin are covered in [First admin](../../docs/INSTALL.md#2-first-admin).

```mermaid
%%{init: {"theme":"base","themeVariables":{"fontFamily":"ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif","fontSize":"13px","lineColor":"#94a3b8","primaryColor":"#eef2ff","primaryBorderColor":"#6366f1","primaryTextColor":"#312e81","edgeLabelBackground":"#ffffff","clusterBkg":"#f8fafc","clusterBorder":"#e2e8f0"},"flowchart":{"curve":"basis","htmlLabels":true,"nodeSpacing":46,"rankSpacing":54,"padding":12}}}%%
flowchart TB
    admin(["Admin"]):::actor
    user(["User"]):::actor
    settings("Settings → Single Sign-On<br/>web UI"):::client
    login("Login page<br/>web UI"):::client

    subgraph cluster["Inside the Kubernetes cluster"]
        auth("auth<br/>handles login &amp; Google SSO"):::svc
        exp("exporter<br/>stores the settings"):::svc
    end

    google{{"Google<br/>Sign-In"}}:::ext

    admin --> settings
    settings -->|"A · save SSO settings (bootstrap admin only)"| auth
    auth -->|"B · check the settings"| auth
    auth -->|"C · store the settings"| exp

    user --> login
    login -->|"1 · get SSO info"| auth
    user -->|"2 · sign in with Google"| google
    login -->|"3 · send Google token"| auth
    auth -->|"4 · verify token with Google"| google
    auth -->|"5 · create a session"| user

    classDef svc fill:#eef2ff,stroke:#6366f1,stroke-width:1.5px,color:#312e81;
    classDef ext fill:#faf5ff,stroke:#a855f7,stroke-width:1.5px,color:#6b21a8;
    classDef client fill:#f1f5f9,stroke:#94a3b8,stroke-width:1.5px,color:#334155;
    classDef actor fill:#f8fafc,stroke:#cbd5e1,stroke-width:1.5px,color:#475569;
```

## Turning SSO on (admin)

- **A.** The bootstrap admin fills in the SSO settings (Google client ID and the options below) on
  Settings → Single Sign-On; everyone else sees them read-only. In the Google OAuth client, register
  `https://<host>/auth/google/callback` as an authorized redirect URI: the dashboard sends it
  as `redirect_uri`.
- **B.** auth-service checks that the settings are usable before saving them, so a
  configuration that can't sign anyone in is never stored.
- **C.** auth-service saves the settings through exporter-service.

## Signing in (user)

1. The login page asks auth-service what sign-in options are on (it learns the
   Google client ID). The Google button only shows if SSO is configured.
2. The user signs in with Google and gets a Google token.
3. The login page sends that token to auth-service.
4. auth-service checks the token really came from Google.
5. If it's valid, auth-service finds the user by the token's subject. On a first
   login it attaches the Google identity to the one user whose email matches, if
   that account has no identity yet, or creates a user when none does; two users
   with that email, or an account that already signs in another way, are refused
   (409). The bootstrap admin only ever signs in with its passkey: a login that
   resolves to it, by its email or by a Google identity bound to it before it was
   promoted, is refused (403, `the bootstrap administrator signs in with a passkey`).
   A created user always gets the ReadOnly role, even with the bootstrap email:
   SSO never grants Admin or the `bootstrap` marker (only `break-glass` does).
   A user being deleted or whose account is not active gets no session (403;
   a suspended account is told `user account is suspended`).
   Otherwise auth-service creates a Telark session and the user is logged in.

## How auth-service trusts Google

To check a Google token in step 4, auth-service needs Google's public keys. There
are two modes, controlled by one setting:

- **Cluster can reach the internet** (`egressAllowed = true`): auth-service fetches
  Google's keys directly and refreshes them automatically.
- **Air-gapped cluster** (`egressAllowed = false`): no internet. An admin pastes
  Google's public keys into the settings, and auth-service trusts **only** those.

## Notes for developers

- SSO settings live in the `TelarkConfig` CR named `default`, under `oidc`
  (`enabled`, `googleClientID`, `egressAllowed`). The pasted key set
  (`googleJwkJson`) is the trust anchor, so it lives apart in the Secret
  `telark-oidc-trust-secret` (key `googleJwkJson`, or the Secret named by
  `app.auth.oidc.existingSecret`), guarded by its own admission policy.
  auth-service reads it as a mounted file (`OIDC_TRUST_FILE`) and re-reads the
  settings per login, so changes take effect without a restart; other auth
  replicas see a new key set after the kubelet sync (about a minute).
  `GET /api/v1/config` on the exporter returns the key set merged back under
  `oidc.googleJwkJson`.
- Clearing the key set: send `"googleJwkJson": ""` (or `null`) in
  `PATCH /auth/oidc/config`. The exporter empties the Secret key, and the replica that
  handled the request stops trusting the old set at once (others after the kubelet
  sync). Omitting the key keeps the stored set. Once cleared, an enabled
  configuration with `egressAllowed = false` is refused (400) until a new set is sent.
- Endpoints: `GET /auth/config` is **public** and returns `selfRegistrationEnabled`,
  `oidcEnabled` and, only when SSO is usable, `googleClientID` (not a secret), from a
  cached config that asks the exporter at most once every 5 seconds;
  `PATCH /auth/oidc/config` saves settings and needs the bootstrap account (its own
  user record's `bootstrap` marker, whatever its grants) on top of Admin on `settings`
  and the `settings.editoidcconfig.deny` rule; `POST /auth/oidc/google/callback` handles the
  token from step 3.
- auth-service validates the settings and writes them with `PATCH /api/v1/config`
  on exporter-service using a service token (the exporter puts the key set in
  the Secret and the rest in the CR). The exporter takes the `oidc` block from a
  service only, so auth's bootstrap check is what enforces authorization.
