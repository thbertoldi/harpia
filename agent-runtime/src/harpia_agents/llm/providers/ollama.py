"""Ollama implementation for local chat completion requests."""

from __future__ import annotations

import math
import os
from collections.abc import AsyncIterator, Mapping, Sequence
from typing import Any

import httpx

from harpia_agents.llm.errors import BadRequestError, ProviderUnavailableError
from harpia_agents.llm.pricing import OLLAMA_PRICING, ModelPricing
from harpia_agents.llm.provider import (
    ChatMessage,
    CompletionChunk,
    CompletionResult,
    LLMProvider,
    TokenUsage,
    estimate_cost,
)


class OllamaProvider(LLMProvider):
    name = "ollama"
    supported_models = frozenset(OLLAMA_PRICING.keys())
    pricing: Mapping[str, ModelPricing] = OLLAMA_PRICING

    def __init__(
        self,
        *,
        host: str | None = None,
        timeout: float = 30.0,
    ) -> None:
        self._base_url = (host or os.getenv("OLLAMA_HOST", "http://localhost:11434")).rstrip("/")
        self._timeout = timeout

    async def complete(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> CompletionResult:
        payload: dict[str, Any] = {
            "model": model_id,
            "messages": [
                {"role": message.role, "content": message.content} for message in messages
            ],
            "stream": False,
        }
        if max_tokens is not None or temperature is not None:
            payload["options"] = {}
            if max_tokens is not None:
                payload["options"]["num_predict"] = max_tokens
            if temperature is not None:
                payload["options"]["temperature"] = temperature

        raw = await self._request("/api/chat", payload)
        message = raw.get("message", {})
        content = str(message.get("content", "")) if isinstance(message, dict) else ""
        input_tokens = int(raw.get("prompt_eval_count", self.count_tokens(model_id, messages)))
        output_tokens = int(raw.get("eval_count", max(1, len(content) // 4)))
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

    async def _request(self, path: str, payload: dict[str, Any]) -> dict[str, Any]:
        try:
            async with httpx.AsyncClient(timeout=self._timeout) as client:
                response = await client.post(f"{self._base_url}{path}", json=payload)
        except httpx.HTTPError as exc:
            raise ProviderUnavailableError(str(exc)) from exc
        if response.status_code >= 500:
            raise ProviderUnavailableError(response.text)
        if response.status_code >= 400:
            raise BadRequestError(response.text)
        data = response.json()
        if not isinstance(data, dict):
            raise ProviderUnavailableError("invalid ollama response")
        return data
