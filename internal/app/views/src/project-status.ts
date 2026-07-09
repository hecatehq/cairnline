// Project Status view for Cairnline's MCP Apps (ui://) extension.
//
// One read-only view backs the projects.health, projects.operations_brief, and
// projects.activity tools. A single view can receive any of the three
// structuredContent shapes, so it detects which shape arrived by distinctive
// fields and renders that section. The view holds only light per-project state:
// results for the same project_id accumulate into one combined view, and the
// first result carrying a different project_id resets every section so two
// projects' results can never bleed together. All action_kind/action_label
// hints render as inert badges: this batch never calls tools back from the view.
//
// The host<->view bridge is the official @modelcontextprotocol/ext-apps SDK
// (io.modelcontextprotocol/ui). The App drives the ui/initialize handshake and
// delivers each tool result via app.ontoolresult; the view reads
// result.structuredContent. The SDK is bundled to ESM and loaded as an inline
// <script type="module">, which runs under the view's strict, no-unsafe-eval
// CSP: the App constructor sets zod to jitless mode, so no eval/new Function
// path is taken at runtime. (Bundling the SDK to an IIFE instead emits an
// undefined __require reference — a Bun IIFE-interop bug, not an SDK defect —
// which is why the build targets ESM.)

import { App } from "@modelcontextprotocol/ext-apps";

// Field names mirror internal/core/types.go exactly (snake_case JSON tags).

interface ProjectHealthSummary {
  attention_count: number;
  available_attention_count: number;
  omitted_attention_count: number;
  attention_limit: number;
  setup_todo_count: number;
  missing_project_root: boolean;
  open_handoff_count: number;
  review_follow_up_count: number;
  active_assignment_count: number;
  blocked_assignment_count: number;
  pending_memory_candidate_count: number;
  project_skill_issue_count: number;
}

interface ProjectHealthAttentionItem {
  kind: string;
  severity: string;
  status?: string;
  title: string;
  detail?: string;
  action_kind?: string;
  action_label?: string;
}

interface ProjectHealth {
  project_id: string;
  status: string;
  title: string;
  detail?: string;
  summary: ProjectHealthSummary;
  attention?: ProjectHealthAttentionItem[];
}

interface ProjectOperationsCounts {
  work_items: number;
  open_work_items: number;
  assignments: number;
  active_assignments: number;
  blocked_assignments: number;
  pending_memory_candidates: number;
  review_follow_ups: number;
  missing_evidence: number;
  open_handoffs: number;
  closeout_ready: number;
}

interface ProjectOperationItem {
  kind: string;
  severity: string;
  status?: string;
  title: string;
  detail?: string;
}

interface ProjectOperationsBrief {
  project_id: string;
  status: string;
  title: string;
  detail?: string;
  counts: ProjectOperationsCounts;
  next?: ProjectOperationItem;
  items?: ProjectOperationItem[];
}

interface ProjectActivityCounts {
  assignments: number;
  queued: number;
  claimed: number;
  running: number;
  awaiting_approval: number;
  awaiting_review: number;
  completed: number;
  failed: number;
  cancelled: number;
  other: number;
  active: number;
  blocked: number;
}

interface ProjectActivityItem {
  bucket: string;
  work_item_id: string;
  work_item_title?: string;
  role_name?: string;
  status: string;
}

interface ProjectActivityBuckets {
  active?: ProjectActivityItem[];
  blocked?: ProjectActivityItem[];
  completed?: ProjectActivityItem[];
  other?: ProjectActivityItem[];
  recent?: ProjectActivityItem[];
}

interface ProjectActivity {
  project_id: string;
  counts: ProjectActivityCounts;
  buckets: ProjectActivityBuckets;
}

type Structured = Record<string, unknown>;

