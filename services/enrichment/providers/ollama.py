"""Ollama enrichment provider (local, OpenAI-compatible, instructor-backed).

Differs from the cloud providers in one way that matters: when Ollama is
unreachable the job must be re-queued, not answered with a fallback — so setup
and connection errors raise OllamaUnavailableError instead of returning a result.
"""

from app_logger import logger
from config import OLLAMA_HOST, OLLAMA_MODEL
from models import EnrichmentResult

from .base import InstructorProvider, fallback_result
from .config import OLLAMA_API_KEY, OLLAMA_RETRIES
from .constants import (
    LOG_PROVIDER_ATTEMPT_RETRY,
    LOG_PROVIDER_RETRIES_FAILED,
    LOG_PROVIDER_SETUP_FAILED,
    LOG_PROVIDER_UNREACHABLE,
    MSG_OLLAMA_UNREACHABLE,
)


class OllamaUnavailableError(Exception):
    """Raised when Ollama is unreachable."""


class OllamaProvider(InstructorProvider):
    name = "ollama"
    model = OLLAMA_MODEL
    retries = OLLAMA_RETRIES
    api_key_required = False

    def __init__(self) -> None:
        super().__init__(OLLAMA_API_KEY)

    def _resolve_base_url(self) -> str:
        return f"{OLLAMA_HOST.rstrip('/')}/v1"

    def _on_setup_error(self, e: Exception) -> EnrichmentResult:
        logger.error(LOG_PROVIDER_SETUP_FAILED, self.name, e)
        raise OllamaUnavailableError(MSG_OLLAMA_UNREACHABLE) from e

    def _handle_error(self, e: Exception, attempt: int) -> EnrichmentResult | None:
        msg = str(e).lower()
        if "connection" in msg or "refused" in msg or "unreachable" in msg:
            logger.error(LOG_PROVIDER_UNREACHABLE, self.name, e)
            raise OllamaUnavailableError(MSG_OLLAMA_UNREACHABLE) from e
        if attempt >= self.retries:
            logger.warning(LOG_PROVIDER_RETRIES_FAILED, self.name, self.retries, e)
            return fallback_result()
        logger.debug(LOG_PROVIDER_ATTEMPT_RETRY, self.name, attempt, e)
        return None
