"""Checks for the scope rules analyzer-service enforces on its endpoints.

These mirror the Go middleware's behaviour. If the two ever disagree, the same
role grants different access depending on which service is asked, so the cases
below are deliberately the same ones covered on the Go side.

Run: python3 tests/test_authz.py
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from authz import _allows, _covers, _grants_scope  # noqa: E402
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
        # FIXED 2026-09-13: this case asserted the opposite of the Go model and was
        # the actual cause of a live bug (an Admin with any unrelated deny rule on
        # "settings" got 403 on the old key-validation route). In x-ware/authz.Allows, a rule
        # only denies the one action it names (via Requirement.Rule); a bare
        # scope+level check like this one passes no action, so no rule can ever
        # apply to it. See x-ware/authz/allow.go:isRuleDenied.
        ("unrelated deny rule does not block a bare scope+level check",
         [role(SCOPE_SETTINGS, PERMISSION_LEVEL_ADMIN, rules=["settings.editOIDCConfig"])], True),
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


def test_denyable_action():
    # The cases of x-ware authz.Allows (allow.go): a rule denies only the action it
    # names, on the route's scope or on ALL, from any active unexpired role, before
    # and independently of the level.
    deny = "settings.controlainsights.deny"
    owner = role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER)
    action = "controlainsights"
    cases = [
        ("rule on the scope denies an Owner", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, rules=[deny])], action, False),
        ("rule on ALL denies an Owner", [owner, role(SCOPE_ALL, PERMISSION_LEVEL_READONLY, rules=[deny])], action, False),
        ("a deny from any active role beats another role's level",
         [role(SCOPE_SETTINGS, PERMISSION_LEVEL_ADMIN), role(SCOPE_SETTINGS, PERMISSION_LEVEL_READONLY, rules=[deny])],
         action, False),
        ("the action is lower-cased like x-ware RuleKey",
         [role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, rules=[deny])], "ControlAInsights", False),
        ("rule for another action does not deny",
         [role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, rules=["settings.editoidcconfig.deny"])], action, True),
        ("rule on another scope does not deny", [owner, role("users", PERMISSION_LEVEL_OWNER, rules=[deny])], action, True),
        ("without an action the rules are ignored", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, rules=[deny])], None, True),
        ("an expired role's rule is ignored",
         [owner, role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, expired=True, rules=[deny])], action, True),
        ("an inactive role's rule is ignored",
         [owner, role(SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, status="Inactive", rules=[deny])], action, True),
        ("no deny and too low a level", [role(SCOPE_SETTINGS, PERMISSION_LEVEL_CONTRIBUTOR)], action, False),
    ]

    for name, roles, act, want in cases:
        got = _allows(roles, SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, act)
        assert got == want, f"{name}: got {got}, want {want}"


if __name__ == "__main__":
    test_grants_scope()
    test_covers_rejects_unknown_levels()
    test_denyable_action()
    print("authz checks passed")


def test_require_any_returns_user_id(monkeypatch):
    import asyncio

    import authz

    async def resolve(_token):
        return "user-42", [{"status": "Active", "scopes": [{"scope": "insights", "level": "Contributor"}]}]

    monkeypatch.setattr(authz, "_resolve", resolve)
    dep = authz.require_any(("insights", "Contributor", None))
    assert asyncio.run(dep(session_token="tok")) == "user-42"
