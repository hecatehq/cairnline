// Headless render check for the Project Status view. It loads the built
// dist/project-status.html in Chromium under the view's real (strict) CSP,
// delivers representative projects.health / projects.operations_brief /
// projects.activity results over the exact ui/notifications/tool-result
// postMessage contract the view implements, asserts the key text rendered, and
// writes a screenshot. It is a reproducibility aid, not part of `go test`.
//
// Requires the `playwright` package and its Chromium build. In this repo:
//   NODE_PATH=/opt/node22/lib/node_modules \
//   PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers \
//   node verify/verify.mjs [screenshot-path]

import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const require = createRequire(import.meta.url);
const { chromium } = require("playwright");

const here = dirname(fileURLToPath(import.meta.url));
const htmlPath = join(here, "..", "dist", "project-status.html");
// Screenshot path is overridable as argv[2]; defaults next to this script.
const screenshotPath = process.argv[2] || join(here, "project-status.png");

// Representative fixtures. Field names mirror internal/core/types.go. A real
// tool result carries one shape; the view renders each shape it has received,
// so delivering all three fills every section for the screenshot.
const health = {
  project_id: "proj_demo",
  status: "attention",
  title: "3 items need attention",
  detail: "Assignments and a handoff are waiting on you.",
  summary: {
    attention_count: 3,
    available_attention_count: 3,
    omitted_attention_count: 0,
    attention_limit: 5,
    setup_todo_count: 1,
    missing_project_root: false,
    open_handoff_count: 1,
    review_follow_up_count: 1,
    active_assignment_count: 2,
    blocked_assignment_count: 1,
    pending_memory_candidate_count: 2,
    project_skill_issue_count: 0,
  },
  attention: [
    {
      kind: "assignment",
      severity: "blocked",
      status: "awaiting_approval",
      title: "Approve deploy step",
      detail: "Assignment asg_9 is awaiting approval.",
      action_kind: "claim_assignment",
      action_label: "Start assignment",
    },
    {
      kind: "handoff",
      severity: "action",
      title: "Open handoff from Reviewer",
      action_kind: "create_handoff",
      action_label: "Review handoff",
    },
  ],
};

const operations = {
  project_id: "proj_demo",
  status: "attention",
  title: "Next: unblock deploy approval",
  detail: "1 blocked assignment leads the queue.",
  counts: {
    work_items: 6,
    open_work_items: 4,
    assignments: 5,
    active_assignments: 2,
    blocked_assignments: 1,
    pending_memory_candidates: 2,
    review_follow_ups: 1,
    missing_evidence: 1,
    open_handoffs: 1,
    closeout_ready: 1,
  },
  items: [
    {
      kind: "assignment",
      severity: "blocked",
      status: "awaiting_approval",
      title: "Deploy to staging",
      detail: "Blocked on approval.",
    },
    {
      kind: "review",
      severity: "action",
      title: "Address review follow-up",
    },
  ],
};

const activity = {
  project_id: "proj_demo",
  counts: {
    assignments: 5,
    queued: 1,
    claimed: 1,
    running: 1,
    awaiting_approval: 1,
    awaiting_review: 0,
    completed: 1,
    failed: 0,
    cancelled: 0,
    other: 0,
    active: 2,
    blocked: 2,
  },
  buckets: {
    active: [
      {
        bucket: "active",
        work_item_id: "work_1",
        work_item_title: "Implement gateway retry",
        role_name: "Builder",
        status: "running",
      },
    ],
    blocked: [
      {
        bucket: "blocked",
        work_item_id: "work_2",
        work_item_title: "Deploy to staging",
        role_name: "Operator",
        status: "awaiting_approval",
      },
    ],
    completed: [
      {
        bucket: "completed",
        work_item_id: "work_3",
        work_item_title: "Write integration tests",
        role_name: "Builder",
        status: "completed",
      },
    ],
  },
};

const browser = await chromium.launch();
const page = await browser.newPage();

const errors = [];
page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
page.on("console", (m) => {
  if (m.type() === "error") errors.push(`console.error: ${m.text()}`);
});

await page.goto("file://" + htmlPath, { waitUntil: "load" });

// Deliver each shape as the host would: a ui/notifications/tool-result whose
// params are a CallToolResult carrying structuredContent.
await page.evaluate((fixtures) => {
  for (const structuredContent of fixtures) {
    window.postMessage(
      {
        jsonrpc: "2.0",
        method: "ui/notifications/tool-result",
        params: { content: [], structuredContent },
      },
      "*",
    );
  }
}, [health, operations, activity]);

await page.waitForFunction(() => {
  const root = document.getElementById("root");
  return !!root && root.textContent.includes("Activity");
});

const rendered = await page.evaluate(() => document.getElementById("root").textContent);
const expected = [
  "Project health",
  "3 items need attention",
  "Start assignment",
  "Operations brief",
  "Next: unblock deploy approval",
  "Activity",
  "Implement gateway retry",
];
const missing = expected.filter((text) => !rendered.includes(text));

await page.screenshot({ path: screenshotPath, fullPage: true });
await browser.close();

if (errors.length > 0) {
  console.error("runtime errors (CSP or script):\n" + errors.join("\n"));
  process.exit(1);
}
if (missing.length > 0) {
  console.error("missing expected text: " + JSON.stringify(missing));
  process.exit(1);
}
console.log("OK: all sections rendered; screenshot -> " + screenshotPath);
