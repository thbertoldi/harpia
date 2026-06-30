import pytest
from harpia.artifacts.v1.artifacts_pb2 import NewsArticle, NewsList, TextDraft

from harpia_agents.agents.newsletter_writer import ElicitationRequest, MANIFEST, run
from harpia_agents.llm import ChatMessage, LLMRegistry
from harpia_agents.llm.testing import FakeLLMProvider


def _registry(response: str = "## Draft\nGenerated content") -> LLMRegistry:
    return LLMRegistry.for_testing(
        model_ids=[MANIFEST.model_id],
        responses=[response],
    )


def _capturing_registry(captured: list[ChatMessage], response: str) -> LLMRegistry:
    def transform(messages):
        captured.extend(messages)
        return response

    return LLMRegistry(
        [
            FakeLLMProvider(
                model_ids=[MANIFEST.model_id],
                transformer=transform,
            )
        ]
    )


def _news_list() -> NewsList:
    return NewsList(
        articles=[
            NewsArticle(
                title="AI regulations update",
                url="https://example.com/regulations",
                summary="Regulators published a new AI governance framework.",
                source="Example News",
                published_at="2026-06-15T10:00:00Z",
            ),
            NewsArticle(
                title="Open-source model release",
                url="https://example.com/open-source",
                summary="A new open-source model reached state-of-the-art benchmarks.",
                source="Tech Daily",
                published_at="2026-06-15T12:00:00Z",
            ),
        ]
    )


@pytest.mark.asyncio
async def test_run_returns_text_draft_referencing_each_article() -> None:
    result = await run(
        _news_list(),
        llm_registry=_registry("## Draft (professional)\nStories listed"),
        elicitation_responses={"tone": "professional", "topics_to_avoid": "rumors"},
    )

    assert isinstance(result, TextDraft)
    assert result.title
    assert "AI regulations update" in result.body
    assert "Open-source model release" in result.body
    assert "[source 1]" in result.body
    assert "[source 2]" in result.body


@pytest.mark.asyncio
async def test_run_passes_topic_language_and_audience_to_llm_prompt() -> None:
    captured: list[ChatMessage] = []

    result = await run(
        _news_list(),
        llm_registry=_capturing_registry(captured, "## Draft\nEnglish content"),
        elicitation_responses={
            "tone": "analytical",
            "topic": "retail growth",
            "language": "en-US",
            "audience": "founders",
            "topics_to_avoid": "rumors",
        },
    )

    assert isinstance(result, TextDraft)
    assert result.title == "Newsletter Draft: 2 Stories"
    system_message = next(message for message in captured if message.role == "system")
    user_message = next(message for message in captured if message.role == "user")
    assert "Respect the requested language exactly." in system_message.content
    assert "topic=retail growth" in user_message.content
    assert "language=en-US" in user_message.content
    assert "audience=founders" in user_message.content
    assert "topics_to_avoid=rumors" in user_message.content


@pytest.mark.asyncio
async def test_run_emits_elicitation_request_when_tone_missing() -> None:
    result = await run(_news_list(), llm_registry=_registry())

    assert isinstance(result, ElicitationRequest)
    assert result.required_fields == ("tone", "topics_to_avoid")
    assert result.thread_id.startswith("elicitation-")


@pytest.mark.asyncio
async def test_run_completes_after_elicitation_response() -> None:
    result = await run(
        _news_list(),
        llm_registry=_registry("## Draft (friendly)\nStories"),
        elicitation_responses={"tone": "friendly"},
    )

    assert isinstance(result, TextDraft)
    assert "Draft (friendly)" in result.body
