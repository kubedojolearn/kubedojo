import "@testing-library/jest-dom/vitest";

// jsdom has no IntersectionObserver; the landing page's scroll animations use it. Nothing ever
// intersects, so components keep the state they render on the server.
if (typeof globalThis.IntersectionObserver === "undefined") {
  class NoopIntersectionObserver {
    readonly root = null;
    readonly rootMargin = "0px";
    readonly thresholds: ReadonlyArray<number> = [];
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return [];
    }
  }
  globalThis.IntersectionObserver = NoopIntersectionObserver as unknown as typeof IntersectionObserver;
}
