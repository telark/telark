"""Groq enrichment provider with signal-based response caching."""

import time
from datetime import datetime

import instructor
from openai import OpenAI

from app_logger import logger
from config import GROQ_API_KEY, GROQ_MODEL
from models import AppSignals, EnrichmentResult, EnrichmentResultLLM
from prompts.k8s_app_analyzer_prompt import build_prompt

from .base import BaseProvider
from .cache import cache_key, get_cached, set_cached, signals_hash
from .config import GROQ_BASE_URL, GROQ_RETRIES
from .constants import (
    FALLBACK_CONFIDENCE,
    FALLBACK_ROLE,
    FALLBACK_SUMMARY,
    LOG_GROQ_API_KEY_INVALID,
    LOG_GROQ_ATTEMPT_RETRY,
    LOG_GROQ_COMPLETED,
    LOG_GROQ_RATE_LIMIT,
    LOG_GROQ_RETRIES_FAILED,
    LOG_GROQ_SETUP_FAILED,
    MSG_GROQ_API_KEY_NOT_SET,
)


class GroqProvider(BaseProvider):
    def _create_client(self) -> instructor.Instructor:
        if not GROQ_API_KEY:
            raise ValueError(MSG_GROQ_API_KEY_NOT_SET)
        return instructor.from_openai(
            OpenAI(base_url=GROQ_BASE_URL, api_key=GROQ_API_KEY),
            mode=instructor.Mode.JSON,
        )

    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        sig_hash = signals_hash(signals)
        key = cache_key(sig_hash)

        cached = get_cached(key)
        if cached is not None:
            logger.debug(
                "Groq cache hit for %s/%s (hash=%s)",
                signals.namespace, signals.name, sig_hash,
            )
            return cached

        prompt = build_prompt(signals)
        start = time.perf_counter()

        try:
            client = self._create_client()
        except ValueError as e:
            logger.error(LOG_GROQ_SETUP_FAILED, e)
            return self._fallback()

        for attempt in range(1, GROQ_RETRIES + 1):
            try:
                resp = client.chat.completions.create(
                    model=GROQ_MODEL,
                    messages=[{"role": "user", "content": prompt}],
                    response_model=EnrichmentResultLLM,
                )
                elapsed_ms = int((time.perf_counter() - start) * 1000)
                logger.info(LOG_GROQ_COMPLETED, elapsed_ms, attempt)

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
                if "authentication" in err_msg or "api key" in err_msg:
                    logger.error(LOG_GROQ_API_KEY_INVALID, e)
                    return self._fallback()
                if "rate" in err_msg or "limit" in err_msg:
                    logger.warning(LOG_GROQ_RATE_LIMIT, e)
                    return self._fallback()
                if attempt == GROQ_RETRIES:
                    logger.warning(LOG_GROQ_RETRIES_FAILED, GROQ_RETRIES, e)
                    return self._fallback()
                logger.debug(LOG_GROQ_ATTEMPT_RETRY, attempt, e)

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