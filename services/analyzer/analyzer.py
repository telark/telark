"""One analysis, in either mode. Never touches Redis: the worker persists the result.

deep (run_analysis): the native /api/chat tool loop, then one schema-constrained EMIT.
Code owns every budget (the per-request fit check, the wall with its EMIT reserve,
the 2-strike abort) and maps every runtime failure to a lastRun.error code.

fast (gather_rules, then narrate): the tools are called directly and rules.py decides
the insights; one tool-less call may rephrase their title/summary and never fails the run.
"""

from __future__ import annotations

import asyncio
import json
import re
import time
from collections.abc import Awaitable, Callable
from datetime import datetime

import httpx
from pydantic import ValidationError

import messages
import rules
import tools
from config import (
    ANALYZER_CHARS_PER_TOKEN,
    ANALYZER_CONTEXT_TOKENS,
    ANALYZER_EMIT_TIMEOUT_SEC,
    ANALYZER_LOOP_TIMEOUT_SEC,
    ANALYZER_MAX_STEPS,
    ANALYZER_MAX_TOOL_CALLS,
    ANALYZER_NARRATE_TIMEOUT_SEC,
    ANALYZER_WALL_SEC,
)
from constants import (
    BUSY_SLEEP_S,
    EMIT_ATTEMPTS,
    EMIT_RESERVE_S,
    EMIT_USER_TOKENS,
    FACT_SEPARATOR,
    FAST_EVENTS_SINCE_MIN,
    FAST_HISTORY_LIMIT,
    MAX_CONSECUTIVE_STRIKES,
    MAX_TOOL_CALLS_PER_STEP,
    NARRATE_KIND_WORDS,
    NARRATE_MARKUP_PATTERN,
    NARRATE_NUMBER_PATTERN,
    NARRATE_NAME_TOKEN_PATTERN,
    NARRATE_NUM_PREDICT_BASE,
    NARRATE_NUM_PREDICT_PER_INSIGHT,
    NARRATE_REASON_ITEM_COUNT,
    NARRATE_REASON_UNFAITHFUL,
    NARRATE_SUMMARY_MAX,
    NARRATE_TITLE_MAX,
    NARRATE_WORD_PATTERN,
    NUM_PREDICT_EMIT,
    NUM_PREDICT_LOOP,
    PER_STEP_GROWTH,
    ROLE_SYSTEM,
    ROLE_TOOL,
    ROLE_USER,
    RUN_ERROR_BUSY,
    RUN_ERROR_CONTEXT_OVERFLOW,
    RUN_ERROR_INVALID_OUTPUT,
    RUN_ERROR_INVALID_TOOL_CALLS,
    RUN_ERROR_MODEL_NOT_INSTALLED,
    RUN_ERROR_MODEL_TIMEOUT,
    RUN_ERROR_MODEL_UNSUPPORTED,
    RUN_ERROR_RUNTIME_UNREACHABLE,
    STRIKE_TOOL_ERRORS,
    SUBJECT_SEPARATOR,
    TIMING_GATHER,
    TIMING_RULES,
    TOOL_GET_APP_OVERVIEW,
    TOOL_GET_CHANGE_HISTORY,
    TOOL_GET_RECENT_EVENTS,
    TOOL_GET_WORKLOAD_STATUS,
)
from models import (
    Candidate,
    ChatMessage,
    ChatResponse,
    EmitOutput,
    Emitted,
    Job,
    NarrateOutput,
    Narrated,
    Outcome,
    Run,
    ToolCallFunction,
)
from prompts.analyzer_prompt import (
    EMIT_MESSAGE,
    EMIT_RETRY_MESSAGE,
    EMIT_SCHEMA,
    NARRATE_SYSTEM,
    SYSTEM_PROMPT,
    narrate_message,
    narrate_schema,
    user_message,
)
from providers import ollama
from providers.ollama import (
    ContextOverflow,
    ModelMissing,
    ModelUnsupported,
    OllamaAbsent,
    OllamaBusy,
    OllamaError,
    OllamaTimeout,
    OllamaUnreachable,
)
from tools import validation_detail
from tools.k8s_tools import K8s

# ContextOverflow reaches this map only from the EMIT: at a loop step it forces the EMIT instead.
_FAILURES = {
    OllamaTimeout: RUN_ERROR_MODEL_TIMEOUT,
    OllamaBusy: RUN_ERROR_BUSY,
    OllamaAbsent: RUN_ERROR_RUNTIME_UNREACHABLE,
    OllamaUnreachable: RUN_ERROR_RUNTIME_UNREACHABLE,
    ModelUnsupported: RUN_ERROR_MODEL_UNSUPPORTED,
    ModelMissing: RUN_ERROR_MODEL_NOT_INSTALLED,
    ContextOverflow: RUN_ERROR_CONTEXT_OVERFLOW,
}
_FAILURE_TYPES = tuple(_FAILURES)


