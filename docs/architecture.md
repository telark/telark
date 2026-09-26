# Architecture

telark is a control plane for **protection plans** over Kubernetes workloads: discover applications, bind policy templates to a scope and a window, decide what may change them at admission, and verify the running state against the cluster.

## System

```mermaid
%%{init: {"theme":"base","themeVariables":{"fontFamily":"ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif","fontSize":"13px","lineColor":"#94a3b8","primaryColor":"#eef2ff","primaryBorderColor":"#6366f1","primaryTextColor":"#312e81","edgeLabelBackground":"#ffffff","clusterBkg":"#f8fafc","clusterBorder":"#e2e8f0"},"flowchart":{"curve":"basis","htmlLabels":true,"nodeSpacing":48,"rankSpacing":64,"padding":12}}}%%
flowchart LR
  UI(["ui / operator"])

  subgraph services["Services"]
    AUTH(auth)
    DISC(discovery)
    EXP(exporter)
    ANL(analyzer)
    NTF(notifier)
  end

  subgraph infra["Infrastructure"]
    REDIS[("Redis")]
    NATS[("NATS JetStream")]
    KYV(kyverno admission)
  end

  subgraph cluster["Cluster"]
    K8S[("Kubernetes API<br/>Telark CRDs")]
    PVC[("Snapshots PVC")]
    RPVC[("Reports PVC")]
  end

  LLM{{"Ollama<br/>local model (optional)"}}

  UI -->|login| AUTH
  UI -->|REST API| DISC
  UI -->|insights SSE| ANL
  UI -->|REST API| EXP

  AUTH -->|CRDs| EXP
  AUTH --> REDIS

  DISC -->|watch| K8S
  DISC -->|read + store| EXP
  DISC -->|publish| NATS --> NTF -->|persist CR| EXP
  NTF -->|reset on delete| DISC
  DISC -->|XADD insights:jobs| REDIS
  DISC <-->|coordinate| REDIS
  DISC -->|plans| KYV
  KYV -->|admit / reject| K8S

  ANL -->|jobs · insights| REDIS
  ANL -->|GlobalConfig| EXP
  ANL -->|permissions| AUTH
  ANL --> LLM
  ANL -->|GET workloads · events| K8S

  EXP -->|CRDs| K8S
  EXP -->|snapshots| PVC
  EXP -->|reports| RPVC

  classDef svc fill:#eef2ff,stroke:#6366f1,stroke-width:1.5px,color:#312e81;
  classDef infra fill:#ecfdf5,stroke:#10b981,stroke-width:1.5px,color:#065f46;
  classDef store fill:#fff7ed,stroke:#f59e0b,stroke-width:1.5px,color:#92400e;
  classDef peer fill:#f1f5f9,stroke:#94a3b8,stroke-width:1.5px,color:#334155;
  classDef ext fill:#faf5ff,stroke:#a855f7,stroke-width:1.5px,color:#6b21a8;
  class AUTH,DISC,EXP,ANL,NTF svc;
  class NATS,KYV infra;
  class REDIS,K8S,PVC,RPVC store;
  class UI peer;
  class LLM ext;
```

Each service's own README carries a focused diagram of its internals: [auth](../services/auth/README.md) · [discovery](../services/discovery/README.md) · [exporter](../services/exporter/README.md) · [analyzer](../services/analyzer/README.md) · [notifier](../services/notifier/README.md).

## Services

| Service | Language | Responsibility |
|---|---|---|
| `exporter` | Go | Owns the CRDs and storage; seeds built-in resources; stores workload snapshots and plan reports on its volumes. The only stateful service. |
| `discovery` | Go | Groups workloads into applications; runs the leader-elected reconcile loop; drives protection-plan lifecycle, approvals and reports. |
| `analyzer` | Python / FastAPI | Local analyzer: investigates incidents with a local model over read-only cluster tools; writes findings to Redis; streams updates to the UI (SSE). |
| `auth` | Go | Passkey (WebAuthn) + Google OIDC login; session and role reconciliation. |
| `notifier` | Go | Consumes discovery's application events from NATS and upserts the `ApplicationAsResource` CRs through `exporter`. |
| `ui` | — | Dashboard SPA (separate repo; the chart ships only the image reference). |

## Shared infrastructure (subcharts)

`redis` (coordination, queues, dedup), `nats` (messaging), `kyverno` (admission policy engine), `metrics-server` (HPAs / `kubectl top`), `vpa` (optional, `vpa.enabled`), `ollama` (the analyzer's model runtime, on by default; `app.ollama.enabled=false` skips it).

## Data flow (high level)

1. `discovery` watches workloads, groups them into applications and publishes each change as `telark.applications.update` on NATS; `notifier` upserts the `ApplicationAsResource` through `exporter`, which writes every telark custom resource.
2. When a change is an incident or a recovery, `discovery` appends an analysis job to the Redis stream `insights:jobs`.
3. `analyzer` (when enabled) consumes the job, investigates with a local model over read-only tools, writes the findings to Redis and streams updates to the UI; the UI reads the insights through `discovery`.
4. Protection plans transition `pending_approval → scheduled → active → terminated` (the approval step only when the plan requires it); while active, admission policies are deployed for the scope minus its exclusions, and their health is verified against live cluster state. `discovery` renders plan reports and `exporter` stores them on the reports volume.
5. `auth` authenticates operators (passkey/OIDC) and reconciles role custom resources. Each route is gated by the caller's role on its scope, and a custom role can withhold single actions with deny rules (see the [discovery](../services/discovery/README.md#api) and [exporter](../services/exporter/README.md#api) READMEs).

## Identity

The app identity is a single value, `app.name` (default `telark`), shared by both charts and hardcoded in the API groups the Go services watch (`erpi.telark`, `auth.telark`, `classification.telark`). See [ADR 0002](adr/0002-app-name-is-the-identity-source-of-truth.md).

## Deeper references

- [Components and flows](architecture/README.md): call graph, Redis and NATS usage, startup, flows
- [Protection plans](architecture/protection-plans.md): lifecycle, policies, violations, reports
- [Security model](security/README.md): authentication, authorization, RBAC, invariants
- [Testing and validation](testing/README.md)
- [Analyzer architecture](../services/analyzer/ARCHITECTURE.md)
- [OIDC / SSO architecture](../services/auth/OIDC.md)
