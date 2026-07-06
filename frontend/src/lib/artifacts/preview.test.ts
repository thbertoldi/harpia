import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ArtifactListSummarySchema,
  ArtifactSchema,
  ImagePreviewSchema,
  PreviewArtifactResponseSchema,
  type PreviewArtifactResponse,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import {
  buildPreviewArtifactRequest,
  formatArtifactPreview,
  resolvePreviewArtifact,
  shouldAutoOpenFinalArtifact,
} from "./preview";

function makePreview(
  preview: PreviewArtifactResponse["preview"],
): PreviewArtifactResponse {
  return create(PreviewArtifactResponseSchema, { preview });
}

describe("formatArtifactPreview", () => {
  it("omits empty artifact version ids from preview requests", () => {
    expect(
      "artifactVersionId" in
        buildPreviewArtifactRequest("tenant-1", "artifact-1", ""),
    ).toBe(false);
    expect(
      buildPreviewArtifactRequest("tenant-1", "artifact-1", "version-1"),
    ).toMatchObject({ artifactVersionId: "version-1" });
  });

  it("formats text previews", () => {
    const formatted = formatArtifactPreview(
      makePreview({ case: "textPreview", value: "# Title\n\nBody" }),
    );
    expect(formatted).toEqual({
      kind: "text",
      text: "# Title\n\nBody",
    });
  });

  it("formats list summaries with titles", () => {
    const formatted = formatArtifactPreview(
      makePreview({
        case: "listSummary",
        value: create(ArtifactListSummarySchema, {
          articleCount: 2,
          titles: ["First", "Second"],
        }),
      }),
    );
    expect(formatted.kind).toBe("list");
    expect(formatted.text).toBe("First\nSecond");
    expect(formatted.listSummary).toEqual({
      articleCount: 2,
      titles: ["First", "Second"],
    });
  });

  it("formats json previews", () => {
    const formatted = formatArtifactPreview(
      makePreview({
        case: "jsonPreview",
        value: `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
      }),
    );
    expect(formatted).toEqual({
      kind: "json",
      text: `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
    });
  });

  it("maps markdown previews to source-preserving markdown output", () => {
    const preview = formatArtifactPreview(
      create(PreviewArtifactResponseSchema, {
        preview: { case: "markdownPreview", value: "# Draft" },
      }),
    );

    expect(preview).toMatchObject({
      kind: "markdown",
      text: "# Draft",
      markdown: "# Draft",
    });
  });

  it("maps html previews to source-preserving html output", () => {
    const preview = formatArtifactPreview(
      create(PreviewArtifactResponseSchema, {
        preview: { case: "htmlPreview", value: "<p>Ready</p>" },
      }),
    );

    expect(preview).toMatchObject({
      kind: "html",
      text: "<p>Ready</p>",
      html: "<p>Ready</p>",
    });
  });

  it("maps list summaries and filters blank titles", () => {
    const preview = formatArtifactPreview(
      create(PreviewArtifactResponseSchema, {
        preview: {
          case: "listSummary",
          value: create(ArtifactListSummarySchema, {
            articleCount: 3,
            titles: ["First", " ", "Second"],
          }),
        },
      }),
    );

    expect(preview.kind).toBe("list");
    expect(preview.text).toBe("First\nSecond");
    expect(preview.listSummary).toEqual({
      articleCount: 3,
      titles: ["First", "Second"],
    });
  });

  it("maps image previews to optional image metadata", () => {
    const preview = formatArtifactPreview(
      create(PreviewArtifactResponseSchema, {
        preview: {
          case: "imagePreview",
          value: create(ImagePreviewSchema, {
            url: "https://example.test/image.png",
            altText: "Generated preview",
          }),
        },
      }),
    );

    expect(preview).toMatchObject({
      kind: "image",
      text: "https://example.test/image.png",
      image: {
        url: "https://example.test/image.png",
        altText: "Generated preview",
      },
    });
  });

  it("formats image previews with inline bytes", () => {
    const formatted = formatArtifactPreview(
      makePreview({
        case: "imagePreview",
        value: create(ImagePreviewSchema, {
          inlineData: new Uint8Array([1, 2, 3]),
        }),
      }),
    );
    expect(formatted.kind).toBe("image");
    expect(formatted.image?.inlineData).toEqual(new Uint8Array([1, 2, 3]));
    expect(formatted.image?.url).toBeUndefined();
  });

  it("falls back to empty for an unset oneof", () => {
    const formatted = formatArtifactPreview(makePreview({ case: undefined }));
    expect(formatted.kind).toBe("empty");
  });
});

describe("artifact preview state helpers", () => {
  const finalArtifact = create(ArtifactSchema, {
    id: "artifact-final",
    artifactTypeKey: "harpia.artifacts.v1.PublishConfirmation",
  });
  const fallbackArtifact = create(ArtifactSchema, {
    id: "artifact-old",
    artifactTypeKey: "harpia.artifacts.v1.TextDraft",
  });

  it("resolves the active artifact from current execution artifacts first", () => {
    expect(
      resolvePreviewArtifact(
        "artifact-final",
        [finalArtifact],
        fallbackArtifact,
      ),
    ).toBe(finalArtifact);
  });

  it("resolves a fetched fallback artifact when it matches the active id", () => {
    expect(resolvePreviewArtifact("artifact-old", [], fallbackArtifact)).toBe(
      fallbackArtifact,
    );
  });

  it("keeps final artifact auto-open suppressed only for the dismissed final artifact", () => {
    expect(
      shouldAutoOpenFinalArtifact("artifact-final", "artifact-final"),
    ).toBe(false);
    expect(shouldAutoOpenFinalArtifact("artifact-next", "artifact-final")).toBe(
      true,
    );
  });
});
