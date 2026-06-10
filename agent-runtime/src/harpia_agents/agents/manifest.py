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
from harpia.agents.v1 import agent_type_pb2


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
    ALLOWED_FIELDS = REQUIRED_FIELDS

    _ID_PATTERN = re.compile(r"^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$")
    _SEMVER_PATTERN = re.compile(
        r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
        r"(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$"
    )
    _JINJA_VARIABLE_PATTERN = re.compile(r"{{\s*([A-Za-z_][A-Za-z0-9_]*)\s*}}")

    def __init__(self, proto: agent_type_pb2.AgentType) -> None:
        self._proto = agent_type_pb2.AgentType()
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
        proto = agent_type_pb2.AgentType(
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
        proto.input_schema.CopyFrom(_mapping_to_struct(payload["input_schema"]))
        proto.output_schema.CopyFrom(_mapping_to_struct(payload["output_schema"]))
        proto.metadata.CopyFrom(_mapping_to_struct(payload["metadata"]))
        return cls(proto)

    @classmethod
    def from_proto(
        cls,
        proto: agent_type_pb2.AgentType,
        *,
        registry: ManifestReferenceRegistry | None = None,
        model_ids: Iterable[str] | None = None,
        tool_ids: Iterable[str] | None = None,
    ) -> Self:
        manifest = cls(proto)
        cls._validate_mapping_shape(manifest.to_dict(), registry, model_ids, tool_ids)
        return manifest

    def to_proto(self) -> agent_type_pb2.AgentType:
        proto = agent_type_pb2.AgentType()
        proto.CopyFrom(self._proto)
        return proto

    def to_dict(self) -> dict[str, Any]:
        return {
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
        }

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

        keys = set(payload.keys())
        missing = sorted(cls.REQUIRED_FIELDS - keys)
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
        cls._validate_mapping(payload, "input_schema", errors)
        cls._validate_mapping(payload, "output_schema", errors)
        cls._validate_mapping(payload, "metadata", errors)

        cost_estimate = payload["cost_estimate"]
        if isinstance(cost_estimate, bool) or not isinstance(cost_estimate, int | float):
            errors.append("cost_estimate must be a number")
        elif cost_estimate < 0:
            errors.append("cost_estimate must be non-negative")

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
