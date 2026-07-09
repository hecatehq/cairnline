import { beforeAll, describe, expect, it } from "vitest";
import { defineComponents } from "../define.js";
import { demoOperations } from "../fixtures.js";
import { collectScripts, deepText, loadGoldenStructuredContent } from "../test-utils.js";
import type { ProjectOperationsBrief } from "../types.js";
import type { OperationsBriefSection } from "./operations-brief-section.js";

function makeSection(brief: ProjectOperationsBrief): OperationsBriefSection {
  const element = document.createElement("operations-brief-section") as OperationsBriefSection;
  element.operations = brief;
  return element;
}

describe("OperationsBriefSection", () => {
  beforeAll(() => {
    defineComponents();
  });

  it("renders the status, title, counts, and item list", () => {
    const element = makeSection(demoOperations);
    const text = deepText(element);

    expect(text).toContain("Operations brief");
    expect(text).toContain("Next: unblock deploy approval");
    expect(text).toContain("1 blocked assignment leads the queue.");
    expect(text).toContain("Open work items");
    expect(text).toContain("Closeout ready");
    expect(text).toContain("Deploy to staging");
    expect(text).toContain("Address review follow-up");
  });

  it("renders from the projects.operations_brief golden structuredContent", () => {
    const golden = loadGoldenStructuredContent("operations_brief") as ProjectOperationsBrief;
    const element = makeSection(golden);
    const text = deepText(element);

    expect(text).toContain("Operations brief");
    expect(text).toContain("No project operations need attention");
    expect(text).toContain("clear");
  });

  it("clears its output when set back to null", () => {
    const element = makeSection(demoOperations);
    expect(deepText(element)).toContain("Operations brief");
    element.operations = null;
    expect(deepText(element)).not.toContain("Operations brief");
  });

  it("renders hostile item strings inert", () => {
    const element = makeSection({
      ...demoOperations,
      title: "<script>alert('ops')</script>",
      items: [
        {
          kind: "review",
          severity: "action",
          title: "<b>bold</b> & <script>alert(1)</script>",
          detail: "<img src=x onerror=alert(2)>",
        },
      ],
    });
    const text = deepText(element);
    expect(text).toContain("<script>alert('ops')</script>");
    expect(text).toContain("<b>bold</b> & <script>alert(1)</script>");
    expect(collectScripts(element)).toHaveLength(0);
  });
});
