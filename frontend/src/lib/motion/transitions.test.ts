import { describe, expect, it, vi, afterEach } from "vitest";
import { chatEnter, cardLift, chipFlash, errorShake, saveCelebration } from "./transitions";

const originalMatchMedia = globalThis.matchMedia;
afterEach(() => {
  globalThis.matchMedia = originalMatchMedia;
});

function noReduce() {
  globalThis.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof matchMedia;
}
function withReduce() {
  globalThis.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof matchMedia;
}

describe("chatEnter", () => {
  it("returns a transition with 180ms duration and fade+slide css", () => {
    noReduce();
    const node = ({} as HTMLElement);
    const t = chatEnter(node);
    expect(t.duration).toBe(180);
    expect(t.css?.(0, 1)).toMatch(/translateY\(8px\)/);
    expect(t.css?.(1, 0)).toMatch(/translateY\(0px\)/);
  });
  it("collapses to opacity-only 0ms under reduced motion", () => {
    withReduce();
    const node = ({} as HTMLElement);
    const t = chatEnter(node);
    expect(t.duration).toBe(0);
    expect(t.css?.(0, 1)).not.toMatch(/translateY/);
  });
});

describe("cardLift", () => {
  it("returns a 120ms transition", () => {
    noReduce();
    const t = cardLift(({} as HTMLElement));
    expect(t.duration).toBe(120);
  });
});

describe("chipFlash", () => {
  it("totals 130ms (50ms flash + 80ms decay)", () => {
    noReduce();
    const t = chipFlash(({} as HTMLElement));
    expect(t.duration).toBe(130);
  });
});

describe("errorShake", () => {
  it("returns a 60ms transition", () => {
    noReduce();
    const t = errorShake(({} as HTMLElement));
    expect(t.duration).toBe(60);
  });
});

describe("saveCelebration", () => {
  it("returns a 1400ms transition", () => {
    noReduce();
    const t = saveCelebration(({} as HTMLElement));
    expect(t.duration).toBe(1400);
  });
  it("collapses to 0ms under reduced motion", () => {
    withReduce();
    const t = saveCelebration(({} as HTMLElement));
    expect(t.duration).toBe(0);
  });
});
