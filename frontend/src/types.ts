export type GenerateRequest = { prompt: string; title: string };
export type GenerateResponse = { url: string };
export type HistoryItem = {
  id: string;
  prompt: string;
  title: string;
  url: string;
  createdAt: number;
};
