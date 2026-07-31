import { useEffect, useReducer } from "react";
import { generatePoster } from "./api";
import { appReducer, createInitialState } from "./reducer";
import { newId } from "./id";
import type { HistoryItem } from "./types";
import { PromptForm } from "./components/PromptForm";
import { Preview } from "./components/Preview";
import { History } from "./components/History";

const STORAGE_KEY = "ai-poster-history";

function loadHistory(): HistoryItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as HistoryItem[]) : [];
  } catch {
    return [];
  }
}

export default function App() {
  const [state, dispatch] = useReducer(
    appReducer,
    undefined,
    () => createInitialState(loadHistory()),
  );

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state.history));
    } catch {
      // storage full or disabled — ignore
    }
  }, [state.history]);

  const handleSubmit = async () => {
    dispatch({ type: "SUBMIT" });
    try {
      const { url } = await generatePoster(state.form);
      const item: HistoryItem = {
        id: newId(),
        prompt: state.form.prompt,
        title: state.form.title,
        url,
        createdAt: Date.now(),
      };
      dispatch({ type: "SUCCESS", item });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      dispatch({ type: "FAILURE", message });
    }
  };

  return (
    <div className="min-h-screen bg-slate-50">
      <div className="max-w-6xl mx-auto p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold text-slate-800">AI 海报生成</h1>
        </header>

        <div className="grid md:grid-cols-2 gap-6">
          <PromptForm
            form={state.form}
            status={state.status}
            onChange={(field, value) =>
              dispatch({ type: "SET_FIELD", field, value })
            }
            onPresetClick={(preset) =>
              dispatch({ type: "APPLY_PRESET", preset })
            }
            onSubmit={handleSubmit}
          />
          <Preview
            status={state.status}
            current={state.current}
            error={state.error}
          />
        </div>

        <History
          items={state.history}
          onSelect={(item) => dispatch({ type: "RESTORE", item })}
          onClear={() => dispatch({ type: "CLEAR_HISTORY" })}
        />
      </div>
    </div>
  );
}
