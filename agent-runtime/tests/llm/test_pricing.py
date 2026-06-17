from harpia_agents.llm.pricing import ANTHROPIC_PRICING, OPENAI_PRICING
from harpia_agents.llm.provider import TokenUsage
from harpia_agents.llm.providers.anthropic import AnthropicProvider
from harpia_agents.llm.providers.openai import OpenAIProvider


def test_estimate_cost_math_openai() -> None:
    provider = OpenAIProvider(api_key="test")
    expected = (1200 / 1_000_000) * OPENAI_PRICING["gpt-4o-mini"].input_per_million_usd + (
        800 / 1_000_000
    ) * OPENAI_PRICING["gpt-4o-mini"].output_per_million_usd

    assert provider.estimate_cost("gpt-4o-mini", 1200, 800) == expected


def test_estimate_cost_math_anthropic() -> None:
    provider = AnthropicProvider(api_key="test")
    expected = (2000 / 1_000_000) * ANTHROPIC_PRICING[
        "claude-3-5-sonnet-20241022"
    ].input_per_million_usd + (1000 / 1_000_000) * ANTHROPIC_PRICING[
        "claude-3-5-sonnet-20241022"
    ].output_per_million_usd
    assert provider.estimate_cost("claude-3-5-sonnet-20241022", 2000, 1000) == expected


def test_token_usage_cost_derivation() -> None:
    provider = OpenAIProvider(api_key="test")
    usage = TokenUsage(
        input_tokens=500,
        output_tokens=250,
        cost_usd=provider.estimate_cost("gpt-4o-mini", 500, 250),
    )

    assert usage.cost_usd > 0
