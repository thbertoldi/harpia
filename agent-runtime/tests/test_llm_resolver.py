from __future__ import annotations

import pytest
from harpia.llm_config.v1.llm_config_pb2 import CredentialBundle, ResolveLLMProviderForTenantResponse

from harpia_agents.llm.resolver import TenantLLMResolver
from harpia_agents.llm.secrets import RedactedSecret


class FakeResolverClient:
    def __init__(self) -> None:
        self.last_headers: dict[str, str] | None = None

    async def resolve_l_l_m_provider_for_tenant(self, request, *, headers=None, timeout_ms=None):  # noqa: ANN001, ANN202
        self.last_headers = dict(headers or {})
        return ResolveLLMProviderForTenantResponse(
            credentials=CredentialBundle(
                provider=request.provider,
                api_key="sk-ant-resolved-key",
                default_model="claude-3-5-sonnet",
                allowed_models=["claude-3-5-sonnet"],
            )
        )


@pytest.mark.asyncio
async def test_tenant_resolver_invokes_internal_rpc_with_tenant_headers() -> None:
    client = FakeResolverClient()
    resolver = TenantLLMResolver(client=client, auth_token="internal-token")

    resolved = await resolver.resolve(
        tenant_id="00000000-0000-4000-8000-000000000001",
        provider="anthropic",
    )

    assert client.last_headers is not None
    assert client.last_headers["Authorization"] == "Bearer internal-token"
    assert client.last_headers["X-Tenant-ID"] == "00000000-0000-4000-8000-000000000001"
    assert resolved.provider == "anthropic"
    assert str(resolved.api_key) == "***REDACTED***"
    assert resolved.api_key.reveal() == "sk-ant-resolved-key"


def test_redacted_secret_masks_string_rendering() -> None:
    secret = RedactedSecret("sk-test-never-log")
    assert str(secret) == "***REDACTED***"
    assert "sk-test-never-log" not in repr(secret)
    assert secret.reveal() == "sk-test-never-log"
