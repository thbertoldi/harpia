"""LinkedIn carousel agent implementation."""

from __future__ import annotations

import json
from collections.abc import Mapping

from google.protobuf.json_format import MessageToDict
from harpia.artifacts.v1.artifacts_pb2 import (
    CarouselDraft,
    CarouselSlide,
    LinkedInPost,
)
from langgraph.graph import END, StateGraph
from pydantic import BaseModel, ConfigDict

from harpia_agents.agents.manifest import AgentType, resolve_manifest_path
from harpia_agents.agents.newsletter_writer import ElicitationRequest
from harpia_agents.llm import ChatMessage, LLMRegistry

MANIFEST_ID = "linkedin-carousel-senior"
MANIFEST_PATH = resolve_manifest_path(MANIFEST_ID)
_KNOWN_MODEL_IDS = tuple(model.model_id for model in LLMRegistry.default().list_models())
MANIFEST = AgentType.from_yaml(MANIFEST_PATH, model_ids=_KNOWN_MODEL_IDS)

type AgentRunResult = LinkedInPost | ElicitationRequest


class LinkedInCarouselState(BaseModel):
    """LangGraph state for the LinkedIn carousel drafting flow."""

    model_config = ConfigDict(extra="forbid", arbitrary_types_allowed=True)

    post: LinkedInPost
    carousel_goal: str
    audience: str
    review_feedback: str = ""
    carousel_draft: CarouselDraft | None = None


def _strip_code_fences(raw: str) -> str:
    """Best-effort removal of ```json ... ``` style fences from LLM output."""
    text = raw.strip()
    if not text.startswith("```"):
        return text
    lines = text.splitlines()
    if lines and lines[0].startswith("```"):
        lines = lines[1:]
    if lines and lines[-1].strip().startswith("```"):
        lines = lines[:-1]
    return "\n".join(lines).strip()


def _validate_carousel_draft_schema(draft: CarouselDraft) -> None:
    if not draft.title.strip():
        raise ValueError("carousel title must be a non-empty string")
    if not draft.slides:
        raise ValueError("output_schema violation: `slides` must contain at least one slide")
    for slide in draft.slides:
        if not (slide.heading.strip() or slide.body.strip()):
            raise ValueError(
                "output_schema violation: each slide must have a non-empty heading or body"
            )


def _build_carousel_from_mapping(parsed: Mapping[str, object]) -> CarouselDraft:
    title = str(parsed.get("title", "")).strip()
    hook = str(parsed.get("hook", "")).strip()
    caption = str(parsed.get("caption", "")).strip()

    raw_slides = parsed.get("slides", [])
    slides: list[CarouselSlide] = []
    if isinstance(raw_slides, list):
        for raw_slide in raw_slides:
            if not isinstance(raw_slide, Mapping):
                continue
            slides.append(
                CarouselSlide(
                    heading=str(raw_slide.get("heading", "")).strip(),
                    body=str(raw_slide.get("body", "")).strip(),
                    alt_text=str(raw_slide.get("alt_text", "")).strip(),
                )
            )

    raw_hashtags = parsed.get("hashtags", [])
    hashtags: list[str] = []
    if isinstance(raw_hashtags, list):
        hashtags = [str(tag).strip() for tag in raw_hashtags if isinstance(tag, str | int | float)]

    return CarouselDraft(
        title=title,
        hook=hook,
        slides=slides,
        caption=caption,
        hashtags=hashtags,
    )


