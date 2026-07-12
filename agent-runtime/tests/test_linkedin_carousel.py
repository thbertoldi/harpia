import json

import pytest
from harpia.artifacts.v1.artifacts_pb2 import LinkedInPost
from harpia_agents.agents.newsletter_writer import ElicitationRequest

from harpia_agents.agents.linkedin_carousel import MANIFEST, run
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


def _post() -> LinkedInPost:
    post = LinkedInPost()
    post.text.text = "Regulators published a new AI governance framework this week."
    post.text.hook = "The governance update"
    post.text.hashtags.extend(["aigovernance", "compliance"])
    return post


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


@pytest.mark.asyncio
async def test_run_elicits_missing_carousel_context_before_drafting() -> None:
    result = await run(
        _post(),
        llm_registry=_registry(json.dumps(_carousel_payload())),
    )
    assert isinstance(result, ElicitationRequest)
    assert result.required_fields == ("carousel_goal", "audience")


@pytest.mark.asyncio
async def test_run_returns_revised_linkedin_post_after_context_and_feedback() -> None:
    result = await run(
        _post(),
        llm_registry=_registry(json.dumps(_carousel_payload())),
        elicitation_responses={"carousel_goal": "Educate operators", "audience": "Operations leaders"},
        review_feedback="Make the first slide more direct.",
    )

    assert isinstance(result, LinkedInPost)
    assert result.text.text == _post().text.text
    assert result.carousel.title == "AI Governance in 6 Slides"
    assert result.carousel.hook == "Regulators just rewrote the rules."
    assert result.carousel.caption
    assert "aigovernance" in result.carousel.hashtags
    assert len(result.carousel.slides) == 2
    assert result.carousel.slides[0].heading == "The shift"
    assert result.carousel.slides[1].alt_text == "Diagram of the new audit pipeline"


@pytest.mark.asyncio
async def test_run_strips_markdown_fences() -> None:
    payload = _carousel_payload()
    fenced = f"```json\n{json.dumps(payload)}\n```"

    result = await run(
        _post(),
        llm_registry=_registry(fenced),
        elicitation_responses={"carousel_goal": "Educate", "audience": "Operators"},
    )

    assert isinstance(result, LinkedInPost)
    assert result.carousel.title == payload["title"]
    assert len(result.carousel.slides) == len(payload["slides"])


@pytest.mark.asyncio
async def test_run_prompt_preserves_source_language() -> None:
    captured: list[ChatMessage] = []

    result = await run(
        _post(),
        llm_registry=_capturing_registry(captured, json.dumps(_carousel_payload())),
        elicitation_responses={"carousel_goal": "Educate", "audience": "Operators"},
        review_feedback="Use a clearer hook.",
    )

    assert isinstance(result, LinkedInPost)
    system_message = next(message for message in captured if message.role == "system")
    user_message = next(message for message in captured if message.role == "user")
    assert "carousel" in system_message.content.lower()
    assert "carousel goal: Educate" in user_message.content
    assert "revision feedback: Use a clearer hook." in user_message.content


@pytest.mark.asyncio
async def test_run_raises_value_error_on_invalid_json() -> None:
    with pytest.raises(ValueError, match="not valid JSON"):
        await run(
            _post(),
            llm_registry=_registry("this is ::: not json {{{"),
            elicitation_responses={"carousel_goal": "Educate", "audience": "Operators"},
        )


@pytest.mark.asyncio
async def test_run_raises_value_error_on_empty_slides() -> None:
    payload = _carousel_payload()
    payload["slides"] = []

    with pytest.raises(ValueError, match="slides"):
        await run(
            _post(),
            llm_registry=_registry(json.dumps(payload)),
            elicitation_responses={"carousel_goal": "Educate", "audience": "Operators"},
        )
