import type { HistoryItem } from "../types";

type Props = {
  items: HistoryItem[];
  onSelect: (item: HistoryItem) => void;
  onClear: () => void;
};

export function History({ items, onSelect, onClear }: Props) {
  return (
    <div className="p-4 bg-white rounded-lg shadow-sm border border-slate-200">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-medium text-slate-700">历史记录</h2>
        {items.length > 0 && (
          <button
            type="button"
            onClick={() => {
              if (window.confirm("确定清空全部历史？")) onClear();
            }}
            className="text-xs px-2 py-1 rounded text-slate-500 hover:bg-slate-100"
          >
            清空
          </button>
        )}
      </div>

      {items.length === 0 ? (
        <div className="text-sm text-slate-400 py-6 text-center">
          暂无历史
        </div>
      ) : (
        <div className="grid grid-cols-4 md:grid-cols-6 gap-3">
          {items.map((it) => (
            <button
              key={it.id}
              type="button"
              onClick={() => onSelect(it)}
              className="block w-full aspect-square overflow-hidden rounded border border-slate-200 hover:border-blue-400"
              title={`${it.title} — ${it.prompt}`}
            >
              <img
                src={it.url}
                alt={it.title}
                className="w-full h-full object-cover"
              />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