def estimate(messages: list[ChatMessage], last_prompt_eval: int, last_eval: int) -> float:
    """Tokens of the request about to be sent. Each call resends the history, so never a sum across calls."""
    size = len(json.dumps([m.model_dump() for m in messages]).encode())
    return max(size / ANALYZER_CHARS_PER_TOKEN, last_prompt_eval + last_eval)


def fits_loop(est: float) -> bool:
    """Room for this step's answer and, after one more step, still for the EMIT."""
    ctx = ANALYZER_CONTEXT_TOKENS
    return est + NUM_PREDICT_LOOP <= ctx and est + PER_STEP_GROWTH + EMIT_USER_TOKENS + NUM_PREDICT_EMIT <= ctx


def fits_emit(est: float) -> bool:
    return est + NUM_PREDICT_EMIT <= ANALYZER_CONTEXT_TOKENS


async def _chat(
    client: httpx.AsyncClient, payload: dict, timeout_s: float, sleep: Callable[[float], Awaitable[None]]
) -> ChatResponse:
    try:
        return await ollama.chat(client, payload, timeout_s)
    except OllamaBusy:
        await sleep(BUSY_SLEEP_S)
        return await ollama.chat(client, payload, timeout_s)


async def _emit(
    client: httpx.AsyncClient,
    model: str,
    messages: list[ChatMessage],
    counters: tuple[int, int],
    deadline: float,
    clock: Callable[[], float],
    sleep: Callable[[float], Awaitable[None]],
) -> list[Emitted] | None:
    """The EMIT, retried once on a decode error; None when both fail. The tools array keeps the KV prefix."""
    messages.append(ChatMessage(role=ROLE_USER, content=EMIT_MESSAGE))
    for _attempt in range(EMIT_ATTEMPTS):
        if not fits_emit(estimate(messages, *counters)):
            raise ContextOverflow()
        payload = ollama.chat_payload(model, messages, tools.SPECS, NUM_PREDICT_EMIT, fmt=EMIT_SCHEMA)
        resp = await _chat(client, payload, min(ANALYZER_EMIT_TIMEOUT_SEC, deadline - clock()), sleep)
        try:
            decoded = EmitOutput.model_validate_json(resp.message.content).insights
            return [Emitted.model_validate(e.model_dump()) for e in decoded]
        except ValidationError as e:
            # Error type and location only; the completion itself is never sent back.
            detail = validation_detail(e)
            messages.append(ChatMessage(role=ROLE_USER, content=EMIT_RETRY_MESSAGE.format(detail=detail)))
    return None


async def run_analysis(
    job: Job,
    run: Run,
    ollama_client: httpx.AsyncClient,
    k8s: K8s | None,
    model: str,
    clock: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
) -> Outcome:
    deadline = clock() + ANALYZER_WALL_SEC
    messages = [
        ChatMessage(role=ROLE_SYSTEM, content=SYSTEM_PROMPT),
        ChatMessage(role=ROLE_USER, content=user_message(job, run.app)),
    ]
    counters = (0, 0)
    steps = strikes = 0
    forced = False
    try:
        while True:
            remaining = deadline - clock()
            if (
                remaining < EMIT_RESERVE_S
                or not fits_loop(estimate(messages, *counters))
                or steps == ANALYZER_MAX_STEPS
                or run.tool_calls >= ANALYZER_MAX_TOOL_CALLS
            ):
                forced = True
                break
            payload = ollama.chat_payload(model, messages, tools.SPECS, NUM_PREDICT_LOOP)
            try:
                resp = await _chat(
                    ollama_client, payload, min(ANALYZER_LOOP_TIMEOUT_SEC, remaining - ANALYZER_EMIT_TIMEOUT_SEC), sleep
                )
            except ContextOverflow:
                forced = True
                break
            steps += 1
            counters = (resp.prompt_eval_count, resp.eval_count)
            message = resp.message
            message.tool_calls = message.tool_calls[:MAX_TOOL_CALLS_PER_STEP]
            messages.append(message)
            calls = [c.function for c in message.tool_calls]
            if not calls:
                # Small models sometimes write the call as text: one strict parse, else the model is done.
                try:
                    calls = [ToolCallFunction.model_validate_json(message.content)]
                except ValidationError:
                    break
            for call in calls:
                result = await tools.invoke(run, call.name, call.arguments, k8s)
                messages.append(ChatMessage(role=ROLE_TOOL, content=result.content, tool_name=call.name))
                strikes = strikes + 1 if result.error in STRIKE_TOOL_ERRORS else 0
                if strikes >= MAX_CONSECUTIVE_STRIKES:
                    return Outcome(steps=steps, tool_calls=run.tool_calls, error=RUN_ERROR_INVALID_TOOL_CALLS)
        emitted = await _emit(ollama_client, model, messages, counters, deadline, clock, sleep)
    except _FAILURE_TYPES as e:
        return Outcome(steps=steps, tool_calls=run.tool_calls, error=_FAILURES[type(e)])
    if emitted is None:
        return Outcome(steps=steps, tool_calls=run.tool_calls, error=RUN_ERROR_INVALID_OUTPUT)
    return Outcome(emitted=emitted, truncated=run.truncated or forced, steps=steps, tool_calls=run.tool_calls)


