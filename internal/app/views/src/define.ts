import { ActivitySection } from "./components/activity-section.js";
import { HealthSection } from "./components/health-section.js";
import { OperationsBriefSection } from "./components/operations-brief-section.js";
import { ProjectStatusApp } from "./components/project-status-app.js";

// defineComponents registers every custom element exactly once. Both the runtime
// entry (main.ts) and the component tests call it; the guard keeps re-registration
// (which would throw) safe when a test suite imports it repeatedly.
export function defineComponents(): void {
  const registry: Array<[string, CustomElementConstructor]> = [
    ["health-section", HealthSection],
    ["operations-brief-section", OperationsBriefSection],
    ["activity-section", ActivitySection],
    ["project-status-app", ProjectStatusApp],
  ];
  for (const [tag, ctor] of registry) {
    if (!customElements.get(tag)) customElements.define(tag, ctor);
  }
}
