import pytest
from pytest_httpx import HTTPXMock

from harpia_agents.llm.errors import BadRequestError, ProviderUnavailableError
from harpia_agents.llm.provider import ChatMessage
from harpia_agents.llm.providers.ollama import OllamaProvider


def _messages() -> list[ChatMessage]:
    return [ChatMessage(role="user", content="Tell me about this week.")]


@pytest.mark.asyncio
async def test_complete_happy_path(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="http://localhost:11434/api/chat",
        json={
            "message": {"content": "Ollama response"},
            "prompt_eval_count": 15,
            "eval_count": 5,
        },
    )
    provider = OllamaProvider()

    result = await provider.complete("llama3:70b", _messages())

    assert result.content == "Ollama response"
    assert result.usage.input_tokens == 15
    assert result.usage.output_tokens == 5


@pytest.mark.asyncio
async def test_stream_happy_path(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="http://localhost:11434/api/chat",
        json={
            "message": {"content": "chunk"},
            "prompt_eval_count": 8,
            "eval_count": 3,
        },
    )
    provider = OllamaProvider()

    chunks = [chunk async for chunk in provider.stream("llama3:70b", _messages())]

    assert chunks[0].content_delta == "chunk"
    assert chunks[0].usage is not None


@pytest.mark.asyncio
async def test_bad_request_maps_to_bad_request_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="http://localhost:11434/api/chat",
        status_code=400,
        text="invalid request",
    )
    provider = OllamaProvider()

    with pytest.raises(BadRequestError):
        await provider.complete("llama3:70b", _messages())


@pytest.mark.asyncio
async def test_server_failure_maps_to_provider_unavailable(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="http://localhost:11434/api/chat",
        status_code=503,
        text="offline",
    )
    provider = OllamaProvider()

    with pytest.raises(ProviderUnavailableError):
        await provider.complete("llama3:70b", _messages())
