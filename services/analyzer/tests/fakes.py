"""Hand-written fakes (async Redis, the Ollama HTTP API) shared by the analyzer suites.

Not collected (no test_ prefix). CI installs only requirements.txt + pytest +
pytest-cov, so there is no fakeredis. TTLs are recorded, not enforced; there is
no real clock: stream ids and idle times come from `clock_ms`, set by the test.
"""

import json
import socket
from fnmatch import fnmatch

import httpx
from redis import ResponseError

BUSYGROUP = "BUSYGROUP Consumer Group name already exists"


class FakeRedis:
    def __init__(self) -> None:
        self.store: dict[str, str] = {}
        self.ttls: dict[str, int] = {}
        self.hashes: dict[str, dict[str, str]] = {}
        self.zsets: dict[str, dict[str, float]] = {}
        self.streams: dict[str, list[tuple[str, dict]]] = {}
        # (stream, group) -> {"last": last delivered id, "pending": {id: (consumer, delivered_ms)}}
        self.groups: dict[tuple[str, str], dict] = {}
        self.clock_ms = 0
        self.ping_error: Exception | None = None
        self._seq = 0

    async def get(self, key):
        return self.store.get(key)

    async def set(self, key, value, nx=False, ex=None):
        if nx and key in self.store:
            return None
        self.store[key] = value
        self.ttls.pop(key, None)
        if ex is not None:
            self.ttls[key] = ex
        return True

    async def delete(self, *keys):
        found = [k for k in keys if k in self.store]
        for k in found:
            self.store.pop(k)
            self.ttls.pop(k, None)
        return len(found)

    async def exists(self, *keys):
        return sum(k in self.store for k in keys)

    async def ping(self):
        if self.ping_error:
            raise self.ping_error
        return True

    async def xadd(self, name, fields, id="*", maxlen=None, approximate=True):
        if id == "*":
            self._seq += 1
            id = f"{self.clock_ms}-{self._seq}"
        entries = self.streams.setdefault(name, [])
        entries.append((id, dict(fields)))
        if maxlen is not None and len(entries) > maxlen:
            del entries[: len(entries) - maxlen]
        return id

    async def xlen(self, name):
        return len(self.streams.get(name, []))

    async def xgroup_create(self, name, groupname, id="$", mkstream=False):
        if name not in self.streams:
            if not mkstream:
                raise ResponseError("ERR The XGROUP subcommand requires the key to exist")
            self.streams[name] = []
        if (name, groupname) in self.groups:
            raise ResponseError(BUSYGROUP)
        last = self.streams[name][-1][0] if id == "$" and self.streams[name] else "0-0"
        self.groups[(name, groupname)] = {"last": last, "pending": {}}
        return True

    async def xreadgroup(self, groupname, consumername, streams, count=None, block=None):
        out = []
        for name in streams:
            group = self.groups[(name, groupname)]
            ids = [i for i, _ in self.streams.get(name, [])]
            start = ids.index(group["last"]) + 1 if group["last"] in ids else 0
            batch = self.streams.get(name, [])[start:][:count]
            for msg_id, _ in batch:
                group["pending"][msg_id] = (consumername, self.clock_ms)
                group["last"] = msg_id
            if batch:
                out.append([name, batch])
        return out

    async def xack(self, name, groupname, *ids):
        pending = self.groups[(name, groupname)]["pending"]
        return sum(pending.pop(i, None) is not None for i in ids)

    async def xdel(self, name, *ids):
        entries = self.streams.get(name, [])
        kept = [e for e in entries if e[0] not in ids]
        self.streams[name] = kept
        return len(entries) - len(kept)

    async def xautoclaim(self, name, groupname, consumername, min_idle_time, start_id="0-0", count=None):
        pending = self.groups[(name, groupname)]["pending"]
        entries = dict(self.streams.get(name, []))
        claimed, deleted = [], []
        for msg_id, (_, since) in list(pending.items())[:count]:
            if self.clock_ms - since < min_idle_time:
                continue
            if msg_id not in entries:
                deleted.append(msg_id)
                pending.pop(msg_id)
                continue
            pending[msg_id] = (consumername, self.clock_ms)
            claimed.append((msg_id, entries[msg_id]))
        return ["0-0", claimed, deleted]

    async def hget(self, name, key):
        return self.hashes.get(name, {}).get(key)

    async def hset(self, name, key=None, value=None, mapping=None):
        fields = dict(mapping or {})
        if key is not None:
            fields[key] = value
        target = self.hashes.setdefault(name, {})
        added = sum(k not in target for k in fields)
        target.update(fields)
        return added

    async def hdel(self, name, *keys):
        target = self.hashes.get(name, {})
        return sum(target.pop(k, None) is not None for k in keys)

    async def hgetall(self, name):
        return dict(self.hashes.get(name, {}))

    async def zadd(self, name, mapping):
        target = self.zsets.setdefault(name, {})
        added = sum(k not in target for k in mapping)
        target.update(mapping)
        return added

    async def zrem(self, name, *members):
        target = self.zsets.get(name, {})
        return sum(target.pop(m, None) is not None for m in members)

    async def zrangebyscore(self, name, min, max, withscores=False):
        lo, hi = (float(v) for v in (min, max))
        items = sorted((s, m) for m, s in self.zsets.get(name, {}).items() if lo <= s <= hi)
        return [(m, s) for s, m in items] if withscores else [m for _, m in items]

    async def zremrangebyscore(self, name, min, max):
        doomed = await self.zrangebyscore(name, min, max)
        return await self.zrem(name, *doomed) if doomed else 0

    def pipeline(self, transaction=True):
        return FakePipeline(self)

    async def scan_iter(self, match=None, count=None):
        for key in list(self.store):
            if match is None or fnmatch(key, match):
                yield key


