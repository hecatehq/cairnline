// Shared structural styles for every Project Status component. The palette lives
// in index.html's :root and inherits through the shadow boundary, so these rules
// reference the --fg / --muted / … tokens without redeclaring them. One stylesheet
// is adopted by every component's shadow root; unused selectors per component are
// harmless. The stylesheet is static and data-free, so injecting it via a
// constructable sheet or an inline <style> is permitted by style-src 'unsafe-inline'.
export const componentStyles = `
  * {
    box-sizing: border-box;
  }
  :host {
    display: block;
  }
  h1 {
    font-size: 16px;
    margin: 0 0 12px;
  }
  h2 {
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--muted);
    margin: 0 0 8px;
  }
  section {
    margin: 0 0 18px;
  }
  .card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px;
  }
  .row {
    display: flex;
    gap: 8px;
    align-items: baseline;
    flex-wrap: wrap;
  }
  .title {
    font-weight: 600;
  }
  .detail {
    color: var(--muted);
  }
  .badge {
    display: inline-block;
    font-size: 11px;
    font-weight: 600;
    padding: 1px 7px;
    border-radius: 999px;
    border: 1px solid var(--border);
    color: var(--muted);
    white-space: nowrap;
  }
  .badge.clear {
    color: var(--clear);
    border-color: var(--clear);
  }
  .badge.attention,
  .badge.action,
  .badge.ready,
  .badge.active {
    color: var(--attention);
    border-color: var(--attention);
  }
  .badge.blocked {
    color: var(--blocked);
    border-color: var(--blocked);
  }
  .badge.action-label {
    color: var(--accent);
    border-color: var(--accent);
  }
  .counts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 6px 12px;
    margin-top: 8px;
  }
  .count {
    display: flex;
    justify-content: space-between;
    gap: 8px;
  }
  .count .k {
    color: var(--muted);
  }
  .count .v {
    font-variant-numeric: tabular-nums;
    font-weight: 600;
  }
  ul {
    list-style: none;
    margin: 8px 0 0;
    padding: 0;
  }
  li.item {
    padding: 8px 0;
    border-top: 1px solid var(--border);
  }
  li.item:first-child {
    border-top: none;
  }
  .bucket {
    margin-top: 10px;
  }
  .bucket > .bhead {
    font-weight: 600;
    margin-bottom: 4px;
  }
  .empty {
    color: var(--muted);
  }
`;

// A single constructable stylesheet shared across every shadow root; built lazily
// and reused so the palette + layout is defined once.
let sharedSheet: CSSStyleSheet | null | undefined;

function buildSharedSheet(): CSSStyleSheet | null {
  try {
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(componentStyles);
    return sheet;
  } catch {
    // Some non-browser DOM implementations lack constructable stylesheets.
    return null;
  }
}

// applyComponentStyles wires the shared structural styles into a component's
// shadow root. It prefers an adopted constructable stylesheet (blessed by the
// view's style-src 'unsafe-inline' CSP) and falls back to an inline <style>
// element for DOM implementations without adoptedStyleSheets. Neither path uses
// payload data.
export function applyComponentStyles(root: ShadowRoot): void {
  const supportsAdopted = "adoptedStyleSheets" in root && typeof CSSStyleSheet !== "undefined";
  if (supportsAdopted) {
    if (sharedSheet === undefined) sharedSheet = buildSharedSheet();
    if (sharedSheet) {
      root.adoptedStyleSheets = [...root.adoptedStyleSheets, sharedSheet];
      return;
    }
  }
  const style = document.createElement("style");
  style.textContent = componentStyles; // static, data-free CSS text
  root.append(style);
}
