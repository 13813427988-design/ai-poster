import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { History } from "./History";
import type { HistoryItem } from "../types";

const items: HistoryItem[] = [
  { id: "1", prompt: "p1", title: "t1", url: "u1", createdAt: 1 },
  { id: "2", prompt: "p2", title: "t2", url: "u2", createdAt: 2 },
];

describe("History", () => {
  let confirmSpy: ReturnType<typeof vi.spyOn>;
  beforeEach(() => {
    confirmSpy = vi.spyOn(window, "confirm");
  });
  afterEach(() => {
    confirmSpy.mockRestore();
  });

  it("renders empty state when no items", () => {
    render(<History items={[]} onSelect={vi.fn()} onClear={vi.fn()} />);
    expect(screen.getByText(/暂无历史/)).toBeInTheDocument();
  });

  it("renders one thumbnail per item", () => {
    render(<History items={items} onSelect={vi.fn()} onClear={vi.fn()} />);
    const imgs = screen.getAllByRole("img");
    expect(imgs).toHaveLength(2);
    expect(imgs[0]).toHaveAttribute("src", "u1");
  });

  it("calls onSelect with the clicked item", async () => {
    const onSelect = vi.fn();
    render(<History items={items} onSelect={onSelect} onClear={vi.fn()} />);
    await userEvent.click(screen.getAllByRole("img")[1]);
    expect(onSelect).toHaveBeenCalledWith(items[1]);
  });

  it("calls onClear when clear is confirmed", async () => {
    confirmSpy.mockReturnValue(true);
    const onClear = vi.fn();
    render(<History items={items} onSelect={vi.fn()} onClear={onClear} />);
    await userEvent.click(screen.getByRole("button", { name: /清空/ }));
    expect(onClear).toHaveBeenCalled();
  });

  it("does NOT call onClear when confirm is cancelled", async () => {
    confirmSpy.mockReturnValue(false);
    const onClear = vi.fn();
    render(<History items={items} onSelect={vi.fn()} onClear={onClear} />);
    await userEvent.click(screen.getByRole("button", { name: /清空/ }));
    expect(onClear).not.toHaveBeenCalled();
  });

  it("hides clear button when history is empty", () => {
    render(<History items={[]} onSelect={vi.fn()} onClear={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /清空/ })).toBeNull();
  });
});
