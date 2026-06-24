import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { generatePoster, downloadPoster } from "./api";

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

describe("downloadPoster", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockReset();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("fetches blob, wires anchor with filename, clicks it, and revokes the object url", async () => {
    const blob = new Blob(["poster-bytes"], { type: "image/png" });
    fetchMock.mockResolvedValueOnce({ ok: true, blob: async () => blob });

    const createObjectUrl = vi.fn(() => "blob:fake-url");
    const revokeObjectUrl = vi.fn();
    vi.spyOn(URL, "createObjectURL").mockImplementation(createObjectUrl);
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(revokeObjectUrl);

    const anchor = document.createElement("a");
    const clickSpy = vi.spyOn(anchor, "click").mockImplementation(() => {});
    const createElementSpy = vi
      .spyOn(document, "createElement")
      .mockImplementation((tag: string) => {
        if (tag === "a") return anchor;
        return document.createElementNS("http://www.w3.org/1999/xhtml", tag) as HTMLElement;
      });

    await downloadPoster("http://x/p.png", "my-poster.png");

    expect(fetchMock).toHaveBeenCalledWith("http://x/p.png");
    expect(createObjectUrl).toHaveBeenCalledWith(blob);
    expect(anchor.href).toContain("blob:fake-url");
    expect(anchor.download).toBe("my-poster.png");
    expect(clickSpy).toHaveBeenCalledOnce();
    expect(revokeObjectUrl).toHaveBeenCalledWith("blob:fake-url");

    createElementSpy.mockRestore();
  });
});
