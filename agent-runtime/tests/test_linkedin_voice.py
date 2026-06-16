import json

import pytest
from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, TextDraft

from harpia_agents.agents.linkedin_voice import (
    MANIFEST,
    linkedin_post_draft_to_mapping,
    run,
)


class FakeLLMClient:
    async def adapt_for_linkedin(self, *, title, body):  # noqa: ANN001
        return f"LinkedIn-ready: {title}\n\n{body[:500]}"


def _text_draft() -> TextDraft:
    return TextDraft(
        title="Weekly AI Governance Brief",
        body=(
            "## Overview\n"
            "Regulators published a new AI governance framework this week.\n\n"
            "## Highlights\n"
            "1. **AI regulations update** - New compliance requirements.\n"
            "2. **Open-source model release** - Benchmark-leading model launch."
        ),
    )


def _validate_linkedin_post_draft_payload(payload: dict[str, object]) -> None:
    required_fields = tuple(MANIFEST.to_dict()["output_schema"].get("required", []))
    max_length = MANIFEST.to_dict()["output_schema"]["properties"]["text"]["maxLength"]

    for field_name in required_fields:
        value = payload.get(field_name)
        assert isinstance(value, str) and value.strip(), f"{field_name} must be non-empty"

    text = payload["text"]
    assert isinstance(text, str)
    assert len(text) <= max_length

    parsed = LinkedInPostDraft()
    ParseDict(payload, parsed)
    roundtrip = MessageToDict(parsed, preserving_proto_field_name=True)
    assert roundtrip["text"] == payload["text"]


@pytest.mark.asyncio
async def test_run_returns_linkedin_post_draft_from_text_draft() -> None:
    result = await run(_text_draft(), llm_client=FakeLLMClient())

    assert isinstance(result, LinkedInPostDraft)
    assert result.text
    assert "Weekly AI Governance Brief" in result.text
    assert result.hook == "Weekly AI Governance Brief"
    assert result.hashtags
    assert len(result.text) <= 3000


@pytest.mark.asyncio
async def test_run_output_passes_artifact_schema_validation() -> None:
    result = await run(_text_draft(), llm_client=FakeLLMClient())
    payload = linkedin_post_draft_to_mapping(result)

    _validate_linkedin_post_draft_payload(payload)
    assert json.dumps(payload)
