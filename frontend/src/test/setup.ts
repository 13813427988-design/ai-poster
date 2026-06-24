import "@testing-library/jest-dom";

// Node 25 exposes an experimental global `localStorage` that lacks the
// real Storage methods and shadows the jsdom-provided window.localStorage.
// Install a working in-memory shim on both globalThis and window before
// each test so test code can call setItem / getItem / clear as usual.
import { beforeEach } from "vitest";

class MemoryStorage {
  private map = new Map<string, string>();
  get length() {
    return this.map.size;
  }
  clear() {
    this.map.clear();
  }
  getItem(key: string) {
    return this.map.has(key) ? this.map.get(key)! : null;
  }
  setItem(key: string, value: string) {
    this.map.set(String(key), String(value));
  }
  removeItem(key: string) {
    this.map.delete(key);
  }
  key(index: number) {
    return Array.from(this.map.keys())[index] ?? null;
  }
}

function installStorage() {
  const storage = new MemoryStorage();
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: storage,
  });
  if (typeof window !== "undefined") {
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      writable: true,
      value: storage,
    });
  }
}

installStorage();
beforeEach(() => {
  installStorage();
});
