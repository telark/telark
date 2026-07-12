"""Config-like constants used by enrichment providers (base URLs, retries)."""

# Ollama (instructor uses literal API key for local server)
OLLAMA_API_KEY = "ollama"
OLLAMA_RETRIES = 3

# Groq (OpenAI-compatible)
GROQ_BASE_URL = "https://api.groq.com/openai/v1"
GROQ_RETRIES = 3

# Gemini (OpenAI-compatible)
GEMINI_BASE_URL = "https://generativelanguage.googleapis.com/v1beta/openai/"
GEMINI_RETRIES = 3
