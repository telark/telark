"""Checks for the insight store's Redis calls: the document, inflight, cooldowns, job ACKs.

Discovery reads the same document key, so the key format, the TTL and the legacy
rule must match the Go side (insights.DocumentKey, discovery cache.go).

Run: python -m pytest tests/test_insights_cov.py
"""

import asyncio
import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from config import ANALYZER_AUTO_COOLDOWN_SEC, ANALYZER_MANUAL_COOLDOWN_SEC  # noqa: E402
from constants import CONSUMER_GROUP, DOCUMENT_TTL_S, INFLIGHT_TTL_S, RUN_STATUS_DONE, STREAM_JOBS  # noqa: E402
from fakes import FakeRedis  # noqa: E402
from insights import InsightStore  # noqa: E402
from models import AppInsights, Insight, LastRun  # noqa: E402

KEY = "analyzer:shop:api"


def _doc():
    return AppInsights(
        insights=[Insight(id="abc", kind="crashloop", subject="deployment/api", status="open", runs=1)],
        lastRun=LastRun(status=RUN_STATUS_DONE, trigger="manual", runId="1-1"),
        version=3,
    )


def test_get_missing_key_is_empty():
    assert asyncio.run(InsightStore(FakeRedis()).get("shop", "api")) == AppInsights()


def test_legacy_document_is_empty():
    fake = FakeRedis()
    fake.store[KEY] = json.dumps({"summary": "old analysis", "role": "backend", "confidence": "high"})
    assert asyncio.run(InsightStore(fake).get("shop", "api")) == AppInsights()


def test_undecodable_document_is_empty():
    fake = FakeRedis()
    fake.store[KEY] = "not-json{"
    assert asyncio.run(InsightStore(fake).get("shop", "api")) == AppInsights()
    fake.store[KEY] = json.dumps([1, 2])
    assert asyncio.run(InsightStore(fake).get("shop", "api")) == AppInsights()


def test_put_records_ttl_and_round_trips():
    fake = FakeRedis()
    store = InsightStore(fake)
    asyncio.run(store.put("shop", "api", _doc()))
    assert fake.ttls[KEY] == DOCUMENT_TTL_S == 604800
    assert asyncio.run(store.get("shop", "api")) == _doc()


def test_delete():
    fake = FakeRedis()
    store = InsightStore(fake)
    asyncio.run(store.put("shop", "api", _doc()))
    asyncio.run(store.delete("shop", "api"))
    assert KEY not in fake.store
    assert asyncio.run(store.get("shop", "api")) == AppInsights()


def test_update_writes_only_on_change():
    async def go():
        fake = FakeRedis()
        store = InsightStore(fake)
        await store.put("shop", "api", _doc())
        raw = fake.store[KEY]
        fake.ttls.clear()

        doc, changed = await store.update("shop", "api", lambda d: False)
        assert (changed, doc.version, fake.store[KEY], fake.ttls) == (False, 3, raw, {})

        def bump(d):
            d.version += 1
            return True

        doc, changed = await store.update("shop", "api", bump)
        assert (changed, doc.version, fake.ttls[KEY]) == (True, 4, DOCUMENT_TTL_S)
        assert (await store.get("shop", "api")).version == 4

        # concurrent updates serialize on the lock: no lost increment even when a read yields
        read = fake.get

        async def yielding_get(key):
            value = await read(key)
            await asyncio.sleep(0)
            return value

        fake.get = yielding_get
        await asyncio.gather(*(store.update("shop", "api", bump) for _ in range(5)))
        assert (await store.get("shop", "api")).version == 9

    asyncio.run(go())


def test_inflight_is_held_and_released_by_holder_only():
    async def go():
        fake = FakeRedis()
        store = InsightStore(fake)
        assert await store.acquire_inflight("shop", "api", "1-1") == (True, "1-1")
        assert fake.ttls["analyzer:inflight:shop:api"] == INFLIGHT_TTL_S == 540
        assert await store.acquire_inflight("shop", "api", "2-1") == (False, "1-1")

        await store.release_inflight("shop", "api", "2-1")
        assert fake.store["analyzer:inflight:shop:api"] == "1-1"
        await store.release_inflight("shop", "api", "1-1")
        assert "analyzer:inflight:shop:api" not in fake.store
        await store.release_inflight("shop", "api", "1-1")  # already gone: no-op
        assert await store.acquire_inflight("shop", "api", "2-1") == (True, "2-1")

    asyncio.run(go())


