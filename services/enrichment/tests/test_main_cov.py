"""Coverage for main.py — classifiers, metrics, DLQ, job processing branches, loops.

Run: pytest tests/test_main_cov.py
"""

import os
import sys
from datetime import datetime

import redis

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import main  # noqa: E402
from enricher import OllamaUnavailableError  # noqa: E402
from models import EnrichmentResult  # noqa: E402


def _result():
    return EnrichmentResult(summary="s", role="r", confidence="low", enrichedAt=datetime.utcnow())


class FakeRedis:
    def __init__(self, blpop_seq=None):
        self.store = {}
        self.queue = []
        self.dlq = []
        self.deleted = []
        self.blpop_seq = list(blpop_seq or [])

    def lpush(self, key, val):
        (self.dlq if key == main.DLQ_KEY else self.queue).insert(0, val)

    def setex(self, key, ttl, val):
        self.store[key] = val

    def delete(self, key):
        self.deleted.append(key)

    def get(self, key):
        return self.store.get(key)

    def exists(self, key):
        return 1 if key in self.store else 0

    def blpop(self, key, timeout=0):
        return self.blpop_seq.pop(0) if self.blpop_seq else None


def _job(name="app"):
    import json

    return json.dumps({"name": name, "namespace": "ns"})


def _metrics():
    return main.WorkerMetrics(worker_id=0)


# --------------------------------------------------------------------------- #
def test_classifiers():
    assert main._is_rate_limit_error(Exception("rate limit")) is True
    assert main._is_rate_limit_error(Exception("HTTP 429")) is True
    assert main._is_rate_limit_error(Exception("ok")) is False
    assert main._is_connection_error(Exception("connection refused")) is True
    assert main._is_connection_error(Exception("timeout")) is True
    assert main._is_connection_error(Exception("ok")) is False


def test_get_num_workers(monkeypatch):
    monkeypatch.setattr(main, "get_ai_config", lambda: type("C", (), {"provider": "ollama"})())
    assert main.get_num_workers() == 1
    monkeypatch.setattr(main, "get_ai_config", lambda: type("C", (), {"provider": "groq"})())
    assert main.get_num_workers() == main.NUM_WORKERS


def test_worker_metrics():
    m = _metrics()
    m.record_processed(100)
    m.record_processed(200)
    assert m.jobs_processed == 2 and m.avg_duration_ms == 150.0
    m.record_failed()
    m.record_skipped()
    assert m.jobs_failed == 1 and m.jobs_skipped == 1 and m.last_job_at is not None


def test_push_to_dlq_ok_and_error():
    r = FakeRedis()
    main._push_to_dlq(r, "raw", "ns", "n", 0)
    assert r.dlq == ["raw"]

    class Boom(FakeRedis):
        def lpush(self, *a):
            raise redis.RedisError("x")

    main._push_to_dlq(Boom(), "raw", "ns", "n", 0)  # swallowed


def test_handle_signal():
    main._shutdown = False
    main._handle_signal(15, None)
    assert main._shutdown is True
    main._shutdown = False


# ---- _process_one_job branches -------------------------------------------- #
def test_job_bad_payload_to_dlq():
    r = FakeRedis()
    m = _metrics()
    main._process_one_job(0, r, object(), "not json", "ck", "ifk", m)
    assert m.jobs_failed == 1 and r.dlq == ["not json"]


def test_job_success_writes_cache(monkeypatch):
    r = FakeRedis()
    m = _metrics()
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: _result())
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert "ck" in r.store and m.jobs_processed == 1 and "ifk" in r.deleted


def test_job_ollama_unavailable_requeues(monkeypatch):
    r = FakeRedis()
    m = _metrics()

    def boom(s, provider=None):
        raise OllamaUnavailableError("down")

    monkeypatch.setattr(main, "enrich", boom)
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert m.jobs_failed == 1 and r.queue and "ifk" in r.deleted


def test_job_connection_error_requeues(monkeypatch):
    r = FakeRedis()
    m = _metrics()
    monkeypatch.setattr(main.time, "sleep", lambda s: None)

    def boom(s, provider=None):
        raise Exception("connection reset")

    monkeypatch.setattr(main, "enrich", boom)
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert r.queue  # re-queued


def test_job_rate_limit_then_success(monkeypatch):
    r = FakeRedis()
    m = _metrics()
    monkeypatch.setattr(main.time, "sleep", lambda s: None)
    state = {"n": 0}

    def flaky(s, provider=None):
        state["n"] += 1
        if state["n"] == 1:
            raise Exception("rate limit")
        return _result()

    monkeypatch.setattr(main, "enrich", flaky)
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert m.jobs_processed == 1


def test_job_terminal_failure_to_dlq(monkeypatch):
    r = FakeRedis()
    m = _metrics()

    def boom(s, provider=None):
        raise Exception("logic bug")

    monkeypatch.setattr(main, "enrich", boom)
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert m.jobs_failed == 1 and r.dlq


