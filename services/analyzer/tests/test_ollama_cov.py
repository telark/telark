"""Checks for the Ollama client: protocol flags on the chat body and the error map.

The flags pin the runtime behavior the loop relies on (no streaming, no
thinking, no silent truncation or context shift), and the error map decides
which runtime state and lastRun.error the UI shows.

Run: python -m pytest tests/test_ollama_cov.py
"""

import asyncio
import importlib
import json
import os
import socket
import sys

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import config  # noqa: E402
from models import ChatMessage, ToolCall  # noqa: E402
from providers import ollama  # noqa: E402

_MESSAGES = [ChatMessage(role="system", content="s"), ChatMessage(role="user", content="u")]
_TOOLS = [{"type": "function", "function": {"name": "get_app_overview"}}]


def _call(handler, fn):
    async def go():
        async with httpx.AsyncClient(transport=httpx.MockTransport(handler), base_url="http://ollama") as client:
            return await fn(client)

    return asyncio.run(go())


def _respond(status, body):
    return lambda request: httpx.Response(status, text=body)


def _raise(exc):
    def handler(request):
        raise exc

    return handler


def _chat(client):
    return ollama.chat(client, ollama.chat_payload("m", _MESSAGES, _TOOLS, 256), 10)


def test_chat_payload_flags():
    payload = ollama.chat_payload("qwen3:4b", _MESSAGES, _TOOLS, 256)
    assert payload["model"] == "qwen3:4b"
    assert payload["stream"] is False
    assert payload["think"] is False
    assert payload["truncate"] is False
    assert payload["shift"] is False
    assert payload["keep_alive"] == -1
    assert payload["options"] == {"num_ctx": 4096, "temperature": 0.2, "num_predict": 256, "num_thread": 2}
    assert payload["tools"] == _TOOLS
    assert payload["messages"] == [{"role": "system", "content": "s"}, {"role": "user", "content": "u"}]
    assert "format" not in payload
    assert ollama.chat_payload("m", [], [], 1, fmt={"type": "object"})["format"] == {"type": "object"}


def test_chat_payload_has_num_thread_and_ctx():
    # num_thread = the pod's CPU limit: Ollama's own guess oversubscribes a throttled container.
    options = ollama.chat_payload("m", _MESSAGES, _TOOLS, 64)["options"]
    assert (options["num_thread"], options["num_ctx"]) == (2, 4096)
    assert ollama.chat_payload("m", _MESSAGES, [], 64)["keep_alive"] == -1


def test_chat_payload_omits_empty_tools():
    # The fast narration must not pay for the tools array in its prompt.
    assert "tools" not in ollama.chat_payload("m", _MESSAGES, [], 64, fmt={"type": "object"})
    assert ollama.chat_payload("m", _MESSAGES, _TOOLS, 64)["tools"] == _TOOLS


def test_analyzer_mode_typo_fails_import(monkeypatch):
    assert (config.ANALYZER_MODE, config.ANALYZER_NUM_THREAD, config.ANALYZER_NARRATE_TIMEOUT_SEC) == ("fast", 2, 45)
    monkeypatch.setenv("ANALYZER_MODE", "turbo")
    with pytest.raises(ValueError):
        importlib.reload(config)
    monkeypatch.setenv("ANALYZER_MODE", "deep")
    assert importlib.reload(config).ANALYZER_MODE == "deep"
    monkeypatch.delenv("ANALYZER_MODE")
    importlib.reload(config)


def test_chat_message_omits_empty_fields():
    call = ToolCall(function={"name": "get_app_overview", "arguments": {"name": "a"}})
    dumped = ChatMessage(role="assistant", tool_calls=[call]).model_dump()
    assert dumped == {
        "role": "assistant",
        "content": "",
        "tool_calls": [{"function": {"name": "get_app_overview", "arguments": {"name": "a"}}}],
    }
    assert ChatMessage(role="tool", content="x", tool_name="t").model_dump()["tool_name"] == "t"


def test_chat_parses_response():
    seen = {}

    def handler(request):
        seen["path"] = request.url.path
        seen["body"] = json.loads(request.content)
        return httpx.Response(200, json={
            "model": "m",
            "message": {
                "role": "assistant",
                "content": "",
                "tool_calls": [{"function": {"name": "get_app_overview", "arguments": {"name": "a"}}}],
            },
            "done": True,
            "done_reason": "stop",
            "prompt_eval_count": 100,
            "eval_count": 7,
            "unknown": 1,
        })

    resp = _call(handler, _chat)
    assert seen["path"] == "/api/chat"
    assert seen["body"]["stream"] is False
    assert resp.done is True
    assert resp.done_reason == "stop"
    assert (resp.prompt_eval_count, resp.eval_count) == (100, 7)
    assert resp.message.tool_calls[0].function.name == "get_app_overview"
    assert resp.message.tool_calls[0].function.arguments == {"name": "a"}


