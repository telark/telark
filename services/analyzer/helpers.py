"""Redis key builders (same format as the Go insights.DocumentKey), time helpers, the primary namespace and the
pure Kubernetes helpers of the review: quantities, label selectors, image references, probes, production."""

import json
import math
import re
import time
from datetime import UTC, datetime

from constants import (
    APP_REF_TEMPLATE,
    MS_PER_S,
    CPU_MILLI_PER_CORE,
    CPU_MILLI_SUFFIXES,
    DEFAULT_REGISTRY,
    DEFAULT_REPOSITORY_PREFIX,
    DIGEST_SEPARATOR,
    DOMAIN_MARKERS,
    IMAGE_REFERENCE_PATTERN,
    LOCALHOST,
    MEM_BYTE_SUFFIXES,
    MEM_FORMAT_UNITS,
    PROBE_DEFAULT_FAILURE_THRESHOLD,
    PROBE_DEFAULT_PERIOD_S,
    PROBE_HANDLER_FIELDS,
    QUANTITY_EPSILON,
    QUANTITY_PATTERN,
    SELECTOR_OPERATORS,
    TAG_SEPARATOR,
    COOLDOWN_AUTO_KEY_PREFIX,
    COOLDOWN_MANUAL_KEY_PREFIX,
    DOCUMENT_KEY_PREFIX,
    INFLIGHT_KEY_PREFIX,
    RFC3339_FORMAT,
    STREAM_ID_SEPARATOR,
)


def document_key(namespace: str, name: str) -> str:
    return DOCUMENT_KEY_PREFIX + f"{namespace}:{name}"


def inflight_key(namespace: str, name: str) -> str:
    return INFLIGHT_KEY_PREFIX + f"{namespace}:{name}"


def cooldown_manual_key(namespace: str, name: str) -> str:
    return COOLDOWN_MANUAL_KEY_PREFIX + f"{namespace}:{name}"


def cooldown_auto_key(namespace: str, name: str) -> str:
    return COOLDOWN_AUTO_KEY_PREFIX + f"{namespace}:{name}"


def now_rfc3339() -> str:
    return datetime.now(UTC).strftime(RFC3339_FORMAT)


def epoch_ms() -> int:
    return int(time.time() * MS_PER_S)


def app_ref(namespace: str, name: str) -> str:
    """'<namespace>/<name>': the SSE subscription key and the analyzer:index member of an app."""
    return APP_REF_TEMPLATE.format(namespace=namespace, name=name)


def primary_namespace(app: dict) -> str:
    """The namespace that keys the app's document: namespaces.items[0] of the exporter's Application."""
    items = (app.get("namespaces") or {}).get("items") or []
    return items[0].get("name") or "" if items else ""


def stream_id_ms(msg_id: str) -> int:
    """Milliseconds part of a Redis stream id ('<ms>-<seq>')."""
    return int(msg_id.split(STREAM_ID_SEPARATOR, 1)[0])


# ---- Quantities (Kubernetes quantities and the kcore usage strings: '%dm' / '%.2f' cores, '%.2f' + B|Ki|Mi|Gi) ----
def _quantity(q: str | None, suffixes: dict[str, float]) -> float | None:
    if not isinstance(q, str) or not (m := re.fullmatch(QUANTITY_PATTERN, q.strip())):
        return None
    number, suffix = m.groups()
    if suffix not in suffixes:
        return None
    try:
        return float(number) * suffixes[suffix]
    except ValueError:
        return None


def parse_cpu_milli(q: str | None) -> int | None:
    value = _quantity(q, CPU_MILLI_SUFFIXES)
    return None if value is None else int(math.ceil(value - QUANTITY_EPSILON))


def parse_mem_bytes(q: str | None) -> int | None:
    value = _quantity(q, MEM_BYTE_SUFFIXES)
    return None if value is None else int(math.ceil(value - QUANTITY_EPSILON))


def format_cpu(milli: int) -> str:
    return f"{milli // CPU_MILLI_PER_CORE}" if milli % CPU_MILLI_PER_CORE == 0 else f"{milli}m"


def format_mem(b: int) -> str:
    unit, size = next(((u, s) for u, s in MEM_FORMAT_UNITS if b >= s), MEM_FORMAT_UNITS[-1])
    return f"{round(b / size, 1):g}{unit}"


def round_up(value: float, step: int) -> int:
    return int(math.ceil(value / step) * step)


# ---- Label selectors (metav1.LabelSelector) --------------------------------------------------------------------
def selector_matches(selector: dict | None, labels: dict | None) -> bool:
    """matchLabels + matchExpressions; {} selects everything, None selects nothing; malformed selects nothing."""
    if selector is None or not isinstance(selector, dict):
        return False
    labels = labels or {}
    if any(labels.get(k) != v for k, v in (selector.get("matchLabels") or {}).items()):
        return False
    for expr in selector.get("matchExpressions") or []:
        key, op, values = expr.get("key"), expr.get("operator"), expr.get("values") or []
        test = SELECTOR_OPERATORS.get(op)
        if test is None or not test(key in labels, labels.get(key), values):
            return False
    return True


# ---- Image references ----------------------------------------------------------------------------------------
def image_parts(image: str | None) -> tuple[str, str, str, str] | None:
    """(registry, repository, tag, digest) by the Docker reference rules; None for an invalid reference."""
    if not image or not re.fullmatch(IMAGE_REFERENCE_PATTERN, image):
        return None
    name, _, digest = image.partition(DIGEST_SEPARATOR)
    first, sep, rest = name.partition("/")
    if sep and (DOMAIN_MARKERS[0] in first or DOMAIN_MARKERS[1] in first or first == LOCALHOST):
        registry, path = first, rest
    else:
        registry, path = DEFAULT_REGISTRY, name
    repository, _, tag = path.rpartition(TAG_SEPARATOR) if TAG_SEPARATOR in path.rsplit("/", 1)[-1] else (path, "", "")
    if registry == DEFAULT_REGISTRY and "/" not in repository:
        repository = DEFAULT_REPOSITORY_PREFIX + repository
    return registry, repository, tag, digest


# ---- Probes --------------------------------------------------------------------------------------------------
def probe_signature(probe: dict | None) -> tuple | None:
    """(handler kind, target, failure window s): equal signatures run the same check with the same patience."""
    if not isinstance(probe, dict):
        return None
    window = probe.get("failureThreshold", PROBE_DEFAULT_FAILURE_THRESHOLD) * probe.get(
        "periodSeconds", PROBE_DEFAULT_PERIOD_S)
    for handler, fields in PROBE_HANDLER_FIELDS.items():
        if isinstance(spec := probe.get(handler), dict):
            target = tuple(json.dumps(spec.get(f), sort_keys=True) for f in fields)
            return handler, target, window
    return None


# ---- Production ----------------------------------------------------------------------------------------------
def is_production(namespaces: list[str], env_names: list[str], pattern: re.Pattern) -> bool:
    return any(pattern.search(n or "") for n in [*namespaces, *env_names])
