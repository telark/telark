"""Provider base classes.

BaseProvider is the minimal contract. InstructorProvider implements the full
enrich flow shared by every OpenAI-compatible, instructor-backed provider
(groq, gemini, ollama); a concrete provider is then just a few class attributes
plus, where it differs, an error-policy override.
"""

import time
from abc import ABC, abstractmethod
from datetime import datetime

import instructor
from openai import OpenAI

from app_logger import logger
from models import AppSignals, EnrichmentResult, EnrichmentResultLLM
from prompts.k8s_app_analyzer_prompt import build_prompt

from .cache import cache_key, get_cached, set_cached, signals_hash
from .constants import (
    FALLBACK_CONFIDENCE,
    FALLBACK_ROLE,
    FALLBACK_SUMMARY,
    LOG_PROVIDER_API_KEY_INVALID,
    LOG_PROVIDER_ATTEMPT_RETRY,
    LOG_PROVIDER_CACHE_HIT,
    LOG_PROVIDER_COMPLETED,
    LOG_PROVIDER_RATE_LIMIT,
    LOG_PROVIDER_RETRIES_FAILED,
    LOG_PROVIDER_SETUP_FAILED,
    MSG_API_KEY_NOT_SET,
)


def fallback_result() -> EnrichmentResult:
    """The single fallback returned when a provider cannot produce a real result.
    Assessment fields fall back to their model defaults."""
    return EnrichmentResult(
        summary=FALLBACK_SUMMARY,
        role=FALLBACK_ROLE,
        confidence=FALLBACK_CONFIDENCE,
        enrichedAt=datetime.utcnow(),
    )


class BaseProvider(ABC):
    @abstractmethod
    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        """Enrich an application with LLM insights."""


class InstructorProvider(BaseProvider):
    """Concrete providers set these class attributes."""

    name: str = "provider"
    base_url: str = ""
    model: str = ""
    retries: int = 3
    api_key_required: bool = True

    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    # --- knobs a subclass may override ------------------------------------
    def _resolve_base_url(self) -> str:
        return self.base_url

    def _resolve_key(self) -> str:
        return self._api_key

    def _client(self) -> instructor.Instructor:
        if self.api_key_required and not self._api_key:
            raise ValueError(MSG_API_KEY_NOT_SET.format(self.name))
        return instructor.from_openai(
            OpenAI(base_url=self._resolve_base_url(), api_key=self._resolve_key()),
            mode=instructor.Mode.JSON,
        )

    def _on_setup_error(self, e: Exception) -> EnrichmentResult:
        logger.error(LOG_PROVIDER_SETUP_FAILED, self.name, e)
        return fallback_result()

    def _handle_error(self, e: Exception, attempt: int) -> EnrichmentResult | None:
        """Return a result to stop; None to keep retrying. May raise to signal
        unavailability."""
        msg = str(e).lower()
        if "authentication" in msg or "api key" in msg:
            logger.error(LOG_PROVIDER_API_KEY_INVALID, self.name, e)
            return fallback_result()
        if "rate" in msg or "limit" in msg or "quota" in msg:
            logger.warning(LOG_PROVIDER_RATE_LIMIT, self.name, e)
            return fallback_result()
        if attempt >= self.retries:
            logger.warning(LOG_PROVIDER_RETRIES_FAILED, self.name, self.retries, e)
            return fallback_result()
        logger.debug(LOG_PROVIDER_ATTEMPT_RETRY, self.name, attempt, e)
        return None

    # --- shared flow ------------------------------------------------------
    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        key = cache_key(signals_hash(signals))
        cached = get_cached(key)
        if cached is not None:
            logger.debug(LOG_PROVIDER_CACHE_HIT, self.name, signals.namespace, signals.name, key)
            return cached

        prompt = build_prompt(signals)
        start = time.perf_counter()

        try:
            client = self._client()
        except Exception as e:
            return self._on_setup_error(e)

        for attempt in range(1, self.retries + 1):
            try:
                resp = client.chat.completions.create(
                    model=self.model,
                    messages=[{"role": "user", "content": prompt}],
                    response_model=EnrichmentResultLLM,
                )
                elapsed_ms = int((time.perf_counter() - start) * 1000)
                logger.info(LOG_PROVIDER_COMPLETED, self.name, elapsed_ms, attempt)
                result = EnrichmentResult(**resp.model_dump(), enrichedAt=datetime.utcnow())
                set_cached(key, result)
                return result
            except Exception as e:
                outcome = self._handle_error(e, attempt)
                if outcome is not None:
                    return outcome

        return fallback_result()
