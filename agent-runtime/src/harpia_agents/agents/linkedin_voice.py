"""LinkedIn voice agent implementation."""

from __future__ import annotations

from collections.abc import Mapping

from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, TextDraft
from langgraph.graph import END, StateGraph
from pydantic import BaseModel, ConfigDict

from harpia_agents.agents.manifest import AgentType, resolve_manifest_path
from harpia_agents.llm import ChatMessage, LLMRegistry

MANIFEST_ID = "linkedin-voice-senior"
MANIFEST_PATH = resolve_manifest_path(MANIFEST_ID)
_KNOWN_MODEL_IDS = tuple(model.model_id for model in LLMRegistry.default().list_models())
MANIFEST = AgentType.from_yaml(MANIFEST_PATH, model_ids=_KNOWN_MODEL_IDS)
_OUTPUT_SCHEMA = MANIFEST.to_dict()["output_schema"]
_OUTPUT_REQUIRED_FIELDS = tuple(_OUTPUT_SCHEMA.get("required", []))
_MAX_TEXT_LENGTH = int(_OUTPUT_SCHEMA.get("properties", {}).get("text", {}).get("maxLength", 3000))

type AgentRunResult = LinkedInPostDraft


class LinkedInVoiceState(BaseModel):
    """LangGraph state for the LinkedIn voice adaptation flow."""

    model_config = ConfigDict(extra="forbid", arbitrary_types_allowed=True)

    text_draft: TextDraft
    post_draft: LinkedInPostDraft | None = None


def _validate_linkedin_post_draft_schema(draft: LinkedInPostDraft) -> None:
    for field_name in _OUTPUT_REQUIRED_FIELDS:
        value = getattr(draft, field_name, "")
        if not isinstance(value, str) or not value.strip():
            raise ValueError(f"output_schema violation: `{field_name}` must be a non-empty string")
    if len(draft.text) > _MAX_TEXT_LENGTH:
        raise ValueError(
            f"output_schema violation: `text` must be at most {_MAX_TEXT_LENGTH} characters"
        )


def _build_hook(title: str) -> str:
    cleaned = title.strip()
    if not cleaned:
        return ""
    if len(cleaned) <= 120:
        return cleaned
    return cleaned[:117] + "..."


def _default_hashtags(title: str) -> list[str]:
    words = [word.strip(".,!?:;\"'()[]") for word in title.split()]
    tags = []
    for word in words:
        normalized = "".join(char for char in word if char.isalnum())
        if len(normalized) < 4:
            continue
        tag = normalized.lower()
        if tag not in tags:
            tags.append(tag)
        if len(tags) == 3:
            break
    return tags


def parse_text_draft_payload(input_text_draft: TextDraft | Mapping[str, object]) -> TextDraft:
    """Parse either a proto or mapping payload into a TextDraft proto."""
    if isinstance(input_text_draft, TextDraft):
        parsed = TextDraft()
        parsed.CopyFrom(input_text_draft)
        return parsed

    parsed = TextDraft()
    ParseDict(dict(input_text_draft), parsed)
    return parsed


def _build_graph(*, llm_registry: LLMRegistry, model_id: str):
    graph: StateGraph[LinkedInVoiceState, None, LinkedInVoiceState, LinkedInVoiceState] = (
        StateGraph(LinkedInVoiceState)
    )

    async def adapt_for_linkedin(state: LinkedInVoiceState) -> dict[str, object]:
        title = state.text_draft.title.strip()
        body = state.text_draft.body.strip()
        if not title or not body:
            raise ValueError("input_schema violation: `title` and `body` must be non-empty strings")

        result = await llm_registry.complete(
            model_id=model_id,
            messages=[
                ChatMessage(
                    role="system",
                    content=(
                        "You are a senior LinkedIn content specialist. Adapt the input into a polished "
                        "LinkedIn post while preserving factual meaning. Preserve the draft's requested "
                        "output language; if the body contains a Target language line, that requested "
                        "output language overrides the language of source article titles and summaries."
                    ),
                ),
                ChatMessage(
                    role="user",
                    content=f"title: {title}\n\nbody:\n{body}",
                ),
            ],
        )
        adapted_text = result.content
        post_draft = LinkedInPostDraft(
            hook=_build_hook(title),
            text=adapted_text.strip()[:_MAX_TEXT_LENGTH],
            hashtags=_default_hashtags(title),
        )
        _validate_linkedin_post_draft_schema(post_draft)
        return {"post_draft": post_draft}

    graph.add_node("adapt_for_linkedin", adapt_for_linkedin)
    graph.set_entry_point("adapt_for_linkedin")
    graph.add_edge("adapt_for_linkedin", END)
    return graph.compile()


async def run(
    input_text_draft: TextDraft | Mapping[str, object],
    *,
    llm_registry: LLMRegistry,
    model_id: str = MANIFEST.model_id,
) -> AgentRunResult:
    """Run LinkedIn voice adaptation and return a LinkedIn post draft."""
    text_draft = parse_text_draft_payload(input_text_draft)
    graph = _build_graph(llm_registry=llm_registry, model_id=model_id)
    state = LinkedInVoiceState(text_draft=text_draft)
    result_state = LinkedInVoiceState.model_validate(await graph.ainvoke(state.model_dump()))

    if result_state.post_draft is None:
        raise RuntimeError("linkedin voice agent did not produce a result")
    return result_state.post_draft


def text_draft_to_mapping(text_draft: TextDraft) -> dict[str, object]:
    """Serialize TextDraft for debugging and transport boundaries."""
    return MessageToDict(text_draft, preserving_proto_field_name=True)


def linkedin_post_draft_to_mapping(post_draft: LinkedInPostDraft) -> dict[str, object]:
    """Serialize LinkedInPostDraft for debugging and transport boundaries."""
    return MessageToDict(post_draft, preserving_proto_field_name=True)