def _build_graph(*, llm_registry: LLMRegistry, model_id: str):
    graph: StateGraph[LinkedInCarouselState, None, LinkedInCarouselState, LinkedInCarouselState] = (
        StateGraph(LinkedInCarouselState)
    )

    async def draft_carousel(state: LinkedInCarouselState) -> dict[str, object]:
        text = state.post.text.text.strip()
        if not text:
            raise ValueError("input_schema violation: LinkedInPost.text.text must be non-empty")

        result = await llm_registry.complete(
            model_id=model_id,
            messages=[
                ChatMessage(
                    role="system",
                    content=(
                        "You are a senior LinkedIn carousel strategist. Turn the supplied text "
                        "draft into a tight carousel outline of 5-8 slides. Return ONLY valid JSON "
                        "(no markdown fences, no commentary) with this shape: an object containing "
                        '"title", "hook", "slides" (an array of objects each with "heading" and '
                        '"body"), "caption", and "hashtags" (an array of strings). Preserve the '
                        "draft's requested output language; if the body contains a Target language "
                        "line, that requested output language overrides the language of source "
                        "article titles and summaries."
                    ),
                ),
                ChatMessage(
                    role="user",
                    content=(
                        f"post text:\n{text}\n\n"
                        f"carousel goal: {state.carousel_goal}\n"
                        f"audience: {state.audience}\n"
                        f"revision feedback: {state.review_feedback or 'none'}"
                    ),
                ),
            ],
        )
        cleaned = _strip_code_fences(result.content)
        try:
            parsed = json.loads(cleaned)
        except json.JSONDecodeError as exc:
            raise ValueError(
                f"carousel LLM output was not valid JSON: {exc}"
            ) from exc
        if not isinstance(parsed, dict):
            raise ValueError("carousel LLM output must decode to a JSON object")

        carousel_draft = _build_carousel_from_mapping(parsed)
        _validate_carousel_draft_schema(carousel_draft)
        return {"carousel_draft": carousel_draft}

    graph.add_node("draft_carousel", draft_carousel)
    graph.set_entry_point("draft_carousel")
    graph.add_edge("draft_carousel", END)
    return graph.compile()


async def run(
    input_post: LinkedInPost | Mapping[str, object],
    *,
    llm_registry: LLMRegistry,
    model_id: str = MANIFEST.model_id,
    elicitation_responses: Mapping[str, str] | None = None,
    review_feedback: str = "",
) -> AgentRunResult:
    """Return a carousel-enriched post or request the missing carousel context.

    ``carousel_goal`` and ``audience`` are intentionally elicited before the
    first candidate. A resumed run supplies them in ``elicitation_responses``;
    review feedback is carried into the drafting prompt as a revision request.
    """
    post = _parse_linkedin_post(input_post)
    answers = {key: value.strip() for key, value in (elicitation_responses or {}).items() if value.strip()}
    missing = tuple(field for field in ("carousel_goal", "audience") if not answers.get(field))
    if missing:
        return ElicitationRequest(
            thread_id="linkedin-carousel-context",
            question="What should this carousel achieve, and who is it for?",
            required_fields=missing,
        )

    graph = _build_graph(llm_registry=llm_registry, model_id=model_id)
    state = LinkedInCarouselState(
        post=post,
        carousel_goal=answers["carousel_goal"],
        audience=answers["audience"],
        review_feedback=review_feedback.strip(),
    )
    result_state = LinkedInCarouselState.model_validate(await graph.ainvoke(state.model_dump()))

    if result_state.carousel_draft is None:
        raise RuntimeError("linkedin carousel agent did not produce a result")
    enriched = LinkedInPost()
    enriched.CopyFrom(post)
    enriched.carousel.CopyFrom(result_state.carousel_draft)
    return enriched


def _parse_linkedin_post(input_post: LinkedInPost | Mapping[str, object]) -> LinkedInPost:
    if isinstance(input_post, LinkedInPost):
        parsed = LinkedInPost()
        parsed.CopyFrom(input_post)
        return parsed
    parsed = LinkedInPost()
    from google.protobuf.json_format import ParseDict

    ParseDict(dict(input_post), parsed)
    return parsed


def carousel_draft_to_mapping(carousel_draft: CarouselDraft) -> dict[str, object]:
    """Serialize CarouselDraft for debugging and transport boundaries."""
    return MessageToDict(carousel_draft, preserving_proto_field_name=True)
