"""Coverage for insights.py internals not exercised by the stub-based suite
(the real _client pool builder).

Run: pytest tests/test_insights_cov.py
"""

import os
import sys

import redis

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import insights as I  # noqa: E402


def test_client_builds_and_reuses_pool(monkeypatch):
    I._pool = None
    made = {"pools": 0}

    def from_url(*a, **k):
        made["pools"] += 1
        return "POOL"

    monkeypatch.setattr(redis.ConnectionPool, "from_url", staticmethod(from_url))
    monkeypatch.setattr(redis, "Redis", lambda connection_pool=None: "CLIENT")

    assert I._client() == "CLIENT"
    assert I._client() == "CLIENT"
    assert made["pools"] == 1  # pool created once, then reused
    I._pool = None
