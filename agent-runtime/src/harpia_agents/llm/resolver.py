"""Thin client for control-plane tenant LLM provider resolution."""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Protocol

from connectrpc.errors import ConnectError
from harpia.llm_config.v1.llm_config_connect import LLMConfigServiceClient
from harpia.llm_config.v1.llm_config_pb2 import (
    ResolveLLMProviderForTenantRequest,
)

from harpia_agents.llm.secrets import RedactedSecret


def _resolver_base_url() -> str:
    explicit = os.environ.get("HARPIA_CONTROL_PLANE_INTERNAL_URL", "").strip()
    if explicit:
        return explicit.rstrip("/")
    public = os.environ.get("HARPIA_CONTROL_PLANE_URL", "http://localhost:8080").rstrip("/")
    return f"{public}/internal"


def _default_internal_auth_token() -> str:
    configured = os.environ.get("HARPIA_INTERNAL_AUTH_TOKEN", "").strip()
    if configured:
        return configured
    if os.environ.get("HARPIA_ALLOW_DEV_AUTH", "").lower() in {"1", "true", "yes"}:
        return "dev-internal-token"
    return ""


@dataclass(frozen=True, slots=True)
class ResolvedProviderCredentials:
    provider: str
    api_key: RedactedSecret
    default_model: str
    allowed_models: tuple[str, ...]
    source: int


class _ResolverClient(Protocol):
    async def resolve_l_l_m_provider_for_tenant(
        self,
        request: ResolveLLMProviderForTenantRequest,
        *,
        headers: dict[str, str] | None = None,
        timeout_ms: int | None = None,
    ): ...


class TenantLLMResolver:
    """Client wrapper around ResolveLLMProviderForTenant."""

    def __init__(
        self,
        client: _ResolverClient | None = None,
        *,
        base_url: str | None = None,
        auth_token: str | None = None,
        timeout_ms: int = 5000,
    ) -> None:
        self._client = client or LLMConfigServiceClient(
            base_url or _resolver_base_url()
        )
        self._auth_token = auth_token if auth_token is not None else _default_internal_auth_token()
        self._timeout_ms = timeout_ms

    async def resolve(
        self,
        *,
        tenant_id: str,
        provider: str,
        requested_model: str = "",
    ) -> ResolvedProviderCredentials:
        if not self._auth_token:
            raise ConnectError("HARPIA_INTERNAL_AUTH_TOKEN is required outside dev mode")
        headers = {
            "Authorization": f"Bearer {self._auth_token}",
            "X-Tenant-ID": tenant_id,
        }
        response = await self._client.resolve_l_l_m_provider_for_tenant(
            ResolveLLMProviderForTenantRequest(
                tenant_id=tenant_id,
                provider=provider,
                requested_model=requested_model,
            ),
            headers=headers,
            timeout_ms=self._timeout_ms,
        )
        if not response.HasField("credentials"):
            raise ConnectError("resolver response missing credentials")
        credentials = response.credentials
        return ResolvedProviderCredentials(
            provider=credentials.provider,
            api_key=RedactedSecret(credentials.api_key),
            default_model=credentials.default_model,
            allowed_models=tuple(credentials.allowed_models),
            source=credentials.source,
        )
