"""Checks for the in-process broadcaster: app scoping and the bounded per-connection queue.

Run: python -m pytest tests/test_events_cov.py
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from constants import EVENT_RESYNC, SSE_MAX_STREAMS_PER_USER, SSE_QUEUE_MAX  # noqa: E402
from events import Broadcaster  # noqa: E402


def _drain(sub):
    out = []
    while not sub.queue.empty():
        out.append(sub.queue.get_nowait())
    return out


def test_app_events_reach_only_their_subscriptions():
    b = Broadcaster()
    shop = b.subscribe({"shop/api", "shop/web"})
    pay = b.subscribe({"pay/api"})

    b.publish("insight.created", "shop/api", {"id": "x", "version": 2})
    b.publish("runtime.changed", "", {"state": "ready", "model": "m", "reason": ""})

    runtime = ("runtime.changed", {"state": "ready", "model": "m", "reason": ""})
    assert _drain(shop) == [("insight.created", {"id": "x", "version": 2, "app": "shop/api"}), runtime]
    assert _drain(pay) == [runtime]


def test_unsubscribe_stops_delivery():
    b = Broadcaster()
    sub = b.subscribe({"shop/api"})
    b.unsubscribe(sub)
    b.unsubscribe(sub)
    b.publish("runtime.changed", "", {})
    b.publish("analysis.started", "shop/api", {"version": 1})
    assert sub.queue.empty()


def test_overflow_drains_and_resyncs():
    b = Broadcaster()
    slow = b.subscribe({"shop/api"})
    fast = b.subscribe({"shop/api"})
    for version in range(SSE_QUEUE_MAX):
        b.publish("insight.updated", "shop/api", {"version": version})
    assert slow.queue.full()
    assert len(_drain(fast)) == SSE_QUEUE_MAX

    b.publish("analysis.finished", "shop/api", {"version": SSE_QUEUE_MAX})

    # The backlog and the event that overflowed are replaced by one resync; the client refetches.
    assert _drain(slow) == [(EVENT_RESYNC, {})]
    assert _drain(fast) == [("analysis.finished", {"version": SSE_QUEUE_MAX, "app": "shop/api"})]
    b.publish("analysis.started", "shop/api", {"version": 1})
    assert _drain(slow) == [("analysis.started", {"version": 1, "app": "shop/api"})]


def test_subscriptions_are_capped_per_user():
    b = Broadcaster()
    subs = [b.subscribe(set(), "u1") for _ in range(SSE_MAX_STREAMS_PER_USER)]
    assert all(subs) and b.subscribe(set(), "u1") is None
    assert b.subscribe(set(), "u2") is not None
    b.unsubscribe(subs[0])
    assert b.subscribe(set(), "u1") is not None
