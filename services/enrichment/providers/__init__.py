"""Provider factory and exports."""

from config import ENRICHMENT_PROVIDER

from .anthropic import AnthropicProvider
from .base import BaseProvider
from .gemini import GeminiProvider
from .groq import GroqProvider
from .ollama import OllamaProvider


def get_provider() -> BaseProvider:
    if ENRICHMENT_PROVIDER == "anthropic":
        return AnthropicProvider()
    if ENRICHMENT_PROVIDER == "groq":
        return GroqProvider()
    if ENRICHMENT_PROVIDER == "gemini":
        return GeminiProvider()
    return OllamaProvider()


__all__ = [
    "get_provider",
    "BaseProvider",
    "OllamaProvider",
    "AnthropicProvider",
    "GroqProvider",
    "GeminiProvider",
]
