"""All text and message constants used by enrichment providers."""

# -----------------------------------------------------------------------------
# Fallback result (shared by all providers)
# -----------------------------------------------------------------------------
FALLBACK_SUMMARY = "Could not analyze — insufficient signals"
FALLBACK_ROLE = "backend-service"
FALLBACK_CONFIDENCE = "low"

# -----------------------------------------------------------------------------
# Ollama
# -----------------------------------------------------------------------------
LOG_OLLAMA_SETUP_FAILED = "Ollama client setup failed: {}"
MSG_OLLAMA_UNREACHABLE = "Ollama unreachable"
LOG_OLLAMA_UNREACHABLE = "Ollama unreachable: {}"
LOG_ENRICHMENT_COMPLETED = "Enrichment completed in {}ms (attempt {})"
LOG_INVALID_JSON_RETRIES = "Invalid or missing JSON after {} retries: {}"
LOG_FALLBACK_AFTER_MS = "Fallback result returned after {}ms"
LOG_ATTEMPT_RETRY = "Attempt {} failed, retrying: {}"

# -----------------------------------------------------------------------------
# Anthropic
# -----------------------------------------------------------------------------
MSG_ANTHROPIC_API_KEY_NOT_SET = "ANTHROPIC_API_KEY is not set"
LOG_ANTHROPIC_COMPLETED = "Anthropic enrichment completed in {}ms"
LOG_ANTHROPIC_API_UNREACHABLE = "Anthropic API unreachable: {}"
LOG_ANTHROPIC_API_KEY_INVALID = "Anthropic API key invalid: {}"
LOG_ANTHROPIC_PARSE_FAILED = "Anthropic response parse failed: {}"
LOG_ANTHROPIC_FAILED = "Anthropic enrichment failed: {}"

# -----------------------------------------------------------------------------
# Groq
# -----------------------------------------------------------------------------
MSG_GROQ_API_KEY_NOT_SET = "GROQ_API_KEY is not set"
LOG_GROQ_SETUP_FAILED = "Groq client setup failed: {}"
LOG_GROQ_COMPLETED = "Groq enrichment completed in {}ms (attempt {})"
LOG_GROQ_API_KEY_INVALID = "Groq API key invalid: {}"
LOG_GROQ_RATE_LIMIT = "Groq rate limit hit: {}"
LOG_GROQ_RETRIES_FAILED = "Groq failed after {} retries: {}"
LOG_GROQ_ATTEMPT_RETRY = "Groq attempt {} failed, retrying: {}"

# -----------------------------------------------------------------------------
# Gemini
# -----------------------------------------------------------------------------
MSG_GEMINI_API_KEY_NOT_SET = "GEMINI_API_KEY is not set"
LOG_GEMINI_SETUP_FAILED = "Gemini client setup failed: {}"
LOG_GEMINI_COMPLETED = "Gemini enrichment completed in {}ms (attempt {})"
LOG_GEMINI_API_KEY_INVALID = "Gemini API key invalid: {}"
LOG_GEMINI_RATE_LIMIT = "Gemini rate limit hit: {}"
LOG_GEMINI_RETRIES_FAILED = "Gemini failed after {} retries: {}"
LOG_GEMINI_ATTEMPT_RETRY = "Gemini attempt {} failed, retrying: {}"
