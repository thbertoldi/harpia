import json

import pytest
from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import CarouselDraft, CarouselSlide, TextDraft

from harpia_agents.agents.linkedin_carousel import (
    MANIFEST,
    carousel_draft_to_mapping,
    run,
)
from harpia_agents.llm import ChatMessage, LLMRegistry


def _registry(response: str) -> LLMRegistry:
    return LLMRegistry.for_testing(
        model_ids=[MANIFEST.model_id],
        responses=[response],
    )


def _capturing_registry(captured: list[ChatMessage], response: str) -> LLMRegistry:
    from harpia_agents.llm.testing import FakeLLMProvider

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
                "alt_text": "Diagram of the new audit pipeline",
            },
        ],
        "caption": "A quick breakdown of this week's governance news.",
        "hashtags": ["aigovernance", "compliance"],
    }


def _validate_carousel_draft_payload(payload: dict[str, object]) -> None:
    required_fields = tuple(MANIFEST.to_dict()["output_schema"].get("required", []))

    for field_name in required_fields:
        if field_name == "slides":
            continue
        value = payload.get(field_name)
        assert isinstance(value, str) and value.strip(), f"{field_name} must be non-empty"

    slides = payload.get("slides")
    assert isinstance(slides, list) and len(slides) >= 1
    for slide in slides:
        assert isinstance(slide, dict)
        heading = slide.get("heading", "")
        body = slide.get("body", "")
        assert (isinstance(heading, str) and heading.strip()) or (
            isinstance(body, str) and body.strip()
        )

    parsed = CarouselDraft()
    ParseDict(payload, parsed)
    roundtrip = MessageToDict(parsed, preserving_proto_field_name=True)
    assert roundtrip["title"] == payload["title"]
    assert len(roundtrip["slides"]) == len(payload["slides"])


@pytest.mark.asyncio
async def test_run_returns_carousel_draft_from_text_draft() -> None:
    result = await run(
        _text_draft(),
        llm_registry=_registry(json.dumps(_carousel_payload())),
    )

    assert isinstance(result, CarouselDraft)
    assert result.title == "AI Governance in 6 Slides"
    assert result.hook == "Regulators just rewrote the rules."
    assert result.caption
    assert "aigovernance" in result.hashtags
    assert len(result.slides) == 2
    assert isinstance(result.slides[0], CarouselSlide)
    assert result.slides[0].heading == "The shift"
    assert result.slides[1].alt_text == "Diagram of the new audit pipeline"


@pytest.mark.asyncio
async def test_run_strips_markdown_fences() -> None:
    payload = _carousel_payload()
    fenced = f"```json\n{json.dumps(payload)}\n```"

    result = await run(
        _text_draft(),
        llm_registry=_registry(fenced),
    )

    assert isinstance(result, CarouselDraft)
    assert result.title == payload["title"]
    assert len(result.slides) == len(payload["slides"])


@pytest.mark.asyncio
async def test_run_prompt_preserves_source_language() -> None:
    captured: list[ChatMessage] = []

    result = await run(
        _text_draft(),
        llm_registry=_capturing_registry(captured, json.dumps(_carousel_payload())),
    )

    assert isinstance(result, CarouselDraft)
    system_message = next(message for message in captured if message.role == "system")
    user_message = next(message for message in captured if message.role == "user")
    assert "requested output language" in system_message.content
    assert "Target language" in user_message.content


@pytest.mark.asyncio
async def test_run_output_passes_artifact_schema_validation() -> None:
    result = await run(
        _text_draft(),
        llm_registry=_registry(json.dumps(_carousel_payload())),
    )
    payload = carousel_draft_to_mapping(result)

    _validate_carousel_draft_payload(payload)
    assert json.dumps(payload)


@pytest.mark.asyncio
async def test_run_raises_value_error_on_invalid_json() -> None:
    with pytest.raises(ValueError, match="not valid JSON"):
        await run(
            _text_draft(),
            llm_registry=_registry("this is ::: not json {{{"),
        )


@pytest.mark.asyncio
async def test_run_raises_value_error_on_empty_slides() -> None:
    payload = _carousel_payload()
    payload["slides"] = []

    with pytest.raises(ValueError, match="slides"):
        await run(
            _text_draft(),
            llm_registry=_registry(json.dumps(payload)),
        )
