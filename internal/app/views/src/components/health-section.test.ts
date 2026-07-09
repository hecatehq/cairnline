import { beforeAll, describe, expect, it } from "vitest";
import { defineComponents } from "../define.js";
import { demoHealth, injectionHealth } from "../fixtures.js";
import { collectScripts, deepText, loadGoldenStructuredContent } from "../test-utils.js";
import type { ProjectHealth } from "../types.js";
import type { HealthSection } from "./health-section.js";

function makeHealthSection(health: ProjectHealth): HealthSection {
  const element = document.createElement("health-section") as HealthSection;
  element.health = health;
  return element;
}

describe("HealthSection", () => {
  beforeAll(() => {
    defineComponents();
  });

  it("renders the status, title, summary counts, and attention items", () => {
    const element = makeHealthSection(demoHealth);
    const text = deepText(element);

    expect(text).toContain("Project health");
    expect(text).toContain("attention");
    expect(text).toContain("3 items need attention");
    expect(text).toContain("Assignments and a handoff are waiting on you.");
    // Summary grid labels + a representative value.
    expect(text).toContain("Active assignments");
    expect(text).toContain("Blocked assignments");
    // Attention item with its inert action badge.
    expect(text).toContain("Approve deploy step");
    expect(text).toContain("Start assignment");
  });

  it("renders from the projects.health golden structuredContent", () => {
    const golden = loadGoldenStructuredContent("health") as ProjectHealth;
    const element = makeHealthSection(golden);
    const text = deepText(element);

    expect(text).toContain("Project needs attention");
    expect(text).toContain("1 project coordination item needs operator attention.");
    // The golden attention item carries an action_label, rendered as an inert badge.
    expect(text).toContain("Set up project");
  });

  it("clears its output when set back to null", () => {
    const element = makeHealthSection(demoHealth);
    expect(deepText(element)).toContain("Project health");
    element.health = null;
    expect(deepText(element)).not.toContain("Project health");
  });

  it("renders a hostile payload inert (literal text, no script element)", () => {
    const element = makeHealthSection(injectionHealth);
    const text = deepText(element);

    // The markup-shaped strings appear as literal text...
    expect(text).toContain("<script>alert(1)</script>");
    expect(text).toContain("<img src=x onerror=alert(2)> & <b>bold</b>");
    expect(text).toContain("</span><script>alert(4)</script>");
    // ...and no <script> element was ever created in the subtree.
    expect(collectScripts(element)).toHaveLength(0);
    // No <img> was parsed from the payload either.
    const imgs: Element[] = [];
    const shadow = element.shadowRoot;
    if (shadow) imgs.push(...Array.from(shadow.querySelectorAll("img")));
    expect(imgs).toHaveLength(0);
  });
});
