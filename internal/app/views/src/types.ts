// Structured-content shapes for the Project Status view. Field names mirror
// internal/core/types.go exactly (snake_case JSON tags): the projections emit
// these as CallToolResult.structuredContent, and the view renders them verbatim.

export interface ProjectHealthSummary {
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

export interface ProjectHealthAttentionItem {
  kind: string;
  severity: string;
  status?: string;
  title: string;
  detail?: string;
  action_kind?: string;
  action_label?: string;
}

export interface ProjectHealth {
  project_id: string;
  status: string;
  title: string;
  detail?: string;
  summary: ProjectHealthSummary;
  attention?: ProjectHealthAttentionItem[];
}

export interface ProjectOperationsCounts {
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

export interface ProjectOperationItem {
  kind: string;
  severity: string;
  status?: string;
  title: string;
  detail?: string;
  action_kind?: string;
  action_label?: string;
}

export interface ProjectOperationsBrief {
  project_id: string;
  status: string;
  title: string;
  detail?: string;
  counts: ProjectOperationsCounts;
  next?: ProjectOperationItem;
  items?: ProjectOperationItem[];
}

export interface ProjectActivityCounts {
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

export interface ProjectActivityItem {
  bucket: string;
  work_item_id: string;
  work_item_title?: string;
  role_name?: string;
  status: string;
}

export interface ProjectActivityBuckets {
  active?: ProjectActivityItem[];
  blocked?: ProjectActivityItem[];
  completed?: ProjectActivityItem[];
  other?: ProjectActivityItem[];
  recent?: ProjectActivityItem[];
}

export interface ProjectActivity {
  project_id: string;
  counts: ProjectActivityCounts;
  buckets: ProjectActivityBuckets;
}

export type Structured = Record<string, unknown>;

export function isRecord(value: unknown): value is Structured {
  return typeof value === "object" && value !== null;
}
