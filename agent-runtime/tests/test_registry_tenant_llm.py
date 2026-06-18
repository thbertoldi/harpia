import pytest
from unittest.mock import AsyncMock

from harpia_agents.agents.registry import _build_registry_with_tenant_credentials
from harpia_agents.llm.resolver import ResolvedProviderCredentials
from harpia_agents.llm.secrets import RedactedSecret


def test_build_registry_with_tenant_credentials_uses_openai_provider() -> None:
    registry = _build_registry_with_tenant_credentials(
        ResolvedProviderCredentials(
            provider="openai",
            api_key=RedactedSecret("sk-test"),
            default_model="gpt-4o-mini",
            allowed_models=("gpt-4o-mini",),
            source=1,
        )
    )

    provider = registry.resolve("openai-gpt-4o-mini")
    assert provider.name == "openai"
