# Login & Single Sign-On — How it works

How signing in with Google works in Telark. There are two separate flows: an
**admin turns SSO on** (letters A–C), and then a **user signs in** (numbers 1–5).

```mermaid
%%{init: {'flowchart': {'curve': 'basis'}}}%%
flowchart TB
    admin([Admin]):::actor
    user([User]):::actor
    settings["Settings → SSO<br/>web UI"]:::client
    login["Login page<br/>web UI"]:::client

    subgraph cluster["Inside the Kubernetes cluster"]
        auth["auth-service<br/>handles login &amp; Google SSO"]:::svc
        exp["exporter-service<br/>stores the settings"]:::svc
    end

    google["Google<br/>Google Sign-In"]:::ext

    admin --> settings
    settings -->|"A · save SSO settings (admin only)"| auth
    auth -->|"B · check the settings"| auth
    auth -->|"C · store the settings"| exp

    user --> login
    login -->|"1 · get SSO info"| auth
    user -->|"2 · sign in with Google"| google
    login -->|"3 · send Google token"| auth
    auth -->|"4 · verify token with Google"| google
    auth -->|"5 · create a session"| user

    classDef svc fill:#e6effc,stroke:#4f8ce6,color:#123663
    classDef ext fill:#ede4fb,stroke:#9871e4,color:#412879
    classDef client fill:#daf4ea,stroke:#34b892,color:#094f3c
    classDef actor fill:#e2e9f2,stroke:#8ea3ba,color:#2c3f55
```

## Turning SSO on (admin)

- **A** — An admin fills in the SSO settings (Google client ID, etc.) on the
  Settings → SSO page. Only an admin can do this.
- **B** — **auth-service** checks the settings are usable before saving them
  (a broken config that can't sign anyone in must never be stored).
- **C** — auth-service saves the settings through **exporter-service**.

## Signing in (user)

1. The login page asks auth-service what sign-in options are on (it learns the
   Google client ID). The Google button only shows if SSO is configured.
2. The user signs in with Google and gets a Google token.
3. The login page sends that token to auth-service.
4. auth-service checks the token really came from Google.
5. If it's valid, auth-service creates a Telark session and the user is logged in.

## How auth-service trusts Google (the important part)

To check a Google token in step 4, auth-service needs Google's public keys. There
are two modes, controlled by one setting:

- **Cluster can reach the internet** (`egressAllowed = true`): auth-service fetches
  Google's keys directly and refreshes them automatically.
- **Air-gapped cluster** (`egressAllowed = false`): no internet. An admin pastes
  Google's public keys into the settings, and auth-service trusts **only** those.

## Notes for developers

- All SSO settings live in one place: `GlobalConfig.oidc`
  (`enabled`, `googleClientID`, `egressAllowed`, `googleJwkJson`). auth-service is
  the only service that reads and validates it, and it re-reads per login — so
  changes take effect without a restart.
- Endpoints: `GET /auth/config` is **public** (returns only the client ID, which is
  not a secret); `PATCH /auth/oidc/config` saves settings and is guarded by
  `editoidcconfig = settings:Admin`; `POST /auth/oidc/google/callback` handles the
  token from step 3.
- auth-service writes the settings to `GlobalConfig` through exporter-service using
  a service token; the route's Admin check is what enforces authorization.
