import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  ArtifactListSummarySchema,
  PreviewArtifactResponseSchema,
  type PreviewArtifactResponse,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import { formatArtifactPreview } from "./preview";

function makePreview(
  preview: PreviewArtifactResponse["preview"],
): PreviewArtifactResponse {
  return create(PreviewArtifactResponseSchema, { preview });
}

describe("formatArtifactPreview", () => {
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
});