async def gather_rules(
    run: Run, k8s: K8s | None, now: datetime, timings: dict[str, float], clock: Callable[[], float] = time.monotonic
) -> list[Candidate]:
    """The fast path's reads (same caps, refs, caches and error mapping as the loop's), then the rules.

    Fills timings[gather] and timings[rules] in seconds.
    """
    start = clock()
    overview = await tools.invoke(run, TOOL_GET_APP_OVERVIEW, {}, k8s)
    history = await tools.invoke(run, TOOL_GET_CHANGE_HISTORY, {"limit": FAST_HISTORY_LIMIT}, k8s)
    await tools.invoke(run, TOOL_GET_RECENT_EVENTS, {"sinceMinutes": FAST_EVENTS_SINCE_MIN}, k8s)
    for key in rules.select_status_subjects(run, run.events_cache):
        kind, name, namespace = run.workloads[key]
        await tools.invoke(run, TOOL_GET_WORKLOAD_STATUS, {"kind": kind, "name": name, "namespace": namespace}, k8s)
    gathered = clock()
    # tools.fit drops whole list items only, so every content is valid JSON; an error reads as absent.
    candidates = rules.evaluate(run, json.loads(overview.content), json.loads(history.content), run.status_cache,
                                run.events_cache, now)
    timings[TIMING_GATHER], timings[TIMING_RULES] = gathered - start, clock() - gathered
    return candidates


async def narrate(
    client: httpx.AsyncClient,
    model: str,
    candidates: list[Candidate],
    sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
) -> tuple[list[Narrated] | None, str]:
    """(one title/summary per candidate, '') or (None, why); never raises for a runtime or decode failure."""
    n = len(candidates)
    messages = [ChatMessage(role=ROLE_SYSTEM, content=NARRATE_SYSTEM),
                ChatMessage(role=ROLE_USER, content=narrate_message(candidates))]
    payload = ollama.chat_payload(model, messages, [], NARRATE_NUM_PREDICT_BASE + NARRATE_NUM_PREDICT_PER_INSIGHT * n,
                                  fmt=narrate_schema(n))
    try:
        resp = await _chat(client, payload, ANALYZER_NARRATE_TIMEOUT_SEC, sleep)
        items = NarrateOutput.model_validate_json(resp.message.content).insights
    # The OllamaError base covers the 500/400s _FAILURES does not map; decode errors are ValueErrors.
    except (OllamaError, ValueError) as e:
        return None, type(e).__name__
    if len(items) != n:
        return None, NARRATE_REASON_ITEM_COUNT
    # An unfaithful item goes back blank, which keeps its template.
    items = [it if _faithful(it, c) else Narrated(title="", summary="") for it, c in zip(items, candidates)]
    if not any(it.title for it in items):
        return None, NARRATE_REASON_UNFAITHFUL
    return items, ""


def _other_kind_word(text: str, template: str, kind: str) -> bool:
    """A symptom word of another incident kind that the template never used: the narration changed the cause."""
    return any(re.search(NARRATE_WORD_PATTERN.format(re.escape(w)), text)
               and not re.search(NARRATE_WORD_PATTERN.format(re.escape(w)), template)
               for k, words in NARRATE_KIND_WORDS.items() if k != kind for w in words)


def _faithful(item: Narrated, candidate: Candidate) -> bool:
    """Rephrasing only: the title names the workload, every value the template states stays, the sub-reason's
    phrase stays, no markup, no new name or number, no other kind's symptom.

    Text at the schema's cap was cut mid-sentence by the grammar."""
    if len(item.title) >= NARRATE_TITLE_MAX or len(item.summary) >= NARRATE_SUMMARY_MAX:
        return False
    text = f"{item.title} {item.summary}"
    template = f"{candidate.title} {candidate.summary}"
    source = " ".join([candidate.subject, *candidate.facts, template])
    stated = [v for f in candidate.facts if (v := f.partition(FACT_SEPARATOR)[2]) and v in candidate.summary]
    phrase = messages.what(candidate.reason, candidate.params)
    return (candidate.subject.partition(SUBJECT_SEPARATOR)[2] in item.title
            and all(v in text for v in stated)
            and phrase.lower() in text.lower()
            and not _other_kind_word(text.lower(), template.lower(), candidate.kind)
            and not re.search(NARRATE_MARKUP_PATTERN, text)
            and set(re.findall(NARRATE_NUMBER_PATTERN, text)) <= set(re.findall(NARRATE_NUMBER_PATTERN, source))
            and all(w.rstrip(".:") in source for w in re.findall(NARRATE_NAME_TOKEN_PATTERN, text)))
