"""Checks for scope-based insights dispatch.

The dispatch must never call the LLM, must split ready vs pending correctly, and
must not pile the same job up across repeated ticks. Redis and the logger are
faked so the real logic runs without those deps installed.

Run: python3 tests/test_insights.py
"""

import os
import sys
import types

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

# --- stub deps insights.py pulls in, before importing it ---------------------
_httpx = types.ModuleType("httpx")
_httpx.get = lambda *a, **k: None
sys.modules["httpx"] = _httpx

_dotenv = types.ModuleType("dotenv")
_dotenv.load_dotenv = lambda *a, **k: None
sys.modules["dotenv"] = _dotenv

_app_logger = types.ModuleType("app_logger")
_app_logger.logger = types.SimpleNamespace(
    debug=lambda *a, **k: None,
    warning=lambda *a, **k: None,
    info=lambda *a, **k: None,
    error=lambda *a, **k: None,
)
sys.modules["app_logger"] = _app_logger


class _FakeRedis:
    def __init__(self):
        self.store: dict[str, str] = {}
        self.queue: list[str] = []

    def get(self, key):
        return self.store.get(key)

    def set(self, key, val, nx=False, ex=None):
        if nx and key in self.store:
            return False
        self.store[key] = val
        return True

    def setex(self, key, ttl, val):
        self.store[key] = val

    def exists(self, key):
        return 1 if key in self.store else 0

    def lpush(self, key, val):
        self.queue.insert(0, val)


_redis = types.ModuleType("redis")
_redis.Redis = _FakeRedis
_redis.ConnectionPool = types.SimpleNamespace(from_url=lambda *a, **k: object())
_redis.RedisError = Exception
sys.modules["redis"] = _redis

# insights only duck-types .name/.namespace/.model_dump on AppSignals, so stub the
# whole models module rather than pull in pydantic and the LLM result schemas.
_models = types.ModuleType("models")
_models.AppSignals = object
sys.modules["models"] = _models

import insights  # noqa: E402
from helpers import cache_key, enqueued_key  # noqa: E402
from config import CACHE_PREFIX  # noqa: E402


class _Sig:
    def __init__(self, name, namespace="default"):
        self.name = name
        self.namespace = namespace

    def model_dump(self, mode="json"):
        return {"name": self.name, "namespace": self.namespace}


def _fresh_cache_value():
    # a cached result whose prompt version matches → not stale
    from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION  # noqa: E402
    from constants import PROMPT_VERSION_KEY  # noqa: E402
    import json

    return json.dumps({PROMPT_VERSION_KEY: PROMPT_VERSION})


def _install(fake):
    insights._client = lambda: fake


def test_ready_vs_pending_split():
    fake = _FakeRedis()
    fake.store[cache_key(CACHE_PREFIX, "default", "cached-app")] = _fresh_cache_value()
    _install(fake)

    ready, pending = insights.dispatch_applications([_Sig("cached-app"), _Sig("new-app")])
    assert ready == ["cached-app"], ready
    assert pending == ["new-app"], pending
    assert len(fake.queue) == 1, "only the uncached app is queued"


def test_no_duplicate_enqueue_across_ticks():
    fake = _FakeRedis()
    _install(fake)

    insights.dispatch_applications([_Sig("app-a")])
    insights.dispatch_applications([_Sig("app-a")])  # second tick, still pending
    assert len(fake.queue) == 1, f"app-a queued twice across ticks: {len(fake.queue)}"
    assert enqueued_key(CACHE_PREFIX, "default", "app-a") in fake.store


def test_inflight_is_not_reenqueued():
    fake = _FakeRedis()
    from helpers import inflight_key

    fake.store[inflight_key(CACHE_PREFIX, "default", "busy")] = "1"
    _install(fake)

    ready, pending = insights.dispatch_applications([_Sig("busy")])
    assert pending == ["busy"], pending
    assert len(fake.queue) == 0, "an in-flight app must not be re-queued"


def _run():
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            fn()
            print(f"ok  {name}")
    print("all insights tests passed")


if __name__ == "__main__":
    _run()
