import { describe, it, expect } from "vitest";
import {
  appReducer,
  createInitialState,
  HISTORY_MAX,
  type State,
} from "./reducer";
import type { HistoryItem } from "./types";

const makeItem = (id: string, ts = 0): HistoryItem => ({
  id,
  prompt: `prompt-${id}`,
  title: `title-${id}`,
  url: `url-${id}`,
  createdAt: ts,
});

const baseState = (overrides: Partial<State> = {}): State => ({
  ...createInitialState([]),
  ...overrides,
});

describe("appReducer", () => {
  it("SET_FIELD updates form.prompt", () => {
    const s = appReducer(baseState(), {
      type: "SET_FIELD",
      field: "prompt",
      value: "hello",
    });
    expect(s.form.prompt).toBe("hello");
    expect(s.form.title).toBe("");
  });

  it("SET_FIELD updates form.title", () => {
    const s = appReducer(baseState(), {
      type: "SET_FIELD",
      field: "title",
      value: "T",
    });
    expect(s.form.title).toBe("T");
  });

  it("APPLY_PRESET fills both fields", () => {
    const s = appReducer(baseState(), {
      type: "APPLY_PRESET",
      preset: { prompt: "p", title: "t" },
    });
    expect(s.form).toEqual({ prompt: "p", title: "t" });
  });

  it("SUBMIT sets status to loading and clears error", () => {
    const s = appReducer(baseState({ status: "error", error: "x" }), {
      type: "SUBMIT",
    });
    expect(s.status).toBe("loading");
    expect(s.error).toBeNull();
  });

  it("SUCCESS sets status, current, and prepends to history", () => {
    const before = baseState({
      status: "loading",
      history: [makeItem("a", 1)],
    });
    const item = makeItem("b", 2);
    const s = appReducer(before, { type: "SUCCESS", item });
    expect(s.status).toBe("success");
    expect(s.current).toEqual(item);
    expect(s.history.map((h) => h.id)).toEqual(["b", "a"]);
  });

  it("SUCCESS truncates history to HISTORY_MAX", () => {
    const history = Array.from({ length: HISTORY_MAX }, (_, i) =>
      makeItem(`old-${i}`, i),
    );
    const before = baseState({ status: "loading", history });
    const item = makeItem("new", 999);
    const s = appReducer(before, { type: "SUCCESS", item });
    expect(s.history).toHaveLength(HISTORY_MAX);
    expect(s.history[0]).toEqual(item);
    expect(s.history[s.history.length - 1].id).toBe(`old-${HISTORY_MAX - 2}`);
  });

  it("FAILURE sets status to error and stores message", () => {
    const s = appReducer(baseState({ status: "loading" }), {
      type: "FAILURE",
      message: "oops",
    });
    expect(s.status).toBe("error");
    expect(s.error).toBe("oops");
  });

  it("RESTORE copies item into form and current", () => {
    const item = makeItem("r", 5);
    const s = appReducer(baseState(), { type: "RESTORE", item });
    expect(s.current).toEqual(item);
    expect(s.form).toEqual({ prompt: item.prompt, title: item.title });
    expect(s.status).toBe("success");
  });

  it("CLEAR_HISTORY empties history but keeps form/current", () => {
    const item = makeItem("k", 1);
    const before = baseState({
      history: [item],
      current: item,
      form: { prompt: "kept", title: "kept" },
    });
    const s = appReducer(before, { type: "CLEAR_HISTORY" });
    expect(s.history).toEqual([]);
    expect(s.current).toEqual(item);
    expect(s.form).toEqual({ prompt: "kept", title: "kept" });
  });
});

describe("createInitialState", () => {
  it("seeds history from arg, leaves other fields default", () => {
    const item = makeItem("x");
    const s = createInitialState([item]);
    expect(s.history).toEqual([item]);
    expect(s.form).toEqual({ prompt: "", title: "" });
    expect(s.status).toBe("idle");
    expect(s.error).toBeNull();
    expect(s.current).toBeNull();
  });
});
