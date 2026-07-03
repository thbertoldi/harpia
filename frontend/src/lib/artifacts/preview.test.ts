import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  ArtifactListSummarySchema,
  ImagePreviewSchema,
  PreviewArtifactResponseSchema,
  type PreviewArtifactResponse,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import { buildPreviewArtifactRequest, formatArtifactPreview } from "./preview";

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

  it("formats html previews with source", () => {
    const formatted = formatArtifactPreview(
      makePreview({ case: "htmlPreview", value: "<h1>Hi</h1>" }),
    );
    expect(formatted.kind).toBe("html");
    expect(formatted.html).toBe("<h1>Hi</h1>");
    expect(formatted.text).toBe("<h1>Hi</h1>");
  });

  it("formats markdown previews with source", () => {
    const formatted = formatArtifactPreview(
      makePreview({ case: "markdownPreview", value: "# Title\n\nBody" }),
    );
    expect(formatted.kind).toBe("markdown");
    expect(formatted.markdown).toBe("# Title\n\nBody");
  });

  it("formats image previews (url)", () => {
    const formatted = formatArtifactPreview(
      makePreview({
        case: "imagePreview",
        value: create(ImagePreviewSchema, {
          url: "https://example.com/a.png",
          altText: "diagram",
        }),
      }),
    );
    expect(formatted.kind).toBe("image");
    expect(formatted.image?.url).toBe("https://example.com/a.png");
    expect(formatted.image?.altText).toBe("diagram");
    expect(formatted.image?.inlineData).toBeUndefined();
  });

  it("formats image previews (inline bytes)", () => {
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
