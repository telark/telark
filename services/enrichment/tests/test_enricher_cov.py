"""Full coverage for enricher.py — the thin provider router.

Run: pytest tests/test_enricher_cov.py
"""

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import enricher as E  # noqa: E402
from enricher import OllamaUnavailableError, enrich  # noqa: E402
from models import AppSignals  # noqa: E402


class _P:
    def __init__(self):
        self.got = None

    def enrich(self, s):
        self.got = s
        return "RESULT"


def _sig():
    return AppSignals(name="a", namespace="n")


def test_uses_given_provider():
    p = _P()
    assert enrich(_sig(), provider=p) == "RESULT"
    assert p.got is not None


def test_resolves_provider_when_none(monkeypatch):
    p = _P()
    monkeypatch.setattr(E, "get_provider", lambda: p)
    assert enrich(_sig()) == "RESULT"


def test_none_provider_raises(monkeypatch):
    monkeypatch.setattr(E, "get_provider", lambda: None)
    with pytest.raises(ValueError):
        enrich(_sig())


def test_exports_error():
    assert issubclass(OllamaUnavailableError, Exception)
