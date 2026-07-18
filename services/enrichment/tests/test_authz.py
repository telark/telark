"""Checks for the scope rules enrichment-service enforces on its endpoints.

These mirror the Go middleware's behaviour. If the two ever disagree, the same
role grants different access depending on which service is asked, so the cases
below are deliberately the same ones covered on the Go side.

Run: python3 tests/test_authz.py
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from authz import _covers, _grants_scope  # noqa: E402
from constants import (  # noqa: E402
    PERMISSION_LEVEL_ADMIN,
    PERMISSION_LEVEL_CONTRIBUTOR,
    PERMISSION_LEVEL_OWNER,
    PERMISSION_LEVEL_READONLY,
    SCOPE_ALL,
    SCOPE_SETTINGS,
)


def role(scope, level, status="Active", expired=False, rules=None):
    entry = {"scope": scope, "level": level}
    if rules:
        entry["rules"] = rules
    return {"status": status, "isExpired": expired, "scopes": [entry]}


def test_grants_scope():
    cases = [
        ("exact level matches", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_CONTRIBUTOR)], True),
        ("higher level covers", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER)], True),
        ("lower level denied", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_READONLY)], False),
        ("admin via wildcard", [role(SCOPE_ALL, PERMISSION_LEVEL_ADMIN)], True),
        ("unrelated scope denied", [role("users", PERMISSION_LEVEL_ADMIN)], False),
        ("expired role grants nothing", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_ADMIN, expired=True)], False),
        ("inactive role grants nothing", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_ADMIN, status="Inactive")], False),
        ("deny rule beats level", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_ADMIN, rules=["any"])], False),
        ("no roles at all", [], False),
        ("unknown level never grants", [role(SCOPE_SETTINGS, "Superuser")], False),
        (
            "strongest of several roles wins",
            [role(SCOPE_SETTINGS, PERMISSION_LEVEL_READONLY), role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER)],
            True,
        ),
    ]

    for name, roles, want in cases:
        got = _grants_scope(roles, SCOPE_SETTINGS, PERMISSION_LEVEL_CONTRIBUTOR)
        assert got == want, f"{name}: got {got}, want {want}"


def test_covers_rejects_unknown_levels():
    assert _covers(PERMISSION_LEVEL_ADMIN, PERMISSION_LEVEL_READONLY)
    assert not _covers(PERMISSION_LEVEL_READONLY, PERMISSION_LEVEL_ADMIN)
    assert not _covers("Superuser", PERMISSION_LEVEL_READONLY)
    assert not _covers(PERMISSION_LEVEL_ADMIN, "Bogus")
    assert not _covers("", "")


if __name__ == "__main__":
    test_grants_scope()
    test_covers_rejects_unknown_levels()
    print("authz checks passed")
