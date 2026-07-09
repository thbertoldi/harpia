"""YAML boundary for canonical agent type manifests."""

from __future__ import annotations

import re
from collections.abc import Iterable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Self

import yaml
from google.protobuf.json_format import MessageToDict
from google.protobuf.struct_pb2 import Struct
from harpia.agents.v1 import agents_pb2


class AgentManifestValidationError(ValueError):
    """Raised when an agent manifest does not satisfy the canonical schema."""


@dataclass(frozen=True)
class ManifestReferenceRegistry:
    """Known IDs from the model and tool registries used during validation."""

    model_ids: frozenset[str] = field(default_factory=frozenset)
    tool_ids: frozenset[str] = field(default_factory=frozenset)

    @classmethod
    def from_iterables(
        cls,
        *,
        model_ids: Iterable[str] = (),
        tool_ids: Iterable[str] = (),
    ) -> Self:
        return cls(model_ids=frozenset(model_ids), tool_ids=frozenset(tool_ids))


class AgentType:
    """Canonical agent manifest with YAML/protobuf conversion helpers."""

    REQUIRED_FIELDS = frozenset(
        {
            "id",
            "version",
            "display_name",
            "description",
            "capabilities",
            "model_id",
            "system_prompt",
            "allowed_tool_ids",
            "input_schema",
            "output_schema",
            "cost_estimate",
            "metadata",
        }
    )
    # Optional fields for multi-capability manifests (ADR-018 Option B). Legacy
    # single-capability manifests omit both and stay valid.
    OPTIONAL_FIELDS = frozenset({"tier", "capability_specs"})
    ALLOWED_FIELDS = REQUIRED_FIELDS | OPTIONAL_FIELDS

    _ID_PATTERN = re.compile(r"^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$")
    _SEMVER_PATTERN = re.compile(
        r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
        r"(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$"
    )
    _JINJA_VARIABLE_PATTERN = re.compile(r"{{\s*([A-Za-z_][A-Za-z0-9_]*)\s*}}")

    def __init__(self, proto: agents_pb2.AgentType) -> None:
        self._proto = agents_pb2.AgentType()
        self._proto.CopyFrom(proto)

    def __getattr__(self, name: str) -> Any:
        return getattr(self._proto, name)

    @classmethod
    def from_yaml(
        cls,
        path: str | Path,
        *,
        registry: ManifestReferenceRegistry | None = None,
        model_ids: Iterable[str] | None = None,
        tool_ids: Iterable[str] | None = None,
    ) -> Self:
        manifest_path = Path(path)
        payload = yaml.safe_load(manifest_path.read_text(encoding="utf-8"))
        return cls.from_mapping(
            payload,
            registry=registry,
            model_ids=model_ids,
            tool_ids=tool_ids,
        )

    @classmethod
    def from_yaml_text(
        cls,
        text: str,
        *,
        registry: ManifestReferenceRegistry | None = None,
        model_ids: Iterable[str] | None = None,
        tool_ids: Iterable[str] | None = None,
    ) -> Self:
        payload = yaml.safe_load(text)
        return cls.from_mapping(
            payload,
            registry=registry,
            model_ids=model_ids,
            tool_ids=tool_ids,
        )

    @classmethod
    def from_mapping(
        cls,
        payload: Any,
        *,
        registry: ManifestReferenceRegistry | None = None,
        model_ids: Iterable[str] | None = None,
        tool_ids: Iterable[str] | None = None,
    ) -> Self:
        cls._validate_mapping_shape(payload, registry, model_ids, tool_ids)
        proto = agents_pb2.AgentType(
            id=payload["id"],
            version=payload["version"],
            display_name=payload["display_name"],
            description=payload["description"],
            capabilities=list(payload["capabilities"]),
            model_id=payload["model_id"],
            system_prompt=payload["system_prompt"],
            allowed_tool_ids=list(payload["allowed_tool_ids"]),
            cost_estimate=float(payload["cost_estimate"]),
        )
        proto.input_schema.CopyFrom(_mapping_to_struct(payload.get("input_schema", {})))
        proto.output_schema.CopyFrom(_mapping_to_struct(payload.get("output_schema", {})))
        proto.metadata.CopyFrom(_mapping_to_struct(payload["metadata"]))

        # Additive multi-capability fields (ADR-018 Option B). Both are optional;
        # legacy single-capability manifests omit them.
        if "tier" in payload:
            proto.tier = str(payload["tier"])
        if "capability_specs" in payload:
            for spec in payload["capability_specs"]:
                capability = proto.capability_specs.add(
                    id=spec["id"],
                    artifact_input_type=spec["artifact_input_type"],
                    artifact_output_type=spec["artifact_output_type"],
                    system_prompt=spec["system_prompt"],
                )
                capability.input_schema.CopyFrom(_mapping_to_struct(spec["input_schema"]))
                capability.output_schema.CopyFrom(_mapping_to_struct(spec["output_schema"]))
        return cls(proto)

    @classmethod
    def from_proto(
        cls,
        proto: agents_pb2.AgentType,
        *,
        registry: ManifestReferenceRegistry | None = None,
        model_ids: Iterable[str] | None = None,
        tool_ids: Iterable[str] | None = None,
    ) -> Self:
        manifest = cls(proto)
        cls._validate_mapping_shape(manifest.to_dict(), registry, model_ids, tool_ids)
        return manifest

    def to_proto(self) -> agents_pb2.AgentType:
        proto = agents_pb2.AgentType()
        proto.CopyFrom(self._proto)
        return proto

    def to_dict(self) -> dict[str, Any]:
        result: dict[str, Any] = {
            "id": self._proto.id,
            "version": self._proto.version,
            "display_name": self._proto.display_name,
            "description": self._proto.description,
            "capabilities": list(self._proto.capabilities),
            "model_id": self._proto.model_id,
            "system_prompt": self._proto.system_prompt,
            "allowed_tool_ids": list(self._proto.allowed_tool_ids),
            "input_schema": _struct_to_mapping(self._proto.input_schema),
            "output_schema": _struct_to_mapping(self._proto.output_schema),
            "cost_estimate": self._proto.cost_estimate,
            "metadata": _struct_to_mapping(self._proto.metadata),
            "tier": self._proto.tier,
        }
        # Only include capability_specs when non-empty so legacy single-capability
        # manifests round-trip cleanly without an empty list.
        if self._proto.capability_specs:
            result["capability_specs"] = [
                {
                    "id": spec.id,
                    "artifact_input_type": spec.artifact_input_type,
                    "artifact_output_type": spec.artifact_output_type,
                    "system_prompt": spec.system_prompt,
                    "input_schema": _struct_to_mapping(spec.input_schema),
                    "output_schema": _struct_to_mapping(spec.output_schema),
                }
                for spec in self._proto.capability_specs
            ]
        return result

    def to_yaml(self, path: str | Path | None = None) -> str:
        text = yaml.safe_dump(
            self.to_dict(),
            sort_keys=False,
            allow_unicode=False,
        )
        if path is not None:
            Path(path).write_text(text, encoding="utf-8")
        return text

    @classmethod
    def _validate_mapping_shape(
        cls,
        payload: Any,
        registry: ManifestReferenceRegistry | None,
        model_ids: Iterable[str] | None,
        tool_ids: Iterable[str] | None,
    ) -> None:
        errors: list[str] = []
        if not isinstance(payload, Mapping):
            raise AgentManifestValidationError("manifest must be a YAML mapping")

        # Multi-capability manifests (ADR-018 Option B) carry their (input -> output)
        # contracts in capability_specs, so the top-level input_schema/output_schema
        # are optional in that mode.
        multi_capability = "capability_specs" in payload
        required = cls.REQUIRED_FIELDS
        if multi_capability:
            required = cls.REQUIRED_FIELDS - {"input_schema", "output_schema"}

        keys = set(payload.keys())
        missing = sorted(required - keys)
        unknown = sorted(keys - cls.ALLOWED_FIELDS)
        if missing:
            errors.append(f"missing required fields: {', '.join(missing)}")
        if unknown:
            errors.append(f"unknown fields: {', '.join(unknown)}")
        if errors:
            raise AgentManifestValidationError("; ".join(errors))

        cls._validate_string(payload, "id", errors)
        cls._validate_string(payload, "version", errors)
        cls._validate_string(payload, "display_name", errors)
        cls._validate_string(payload, "description", errors)
        cls._validate_string(payload, "model_id", errors)
        cls._validate_string(payload, "system_prompt", errors)

        if isinstance(payload["id"], str) and not cls._ID_PATTERN.fullmatch(payload["id"]):
            errors.append("id must be stable kebab-case")
        if isinstance(payload["version"], str) and not cls._SEMVER_PATTERN.fullmatch(
            payload["version"]
        ):
            errors.append("version must be semver")

        cls._validate_string_list(payload, "capabilities", errors)
        cls._validate_string_list(payload, "allowed_tool_ids", errors)
        # Top-level input_schema/output_schema are always present on legacy
        # manifests; validate them when present and tolerate their absence in
        # multi-capability mode.
        if "input_schema" in payload:
            cls._validate_mapping(payload, "input_schema", errors)
        if "output_schema" in payload:
            cls._validate_mapping(payload, "output_schema", errors)
        cls._validate_mapping(payload, "metadata", errors)

        cost_estimate = payload["cost_estimate"]
        if isinstance(cost_estimate, bool) or not isinstance(cost_estimate, int | float):
            errors.append("cost_estimate must be a number")
        elif cost_estimate < 0:
            errors.append("cost_estimate must be non-negative")

        if multi_capability:
            # The per-spec prompt-variable check replaces the legacy top-level one.
            cls._validate_capability_specs(payload, errors)
        else:
            cls._validate_prompt_variables(payload, errors)
        cls._validate_registry_references(payload, registry, model_ids, tool_ids, errors)

        if errors:
            raise AgentManifestValidationError("; ".join(errors))

    @staticmethod
    def _validate_string(payload: Mapping[str, Any], field_name: str, errors: list[str]) -> None:
        value = payload[field_name]
        if not isinstance(value, str) or not value.strip():
            errors.append(f"{field_name} must be a non-empty string")

    @staticmethod
    def _validate_string_list(
        payload: Mapping[str, Any],
        field_name: str,
        errors: list[str],
    ) -> None:
        value = payload[field_name]
        if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
            errors.append(f"{field_name} must be a list of strings")
            return
        duplicates = sorted({item for item in value if value.count(item) > 1})
        if duplicates:
            errors.append(f"{field_name} contains duplicates: {', '.join(duplicates)}")

    @staticmethod
    def _validate_mapping(payload: Mapping[str, Any], field_name: str, errors: list[str]) -> None:
        if not isinstance(payload[field_name], Mapping):
            errors.append(f"{field_name} must be a mapping")

    @classmethod
    def _validate_prompt_variables(
        cls,
        payload: Mapping[str, Any],
        errors: list[str],
    ) -> None:
        if not isinstance(payload["system_prompt"], str) or not isinstance(
            payload["input_schema"], Mapping
        ):
            return

        variables = set(cls._JINJA_VARIABLE_PATTERN.findall(payload["system_prompt"]))
        properties = payload["input_schema"].get("properties", {})
        declared = set(properties.keys()) if isinstance(properties, Mapping) else set()
        undeclared = sorted(variables - declared)
        if undeclared:
            errors.append(
                "system_prompt references variables missing from input_schema.properties: "
                + ", ".join(undeclared)
            )

    @classmethod
    def _validate_capability_specs(
        cls,
        payload: Mapping[str, Any],
        errors: list[str],
    ) -> None:
        """Validate capability_specs for multi-capability manifests (ADR-018 Option B).

        Each spec carries its own (input -> output) contract with its own prompt and
        schemas; the prompt-variable check is scoped per spec instead of at the top
        level.
        """
        specs = payload["capability_specs"]
        if not isinstance(specs, list) or not specs:
            errors.append("capability_specs must be a non-empty list")
            return

        seen_ids: set[str] = set()
        for index, spec in enumerate(specs):
            prefix = f"capability_specs[{index}]"
            if not isinstance(spec, Mapping):
                errors.append(f"{prefix} must be a mapping")
                continue

            spec_id = spec.get("id")
            if not isinstance(spec_id, str) or not spec_id.strip():
                errors.append(f"{prefix}.id must be a non-empty string")
            elif not cls._ID_PATTERN.fullmatch(spec_id):
                errors.append(f"{prefix}.id must be stable kebab-case")
            elif spec_id in seen_ids:
                errors.append(f"{prefix}.id is duplicate: {spec_id}")
            else:
                seen_ids.add(spec_id)

            for field_name in ("artifact_input_type", "artifact_output_type"):
                value = spec.get(field_name)
                if not isinstance(value, str) or not value.strip():
                    errors.append(f"{prefix}.{field_name} must be a non-empty string")

            system_prompt = spec.get("system_prompt")
            if not isinstance(system_prompt, str) or not system_prompt.strip():
                errors.append(f"{prefix}.system_prompt must be a non-empty string")

            for field_name in ("input_schema", "output_schema"):
                if not isinstance(spec.get(field_name), Mapping):
                    errors.append(f"{prefix}.{field_name} must be a mapping")

            # Scope the Jinja-variable check to this spec's own input_schema.
            if isinstance(system_prompt, str) and isinstance(
                spec.get("input_schema"), Mapping
            ):
                variables = set(cls._JINJA_VARIABLE_PATTERN.findall(system_prompt))
                properties = spec["input_schema"].get("properties", {})
                declared = (
                    set(properties.keys()) if isinstance(properties, Mapping) else set()
                )
                undeclared = sorted(variables - declared)
                if undeclared:
                    errors.append(
                        f"{prefix}.system_prompt references variables missing from "
                        f"input_schema.properties: {', '.join(undeclared)}"
                    )

    @staticmethod
    def _validate_registry_references(
        payload: Mapping[str, Any],
        registry: ManifestReferenceRegistry | None,
        model_ids: Iterable[str] | None,
        tool_ids: Iterable[str] | None,
        errors: list[str],
    ) -> None:
        known_models = frozenset(model_ids) if model_ids is not None else None
        known_tools = frozenset(tool_ids) if tool_ids is not None else None
        if registry is not None:
            known_models = registry.model_ids
            known_tools = registry.tool_ids

        if known_models is not None and payload["model_id"] not in known_models:
            errors.append(f"unknown model_id: {payload['model_id']}")

        if known_tools is not None and isinstance(payload["allowed_tool_ids"], list):
            unknown_tools = sorted(set(payload["allowed_tool_ids"]) - known_tools)
            if unknown_tools:
                errors.append(f"undefined tool_id references: {', '.join(unknown_tools)}")


def _mapping_to_struct(value: Mapping[str, Any]) -> Struct:
    struct = Struct()
    struct.update(dict(value))
    return struct


def _struct_to_mapping(value: Struct) -> dict[str, Any]:
    return dict(MessageToDict(value, preserving_proto_field_name=True))


def resolve_manifest_path(manifest_id: str, version: str = "0.1.0") -> Path:
    """Locate agents/{id}/{version}.yaml from repo root or container /app layout."""
    relative = Path("agents") / manifest_id / f"{version}.yaml"
    start = Path(__file__).resolve().parent
    for base in (start, *start.parents):
        candidate = base / relative
        if candidate.is_file():
            return candidate
    raise FileNotFoundError(f"agent manifest not found: {relative.as_posix()}")