def test_job_rate_limit_exhausted_to_dlq(monkeypatch):
    r = FakeRedis()
    m = _metrics()
    monkeypatch.setattr(main.time, "sleep", lambda s: None)

    def always_rate(s, provider=None):
        raise Exception("rate limit")

    monkeypatch.setattr(main, "enrich", always_rate)
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert m.jobs_failed == 1 and r.dlq


def test_job_cache_write_redis_error_requeues(monkeypatch):
    class SetexBoom(FakeRedis):
        def setex(self, *a):
            raise redis.RedisError("write fail")

    r = SetexBoom()
    m = _metrics()
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: _result())
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert m.jobs_failed == 1 and r.queue


def test_job_cache_write_generic_error_to_dlq(monkeypatch):
    class SetexBoom(FakeRedis):
        def setex(self, *a):
            raise RuntimeError("weird")

    r = SetexBoom()
    m = _metrics()
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: _result())
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", m)
    assert m.jobs_failed == 1 and r.dlq


# ---- background loops ------------------------------------------------------ #
def test_metrics_reporter_logs_then_stops(monkeypatch):
    main._shutdown = False
    main._metrics.clear()
    main._metrics[0] = _metrics()
    calls = {"n": 0}

    def sleep(_):
        calls["n"] += 1
        if calls["n"] >= 2:
            main._shutdown = True

    monkeypatch.setattr(main.time, "sleep", sleep)
    main._metrics_reporter()
    main._shutdown = False


def test_connectivity_heartbeat(monkeypatch):
    main._shutdown = False

    class OkRedis:
        def set(self, *a, **k):
            pass

    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: OkRedis())
    monkeypatch.setattr(main.time, "sleep", lambda s: setattr(main, "_shutdown", True))
    main._connectivity_heartbeat(object())
    main._shutdown = False


def test_connectivity_heartbeat_redis_error(monkeypatch):
    main._shutdown = False

    class BoomRedis:
        def set(self, *a, **k):
            raise redis.RedisError("x")

    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: BoomRedis())
    monkeypatch.setattr(main.time, "sleep", lambda s: setattr(main, "_shutdown", True))
    main._connectivity_heartbeat(object())
    main._shutdown = False


def _one_iter_redis(first):
    """FakeRedis whose blpop yields `first` once, then stops the loop."""
    r = FakeRedis()
    seq = [first, "STOP"]

    def blpop(key, timeout=0):
        v = seq.pop(0) if seq else "STOP"
        if v == "STOP":
            main._shutdown = True
            return None
        if isinstance(v, Exception):
            raise v
        return v

    r.blpop = blpop
    return r


def _keys():
    from helpers import cache_key, inflight_key

    return (cache_key(main.CACHE_PREFIX, "ns", "app"), inflight_key(main.CACHE_PREFIX, "ns", "app"))


def test_worker_loop_happy_path(monkeypatch):
    main._shutdown = False
    r = _one_iter_redis(("k", _job()))
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main, "get_provider", lambda: object())
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: _result())
    main.worker_loop(0, object())
    ckey, _ = _keys()
    assert ckey in r.store
    main._shutdown = False


def test_worker_loop_blpop_error(monkeypatch):
    main._shutdown = False
    r = _one_iter_redis(redis.RedisError("blpop down"))
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main.time, "sleep", lambda s: None)
    main.worker_loop(0, object())
    main._shutdown = False


def test_worker_loop_bad_job(monkeypatch):
    main._shutdown = False
    r = _one_iter_redis(("k", "not json"))
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    main.worker_loop(0, object())
    assert r.dlq  # bad payload dead-lettered
    main._shutdown = False


def test_worker_loop_provider_none_skips(monkeypatch):
    main._shutdown = False
    r = _one_iter_redis(("k", _job()))
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main, "get_provider", lambda: None)
    main.worker_loop(0, object())
    assert main._metrics[0].jobs_skipped >= 1
    main._shutdown = False


def test_worker_loop_cache_fresh_skips(monkeypatch):
    import json
    from constants import PROMPT_VERSION_KEY
    from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION

    main._shutdown = False
    r = _one_iter_redis(("k", _job()))
    ckey, _ = _keys()
    r.store[ckey] = json.dumps({PROMPT_VERSION_KEY: PROMPT_VERSION})  # fresh
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main, "get_provider", lambda: object())
    main.worker_loop(0, object())
    assert main._metrics[0].jobs_skipped >= 1
    main._shutdown = False


def test_worker_loop_inflight_skips(monkeypatch):
    main._shutdown = False
    r = _one_iter_redis(("k", _job()))
    _, ifk = _keys()
    r.store[ifk] = "1"  # already in flight
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main, "get_provider", lambda: object())
    main.worker_loop(0, object())
    assert main._metrics[0].jobs_skipped >= 1
    main._shutdown = False


