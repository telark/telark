"""In-process fan-out of analyzer events to the open SSE connections (one replica, one event loop).

Each connection reads its own bounded queue: a per-connection Redis XREAD would pin a pooled connection per open tab.
"""

from __future__ import annotations

import asyncio

from constants import EVENT_RESYNC, SSE_MAX_STREAMS_PER_USER
from models import Subscription


class Broadcaster:
    def __init__(self) -> None:
        self._subs: set[Subscription] = set()

    def subscribe(self, apps: set[str], user: str = "") -> Subscription | None:
        """None once user already holds SSE_MAX_STREAMS_PER_USER subscriptions."""
        if sum(1 for s in self._subs if s.user == user) >= SSE_MAX_STREAMS_PER_USER:
            return None
        sub = Subscription(apps=apps, user=user)
        self._subs.add(sub)
        return sub

    def unsubscribe(self, sub: Subscription) -> None:
        self._subs.discard(sub)

    def publish(self, name: str, app: str, data: dict) -> None:
        """app '' is a global event (runtime.*) for every subscription; else only subscriptions holding app."""
        if app:
            data = dict(data, app=app)
        for sub in self._subs:
            if app and app not in sub.apps:
                continue
            try:
                sub.queue.put_nowait((name, data))
            except asyncio.QueueFull:
                # The client refetches on resync, so dropping the backlog loses nothing.
                while not sub.queue.empty():
                    sub.queue.get_nowait()
                sub.queue.put_nowait((EVENT_RESYNC, {}))
