"""Deterministic prompt builder from AppSignals."""

import hashlib
import inspect

from models import AppSignals


def build_prompt(signals: AppSignals) -> str:
    """Build a tight deterministic prompt for the LLM. No explanation, JSON only."""
    return f"""You are a Kubernetes application analyzer.
Analyze the signals below and return ONLY a JSON object.
Do not include any explanation, markdown, or text outside the JSON.

Application: {signals.name} (namespace: {signals.namespace})
Container images: {signals.images}
Exposed ports: {signals.ports}
Environment variable keys: {signals.envVarKeys}
Resource kinds: {signals.resourceKinds}
Has Ingress: {signals.hasIngress}
Has PersistentVolumeClaim: {signals.hasPVC}

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

Env var key → meaning mappings (use these to infer purpose):
  RP_ID / RP_ORIGIN / RP_NAME / CHALLENGE_TIMEOUT → WebAuthn/FIDO2 authentication
  NATS_USER / NATS_URL / NATS_HOST → NATS messaging integration
  REDIS_HOST / REDIS_URL → Redis data store integration
  STRIPE_KEY / PAYMENT_PROVIDER → payment processing
  JWT_SECRET / JWT_EXPIRY → JWT-based authentication
  OLLAMA_HOST / OLLAMA_MODEL → Ollama AI inference integration
  KAFKA_BROKER → Kafka event streaming
  POSTGRES_HOST / DATABASE_URL → PostgreSQL database integration
  SESSION_EXPIRY → session management
  REDIS_REPLICATION_MODE → Redis replication/clustering

For opaque private images (e.g. botriack/plsyro:auth-x.x.x):
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

For opaque private images infer from env keys and ports:
  NATS_USER / NATS_URL → NATS
  REDIS_HOST / REDIS_URL → Redis
  POSTGRES_HOST / DATABASE_URL → PostgreSQL
  OLLAMA_HOST → Ollama
  PORT 443 → HTTPS/TLS
  PORT 4222 → NATS
  PORT 6379 → Redis
  PORT 5432 → PostgreSQL
  RP_ID / RP_ORIGIN / CHALLENGE_TIMEOUT → WebAuthn
  SESSION_EXPIRY → Session management
  PAYMENT_PROVIDER / STRIPE_KEY → Payment processing

NEVER use image registry paths as tech stack entries:
  "botriack/plsyro", "bitnami/redis", "docker.io/nats" are INVALID
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

Use these mappings for both:
  REDIS_HOST / REDIS_URL → "redis"
  NATS_URL / NATS_USER / NATS_HOST → "nats"
  POSTGRES_HOST / DATABASE_URL / POSTGRES_URL → "postgres"
  KAFKA_BROKER / KAFKA_URL → "kafka"
  MONGO_URI / MONGODB_URL → "mongodb"
  MYSQL_HOST / MYSQL_URL → "mysql"
  OLLAMA_HOST / OLLAMA_URL → "ollama"
  ELASTICSEARCH_URL / ES_HOST → "elasticsearch"
  RABBITMQ_URL / RABBITMQ_HOST → "rabbitmq"
  S3_ENDPOINT / AWS_S3_BUCKET → "s3"
  VAULT_ADDR / VAULT_URL → "vault"

════════════════════════════════════════
RISKS RULES
════════════════════════════════════════
Identify ONLY real risks visible from the provided signals.
Do NOT invent risks not supported by the signals.
Do NOT mention "insufficient signals" as a risk.
Do NOT suggest adding Ingress or PVC unless clearly needed.
Return [] if no real risks are detected.
Maximum 5 risks, each a single concise sentence.

Signal → risk mappings:
  Single replica workload → "Single replica — no high availability"
  Sensitive env keys in plain env (REDIS_HOST, NATS_USER, JWT_SECRET,
  POSTGRES_HOST, STRIPE_KEY, API_KEY, SECRET, PASSWORD, TOKEN, KEY)
    → "Sensitive config exposed in plain env vars instead of Secrets"
  REDIS_TLS_ENABLED=false or absent → "TLS disabled — Redis traffic transmitted in plaintext"
  No PodDisruptionBudget with stateful workload → "No PodDisruptionBudget — vulnerable during node drain"

CRITICAL: if "NetworkPolicy" appears in resource kinds →
  a NetworkPolicy ALREADY EXISTS.
  Do NOT flag "No NetworkPolicy" as a risk.
  Only flag missing NetworkPolicy if NetworkPolicy is ABSENT from resource kinds.

════════════════════════════════════════
SUGGESTIONS RULES
════════════════════════════════════════
Provide ONLY actionable improvements directly supported by signals.
Do NOT suggest Ingress unless app clearly needs external access.
Do NOT suggest PVC unless app clearly needs persistence.
Do NOT invent suggestions unrelated to actual signals.
Return [] if no relevant suggestions exist.
Maximum 5 suggestions, each a single concise sentence.

Signal → suggestion mappings:
  REDIS_TLS_ENABLED present → "Set REDIS_TLS_ENABLED=true to encrypt Redis traffic"
  NATS_USER in plain env → "Move NATS_USER to a Kubernetes Secret"
  REDIS_HOST in plain env → "Move REDIS_HOST to a Kubernetes Secret"
  POSTGRES_HOST in plain env → "Move POSTGRES_HOST to a Kubernetes Secret"
  Single replica → "Add HorizontalPodAutoscaler to handle traffic spikes"
  No probes (inferred from sparse signals) → "Add liveness and readiness probes on port {{port}}"

════════════════════════════════════════
RELATED APPS RULES
════════════════════════════════════════
Same mappings as DEPENDENCIES above.
Each entry: name (lowercase) + reason (exact env key that revealed it).
Must be consistent with dependencies field — same services, same data.
Return [] if no dependencies detected from env var keys.

Return ONLY this exact JSON structure with no extra fields:
{{
  "summary": "one specific technical sentence about what this app does",
  "techStack": ["Technology1", "Technology2"],
  "role": "picked role or short custom description",
  "dependencies": ["service1", "service2"],
  "confidence": "high|medium|low",
  "category": "infrastructure|application|data|messaging|security",
  "risks": ["risk1", "risk2"],
  "suggestions": ["suggestion1", "suggestion2"],
  "relatedApps": [
    {{"name": "redis", "reason": "REDIS_HOST env key present"}}
  ]
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