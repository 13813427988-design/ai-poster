import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PromptForm } from "./PromptForm";
import { PRESETS } from "../presets";

const defaultProps = {
  form: { prompt: "", title: "" },
  status: "idle" as const,
  onChange: vi.fn(),
  onPresetClick: vi.fn(),
  onSubmit: vi.fn(),
};

describe("PromptForm", () => {
  it("renders title input, prompt textarea, and submit button", () => {
    render(<PromptForm {...defaultProps} />);
    expect(screen.getByLabelText(/标题/)).toBeInTheDocument();
    expect(screen.getByLabelText(/描述/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /生成/ })).toBeInTheDocument();
  });

  it("renders one button per preset", () => {
    render(<PromptForm {...defaultProps} />);
    for (const p of PRESETS) {
      expect(screen.getByRole("button", { name: p.label })).toBeInTheDocument();
    }
  });

  it("calls onChange when typing title", async () => {
    const onChange = vi.fn();
    render(<PromptForm {...defaultProps} onChange={onChange} />);
    await userEvent.type(screen.getByLabelText(/标题/), "h");
    expect(onChange).toHaveBeenCalledWith("title", "h");
  });

  it("calls onPresetClick when preset button clicked", async () => {
    const onPresetClick = vi.fn();
    render(<PromptForm {...defaultProps} onPresetClick={onPresetClick} />);
    await userEvent.click(
      screen.getByRole("button", { name: PRESETS[0].label }),
    );
    expect(onPresetClick).toHaveBeenCalledWith({
      prompt: PRESETS[0].prompt,
      title: PRESETS[0].title,
    });
  });

  it("disables submit when prompt is empty", () => {
    render(<PromptForm {...defaultProps} form={{ prompt: "", title: "t" }} />);
    expect(screen.getByRole("button", { name: /生成/ })).toBeDisabled();
  });

  it("disables submit when title is empty", () => {
    render(<PromptForm {...defaultProps} form={{ prompt: "p", title: "" }} />);
    expect(screen.getByRole("button", { name: /生成/ })).toBeDisabled();
  });

  it("disables submit when status is loading", () => {
    render(
      <PromptForm
        {...defaultProps}
        status="loading"
        form={{ prompt: "p", title: "t" }}
      />,
    );
    expect(screen.getByRole("button", { name: /生成/ })).toBeDisabled();
  });

  it("calls onSubmit when valid form is submitted", async () => {
    const onSubmit = vi.fn();
    render(
      <PromptForm
        {...defaultProps}
        form={{ prompt: "p", title: "t" }}
        onSubmit={onSubmit}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /生成/ }));
    expect(onSubmit).toHaveBeenCalled();
  });
});
