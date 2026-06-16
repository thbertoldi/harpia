"""Agent runner registry keyed by manifest id."""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping

from harpia.artifacts.v1.artifacts_pb2 import NewsList

from harpia_agents.agents.newsletter_writer import (
    AgentRunResult,
    LLMClient,
    TemplateLLMClient,
)
from harpia_agents.agents.newsletter_writer import (
    run as run_newsletter_writer,
)

AgentRunner = Callable[
    [NewsList | Mapping[str, object], LLMClient, Mapping[str, str] | None],
    Awaitable[AgentRunResult],
]


async def _run_newsletter(
    input_news_list: NewsList | Mapping[str, object],
    llm_client: LLMClient,
    elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_newsletter_writer(
        input_news_list,
        llm_client=llm_client,
        elicitation_responses=elicitation_responses,
    )


_RUNNERS: dict[str, AgentRunner] = {
    "newsletter-writer-senior": _run_newsletter,
}


def has_runner(manifest_id: str) -> bool:
    return manifest_id in _RUNNERS


async def run_registered_agent(
    manifest_id: str,
    *,
    input_news_list: NewsList | Mapping[str, object],
    llm_client: LLMClient | None = None,
    elicitation_responses: Mapping[str, str] | None = None,
) -> AgentRunResult:
    try:
        runner = _RUNNERS[manifest_id]
    except KeyError as exc:
        raise ValueError(f"unsupported manifest id: {manifest_id}") from exc
    return await runner(input_news_list, llm_client or TemplateLLMClient(), elicitation_responses)
