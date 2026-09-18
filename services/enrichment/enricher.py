"""Thin router: delegates to the configured enrichment provider."""

from constants import MSG_AI_DISABLED
from models import AppSignals, EnrichmentResult
from providers import get_provider
from providers.base import BaseProvider
from providers.ollama import OllamaUnavailableError


def enrich(signals: AppSignals, provider: BaseProvider | None = None) -> EnrichmentResult:
    """Enrich using the given provider, or the one resolved from GlobalConfig.

    Not memoised: provider and key can change under us via GlobalConfig, so the
    caller either passes the provider it already resolved this cycle, or we
    resolve fresh.
    """
    p = provider if provider is not None else get_provider()
    if p is None:
        raise ValueError(MSG_AI_DISABLED)
    return p.enrich(signals)


__all__ = ["enrich", "OllamaUnavailableError"]
