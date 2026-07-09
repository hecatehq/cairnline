import type { ProjectStatusApp } from "./components/project-status-app.js";
import { previewFixtures } from "./fixtures.js";

// Preview harness for developing the view OUTSIDE an MCP host. It is loaded only
// under `vite` / `vite dev` (import.meta.env.DEV) and is dead-code-eliminated
// from the embedded production bundle. It renders a small fixture selector that
// feeds the same structuredContent a host would deliver via app.ontoolresult,
// straight into <project-status-app>.ingest — so the components can be exercised
// with a click, no MCP host required.
export function mountPreview(): void {
  const app = document.querySelector<ProjectStatusApp>("project-status-app");
  if (!app) return;

  const bar = document.createElement("div");
  bar.style.cssText =
    "display:flex;gap:8px;align-items:center;margin:0 0 16px;padding:8px 12px;border:1px dashed var(--border);border-radius:8px;font:13px system-ui,sans-serif;color:var(--muted)";

  const label = document.createElement("span");
  label.textContent = "Preview fixture:";

  const select = document.createElement("select");
  select.style.cssText =
    "font:inherit;color:var(--fg);background:var(--card);border:1px solid var(--border);border-radius:6px;padding:4px 8px";
  for (const [index, fixture] of previewFixtures.entries()) {
    const option = document.createElement("option");
    option.value = String(index);
    option.textContent = fixture.label;
    select.append(option);
  }

  const note = document.createElement("span");
  note.textContent = "(feeds structuredContent directly, no MCP host)";

  const apply = (index: number): void => {
    const fixture = previewFixtures[index];
    if (!fixture) return;
    // Reset to the empty state, then replay the fixture's tool results in order,
    // exactly as a host would deliver them.
    app.reset();
    for (const payload of fixture.payloads) app.ingest(payload);
  };

  select.addEventListener("change", () => {
    apply(Number(select.value));
  });

  bar.append(label, select, note);
  document.body.insertBefore(bar, app);

  // Show the first fixture immediately so the preview is populated on load.
  apply(0);
}
