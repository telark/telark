from __future__ import annotations

"""Runtime AI provider config, sourced from the GlobalConfig CR.

Provider and API key live in GlobalConfig, not env, so an admin can change them
from the UI without a redeploy. Read through exporter (the sole CRD reader) with
the service token, and cached briefly so a busy worker pool does not fetch it per
job. The key never rides between services on the wire — each side reads it from
the CR itself.
"""

import threading
import time
from dataclasses import dataclass

import httpx

from app_logger import logger
from config import GLOBALCONFIG_URL, SERVICE_TOKEN
from constants import (
    GLOBALCONFIG_TIMEOUT_SECONDS,
    GLOBALCONFIG_TTL_SECONDS,
    HEADER_SERVICE_TOKEN,
    LOG_GLOBALCONFIG_FETCH_FAILED,
)


@dataclass(frozen=True)
class AIConfig:
    enabled: bool
    provider: str
    api_key: str


_DISABLED = AIConfig(enabled=False, provider="", api_key="")

_lock = threading.Lock()
_cached: AIConfig | None = None
_fetched_at: float = 0.0


def _fetch() -> AIConfig:
    resp = httpx.get(
        GLOBALCONFIG_URL,
        headers={HEADER_SERVICE_TOKEN: SERVICE_TOKEN},
        timeout=GLOBALCONFIG_TIMEOUT_SECONDS,
    )
    resp.raise_for_status()
    ai = (resp.json() or {}).get("data", {}).get("ai", {}) or {}
    return AIConfig(
        enabled=bool(ai.get("enabled", False)),
        provider=str(ai.get("provider", "")).strip().lower(),
        api_key=str(ai.get("apiKey", "")),
    )


def get_ai_config() -> AIConfig:
    """Current AI config, cached for GLOBALCONFIG_TTL_SECONDS.

    A failed read returns the last good value if there is one, else disabled — a
    transient exporter blip must not flip a working provider off.
    """
    global _cached, _fetched_at
    with _lock:
        fresh = _cached is not None and (time.monotonic() - _fetched_at) < GLOBALCONFIG_TTL_SECONDS
        if fresh:
            return _cached

        try:
            _cached = _fetch()
            _fetched_at = time.monotonic()
        except Exception as exc:  # noqa: BLE001 - any failure falls back, never raises
            logger.warning(LOG_GLOBALCONFIG_FETCH_FAILED.format(error=exc))
            if _cached is None:
                return _DISABLED
        return _cached
