import { describe, expect, it } from "vitest";
import { ArtifactStatus } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import {
  artifactCardOpenActionKey,
  artifactCardStatusLabelKey,
  artifactStatusLabelKey,
} from "./text";

describe("artifactCardStatusLabelKey", () => {
  it("prefers the generating caption over the persisted status while producing", () => {
    expect(artifactCardStatusLabelKey(ArtifactStatus.GENERATED, true)).toBe(
      "artifacts.status.generating",
    );
  });

  it("falls back to the persisted status label once not generating", () => {
    expect(artifactCardStatusLabelKey(ArtifactStatus.GENERATED, false)).toBe(
      artifactStatusLabelKey(ArtifactStatus.GENERATED),
    );
    expect(artifactCardStatusLabelKey(ArtifactStatus.EDITED, false)).toBe(
      "artifact.status.edited",
    );
  });
});

describe("artifactCardOpenActionKey", () => {
  it("reads as Open for the active (currently open) card", () => {
    expect(artifactCardOpenActionKey(true)).toBe("artifacts.actions.open");
  });

  it("reads as Preview for every other card", () => {
    expect(artifactCardOpenActionKey(false)).toBe("artifacts.actions.preview");
  });
});
