"""Anthropic implementation for chat completion requests."""

from __future__ import annotations

import math
import os
from collections.abc import AsyncIterator, Mapping, Sequence
from typing import Any

import httpx

from harpia_agents.llm.errors import (
    AuthenticationError,
    BadRequestError,
    ProviderUnavailableError,
    RateLimitError,
)
from harpia_agents.llm.pricing import ANTHROPIC_PRICING, ModelPricing
from harpia_agents.llm.provider import (
    ChatMessage,
    CompletionChunk,
    CompletionResult,
    LLMProvider,
    TokenUsage,
    estimate_cost,
)


class AnthropicProvider(LLMProvider):
    name = "anthropic"
    supported_models = frozenset(ANTHROPIC_PRICING.keys())
    pricing: Mapping[str, ModelPricing] = ANTHROPIC_PRICING

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = "https://api.anthropic.com",
        timeout: float = 30.0,
    ) -> None:
        self._api_key = api_key or os.getenv("ANTHROPIC_API_KEY", "")
        self._base_url = base_url.rstrip("/")
        self._timeout = timeout

    async def complete(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> CompletionResult:
        payload = self._to_payload(
            model_id, messages, max_tokens=max_tokens, temperature=temperature
        )
        raw = await self._request("/v1/messages", payload)

        content_blocks = raw.get("content", [])
        content = "".join(
            block.get("text", "")
            for block in content_blocks
            if isinstance(block, dict) and block.get("type") == "text"
        )
        usage_payload = raw.get("usage", {})
        input_tokens = int(usage_payload.get("input_tokens", self.count_tokens(model_id, messages)))
        output_tokens = int(usage_payload.get("output_tokens", max(1, len(content) // 4)))
        usage = TokenUsage(
            input_tokens=input_tokens,
            output_tokens=output_tokens,
            cost_usd=self.estimate_cost(model_id, input_tokens, output_tokens),
        )
        return CompletionResult(
            content=content,
            model_id=model_id,
            usage=usage,
            provider=self.name,
            raw=raw,
        )

    async def stream(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> AsyncIterator[CompletionChunk]:
        result = await self.complete(
            model_id=model_id,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )
        yield CompletionChunk(content_delta=result.content, usage=result.usage)

    def count_tokens(self, model_id: str, messages: Sequence[ChatMessage]) -> int:
        del model_id
        return sum(max(1, math.ceil(len(message.content) / 4)) for message in messages)

    def estimate_cost(self, model_id: str, input_tokens: int, output_tokens: int) -> float:
        return estimate_cost(
            self.pricing,
            model_id,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )

    def _to_payload(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None,
        temperature: float | None,
    ) -> dict[str, Any]:
        system_prompt = "\n".join(
            message.content for message in messages if message.role == "system"
        )
        chat_messages = [
            {"role": message.role, "content": message.content}
            for message in messages
            if message.role in {"user", "assistant"}
        ]
        payload: dict[str, Any] = {
            "model": model_id,
            "messages": chat_messages,
            "max_tokens": max_tokens or 1024,
        }
        if system_prompt:
            payload["system"] = system_prompt
        if temperature is not None:
            payload["temperature"] = temperature
        return payload

    async def _request(self, path: str, payload: dict[str, Any]) -> dict[str, Any]:
        headers = {
            "x-api-key": self._api_key,
            "anthropic-version": "2023-06-01",
            "content-type": "application/json",
        }
        try:
            async with httpx.AsyncClient(timeout=self._timeout) as client:
                response = await client.post(
                    f"{self._base_url}{path}", json=payload, headers=headers
                )
        except httpx.HTTPError as exc:
            raise ProviderUnavailableError(str(exc)) from exc
        self._raise_for_status(response)
        data = response.json()
        if not isinstance(data, dict):
            raise ProviderUnavailableError("invalid anthropic response")
        return data

    @staticmethod
    def _raise_for_status(response: httpx.Response) -> None:
        if response.status_code < 400:
            return
        body = response.text
        if response.status_code in {401, 403}:
            raise AuthenticationError(body)
        if response.status_code == 429:
            raise RateLimitError(body)
        if response.status_code in {400, 404, 422}:
            raise BadRequestError(body)
        if response.status_code >= 500:
            raise ProviderUnavailableError(body)
        raise ProviderUnavailableError(body)
