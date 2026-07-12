"""Anthropic enrichment provider with signal-based response caching."""

import json
import time
from datetime import datetime

import anthropic

from app_logger import logger
from config import ANTHROPIC_API_KEY, ANTHROPIC_MODEL
from models import AppSignals, EnrichmentResult, RelatedApp
from prompts.k8s_app_analyzer_prompt import build_prompt

from .base import BaseProvider
from .cache import cache_key, get_cached, set_cached, signals_hash
from .constants import (
    FALLBACK_CONFIDENCE,
    FALLBACK_ROLE,
    FALLBACK_SUMMARY,
    LOG_ANTHROPIC_API_KEY_INVALID,
    LOG_ANTHROPIC_API_UNREACHABLE,
    LOG_ANTHROPIC_COMPLETED,
    LOG_ANTHROPIC_FAILED,
    LOG_ANTHROPIC_PARSE_FAILED,
    MSG_ANTHROPIC_API_KEY_NOT_SET,
)


class AnthropicProvider(BaseProvider):
    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        if not ANTHROPIC_API_KEY:
            raise ValueError(MSG_ANTHROPIC_API_KEY_NOT_SET)

        sig_hash = signals_hash(signals)
        key = cache_key(sig_hash)

        cached = get_cached(key)
        if cached is not None:
            logger.debug(
                "Anthropic cache hit for %s/%s (hash=%s)",
                signals.namespace, signals.name, sig_hash,
            )
            return cached

        client = anthropic.Anthropic(api_key=ANTHROPIC_API_KEY)
        prompt = build_prompt(signals)
        start = time.perf_counter()

        try:
            message = client.messages.create(
                model=ANTHROPIC_MODEL,
                max_tokens=1024,
                messages=[{"role": "user", "content": prompt}],
            )

            elapsed_ms = int((time.perf_counter() - start) * 1000)
            logger.info(LOG_ANTHROPIC_COMPLETED, elapsed_ms)

            raw = message.content[0].text.strip()
            if raw.startswith("```"):
                raw = raw.split("```")[1]
                if raw.startswith("json"):
                    raw = raw[4:]
            raw = raw.strip()

            data = json.loads(raw)

            result = EnrichmentResult(
                summary=data.get("summary", ""),
                techStack=data.get("techStack", []),
                role=data.get("role", FALLBACK_ROLE),
                dependencies=data.get("dependencies", []),
                confidence=data.get("confidence", FALLBACK_CONFIDENCE),
                enrichedAt=datetime.utcnow(),
                category=data.get("category", "application"),
                risks=data.get("risks", []),
                suggestions=data.get("suggestions", []),
                relatedApps=[
                    RelatedApp(**r)
                    for r in data.get("relatedApps", [])
                    if isinstance(r, dict)
                ],
            )
            set_cached(key, result)
            return result

        except anthropic.APIConnectionError as e:
            logger.error(LOG_ANTHROPIC_API_UNREACHABLE, e)
            raise

        except anthropic.AuthenticationError as e:
            logger.error(LOG_ANTHROPIC_API_KEY_INVALID, e)
            raise

        except (json.JSONDecodeError, KeyError) as e:
            logger.warning(LOG_ANTHROPIC_PARSE_FAILED, e)
            return self._fallback()

        except Exception as e:
            logger.error(LOG_ANTHROPIC_FAILED, e)
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