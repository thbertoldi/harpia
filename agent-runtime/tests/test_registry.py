import pytest
from harpia.artifacts.v1.artifacts_pb2 import NewsArticle, NewsList, TextDraft

from harpia_agents.agents.newsletter_writer import MANIFEST
from harpia_agents.agents.registry import run_registered_agent
from harpia_agents.llm import LLMRegistry


@pytest.mark.asyncio
async def test_run_registered_agent_keeps_template_default_without_tenant_id() -> None:
    registry = LLMRegistry.for_testing(
        model_ids=[MANIFEST.model_id],
        responses=["## Draft\nGenerated content"],
    )
    result = await run_registered_agent(
        "newsletter-writer-senior",
        input_payload=NewsList(
            articles=[
                NewsArticle(
                    title="Story",
                    url="https://example.com/story",
                    summary="Summary",
                    source="Example",
                    published_at="2026-06-17T00:00:00Z",
                )
            ]
        ),
        llm_registry=registry,
        elicitation_responses={"tone": "neutral"},
    )

    assert isinstance(result, TextDraft)
    assert result.title