class _FakePool:
    def disconnect(self):
        pass


class _FakeThread:
    def __init__(self, *a, **k):
        pass

    def start(self):
        pass


class _FakeExecutor:
    def __init__(self, *a, **k):
        pass

    def __enter__(self):
        return self

    def __exit__(self, *a):
        return False

    def submit(self, fn, *a):
        return "FUTURE"


def _patch_run(monkeypatch, provider, wait_result):
    monkeypatch.setattr(main, "configure", lambda level=None: None)
    monkeypatch.setattr(main.signal, "signal", lambda *a: None)
    monkeypatch.setattr(main, "get_ai_config", lambda: type("C", (), {"provider": provider})())
    monkeypatch.setattr(main, "connect_redis", lambda url: None)
    monkeypatch.setattr(main.redis.ConnectionPool, "from_url", staticmethod(lambda *a, **k: _FakePool()))
    monkeypatch.setattr(main, "wait_for_ollama", lambda host: None)
    monkeypatch.setattr(main, "get_num_workers", lambda: 1)
    monkeypatch.setattr(main, "worker_loop", lambda *a: None)
    # Fake the executor + wait so no real threads spawn (patching threading.Thread
    # globally would otherwise corrupt ThreadPoolExecutor's own internals).
    monkeypatch.setattr(main, "ThreadPoolExecutor", _FakeExecutor)
    monkeypatch.setattr(main, "wait", lambda futures, timeout=None: wait_result)
    monkeypatch.setattr(main.threading, "Thread", _FakeThread)


def test_worker_loop_cache_stale_reprocesses(monkeypatch):
    import json
    from constants import PROMPT_VERSION_KEY

    main._shutdown = False
    r = _one_iter_redis(("k", _job()))
    ckey, _ = _keys()
    r.store[ckey] = json.dumps({PROMPT_VERSION_KEY: "OLD"})  # stale -> re-enrich
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main, "get_provider", lambda: object())
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: _result())
    main.worker_loop(0, object())
    assert main._metrics[0].jobs_processed == 1
    main._shutdown = False


def test_worker_loop_inflight_setex_error(monkeypatch):
    main._shutdown = False
    r = _one_iter_redis(("k", _job()))

    def boom(*a):
        raise redis.RedisError("setex down")

    r.setex = boom
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: r)
    monkeypatch.setattr(main, "get_provider", lambda: object())
    monkeypatch.setattr(main.time, "sleep", lambda s: None)
    main.worker_loop(0, object())  # setex(if_key) raises -> RedisError branch
    main._shutdown = False


def test_job_ollama_requeue_lpush_error(monkeypatch):
    class NoLpush(FakeRedis):
        def lpush(self, *a):
            raise redis.RedisError("full")

    monkeypatch.setattr(main, "enrich", lambda s, provider=None: (_ for _ in ()).throw(OllamaUnavailableError("x")))
    main._process_one_job(0, NoLpush(), object(), _job(), "ck", "ifk", _metrics())


def test_job_connection_requeue_lpush_error(monkeypatch):
    class NoLpush(FakeRedis):
        def lpush(self, *a):
            raise redis.RedisError("full")

    monkeypatch.setattr(main.time, "sleep", lambda s: None)
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: (_ for _ in ()).throw(Exception("connection reset")))
    main._process_one_job(0, NoLpush(), object(), _job(), "ck", "ifk", _metrics())


def test_job_cache_write_requeue_lpush_error_to_dlq(monkeypatch):
    class Bad(FakeRedis):
        def setex(self, *a):
            raise redis.RedisError("write")

        def lpush(self, key, val):
            if key != main.DLQ_KEY:
                raise redis.RedisError("full")
            super().lpush(key, val)

    r = Bad()
    monkeypatch.setattr(main, "enrich", lambda s, provider=None: _result())
    main._process_one_job(0, r, object(), _job(), "ck", "ifk", _metrics())
    assert r.dlq  # requeue failed -> dead-lettered


def test_run_orchestration_cloud(monkeypatch):
    _patch_run(monkeypatch, "groq", (set(), set()))
    main._shutdown = True  # while-loop body skipped; drains immediately
    main.run()
    main._shutdown = False


def test_run_orchestration_ollama_with_unfinished_workers(monkeypatch):
    # ollama branch (host/model log + wait_for_ollama) and the not_done cleanup path.
    _patch_run(monkeypatch, "ollama", ({"FUTURE"}, {"FUTURE"}))
    monkeypatch.setattr(main.redis, "Redis", lambda connection_pool=None: type("R", (), {"delete": lambda self, k: None})())
    main._worker_inflight[99] = "enrichment:inflight:ns:app"  # so the cleanup loop has work
    main._shutdown = True
    main.run()
    main._worker_inflight.pop(99, None)
    main._shutdown = False
