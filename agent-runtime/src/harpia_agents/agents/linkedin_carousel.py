"""LinkedIn carousel agent implementation."""

from __future__ import annotations

import json
from collections.abc import Mapping

from google.protobuf.json_format import MessageToDict
from harpia.artifacts.v1.artifacts_pb2 import CarouselDraft, CarouselSlide, TextDraft
from langgraph.graph import END, StateGraph
from pydantic import BaseModel, ConfigDict

from harpia_agents.agents.linkedin_voice import parse_text_draft_payload
from harpia_agents.agents.manifest import AgentType, resolve_manifest_path
from harpia_agents.llm import ChatMessage, LLMRegistry

MANIFEST_ID = "linkedin-carousel-senior"
MANIFEST_PATH = resolve_manifest_path(MANIFEST_ID)
_KNOWN_MODEL_IDS = tuple(model.model_id for model in LLMRegistry.default().list_models())
MANIFEST = AgentType.from_yaml(MANIFEST_PATH, model_ids=_KNOWN_MODEL_IDS)
_OUTPUT_SCHEMA = MANIFEST.to_dict()["output_schema"]
_OUTPUT_REQUIRED_FIELDS = tuple(_OUTPUT_SCHEMA.get("required", []))

type AgentRunResult = CarouselDraft


class LinkedInCarouselState(BaseModel):
    """LangGraph state for the LinkedIn carousel drafting flow."""

    model_config = ConfigDict(extra="forbid", arbitrary_types_allowed=True)

    text_draft: TextDraft
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
    for field_name in _OUTPUT_REQUIRED_FIELDS:
        if field_name == "slides":
            continue
        value = getattr(draft, field_name, "")
        if not isinstance(value, str) or not value.strip():
            raise ValueError(
                f"output_schema violation: `{field_name}` must be a non-empty string"
            )
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
        title = state.text_draft.title.strip()
        body = state.text_draft.body.strip()
        if not title or not body:
            raise ValueError(
                "input_schema violation: `title` and `body` must be non-empty strings"
            )

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
                    content=f"title: {title}\n\nbody:\n{body}",
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
    input_text_draft: TextDraft | Mapping[str, object],
    *,
    llm_registry: LLMRegistry,
    model_id: str = MANIFEST.model_id,
) -> AgentRunResult:
    """Run LinkedIn carousel drafting and return a carousel draft."""
    text_draft = parse_text_draft_payload(input_text_draft)
    graph = _build_graph(llm_registry=llm_registry, model_id=model_id)
    state = LinkedInCarouselState(text_draft=text_draft)
    result_state = LinkedInCarouselState.model_validate(await graph.ainvoke(state.model_dump()))

    if result_state.carousel_draft is None:
        raise RuntimeError("linkedin carousel agent did not produce a result")
    return result_state.carousel_draft


def carousel_draft_to_mapping(carousel_draft: CarouselDraft) -> dict[str, object]:
    """Serialize CarouselDraft for debugging and transport boundaries."""
    return MessageToDict(carousel_draft, preserving_proto_field_name=True)
