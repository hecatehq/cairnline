import { beforeAll, describe, expect, it } from "vitest";
import { defineComponents } from "../define.js";
import {
  demoActivity,
  demoHealth,
  demoOperations,
  injectionHealth,
  otherHealth,
} from "../fixtures.js";
import { collectScripts, deepText, loadGoldenStructuredContent } from "../test-utils.js";
import type { ProjectStatusApp } from "./project-status-app.js";

// The root element is created but not appended to the document, so connectedCallback
// (and the ext-apps host handshake) does not run: the tests drive it purely through
// its ingest() entry point, exactly as the host bridge and preview harness do.
function makeApp(): ProjectStatusApp {
  return document.createElement("project-status-app") as ProjectStatusApp;
}

describe("ProjectStatusApp", () => {
  beforeAll(() => {
    defineComponents();
  });

  it("shows the waiting placeholder before any data arrives", () => {
    const app = makeApp();
    expect(deepText(app)).toContain("Waiting for project status data");
  });

  it("accumulates same-project results into one combined view", () => {
    const app = makeApp();
    app.ingest(demoHealth);
    app.ingest(demoOperations);
    app.ingest(demoActivity);
    const text = deepText(app);

    expect(text).toContain("Project health");
    expect(text).toContain("Operations brief");
    expect(text).toContain("Activity");
    expect(text).toContain("3 items need attention");
    expect(text).toContain("Next: unblock deploy approval");
    expect(text).toContain("Implement gateway retry");
    expect(text).not.toContain("Waiting for project status data");
  });

  it("renders each of the three golden structuredContent shapes", () => {
    const app = makeApp();
    app.ingest(loadGoldenStructuredContent("health"));
    app.ingest(loadGoldenStructuredContent("operations_brief"));
    app.ingest(loadGoldenStructuredContent("activity"));
    const text = deepText(app);

    expect(text).toContain("Project needs attention");
    expect(text).toContain("No project operations need attention");
    expect(text).toContain("Activity");
  });

  it("resets every section when a result for a different project arrives (no bleed)", () => {
    const app = makeApp();
    app.ingest(demoHealth);
    app.ingest(demoOperations);
    app.ingest(demoActivity);
    expect(deepText(app)).toContain("3 items need attention");

    // A new project's health must clear the prior project's sections entirely.
    app.ingest(otherHealth);
    const text = deepText(app);

    expect(text).toContain("Second project is clear");
    // None of project A's content survives.
    expect(text).not.toContain("3 items need attention");
    expect(text).not.toContain("Operations brief");
    expect(text).not.toContain("Activity");
    expect(text).not.toContain("Implement gateway retry");
  });

  it("keeps the view when a payload carries no project_id", () => {
    const app = makeApp();
    app.ingest(demoHealth);
    // No project_id to key on: this merges rather than resetting.
    app.ingest(demoActivity);
    const text = deepText(app);
    expect(text).toContain("Project health");
    expect(text).toContain("Activity");
  });

  it("ignores non-record payloads", () => {
    const app = makeApp();
    app.ingest(demoHealth);
    app.ingest(null);
    app.ingest("nope");
    app.ingest(42);
    expect(deepText(app)).toContain("Project health");
  });

  it("renders a hostile payload delivered through ingest inert", () => {
    const app = makeApp();
    app.ingest(injectionHealth);
    const text = deepText(app);
    expect(text).toContain("<script>alert(1)</script>");
    expect(collectScripts(app)).toHaveLength(0);
  });

  it("returns to the placeholder after reset()", () => {
    const app = makeApp();
    app.ingest(demoHealth);
    expect(deepText(app)).toContain("Project health");
    app.reset();
    expect(deepText(app)).toContain("Waiting for project status data");
  });
});
