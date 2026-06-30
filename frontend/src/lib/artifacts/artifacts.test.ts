import { describe, expect, it } from "vitest";
import { editableTextFromPayload, artifactTitle } from "./text";

describe("artifact text helpers", () => {
  it("extracts editable text from TextDraft", () => {
    const result = editableTextFromPayload("harpia.artifacts.v1.TextDraft", {
      title: "Newsletter",
      body: "Draft body",
    });

    expect(result).toEqual({
      editable: true,
      title: "Newsletter",
      text: "Draft body",
    });
  });

  it("extracts editable text from LinkedInPostDraft", () => {
    const result = editableTextFromPayload(
      "harpia.artifacts.v1.LinkedInPostDraft",
      {
        hook: "Hook",
        text: "Post body",
        hashtags: ["growth"],
      },
    );

    expect(result).toEqual({
      editable: true,
      title: "Hook",
      text: "Post body",
    });
  });

  it("builds a readable artifact title", () => {
    expect(
      artifactTitle({
        id: "artifact-1",
        artifactTypeKey: "harpia.artifacts.v1.LinkedInPostDraft",
      }),
    ).toBe("LinkedIn post draft");
  });
});
