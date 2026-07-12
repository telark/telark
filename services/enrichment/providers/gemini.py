"""Gemini enrichment provider with signal-based response caching."""

import time
from datetime import datetime

import instructor
from openai import OpenAI

from app_logger import logger
from config import GEMINI_API_KEY, GEMINI_MODEL
from models import AppSignals, EnrichmentResult, EnrichmentResultLLM
from prompts.k8s_app_analyzer_prompt import build_prompt

from .base import BaseProvider
from .cache import cache_key, get_cached, set_cached, signals_hash
from .config import GEMINI_BASE_URL, GEMINI_RETRIES
from .constants import (
    FALLBACK_CONFIDENCE,
    FALLBACK_ROLE,
    FALLBACK_SUMMARY,
    LOG_GEMINI_API_KEY_INVALID,
    LOG_GEMINI_ATTEMPT_RETRY,
    LOG_GEMINI_COMPLETED,
    LOG_GEMINI_RATE_LIMIT,
    LOG_GEMINI_RETRIES_FAILED,
    LOG_GEMINI_SETUP_FAILED,
    MSG_GEMINI_API_KEY_NOT_SET,
)


class GeminiProvider(BaseProvider):
    def _create_client(self) -> instructor.Instructor:
        if not GEMINI_API_KEY:
            raise ValueError(MSG_GEMINI_API_KEY_NOT_SET)
        return instructor.from_openai(
            OpenAI(base_url=GEMINI_BASE_URL, api_key=GEMINI_API_KEY),
            mode=instructor.Mode.JSON,
        )

    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        sig_hash = signals_hash(signals)
        key = cache_key(sig_hash)

        cached = get_cached(key)
        if cached is not None:
            logger.debug(
                "Gemini cache hit for %s/%s (hash=%s)",
                signals.namespace, signals.name, sig_hash,
            )
            return cached

        prompt = build_prompt(signals)
        start = time.perf_counter()

        try:
            client = self._create_client()
        except ValueError as e:
            logger.error(LOG_GEMINI_SETUP_FAILED, e)
            return self._fallback()

        for attempt in range(1, GEMINI_RETRIES + 1):
            try:
                resp = client.chat.completions.create(
                    model=GEMINI_MODEL,
                    messages=[{"role": "user", "content": prompt}],
                    response_model=EnrichmentResultLLM,
                )
                elapsed_ms = int((time.perf_counter() - start) * 1000)
                logger.info(LOG_GEMINI_COMPLETED, elapsed_ms, attempt)

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
                    logger.error(LOG_GEMINI_API_KEY_INVALID, e)
                    return self._fallback()
                if "rate" in err_msg or "quota" in err_msg:
                    logger.warning(LOG_GEMINI_RATE_LIMIT, e)
                    return self._fallback()
                if attempt == GEMINI_RETRIES:
                    logger.warning(LOG_GEMINI_RETRIES_FAILED, GEMINI_RETRIES, e)
                    return self._fallback()
                logger.debug(LOG_GEMINI_ATTEMPT_RETRY, attempt, e)

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