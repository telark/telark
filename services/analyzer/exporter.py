"""The analyzer's only exporter client (exporter is the sole CRD reader).

Every read carries the service token. The TelarkConfig read keeps the last good
value, so a transient exporter blip never flips a working analyzer off. The review
reads the app list (summary view), the protection plans and the plan environments.
"""

from __future__ import annotations

from urllib.parse import quote

import httpx

from app_logger import logger
from config import (
    ANALYZER_REVIEW_TICK_SEC,
    APPLICATION_URL_TEMPLATE,
    APPLICATIONS_URL,
    CONFIG_URL,
    PLAN_ENVIRONMENTS_URL,
    PLANS_URL,
    SERVICE_TOKEN,
)
from constants import (
    AI_FIELD_AUTO_ANALYZE,
    AI_FIELD_ENABLED,
    AI_FIELD_MODEL,
    DEFAULT_ANALYZER_MODEL,
    ENVELOPE_DATA,
    EXPORTER_LIST_TIMEOUT_S,
    EXPORTER_TIMEOUT_S,
    FIELD_ITEMS,
    CONFIG_FIELD_AI,
    CONFIG_FIELD_EXCLUDED_NAMESPACES,
    HEADER_SERVICE_TOKEN,
    LOG_CONFIG_FETCH_FAILED,
    PARAM_VIEW,
    VIEW_SUMMARY,
)
from models import AnalyzerConfig


class AppNotFound(Exception):
    """The exporter answered 404 for the application."""


class ExporterUnavailable(Exception):
    """Any other non-2xx answer or a transport error."""


_current = AnalyzerConfig()


async def _get(client: httpx.AsyncClient, url: str, timeout: float = EXPORTER_TIMEOUT_S,
               params: dict | None = None) -> httpx.Response:
    return await client.get(url, headers={HEADER_SERVICE_TOKEN: SERVICE_TOKEN}, timeout=timeout, params=params)


async def fetch_config(client: httpx.AsyncClient) -> AnalyzerConfig:
    resp = await _get(client, CONFIG_URL)
    resp.raise_for_status()
    data = (resp.json() or {}).get(ENVELOPE_DATA) or {}
    ai = data.get(CONFIG_FIELD_AI) or {}
    return AnalyzerConfig(
        enabled=bool(ai.get(AI_FIELD_ENABLED, False)),
        # An analyzer image newer than the CRD sees the field pruned.
        model=ai.get(AI_FIELD_MODEL) or DEFAULT_ANALYZER_MODEL,
        autoAnalyze=bool(ai.get(AI_FIELD_AUTO_ANALYZE, False)),
        excludedNamespaces=data.get(CONFIG_FIELD_EXCLUDED_NAMESPACES) or [],
    )


def current() -> AnalyzerConfig:
    """Last good config; disabled until the first successful read."""
    return _current


async def refresh(client: httpx.AsyncClient) -> None:
    global _current
    try:
        _current = await fetch_config(client)
    except Exception as e:  # any failure keeps the last good value
        logger.warning(LOG_CONFIG_FETCH_FAILED, type(e).__name__)


async def get_application(client: httpx.AsyncClient, name: str) -> dict:
    try:
        resp = await _get(client, APPLICATION_URL_TEMPLATE.format(name=quote(name, safe="")))
    except httpx.HTTPError as e:
        raise ExporterUnavailable(type(e).__name__) from e
    if resp.status_code == httpx.codes.NOT_FOUND:
        raise AppNotFound(name)
    if not resp.is_success:
        raise ExporterUnavailable(resp.status_code)
    return (resp.json() or {}).get(ENVELOPE_DATA) or {}


async def _items(client: httpx.AsyncClient, url: str, timeout: float = EXPORTER_TIMEOUT_S,
                 params: dict | None = None) -> list[dict]:
    """data.items of a list route; any failure is ExporterUnavailable."""
    try:
        resp = await _get(client, url, timeout, params)
        if not resp.is_success:
            raise ExporterUnavailable(resp.status_code)
        items = ((resp.json() or {}).get(ENVELOPE_DATA) or {}).get(FIELD_ITEMS)
    except (httpx.HTTPError, ValueError, AttributeError) as e:
        raise ExporterUnavailable(type(e).__name__) from e
    if not isinstance(items, list):
        raise ExporterUnavailable(FIELD_ITEMS)
    return items


async def list_applications(client: httpx.AsyncClient) -> list[dict]:
    return await _items(client, APPLICATIONS_URL, EXPORTER_LIST_TIMEOUT_S, {PARAM_VIEW: VIEW_SUMMARY})


async def list_plans(client: httpx.AsyncClient) -> list[dict]:
    return await _items(client, PLANS_URL)


async def list_environments(client: httpx.AsyncClient) -> dict[str, str]:
    """Plan environment category id -> name."""
    return {c.get("id"): c.get("name") or "" for c in await _items(client, PLAN_ENVIRONMENTS_URL)}


_plans_cache: tuple[float, list[dict], dict[str, str]] | None = None


async def plans_snapshot(client: httpx.AsyncClient, now: float) -> tuple[list[dict], dict[str, str]]:
    """(plans, environment names), read at most once per ANALYZER_REVIEW_TICK_SEC; a failure is never cached."""
    global _plans_cache
    if _plans_cache and now - _plans_cache[0] < ANALYZER_REVIEW_TICK_SEC:
        return _plans_cache[1], _plans_cache[2]
    plans, envs = await list_plans(client), await list_environments(client)
    _plans_cache = (now, plans, envs)
    return plans, envs
