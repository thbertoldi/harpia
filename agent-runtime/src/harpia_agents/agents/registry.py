"""Agent runner registry keyed by manifest id."""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping

from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, NewsList, TextDraft

from harpia_agents.agents.linkedin_voice import MANIFEST as LINKEDIN_MANIFEST
from harpia_agents.agents.linkedin_voice import (
    run as run_linkedin_voice,
)
from harpia_agents.agents.newsletter_writer import MANIFEST as NEWSLETTER_MANIFEST
from harpia_agents.agents.newsletter_writer import (
    AgentRunResult as NewsletterAgentRunResult,
)
from harpia_agents.agents.newsletter_writer import (
    run as run_newsletter_writer,
)
from harpia_agents.llm import LLMRegistry

type AgentInput = NewsList | TextDraft | Mapping[str, object]
type AgentRunResult = NewsletterAgentRunResult | LinkedInPostDraft

AgentRunner = Callable[
    [AgentInput, LLMRegistry, Mapping[str, str] | None],
    Awaitable[AgentRunResult],
]


async def _run_newsletter(
    input_payload: AgentInput,
    llm_registry: LLMRegistry,
    elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_newsletter_writer(
        input_payload,
        llm_registry=llm_registry,
        model_id=NEWSLETTER_MANIFEST.model_id,
        elicitation_responses=elicitation_responses,
    )


async def _run_linkedin(
    input_payload: AgentInput,
    llm_registry: LLMRegistry,
    _elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_linkedin_voice(
        input_payload,
        llm_registry=llm_registry,
        model_id=LINKEDIN_MANIFEST.model_id,
    )


_RUNNERS: dict[str, AgentRunner] = {
    "newsletter-writer-senior": _run_newsletter,
    "linkedin-voice-senior": _run_linkedin,
}

_MANIFEST_MODEL_IDS: dict[str, str] = {
    "newsletter-writer-senior": NEWSLETTER_MANIFEST.model_id,
    "linkedin-voice-senior": LINKEDIN_MANIFEST.model_id,
}


def has_runner(manifest_id: str) -> bool:
    return manifest_id in _RUNNERS


async def run_registered_agent(
    manifest_id: str,
    *,
    input_payload: AgentInput | None = None,
    input_news_list: AgentInput | None = None,
    llm_registry: LLMRegistry | None = None,
    elicitation_responses: Mapping[str, str] | None = None,
) -> AgentRunResult:
    try:
        runner = _RUNNERS[manifest_id]
    except KeyError as exc:
        raise ValueError(f"unsupported manifest id: {manifest_id}") from exc

    resolved_input = input_payload if input_payload is not None else input_news_list
    if resolved_input is None:
        raise ValueError("input_payload is required")
    registry = llm_registry or LLMRegistry.default()
    registry.resolve(_MANIFEST_MODEL_IDS[manifest_id])

    return await runner(
        resolved_input,
        registry,
        elicitation_responses,
    )
