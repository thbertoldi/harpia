import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { getTenant } from "$lib/auth";
import { listArtifactVersions } from "$lib/artifacts/artifacts";
import { editableTextFromPayload } from "$lib/artifacts/text";
import { toUserMessage } from "$lib/connect-errors";
import { artifactClient } from "$lib/rpc";

export const ssr = false;

function parsePayload(payload: Uint8Array): Record<string, unknown> {
  const decoded = new TextDecoder().decode(payload);
  const parsed = JSON.parse(decoded) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("Artifact payload is not an object");
  }
  return parsed as Record<string, unknown>;
}

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }

  try {
    const artifactResponse = await artifactClient.getArtifact({
      tenantId: tenant.id,
      artifactId: params.artifactId,
    });
    if (!artifactResponse.artifact) {
      throw error(404, "Artifact not found");
    }
    const payloadResponse = await artifactClient.getArtifactPayload({
      tenantId: tenant.id,
      artifactId: params.artifactId,
    });
    const payload = parsePayload(payloadResponse.payloadJson);
    const versions = await listArtifactVersions(tenant.id, params.artifactId);
    return {
      tenantId: tenant.id,
      artifact: artifactResponse.artifact,
      payload,
      contentHash: payloadResponse.contentHash,
      editableText: editableTextFromPayload(
        artifactResponse.artifact.artifactTypeKey,
        payload,
      ),
      versions,
    };
  } catch (err) {
    throw error(404, toUserMessage(err));
  }
};
