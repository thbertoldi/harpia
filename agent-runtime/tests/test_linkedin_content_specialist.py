import json

import pytest
from harpia.artifacts.v1.artifacts_pb2 import CarouselDraft, LinkedInPostDraft, TextDraft

from harpia_agents.agents.linkedin_content_specialist import (
    MANIFEST,
    run,
)
from harpia_agents.llm import ChatMessage, LLMRegistry
from harpia_agents.llm.testing import FakeLLMProvider


def _registry(response: str) -> LLMRegistry:
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


def _specs_by_output() -> dict[str, dict[str, object]]:
    return {spec["artifact_output_type"]: spec for spec in MANIFEST.to_dict()["capability_specs"]}


def _text_draft() -> TextDraft:
    return TextDraft(
        title="Weekly AI Governance Brief",
        body=(
            "Target language: English (United States).\n\n"
            "## Overview\n"
            "Regulators published a new AI governance framework this week.\n\n"
            "## Highlights\n"
            "1. **AI regulations update** - New compliance requirements.\n"
            "2. **Open-source model release** - Benchmark-leading model launch."
        ),
    )


def _carousel_payload() -> dict[str, object]:
    return {
        "title": "AI Governance in 6 Slides",
        "hook": "Regulators just rewrote the rules.",
        "slides": [
            {"heading": "The shift", "body": "New compliance requirements landed this week."},
            {
                "heading": "Why it matters",
                "body": "Every model launch now carries audit obligations.",
            },
        ],
        "caption": "A quick breakdown of this week's governance news.",
        "hashtags": ["aigovernance", "compliance"],
    }


@pytest.mark.asyncio
async def test_adapt_capability_returns_linkedin_post_draft() -> None:
    result = await run(
        _text_draft(),
        llm_registry=_registry("LinkedIn-ready: Weekly AI Governance Brief\n\nShort body"),
        output_artifact_type_key="harpia.artifacts.v1.LinkedInPostDraft",
    )

    assert isinstance(result, LinkedInPostDraft)
    assert result.text
    assert "Weekly AI Governance Brief" in result.text
    assert result.hook == "Weekly AI Governance Brief"
    assert result.hashtags
    assert len(result.text) <= 3000


@pytest.mark.asyncio
async def test_carousel_capability_returns_carousel_draft() -> None:
    result = await run(
        _text_draft(),
        llm_registry=_registry(json.dumps(_carousel_payload())),
        output_artifact_type_key="harpia.artifacts.v1.CarouselDraft",
    )

    assert isinstance(result, CarouselDraft)
    assert result.title == "AI Governance in 6 Slides"
    assert result.hook == "Regulators just rewrote the rules."
    assert len(result.slides) == 2
    assert result.slides[0].heading == "The shift"
    assert "aigovernance" in result.hashtags


@pytest.mark.asyncio
async def test_unknown_output_type_raises_value_error() -> None:
    with pytest.raises(ValueError, match="no capability producing"):
        await run(
            _text_draft(),
            llm_registry=_registry("anything"),
            output_artifact_type_key="harpia.artifacts.v1.NewsList",
        )


@pytest.mark.asyncio
async def test_adapt_capability_uses_adapt_spec_system_prompt() -> None:
    captured: list[ChatMessage] = []

    await run(
        _text_draft(),
        llm_registry=_capturing_registry(
            captured,
            "LinkedIn-ready: Weekly AI Governance Brief\n\nShort body",
        ),
        output_artifact_type_key="harpia.artifacts.v1.LinkedInPostDraft",
    )

    system_message = next(message for message in captured if message.role == "system")
    expected = str(_specs_by_output()["harpia.artifacts.v1.LinkedInPostDraft"]["system_prompt"])
    assert system_message.content == expected
    assert "polished" in system_message.content
    assert "carousel" not in system_message.content.lower()


@pytest.mark.asyncio
async def test_carousel_capability_uses_carousel_spec_system_prompt() -> None:
    captured: list[ChatMessage] = []

    await run(
        _text_draft(),
        llm_registry=_capturing_registry(captured, json.dumps(_carousel_payload())),
        output_artifact_type_key="harpia.artifacts.v1.CarouselDraft",
    )

    system_message = next(message for message in captured if message.role == "system")
    expected = str(_specs_by_output()["harpia.artifacts.v1.CarouselDraft"]["system_prompt"])
    assert system_message.content == expected
    assert "carousel outline" in system_message.content.lower()


@pytest.mark.asyncio
async def test_empty_title_or_body_raises_value_error() -> None:
    with pytest.raises(ValueError, match="input_schema violation"):
        await run(
            TextDraft(title="", body="has body"),
            llm_registry=_registry("anything"),
            output_artifact_type_key="harpia.artifacts.v1.LinkedInPostDraft",
        )


@pytest.mark.asyncio
async def test_carousel_capability_raises_on_invalid_json() -> None:
    with pytest.raises(ValueError, match="not valid JSON"):
        await run(
            _text_draft(),
            llm_registry=_registry("::: not json {{{"),
            output_artifact_type_key="harpia.artifacts.v1.CarouselDraft",
        )
