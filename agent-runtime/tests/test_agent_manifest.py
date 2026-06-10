from pathlib import Path

import pytest

from harpia_agents.agents import (
    AgentManifestValidationError,
    AgentType,
    ManifestReferenceRegistry,
)

REGISTRY = ManifestReferenceRegistry.from_iterables(
    model_ids=["openai-gpt-4o-mini"],
    tool_ids=["tenant-knowledge-search"],
)


def manifest_path() -> Path:
    return Path(__file__).parents[2] / "agents" / "email-drafter" / "v1.yaml"


def test_agent_manifest_loads_and_round_trips_through_proto(tmp_path: Path):
    manifest = AgentType.from_yaml(manifest_path(), registry=REGISTRY)

    assert manifest.id == "email-drafter"
    assert manifest.version == "1.0.0"
    assert manifest.display_name == "Email Drafter"
    assert list(manifest.allowed_tool_ids) == ["tenant-knowledge-search"]

    from_proto = AgentType.from_proto(manifest.to_proto(), registry=REGISTRY)
    assert from_proto.to_dict() == manifest.to_dict()

    output_path = tmp_path / "v1.yaml"
    from_proto.to_yaml(output_path)
    from_yaml = AgentType.from_yaml(output_path, registry=REGISTRY)
    assert from_yaml.to_dict() == manifest.to_dict()


def test_agent_manifest_rejects_missing_required_fields():
    with pytest.raises(AgentManifestValidationError, match="missing required fields: model_id"):
        AgentType.from_yaml_text(
            """
id: email-drafter
version: 1.0.0
display_name: Email Drafter
description: Drafts emails.
capabilities: []
system_prompt: No variables.
allowed_tool_ids: []
input_schema: {}
output_schema: {}
cost_estimate: 0.01
metadata: {}
"""
        )


def test_agent_manifest_rejects_unknown_model_id():
    text = (
        manifest_path()
        .read_text(encoding="utf-8")
        .replace(
            "model_id: openai-gpt-4o-mini",
            "model_id: unknown-model",
        )
    )

    with pytest.raises(AgentManifestValidationError, match="unknown model_id: unknown-model"):
        AgentType.from_yaml_text(text, registry=REGISTRY)


def test_agent_manifest_rejects_undefined_tool_ids():
    text = (
        manifest_path()
        .read_text(encoding="utf-8")
        .replace(
            "tenant-knowledge-search",
            "missing-tool",
            1,
        )
    )

    with pytest.raises(
        AgentManifestValidationError,
        match="undefined tool_id references: missing-tool",
    ):
        AgentType.from_yaml_text(text, registry=REGISTRY)


def test_agent_manifest_rejects_undeclared_prompt_variables():
    text = (
        manifest_path()
        .read_text(encoding="utf-8")
        .replace(
            "{{tone}}",
            "{{missing_context}}",
        )
    )

    with pytest.raises(
        AgentManifestValidationError,
        match="system_prompt references variables missing from input_schema.properties: missing_context",
    ):
        AgentType.from_yaml_text(text, registry=REGISTRY)
