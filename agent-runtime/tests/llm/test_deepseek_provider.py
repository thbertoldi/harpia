import pytest
from pytest_httpx import HTTPXMock

from harpia_agents.llm.errors import AuthenticationError, RateLimitError
from harpia_agents.llm.provider import ChatMessage
from harpia_agents.llm.providers.deepseek import DeepSeekProvider


def _messages() -> list[ChatMessage]:
    return [ChatMessage(role="user", content="Write a sports update.")]


@pytest.mark.asyncio
async def test_complete_uses_deepseek_openai_compatible_endpoint(
    httpx_mock: HTTPXMock,
) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.deepseek.com/chat/completions",
        match_headers={"authorization": "Bearer ds-key"},
        json={
            "choices": [{"message": {"content": "Sports draft"}}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 4},
        },
    )
    provider = DeepSeekProvider(api_key="ds-key")

    result = await provider.complete("deepseek-v4-flash", _messages())

    assert result.provider == "deepseek"
    assert result.model_id == "deepseek-v4-flash"
    assert result.content == "Sports draft"
    assert result.usage.input_tokens == 10
    assert result.usage.output_tokens == 4


@pytest.mark.asyncio
async def test_auth_failure_maps_to_authentication_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.deepseek.com/chat/completions",
        status_code=401,
        text="invalid api key",
    )
    provider = DeepSeekProvider(api_key="bad")

    with pytest.raises(AuthenticationError):
        await provider.complete("deepseek-v4-flash", _messages())


@pytest.mark.asyncio
async def test_rate_limit_maps_to_rate_limit_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.deepseek.com/chat/completions",
        status_code=429,
        text="rate limited",
    )
    provider = DeepSeekProvider(api_key="ds-key")

    with pytest.raises(RateLimitError):
        await provider.complete("deepseek-v4-pro", _messages())
