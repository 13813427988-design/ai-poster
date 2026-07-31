// Local identifier for history entries: a React key and a localStorage record
// id. Needs no cryptographic strength and no global uniqueness — only
// uniqueness within one browser's history list.
//
// crypto.randomUUID is exposed only in a secure context (HTTPS or localhost).
// The app is served over plain HTTP, so feature-detect the function itself and
// fall back when the browser withholds it.

let counter = 0;

export function newId(): string {
  if (typeof crypto?.randomUUID === "function") {
    return crypto.randomUUID();
  }
  counter += 1;
  if (typeof crypto?.getRandomValues === "function") {
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
    return `${hex}-${counter}`;
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${counter}`;
}
