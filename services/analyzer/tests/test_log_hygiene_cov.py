"""Guard: logging must never interpolate a caught exception object.

Provider/pydantic/JSON exception strings embed the raw model completion or the
raw job body (customer namespaces, workload names, images, labels), so only the
exception *type* may reach the logs.
"""

import ast
from pathlib import Path

_ROOT = Path(__file__).resolve().parent.parent

_GUARDED_MODULES = [
    "main.py",
    "helpers.py",
    "insights.py",
    "api_server.py",
    "exporter.py",
    "providers/ollama.py",
    "tools/__init__.py",
    "tools/app_tools.py",
    "tools/k8s_tools.py",
    "analyzer.py",
    "prompts/analyzer_prompt.py",
    "runtime.py",
    "events.py",
    "messages.py",
    "review.py",
    "recommendations.py",
]


def _is_logger_call(node: ast.Call) -> bool:
    func = node.func
    return (
        isinstance(func, ast.Attribute)
        and isinstance(func.value, ast.Name)
        and func.value.id == "logger"
    )


def _violations(path: Path) -> list[str]:
    tree = ast.parse(path.read_text())
    caught: set[str] = {
        handler.name for handler in ast.walk(tree)
        if isinstance(handler, ast.ExceptHandler) and handler.name
    }

    found = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call) or not _is_logger_call(node):
            continue
        where = f"{path.name}:{node.lineno}"
        if node.func.attr == "exception":
            found.append(f"{where}: logger.exception dumps a full traceback")
            continue
        for arg in node.args:
            if isinstance(arg, ast.Name) and arg.id in caught:
                found.append(f"{where}: logs caught exception '{arg.id}' — use type({arg.id}).__name__")
    return found


def test_no_bare_exception_in_logs():
    findings = []
    for module in _GUARDED_MODULES:
        findings.extend(_violations(_ROOT / module))

    assert not findings, "logging must not interpolate caught exceptions:\n" + "\n".join(findings)
