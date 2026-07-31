import { describe, it, expect, afterEach, vi } from "vitest";
import { newId } from "./id";

describe("newId", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns a non-empty string", () => {
    expect(newId()).toBeTypeOf("string");
    expect(newId().length).toBeGreaterThan(0);
  });

  it("returns distinct values across many calls", () => {
    const ids = new Set<string>();
    for (let i = 0; i < 1000; i++) ids.add(newId());
    expect(ids.size).toBe(1000);
  });

  // The deployed app runs on plain HTTP, where crypto.randomUUID is not
  // exposed at all. Simulate that here so the fallback path stays covered.
  it("still works when crypto.randomUUID is unavailable", () => {
    vi.stubGlobal("crypto", { getRandomValues: crypto.getRandomValues.bind(crypto) });
    expect(crypto.randomUUID).toBeUndefined();

    const ids = new Set<string>();
    for (let i = 0; i < 1000; i++) ids.add(newId());
    expect(ids.size).toBe(1000);
    for (const id of ids) expect(id.length).toBeGreaterThan(0);
  });

  it("still works when crypto is entirely unavailable", () => {
    vi.stubGlobal("crypto", undefined);

    const ids = new Set<string>();
    for (let i = 0; i < 1000; i++) ids.add(newId());
    expect(ids.size).toBe(1000);
  });
});
