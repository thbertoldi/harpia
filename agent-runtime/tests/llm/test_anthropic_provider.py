import pytest
from pytest_httpx import HTTPXMock

from harpia_agents.llm.errors import (
    AuthenticationError,
    ProviderUnavailableError,
    RateLimitError,
)
from harpia_agents.llm.provider import ChatMessage
from harpia_agents.llm.providers.anthropic import AnthropicProvider


def _messages() -> list[ChatMessage]:
    return [ChatMessage(role="user", content="Summarize this text.")]


@pytest.mark.asyncio
async def test_complete_happy_path(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.anthropic.com/v1/messages",
        json={
            "content": [{"type": "text", "text": "Here is the summary."}],
            "usage": {"input_tokens": 10, "output_tokens": 6},
        },
    )
    provider = AnthropicProvider(api_key="key")

    result = await provider.complete("claude-3-haiku-20240307", _messages())

    assert result.content == "Here is the summary."
    assert result.usage.input_tokens == 10
    assert result.usage.output_tokens == 6


@pytest.mark.asyncio
async def test_stream_happy_path(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.anthropic.com/v1/messages",
        json={
            "content": [{"type": "text", "text": "stream payload"}],
            "usage": {"input_tokens": 9, "output_tokens": 3},
        },
    )
    provider = AnthropicProvider(api_key="key")

    chunks = [chunk async for chunk in provider.stream("claude-3-haiku-20240307", _messages())]

    assert chunks[0].content_delta == "stream payload"
    assert chunks[0].usage is not None


@pytest.mark.asyncio
async def test_auth_failure_maps_to_authentication_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.anthropic.com/v1/messages",
        status_code=401,
        text="invalid key",
    )
    provider = AnthropicProvider(api_key="bad")

    with pytest.raises(AuthenticationError):
        await provider.complete("claude-3-haiku-20240307", _messages())


@pytest.mark.asyncio
async def test_rate_limit_maps_to_rate_limit_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.anthropic.com/v1/messages",
        status_code=429,
        text="too many requests",
    )
    provider = AnthropicProvider(api_key="key")

    with pytest.raises(RateLimitError):
        await provider.complete("claude-3-haiku-20240307", _messages())


@pytest.mark.asyncio
async def test_server_failure_maps_to_provider_unavailable(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.anthropic.com/v1/messages",
        status_code=503,
        text="service unavailable",
    )
    provider = AnthropicProvider(api_key="key")

    with pytest.raises(ProviderUnavailableError):
        await provider.complete("claude-3-haiku-20240307", _messages())
