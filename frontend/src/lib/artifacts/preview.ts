import { create } from "@bufbuild/protobuf";
import {
  PreviewArtifactRequestSchema,
  type PreviewArtifactRequest,
  type PreviewArtifactResponse,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import { artifactClient } from "$lib/rpc";

export type ArtifactPreviewKind = "text" | "list" | "json" | "empty";

export type FormattedArtifactPreview = {
  kind: ArtifactPreviewKind;
  text: string;
  listSummary?: {
    articleCount: number;
    titles: string[];
  };
};

export function buildPreviewArtifactRequest(
  tenantId: string,
  artifactId: string,
  artifactVersionId = "",
): PreviewArtifactRequest {
  const trimmedVersionId = artifactVersionId.trim();
  return create(PreviewArtifactRequestSchema, {
    tenantId,
    artifactId,
    artifactVersionId: trimmedVersionId || undefined,
  });
}

export async function fetchArtifactPreview(
  tenantId: string,
  artifactId: string,
  artifactVersionId = "",
): Promise<PreviewArtifactResponse> {
  return artifactClient.previewArtifact(
    buildPreviewArtifactRequest(tenantId, artifactId, artifactVersionId),
  );
}

export function formatArtifactPreview(
  response: PreviewArtifactResponse,
): FormattedArtifactPreview {
  switch (response.preview.case) {
    case "textPreview":
      return {
        kind: "text",
        text: response.preview.value,
      };
    case "listSummary": {
      const summary = response.preview.value;
      const titles = summary.titles.filter((title) => title.trim().length > 0);
      const headline =
        titles.length > 0
          ? titles.join("\n")
          : `${summary.articleCount} articles`;
      return {
        kind: "list",
        text: headline,
        listSummary: {
          articleCount: summary.articleCount,
          titles,
        },
      };
    }
    case "jsonPreview":
      return {
        kind: "json",
        text: response.preview.value,
      };
    default:
      return {
        kind: "empty",
        text: "",
      };
  }
}
