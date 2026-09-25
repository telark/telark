"""The shared Redis fake must behave like Redis, or the suites built on it prove nothing.

Run: pytest tests/test_fakes_cov.py
"""

import asyncio

import pytest
from redis import ResponseError

from fakes import BUSYGROUP, FakeRedis


def test_strings_and_ttls():
    async def go():
        r = FakeRedis()
        assert await r.set("k", "1", nx=True, ex=540) is True
        assert await r.set("k", "2", nx=True) is None
        assert r.store["k"] == "1" and r.ttls["k"] == 540
        await r.set("k", "3")
        assert "k" not in r.ttls
        assert await r.exists("k", "missing") == 1
        assert [k async for k in r.scan_iter(match="analyzer:*")] == []
        assert [k async for k in r.scan_iter()] == ["k"]
        assert await r.delete("k", "missing") == 1
        assert await r.ping() is True

    asyncio.run(go())


def test_consumer_group_round_trip():
    async def go():
        r = FakeRedis()
        with pytest.raises(ResponseError):
            await r.xgroup_create("s", "g")
        await r.xgroup_create("s", "g", id="0", mkstream=True)
        with pytest.raises(ResponseError, match=BUSYGROUP):
            await r.xgroup_create("s", "g", id="0", mkstream=True)

        r.clock_ms = 1000
        first = await r.xadd("s", {"name": "a"}, maxlen=2)
        await r.xadd("s", {"name": "b"}, maxlen=2)
        await r.xadd("s", {"name": "c"}, maxlen=2)
        assert first == "1000-1" and await r.xlen("s") == 2  # trimmed to maxlen

        await r.xgroup_create("s", "late")  # '$': only new entries
        assert await r.xreadgroup("late", "c1", {"s": ">"}, count=1) == []

        [[name, batch]] = await r.xreadgroup("g", "c1", {"s": ">"}, count=1)
        assert name == "s" and batch[0][1] == {"name": "b"}
        assert await r.xautoclaim("s", "g", "c2", min_idle_time=600000) == ["0-0", [], []]

        r.clock_ms += 600000
        _, claimed, deleted = await r.xautoclaim("s", "g", "c2", min_idle_time=600000, count=10)
        assert [m for m, _ in claimed] == [batch[0][0]] and deleted == []

        assert await r.xack("s", "g", batch[0][0]) == 1
        assert await r.xdel("s", batch[0][0]) == 1

        [[_, rest]] = await r.xreadgroup("g", "c1", {"s": ">"}, count=5)
        await r.xdel("s", rest[0][0])
        r.clock_ms += 600000
        assert (await r.xautoclaim("s", "g", "c2", min_idle_time=0))[2] == [rest[0][0]]

    asyncio.run(go())


def test_hashes_zsets_and_pipeline():
    async def go():
        r = FakeRedis()
        assert await r.hset("h", "a", "1") == 1 and await r.hset("h", mapping={"a": "2", "b": "3"}) == 1
        assert await r.hget("h", "a") == "2" and await r.hget("h", "x") is None
        assert await r.hgetall("h") == {"a": "2", "b": "3"}
        assert await r.hdel("h", "a", "x") == 1 and await r.hgetall("missing") == {}
        assert await r.zadd("z", {"a": 3, "b": 1}) == 2 and await r.zadd("z", {"a": 5}) == 0
        assert await r.zrangebyscore("z", "-inf", "+inf") == ["b", "a"]
        assert await r.zrangebyscore("z", 2, 10, withscores=True) == [("a", 5)]
        assert await r.zremrangebyscore("z", "-inf", 1) == 1 and await r.zremrangebyscore("z", 100, 200) == 0
        assert await r.zrem("z", "a", "x") == 1 and r.zsets["z"] == {}
        pipe = r.pipeline(transaction=False)
        pipe.set("k", "v", ex=5).zadd("z", {"k": 1})
        assert await pipe.execute() == [True, 1] and r.store["k"] == "v" and r.zsets["z"] == {"k": 1}

    asyncio.run(go())
