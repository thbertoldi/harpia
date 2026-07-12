"""Internal ArtifactService client for Temporal agent activities."""

from __future__ import annotations

import json
import os

from connectrpc.errors import ConnectError
from harpia.artifacts.v1.artifacts_connect import ArtifactServiceClient
from harpia.artifacts.v1.artifacts_pb2 import (
    CreateArtifactVersionWithPayloadRequest,
    CreateArtifactWithPayloadRequest,
    GetArtifactPayloadRequest,
    GetArtifactRequest,
)


def _control_plane_internal_base_url() -> str:
    explicit = os.environ.get("HARPIA_CONTROL_PLANE_INTERNAL_URL", "").strip()
    if explicit:
        return explicit.rstrip("/")
    public = os.environ.get("HARPIA_CONTROL_PLANE_URL", "http://localhost:8080").rstrip("/")
    return f"{public}/internal"


def _default_internal_auth_token() -> str:
    configured = os.environ.get("HARPIA_INTERNAL_AUTH_TOKEN", "").strip()
    if configured:
        return configured
    if os.environ.get("HARPIA_ALLOW_DEV_AUTH", "").lower() in {"1", "true", "yes"}:
        return "dev-internal-token"
    return ""


class ArtifactPayloadClient:
    """Small wrapper around the generated internal ArtifactService client."""

    def __init__(
        self,
        client: ArtifactServiceClient | None = None,
        *,
        base_url: str | None = None,
        auth_token: str | None = None,
        timeout_ms: int = 5000,
    ) -> None:
        self._client = client or ArtifactServiceClient(
            base_url or _control_plane_internal_base_url()
        )
        self._auth_token = auth_token if auth_token is not None else _default_internal_auth_token()
        self._timeout_ms = timeout_ms

    def _headers(self, tenant_id: str) -> dict[str, str]:
        if not self._auth_token:
            raise ConnectError("HARPIA_INTERNAL_AUTH_TOKEN is required outside dev mode")
        return {
            "Authorization": f"Bearer {self._auth_token}",
            "X-Tenant-ID": tenant_id,
        }

    async def get_payload(
        self,
        *,
        tenant_id: str,
        artifact_id: str,
        artifact_version_id: str = "",
    ) -> dict[str, object]:
        request = GetArtifactPayloadRequest(tenant_id=tenant_id, artifact_id=artifact_id)
        if artifact_version_id:
            request.artifact_version_id = artifact_version_id
        response = await self._client.get_artifact_payload(
            request,
            headers=self._headers(tenant_id),
            timeout_ms=self._timeout_ms,
        )
        decoded = json.loads(response.payload_json.decode("utf-8"))
        if not isinstance(decoded, dict):
            raise ValueError("artifact payload must decode to object")
        return decoded

    async def get_current_content_hash(self, *, tenant_id: str, artifact_id: str) -> str:
        response = await self._client.get_artifact(
            GetArtifactRequest(tenant_id=tenant_id, artifact_id=artifact_id),
            headers=self._headers(tenant_id),
            timeout_ms=self._timeout_ms,
        )
        if not response.HasField("artifact") or not response.artifact.content_hash:
            raise ConnectError("get artifact response missing content hash")
        return response.artifact.content_hash

    async def create_payload(
        self,
        *,
        tenant_id: str,
        artifact_type_key: str,
        payload: dict[str, object],
        step_execution_id: str,
        plan_execution_id: str,
    ) -> str:
        response = await self._client.create_artifact_with_payload(
            CreateArtifactWithPayloadRequest(
                tenant_id=tenant_id,
                artifact_type_key=artifact_type_key,
                payload_json=json.dumps(payload).encode("utf-8"),
                step_execution_id=step_execution_id,
                plan_execution_id=plan_execution_id,
            ),
            headers=self._headers(tenant_id),
            timeout_ms=self._timeout_ms,
        )
        if not response.HasField("artifact"):
            raise ConnectError("create artifact response missing artifact")
        return response.artifact.id

    async def create_version_payload(
        self,
        *,
        tenant_id: str,
        artifact_id: str,
        expected_content_hash: str,
        payload: dict[str, object],
        edit_summary: str,
    ) -> tuple[str, str, str]:
        response = await self._client.create_artifact_version_with_payload(
            CreateArtifactVersionWithPayloadRequest(
                tenant_id=tenant_id,
                artifact_id=artifact_id,
                expected_content_hash=expected_content_hash,
                payload_json=json.dumps(payload).encode("utf-8"),
                edit_summary=edit_summary,
            ),
            headers=self._headers(tenant_id),
            timeout_ms=self._timeout_ms,
        )
        if not response.HasField("artifact") or not response.HasField("artifact_version"):
            raise ConnectError("create artifact version response missing artifact or version")
        return (
            response.artifact.id,
            response.artifact_version.id,
            response.artifact_version.content_hash,
        )
