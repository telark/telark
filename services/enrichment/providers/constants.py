"""Text and constant values shared by enrichment providers.

Provider-specific log lines were collapsed into generic templates parameterised by
the provider name — the first "{}" is always the provider (groq, gemini, ollama,
anthropic).
"""

# -----------------------------------------------------------------------------
# Fallback result (shared by all providers)
# -----------------------------------------------------------------------------
FALLBACK_SUMMARY = "Could not analyze — insufficient signals"
FALLBACK_ROLE = "backend-service"
FALLBACK_CONFIDENCE = "low"

# -----------------------------------------------------------------------------
# Messages
# -----------------------------------------------------------------------------
MSG_API_KEY_NOT_SET = "{} API key is not set"
MSG_OLLAMA_UNREACHABLE = "Ollama unreachable"

# -----------------------------------------------------------------------------
# Generic provider log templates (first {} = provider name)
# -----------------------------------------------------------------------------
LOG_PROVIDER_CACHE_HIT = "{} cache hit for {}/{} (key={})"
LOG_PROVIDER_SETUP_FAILED = "{} client setup failed: {}"
LOG_PROVIDER_COMPLETED = "{} enrichment completed in {}ms (attempt {})"
LOG_PROVIDER_API_KEY_INVALID = "{} API key invalid: {}"
LOG_PROVIDER_RATE_LIMIT = "{} rate limit hit: {}"
LOG_PROVIDER_RETRIES_FAILED = "{} failed after {} retries: {}"
LOG_PROVIDER_ATTEMPT_RETRY = "{} attempt {} failed, retrying: {}"
LOG_PROVIDER_UNREACHABLE = "{} API unreachable: {}"
LOG_PROVIDER_PARSE_FAILED = "{} response parse failed: {}"
LOG_PROVIDER_FAILED = "{} enrichment failed: {}"