class FakePipeline:
    """Queues FakeRedis calls; execute() runs them in order (MULTI/EXEC semantics are not modelled)."""

    def __init__(self, redis):
        self._redis = redis
        self.calls: list[tuple[str, tuple, dict]] = []

    def __getattr__(self, name):
        def queue(*args, **kwargs):
            self.calls.append((name, args, kwargs))
            return self
        return queue

    async def execute(self):
        return [await getattr(self._redis, name)(*args, **kwargs) for name, args, kwargs in self.calls]


class FakeOllama:
    """httpx.MockTransport handler for /api/tags, /api/show, /api/chat and /api/pull; records every request path.

    `overrides[path]` (an httpx.Response or an exception to raise) replaces the default
    answer; `hook(path)` runs before each answer. A pull installs the model only when
    its scripted lines end with 'success'. /api/chat answers the fast narration with
    `narration`, a list of (title, summary).
    """

    def __init__(self, models=("qwen3:4b",), capabilities=("completion", "tools")):
        self.models = set(models)
        self.capabilities = list(capabilities)
        self.overrides: dict = {}
        self.pull_lines = [{"status": "pulling manifest"}, {"status": "success"}]
        self.narration: list[tuple[str, str]] = []
        self.hook = None
        self.paths: list[str] = []

    def __call__(self, request):
        path = request.url.path
        self.paths.append(path)
        if self.hook:
            self.hook(path)
        override = self.overrides.get(path)
        if isinstance(override, Exception):
            raise override
        if override is not None:
            return override
        if path == "/api/tags":
            return httpx.Response(200, json={"models": [{"name": m} for m in sorted(self.models)]})
        body = json.loads(request.content)
        if path == "/api/show":
            if body["model"] not in self.models:
                return httpx.Response(404, json={"error": f"model '{body['model']}' not found"})
            return httpx.Response(200, json={"capabilities": self.capabilities})
        if path == "/api/chat":
            content = json.dumps({"insights": [{"title": t, "summary": s} for t, s in self.narration]})
            return httpx.Response(200, json={"message": {"role": "assistant", "content": content}, "done": True})
        assert path == "/api/pull", path
        if self.pull_lines[-1].get("status") == "success":
            self.models.add(body["model"])
        return httpx.Response(200, text="\n".join(json.dumps(line) for line in self.pull_lines))

    def count(self, path):
        return self.paths.count(path)

    def client(self):
        return httpx.AsyncClient(transport=httpx.MockTransport(self), base_url="http://ollama")


def dns_error():
    """An httpx.ConnectError caused by a DNS failure, as httpx raises it (the absent runtime)."""
    try:
        try:
            raise socket.gaierror(socket.EAI_NONAME, "nodename nor servname provided")
        except OSError as e:
            raise httpx.ConnectError("connect failed") from e
    except httpx.ConnectError as err:
        return err
