from __future__ import annotations

"""Session and scope checks for enrichment-service endpoints.

The Go services share a middleware for this. This service is Python, so it
cannot use it, and reimplementing grant resolution here would put the rules
that decide what a role grants in two places. Instead it asks auth-service to
resolve the caller, which is the same resolution every other service performs.
"""

import hmac

import httpx
from fastapi import Header, HTTPException, status

from app_logger import logger
from config import AUTHZ_PERMISSIONS_URL, SERVICE_TOKEN
from constants import (
    AUTHZ_TIMEOUT_SECONDS,
    HEADER_SERVICE_TOKEN,
    HEADER_SESSION_TOKEN,
    LOG_AUTHZ_RESOLVE_FAILED,
    MSG_AUTHZ_FORBIDDEN,
    MSG_AUTHZ_MISSING_SESSION,
    MSG_AUTHZ_SERVICE_ONLY,
    MSG_AUTHZ_UNAVAILABLE,
    PERMISSION_RANKS,
    SCOPE_ALL,
)


def _covers(granted: str, required: str) -> bool:
    granted_rank = PERMISSION_RANKS.get(granted, 0)
    required_rank = PERMISSION_RANKS.get(required, 0)
    if granted_rank == 0 or required_rank == 0:
        return False
    return granted_rank >= required_rank


def _grants_scope(roles: list[dict], scope: str, min_level: str) -> bool:
    for role in roles:
        if role.get("isExpired") or role.get("status") != "Active":
            continue
        for entry in role.get("scopes") or []:
            # A rule denies one specific action, not the whole scope grant (see
            # x-ware/authz.Allows on the Go side). require_scope only ever checks
            # scope+level with no action to look up, so a rules list here can
            # never apply and must not disqualify the grant.
            if entry.get("scope") not in (scope, SCOPE_ALL):
                continue
            if _covers(entry.get("level", ""), min_level):
                return True
    return False


async def _resolve(session_token: str) -> list[dict]:
    async with httpx.AsyncClient(timeout=AUTHZ_TIMEOUT_SECONDS) as client:
        response = await client.get(
            AUTHZ_PERMISSIONS_URL,
            headers={HEADER_SESSION_TOKEN: session_token},
        )
    if response.status_code == status.HTTP_401_UNAUTHORIZED:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, MSG_AUTHZ_MISSING_SESSION)
    response.raise_for_status()
    return (response.json() or {}).get("data", {}).get("roles") or []


def require_service_token(
    token: str | None = Header(default=None, alias=HEADER_SERVICE_TOKEN),
) -> None:
    """Admit only a peer service, by the shared token. Used for routes another
    Telark service calls but no user should reach directly. Compared in constant
    time so a mismatch does not leak position by timing."""
    if not SERVICE_TOKEN or not token or not hmac.compare_digest(token, SERVICE_TOKEN):
        raise HTTPException(status.HTTP_403_FORBIDDEN, MSG_AUTHZ_SERVICE_ONLY)


def require_scope(scope: str, min_level: str):
    """Reject a request unless the caller's session grants scope at min_level."""

    async def dependency(
        session_token: str | None = Header(default=None, alias=HEADER_SESSION_TOKEN),
    ) -> None:
        if not session_token or not session_token.strip():
            raise HTTPException(status.HTTP_401_UNAUTHORIZED, MSG_AUTHZ_MISSING_SESSION)

        try:
            roles = await _resolve(session_token.strip())
        except HTTPException:
            raise
        except Exception as exc:  # noqa: BLE001 - any failure must deny, not allow
            logger.warning(LOG_AUTHZ_RESOLVE_FAILED.format(error=exc))
            raise HTTPException(
                status.HTTP_503_SERVICE_UNAVAILABLE, MSG_AUTHZ_UNAVAILABLE
            ) from exc

        if not _grants_scope(roles, scope, min_level):
            raise HTTPException(status.HTTP_403_FORBIDDEN, MSG_AUTHZ_FORBIDDEN)

    return dependency
