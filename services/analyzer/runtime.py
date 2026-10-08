"""Model runtime state: the one place that probes Ollama and pulls models.

check() never pulls. start_pull has exactly two callers: ensure_model (the
worker's run path and the config poll) and the runtime/pull route. Every change of state, model,
reason or enabled publishes runtime.changed. On the bundled runtime the other catalog models but the
default are deleted before a pull and, from the config poll, once the configured model is ready.
"""

from __future__ import annotations

import asyncio
import re
import time
from collections.abc import Callable, Iterator
from contextlib import contextmanager

import httpx

import exporter
from app_logger import logger
from config import ANALYZER_MODE, OLLAMA_AUTO_PULL, OLLAMA_PRUNE_MODELS
from constants import (
    CAPABILITY_TOOLS,
    DEFAULT_ANALYZER_MODEL,
    EVENT_RUNTIME_CHANGED,
    EVENT_RUNTIME_PULL,
    LICENSES,
    LOG_PRUNE_FAILED,
    LOG_PULL_FAILED,
    MODE_DEEP,
    MODEL_IN_USE_POLL_S,
    MODEL_NAME_PATTERN,
    PULL_DEADLINE_S,
    PULL_PROGRESS_INTERVAL_S,
    RUN_ERROR_MODEL_NOT_INSTALLED,
    RUNTIME_REASON_TEMPLATE,
    RUNTIME_STATE_ABSENT,
    RUNTIME_STATE_MODEL_MISSING,
    RUNTIME_STATE_PULLING,
    RUNTIME_STATE_READY,
    RUNTIME_STATE_UNREACHABLE,
    RUNTIME_STATE_UNSUPPORTED,
    VALIDATE_REASON_INVALID_MODEL_NAME,
    VALIDATE_REASON_MODEL_LACKS_TOOLS,
)
from events import Broadcaster
from models import PullProgress, RuntimeStatus, ValidateModelResponse
from providers import ollama
from providers.ollama import ModelMissing, OllamaAbsent, OllamaError

_VALIDATE_REASONS = {
    RUNTIME_STATE_ABSENT: RUNTIME_REASON_TEMPLATE.format(RUNTIME_STATE_ABSENT),
    RUNTIME_STATE_UNREACHABLE: RUNTIME_REASON_TEMPLATE.format(RUNTIME_STATE_UNREACHABLE),
    RUNTIME_STATE_MODEL_MISSING: RUN_ERROR_MODEL_NOT_INSTALLED,
    RUNTIME_STATE_UNSUPPORTED: VALIDATE_REASON_MODEL_LACKS_TOOLS,
    RUNTIME_STATE_READY: "",
}


def pull_allowed(model: str) -> bool:
    """Only catalog models are pulled: an arbitrary library model could fill the Ollama volume.
    A row with a warning (a non-commercial license) is there for validate() only."""
    return model in LICENSES and not LICENSES[model][1]


