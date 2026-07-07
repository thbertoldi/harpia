import {
  ArtifactStatus,
  type Artifact,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

export interface EditableTextProjection {
  editable: boolean;
  title: string;
  text: string;
}

export function artifactTitle(
  artifact: Pick<Artifact, "artifactTypeKey" | "id">,
): string {
  switch (artifact.artifactTypeKey) {
    case "harpia.artifacts.v1.NewsList":
      return "Sources";
    case "harpia.artifacts.v1.TextDraft":
      return "Text draft";
    case "harpia.artifacts.v1.LinkedInPostDraft":
      return "LinkedIn post draft";
    case "harpia.artifacts.v1.PublishConfirmation":
      return "Publish confirmation";
    default:
      return "Artifact";
  }
}

export function artifactTypeLabelKey(artifactTypeKey: string): string {
  switch (artifactTypeKey) {
    case "harpia.artifacts.v1.DateRange":
      return "artifact.type.dateRange";
    case "harpia.artifacts.v1.NewsList":
      return "artifact.type.newsList";
    case "harpia.artifacts.v1.TextDraft":
      return "artifact.type.textDraft";
    case "harpia.artifacts.v1.LinkedInPostDraft":
      return "artifact.type.linkedInPostDraft";
    case "harpia.artifacts.v1.PublishConfirmation":
      return "artifact.type.publishConfirmation";
    default:
      return "artifact.type.unknown";
  }
}

export function artifactStatusLabelKey(status: ArtifactStatus): string {
  switch (status) {
    case ArtifactStatus.GENERATED:
      return "artifact.status.generated";
    case ArtifactStatus.EDITED:
      return "artifact.status.edited";
    case ArtifactStatus.APPROVED:
      return "artifact.status.approved";
    case ArtifactStatus.REJECTED:
      return "artifact.status.rejected";
    case ArtifactStatus.SUPERSEDED:
      return "artifact.status.superseded";
    case ArtifactStatus.FAILED:
      return "artifact.status.failed";
    default:
      return "artifact.status.unspecified";
  }
}

/**
 * Status label key for an ArtifactCard: "generating" takes priority over the
 * artifact's own persisted status while a step is actively producing it.
 */
export function artifactCardStatusLabelKey(
  status: ArtifactStatus,
  generating: boolean,
): string {
  return generating
    ? "artifacts.status.generating"
    : artifactStatusLabelKey(status);
}

/**
 * Open-action i18n key for an ArtifactCard: the currently active (open in the
 * panel) card reads "Open"; every other card reads "Preview".
 */
export function artifactCardOpenActionKey(active: boolean): string {
  return active ? "artifacts.actions.open" : "artifacts.actions.preview";
}

export function artifactStatusLabel(status: ArtifactStatus): string {
  switch (status) {
    case ArtifactStatus.GENERATED:
      return "generated";
    case ArtifactStatus.EDITED:
      return "edited";
    case ArtifactStatus.APPROVED:
      return "approved";
    case ArtifactStatus.REJECTED:
      return "rejected";
    case ArtifactStatus.SUPERSEDED:
      return "superseded";
    case ArtifactStatus.FAILED:
      return "failed";
    default:
      return "unspecified";
  }
}

export function editableTextFromPayload(
  artifactTypeKey: string,
  payload: Record<string, unknown>,
): EditableTextProjection {
  if (artifactTypeKey === "harpia.artifacts.v1.TextDraft") {
    return {
      editable: true,
      title: typeof payload.title === "string" ? payload.title : "",
      text: typeof payload.body === "string" ? payload.body : "",
    };
  }
  if (artifactTypeKey === "harpia.artifacts.v1.LinkedInPostDraft") {
    return {
      editable: true,
      title: typeof payload.hook === "string" ? payload.hook : "",
      text: typeof payload.text === "string" ? payload.text : "",
    };
  }
  return {
    editable: false,
    title: artifactTitle({ artifactTypeKey, id: "" }),
    text: "",
  };
}
