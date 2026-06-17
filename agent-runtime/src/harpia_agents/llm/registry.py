"""Model-to-provider routing for LLM calls."""

from __future__ import annotations

from collections.abc import AsyncIterator, Mapping, Sequence
from dataclasses import dataclass

from harpia_agents.llm.errors import ModelNotFoundError
from harpia_agents.llm.provider import ChatMessage, CompletionChunk, CompletionResult, LLMProvider
from harpia_agents.llm.providers.anthropic import AnthropicProvider
from harpia_agents.llm.providers.ollama import OllamaProvider
from harpia_agents.llm.providers.openai import OpenAIProvider
from harpia_agents.llm.testing import FakeLLMProvider


@dataclass(frozen=True)
class ModelInfo:
    model_id: str
    provider: str
    pricing: object


class LLMRegistry:
    """Resolves model IDs to concrete provider adapters."""

    def __init__(
        self,
        providers: Sequence[LLMProvider],
        *,
        model_routes: Mapping[str, str] | None = None,
    ) -> None:
        self._providers_by_name = {provider.name: provider for provider in providers}
        self._model_routes = dict(model_routes or self._derive_routes(providers))

    @staticmethod
    def _derive_routes(providers: Sequence[LLMProvider]) -> dict[str, str]:
        routes: dict[str, str] = {}
        for provider in providers:
            for model_id in provider.supported_models:
                routes[model_id] = provider.name
        return routes

    @classmethod
    def default(cls) -> LLMRegistry:
        providers: list[LLMProvider] = [
            AnthropicProvider(),
            OpenAIProvider(),
            OllamaProvider(),
        ]
        return cls(providers)

    @classmethod
    def for_testing(
        cls,
        *,
        model_ids: Sequence[str],
        responses: Sequence[str] | None = None,
    ) -> LLMRegistry:
        provider = FakeLLMProvider(model_ids=model_ids, responses=responses)
        return cls([provider])

    def resolve(self, model_id: str) -> LLMProvider:
        provider_name = self._model_routes.get(model_id)
        if provider_name is None:
            raise ModelNotFoundError(f"unknown model_id: {model_id}")
        provider = self._providers_by_name.get(provider_name)
        if provider is None:
            raise ModelNotFoundError(
                f"model_id {model_id} routes to unknown provider {provider_name}"
            )
        return provider

    def list_models(self) -> list[ModelInfo]:
        items: list[ModelInfo] = []
        for model_id, provider_name in sorted(self._model_routes.items()):
            provider = self._providers_by_name[provider_name]
            items.append(
                ModelInfo(
                    model_id=model_id,
                    provider=provider_name,
                    pricing=provider.pricing.get(model_id),
                )
            )
        return items

    async def complete(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> CompletionResult:
        provider = self.resolve(model_id)
        return await provider.complete(
            model_id=model_id,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )

    async def stream(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> AsyncIterator[CompletionChunk]:
        provider = self.resolve(model_id)
        async for chunk in provider.stream(
            model_id=model_id,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
        ):
            yield chunk
