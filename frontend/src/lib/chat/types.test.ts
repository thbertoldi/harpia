import { describe, expect, it } from "vitest";
import { ThreadMessageKind } from "$lib/gen/harpia/chat/v1/chat_pb";
import { chatKindToProto, type ChatMessageKind } from "./types";

describe("chat message kind mapping", () => {
  it("maps the typed execution conversation prompt", () => {
    const kind: ChatMessageKind = "EXECUTION_PROMPT";
    expect(chatKindToProto(kind)).toBe(ThreadMessageKind.EXECUTION_PROMPT);
  });
});
