import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// Test helpers shared by the component suites. This file is not itself a test
// (vitest only picks up *.test.ts), but it is typechecked as part of src.

const here = dirname(fileURLToPath(import.meta.url));

// goldenDir points at the Go-side status-app golden fixtures. The tests read the
// exact structuredContent the projections emit, so the view stays in sync with
// the backend contract rather than duplicating fixtures.
const goldenDir = join(here, "..", "..", "testdata", "status_app");

// loadGoldenStructuredContent reads a committed golden tool-call result and
// returns its structuredContent — the payload a host delivers to the view.
export function loadGoldenStructuredContent(name: string): unknown {
  const raw = readFileSync(join(goldenDir, `${name}.golden.json`), "utf8");
  const parsed = JSON.parse(raw) as { structuredContent?: unknown };
  return parsed.structuredContent;
}

// deepText collects the visible text of a node and every shadow root beneath it.
// The components render into shadow DOM, so plain textContent stops at the
// boundary; this walker crosses it to assert what the operator actually sees.
export function deepText(node: Node): string {
  const parts: string[] = [];
  const walk = (current: Node): void => {
    if (current.nodeType === current.TEXT_NODE) {
      parts.push(current.nodeValue ?? "");
    }
    const shadow = (current as Element).shadowRoot;
    if (shadow) walk(shadow);
    for (const child of Array.from(current.childNodes)) walk(child);
  };
  walk(node);
  return parts.join(" ");
}

// collectScripts returns every <script> element in the subtree, crossing shadow
// boundaries. The injection-safety tests assert this stays empty: no payload
// string is ever parsed into a <script> element.
export function collectScripts(node: Node): Element[] {
  const found: Element[] = [];
  const walk = (current: Node): void => {
    if ((current as Element).nodeName?.toLowerCase() === "script") {
      found.push(current as Element);
    }
    const shadow = (current as Element).shadowRoot;
    if (shadow) walk(shadow);
    for (const child of Array.from(current.childNodes)) walk(child);
  };
  walk(node);
  return found;
}
