import { beforeAll, describe, expect, it } from "vitest";
import { defineComponents } from "../define.js";
import { demoActivity } from "../fixtures.js";
import { collectScripts, deepText, loadGoldenStructuredContent } from "../test-utils.js";
import type { ProjectActivity } from "../types.js";
import type { ActivitySection } from "./activity-section.js";

function makeSection(activity: ProjectActivity): ActivitySection {
  const element = document.createElement("activity-section") as ActivitySection;
  element.activity = activity;
  return element;
}

describe("ActivitySection", () => {
  beforeAll(() => {
    defineComponents();
  });

  it("renders the count grid and each non-empty bucket", () => {
    const element = makeSection(demoActivity);
    const text = deepText(element);

    expect(text).toContain("Activity");
    expect(text).toContain("Assignments");
    expect(text).toContain("Awaiting approval");
    // Bucket headers carry their item counts.
    expect(text).toContain("Active (1)");
    expect(text).toContain("Blocked (1)");
    expect(text).toContain("Completed (1)");
    // Work item titles + role names.
    expect(text).toContain("Implement gateway retry");
    expect(text).toContain("Builder");
    expect(text).toContain("Deploy to staging");
  });

  it("renders from the projects.activity golden structuredContent (empty buckets)", () => {
    const golden = loadGoldenStructuredContent("activity") as ProjectActivity;
    const element = makeSection(golden);
    const text = deepText(element);

    expect(text).toContain("Activity");
    expect(text).toContain("Assignments");
    // No buckets in the fresh-project golden, so no bucket headers appear.
    expect(text).not.toContain("(1)");
  });

  it("clears its output when set back to null", () => {
    const element = makeSection(demoActivity);
    expect(deepText(element)).toContain("Activity");
    element.activity = null;
    expect(deepText(element)).not.toContain("Activity");
  });

  it("renders hostile work-item titles inert", () => {
    const element = makeSection({
      ...demoActivity,
      buckets: {
        active: [
          {
            bucket: "active",
            work_item_id: "work_x",
            work_item_title: "<script>alert(1)</script>",
            role_name: "<img src=x onerror=alert(2)>",
            status: "running",
          },
        ],
      },
    });
    const text = deepText(element);
    expect(text).toContain("<script>alert(1)</script>");
    expect(text).toContain("<img src=x onerror=alert(2)>");
    expect(collectScripts(element)).toHaveLength(0);
  });
});
