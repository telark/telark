# Architecture

telark is a control plane for **protection plans** over Kubernetes workloads: discover applications, bind policy templates to a scope and a window, decide what may change them at admission, and verify the running state against the cluster.

## System

```mermaid
flowchart LR
  UI[ui / operator]

  subgraph services["Services"]
    AUTH[auth]
    DISC[discovery]
    EXP[exporter]
    ENR[enrichment]
    NTF[notifier]
  end

  subgraph infra["Infrastructure"]
    REDIS[("Redis")]
    NATS[("NATS JetStream")]
    KYV[kyverno admission]
  end

  subgraph cluster["Cluster"]
    K8S[("Kubernetes API<br/>Telark CRDs")]
    PVC[("Snapshots PVC")]
  end

  LLM[["LLM provider"]]

  UI -->|login| AUTH
  UI -->|REST API| DISC
  UI -->|insights: windowed read| REDIS

  AUTH -->|CRDs| EXP
  AUTH --> REDIS

  DISC -->|watch / list| K8S
  DISC -->|read + store| EXP
  DISC -->|publish app events| NATS --> NTF -->|persist CR| EXP
  DISC -->|dispatch signals| ENR
  DISC <-->|coordination| REDIS
  DISC -->|protection plans| KYV
  KYV -->|admit / reject writes| K8S

  ENR -->|queue · cache| REDIS
  ENR -->|GlobalConfig| EXP
  ENR --> LLM

  EXP -->|read/write CRDs| K8S
  EXP -->|snapshots| PVC

  classDef svc fill:#4f46e5,stroke:#3730a3,color:#fff;
  classDef infra fill:#0f766e,stroke:#134e4a,color:#fff;
  classDef store fill:#b45309,stroke:#92400e,color:#fff;
  classDef peer fill:#475569,stroke:#334155,color:#fff;
  class AUTH,DISC,EXP,ENR,NTF svc;
  class NATS,KYV infra;
  class REDIS,K8S,PVC store;
  class UI,LLM peer;
```

Each service's own README carries a focused diagram of its internals: [auth](../services/auth/README.md) · [discovery](../services/discovery/README.md) · [exporter](../services/exporter/README.md) · [enrichment](../services/enrichment/README.md) · [notifier](../services/notifier/README.md).

## Services

| Service | Language | Responsibility |
|---|---|---|
| `exporter` | Go | Owns the CRDs and storage; seeds built-in resources; snapshots cluster state. The only stateful service. |
| `discovery` | Go | Groups workloads into applications; runs the leader-elected reconcile loop; dispatches enrichment; drives protection-plan lifecycle. |
| `enrichment` | Python / FastAPI | Generates AI insights for applications; provider-pluggable (groq, gemini, ollama, anthropic). |
| `auth` | Go | Passkey (WebAuthn) + Google OIDC login; session and role reconciliation. |
| `notifier` | Go | Notifications. |
| `ui` | — | Dashboard SPA (separate repo; the chart ships only the image reference). |

## Shared infrastructure (subcharts)

`redis` (coordination, queues, dedup), `nats` (messaging), `kyverno` (admission policy engine), `metrics-server` (HPAs / `kubectl top`), `ollama` (optional local LLM).

## Data flow (high level)

1. `exporter` watches the cluster and materializes telark custom resources (applications, groups, roles…).
2. `discovery`'s leader groups workloads into applications and, on a tick, dispatches application signals to `enrichment`.
3. `enrichment` calls the configured LLM and writes insights back; the UI polls a windowed read.
4. Protection plans transition `scheduled → active → terminated`; while active, admission policies are deployed and their health is verified against live cluster state.
5. `auth` authenticates operators (passkey/OIDC) and reconciles role custom resources.

## Identity

The app identity is a single value, `app.name` (default `telark`), shared by both charts and hardcoded in the API groups the Go services watch (`erpi.telark`, `auth.telark`, `classification.telark`). See [ADR 0002](adr/0002-app-name-is-the-identity-source-of-truth.md).

## Deeper references

- [Enrichment architecture](../ENRICHMENT-ARCHITECTURE.md)
- [OIDC architecture](../OIDC-ARCHITECTURE.md)