@pytest.mark.parametrize(
    ("status", "body", "expected"),
    [
        (503, "server busy", ollama.OllamaBusy),
        (400, '{"error":"input length exceeds the context length"}', ollama.ContextOverflow),
        (500, '{"error":"context too long"}', ollama.ContextOverflow),
        (400, '{"error":"registry.ollama.ai/library/x does not support tools"}', ollama.ModelUnsupported),
        (404, '{"error":"model not found"}', ollama.ModelMissing),
        (500, '{"error":"boom"}', ollama.OllamaError),
    ],
)
def test_chat_maps_errors(status, body, expected):
    with pytest.raises(expected):
        _call(_respond(status, body), _chat)


def test_non_json_body_is_an_ollama_error():
    with pytest.raises(ollama.OllamaError, match="JSONDecodeError"):
        _call(_respond(200, "<html>login</html>"), ollama.tags)


def test_chat_maps_timeout():
    with pytest.raises(ollama.OllamaTimeout):
        _call(_raise(httpx.ReadTimeout("slow")), _chat)


def test_show_missing_model():
    with pytest.raises(ollama.ModelMissing):
        _call(_respond(404, '{"error":"model not found"}'), lambda c: ollama.show(c, "m"))


def test_show_and_tags():
    def handler(request):
        if request.url.path == "/api/show":
            assert json.loads(request.content) == {"model": "m"}
            return httpx.Response(200, json={"capabilities": ["completion", "tools"], "license": "x"})
        assert request.url.path == "/api/tags"
        return httpx.Response(200, json={"models": [{"name": "qwen3:4b"}, {"name": "llama3.2:3b"}]})

    assert _call(handler, lambda c: ollama.show(c, "m")) == ["completion", "tools"]
    assert _call(handler, ollama.tags) == ["qwen3:4b", "llama3.2:3b"]
    assert _call(lambda r: httpx.Response(200, json={}), ollama.tags) == []


def _connect_error(cause):
    try:
        try:
            raise cause
        except OSError as e:
            raise httpx.ConnectError("connect failed") from e
    except httpx.ConnectError as err:
        return err


def test_dns_failure_is_absent():
    dns = _connect_error(socket.gaierror(socket.EAI_NONAME, "nodename nor servname provided"))
    with pytest.raises(ollama.OllamaAbsent):
        _call(_raise(dns), ollama.tags)
    refused = _connect_error(ConnectionRefusedError(111, "refused"))
    with pytest.raises(ollama.OllamaUnreachable) as info:
        _call(_raise(refused), ollama.tags)
    assert not isinstance(info.value, ollama.OllamaAbsent)
    with pytest.raises(ollama.OllamaUnreachable):
        _call(_raise(httpx.RemoteProtocolError("reset")), lambda c: ollama.show(c, "m"))


def _ndjson(*lines):
    return "\n".join(json.dumps(line) for line in lines) + "\n"


def test_pull_streams_progress():
    body = _ndjson(
        {"status": "pulling manifest"},
        {"status": "pulling abc", "digest": "sha256:abc", "total": 100, "completed": 10},
        {"status": "pulling abc", "digest": "sha256:abc", "total": 100, "completed": 100},
        {"status": "success"},
    )
    seen = {}

    def handler(request):
        seen["path"] = request.url.path
        seen["body"] = json.loads(request.content)
        return httpx.Response(200, text=body)

    progress = []
    _call(handler, lambda c: ollama.pull(c, "qwen3:4b", progress.append))
    assert seen == {"path": "/api/pull", "body": {"model": "qwen3:4b", "stream": True}}
    assert len(progress) == 4
    assert (progress[0].status, progress[0].completed, progress[0].total) == ("pulling manifest", 0, 0)
    assert (progress[1].completed, progress[1].total) == (10, 100)
    assert progress[-1].status == "success"
    assert progress[0].model == "qwen3:4b"


def test_pull_error_line_raises():
    body = _ndjson({"status": "pulling manifest"}, {"error": "pull model manifest: file does not exist"})
    with pytest.raises(ollama.OllamaError):
        _call(_respond(200, body), lambda c: ollama.pull(c, "nope", lambda p: None))


def test_pull_without_success_raises():
    with pytest.raises(ollama.OllamaError):
        _call(_respond(200, _ndjson({"status": "pulling manifest"})), lambda c: ollama.pull(c, "m", lambda p: None))


def test_pull_maps_http_and_transport_errors():
    with pytest.raises(ollama.OllamaBusy):
        _call(_respond(503, "busy"), lambda c: ollama.pull(c, "m", lambda p: None))
    with pytest.raises(ollama.OllamaUnreachable):
        _call(_raise(_connect_error(ConnectionRefusedError(111, "refused"))), lambda c: ollama.pull(c, "m", lambda p: None))


def test_delete_tolerates_404_and_maps_errors():
    _call(_respond(404, '{"error": "model not found"}'), lambda c: ollama.delete(c, "m"))
    with pytest.raises(ollama.OllamaError):
        _call(_respond(500, "disk error"), lambda c: ollama.delete(c, "m"))
    with pytest.raises(ollama.OllamaUnreachable):
        _call(_raise(_connect_error(ConnectionRefusedError(111, "refused"))), lambda c: ollama.delete(c, "m"))
