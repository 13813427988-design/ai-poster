import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { generatePoster } from "./api";

describe("generatePoster", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockReset();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts to /api/generate and returns the parsed response", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ url: "http://x/poster.png" }),
    });

    const out = await generatePoster({ prompt: "p", title: "t" });

    expect(fetchMock).toHaveBeenCalledWith("/api/generate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prompt: "p", title: "t" }),
    });
    expect(out).toEqual({ url: "http://x/poster.png" });
  });

  it("throws with status code on non-2xx responses", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "boom",
    });

    await expect(generatePoster({ prompt: "p", title: "t" })).rejects.toThrow(
      /HTTP 500.*boom/,
    );
  });

  it("throws on 4xx as well", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 400,
      text: async () => "bad",
    });

    await expect(generatePoster({ prompt: "p", title: "t" })).rejects.toThrow(
      /HTTP 400.*bad/,
    );
  });
});
