"""Newsletter writer agent implementation."""

from __future__ import annotations

import uuid
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import NewsArticle, NewsList, TextDraft
from langgraph.graph import END, StateGraph
from pydantic import BaseModel, ConfigDict, Field

from harpia_agents.agents.manifest import AgentType

MANIFEST_ID = "newsletter-writer-senior"
MANIFEST_PATH = Path(__file__).resolve().parents[4] / "agents" / MANIFEST_ID / "0.1.0.yaml"
MANIFEST = AgentType.from_yaml(MANIFEST_PATH)
_OUTPUT_REQUIRED_FIELDS = tuple(MANIFEST.to_dict()["output_schema"].get("required", []))


@dataclass(frozen=True)
class ElicitationRequest:
    """Structured elicitation request for runtime suspend/resume."""

    thread_id: str
    question: str
    required_fields: tuple[str, ...]


type AgentRunResult = TextDraft | ElicitationRequest


class LLMClient(Protocol):
    """LLM client port used by the newsletter writer."""

    async def generate_draft(
        self,
        *,
        articles: Sequence[NewsArticle],
        tone: str,
        topics_to_avoid: Sequence[str],
    ) -> str: ...


class TemplateLLMClient:
    """Default deterministic implementation used when no API client is injected."""

    async def generate_draft(
        self,
        *,
        articles: Sequence[NewsArticle],
        tone: str,
        topics_to_avoid: Sequence[str],
    ) -> str:
        avoid_text = ", ".join(topics_to_avoid) if topics_to_avoid else "none"
        return (
            f"## Weekly News Brief\n"
            f"Tone: {tone}.\n"
            f"Topics to avoid: {avoid_text}.\n"
            f"Covers {len(articles)} curated stories."
        )


class NewsletterState(BaseModel):
    """LangGraph state for the newsletter writer flow."""

    model_config = ConfigDict(extra="forbid", arbitrary_types_allowed=True)

    news_list: NewsList
    elicitation_responses: dict[str, str] = Field(default_factory=dict)
    draft: TextDraft | None = None
    elicitation_request: ElicitationRequest | None = None


def _thread_id_for(news_list: NewsList) -> str:
    seed = "|".join(f"{article.title}:{article.url}" for article in news_list.articles)
    return f"elicitation-{uuid.uuid5(uuid.NAMESPACE_URL, seed or 'newsletter-writer')}"


def _topics_to_avoid(raw_topics: str | None) -> list[str]:
    if raw_topics is None:
        return []
    return [topic.strip() for topic in raw_topics.split(",") if topic.strip()]


def _validate_text_draft_schema(draft: TextDraft) -> None:
    for field_name in _OUTPUT_REQUIRED_FIELDS:
        value = getattr(draft, field_name, "")
        if not isinstance(value, str) or not value.strip():
            raise ValueError(f"output_schema violation: `{field_name}` must be a non-empty string")


def _render_body(
    *,
    llm_body: str,
    news_list: NewsList,
) -> str:
    highlights = []
    sources = []
    for idx, article in enumerate(news_list.articles, start=1):
        highlights.append(
            f"{idx}. **{article.title}** - {article.summary} ([source {idx}]({article.url}))"
        )
        sources.append(f"- [{idx}] [{article.title}]({article.url}) - {article.source}")

    return (
        f"{llm_body}\n\n"
        "### Highlights\n"
        f"{chr(10).join(highlights)}\n\n"
        "### Sources\n"
        f"{chr(10).join(sources)}"
    )


def _build_title(news_list: NewsList) -> str:
    return f"Newsletter Draft: {len(news_list.articles)} Stories"


def parse_news_list_payload(input_news_list: NewsList | Mapping[str, object]) -> NewsList:
    """Parse either a proto or mapping payload into a NewsList proto."""
    if isinstance(input_news_list, NewsList):
        parsed = NewsList()
        parsed.CopyFrom(input_news_list)
        return parsed

    parsed = NewsList()
    ParseDict(dict(input_news_list), parsed)
    return parsed


def _build_graph(*, llm_client: LLMClient):
    graph: StateGraph[NewsletterState, None, NewsletterState, NewsletterState] = StateGraph(
        NewsletterState
    )

    async def write_draft(state: NewsletterState) -> dict[str, object]:
        tone = state.elicitation_responses.get("tone", "").strip()
        if not tone:
            return {
                "elicitation_request": ElicitationRequest(
                    thread_id=_thread_id_for(state.news_list),
                    question=(
                        "Before writing the newsletter, what tone should I use "
                        "and are there topics to avoid?"
                    ),
                    required_fields=("tone", "topics_to_avoid"),
                )
            }

        topics_to_avoid = _topics_to_avoid(state.elicitation_responses.get("topics_to_avoid"))
        llm_body = await llm_client.generate_draft(
            articles=list(state.news_list.articles),
            tone=tone,
            topics_to_avoid=topics_to_avoid,
        )
        draft = TextDraft(
            title=_build_title(state.news_list),
            body=_render_body(llm_body=llm_body, news_list=state.news_list),
        )
        _validate_text_draft_schema(draft)
        return {"draft": draft}

    graph.add_node("write_draft", write_draft)
    graph.set_entry_point("write_draft")
    graph.add_edge("write_draft", END)
    return graph.compile()


async def run(
    input_news_list: NewsList | Mapping[str, object],
    *,
    llm_client: LLMClient,
    elicitation_responses: Mapping[str, str] | None = None,
) -> AgentRunResult:
    """Run newsletter writer and return either a draft or elicitation request."""
    news_list = parse_news_list_payload(input_news_list)
    graph = _build_graph(llm_client=llm_client)
    state = NewsletterState(
        news_list=news_list,
        elicitation_responses=dict(elicitation_responses or {}),
    )
    result_state = NewsletterState.model_validate(await graph.ainvoke(state.model_dump()))

    if result_state.elicitation_request is not None:
        return result_state.elicitation_request

    if result_state.draft is None:
        raise RuntimeError("newsletter writer did not produce a result")
    return result_state.draft


def news_list_to_mapping(news_list: NewsList) -> dict[str, object]:
    """Serialize NewsList for debugging and transport boundaries."""
    return MessageToDict(news_list, preserving_proto_field_name=True)
