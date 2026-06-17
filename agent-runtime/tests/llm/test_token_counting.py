from harpia_agents.llm.provider import ChatMessage
from harpia_agents.llm.providers.anthropic import AnthropicProvider
from harpia_agents.llm.providers.ollama import OllamaProvider
from harpia_agents.llm.providers.openai import OpenAIProvider


def test_token_counting_sanity_per_provider() -> None:
    messages = [
        ChatMessage(role="system", content="You are helpful."),
        ChatMessage(role="user", content="Summarize this short paragraph."),
    ]

    anthropic_tokens = AnthropicProvider(api_key="x").count_tokens(
        "claude-3-haiku-20240307",
        messages,
    )
    openai_tokens = OpenAIProvider(api_key="x").count_tokens("gpt-4o-mini", messages)
    ollama_tokens = OllamaProvider().count_tokens("llama3:70b", messages)

    assert anthropic_tokens > 0
    assert openai_tokens > 0
    assert ollama_tokens > 0
