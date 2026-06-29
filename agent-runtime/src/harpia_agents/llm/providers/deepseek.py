"""DeepSeek OpenAI-compatible implementation for chat completion requests."""

from __future__ import annotations

import os

from harpia_agents.llm.pricing import DEEPSEEK_PRICING, ModelPricing
from harpia_agents.llm.providers.openai import OpenAIProvider


class DeepSeekProvider(OpenAIProvider):
    name = "deepseek"
    supported_models = frozenset(DEEPSEEK_PRICING.keys())
    pricing: dict[str, ModelPricing] = DEEPSEEK_PRICING

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = "https://api.deepseek.com",
        timeout: float = 30.0,
    ) -> None:
        key = os.getenv("DEEPSEEK_API_KEY", "") if api_key is None else api_key
        super().__init__(api_key=key, base_url=base_url, timeout=timeout)
        self._api_key = key
