"""Coverage for helpers.py — key builders match the Go DocumentKey format, time helpers.

Run: pytest tests/test_helpers_cov.py
"""

import asyncio
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import helpers as H  # noqa: E402
from fakes import FakeRedis  # noqa: E402


def test_keys_match_go_format():
    # insights.DocumentKey: fmt.Sprintf("%s%s:%s", DocumentKeyPrefix, namespace, name)
    assert H.document_key("shop", "api") == "analyzer:shop:api"
    assert H.inflight_key("shop", "api") == "analyzer:inflight:shop:api"
    assert H.cooldown_manual_key("shop", "api") == "analyzer:cooldown:manual:shop:api"
    assert H.cooldown_auto_key("shop", "api") == "analyzer:cooldown:auto:shop:api"


def test_now_rfc3339():
    assert re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", H.now_rfc3339())


def test_stream_id_ms():
    assert H.stream_id_ms("1727000000123-4") == 1727000000123
    fake = FakeRedis()
    fake.clock_ms = 42
    assert H.stream_id_ms(asyncio.run(fake.xadd("s", {"a": "1"}))) == 42


# ---- quantities, selectors, images, probes, production (S6) ----------------------------------
import pytest  # noqa: E402

CPU_TABLE = [("250m", 250), ("1", 1000), ("0.5", 500), ("1e3m", 1000), ("2.00", 2000), ("12m", 12),
             ("100n", 1), ("1500u", 2), ("0.25", 250), (" 3 ", 3000), ("1Mi", None), ("", None), (None, None),
             ("abc", None), (42, None)]
MEM_TABLE = [("64Mi", 64 * 2**20), ("1Gi", 2**30), ("128974848", 128974848), ("129e6", 129 * 10**6),
             ("1k", 1000), ("1M", 10**6), ("45.23Mi", 47427093), ("512.00B", 512), ("1.50Gi", 1610612736),
             ("3Ki", 3072), ("2Ti", 2 * 2**40), ("x", None), ("1Zi", None), (None, None)]


@pytest.mark.parametrize("q,expected", CPU_TABLE)
def test_parse_cpu_table(q, expected):
    assert H.parse_cpu_milli(q) == expected


@pytest.mark.parametrize("q,expected", MEM_TABLE)
def test_parse_mem_table(q, expected):
    assert H.parse_mem_bytes(q) == expected


def test_format_roundtrip():
    assert [H.format_cpu(m) for m in (150, 2000, 10, 1500)] == ["150m", "2", "10m", "1500m"]
    assert [H.format_mem(b) for b in (64 * 2**20, int(1.5 * 2**30), 100, 2048, 70 * 2**20 + 2**19)] == [
        "64Mi", "1.5Gi", "100B", "2Ki", "70.5Mi"]
    for text in ("64Mi", "1.5Gi", "2Ki"):
        assert H.format_mem(H.parse_mem_bytes(text)) == text
    for text in ("150m", "2"):
        assert H.format_cpu(H.parse_cpu_milli(text)) == text
    assert H.round_up(101, 10) == 110 and H.round_up(100, 10) == 100 and H.round_up(0.2, 16) == 16


