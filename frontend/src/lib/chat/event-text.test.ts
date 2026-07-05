import { describe, it, expect } from "vitest";
import { eventTextKey, type StepTitleResolver } from "./event-text";
import type { ChatMessage, ChatMessageKind } from "./types";

// Builds a minimal ChatMessage with only the fields eventTextKey reads. The
// other fields default to empty/zero so the helper stays decoupled from the
// full proto-derived shape.
function message(
  kind: ChatMessageKind,
  payloadJson = "",
): Pick<ChatMessage, "kind" | "payloadJson"> {
  return { kind, payloadJson };
}

// A title resolver that wraps the key so tests can assert the resolver was
// actually consulted (and the raw key wasn't used directly).
const titles: StepTitleResolver = (key) =>
  key === "write-draft" ? "Write draft" : key;

describe("eventTextKey — static system events", () => {
  it("maps PLAN_ATTACHED to a key without params", () => {
    expect(eventTextKey(message("PLAN_ATTACHED"))).toEqual({
      key: "thread.event.plan_attached",
    });
  });

  it("maps CONFIGURATION_STARTED to a key without params", () => {
    expect(eventTextKey(message("CONFIGURATION_STARTED"))).toEqual({
      key: "thread.event.configuration_started",
    });
  });

  it("maps CONFIGURATION_SAVED to a key without params", () => {
    expect(eventTextKey(message("CONFIGURATION_SAVED"))).toEqual({
      key: "thread.event.configuration_saved",
    });
  });

  it("maps SCHEDULE_SET to a key without params", () => {
    expect(eventTextKey(message("SCHEDULE_SET"))).toEqual({
      key: "thread.event.schedule_set",
    });
  });

  it("maps RUN_STARTED to a key without params", () => {
    expect(eventTextKey(message("RUN_STARTED"))).toEqual({
      key: "thread.event.run_started",
    });
  });

  it("maps RUN_COMPLETED to a key without params", () => {
    expect(eventTextKey(message("RUN_COMPLETED"))).toEqual({
      key: "thread.event.run_completed",
    });
  });
});

describe("eventTextKey — RUN_FAILED reason branching", () => {
  it("uses the generic key when there is no payload", () => {
    expect(eventTextKey(message("RUN_FAILED"))).toEqual({
      key: "thread.event.run_failed",
    });
  });

  it("uses the generic key when the error field is absent", () => {
    expect(eventTextKey(message("RUN_FAILED", "{}"))).toEqual({
      key: "thread.event.run_failed",
    });
  });

  it("uses the generic key when the error field is blank", () => {
    expect(eventTextKey(message("RUN_FAILED", '{"error": "  "}'))).toEqual({
      key: "thread.event.run_failed",
    });
  });

  it("uses the reason key with {reason} when an error is present", () => {
    expect(
      eventTextKey(message("RUN_FAILED", '{"error": "step timed out"}')),
    ).toEqual({
      key: "thread.event.run_failed_reason",
      params: { reason: "step timed out" },
    });
  });

  it("ignores a non-string error field", () => {
    expect(eventTextKey(message("RUN_FAILED", '{"error": 42}'))).toEqual({
      key: "thread.event.run_failed",
    });
  });

  it("degrades on malformed JSON", () => {
    expect(eventTextKey(message("RUN_FAILED", "not-json"))).toEqual({
      key: "thread.event.run_failed",
    });
  });
});

describe("eventTextKey — step events use the title resolver", () => {
  it("maps STEP_STARTED with the resolved step title", () => {
    expect(
      eventTextKey(
        message("STEP_STARTED", '{"step_key": "write-draft"}'),
        titles,
      ),
    ).toEqual({
      key: "thread.event.step_started",
      params: { step: "Write draft" },
    });
  });

  it("maps STEP_BOUND with the resolved step title", () => {
    expect(
      eventTextKey(
        message("STEP_BOUND", '{"step_key": "write-draft"}'),
        titles,
      ),
    ).toEqual({
      key: "thread.event.step_bound",
      params: { step: "Write draft" },
    });
  });

  it("falls back to the raw step_key when no resolver is supplied", () => {
    expect(
      eventTextKey(message("STEP_STARTED", '{"step_key": "write-draft"}')),
    ).toEqual({
      key: "thread.event.step_started",
      params: { step: "write-draft" },
    });
  });

  it("emits an empty step param when the payload lacks a step_key", () => {
    expect(eventTextKey(message("STEP_STARTED", "{}"))).toEqual({
      key: "thread.event.step_started",
      params: { step: "" },
    });
  });

  it("ignores a non-string step_key", () => {
    expect(
      eventTextKey(message("STEP_BOUND", '{"step_key": 7}'), titles),
    ).toEqual({
      key: "thread.event.step_bound",
      params: { step: "" },
    });
  });

  it("degrades on malformed JSON for STEP_BOUND", () => {
    expect(eventTextKey(message("STEP_BOUND", "{bad"), titles)).toEqual({
      key: "thread.event.step_bound",
      params: { step: "" },
    });
  });
});

describe("eventTextKey — fallbacks", () => {
  it("returns null for STEP_REBOUND (dynamic user-selection echo)", () => {
    expect(
      eventTextKey(message("STEP_REBOUND", '{"step_key": "write-draft"}')),
    ).toBeNull();
  });

  it("returns null for unknown / user-authored kinds", () => {
    expect(eventTextKey(message("USER_TEXT"))).toBeNull();
    expect(eventTextKey(message("USER_SELECTION"))).toBeNull();
    expect(eventTextKey(message("ASSISTANT_TEXT"))).toBeNull();
  });
});
