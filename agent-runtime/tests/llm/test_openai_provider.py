import pytest
from pytest_httpx import HTTPXMock

from harpia_agents.llm.errors import (
    AuthenticationError,
    ProviderUnavailableError,
    RateLimitError,
)
from harpia_agents.llm.provider import ChatMessage
from harpia_agents.llm.providers.openai import OpenAIProvider


def _messages() -> list[ChatMessage]:
    return [ChatMessage(role="user", content="Write a short update.")]


@pytest.mark.asyncio
async def test_complete_happy_path(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.openai.com/v1/chat/completions",
        json={
            "choices": [{"message": {"content": "Draft post"}}],
            "usage": {"prompt_tokens": 12, "completion_tokens": 3},
        },
    )
    provider = OpenAIProvider(api_key="key")

    result = await provider.complete("gpt-4o-mini", _messages())

    assert result.content == "Draft post"
    assert result.usage.input_tokens == 12
    assert result.usage.output_tokens == 3


@pytest.mark.asyncio
async def test_stream_happy_path(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.openai.com/v1/chat/completions",
        json={
            "choices": [{"message": {"content": "streamed text"}}],
            "usage": {"prompt_tokens": 8, "completion_tokens": 2},
        },
    )
    provider = OpenAIProvider(api_key="key")

    chunks = [chunk async for chunk in provider.stream("gpt-4o-mini", _messages())]

    assert chunks[0].content_delta == "streamed text"
    assert chunks[0].usage is not None


@pytest.mark.asyncio
async def test_auth_failure_maps_to_authentication_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.openai.com/v1/chat/completions",
        status_code=401,
        text="invalid api key",
    )
    provider = OpenAIProvider(api_key="bad")

    with pytest.raises(AuthenticationError):
        await provider.complete("gpt-4o-mini", _messages())


@pytest.mark.asyncio
async def test_rate_limit_maps_to_rate_limit_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.openai.com/v1/chat/completions",
        status_code=429,
        text="rate limited",
    )
    provider = OpenAIProvider(api_key="key")

    with pytest.raises(RateLimitError):
        await provider.complete("gpt-4o-mini", _messages())


@pytest.mark.asyncio
async def test_server_failure_maps_to_provider_unavailable(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.openai.com/v1/chat/completions",
        status_code=500,
        text="oops",
    )
    provider = OpenAIProvider(api_key="key")

    with pytest.raises(ProviderUnavailableError):
        await provider.complete("gpt-4o-mini", _messages())
