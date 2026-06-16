from pathlib import Path

from harpia_agents.agents import AgentType


def test_newsletter_writer_manifest_loads() -> None:
    manifest_path = Path(__file__).parents[2] / "agents" / "newsletter-writer-senior" / "0.1.0.yaml"
    manifest = AgentType.from_yaml(manifest_path)
    metadata = manifest.to_dict()["metadata"]

    assert manifest.id == "newsletter-writer-senior"
    assert manifest.version == "0.1.0"
    assert list(manifest.allowed_tool_ids) == []
    assert metadata["artifact_input_type"] == "harpia.artifacts.v1.NewsList"
    assert metadata["artifact_output_type"] == "harpia.artifacts.v1.TextDraft"
