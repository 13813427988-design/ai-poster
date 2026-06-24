import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";

describe("App", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockReset();
    localStorage.clear();
    vi.spyOn(crypto, "randomUUID").mockReturnValue("test-uuid" as any);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("renders the three sections on mount", () => {
    render(<App />);
    expect(screen.getByLabelText(/标题/)).toBeInTheDocument();
    expect(screen.getByText(/在左侧填写/)).toBeInTheDocument();
    expect(screen.getByText(/历史记录/)).toBeInTheDocument();
  });

  it("submits form and shows preview on success", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ url: "http://x/p.png" }),
    });
    render(<App />);
    await userEvent.type(screen.getByLabelText(/标题/), "T");
    await userEvent.type(screen.getByLabelText(/描述/), "P");
    await userEvent.click(screen.getByRole("button", { name: /生成/ }));

    await waitFor(() => {
      const imgs = screen.getAllByRole("img");
      expect(imgs.some((i) => i.getAttribute("src") === "http://x/p.png")).toBe(true);
    });
  });

  it("shows error message when API fails", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "server down",
    });
    render(<App />);
    await userEvent.type(screen.getByLabelText(/标题/), "T");
    await userEvent.type(screen.getByLabelText(/描述/), "P");
    await userEvent.click(screen.getByRole("button", { name: /生成/ }));

    await waitFor(() => {
      expect(screen.getByText(/生成失败/)).toBeInTheDocument();
    });
  });

  it("persists history to localStorage", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ url: "http://x/p.png" }),
    });
    render(<App />);
    await userEvent.type(screen.getByLabelText(/标题/), "T");
    await userEvent.type(screen.getByLabelText(/描述/), "P");
    await userEvent.click(screen.getByRole("button", { name: /生成/ }));

    await waitFor(() => {
      const raw = localStorage.getItem("ai-poster-history");
      expect(raw).not.toBeNull();
      const parsed = JSON.parse(raw!);
      expect(parsed).toHaveLength(1);
      expect(parsed[0].url).toBe("http://x/p.png");
    });
  });

  it("loads history from localStorage on mount", () => {
    localStorage.setItem(
      "ai-poster-history",
      JSON.stringify([
        { id: "x", prompt: "p", title: "t", url: "u", createdAt: 1 },
      ]),
    );
    render(<App />);
    const imgs = screen.getAllByRole("img");
    expect(imgs.some((i) => i.getAttribute("src") === "u")).toBe(true);
  });

  it("recovers from corrupt localStorage", () => {
    localStorage.setItem("ai-poster-history", "{not json");
    expect(() => render(<App />)).not.toThrow();
    expect(screen.getByText(/暂无历史/)).toBeInTheDocument();
  });
});
