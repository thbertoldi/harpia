"""Agent runner registry keyed by manifest id."""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping
from typing import Any

from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, NewsList, TextDraft

from harpia_agents.agents.linkedin_voice import (
    LLMClient as LinkedInLLMClient,
)
from harpia_agents.agents.linkedin_voice import (
    TemplateLLMClient as LinkedInTemplateLLMClient,
)
from harpia_agents.agents.linkedin_voice import (
    run as run_linkedin_voice,
)
from harpia_agents.agents.newsletter_writer import (
    AgentRunResult as NewsletterAgentRunResult,
)
from harpia_agents.agents.newsletter_writer import (
    LLMClient as NewsletterLLMClient,
)
from harpia_agents.agents.newsletter_writer import (
    TemplateLLMClient as NewsletterTemplateLLMClient,
)
from harpia_agents.agents.newsletter_writer import (
    run as run_newsletter_writer,
)

type AgentInput = NewsList | TextDraft | Mapping[str, object]
type AgentRunResult = NewsletterAgentRunResult | LinkedInPostDraft

AgentRunner = Callable[
    [AgentInput, Any, Mapping[str, str] | None],
    Awaitable[AgentRunResult],
]


async def _run_newsletter(
    input_payload: AgentInput,
    llm_client: NewsletterLLMClient,
    elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_newsletter_writer(
        input_payload,
        llm_client=llm_client,
        elicitation_responses=elicitation_responses,
    )


async def _run_linkedin(
    input_payload: AgentInput,
    llm_client: LinkedInLLMClient,
    _elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_linkedin_voice(input_payload, llm_client=llm_client)


_RUNNERS: dict[str, AgentRunner] = {
    "newsletter-writer-senior": _run_newsletter,
    "linkedin-voice-senior": _run_linkedin,
}

_DEFAULT_LLM_CLIENTS: dict[str, Any] = {
    "newsletter-writer-senior": NewsletterTemplateLLMClient(),
    "linkedin-voice-senior": LinkedInTemplateLLMClient(),
}


def has_runner(manifest_id: str) -> bool:
    return manifest_id in _RUNNERS


async def run_registered_agent(
    manifest_id: str,
    *,
    input_payload: AgentInput | None = None,
    input_news_list: AgentInput | None = None,
    llm_client: NewsletterLLMClient | LinkedInLLMClient | None = None,
    elicitation_responses: Mapping[str, str] | None = None,
) -> AgentRunResult:
    try:
        runner = _RUNNERS[manifest_id]
    except KeyError as exc:
        raise ValueError(f"unsupported manifest id: {manifest_id}") from exc

    resolved_input = input_payload if input_payload is not None else input_news_list
    if resolved_input is None:
        raise ValueError("input_payload is required")

    return await runner(
        resolved_input,
        llm_client or _DEFAULT_LLM_CLIENTS[manifest_id],
        elicitation_responses,
    )
