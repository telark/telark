"""Ollama enrichment provider with signal-based response caching."""

import time
from datetime import datetime

import instructor
from openai import OpenAI

from app_logger import logger
from config import OLLAMA_HOST, OLLAMA_MODEL
from models import AppSignals, EnrichmentResult, EnrichmentResultLLM
from prompts.k8s_app_analyzer_prompt import build_prompt

from .base import BaseProvider
from .cache import cache_key, get_cached, set_cached, signals_hash
from .config import OLLAMA_API_KEY, OLLAMA_RETRIES
from .constants import (
    FALLBACK_CONFIDENCE,
    FALLBACK_ROLE,
    FALLBACK_SUMMARY,
    LOG_ATTEMPT_RETRY,
    LOG_ENRICHMENT_COMPLETED,
    LOG_FALLBACK_AFTER_MS,
    LOG_INVALID_JSON_RETRIES,
    LOG_OLLAMA_SETUP_FAILED,
    LOG_OLLAMA_UNREACHABLE,
    MSG_OLLAMA_UNREACHABLE,
)


class OllamaUnavailableError(Exception):
    """Raised when Ollama is unreachable."""
    pass


def _create_client() -> instructor.Instructor:
    base_url = f"{OLLAMA_HOST.rstrip('/')}/v1"
    return instructor.from_openai(
        OpenAI(base_url=base_url, api_key=OLLAMA_API_KEY),
        mode=instructor.Mode.JSON,
    )


class OllamaProvider(BaseProvider):
    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        sig_hash = signals_hash(signals)
        key = cache_key(sig_hash)

        # Cache hit — skips ~40s Ollama inference entirely
        cached = get_cached(key)
        if cached is not None:
            logger.debug(
                "Ollama cache hit for %s/%s (hash=%s) — skipped inference",
                signals.namespace, signals.name, sig_hash,
            )
            return cached

        prompt = build_prompt(signals)
        start = time.perf_counter()

        try:
            client = _create_client()
        except Exception as e:
            logger.error(LOG_OLLAMA_SETUP_FAILED, e)
            raise OllamaUnavailableError(MSG_OLLAMA_UNREACHABLE) from e

        for attempt in range(1, OLLAMA_RETRIES + 1):
            try:
                resp = client.chat.completions.create(
                    model=OLLAMA_MODEL,
                    messages=[{"role": "user", "content": prompt}],
                    response_model=EnrichmentResultLLM,
                )
                elapsed_ms = int((time.perf_counter() - start) * 1000)
                logger.info(LOG_ENRICHMENT_COMPLETED, elapsed_ms, attempt)

                result = EnrichmentResult(
                    summary=resp.summary,
                    techStack=resp.techStack,
                    role=resp.role,
                    dependencies=resp.dependencies,
                    confidence=resp.confidence,
                    enrichedAt=datetime.utcnow(),
                    category=resp.category,
                    risks=resp.risks,
                    suggestions=resp.suggestions,
                    relatedApps=resp.relatedApps,
                )
                set_cached(key, result)
                return result

            except Exception as e:
                err_msg = str(e).lower()
                if "connection" in err_msg or "refused" in err_msg or "unreachable" in err_msg:
                    logger.error(LOG_OLLAMA_UNREACHABLE, e)
                    raise OllamaUnavailableError(MSG_OLLAMA_UNREACHABLE) from e
                if attempt == OLLAMA_RETRIES:
                    logger.warning(LOG_INVALID_JSON_RETRIES, OLLAMA_RETRIES, e)
                    elapsed_ms = int((time.perf_counter() - start) * 1000)
                    logger.info(LOG_FALLBACK_AFTER_MS, elapsed_ms)
                    return self._fallback()
                logger.debug(LOG_ATTEMPT_RETRY, attempt, e)

        return self._fallback()

    def _fallback(self) -> EnrichmentResult:
        return EnrichmentResult(
            summary=FALLBACK_SUMMARY,
            techStack=[],
            role=FALLBACK_ROLE,
            dependencies=[],
            confidence=FALLBACK_CONFIDENCE,
            enrichedAt=datetime.utcnow(),
            category="application",
            risks=[],
            suggestions=[],
            relatedApps=[],
        )