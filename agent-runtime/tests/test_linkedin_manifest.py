from pathlib import Path

from harpia_agents.agents import AgentType


def test_linkedin_voice_manifest_loads() -> None:
    manifest_path = Path(__file__).parents[2] / "agents" / "linkedin-voice-senior" / "0.1.0.yaml"
    manifest = AgentType.from_yaml(manifest_path)
    metadata = manifest.to_dict()["metadata"]
    output_schema = manifest.to_dict()["output_schema"]

    assert manifest.id == "linkedin-voice-senior"
    assert manifest.version == "0.1.0"
    assert list(manifest.allowed_tool_ids) == []
    assert metadata["artifact_input_type"] == "harpia.artifacts.v1.TextDraft"
    assert metadata["artifact_output_type"] == "harpia.artifacts.v1.LinkedInPostDraft"
    assert metadata["supports_elicitation"] is False
    assert output_schema["properties"]["text"]["maxLength"] == 3000
