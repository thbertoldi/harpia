"""LinkedIn content specialist — multi-capability agent (ADR-018 Option B).

A single manifest declares two capabilities, each its own
(input -> output) contract with its own prompt and schemas:

- ``linkedin-content-adaptation`` (TextDraft -> LinkedInPostDraft)
- ``carousel-authoring``           (TextDraft -> CarouselDraft)

The runner is **capability-driven**: it selects the active capability by the
step's expected output type and reuses the parsing/validation helpers from the
single-capability voice/carousel modules rather than duplicating them.
"""

from __future__ import annotations

import json
from collections.abc import Mapping

from harpia.artifacts.v1.artifacts_pb2 import (
    CarouselDraft,
    LinkedInPostDraft,
    TextDraft,
)

from harpia_agents.agents.linkedin_carousel import (
    _build_carousel_from_mapping,
    _strip_code_fences,
    _validate_carousel_draft_schema,
)
from harpia_agents.agents.linkedin_voice import (
    _MAX_TEXT_LENGTH,
    _build_hook,
    _default_hashtags,
    _validate_linkedin_post_draft_schema,
    parse_text_draft_payload,
)
from harpia_agents.agents.manifest import AgentType, resolve_manifest_path
from harpia_agents.llm import ChatMessage, LLMRegistry

MANIFEST_ID = "linkedin-content-specialist"
MANIFEST_PATH = resolve_manifest_path(MANIFEST_ID)
_KNOWN_MODEL_IDS = tuple(model.model_id for model in LLMRegistry.default().list_models())
MANIFEST = AgentType.from_yaml(MANIFEST_PATH, model_ids=_KNOWN_MODEL_IDS)

type AgentRunResult = LinkedInPostDraft | CarouselDraft


def _specs_by_output() -> dict[str, dict[str, object]]:
    """Capability specs keyed by their declared ``artifact_output_type``."""
    return {
        spec["artifact_output_type"]: spec
        for spec in MANIFEST.to_dict()["capability_specs"]
    }


async def run(
    input_text_draft: TextDraft | Mapping[str, object],
    *,
    llm_registry: LLMRegistry,
    model_id: str = MANIFEST.model_id,
    output_artifact_type_key: str = "",
) -> AgentRunResult:
    """Run the capability matching ``output_artifact_type_key``.

    The step's expected output type selects which capability (and therefore
    which system prompt + parsing path) is active for this run.
    """
    spec = _specs_by_output().get(output_artifact_type_key)
    if spec is None:
        raise ValueError(
            f"{MANIFEST_ID} has no capability producing {output_artifact_type_key!r}"
        )

    text_draft = parse_text_draft_payload(input_text_draft)
    title = text_draft.title.strip()
    body = text_draft.body.strip()
    if not title or not body:
        raise ValueError(
            "input_schema violation: `title` and `body` must be non-empty strings"
        )

    result = await llm_registry.complete(
        model_id=model_id,
        messages=[
            ChatMessage(role="system", content=str(spec["system_prompt"])),
            ChatMessage(role="user", content=f"title: {title}\n\nbody:\n{body}"),
        ],
    )

    output_type = spec["artifact_output_type"]
    if output_type == "harpia.artifacts.v1.LinkedInPostDraft":
        post_draft = LinkedInPostDraft(
            hook=_build_hook(title),
            text=result.content.strip()[:_MAX_TEXT_LENGTH],
            hashtags=_default_hashtags(title),
        )
        _validate_linkedin_post_draft_schema(post_draft)
        return post_draft

    if output_type == "harpia.artifacts.v1.CarouselDraft":
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
        return carousel_draft

    raise ValueError(f"unsupported capability output type: {output_type!r}")
