"""Tenant and identity helpers for agent-runtime entry points."""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping, MutableMapping
from dataclasses import dataclass
from http import HTTPStatus
from typing import Any

DEV_TENANT_ID = "dev"

Scope = MutableMapping[str, Any]
Receive = Callable[[], Awaitable[dict[str, Any]]]
Send = Callable[[dict[str, Any]], Awaitable[None]]
ASGIApp = Callable[[Scope, Receive, Send], Awaitable[None]]


@dataclass(frozen=True)
class RequestContext:
    tenant_id: str
    user_id: str
    roles: tuple[str, ...] = ()


class TenantAuthError(PermissionError):
    code: HTTPStatus

    def __init__(self, code: HTTPStatus, message: str) -> None:
        super().__init__(message)
        self.code = code


class TenantResolverMiddleware:
    """Resolve tenant/user metadata before ConnectRPC dispatch."""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return

        headers = _headers_from_scope(scope)
        try:
            scope["harpia.request_context"] = resolve_request_context(headers)
        except TenantAuthError as exc:
            await _send_plain_error(send, exc.code, str(exc))
            return

        await self.app(scope, receive, send)


def resolve_request_context(headers: Mapping[str, str]) -> RequestContext:
    auth = headers.get("authorization", "")
    cookie = headers.get("cookie", "")
    if not auth.startswith("Bearer ") and "harpia_session=" not in cookie:
        raise TenantAuthError(HTTPStatus.UNAUTHORIZED, "missing authorization")

    tenant_id = headers.get("x-tenant-id", "")
    if not tenant_id:
        raise TenantAuthError(HTTPStatus.UNAUTHORIZED, "missing tenant")

    user_id = "dev-user" if auth == "Bearer dev-token" else "authenticated-user"
    return RequestContext(tenant_id=tenant_id, user_id=user_id)


def require_tenant(ctx: object, requested_tenant_id: str) -> str:
    if not requested_tenant_id:
        raise TenantAuthError(HTTPStatus.UNAUTHORIZED, "missing tenant")

    context = _request_context_from_connect_context(ctx)
    if context is None:
        return requested_tenant_id

    if requested_tenant_id != context.tenant_id:
        raise TenantAuthError(
            HTTPStatus.FORBIDDEN,
            f"tenant {requested_tenant_id!r} is not available to the caller",
        )
    return context.tenant_id


def validate_temporal_input(input: Mapping[str, object]) -> str:
    tenant_id = input.get("tenant_id")
    if not isinstance(tenant_id, str) or not tenant_id:
        raise TenantAuthError(HTTPStatus.UNAUTHORIZED, "missing tenant")
    return tenant_id


def _headers_from_scope(scope: Mapping[str, Any]) -> dict[str, str]:
    raw_headers = scope.get("headers", [])
    return {
        key.decode("latin-1").lower(): value.decode("latin-1")
        for key, value in raw_headers
    }


def _request_context_from_connect_context(ctx: object) -> RequestContext | None:
    scope = getattr(ctx, "scope", None)
    if isinstance(scope, dict):
        found = scope.get("harpia.request_context")
        if isinstance(found, RequestContext):
            return found

    headers = getattr(ctx, "headers", None) or getattr(ctx, "request_headers", None)
    if isinstance(headers, Mapping):
        normalized = {str(k).lower(): str(v) for k, v in headers.items()}
        try:
            return resolve_request_context(normalized)
        except TenantAuthError:
            return None

    return None


async def _send_plain_error(send: Send, status: HTTPStatus, message: str) -> None:
    body = message.encode("utf-8")
    await send(
        {
            "type": "http.response.start",
            "status": int(status),
            "headers": [
                (b"content-type", b"text/plain; charset=utf-8"),
                (b"content-length", str(len(body)).encode("ascii")),
            ],
        }
    )
    await send({"type": "http.response.body", "body": body})
