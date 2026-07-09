import { create } from "@bufbuild/protobuf";
import {
  PreviewArtifactRequestSchema,
  type Artifact,
  type PreviewArtifactRequest,
  type PreviewArtifactResponse,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import { artifactClient } from "$lib/rpc";

export type ArtifactPreviewKind =
  | "text"
  | "list"
  | "json"
  | "html"
  | "markdown"
  | "image"
  | "empty";

export type FormattedArtifactPreview = {
  kind: ArtifactPreviewKind;
  text: string;
  listSummary?: {
    articleCount: number;
    titles: string[];
  };
  /** HTML source when kind === "html" (rendered via a sandboxed iframe). */
  html?: string;
  /** Markdown source when kind === "markdown". */
  markdown?: string;
  /** Image preview when kind === "image". */
  image?: {
    url?: string;
    inlineData?: Uint8Array;
    altText?: string;
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
    case "htmlPreview":
      return {
        kind: "html",
        text: response.preview.value,
        html: response.preview.value,
      };
    case "markdownPreview":
      return {
        kind: "markdown",
        text: response.preview.value,
        markdown: response.preview.value,
      };
    case "imagePreview": {
      const img = response.preview.value;
      const altText = img.altText ?? "";
      return {
        kind: "image",
        text: img.url ?? "",
        image: {
          url: img.url || undefined,
          inlineData: img.inlineData?.length ? img.inlineData : undefined,
          altText: altText || undefined,
        },
      };
    }
    default:
      return {
        kind: "empty",
        text: "",
      };
  }
}

export function resolvePreviewArtifact(
  activeArtifactId: string | null,
  artifacts: Artifact[],
  fallbackArtifact: Artifact | null,
): Artifact | null {
  if (!activeArtifactId) return null;
  return (
    artifacts.find((artifact) => artifact.id === activeArtifactId) ??
    (fallbackArtifact?.id === activeArtifactId ? fallbackArtifact : null)
  );
}

export function shouldAutoOpenFinalArtifact(
  finalArtifactId: string | null,
  dismissedFinalArtifactId: string | null,
): boolean {
  return !!finalArtifactId && finalArtifactId !== dismissedFinalArtifactId;
}

/**
 * Early-open predicate: whether the side preview should pre-open in a
 * generating state because the step producing the emphasized/final artifact
 * is currently running. Yields to the existence-based open once the artifact
 * is ready (`hasReadyFinalArtifact`), and respects the same per-execution
 * dismissal memory the generating placeholder itself was closed under (an
 * artifact id doesn't exist yet to key dismissal on, so this is keyed to the
 * producing execution id instead).
 */
export function shouldOpenGeneratingPreview(
  runningStepOutputTypeKey: string | null,
  finalArtifactTypeKeys: Set<string>,
  hasReadyFinalArtifact: boolean,
  producingExecutionId: string | null,
  dismissedGeneratingExecutionId: string | null,
): boolean {
  if (hasReadyFinalArtifact) return false;
  if (!producingExecutionId) return false;
  if (!runningStepOutputTypeKey) return false;
  if (!finalArtifactTypeKeys.has(runningStepOutputTypeKey)) return false;
  return producingExecutionId !== dismissedGeneratingExecutionId;
}

const PRIMARY_PREVIEW_ARTIFACT_TYPES = [
  "harpia.artifacts.v1.LinkedInPostDraft",
  "harpia.artifacts.v1.CarouselDraft",
  "harpia.artifacts.v1.TextDraft",
];

export function primaryPreviewArtifact(
  artifacts: Artifact[],
  finalArtifactTypeKeys: Set<string>,
): Artifact | null {
  for (const typeKey of PRIMARY_PREVIEW_ARTIFACT_TYPES) {
    const artifact = artifacts.find(
      (candidate) => candidate.artifactTypeKey === typeKey,
    );
    if (artifact) return artifact;
  }
  return (
    artifacts.find((artifact) =>
      finalArtifactTypeKeys.has(artifact.artifactTypeKey),
    ) ?? null
  );
}
