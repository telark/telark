"""The analyzer's four read-only tools: one registry for schema, validation and dispatch.

`invoke` never raises for a tool failure: every guard answers a ToolResult whose
`error` the model reads back, so one API failure never ends the run.
"""

from __future__ import annotations

import asyncio
import json

import httpx
from pydantic import BaseModel, ValidationError

from config import ANALYZER_TOOL_RESULT_MAX_BYTES
from constants import (
    JSON_COMPACT_SEPARATORS,
    SUBJECT_TEMPLATE,
    TOOL_CAPS,
    TOOL_DESCRIPTIONS,
    TOOL_ERROR_CALL_CAP_EXCEEDED,
    TOOL_ERROR_INVALID_ARGUMENTS,
    TOOL_ERROR_K8S_ERROR,
    TOOL_ERROR_K8S_UNAVAILABLE,
    TOOL_ERROR_TIMEOUT,
    TOOL_ERROR_UNKNOWN_NAMESPACE,
    TOOL_ERROR_UNKNOWN_TOOL,
    TOOL_ERROR_UNKNOWN_WORKLOAD,
    TOOL_FIELD_DETAIL,
    TOOL_FIELD_ERROR,
    TOOL_GET_APP_OVERVIEW,
    TOOL_GET_CHANGE_HISTORY,
    TOOL_GET_RECENT_EVENTS,
    TOOL_GET_WORKLOAD_STATUS,
    TOOL_TIMEOUT_S,
    TOOL_TYPE_FUNCTION,
)
from models import EventsArgs, HistoryArgs, OverviewArgs, Run, ToolResult, WorkloadArgs
from tools import app_tools, k8s_tools
from tools.app_tools import app_namespaces
from tools.k8s_tools import K8s, K8sError

# name -> (args model, handler(run, args, k8s) -> (payload, list_key), refs(payload))
_TOOLS = {
    TOOL_GET_APP_OVERVIEW: (OverviewArgs, lambda run, args, k8s: app_tools.get_app_overview(run),
                            app_tools.overview_refs),
    TOOL_GET_CHANGE_HISTORY: (HistoryArgs, lambda run, args, k8s: app_tools.get_change_history(run, args),
                              app_tools.history_refs),
    TOOL_GET_WORKLOAD_STATUS: (WorkloadArgs, k8s_tools.get_workload_status, k8s_tools.status_refs),
    TOOL_GET_RECENT_EVENTS: (EventsArgs, k8s_tools.get_recent_events, k8s_tools.events_refs),
}
_K8S_TOOLS = (TOOL_GET_WORKLOAD_STATUS, TOOL_GET_RECENT_EVENTS)


def _schema(model: type[BaseModel]) -> dict:
    def strip(node):
        if isinstance(node, dict):
            return {k: strip(v) for k, v in node.items() if k != "title"}
        if isinstance(node, list):
            return [strip(v) for v in node]
        return node

    return strip(model.model_json_schema())


# One source for validation and schema: additionalProperties false comes from extra='forbid'.
SPECS = [
    {
        "type": TOOL_TYPE_FUNCTION,
        "function": {"name": name, "description": TOOL_DESCRIPTIONS[name], "parameters": _schema(model)},
    }
    for name, (model, _handler, _refs) in _TOOLS.items()
]


def fit(payload: dict, list_key: str) -> tuple[str, bool]:
    """Compact JSON within ANALYZER_TOOL_RESULT_MAX_BYTES, dropping whole trailing items of `list_key`."""
    truncated = False
    content = json.dumps(payload, separators=JSON_COMPACT_SEPARATORS)
    while len(content.encode()) > ANALYZER_TOOL_RESULT_MAX_BYTES and payload[list_key]:
        payload[list_key].pop()
        truncated = True
        content = json.dumps(payload, separators=JSON_COMPACT_SEPARATORS)
    return content, truncated


def _error(name: str, code: str, detail: str | int = "") -> ToolResult:
    content = json.dumps({TOOL_FIELD_ERROR: code, TOOL_FIELD_DETAIL: detail}, separators=JSON_COMPACT_SEPARATORS)
    return ToolResult(name=name, content=content, error=code)


def validation_detail(exc: ValidationError) -> str:
    # Error type and location only: the input itself is model output and never echoed.
    return "; ".join(f"{e['type']}:{'.'.join(map(str, e['loc']))}" for e in exc.errors())


def _binding_error(run: Run, args: BaseModel) -> str:
    if isinstance(args, WorkloadArgs):
        if run.bind(SUBJECT_TEMPLATE.format(kind=args.kind, name=args.name), args.namespace) is None:
            return TOOL_ERROR_UNKNOWN_WORKLOAD
    if isinstance(args, EventsArgs) and args.namespace and args.namespace not in app_namespaces(run):
        return TOOL_ERROR_UNKNOWN_NAMESPACE
    return ""


async def invoke(run: Run, name: str, arguments: dict | str, k8s: K8s | None) -> ToolResult:
    if name not in _TOOLS:
        return _error(name, TOOL_ERROR_UNKNOWN_TOOL)
    model, handler, refs_of = _TOOLS[name]
    try:
        args = model.model_validate_json(arguments) if isinstance(arguments, str) else model.model_validate(arguments)
    except ValidationError as e:
        return _error(name, TOOL_ERROR_INVALID_ARGUMENTS, validation_detail(e))

    if name != TOOL_GET_APP_OVERVIEW and run.calls[name] >= TOOL_CAPS[name]:
        return _error(name, TOOL_ERROR_CALL_CAP_EXCEEDED)
    if code := _binding_error(run, args):
        return _error(name, code)
    if name in _K8S_TOOLS and k8s is None:
        run.truncated = True
        return _error(name, TOOL_ERROR_K8S_UNAVAILABLE)

    run.calls[name] += 1
    run.tool_calls += 1
    try:
        async with asyncio.timeout(TOOL_TIMEOUT_S):
            out = handler(run, args, k8s)
            payload, list_key = await out if asyncio.iscoroutine(out) else out
    # httpx.TimeoutException is an httpx.HTTPError: it must be caught first.
    except (TimeoutError, httpx.TimeoutException):
        run.truncated = True
        return _error(name, TOOL_ERROR_TIMEOUT)
    except K8sError as e:
        # A 404 is a named workload that no longer exists (run.gone), not a read that fell short.
        if e.status != httpx.codes.NOT_FOUND:
            run.truncated = True
        return _error(name, TOOL_ERROR_K8S_ERROR, e.status)
    except httpx.HTTPError as e:
        run.truncated = True
        return _error(name, TOOL_ERROR_K8S_ERROR, type(e).__name__)

    content, truncated = fit(payload, list_key)
    refs = refs_of(payload)
    run.refs.update(refs)
    return ToolResult(name=name, content=content, truncated=truncated, refs=refs)
