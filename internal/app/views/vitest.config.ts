import { defineConfig } from "vitest/config";

// Component tests exercise the custom elements against a DOM. happy-dom provides
// customElements, shadow DOM, and constructable stylesheets, so the components
// render exactly as they do in a host without a browser.
export default defineConfig({
  test: {
    environment: "happy-dom",
    globals: true,
    include: ["src/**/*.test.ts"],
    testTimeout: 10_000,
  },
});