// State is keyed on the project the sections describe. When a tool-result
// arrives for a different project_id than the one held, every section is
// cleared before the new shape is stored, so results from one project never
// bleed into another project's view.
const state: {
  projectId?: string;
  health?: ProjectHealth;
  operations?: ProjectOperationsBrief;
  activity?: ProjectActivity;
} = {};

function isRecord(value: unknown): value is Structured {
  return typeof value === "object" && value !== null;
}

// resetForProject clears accumulated sections when the incoming project_id does
// not match the project currently rendered. Same-project results merge; a new
// project replaces the view. Payloads without a project_id keep the current
// project (nothing to key on) rather than merging blindly.
function resetForProject(data: Structured): void {
  const projectId = typeof data.project_id === "string" ? data.project_id : undefined;
  if (projectId === undefined) return;
  if (state.projectId !== projectId) {
    state.projectId = projectId;
    state.health = undefined;
    state.operations = undefined;
    state.activity = undefined;
  }
}

// Detect a shape by a field unique to it: health carries `summary`, activity
// carries `buckets`, and the operations brief carries a `counts.work_items`
// counter that neither of the others has.
function classify(data: Structured): void {
  if (isRecord(data.summary)) {
    state.health = data as unknown as ProjectHealth;
  }
  if (isRecord(data.buckets)) {
    state.activity = data as unknown as ProjectActivity;
  }
  if (isRecord(data.counts) && "work_items" in (data.counts as Structured)) {
    state.operations = data as unknown as ProjectOperationsBrief;
  }
}