class Runtime:
    def __init__(
        self,
        client: httpx.AsyncClient,
        events: Broadcaster,
        clock: Callable[[], float] = time.monotonic,
        mode: str = ANALYZER_MODE,
    ) -> None:
        self._client = client
        self._events = events
        self._clock = clock
        # Nothing may run before the first check proves the runtime reachable. mode and autoPull are
        # deploy-time: the status carries them so the UI never has to guess.
        self.status = RuntimeStatus(state=RUNTIME_STATE_UNREACHABLE, mode=mode, autoPull=OLLAMA_AUTO_PULL)
        self.pull_task: asyncio.Task | None = None
        self._pull_errors: dict[str, str] = {}
        self._pull_published_at = 0.0
        self._in_use = 0
        self._prune_failed = False

    @property
    def pulling(self) -> bool:
        return self.pull_task is not None and not self.pull_task.done()

    @contextmanager
    def using_model(self) -> Iterator[None]:
        """Held around every call that runs a model: a pull waits for it before deleting models."""
        self._in_use += 1
        try:
            yield
        finally:
            self._in_use -= 1

    def _publish_changed(self) -> None:
        s = self.status
        # The UI replaces its runtime object with this payload, so it carries every status field.
        self._events.publish(EVENT_RUNTIME_CHANGED, "", dict(
            state=s.state, model=s.model, reason=s.reason, mode=s.mode, autoPull=s.autoPull, enabled=s.enabled))

    def _set(self, state: str, model: str, reason: str) -> str:
        s = self.status
        if (s.state, s.model, s.reason) != (state, model, reason):
            s.state, s.model, s.reason = state, model, reason
            self._publish_changed()
        return state

    def set_enabled(self, enabled: bool) -> None:
        """TelarkConfig ai.enabled, for the users who may read the runtime but not the settings."""
        if self.status.enabled != enabled:
            self.status.enabled = enabled
            self._publish_changed()

    async def _probe(self, model: str) -> tuple[str, str, list[str]]:
        """(state, reason, capabilities); a pull's error stays the reason while its model is missing."""
        try:
            await ollama.tags(self._client)
        except OllamaAbsent as e:
            return RUNTIME_STATE_ABSENT, str(e), []
        except OllamaError as e:
            return RUNTIME_STATE_UNREACHABLE, str(e), []
        try:
            capabilities = await ollama.show(self._client, model)
        except ModelMissing:
            return RUNTIME_STATE_MODEL_MISSING, self._pull_errors.get(model, ""), []
        except OllamaError as e:
            return RUNTIME_STATE_UNREACHABLE, str(e), []
        # Only the deep loop calls tools; the fast narration needs just an installed model.
        if self.status.mode == MODE_DEEP and CAPABILITY_TOOLS not in capabilities:
            return RUNTIME_STATE_UNSUPPORTED, "", capabilities
        return RUNTIME_STATE_READY, "", capabilities

    async def check(self, model: str) -> str:
        """Probe and publish any change. While a pull runs the state stays pulling."""
        if not self.pulling:
            state, reason, _caps = await self._probe(model)
            # A pull may have started while probing: it owns the state until it ends.
            if not self.pulling:
                return self._set(state, model, reason)
        return RUNTIME_STATE_PULLING

    def ensure_model(self, model: str) -> None:
        """Pull the model only when it is missing, in the catalog, autoPull is on and no pull runs."""
        if (self.status.state == RUNTIME_STATE_MODEL_MISSING and OLLAMA_AUTO_PULL and not self.pulling
                and pull_allowed(model)):
            self.start_pull(model)

    def start_pull(self, model: str) -> None:
        if self.pulling:
            return
        # The config poll retries a failed pull every tick: only the first failure of a streak is logged.
        retry = self._pull_errors.pop(model, None) is not None
        self._set(RUNTIME_STATE_PULLING, model, "")
        self.pull_task = asyncio.create_task(self._pull(model, quiet=retry))

    async def _pull(self, model: str, quiet: bool = False) -> None:
        try:
            if OLLAMA_PRUNE_MODELS:
                await self._prune(keep=model)
            async with asyncio.timeout(PULL_DEADLINE_S):
                await ollama.pull(self._client, model, self._on_progress)
        except Exception as e:  # any failure ends the pull; the re-check below reports the state
            if not quiet:
                logger.warning(LOG_PULL_FAILED, type(e).__name__)
            self._pull_errors[model] = str(e) or type(e).__name__
        self.status.pull = None
        state, reason, _caps = await self._probe(model)
        self._set(state, model, reason)

    async def sync(self, exporter_client: httpx.AsyncClient | None) -> None:
        """One config-poll step. GET runtime runs it too, so a model saved in Settings shows at once."""
        await exporter.refresh(exporter_client)
        cfg = exporter.current()
        self.set_enabled(cfg.enabled)
        await self.check(cfg.model)
        if cfg.enabled:
            self.ensure_model(cfg.model)
        await self.prune_idle(cfg.model)

    async def prune_idle(self, model: str) -> None:
        """A switch to an installed model starts no pull: the config poll deletes the leftovers once it is ready."""
        if (not OLLAMA_PRUNE_MODELS or self.pulling or self._in_use
                or (self.status.state, self.status.model) != (RUNTIME_STATE_READY, model)):
            return
        try:
            await self._prune(keep=model)
        except OllamaError as e:
            # Retried every poll: only the first failure of a streak is logged.
            if not self._prune_failed:
                logger.warning(LOG_PRUNE_FAILED, type(e).__name__)
            self._prune_failed = True
            return
        self._prune_failed = False

    async def _prune(self, keep: str) -> None:
        """The volume holds the default plus one catalog model, never the old one beside the new."""
        while self._in_use:
            await asyncio.sleep(MODEL_IN_USE_POLL_S)
        for name in await ollama.tags(self._client):
            if name in LICENSES and name not in (keep, DEFAULT_ANALYZER_MODEL):
                await ollama.delete(self._client, name)

    def _on_progress(self, progress: PullProgress) -> None:
        last = self.status.pull
        self.status.pull = progress
        now = self._clock()
        # Ollama sends many lines per second; unthrottled they would overflow every SSE queue.
        if last is None or last.status != progress.status or now - self._pull_published_at >= PULL_PROGRESS_INTERVAL_S:
            self._pull_published_at = now
            self._events.publish(EVENT_RUNTIME_PULL, "", progress.model_dump())

    async def validate(self, model: str) -> ValidateModelResponse:
        """Report whether the model can run the analyzer; never changes the runtime state."""
        license_, warning = LICENSES.get(model, ("", ""))
        resp = ValidateModelResponse(model=model, license=license_, warning=warning)
        if not re.fullmatch(MODEL_NAME_PATTERN, model):
            resp.reason = VALIDATE_REASON_INVALID_MODEL_NAME
            return resp
        state, _reason, resp.capabilities = await self._probe(model)
        resp.reason = _VALIDATE_REASONS[state]
        resp.ok = state == RUNTIME_STATE_READY
        return resp
