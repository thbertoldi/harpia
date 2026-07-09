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


# --- Multi-capability manifests (ADR-018 Option B) ------------------------------


def _multi_capability_manifest_text() -> str:
    return """
id: linkedin-content-senior
version: 0.1.0
display_name: LinkedIn Content Senior
description: Multi-capability LinkedIn content agent.
capabilities:
  - linkedin-content-adaptation
  - linkedin-carousel
model_id: deepseek-v4-flash
system_prompt: Default fallback prompt.
allowed_tool_ids: []
input_schema: {}
output_schema: {}
cost_estimate: 0.02
metadata:
  artifact_input_type: harpia.artifacts.v1.TextDraft
  artifact_output_type: harpia.artifacts.v1.LinkedInPostDraft
tier: senior
capability_specs:
  - id: linkedin-content-adaptation
    artifact_input_type: harpia.artifacts.v1.TextDraft
    artifact_output_type: harpia.artifacts.v1.LinkedInPostDraft
    system_prompt: Adapt {{draft}} into a LinkedIn post.
    input_schema:
      type: object
      properties:
        draft:
          type: string
    output_schema:
      type: object
      properties:
        body:
          type: string
  - id: linkedin-carousel
    artifact_input_type: harpia.artifacts.v1.TextDraft
    artifact_output_type: harpia.artifacts.v1.CarouselDraft
    system_prompt: Turn {{draft}} into a carousel.
    input_schema:
      type: object
      properties:
        draft:
          type: string
    output_schema:
      type: object
      properties:
        slides:
          type: array
"""


def test_multi_capability_manifest_parses_and_round_trips():
    manifest = AgentType.from_yaml_text(_multi_capability_manifest_text())

    assert manifest.tier == "senior"
    as_dict = manifest.to_dict()
    assert as_dict["tier"] == "senior"

    specs = as_dict["capability_specs"]
    assert [spec["id"] for spec in specs] == [
        "linkedin-content-adaptation",
        "linkedin-carousel",
    ]
    assert specs[0]["artifact_input_type"] == "harpia.artifacts.v1.TextDraft"
    assert specs[0]["artifact_output_type"] == "harpia.artifacts.v1.LinkedInPostDraft"
    assert specs[1]["artifact_output_type"] == "harpia.artifacts.v1.CarouselDraft"
    assert specs[0]["input_schema"] == {
        "type": "object",
        "properties": {"draft": {"type": "string"}},
    }

    # Round-trips losslessly through proto and YAML.
    from_proto = AgentType.from_proto(manifest.to_proto())
    assert from_proto.to_dict() == as_dict

    dumped = manifest.to_yaml()
    from_yaml = AgentType.from_yaml_text(dumped)
    assert from_yaml.to_dict() == as_dict


def test_multi_capability_manifest_rejects_spec_prompt_variable_not_in_its_schema():
    text = _multi_capability_manifest_text().replace(
        "Adapt {{draft}} into a LinkedIn post.",
        "Adapt {{missing_context}} into a LinkedIn post.",
    )
    with pytest.raises(
        AgentManifestValidationError,
        match=(
            r"capability_specs\[0\]\.system_prompt references variables missing "
            r"from input_schema\.properties: missing_context"
        ),
    ):
        AgentType.from_yaml_text(text)


def test_multi_capability_manifest_rejects_duplicate_capability_ids():
    text = _multi_capability_manifest_text().replace(
        "  - id: linkedin-carousel",
        "  - id: linkedin-content-adaptation",
    )
    with pytest.raises(
        AgentManifestValidationError,
        match=r"capability_specs\[1\]\.id is duplicate: linkedin-content-adaptation",
    ):
        AgentType.from_yaml_text(text)


def test_legacy_single_capability_manifest_still_round_trips():
    """Backward-compat guard: a legacy single-capability manifest (no
    capability_specs) must continue to load and round-trip unchanged."""
    manifest = AgentType.from_yaml(manifest_path(), registry=REGISTRY)

    as_dict = manifest.to_dict()
    # Legacy manifests carry no capability_specs and an empty tier default.
    assert "capability_specs" not in as_dict
    assert as_dict["tier"] == ""

    from_proto = AgentType.from_proto(manifest.to_proto(), registry=REGISTRY)
    assert from_proto.to_dict() == as_dict

    dumped = manifest.to_yaml()
    from_yaml = AgentType.from_yaml_text(dumped, registry=REGISTRY)
    assert from_yaml.to_dict() == as_dict
