import type { Status } from "../reducer";
import type { HistoryItem } from "../types";

type Props = {
  status: Status;
  current: HistoryItem | null;
  error: string | null;
};

export function Preview({ status, current, error }: Props) {
  return (
    <div className="p-4 bg-white rounded-lg shadow-sm border border-slate-200 min-h-[400px] flex flex-col">
      {status === "loading" && (
        <div
          data-testid="preview-skeleton"
          className="flex-1 flex items-center justify-center bg-slate-100 rounded animate-pulse"
        >
          <div className="w-8 h-8 border-4 border-slate-300 border-t-blue-500 rounded-full animate-spin" />
        </div>
      )}

      {status === "error" && (
        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded p-3">
          生成失败：{error ?? "未知错误"}
        </div>
      )}

      {status === "success" && current && (
        <div className="flex-1 flex flex-col gap-3">
          <img
            src={current.url}
            alt={current.title}
            className="max-w-full rounded shadow-sm"
          />
          <div className="text-sm text-slate-600">
            <div className="font-medium">{current.title}</div>
            <div className="text-xs text-slate-400">{current.prompt}</div>
          </div>
          <a
            href={current.url}
            download
            className="inline-block self-start px-3 py-1 text-xs rounded bg-slate-100 hover:bg-slate-200"
          >
            下载海报
          </a>
        </div>
      )}

      {status === "idle" && !current && (
        <div className="flex-1 flex items-center justify-center border-2 border-dashed border-slate-200 rounded text-slate-400 text-sm">
          在左侧填写后点"生成海报"
        </div>
      )}
    </div>
  );
}
