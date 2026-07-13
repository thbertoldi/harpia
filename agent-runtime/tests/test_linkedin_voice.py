import json

import pytest
from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import LinkedInPost, LinkedInPostDraft, TextDraft

from harpia_agents.agents.linkedin_voice import (
    MANIFEST,
    linkedin_post_draft_to_mapping,
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


def _validate_linkedin_post_payload(payload: dict[str, object]) -> None:
    max_length = MANIFEST.to_dict()["output_schema"]["properties"]["text"]["maxLength"]

    text = payload["text"]
    assert isinstance(text, dict)
    assert isinstance(text["text"], str)
    assert len(text["text"]) <= max_length

    parsed = LinkedInPost()
    ParseDict(payload, parsed)
    roundtrip = MessageToDict(parsed, preserving_proto_field_name=True)
    assert roundtrip["text"] == payload["text"]


@pytest.mark.asyncio
async def test_run_returns_linkedin_post_from_text_draft() -> None:
    result = await run(
        _text_draft(),
        llm_registry=_registry("LinkedIn-ready: Weekly AI Governance Brief\n\nShort body"),
    )

    assert isinstance(result, LinkedInPost)
    assert result.text.text
    assert "Weekly AI Governance Brief" in result.text.text
    assert result.text.hook == "Weekly AI Governance Brief"
    assert result.text.hashtags
    assert len(result.text.text) <= 3000


@pytest.mark.asyncio
async def test_run_prompt_preserves_source_language() -> None:
    captured: list[ChatMessage] = []

    result = await run(
        _text_draft(),
        llm_registry=_capturing_registry(
            captured,
            "LinkedIn-ready: Weekly AI Governance Brief\n\nShort body",
        ),
    )

    assert isinstance(result, LinkedInPost)
    system_message = next(message for message in captured if message.role == "system")
    user_message = next(message for message in captured if message.role == "user")
    assert "requested output language" in system_message.content
    assert "Target language" in user_message.content


@pytest.mark.asyncio
async def test_run_output_passes_artifact_schema_validation() -> None:
    result = await run(
        _text_draft(),
        llm_registry=_registry("LinkedIn-ready: Weekly AI Governance Brief\n\nShort body"),
    )
    payload = linkedin_post_draft_to_mapping(result)

    _validate_linkedin_post_payload(payload)
    assert json.dumps(payload)
