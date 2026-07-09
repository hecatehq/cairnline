// Project Status view for Cairnline's MCP Apps (ui://) extension.
//
// One stateless, read-only view backs the projects.health,
// projects.operations_brief, and projects.activity tools. A single view can
// receive any of the three structuredContent shapes, so it detects which shape
// arrived by distinctive fields and renders that section, keeping any sections
// already populated from earlier tool results. All action_kind/action_label
// hints render as inert badges: this batch never calls tools back from the view.
//
// The host<->view bridge is hand-rolled against the MCP Apps
// (io.modelcontextprotocol/ui) postMessage contract rather than the official
// @modelcontextprotocol/ext-apps SDK: that SDK does not bundle to a
// self-contained browser IIFE (a transitive dynamic require survives bundling)
// and pulls in eval/new Function code paths that the view's strict default-deny
// CSP forbids. The bridge below sends only the two messages this read-only view
// needs and reads params.structuredContent from ui/notifications/tool-result.

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

const state: {
  health?: ProjectHealth;
  operations?: ProjectOperationsBrief;
  activity?: ProjectActivity;
} = {};

function isRecord(value: unknown): value is Structured {
  return typeof value === "object" && value !== null;
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
  classify(structuredContent);
  render();
}

interface JsonRpcMessage {
  jsonrpc?: string;
  method?: string;
  params?: { structuredContent?: unknown };
}

const APP_NAME = "cairnline-project-status";
const APP_VERSION = "1.0.0";

function postToHost(message: Record<string, unknown>): void {
  // Views render inside a sandboxed iframe; the host is the parent window.
  window.parent.postMessage(message, "*");
}

// Host -> View: the host proxies a tool result to the view as a
// ui/notifications/tool-result JSON-RPC notification whose params are a standard
// MCP CallToolResult. Read structuredContent from it.
window.addEventListener("message", (event: MessageEvent) => {
  const message = event.data as JsonRpcMessage | null;
  if (!message || message.jsonrpc !== "2.0") return;
  if (message.method === "ui/notifications/tool-result") {
    ingest(message.params?.structuredContent);
  }
});

// View -> Host: announce the app during the initialize handshake, then signal
// readiness. A spec host responds to ui/initialize; this read-only view does not
// depend on the response, so it renders whenever the first tool-result arrives.
postToHost({
  jsonrpc: "2.0",
  id: 1,
  method: "ui/initialize",
  params: {
    appInfo: { name: APP_NAME, version: APP_VERSION },
    appCapabilities: { availableDisplayModes: ["inline", "fullscreen"] },
  },
});
postToHost({ jsonrpc: "2.0", method: "ui/notifications/initialized" });
