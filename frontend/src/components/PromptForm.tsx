import type { Status } from "../reducer";
import { PRESETS } from "../presets";

type Props = {
  form: { prompt: string; title: string };
  status: Status;
  onChange: (field: "prompt" | "title", value: string) => void;
  onPresetClick: (preset: { prompt: string; title: string }) => void;
  onSubmit: () => void;
};

const TITLE_MAX = 30;
const PROMPT_MAX = 200;

export function PromptForm({
  form,
  status,
  onChange,
  onPresetClick,
  onSubmit,
}: Props) {
  const isLoading = status === "loading";
  const canSubmit =
    form.prompt.trim().length > 0 && form.title.trim().length > 0 && !isLoading;

  return (
    <form
      className="space-y-4 p-4 bg-white rounded-lg shadow-sm border border-slate-200"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) onSubmit();
      }}
    >
      <div>
        <label
          htmlFor="title"
          className="block text-sm font-medium text-slate-700 mb-1"
        >
          标题
        </label>
        <input
          id="title"
          type="text"
          value={form.title}
          maxLength={TITLE_MAX}
          onChange={(e) => onChange("title", e.target.value)}
          className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-400"
          placeholder="海报标题"
        />
      </div>

      <div>
        <label
          htmlFor="prompt"
          className="block text-sm font-medium text-slate-700 mb-1"
        >
          描述（prompt）
        </label>
        <textarea
          id="prompt"
          value={form.prompt}
          maxLength={PROMPT_MAX}
          rows={4}
          onChange={(e) => onChange("prompt", e.target.value)}
          className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-400"
          placeholder="想生成什么样的海报？"
        />
        <div className="text-xs text-slate-400 mt-1 text-right">
          {form.prompt.length} / {PROMPT_MAX}
        </div>
      </div>

      <div>
        <div className="text-sm font-medium text-slate-700 mb-2">预设</div>
        <div className="flex flex-wrap gap-2">
          {PRESETS.map((p) => (
            <button
              key={p.label}
              type="button"
              onClick={() => onPresetClick({ prompt: p.prompt, title: p.title })}
              className="text-xs px-3 py-1 rounded-full bg-slate-100 hover:bg-slate-200 text-slate-700"
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      <button
        type="submit"
        disabled={!canSubmit}
        className="w-full py-2 rounded bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 disabled:bg-slate-300 disabled:cursor-not-allowed"
      >
        {isLoading ? "生成中…" : "生成海报"}
      </button>
    </form>
  );
}
