from __future__ import annotations

import json
from dataclasses import dataclass, field

import pytest
from connectrpc.code import Code
from connectrpc.errors import ConnectError
from temporalio.exceptions import ApplicationError

from harpia_agents.temporal import worker


@dataclass
class FakeArtifactClient:
    payloads: dict[str, dict[str, object]]
    created: list[dict[str, object]] = field(default_factory=list)

    async def get_payload(self, *, tenant_id: str, artifact_id: str) -> dict[str, object]:
        return self.payloads[artifact_id]

    async def create_payload(
        self,
        *,
        tenant_id: str,
        artifact_type_key: str,
        payload: dict[str, object],
        step_execution_id: str,
        plan_execution_id: str,
    ) -> str:
        self.created.append(
            {
                "tenant_id": tenant_id,
                "artifact_type_key": artifact_type_key,
                "payload": payload,
                "step_execution_id": step_execution_id,
                "plan_execution_id": plan_execution_id,
            }
        )
        return f"artifact-{len(self.created)}"


@pytest.mark.asyncio
async def test_run_newsletter_agent_loads_news_artifact_and_persists_text_draft(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = FakeArtifactClient(
        payloads={
            "news-1": {
                "articles": [
                    {
                        "title": "Final match",
                        "url": "https://example.com/match",
                        "summary": "A decisive game.",
                        "source": "Sports",
                        "publishedAt": "2026-06-25T12:00:00Z",
                    }
                ]
            }
        }
    )

    async def fake_run_registered_agent(*args, **kwargs):
        from harpia.artifacts.v1.artifacts_pb2 import TextDraft

        assert kwargs["input_payload"].articles[0].title == "Final match"
        assert kwargs["elicitation_responses"]["tone"] == "analytical"
        assert kwargs["elicitation_responses"]["topic"] == "retail growth"
        assert kwargs["elicitation_responses"]["language"] == "en-US"
        assert kwargs["elicitation_responses"]["audience"] == "founders"
        assert kwargs["elicitation_responses"]["topics_to_avoid"] == "rumors"
        return TextDraft(title="Newsletter", body="Draft body")

    monkeypatch.setattr(worker, "ArtifactPayloadClient", lambda: fake)
    monkeypatch.setattr(worker, "run_registered_agent", fake_run_registered_agent)

    result = await worker.run_agent_activity(
        {
            "tenant_id": "00000000-0000-4000-8000-000000000001",
            "plan_execution_id": "plan-execution-write",
            "step_execution_id": "step-write",
            "output_artifact_type_key": "harpia.artifacts.v1.TextDraft",
            "executor_installation_snapshot": {
                "manifest_id": "newsletter-writer-senior",
            },
            "input_artifacts": [
                {
                    "artifact_type_key": "harpia.internal.ContentPreferences",
                    "input_name": "harpia.internal.ContentPreferences",
                    "literal_json": json.dumps(
                        {
                            "tone": "analytical",
                            "topic": "retail growth",
                            "language": "en-US",
                            "audience": "founders",
                            "topics_to_avoid": "rumors",
                        }
                    ),
                },
                {
                    "artifact_type_key": "harpia.artifacts.v1.NewsList",
                    "artifact_id": "news-1",
                },
            ],
        }
    )

    assert result == {"status": "completed", "output_artifact_id": "artifact-1"}
    assert fake.created[0]["artifact_type_key"] == "harpia.artifacts.v1.TextDraft"
    assert fake.created[0]["payload"] == {"title": "Newsletter", "body": "Draft body"}
    assert fake.created[0]["plan_execution_id"] == "plan-execution-write"


@pytest.mark.asyncio
async def test_run_linkedin_agent_loads_text_draft_and_persists_linkedin_draft(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = FakeArtifactClient(
        payloads={
            "draft-1": {
                "title": "Newsletter",
                "body": "Draft body",
            }
        }
    )

    async def fake_run_registered_agent(*args, **kwargs):
        from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft

        assert kwargs["input_payload"].title == "Newsletter"
        return LinkedInPostDraft(text="LinkedIn text", hook="Newsletter", hashtags=["sports"])

    monkeypatch.setattr(worker, "ArtifactPayloadClient", lambda: fake)
    monkeypatch.setattr(worker, "run_registered_agent", fake_run_registered_agent)

    result = await worker.run_agent_activity(
        {
            "tenant_id": "00000000-0000-4000-8000-000000000001",
            "step_execution_id": "step-linkedin",
            "output_artifact_type_key": "harpia.artifacts.v1.LinkedInPostDraft",
            "executor_installation_snapshot": {
                "manifest_id": "linkedin-voice-senior",
            },
            "input_artifacts": [
                {
                    "artifact_type_key": "harpia.artifacts.v1.TextDraft",
                    "artifact_id": "draft-1",
                }
            ],
        }
    )

    assert result == {"status": "completed", "output_artifact_id": "artifact-1"}
    assert fake.created[0]["payload"] == {
        "text": "LinkedIn text",
        "hook": "Newsletter",
        "hashtags": ["sports"],
    }


@pytest.mark.asyncio
async def test_run_agent_activity_marks_llm_config_errors_non_retryable(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = FakeArtifactClient(
        payloads={
            "news-1": {
                "articles": [
                    {
                        "title": "Final match",
                        "url": "https://example.com/match",
                        "summary": "A decisive game.",
                        "source": "Sports",
                        "publishedAt": "2026-06-25T12:00:00Z",
                    }
                ]
            }
        }
    )

    async def fake_run_registered_agent(*args, **kwargs):
        raise ConnectError(
            Code.FAILED_PRECONDITION,
            'LLM_KEY_DECRYPTION_FAILED: unknown KEK version: "dev"',
        )

    monkeypatch.setattr(worker, "ArtifactPayloadClient", lambda: fake)
    monkeypatch.setattr(worker, "run_registered_agent", fake_run_registered_agent)

    with pytest.raises(ApplicationError) as err:
        await worker.run_agent_activity(
            {
                "tenant_id": "00000000-0000-4000-8000-000000000001",
                "step_execution_id": "step-write",
                "output_artifact_type_key": "harpia.artifacts.v1.TextDraft",
                "executor_installation_snapshot": {
                    "manifest_id": "newsletter-writer-senior",
                },
                "input_artifacts": [
                    {
                        "artifact_type_key": "harpia.artifacts.v1.NewsList",
                        "artifact_id": "news-1",
                    },
                ],
            }
        )

    assert err.value.non_retryable is True
    assert err.value.type == "ConnectError"
    assert "LLM_KEY_DECRYPTION_FAILED" in str(err.value)


@pytest.mark.asyncio
async def test_run_agent_activity_marks_static_garage_credentials_non_retryable(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class BrokenArtifactClient:
        async def get_payload(self, *, tenant_id: str, artifact_id: str) -> dict[str, object]:
            raise ConnectError(
                Code.UNKNOWN,
                "get garage object: get credentials: static credentials are empty",
            )

    monkeypatch.setattr(worker, "ArtifactPayloadClient", BrokenArtifactClient)

    with pytest.raises(ApplicationError) as err:
        await worker.run_agent_activity(
            {
                "tenant_id": "00000000-0000-4000-8000-000000000001",
                "step_execution_id": "step-write",
                "output_artifact_type_key": "harpia.artifacts.v1.TextDraft",
                "executor_installation_snapshot": {
                    "manifest_id": "newsletter-writer-senior",
                },
                "input_artifacts": [
                    {
                        "artifact_type_key": "harpia.artifacts.v1.NewsList",
                        "artifact_id": "news-1",
                    },
                ],
            }
        )

    assert err.value.non_retryable is True
    assert err.value.type == "ConnectError"
    assert "static credentials are empty" in str(err.value)
