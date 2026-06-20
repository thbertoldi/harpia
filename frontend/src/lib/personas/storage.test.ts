import { describe, expect, it } from "vitest";
import {
  PERSONA_STORAGE_KEY,
  canSwitchToAdminPersona,
  readPersonaMode,
  writePersonaMode,
} from "./storage";

function makeStorage(initial: Record<string, string> = {}) {
  const state = { ...initial };
  return {
    getItem: (k: string) => state[k] ?? null,
    setItem: (k: string, v: string) => {
      state[k] = v;
    },
    snapshot: () => ({ ...state }),
  };
}

describe("readPersonaMode", () => {
  it("defaults to operator when nothing is stored", () => {
    expect(readPersonaMode(makeStorage())).toBe("operator");
  });

  it("returns admin when the stored value is 'admin'", () => {
    const s = makeStorage({ [PERSONA_STORAGE_KEY]: "admin" });
    expect(readPersonaMode(s)).toBe("admin");
  });

  it("returns operator for any non-'admin' stored value", () => {
    const s = makeStorage({ [PERSONA_STORAGE_KEY]: "garbage" });
    expect(readPersonaMode(s)).toBe("operator");
  });
});

describe("writePersonaMode", () => {
  it("persists the mode under the storage key", () => {
    const s = makeStorage();
    writePersonaMode(s, "admin");
    expect(s.snapshot()[PERSONA_STORAGE_KEY]).toBe("admin");
  });
});

describe("canSwitchToAdminPersona", () => {
  it("returns true for Leader (has all admin permissions)", () => {
    expect(canSwitchToAdminPersona("Leader")).toBe(true);
  });

  it("returns true for Engineer (has all admin permissions)", () => {
    expect(canSwitchToAdminPersona("Engineer")).toBe(true);
  });

  it("returns true for Overseer (holds viewAudit, which is admin-side)", () => {
    expect(canSwitchToAdminPersona("Overseer")).toBe(true);
  });

  it("returns false for undefined role", () => {
    expect(canSwitchToAdminPersona(undefined)).toBe(false);
  });

  it("returns false for unknown role", () => {
    expect(canSwitchToAdminPersona("member")).toBe(false);
  });
});
