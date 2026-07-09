// Headless render check for the Project Status view. It renders the built
// dist/project-status.html inside a sandboxed iframe (as a real host does) under
// the view's strict, no-unsafe-eval CSP, plays the host side of the SDK's
// ui/initialize handshake, delivers representative projects.health /
// projects.operations_brief / projects.activity results over the
// ui/notifications/tool-result contract, asserts the key text rendered, and
// writes a screenshot. It is a reproducibility aid, not part of `go test`.
//
// The view uses the @modelcontextprotocol/ext-apps App, a full JSON-RPC peer, so
// it must run in an iframe whose parent is the host: at top level window.parent
// is the view itself, which would answer its own ui/initialize with -32601. The
// iframe host below is the faithful arrangement.
//
// Run with bun (no separate Node step). Requires the `playwright` package and its
// Chromium build. In this repo:
//   NODE_PATH=/opt/node22/lib/node_modules \
//   PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers \
//   bun verify/verify.ts [screenshot-path]

import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";

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

// Host page: embeds the view file in a sandboxed iframe and plays the host side
// of the handshake. It must be a real file:// document so Chromium lets the
// iframe load the file:// view (a data:/about:blank parent cannot). The host
// answers ui/initialize with an McpUiInitializeResult, records readiness, and
// forwards tool-results into the iframe.
const hostHtml = `<!doctype html>
<html>
  <head><meta charset="utf-8" /><title>verify host</title></head>
  <body>
    <iframe
      id="view"
      sandbox="allow-scripts allow-same-origin"
      src="file://${htmlPath}"
      style="width: 520px; height: 940px; border: 0"
    ></iframe>
    <script>
      window.__hostLog = [];
      const view = document.getElementById("view");
      window.addEventListener("message", (event) => {
        const message = event.data;
        if (!message || message.jsonrpc !== "2.0") return;
        if (message.method === "ui/initialize") {
          window.__hostLog.push("initialize");
          view.contentWindow.postMessage(
            {
              jsonrpc: "2.0",
              id: message.id,
              result: {
                protocolVersion: "2026-01-26",
                hostCapabilities: {},
                hostInfo: { name: "cairnline-verify", version: "0.0.0" },
                hostContext: { displayMode: "inline" },
              },
            },
            "*",
          );
        } else if (message.method === "ui/notifications/initialized") {
          window.__hostLog.push("initialized");
        }
      });
      window.__deliver = (structuredContent) =>
        view.contentWindow.postMessage(
          { jsonrpc: "2.0", method: "ui/notifications/tool-result", params: { content: [], structuredContent } },
          "*",
        );
    </script>
  </body>
</html>`;
const hostFile = join(tmpdir(), `cairnline-verify-host-${process.pid}.html`);
writeFileSync(hostFile, hostHtml, "utf8");

const browser = await chromium.launch();
const page = await browser.newPage();

const errors = [];
page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
page.on("console", (m) => {
  if (m.type() === "error") errors.push(`console.error: ${m.text()}`);
});

await page.goto("file://" + hostFile, { waitUntil: "load" });

const viewFrame = () => page.frames().find((f) => f.url().includes("project-status"));
const deliver = (structuredContent) => page.evaluate((sc) => window.__deliver(sc), structuredContent);

// The view renders into Web Component shadow roots, so the rendered text is not
// reachable via a single element's textContent. These walkers cross every shadow
// boundary to collect what the operator actually sees. They run inside the view
// frame via Playwright's evaluate (which is not subject to the page CSP).
const deepText = () => {
  const walk = (node, acc) => {
    if (node.nodeType === 3) acc.push(node.nodeValue || "");
    const shadow = node.shadowRoot;
    if (shadow) walk(shadow, acc);
    for (const child of node.childNodes) walk(child, acc);
    return acc;
  };
  return walk(document.body, []).join(" ");
};
const deepIncludes = (needle) => {
  const walk = (node, acc) => {
    if (node.nodeType === 3) acc.push(node.nodeValue || "");
    const shadow = node.shadowRoot;
    if (shadow) walk(shadow, acc);
    for (const child of node.childNodes) walk(child, acc);
    return acc;
  };
  return walk(document.body, []).join(" ").includes(needle);
};

// The view must complete the request/response handshake before signaling ready:
// it should not post ui/notifications/initialized until the host answers
// ui/initialize.
await page.waitForFunction(
  () => {
    const log = window.__hostLog || [];
    return log.indexOf("initialize") !== -1 && log.indexOf("initialized") > log.indexOf("initialize");
  },
  null,
  { timeout: 15000 },
);

// Deliver each shape as the host would: a ui/notifications/tool-result whose
// params are a CallToolResult carrying structuredContent. All three share one
// project_id, so the same-project results accumulate into one combined view.
for (const structuredContent of [health, operations, activity]) {
  await deliver(structuredContent);
}

const frame = viewFrame();
await frame.waitForFunction(deepIncludes, "Activity", { timeout: 15000 });

const rendered = await frame.evaluate(deepText);
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

// State-bleed check: a result for a different project_id must reset the view so
// none of the first project's sections survive. Deliver a health result for a
// new project and assert the prior project's content is gone.
const otherHealth = {
  project_id: "proj_other",
  status: "clear",
  title: "Second project is clear",
  summary: { ...health.summary, attention_count: 0 },
  attention: [],
};
await deliver(otherHealth);
await frame.waitForFunction(deepIncludes, "Second project is clear", { timeout: 15000 });
const afterSwitch = await frame.evaluate(deepText);
const bled = [
  "3 items need attention", // first project's health title
  "Operations brief", // first project's operations section
  "Activity", // first project's activity section
  "Implement gateway retry",
].filter((text) => afterSwitch.includes(text));

await browser.close();
rmSync(hostFile, { force: true });

if (errors.length > 0) {
  console.error("runtime errors (CSP or script):\n" + errors.join("\n"));
  process.exit(1);
}
if (missing.length > 0) {
  console.error("missing expected text: " + JSON.stringify(missing));
  process.exit(1);
}
if (bled.length > 0) {
  console.error("state bled across projects; stale text after project switch: " + JSON.stringify(bled));
  process.exit(1);
}
console.log("OK: handshake ordered, all sections rendered, no cross-project bleed; screenshot -> " + screenshotPath);
