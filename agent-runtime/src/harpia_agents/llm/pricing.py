"""Point-in-time model pricing in USD per 1M tokens.

Prices move over time and should be moved to runtime config in a follow-up
settings story (#34).
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ModelPricing:
    """Provider pricing for one model."""

    input_per_million_usd: float
    output_per_million_usd: float


ANTHROPIC_PRICING: dict[str, ModelPricing] = {
    "claude-opus-4-7": ModelPricing(input_per_million_usd=15.0, output_per_million_usd=75.0),
    "claude-3-5-sonnet-20241022": ModelPricing(
        input_per_million_usd=3.0, output_per_million_usd=15.0
    ),
    "claude-3-opus-20240229": ModelPricing(input_per_million_usd=15.0, output_per_million_usd=75.0),
    "claude-3-haiku-20240307": ModelPricing(
        input_per_million_usd=0.25, output_per_million_usd=1.25
    ),
}

OPENAI_PRICING: dict[str, ModelPricing] = {
    "gpt-4o": ModelPricing(input_per_million_usd=5.0, output_per_million_usd=15.0),
    "gpt-4o-mini": ModelPricing(input_per_million_usd=0.15, output_per_million_usd=0.6),
    # Existing manifests currently use this alias.
    "openai-gpt-4o-mini": ModelPricing(input_per_million_usd=0.15, output_per_million_usd=0.6),
}

OLLAMA_PRICING: dict[str, ModelPricing] = {
    "llama3:70b": ModelPricing(input_per_million_usd=0.0, output_per_million_usd=0.0),
}
