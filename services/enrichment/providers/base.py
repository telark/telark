"""Abstract base class for enrichment providers."""

from abc import ABC, abstractmethod

from models import AppSignals, EnrichmentResult


class BaseProvider(ABC):
    @abstractmethod
    def enrich(self, signals: AppSignals) -> EnrichmentResult:
        """Enrich an application with LLM insights."""
        pass
