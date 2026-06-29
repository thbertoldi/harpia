import pytest

from harpia_agents.llm.errors import ModelNotFoundError
from harpia_agents.llm.registry import LLMRegistry


def test_registry_resolves_known_models() -> None:
    registry = LLMRegistry.default()

    assert registry.resolve("gpt-4o-mini").name == "openai"
    assert registry.resolve("claude-opus-4-7").name == "anthropic"
    assert registry.resolve("claude-3-haiku-20240307").name == "anthropic"
    assert registry.resolve("deepseek-v4-flash").name == "deepseek"
    assert registry.resolve("llama3:70b").name == "ollama"


def test_registry_raises_for_unknown_model() -> None:
    registry = LLMRegistry.default()

    with pytest.raises(ModelNotFoundError, match="unknown model_id"):
        registry.resolve("does-not-exist")
