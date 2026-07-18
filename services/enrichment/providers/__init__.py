"""Provider factory and exports.

Provider and key come from the GlobalConfig CR at call time, not from env. The
built provider is memoised by (provider, key) so a busy worker pool reuses one
SDK client until an admin changes either; the next call after that rebuilds.
"""

import threading

from provider_config import get_ai_config

from .anthropic import AnthropicProvider
from .base import BaseProvider
from .gemini import GeminiProvider
from .groq import GroqProvider
from .ollama import OllamaProvider

_lock = threading.Lock()
_cache_key: tuple[str, str] | None = None
_provider: BaseProvider | None = None


def _build(provider_name: str, api_key: str) -> BaseProvider | None:
    if provider_name == "anthropic":
        return AnthropicProvider(api_key)
    if provider_name == "groq":
        return GroqProvider(api_key)
    if provider_name == "gemini":
        return GeminiProvider(api_key)
    if provider_name == "ollama":
        return OllamaProvider()
    return None


def get_provider() -> BaseProvider | None:
    """Resolve the current provider from GlobalConfig, or None when AI is off.

    None also covers a configured cloud provider with no key: the caller treats
    that the same as disabled rather than dispatching a call that must fail.
    """
    global _cache_key, _provider

    cfg = get_ai_config()
    if not cfg.enabled or not cfg.provider:
        return None
    if cfg.provider != "ollama" and not cfg.api_key:
        return None

    key = (cfg.provider, cfg.api_key)
    with _lock:
        if key != _cache_key:
            _provider = _build(cfg.provider, cfg.api_key)
            _cache_key = key
        return _provider


__all__ = [
    "get_provider",
    "BaseProvider",
    "OllamaProvider",
    "AnthropicProvider",
    "GroqProvider",
    "GeminiProvider",
]
