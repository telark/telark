"""Anthropic enrichment provider.

Unlike the instructor-backed providers, Anthropic returns free text that we parse
into the schema ourselves, so it keeps its own flow rather than InstructorProvider.
"""

import json
from datetime import datetime

import anthropic

from app_logger import logger
from config import ANTHROPIC_MODEL
from models import AppSignals, EnrichmentResult, EnrichmentResultLLM
from prompts.k8s_app_analyzer_prompt import build_prompt

from .base import BaseProvider, fallback_result
from .cache import cache_key, get_cached, set_cached, signals_hash
from .constants import (
    LOG_PROVIDER_API_KEY_INVALID,
    LOG_PROVIDER_CACHE_HIT,
    LOG_PROVIDER_FAILED,
    LOG_PROVIDER_PARSE_FAILED,
    LOG_PROVIDER_UNREACHABLE,
    MSG_API_KEY_NOT_SET,
)

_NAME = "anthropic"
_MAX_TOKENS = 1024


class AnthropicProvider(BaseProvider):
    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        if not self._api_key:
            raise ValueError(MSG_API_KEY_NOT_SET.format(_NAME))

        key = cache_key(signals_hash(signals))
        cached = get_cached(key)
        if cached is not None:
            logger.debug(LOG_PROVIDER_CACHE_HIT, _NAME, signals.namespace, signals.name, key)
            return cached

        client = anthropic.Anthropic(api_key=self._api_key)
        prompt = build_prompt(signals)

        try:
            message = client.messages.create(
                model=ANTHROPIC_MODEL,
                max_tokens=_MAX_TOKENS,
                messages=[{"role": "user", "content": prompt}],
            )
            data = json.loads(_strip_fences(message.content[0].text.strip()))
            llm = EnrichmentResultLLM.model_validate(data)
            result = EnrichmentResult(**llm.model_dump(), enrichedAt=datetime.utcnow())
            set_cached(key, result)
            return result

        except anthropic.APIConnectionError as e:
            logger.error(LOG_PROVIDER_UNREACHABLE, _NAME, type(e).__name__)
            raise
        except anthropic.AuthenticationError as e:
            logger.error(LOG_PROVIDER_API_KEY_INVALID, _NAME, type(e).__name__)
            raise
        except (json.JSONDecodeError, KeyError) as e:
            # e.doc is the raw model completion — never log it.
            detail = (
                f"{type(e).__name__} at pos {e.pos}: {e.msg}"
                if isinstance(e, json.JSONDecodeError)
                else type(e).__name__
            )
            logger.warning(LOG_PROVIDER_PARSE_FAILED, _NAME, detail)
            return fallback_result()
        except Exception as e:
            logger.error(LOG_PROVIDER_FAILED, _NAME, type(e).__name__)
            return fallback_result()


def _strip_fences(raw: str) -> str:
    if raw.startswith("```"):
        raw = raw.split("```")[1]
        if raw.startswith("json"):
            raw = raw[4:]
    return raw.strip()
