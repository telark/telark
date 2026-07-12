"""Thin router: delegates to the configured enrichment provider."""

from typing import Optional

from models import AppSignals, EnrichmentResult
from providers import get_provider
from providers.base import BaseProvider
from providers.ollama import OllamaUnavailableError

_provider: Optional[BaseProvider] = None


def _get_default_provider() -> BaseProvider:
    global _provider
    if _provider is None:
        _provider = get_provider()
    return _provider


def enrich(signals: AppSignals, provider: Optional[BaseProvider] = None) -> EnrichmentResult:
    """Enrich using the given provider or the default module-level one."""
    p = provider if provider is not None else _get_default_provider()
    return p.enrich(signals)


__all__ = ["enrich", "OllamaUnavailableError"]
