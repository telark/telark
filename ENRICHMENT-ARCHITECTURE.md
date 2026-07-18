# AI Insights — How it works

Telark generates AI insights for each discovered app. This is a simple map of the
pieces and the order things happen in.

```mermaid
%%{init: {'flowchart': {'curve': 'basis'}}}%%
flowchart TB
    user([Operator]):::actor
    ui["Web UI<br/>shows insights"]:::client

    subgraph cluster["Inside the Kubernetes cluster"]
        disco["discovery-service<br/>watches your apps"]:::svc
        enr["enrichment-service<br/>runs the AI analysis"]:::svc
        exp["exporter-service<br/>stores apps + AI settings"]:::svc
    end

    subgraph data["Redis (shared)"]
        jobs[("job queue")]:::store
        cache[("results cache")]:::store
    end

    llm["AI model<br/>Anthropic · Groq · Gemini · Ollama"]:::ext

    user --> ui
    ui -->|"1 · ask for insights"| disco
    disco -->|"2 · read AI settings"| exp
    disco -->|"3 · send apps to analyze"| enr
    enr -->|"4 · add to queue"| jobs
    jobs -->|"5 · worker picks up"| enr
    enr -->|"6 · pick provider + key"| exp
    enr -->|"7 · ask the AI"| llm
    enr -->|"8 · save result"| cache
    disco -->|"9 · read results"| cache

    classDef svc fill:#e6effc,stroke:#4f8ce6,color:#123663
    classDef store fill:#fcefd7,stroke:#dc9a3c,color:#6d420b
    classDef ext fill:#ede4fb,stroke:#9871e4,color:#412879
    classDef client fill:#daf4ea,stroke:#34b892,color:#094f3c
    classDef actor fill:#e2e9f2,stroke:#8ea3ba,color:#2c3f55
```

## Step by step

1. The **Web UI** opens an app and asks **discovery-service** for its insights.
2. On a timer (every 5 minutes), **discovery-service** reads the AI settings
   (is AI on? which provider?) from **exporter-service**.
3. If AI is on, it sends the current apps to **enrichment-service** to analyze.
   This just hands off the work — it does **not** wait for the AI.
4. enrichment-service drops each app it hasn't seen yet into a Redis **job queue**.
5. A worker picks up a job from the queue.
6. The worker looks up which AI provider and API key to use.
7. It asks the AI model to analyze the app.
8. It saves the answer into the Redis **results cache**.
9. The Web UI keeps polling discovery-service, which reads those saved results
   back from the cache and shows them.

The key idea: **sending work (steps 3–4) and reading results (step 9) are
separate.** The AI can be slow, but the UI never freezes waiting for it — results
just appear on the next poll.

## Notes for developers

- **Redis is shared** by discovery and enrichment on **DB 0** (auth uses DB 1).
  The job queue is the list `enrichment:jobs`; each result is cached under
  `enrichment:<namespace>:<name>`.
- **exporter-service is the only service that reads/writes cluster config** (the
  `GlobalConfig` CR). Both discovery and enrichment read AI settings through it —
  never from environment variables.
- The **timer is the only trigger.** (An older on-demand path existed but was dead
  code and has been removed.)
- Input `Signal` type lives in `internal/data/insights`; the result type
  (`Insights`) carries summary, tech stack, role, dependencies, confidence,
  category, risks (with severity), suggestions (with priority), resource
  efficiency, criticality, and tags.
- Each cached result records a prompt version. Editing the analysis prompt
  automatically refreshes stale cached results.
