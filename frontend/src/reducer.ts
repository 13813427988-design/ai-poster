import type { HistoryItem } from "./types";

export const HISTORY_MAX = 50;

export type Status = "idle" | "loading" | "success" | "error";

export type State = {
  form: { prompt: string; title: string };
  current: HistoryItem | null;
  status: Status;
  error: string | null;
  history: HistoryItem[];
};

export type Action =
  | { type: "SET_FIELD"; field: "prompt" | "title"; value: string }
  | { type: "APPLY_PRESET"; preset: { prompt: string; title: string } }
  | { type: "SUBMIT" }
  | { type: "SUCCESS"; item: HistoryItem }
  | { type: "FAILURE"; message: string }
  | { type: "RESTORE"; item: HistoryItem }
  | { type: "CLEAR_HISTORY" };

export function createInitialState(history: HistoryItem[]): State {
  return {
    form: { prompt: "", title: "" },
    current: null,
    status: "idle",
    error: null,
    history,
  };
}

export function appReducer(state: State, action: Action): State {
  switch (action.type) {
    case "SET_FIELD":
      return { ...state, form: { ...state.form, [action.field]: action.value } };
    case "APPLY_PRESET":
      return { ...state, form: { ...action.preset } };
    case "SUBMIT":
      return { ...state, status: "loading", error: null };
    case "SUCCESS":
      return {
        ...state,
        status: "success",
        error: null,
        current: action.item,
        history: [action.item, ...state.history].slice(0, HISTORY_MAX),
      };
    case "FAILURE":
      return { ...state, status: "error", error: action.message };
    case "RESTORE":
      return {
        ...state,
        status: "success",
        error: null,
        current: action.item,
        form: { prompt: action.item.prompt, title: action.item.title },
      };
    case "CLEAR_HISTORY":
      return { ...state, history: [] };
  }
}
