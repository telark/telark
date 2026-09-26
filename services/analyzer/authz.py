"""Session and scope checks for analyzer-service endpoints.

The Go services share a middleware for this. This service is Python, so it
cannot use it, and reimplementing grant resolution here would put the rules
that decide what a role grants in two places. Instead it asks auth-service to
resolve the caller, which is the same resolution every other service performs.
"""

from __future__ import annotations

import httpx
from fastapi import Header, HTTPException, status

from app_logger import logger
from config import AUTHZ_PERMISSIONS_URL
from constants import (
    AUTHZ_TIMEOUT_SECONDS,
    HEADER_SESSION_TOKEN,
    LOG_AUTHZ_RESOLVE_FAILED,
    MSG_AUTHZ_FORBIDDEN,
    MSG_AUTHZ_INVALID_SESSION,
    MSG_AUTHZ_MISSING_SESSION,
    MSG_AUTHZ_UNAVAILABLE,
    PERMISSION_RANKS,
    PERMISSIONS_FIELD_ROLES,
    PERMISSIONS_FIELD_USER_ID,
    ROLE_STATUS_ACTIVE,
    RULE_KEY_TEMPLATE,
    SCOPE_ALL,
)


def _covers(granted: str, required: str) -> bool:
    granted_rank = PERMISSION_RANKS.get(granted, 0)
    required_rank = PERMISSION_RANKS.get(required, 0)
    if granted_rank == 0 or required_rank == 0:
        return False
    return granted_rank >= required_rank


def _active_entries(roles: list[dict], scope: str):
    """Scope entries for scope or ALL of every active, unexpired role (x-ware RoleGrantsAccess)."""
    for role in roles:
        if role.get("isExpired") or role.get("status") != ROLE_STATUS_ACTIVE:
            continue
        for entry in role.get("scopes") or []:
            if entry.get("scope") in (scope, SCOPE_ALL):
                yield entry


def _grants_scope(roles: list[dict], scope: str, min_level: str) -> bool:
    # A rule denies one action, not the scope grant (x-ware authz.Allows): rules are checked
    # only by _allows for denyable routes and never disqualify the level grant.
    return any(_covers(entry.get("level", ""), min_level) for entry in _active_entries(roles, scope))


def _allows(roles: list[dict], scope: str, min_level: str, action: str | None = None) -> bool:
    """x-ware authz.Allows: the action's deny rule is checked first and independently of the level."""
    if action:
        rule = RULE_KEY_TEMPLATE.format(scope=scope, action=action).lower()
        if any(rule in (entry.get("rules") or []) for entry in _active_entries(roles, scope)):
            return False
    return _grants_scope(roles, scope, min_level)


async def _resolve(session_token: str) -> tuple[str, list[dict]]:
    """(userID, roles) of the session."""
    async with httpx.AsyncClient(timeout=AUTHZ_TIMEOUT_SECONDS) as client:
        response = await client.get(
            AUTHZ_PERMISSIONS_URL,
            headers={HEADER_SESSION_TOKEN: session_token},
        )
    if response.status_code == status.HTTP_401_UNAUTHORIZED:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, MSG_AUTHZ_INVALID_SESSION)
    # auth-service answers 403 for a suspended or deleted user: a verdict, not an outage.
    if response.status_code == status.HTTP_403_FORBIDDEN:
        raise HTTPException(status.HTTP_403_FORBIDDEN, MSG_AUTHZ_FORBIDDEN)
    response.raise_for_status()
    # auth-service answers this route with the bare grants object, not the response envelope.
    body = response.json() or {}
    return body.get(PERMISSIONS_FIELD_USER_ID) or "", body.get(PERMISSIONS_FIELD_ROLES) or []


def require_any(*requirements: tuple[str, str, str | None]):
    """Reject a request unless the caller's session meets one of the (scope, min_level, action) requirements:
    grants scope at min_level and no role denies action.

    The dependency's value is the caller's userID (the triage route records it)."""

    async def dependency(
        session_token: str | None = Header(default=None, alias=HEADER_SESSION_TOKEN),
    ) -> str:
        if not session_token or not session_token.strip():
            raise HTTPException(status.HTTP_401_UNAUTHORIZED, MSG_AUTHZ_MISSING_SESSION)

        try:
            user_id, roles = await _resolve(session_token.strip())
        except HTTPException:
            raise
        except Exception as exc:  # noqa: BLE001 - any failure must deny, not allow
            logger.warning(LOG_AUTHZ_RESOLVE_FAILED.format(error=exc))
            raise HTTPException(
                status.HTTP_503_SERVICE_UNAVAILABLE, MSG_AUTHZ_UNAVAILABLE
            ) from exc

        if not any(_allows(roles, *requirement) for requirement in requirements):
            raise HTTPException(status.HTTP_403_FORBIDDEN, MSG_AUTHZ_FORBIDDEN)
        return user_id

    return dependency
