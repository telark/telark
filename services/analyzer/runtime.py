"""Model runtime state: the one place that probes Ollama and pulls models.

check() never pulls. start_pull has exactly two callers: ensure_model (the
worker's run path and the config poll) and the runtime/pull route. Every change of state, model or
reason publishes runtime.changed.
"""

from __future__ import annotations

import asyncio
import re
import time
from collections.abc import Callable

import httpx

from app_logger import logger
from config import ANALYZER_MODE, OLLAMA_AUTO_PULL
from constants import (
    CAPABILITY_TOOLS,
    EVENT_RUNTIME_CHANGED,
    EVENT_RUNTIME_PULL,
    LICENSES,
    LOG_PULL_FAILED,
    MODE_DEEP,
    MODEL_NAME_PATTERN,
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

    @property
    def pulling(self) -> bool:
        return self.pull_task is not None and not self.pull_task.done()

    def _set(self, state: str, model: str, reason: str) -> str:
        s = self.status
        if (s.state, s.model, s.reason) != (state, model, reason):
            s.state, s.model, s.reason = state, model, reason
            # The UI replaces its runtime object with this payload, so it carries every status field.
            self._events.publish(EVENT_RUNTIME_CHANGED, "", dict(
                state=state, model=model, reason=reason, mode=s.mode, autoPull=s.autoPull))
        return state

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
        """Pull the model only when it is missing, autoPull is on and no pull runs."""
        if self.status.state == RUNTIME_STATE_MODEL_MISSING and OLLAMA_AUTO_PULL and not self.pulling:
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
            await ollama.pull(self._client, model, self._on_progress)
        except Exception as e:  # any failure ends the pull; the re-check below reports the state
            if not quiet:
                logger.warning(LOG_PULL_FAILED, type(e).__name__)
            self._pull_errors[model] = str(e) or type(e).__name__
        self.status.pull = None
        state, reason, _caps = await self._probe(model)
        self._set(state, model, reason)

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
