Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});

Object.defineProperty(window, "ResizeObserver", {
  writable: true,
  value: class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
});

// jsdom does not implement scrollIntoView, which Mantine's Combobox/Select
// calls when an option list opens. A no-op keeps those interactions from
// raising unhandled exceptions during tests.
Object.defineProperty(Element.prototype, "scrollIntoView", {
  writable: true,
  value: () => {},
});
