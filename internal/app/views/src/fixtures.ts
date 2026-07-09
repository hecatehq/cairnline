import type { ProjectActivity, ProjectHealth, ProjectOperationsBrief } from "./types.js";

// Representative structuredContent fixtures. Field names mirror
// internal/core/types.go. They feed both the preview harness (so the view can be
// exercised outside an MCP host) and the component tests, keeping the two in sync.

export const demoHealth: ProjectHealth = {
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

export const demoOperations: ProjectOperationsBrief = {
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

export const demoActivity: ProjectActivity = {
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

// A second project's health, used to demonstrate the per-project reset: feeding
// this after the demo project must clear every demo section.
export const otherHealth: ProjectHealth = {
  project_id: "proj_other",
  status: "clear",
  title: "Second project is clear",
  summary: { ...demoHealth.summary, attention_count: 0 },
  attention: [],
};

// A hostile payload used to demonstrate injection-safety: the markup-shaped
// strings must render as literal text, never as elements.
export const injectionHealth: ProjectHealth = {
  project_id: "proj_xss",
  status: "attention",
  title: "<script>alert(1)</script>",
  detail: "<img src=x onerror=alert(2)> & <b>bold</b>",
  summary: { ...demoHealth.summary, attention_count: 1 },
  attention: [
    {
      kind: "assignment",
      severity: "<svg/onload=alert(3)>",
      title: "</span><script>alert(4)</script>",
      detail: "plain & <i>text</i>",
      action_label: "<script>alert(5)</script>",
    },
  ],
};

export interface Fixture {
  readonly label: string;
  readonly payloads: readonly unknown[];
}

// Named fixtures for the preview selector. Each entry is a sequence of tool
// results delivered in order, mirroring how a host streams them.
export const previewFixtures: readonly Fixture[] = [
  { label: "All sections (demo project)", payloads: [demoHealth, demoOperations, demoActivity] },
  { label: "Health only", payloads: [demoHealth] },
  { label: "Operations brief only", payloads: [demoOperations] },
  { label: "Activity only", payloads: [demoActivity] },
  {
    label: "Project switch (no bleed)",
    payloads: [demoHealth, demoOperations, demoActivity, otherHealth],
  },
  { label: "Injection payload (renders inert)", payloads: [injectionHealth] },
];
