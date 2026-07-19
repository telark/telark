"""Deterministic prompt builder from AppSignals."""

import hashlib
import inspect

from models import AppSignals


def _fmt(values: list) -> str:
    """Render a signal list, marking emptiness explicitly so the model does not guess."""
    return ", ".join(str(v) for v in values) if values else "(none provided)"


def _fmt_workloads(workloads: list) -> str:
    """One compact line per workload: kind, replicas, QoS, usage, whether limits are set."""
    if not workloads:
        return "  (none provided)"
    return "\n".join(
        f"  - {w.name} {w.kind} x{w.replicas} "
        f"QoS={w.qos or '?'} cpu={w.cpu or '?'} mem={w.memory or '?'} "
        f"limitsSet={w.limitsSet}"
        for w in workloads
    )


def build_prompt(signals: AppSignals) -> str:
    """Build a tight deterministic prompt for the LLM. No explanation, JSON only."""
    return f"""You are a Kubernetes application analyzer.
Analyze the signals below and return ONLY a JSON object.
Do not include any explanation, markdown, or text outside the JSON.
Base every field strictly on the signals. A "(none provided)" signal is
missing data, NOT evidence of absence — never invent values to fill it, and
lower your confidence when core signals are missing.

Application: {signals.name} (namespace: {signals.namespace})
Container images: {_fmt(signals.images)}
Exposed ports: {_fmt(signals.ports)}
Environment variable keys: {_fmt(signals.envVarKeys)}
Resource kinds: {_fmt(signals.resourceKinds)}
Workload kinds: {_fmt(signals.workloadKinds)}
Has Ingress: {signals.hasIngress} | Has Service: {signals.hasService} | Has HPA: {signals.hasHPA} | Has NetworkPolicy: {signals.hasNetworkPolicy} | Has PVC: {signals.hasPVC}
Replicas: {signals.replicas} (ready {signals.readyReplicas}) | Health: {signals.healthStatus or "unknown"}
Secret refs: {_fmt(signals.secretRefs)} | ConfigMap refs: {_fmt(signals.configMapRefs)}
Managed by: {signals.managedBy or "unknown"} {signals.chart}
Stability: {signals.changeVelocityPerDay}/day changes, {signals.incidents} incidents, {signals.recoveries} recoveries
Workloads (usage snapshot):
{_fmt_workloads(signals.workloads)}

════════════════════════════════════════
SIGNAL DICTIONARY  (single source of truth — every section below uses this)
════════════════════════════════════════
Env keys / ports map to a technology, a dependency name, and a purpose.
Match keys case-insensitively; a prefix match counts (REDIS_URL matches REDIS_).

  Env key(s)                          | port | techStack   | dependency | purpose
  REDIS_HOST/REDIS_URL                | 6379 | Redis       | redis      | Redis data store
  NATS_URL/NATS_USER/NATS_HOST        | 4222 | NATS        | nats       | NATS messaging
  POSTGRES_HOST/DATABASE_URL/POSTGRES_URL | 5432 | PostgreSQL | postgres | PostgreSQL database
  MYSQL_HOST/MYSQL_URL                | 3306 | MySQL       | mysql      | MySQL database
  MONGO_URI/MONGODB_URL               |      | MongoDB     | mongodb    | MongoDB database
  KAFKA_BROKER/KAFKA_URL              | 9092 | Kafka       | kafka      | Kafka event streaming
  RABBITMQ_URL/RABBITMQ_HOST          | 5672 | RabbitMQ    | rabbitmq   | RabbitMQ messaging
  ELASTICSEARCH_URL/ES_HOST           |      | Elasticsearch | elasticsearch | search/indexing
  OLLAMA_HOST/OLLAMA_URL/OLLAMA_MODEL |      | Ollama      | ollama     | Ollama AI inference
  S3_ENDPOINT/AWS_S3_BUCKET           |      | S3          | s3         | object storage
  VAULT_ADDR/VAULT_URL                |      | Vault       | vault      | secret management
  RP_ID/RP_ORIGIN/RP_NAME/CHALLENGE_TIMEOUT |      | WebAuthn | —      | WebAuthn/FIDO2 authentication
  JWT_SECRET/JWT_EXPIRY               |      | JWT         | —          | JWT-based authentication
  SESSION_EXPIRY                      |      | —           | —          | session management
  STRIPE_KEY/PAYMENT_PROVIDER         |      | Payments    | —          | payment processing
  (port 80/443)                       | 443  | HTTPS/TLS   | —          | HTTP(S) serving

A "—" dependency means it is a capability of THIS app, not an external service:
it belongs in techStack but NOT in dependencies/relatedApps.

════════════════════════════════════════
CONFIDENCE RULES
════════════════════════════════════════
- high: images AND ports AND envVarKeys are ALL present and non-empty
         AND you can write a specific technical summary without guessing
- medium: at least two of (images, ports, envVarKeys) are present
           OR you can infer the purpose with reasonable certainty
- low: only name/kinds available, signals are sparse, or purpose is unclear
       NEVER use high or medium if you are guessing

════════════════════════════════════════
ROLE RULES
════════════════════════════════════════
Pick the single best fit. If none fit exactly, use a short
lowercase hyphenated description (e.g. "webhook-handler",
"certificate-manager", "data-exporter").

Roles and their signals:
- gateway: has Ingress, exposes port 80 or 443, routes external traffic
- frontend: serves UI — image contains nginx/node/react/next/vue
- backend-service: business logic service with a Service resource and env vars
- worker: PURELY background processing with NO Service resource
           (batch jobs, queue consumers, cron tasks)
           Do NOT assign worker just because a PVC is present —
           a PVC alone means persistence, not background processing
- database: persistent data store — image contains postgres/mysql/mongo/mariadb
- messaging: message broker — image contains nats/kafka/rabbitmq
- cache: in-memory store — image contains redis/memcached
- operator: manages K8s resources, admission controllers, webhooks
- proxy: traffic routing, load balancing, ingress controllers
- monitor: metrics collection, logging, observability, tracing
- ai-runtime: runs AI/ML inference — image contains ollama/triton/torchserve
             or OLLAMA_HOST env key is present

════════════════════════════════════════
CATEGORY RULES
════════════════════════════════════════
Pick exactly one. When in doubt use these tiebreakers:

- infrastructure: system-level components, operators, runtimes,
                  AI runtimes, admission webhooks, cert managers
- application: business logic services that process user or system requests
- data: databases, caches, persistent storage systems
- messaging: message brokers, event queues, pub/sub systems,
             AND services whose PRIMARY purpose is routing messages
             between other services (even if they are custom code)
- security: auth services, certificate managers, policy enforcers,
            secret managers, identity providers

Tiebreaker — messaging vs application:
  If NATS_USER or NATS_URL is the ONLY meaningful env signal
  AND the app name suggests routing/sync/notification → messaging
  If the app has broader business logic beyond just messaging → application

════════════════════════════════════════
SUMMARY RULES
════════════════════════════════════════
Write ONE specific technical sentence describing what this app
actually does based on the signals. Be precise and concrete.
Infer purpose from the SIGNAL DICTIONARY above; name the concrete
technologies and what the app does WITH them.

For opaque private images (e.g. org/private-app:auth-x.x.x):
  Rely on app name + env var keys + ports to describe purpose.
  The image tag prefix (auth, sync, con, not, exp, ao, enrich)
  often reveals the service role — use it.

FORBIDDEN generic summaries — never write these:
  "handles business logic"
  "exposes a service on port X"
  "application handles X services"
  "manages Kubernetes resources"
  "handles data X using Y"  ← too vague
  "provides X service"       ← too vague
  "synchronizes data between services" ← too vague, always add HOW and WHAT

GOOD summary examples:
  "Handles WebAuthn/FIDO2 authentication flows including session management and relying party validation"
  "Consumes NATS messages and deduplicates events using Redis-backed persistent storage before forwarding"
  "Exports cluster application metrics and health data to external systems via Redis-backed state tracking"
  "Admission webhook controller enforcing custom policies on Kubernetes resource creation and updates via port 443"
  "Runs Ollama AI inference server for local LLM model serving with persistent model storage"
  "Processes payment transactions via PAYMENT_PROVIDER with Redis caching and PostgreSQL persistence"
  "Delivers notifications to downstream consumers by subscribing to NATS subjects and forwarding events"

════════════════════════════════════════
TECHSTACK RULES
════════════════════════════════════════
Infer from image name, ports, and env var keys.
Use the techStack column of the SIGNAL DICTIONARY. Also add the app's own
runtime when the image name reveals it (nginx, node, python, go, java).

NEVER use image registry paths as tech stack entries:
  "acme/private-app", "bitnami/redis", "docker.io/nats" are INVALID
Use clean names only: "Redis", "NATS", "Node.js", "PostgreSQL", "WebAuthn"

════════════════════════════════════════
DEPENDENCIES RULES
════════════════════════════════════════
IMPORTANT: dependencies and relatedApps must be consistent.
dependencies = list of service names (strings)
relatedApps = same services with reasons

Both must be derived from the SAME env key signals.
If relatedApps contains "redis" then dependencies must contain "redis".
If dependencies is empty then relatedApps must be empty too.

Use the dependency column of the SIGNAL DICTIONARY. Only include entries whose
dependency name is a real service (not "—"): WebAuthn, JWT, and payments are
capabilities of this app, so they NEVER appear here.

════════════════════════════════════════
RISKS RULES  →  [{{"severity": "high|medium|low", "message": "..."}}]
════════════════════════════════════════
Report ONLY risks the signals above actually show. Each risk is an object with a
severity and a one-sentence message. Max 5. Return [] if none. NEVER invent a
risk that no signal value supports.

Grounded risk sources (read the real values above):
- replicas == 1 AND workloadKinds has Deployment or StatefulSet
    → medium "Single replica — no high availability"
- healthStatus is degraded or down
    → high "Workload unhealthy ({{readyReplicas}}/{{replicas}} replicas ready)"
- a workload with QoS == BestEffort
    → high "BestEffort QoS on {{workload}} — first evicted under memory pressure"
- a workload with limitsSet == false
    → medium "No CPU/memory limits on {{workload}} — resource usage is unbounded"
- hasNetworkPolicy == false
    → low "No NetworkPolicy — pod traffic is unrestricted"
- sensitive env keys present (SECRET/PASSWORD/TOKEN/KEY/API_KEY, or REDIS_HOST/
  NATS_USER/POSTGRES_HOST/STRIPE_KEY) AND secretRefs is empty
    → medium "Sensitive config in plain env vars, not Secret refs"
    (if secretRefs is NON-empty, assume values come from there — do NOT flag)
- incidents > 0
    → low "{{incidents}} incident(s) in recent change history"

════════════════════════════════════════
SUGGESTIONS RULES  →  [{{"priority": "high|medium|low", "message": "..."}}]
════════════════════════════════════════
Actionable improvements, each backed by a signal above. Max 5. Return [] if none.
- any workload with limitsSet == false
    → high "Set CPU and memory limits on {{workload}}"
- replicas == 1 AND hasHPA == false AND hasService == true
    → medium "Add a HorizontalPodAutoscaler or raise replicas for availability"
- sensitive env keys present AND secretRefs empty
    → high "Move sensitive config into Secret references"
- hasNetworkPolicy == false
    → low "Add a NetworkPolicy to restrict pod-to-pod traffic"
Do NOT suggest Ingress or PVC unless a signal clearly calls for it.

════════════════════════════════════════
RESOURCE EFFICIENCY RULES  →  {{"status": "over|under|balanced|unknown", "note": "..."}}
════════════════════════════════════════
Judge from the workloads' usage vs their limits:
- unknown:  no workloads, or none report cpu/mem usage
- under:    a workload has real usage but limitsSet == false (bursts unbounded),
            or usage sits close to / above a set limit
- over:     workloads are sized far above their observed usage (idle waste)
- balanced: usage sits sensibly within limits
note: one sentence naming the workload and the numbers you used.

════════════════════════════════════════
CRITICALITY RULES  →  {{"level": "critical|high|medium|low", "reason": "..."}}
════════════════════════════════════════
How important is this app to the platform? Base it on exposure, statefulness and
incident history:
- critical/high: internet-facing (hasIngress, gateway/frontend role, port 443),
  OR a stateful data store (database/cache/messaging role, or StatefulSet + PVC)
- raise one level if incidents > 0 (recent instability)
- medium: internal stateless backend that others depend on
- low: isolated or ancillary workload
reason: one sentence citing the signals used.

════════════════════════════════════════
TAGS RULES  →  ["tag1", "tag2", ...]
════════════════════════════════════════
3-6 short lowercase keyword tags for search and filtering. Single or hyphenated
words, never sentences. Draw from workload shape (stateless/stateful), exposure
(public-facing/internal), backing stores (redis-backed/postgres-backed),
management (helm-managed) and role.

════════════════════════════════════════
RELATED APPS RULES
════════════════════════════════════════
Same mappings as DEPENDENCIES above. Each entry: name (lowercase) + reason
(exact env key that revealed it). Must match the dependencies field exactly.
Return [] if no dependencies detected from env var keys.

Return ONLY this exact JSON structure with no extra fields:
{{
  "summary": "one specific technical sentence about what this app does",
  "techStack": ["Technology1", "Technology2"],
  "role": "picked role or short custom description",
  "dependencies": ["service1", "service2"],
  "confidence": "high|medium|low",
  "category": "infrastructure|application|data|messaging|security",
  "risks": [{{"severity": "high|medium|low", "message": "..."}}],
  "suggestions": [{{"priority": "high|medium|low", "message": "..."}}],
  "resourceEfficiency": {{"status": "over|under|balanced|unknown", "note": "..."}},
  "criticality": {{"level": "critical|high|medium|low", "reason": "..."}},
  "tags": ["tag1", "tag2"],
  "relatedApps": [{{"name": "redis", "reason": "REDIS_HOST env key present"}}]
}}"""

def get_prompt_version() -> str:
    """
    Compute a short stable hash of the build_prompt source.
    Changes automatically whenever build_prompt is modified.
    First 8 chars of SHA256 of the function source code.
    """
    source = inspect.getsource(build_prompt)
    return hashlib.sha256(source.encode()).hexdigest()[:8]


# Module-level constant — computed once at import time
PROMPT_VERSION: str = get_prompt_version()