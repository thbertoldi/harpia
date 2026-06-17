"""OpenAI implementation for chat completion requests."""

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
from harpia_agents.llm.pricing import OPENAI_PRICING, ModelPricing
from harpia_agents.llm.provider import (
    ChatMessage,
    CompletionChunk,
    CompletionResult,
    LLMProvider,
    TokenUsage,
    estimate_cost,
)

try:
    import tiktoken
except ImportError:  # pragma: no cover - fallback is covered
    tiktoken = None


class OpenAIProvider(LLMProvider):
    name = "openai"
    supported_models = frozenset(OPENAI_PRICING.keys())
    pricing: Mapping[str, ModelPricing] = OPENAI_PRICING

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = "https://api.openai.com/v1",
        timeout: float = 30.0,
    ) -> None:
        self._api_key = api_key or os.getenv("OPENAI_API_KEY", "")
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
        payload = {
            "model": model_id,
            "messages": [
                {"role": message.role, "content": message.content} for message in messages
            ],
        }
        if max_tokens is not None:
            payload["max_tokens"] = max_tokens
        if temperature is not None:
            payload["temperature"] = temperature

        raw = await self._request("/chat/completions", payload)
        choices = raw.get("choices", [])
        content = ""
        if isinstance(choices, list) and choices:
            first = choices[0]
            if isinstance(first, dict):
                message = first.get("message", {})
                if isinstance(message, dict):
                    content = str(message.get("content", ""))

        usage_payload = raw.get("usage", {})
        input_tokens = int(
            usage_payload.get("prompt_tokens", self.count_tokens(model_id, messages))
        )
        output_tokens = int(usage_payload.get("completion_tokens", max(1, len(content) // 4)))
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
        if tiktoken is None:
            return sum(max(1, math.ceil(len(message.content) / 4)) for message in messages)
        model_for_encoding = "gpt-4o-mini" if model_id == "openai-gpt-4o-mini" else model_id
        try:
            encoding = tiktoken.encoding_for_model(model_for_encoding)
        except KeyError:
            encoding = tiktoken.get_encoding("cl100k_base")
        total = 0
        for message in messages:
            total += 4
            total += len(encoding.encode(message.role))
            total += len(encoding.encode(message.content))
        return total + 2

    def estimate_cost(self, model_id: str, input_tokens: int, output_tokens: int) -> float:
        return estimate_cost(
            self.pricing,
            model_id,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )

    async def _request(self, path: str, payload: dict[str, Any]) -> dict[str, Any]:
        headers = {
            "authorization": f"Bearer {self._api_key}",
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
            raise ProviderUnavailableError("invalid openai response")
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
