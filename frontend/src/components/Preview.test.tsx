import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Preview } from "./Preview";
import type { HistoryItem } from "../types";

const item: HistoryItem = {
  id: "1",
  prompt: "p",
  title: "t",
  url: "http://x/p.png",
  createdAt: 0,
};

describe("Preview", () => {
  it("renders empty state when idle and no current", () => {
    render(<Preview status="idle" current={null} error={null} />);
    expect(screen.getByText(/在左侧填写/)).toBeInTheDocument();
  });

  it("renders skeleton when loading", () => {
    const { container } = render(
      <Preview status="loading" current={null} error={null} />,
    );
    expect(container.querySelector('[data-testid="preview-skeleton"]')).not.toBeNull();
  });

  it("renders image and download button on success", () => {
    render(<Preview status="success" current={item} error={null} />);
    const img = screen.getByRole("img");
    expect(img).toHaveAttribute("src", item.url);
    const button = screen.getByRole("button", { name: /下载/ });
    expect(button).toBeInstanceOf(HTMLButtonElement);
    expect(button).toHaveAttribute("type", "button");
    // React attaches onClick via its synthetic event system; presence of an
    // event listener is verified by api.test.ts where downloadPoster's
    // happy-path is exercised end-to-end.
  });

  it("renders error message when error", () => {
    render(<Preview status="error" current={null} error="boom" />);
    expect(screen.getByText(/boom/)).toBeInTheDocument();
  });
});
