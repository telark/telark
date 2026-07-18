"""Full coverage for helpers.py — Redis/Ollama backoff, key builders, staleness, DLQ replay.

Run: pytest tests/test_helpers_cov.py
"""

import json
import os
import sys

import redis

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import helpers as H  # noqa: E402
from constants import PROMPT_VERSION_KEY  # noqa: E402
from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION  # noqa: E402


def test_key_builders():
    assert H.cache_key("p", "ns", "n") == "p:ns:n"
    assert H.inflight_key("p", "ns", "n") == "p:inflight:ns:n"
    assert H.enqueued_key("p", "ns", "n") == "p:enqueued:ns:n"


def test_clear_inflight_ok_and_swallows_error():
    class OK:
        def __init__(self):
            self.deleted = None

        def delete(self, k):
            self.deleted = k

    r = OK()
    H.clear_inflight(r, "k")
    assert r.deleted == "k"

    class Boom:
        def delete(self, k):
            raise redis.RedisError("nope")

    H.clear_inflight(Boom(), "k")  # must not raise


def test_is_prompt_stale():
    assert H.is_prompt_stale(json.dumps({PROMPT_VERSION_KEY: PROMPT_VERSION})) is False
    assert H.is_prompt_stale(json.dumps({PROMPT_VERSION_KEY: "old"})) is True
    assert H.is_prompt_stale(json.dumps({})) is True
    assert H.is_prompt_stale("not-json") is True


def test_replay_dlq_moves_all():
    class R:
        def __init__(self):
            self.dlq = ["j1", "j2", "j3"]
            self.q = []

        def rpoplpush(self, dlq, q):
            if not self.dlq:
                return None
            j = self.dlq.pop()
            self.q.append(j)
            return j

    r = R()
    assert H.replay_dlq(r, "q", "dlq") == 3
    assert len(r.q) == 3


def test_replay_dlq_empty():
    class R:
        def rpoplpush(self, dlq, q):
            return None

    assert H.replay_dlq(R(), "q", "dlq") == 0


def test_connect_redis_succeeds(monkeypatch):
    class FakeR:
        def ping(self):
            return True

    monkeypatch.setattr(H.redis, "from_url", lambda u: FakeR())
    assert isinstance(H.connect_redis("redis://x"), FakeR)


def test_connect_redis_retries_then_succeeds(monkeypatch):
    seq = [redis.RedisError("cold"), None]

    class FakeR:
        def ping(self):
            e = seq.pop(0)
            if e:
                raise e

    monkeypatch.setattr(H.redis, "from_url", lambda u: FakeR())
    monkeypatch.setattr(H.time, "sleep", lambda s: None)
    assert H.connect_redis("redis://x") is not None


def test_wait_for_ollama_ready(monkeypatch):
    monkeypatch.setattr(H.httpx, "get", lambda *a, **k: type("R", (), {"status_code": 200})())
    H.wait_for_ollama("http://x")  # returns without sleeping


def test_wait_for_ollama_retries(monkeypatch):
    seq = [Exception("down"), type("R", (), {"status_code": 200})()]

    def get(*a, **k):
        v = seq.pop(0)
        if isinstance(v, Exception):
            raise v
        return v

    monkeypatch.setattr(H.httpx, "get", get)
    monkeypatch.setattr(H.time, "sleep", lambda s: None)
    H.wait_for_ollama("http://x")


def test_wait_for_ollama_non_200_then_ready(monkeypatch):
    seq = [type("R", (), {"status_code": 503})(), type("R", (), {"status_code": 200})()]
    monkeypatch.setattr(H.httpx, "get", lambda *a, **k: seq.pop(0))
    monkeypatch.setattr(H.time, "sleep", lambda s: None)
    H.wait_for_ollama("http://x")
