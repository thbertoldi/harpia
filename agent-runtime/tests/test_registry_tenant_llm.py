from __future__ import annotations

from unittest.mock import AsyncMock

import pytest
from harpia.artifacts.v1.artifacts_pb2 import NewsArticle, NewsList

from harpia_agents.agents.newsletter_writer import MANIFEST
from harpia_agents.agents.registry import (
    _build_registry_with_tenant_credentials,
    run_registered_agent,
)
from harpia_agents.llm import LLMRegistry
from harpia_agents.llm.errors import ModelNotFoundError
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


def test_build_registry_with_tenant_credentials_uses_deepseek_provider() -> None:
    registry = _build_registry_with_tenant_credentials(
        ResolvedProviderCredentials(
            provider="deepseek",
            api_key=RedactedSecret("ds-test"),
            default_model="deepseek-v4-flash",
            allowed_models=("deepseek-v4-flash",),
            source=1,
        )
    )

    provider = registry.resolve("deepseek-v4-flash")
    assert provider.name == "deepseek"


@pytest.mark.asyncio
async def test_run_registered_agent_rejects_requested_model_from_other_provider(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class DeepSeekResolver:
        async def resolve(
            self,
            *,
            tenant_id: str,
            provider: str,
            requested_model: str = "",
        ) -> ResolvedProviderCredentials:
            return ResolvedProviderCredentials(
                provider="deepseek",
                api_key=RedactedSecret("ds-test"),
                default_model="deepseek-v4-flash",
                allowed_models=("deepseek-v4-flash",),
                source=1,
            )

    monkeypatch.setattr(
        "harpia_agents.agents.registry.TenantLLMResolver",
        lambda: DeepSeekResolver(),
    )
    monkeypatch.setattr(
        "harpia_agents.agents.registry._build_registry_with_tenant_credentials",
        lambda resolved: LLMRegistry.for_testing(
            model_ids=["deepseek-v4-flash", "openai-gpt-4o-mini"],
            responses=["## Draft\nGenerated content"],
        ),
    )

    with pytest.raises(ModelNotFoundError, match="deepseek"):
        await run_registered_agent(
            "newsletter-writer-senior",
            tenant_id="00000000-0000-4000-8000-000000000001",
            input_payload=NewsList(
                articles=[
                    NewsArticle(
                        title="Story",
                        url="https://example.com/story",
                        summary="Summary",
                        source="Example",
                        published_at="2026-06-17T00:00:00Z",
                    )
                ]
            ),
            elicitation_responses={
                "tone": "neutral",
                "requested_model": "openai-gpt-4o-mini",
            },
        )


@pytest.mark.asyncio
async def test_run_registered_agent_fails_closed_when_resolver_errors(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("HARPIA_ALLOW_DEV_AUTH", raising=False)
    monkeypatch.setattr(
        "harpia_agents.agents.registry.TenantLLMResolver",
        lambda: type(
            "BrokenResolver",
            (),
            {"resolve": AsyncMock(side_effect=RuntimeError("resolver unavailable"))},
        )(),
    )

    registry = LLMRegistry.for_testing(
        model_ids=[MANIFEST.model_id],
        responses=["## Draft\nGenerated content"],
    )

    with pytest.raises(RuntimeError, match="resolver unavailable"):
        await run_registered_agent(
            "newsletter-writer-senior",
            tenant_id="00000000-0000-4000-8000-000000000001",
            input_payload=NewsList(
                articles=[
                    NewsArticle(
                        title="Story",
                        url="https://example.com/story",
                        summary="Summary",
                        source="Example",
                        published_at="2026-06-17T00:00:00Z",
                    )
                ]
            ),
            llm_registry=registry,
        )


@pytest.mark.asyncio
async def test_run_registered_agent_does_not_dev_fallback_without_provider_key(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("HARPIA_ALLOW_DEV_AUTH", "true")
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    monkeypatch.setattr(
        "harpia_agents.agents.registry.TenantLLMResolver",
        lambda: type(
            "BrokenResolver",
            (),
            {"resolve": AsyncMock(side_effect=RuntimeError("resolver unavailable"))},
        )(),
    )

    registry = LLMRegistry.for_testing(
        model_ids=[MANIFEST.model_id],
        responses=["## Draft\nGenerated content"],
    )

    with pytest.raises(RuntimeError, match="resolver unavailable"):
        await run_registered_agent(
            "newsletter-writer-senior",
            tenant_id="00000000-0000-4000-8000-000000000001",
            input_payload=NewsList(
                articles=[
                    NewsArticle(
                        title="Story",
                        url="https://example.com/story",
                        summary="Summary",
                        source="Example",
                        published_at="2026-06-17T00:00:00Z",
                    )
                ]
            ),
            llm_registry=registry,
        )