def test_cooldowns_are_nx():
    async def go():
        fake = FakeRedis()
        store = InsightStore(fake)
        assert await store.cooldown_manual("shop", "api") is True
        assert await store.cooldown_manual("shop", "api") is False
        assert fake.ttls["analyzer:cooldown:manual:shop:api"] == ANALYZER_MANUAL_COOLDOWN_SEC
        assert await store.cooldown_auto("shop", "api") is True
        assert await store.cooldown_auto("shop", "api") is False
        assert fake.ttls["analyzer:cooldown:auto:shop:api"] == ANALYZER_AUTO_COOLDOWN_SEC

    asyncio.run(go())


def test_ack_job_leaves_xlen_zero():
    async def go():
        fake = FakeRedis()
        store = InsightStore(fake)
        await fake.xgroup_create(STREAM_JOBS, CONSUMER_GROUP, id="0", mkstream=True)
        await fake.xadd(STREAM_JOBS, {"namespace": "shop", "name": "api"})
        await fake.xadd(STREAM_JOBS, {"namespace": "shop", "name": "web"})
        assert await store.queue_len() == 2

        [[_, batch]] = await fake.xreadgroup(CONSUMER_GROUP, "c1", {STREAM_JOBS: ">"}, count=2)
        for msg_id, _ in batch:
            await store.ack_job(msg_id)
        assert await store.queue_len() == 0
        assert fake.groups[(STREAM_JOBS, CONSUMER_GROUP)]["pending"] == {}

    asyncio.run(go())


def test_put_updates_index_and_delete_removes(monkeypatch):
    import insights

    fake = FakeRedis()
    calls = []
    real_set, real_zadd = fake.set, fake.zadd

    async def set_(*args, **kwargs):
        calls.append("set")
        return await real_set(*args, **kwargs)

    async def zadd(*args, **kwargs):
        calls.append("zadd")
        return await real_zadd(*args, **kwargs)

    monkeypatch.setattr(fake, "set", set_)
    monkeypatch.setattr(fake, "zadd", zadd)
    monkeypatch.setattr(insights, "epoch_ms", lambda: 1_700_000_000_000)
    store = InsightStore(fake)
    asyncio.run(store.put("shop", "api", _doc()))
    # Write order: the document first, then its index entry (discovery re-reads a member when its score moves).
    assert calls == ["set", "zadd"]
    assert fake.zsets["analyzer:index"] == {"shop/api": 1_700_000_000_000}
    real_delete, real_zrem = fake.delete, fake.zrem

    async def delete(*args):
        calls.append("del")
        return await real_delete(*args)

    async def zrem(*args):
        calls.append("zrem")
        return await real_zrem(*args)

    monkeypatch.setattr(fake, "delete", delete)
    monkeypatch.setattr(fake, "zrem", zrem)
    asyncio.run(store.delete("shop", "api"))
    assert KEY not in fake.store and fake.zsets["analyzer:index"] == {}
    # Removal: the document first, then the member (a reader never follows a member to a missing document it
    # would cache as current).
    assert calls == ["set", "zadd", "del", "zrem"]
    # Every path that writes the document (runs, reviews, triage, drops) goes through update() -> put().
    asyncio.run(store.update("shop", "api", lambda doc: True))
    assert calls[-2:] == ["set", "zadd"]


def test_gc_index():
    fake = FakeRedis()
    now = 2_000_000_000_000
    fake.zsets["analyzer:index"] = {"shop/old": now - DOCUMENT_TTL_S * 1000 - 1, "shop/new": now - 1000}
    assert asyncio.run(InsightStore(fake).gc_index(now)) == 1
    assert fake.zsets["analyzer:index"] == {"shop/new": now - 1000}
