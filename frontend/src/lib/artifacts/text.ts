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
      return artifact.id;
  }
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