function el(tag: string, className?: string, text?: string): HTMLElement {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function badge(text: string, kind?: string): HTMLElement {
  return el("span", kind ? `badge ${kind}` : "badge", text);
}

function countGrid(entries: Array<[string, number]>): HTMLElement {
  const grid = el("div", "counts");
  for (const [label, value] of entries) {
    const count = el("div", "count");
    count.appendChild(el("span", "k", label));
    count.appendChild(el("span", "v", String(value)));
    grid.appendChild(count);
  }
  return grid;
}

function renderHealth(health: ProjectHealth): HTMLElement {
  const section = el("section");
  section.appendChild(el("h2", undefined, "Project health"));
  const card = el("div", "card");
  const head = el("div", "row");
  head.appendChild(badge(health.status, health.status));
  head.appendChild(el("span", "title", health.title));
  card.appendChild(head);
  if (health.detail) card.appendChild(el("p", "detail", health.detail));

  const s = health.summary;
  card.appendChild(
    countGrid([
      ["Attention", s.attention_count],
      ["Setup to-do", s.setup_todo_count],
      ["Active assignments", s.active_assignment_count],
      ["Blocked assignments", s.blocked_assignment_count],
      ["Open handoffs", s.open_handoff_count],
      ["Review follow-ups", s.review_follow_up_count],
      ["Pending memory", s.pending_memory_candidate_count],
      ["Skill issues", s.project_skill_issue_count],
    ]),
  );

  const attention = health.attention ?? [];
  if (attention.length > 0) {
    const list = el("ul");
    for (const item of attention) {
      const li = el("li", "item");
      const row = el("div", "row");
      row.appendChild(badge(item.severity, item.severity));
      row.appendChild(el("span", "title", item.title));
      // Inert affordance: action_label is shown as a plain badge, not a button.
      if (item.action_label) row.appendChild(badge(item.action_label, "action-label"));
      li.appendChild(row);
      if (item.detail) li.appendChild(el("div", "detail", item.detail));
      list.appendChild(li);
    }
    card.appendChild(list);
  }
  section.appendChild(card);
  return section;
}

function renderOperations(brief: ProjectOperationsBrief): HTMLElement {
  const section = el("section");
  section.appendChild(el("h2", undefined, "Operations brief"));
  const card = el("div", "card");
  const head = el("div", "row");
  head.appendChild(badge(brief.status, brief.status));
  head.appendChild(el("span", "title", brief.title));
  card.appendChild(head);
  if (brief.detail) card.appendChild(el("p", "detail", brief.detail));

  const c = brief.counts;
  card.appendChild(
    countGrid([
      ["Open work items", c.open_work_items],
      ["Active assignments", c.active_assignments],
      ["Blocked assignments", c.blocked_assignments],
      ["Review follow-ups", c.review_follow_ups],
      ["Missing evidence", c.missing_evidence],
      ["Open handoffs", c.open_handoffs],
      ["Pending memory", c.pending_memory_candidates],
      ["Closeout ready", c.closeout_ready],
    ]),
  );

  const items = brief.items ?? [];
  if (items.length > 0) {
    const list = el("ul");
    for (const item of items) {
      const li = el("li", "item");
      const row = el("div", "row");
      row.appendChild(badge(item.severity, item.severity));
      row.appendChild(el("span", "title", item.title));
      li.appendChild(row);
      if (item.detail) li.appendChild(el("div", "detail", item.detail));
      list.appendChild(li);
    }
    card.appendChild(list);
  }
  section.appendChild(card);
  return section;
}

function renderActivity(activity: ProjectActivity): HTMLElement {
  const section = el("section");
  section.appendChild(el("h2", undefined, "Activity"));
  const card = el("div", "card");
  const c = activity.counts;
  card.appendChild(
    countGrid([
      ["Assignments", c.assignments],
      ["Active", c.active],
      ["Blocked", c.blocked],
      ["Completed", c.completed],
      ["Awaiting approval", c.awaiting_approval],
      ["Awaiting review", c.awaiting_review],
    ]),
  );

  const buckets: Array<[string, ProjectActivityItem[] | undefined]> = [
    ["Active", activity.buckets.active],
    ["Blocked", activity.buckets.blocked],
    ["Completed", activity.buckets.completed],
    ["Other", activity.buckets.other],
  ];
  for (const [label, items] of buckets) {
    if (!items || items.length === 0) continue;
    const bucket = el("div", "bucket");
    bucket.appendChild(el("div", "bhead", `${label} (${items.length})`));
    const list = el("ul");
    for (const item of items) {
      const li = el("li", "item");
      const row = el("div", "row");
      row.appendChild(badge(item.status, item.bucket));
      row.appendChild(el("span", "title", item.work_item_title || item.work_item_id));
      if (item.role_name) row.appendChild(el("span", "detail", item.role_name));
      li.appendChild(row);
      list.appendChild(li);
    }
    bucket.appendChild(list);
    card.appendChild(bucket);
  }
  section.appendChild(card);
  return section;
}

function render(): void {
  const root = document.getElementById("root");
  if (!root) return;
  root.textContent = "";
  if (!state.health && !state.operations && !state.activity) {
    root.appendChild(el("p", "empty", "Waiting for project status data…"));
    return;
  }
  if (state.health) root.appendChild(renderHealth(state.health));
  if (state.operations) root.appendChild(renderOperations(state.operations));
  if (state.activity) root.appendChild(renderActivity(state.activity));
}

function ingest(structuredContent: unknown): void {
  if (!isRecord(structuredContent)) return;
  resetForProject(structuredContent);
  classify(structuredContent);
  render();
}

// Host <-> view bridge via the official SDK. new App() sets zod to jitless mode
// (allowUnsafeEval defaults false), so nothing here needs eval/new Function. The
// App performs the ui/initialize -> McpUiInitializeResult -> initialized
// handshake against window.parent; app.ontoolresult fires for each host
// ui/notifications/tool-result, whose CallToolResult carries structuredContent.
const app = new App(
  { name: "cairnline-project-status", version: "1.0.0" },
  { availableDisplayModes: ["inline", "fullscreen"] },
);

// Register the result handler before connecting so no early notification is
// missed, then complete the handshake.
app.ontoolresult = (result) => {
  ingest(result.structuredContent);
};

app.connect().catch((err: unknown) => {
  console.error("[project-status] app.connect failed", err);
});