SELECTOR_TABLE = [
    ({}, {"a": "1"}, True),
    ({}, {}, True),
    (None, {"a": "1"}, False),
    ("bogus", {"a": "1"}, False),
    ({"matchLabels": {"a": "1"}}, {"a": "1", "b": "2"}, True),
    ({"matchLabels": {"a": "1"}}, {"a": "2"}, False),
    ({"matchLabels": {"a": "1"}}, None, False),
    ({"matchExpressions": [{"key": "a", "operator": "In", "values": ["1", "2"]}]}, {"a": "2"}, True),
    ({"matchExpressions": [{"key": "a", "operator": "In", "values": ["1"]}]}, {}, False),
    ({"matchExpressions": [{"key": "a", "operator": "NotIn", "values": ["1"]}]}, {"a": "2"}, True),
    ({"matchExpressions": [{"key": "a", "operator": "NotIn", "values": ["1"]}]}, {}, True),
    ({"matchExpressions": [{"key": "a", "operator": "NotIn", "values": ["1"]}]}, {"a": "1"}, False),
    ({"matchExpressions": [{"key": "a", "operator": "Exists"}]}, {"a": ""}, True),
    ({"matchExpressions": [{"key": "a", "operator": "Exists"}]}, {"b": "1"}, False),
    ({"matchExpressions": [{"key": "a", "operator": "DoesNotExist"}]}, {"b": "1"}, True),
    ({"matchExpressions": [{"key": "a", "operator": "DoesNotExist"}]}, {"a": "1"}, False),
    ({"matchExpressions": [{"key": "a", "operator": "Bogus"}]}, {"a": "1"}, False),
]


@pytest.mark.parametrize("selector,labels,expected", SELECTOR_TABLE)
def test_selector_matches_table(selector, labels, expected):
    assert H.selector_matches(selector, labels) is expected


IMAGE_TABLE = [
    ("nginx", ("docker.io", "library/nginx", "", "")),
    ("nginx:1.27", ("docker.io", "library/nginx", "1.27", "")),
    ("telarklabnope/nope:1", ("docker.io", "telarklabnope/nope", "1", "")),
    ("registry.k8s.io/pause:3.10", ("registry.k8s.io", "pause", "3.10", "")),
    ("localhost:5000/team/app:1", ("localhost:5000", "team/app", "1", "")),
    ("localhost/app", ("localhost", "app", "", "")),
    ("my.reg:5000/app", ("my.reg:5000", "app", "", "")),
    ("ghcr.io/o/r@sha256:" + "a" * 64, ("ghcr.io", "o/r", "", "sha256:" + "a" * 64)),
    ("nginx:1.27@sha256:" + "b" * 64, ("docker.io", "library/nginx", "1.27", "sha256:" + "b" * 64)),
    ("registry.k8s.io/Pause:3.10", None),
    ("a b", None),
    ("", None),
    (None, None),
]


@pytest.mark.parametrize("image,expected", IMAGE_TABLE)
def test_image_parts_table(image, expected):
    assert H.image_parts(image) == expected


def test_probe_signature_equal_and_different():
    http = {"httpGet": {"path": "/", "port": "http"}, "periodSeconds": 5, "failureThreshold": 3}
    assert H.probe_signature(http) == H.probe_signature(dict(http))
    assert H.probe_signature(http) == H.probe_signature({"httpGet": {"path": "/", "port": "http"},
                                                         "periodSeconds": 15, "failureThreshold": 1})
    assert H.probe_signature(http) != H.probe_signature({**http, "httpGet": {"path": "/healthz", "port": "http"}})
    assert H.probe_signature(http) != H.probe_signature({**http, "failureThreshold": 6})
    tcp = {"tcpSocket": {"port": 8080}}
    assert H.probe_signature(tcp) == ("tcpSocket", ("8080", "null"), 30)
    assert H.probe_signature(tcp) != H.probe_signature({"grpc": {"port": 8080}})
    assert H.probe_signature({"exec": {"command": ["true"]}}) != H.probe_signature({"exec": {"command": ["false"]}})
    assert H.probe_signature(None) is None and H.probe_signature({"periodSeconds": 1}) is None


def test_is_production():
    import re
    pattern = re.compile(r"(^|[-_.])(prod|production|prd)($|[-_.])", re.IGNORECASE)
    assert H.is_production(["shop-prod"], [], pattern)
    assert H.is_production(["shop"], ["Production"], pattern)
    assert H.is_production(["PRD"], [], pattern)
    assert not H.is_production(["shop", "products", "reproduce"], ["Staging"], pattern)
    assert not H.is_production([], [], pattern)
