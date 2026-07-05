import { describe, expect, it } from "vitest";
import {
  localizedOverseerLabel,
  overseerAvatarInitial,
} from "$lib/plans/overseer-label";

describe("localizedOverseerLabel", () => {
  it("returns empty string for an empty id", () => {
    expect(localizedOverseerLabel("", "user-self", "en")).toBe("");
    expect(localizedOverseerLabel("   ", "user-self", "en")).toBe("");
  });

  it("localizes the current user by id equality, never by string-matching", () => {
    expect(localizedOverseerLabel("user-self", "user-self", "en")).toBe("You");
    expect(localizedOverseerLabel("user-self", "user-self", "pt-BR")).toBe(
      "Eu",
    );
    // A label that happens to read "You" must not short-circuit resolution;
    // an id that is not the session user falls through to the fallback.
    expect(localizedOverseerLabel("someone-else", "user-self", "en")).toBe(
      "Someone",
    );
  });

  it("prefers a known user's name, then email, for non-self overseers", () => {
    const known = new Map(
      [
        { sub: "user-ana", name: "Ana" },
        { sub: "user-bob", email: "bob@example.com" },
      ].map((u) => [u.sub, u]),
    );

    expect(localizedOverseerLabel("user-ana", "user-self", "en", known)).toBe(
      "Ana",
    );
    expect(localizedOverseerLabel("user-bob", "user-self", "en", known)).toBe(
      "bob@example.com",
    );
  });

  it("falls back to the localized generic label for unknown ids", () => {
    expect(localizedOverseerLabel("user-ghost", undefined, "en")).toBe(
      "Someone",
    );
    expect(localizedOverseerLabel("user-ghost", undefined, "pt-BR")).toBe(
      "Alguém",
    );
  });

  it("never returns the raw id", () => {
    const raw = "acct:overseer-xyz@harpia";
    expect(localizedOverseerLabel(raw, "user-self", "en")).not.toBe(raw);
    expect(localizedOverseerLabel(raw, "user-self", "pt-BR")).not.toBe(raw);
  });
});

describe("overseerAvatarInitial", () => {
  it("uppercases the first non-space character", () => {
    expect(overseerAvatarInitial("You")).toBe("Y");
    expect(overseerAvatarInitial("eu")).toBe("E");
    expect(overseerAvatarInitial("  ana ")).toBe("A");
  });

  it("falls back to a bullet when there is nothing to seed from", () => {
    expect(overseerAvatarInitial("")).toBe("•");
    expect(overseerAvatarInitial("   ")).toBe("•");
  });
});
