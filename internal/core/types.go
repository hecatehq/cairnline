package core

import "time"

type Project struct {
	ID                        string    `json:"id"`
	Name                      string    `json:"name"`
	Description               string    `json:"description,omitempty"`
	Roots                     []Root    `json:"roots,omitempty"`
	DefaultRootID             string    `json:"default_root_id,omitempty"`
	DefaultProfileID          string    `json:"default_profile_id,omitempty"`
	DefaultExecutionProfileID string    `json:"default_execution_profile_id,omitempty"`
	ContextSources            []Source  `json:"context_sources,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

type Root struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Kind      string `json:"kind,omitempty"`
	GitRemote string `json:"git_remote,omitempty"`
	GitBranch string `json:"git_branch,omitempty"`
	Active    bool   `json:"active"`
}

type Source struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Title          string            `json:"title"`
	Locator        string            `json:"locator,omitempty"`
	Enabled        bool              `json:"enabled"`
	Format         string            `json:"format,omitempty"`
	Scope          string            `json:"scope,omitempty"`
	TrustLabel     string            `json:"trust_label,omitempty"`
	SourceCategory string            `json:"source_category,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at,omitempty"`
	UpdatedAt      time.Time         `json:"updated_at,omitempty"`
}

type ProjectSkill struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	Path         string    `json:"path"`
	RootID       string    `json:"root_id,omitempty"`
	Format       string    `json:"format"`
	Enabled      bool      `json:"enabled"`
	Status       string    `json:"status"`
	TrustLabel   string    `json:"trust_label"`
	SourceRefs   []string  `json:"source_refs,omitempty"`
	Warnings     []string  `json:"warnings,omitempty"`
	DiscoveredAt time.Time `json:"discovered_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Role struct {
	ID                        string   `json:"id"`
	ProjectID                 string   `json:"project_id"`
	Name                      string   `json:"name"`
	Description               string   `json:"description,omitempty"`
	Instructions              string   `json:"instructions,omitempty"`
	DefaultProfileID          string   `json:"default_profile_id,omitempty"`
	DefaultExecutionProfileID string   `json:"default_execution_profile_id,omitempty"`
	DefaultSkillIDs           []string `json:"default_skill_ids,omitempty"`
	DefaultExecutionMode      string   `json:"default_execution_mode,omitempty"`
}

type AgentProfile struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Instructions  string    `json:"instructions,omitempty"`
	ContextPolicy string    `json:"context_policy,omitempty"`
	MemoryPolicy  string    `json:"memory_policy,omitempty"`
	SourcePolicy  string    `json:"source_policy,omitempty"`
	SkillIDs      []string  `json:"skill_ids,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ExecutionProfile struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	AgentKind      string         `json:"agent_kind,omitempty"`
	ModelHint      string         `json:"model_hint,omitempty"`
	ProviderHint   string         `json:"provider_hint,omitempty"`
	ToolsPolicy    string         `json:"tools_policy,omitempty"`
	WritesPolicy   string         `json:"writes_policy,omitempty"`
	NetworkPolicy  string         `json:"network_policy,omitempty"`
	ApprovalPolicy string         `json:"approval_policy,omitempty"`
	AdapterOptions map[string]any `json:"adapter_options,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type WorkItem struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	Title           string    `json:"title"`
	Brief           string    `json:"brief,omitempty"`
	Status          string    `json:"status"`
	Priority        string    `json:"priority"`
	OwnerRoleID     string    `json:"owner_role_id,omitempty"`
	ReviewerRoleIDs []string  `json:"reviewer_role_ids,omitempty"`
	RootID          string    `json:"root_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type WorkItemCloseoutReadiness struct {
	ProjectID                    string                    `json:"project_id"`
	WorkItemID                   string                    `json:"work_item_id"`
	Ready                        bool                      `json:"ready"`
	Status                       string                    `json:"status"`
	Title                        string                    `json:"title"`
	Detail                       string                    `json:"detail"`
	Blockers                     []string                  `json:"blockers,omitempty"`
	Warnings                     []string                  `json:"warnings,omitempty"`
	AssignmentCount              int                       `json:"assignment_count"`
	CompletedAssignments         int                       `json:"completed_assignments"`
	ReviewFollowUpCount          int                       `json:"review_follow_up_count"`
	ReviewFollowUpArtifactIDs    []string                  `json:"review_follow_up_artifact_ids,omitempty"`
	ReviewFollowUps              []ReviewFollowUpReadiness `json:"review_follow_ups,omitempty"`
	MissingEvidenceAssignmentIDs []string                  `json:"missing_evidence_assignment_ids,omitempty"`
	OpenHandoffIDs               []string                  `json:"open_handoff_ids,omitempty"`
}

type ProjectSetupReadiness struct {
	ProjectID      string                       `json:"project_id"`
	ShowOnboarding bool                         `json:"show_onboarding"`
	SetupStarted   bool                         `json:"setup_started"`
	FirstWorkReady bool                         `json:"first_work_ready"`
	Summary        ProjectSetupReadinessSummary `json:"summary"`
	PrimaryAction  ProjectSetupReadinessAction  `json:"primary_action"`
	Checks         []ProjectSetupReadinessCheck `json:"checks"`
	CreatedAt      time.Time                    `json:"created_at"`
}

type ProjectSetupReadinessSummary struct {
	WorkItemCount               int  `json:"work_item_count"`
	RoleCount                   int  `json:"role_count"`
	SkillCount                  int  `json:"skill_count"`
	ExecutionProfileCount       int  `json:"execution_profile_count"`
	EnabledContextSourceCount   int  `json:"enabled_context_source_count"`
	SavedMemoryCount            int  `json:"saved_memory_count"`
	PendingMemoryCandidateCount int  `json:"pending_memory_candidate_count"`
	HasPurpose                  bool `json:"has_purpose"`
	HasActiveRoot               bool `json:"has_active_root"`
	HasExecutionProfile         bool `json:"has_execution_profile"`
}

type ProjectSetupReadinessCheck struct {
	ID       string                       `json:"id"`
	Label    string                       `json:"label"`
	Detail   string                       `json:"detail"`
	Status   string                       `json:"status"`
	Optional bool                         `json:"optional,omitempty"`
	Action   *ProjectSetupReadinessAction `json:"action,omitempty"`
}

type ProjectSetupReadinessAction struct {
	Kind      string `json:"kind"`
	ProjectID string `json:"project_id"`
	Label     string `json:"label"`
}

type ProjectHealth struct {
	ProjectID string                       `json:"project_id"`
	Status    string                       `json:"status"`
	Title     string                       `json:"title"`
	Detail    string                       `json:"detail,omitempty"`
	Summary   ProjectHealthSummary         `json:"summary"`
	Attention []ProjectHealthAttentionItem `json:"attention,omitempty"`
	CreatedAt time.Time                    `json:"created_at"`
}

type ProjectHealthSummary struct {
	AttentionCount               int  `json:"attention_count"`
	AvailableAttentionCount      int  `json:"available_attention_count"`
	OmittedAttentionCount        int  `json:"omitted_attention_count"`
	AttentionLimit               int  `json:"attention_limit"`
	SetupTodoCount               int  `json:"setup_todo_count"`
	MissingProjectRoot           bool `json:"missing_project_root"`
	HasExecutionProfile          bool `json:"has_execution_profile"`
	EnabledMemoryCount           int  `json:"enabled_memory_count"`
	SavedMemoryCount             int  `json:"saved_memory_count"`
	EnabledContextSourceCount    int  `json:"enabled_context_source_count"`
	PendingMemoryCandidateCount  int  `json:"pending_memory_candidate_count"`
	PromotedMemoryCandidateCount int  `json:"promoted_memory_candidate_count"`
	RejectedMemoryCandidateCount int  `json:"rejected_memory_candidate_count"`
	OpenHandoffCount             int  `json:"open_handoff_count"`
	AcceptedHandoffCount         int  `json:"accepted_handoff_count"`
	SupersededHandoffCount       int  `json:"superseded_handoff_count"`
	DismissedHandoffCount        int  `json:"dismissed_handoff_count"`
	ReviewFollowUpCount          int  `json:"review_follow_up_count"`
	BlockedReviewCount           int  `json:"blocked_review_count"`
	ChangesRequestedReviewCount  int  `json:"changes_requested_review_count"`
	ActiveAssignmentCount        int  `json:"active_assignment_count"`
	BlockedAssignmentCount       int  `json:"blocked_assignment_count"`
	MissingProfileReferenceCount int  `json:"missing_profile_reference_count"`
	ProjectSkillIssueCount       int  `json:"project_skill_issue_count"`
}

type ProjectHealthAttentionItem struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Kind              string    `json:"kind"`
	Severity          string    `json:"severity"`
	Status            string    `json:"status,omitempty"`
	Title             string    `json:"title"`
	Detail            string    `json:"detail,omitempty"`
	ActionKind        string    `json:"action_kind,omitempty"`
	ActionLabel       string    `json:"action_label,omitempty"`
	WorkItemID        string    `json:"work_item_id,omitempty"`
	AssignmentID      string    `json:"assignment_id,omitempty"`
	ArtifactID        string    `json:"artifact_id,omitempty"`
	HandoffID         string    `json:"handoff_id,omitempty"`
	MemoryCandidateID string    `json:"memory_candidate_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at,omitempty"`
}

type AssistantProposal struct {
	ID                   string            `json:"id"`
	ProjectID            string            `json:"project_id,omitempty"`
	Title                string            `json:"title"`
	Summary              string            `json:"summary,omitempty"`
	Warnings             []string          `json:"warnings,omitempty"`
	Source               string            `json:"source,omitempty"`
	RequiresConfirmation bool              `json:"requires_confirmation"`
	Actions              []AssistantAction `json:"actions"`
	CreatedAt            time.Time         `json:"created_at"`
}

type AssistantProposalRecord struct {
	ID            string                  `json:"id"`
	ProjectID     string                  `json:"project_id,omitempty"`
	Source        string                  `json:"source,omitempty"`
	SourceID      string                  `json:"source_id,omitempty"`
	Proposal      AssistantProposal       `json:"proposal"`
	Status        string                  `json:"status"`
	LatestResult  *AssistantApplyResult   `json:"latest_result,omitempty"`
	ApplyAttempts []AssistantApplyAttempt `json:"apply_attempts,omitempty"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	AppliedAt     *time.Time              `json:"applied_at,omitempty"`
}

type AssistantApplyAttempt struct {
	ID           string               `json:"id"`
	ProposalID   string               `json:"proposal_id"`
	Status       string               `json:"status"`
	Confirmed    bool                 `json:"confirmed"`
	Result       AssistantApplyResult `json:"result"`
	ErrorMessage string               `json:"error_message,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
}

type AssistantAction struct {
	Kind            string           `json:"kind"`
	Title           string           `json:"title,omitempty"`
	Summary         string           `json:"summary,omitempty"`
	Target          AssistantTarget  `json:"target,omitempty"`
	Project         *Project         `json:"project,omitempty"`
	Root            *Root            `json:"root,omitempty"`
	Role            *Role            `json:"role,omitempty"`
	WorkItem        *WorkItem        `json:"work_item,omitempty"`
	Assignment      *Assignment      `json:"assignment,omitempty"`
	Evidence        *Evidence        `json:"evidence,omitempty"`
	Review          *Review          `json:"review,omitempty"`
	Handoff         *Handoff         `json:"handoff,omitempty"`
	MemoryCandidate *MemoryCandidate `json:"memory_candidate,omitempty"`
}

type AssistantTarget struct {
	ProjectID    string `json:"project_id,omitempty"`
	RootID       string `json:"root_id,omitempty"`
	RoleID       string `json:"role_id,omitempty"`
	WorkItemID   string `json:"work_item_id,omitempty"`
	AssignmentID string `json:"assignment_id,omitempty"`
	ArtifactID   string `json:"artifact_id,omitempty"`
	HandoffID    string `json:"handoff_id,omitempty"`
}

type AssistantApplyResult struct {
	ProposalID         string                  `json:"proposal_id"`
	Status             string                  `json:"status"`
	Applied            bool                    `json:"applied"`
	Confirmed          bool                    `json:"confirmed"`
	TotalActionCount   int                     `json:"total_action_count"`
	AppliedActionCount int                     `json:"applied_action_count"`
	FailedActionIndex  *int                    `json:"failed_action_index,omitempty"`
	Actions            []AssistantActionResult `json:"actions,omitempty"`
}

type AssistantActionResult struct {
	Kind              string `json:"kind"`
	Status            string `json:"status"`
	ProjectID         string `json:"project_id,omitempty"`
	RootID            string `json:"root_id,omitempty"`
	RoleID            string `json:"role_id,omitempty"`
	WorkItemID        string `json:"work_item_id,omitempty"`
	AssignmentID      string `json:"assignment_id,omitempty"`
	ArtifactID        string `json:"artifact_id,omitempty"`
	HandoffID         string `json:"handoff_id,omitempty"`
	MemoryCandidateID string `json:"memory_candidate_id,omitempty"`
	Error             string `json:"error,omitempty"`
}

type ProjectOperationsBrief struct {
	ProjectID string                  `json:"project_id"`
	Status    string                  `json:"status"`
	Title     string                  `json:"title"`
	Detail    string                  `json:"detail,omitempty"`
	Counts    ProjectOperationsCounts `json:"counts"`
	Next      *ProjectOperationItem   `json:"next,omitempty"`
	Items     []ProjectOperationItem  `json:"items,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
}

type ProjectOperationsCounts struct {
	WorkItems               int `json:"work_items"`
	OpenWorkItems           int `json:"open_work_items"`
	Assignments             int `json:"assignments"`
	ActiveAssignments       int `json:"active_assignments"`
	BlockedAssignments      int `json:"blocked_assignments"`
	PendingMemoryCandidates int `json:"pending_memory_candidates"`
	ReviewFollowUps         int `json:"review_follow_ups"`
	MissingEvidence         int `json:"missing_evidence"`
	OpenHandoffs            int `json:"open_handoffs"`
	CloseoutReady           int `json:"closeout_ready"`
}

type ProjectOperationItem struct {
	Kind              string    `json:"kind"`
	Severity          string    `json:"severity"`
	Status            string    `json:"status,omitempty"`
	Title             string    `json:"title"`
	Detail            string    `json:"detail,omitempty"`
	WorkItemID        string    `json:"work_item_id,omitempty"`
	AssignmentID      string    `json:"assignment_id,omitempty"`
	ArtifactID        string    `json:"artifact_id,omitempty"`
	MemoryCandidateID string    `json:"memory_candidate_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at,omitempty"`
}

type ProjectActivity struct {
	ProjectID string                 `json:"project_id"`
	Counts    ProjectActivityCounts  `json:"counts"`
	Buckets   ProjectActivityBuckets `json:"buckets"`
	Items     []ProjectActivityItem  `json:"items,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type ProjectActivityCounts struct {
	Assignments    int `json:"assignments"`
	Queued         int `json:"queued"`
	Claimed        int `json:"claimed"`
	Running        int `json:"running"`
	AwaitingReview int `json:"awaiting_review"`
	Completed      int `json:"completed"`
	Failed         int `json:"failed"`
	Cancelled      int `json:"cancelled"`
	Other          int `json:"other"`
	Active         int `json:"active"`
	Blocked        int `json:"blocked"`
}

type ProjectActivityBuckets struct {
	Active    []ProjectActivityItem `json:"active,omitempty"`
	Blocked   []ProjectActivityItem `json:"blocked,omitempty"`
	Completed []ProjectActivityItem `json:"completed,omitempty"`
	Other     []ProjectActivityItem `json:"other,omitempty"`
	Recent    []ProjectActivityItem `json:"recent,omitempty"`
}

type ProjectActivityItem struct {
	Bucket           string    `json:"bucket"`
	AssignmentID     string    `json:"assignment_id"`
	WorkItemID       string    `json:"work_item_id"`
	WorkItemTitle    string    `json:"work_item_title,omitempty"`
	RoleID           string    `json:"role_id,omitempty"`
	RoleName         string    `json:"role_name,omitempty"`
	RootID           string    `json:"root_id,omitempty"`
	Status           string    `json:"status"`
	ExecutionMode    string    `json:"execution_mode,omitempty"`
	DesiredAgentKind string    `json:"desired_agent_kind,omitempty"`
	ExecutionRef     string    `json:"execution_ref,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ReviewFollowUpReadiness struct {
	ArtifactID           string `json:"artifact_id"`
	Title                string `json:"title"`
	Status               string `json:"status"`
	Blocker              string `json:"blocker,omitempty"`
	ReviewedAssignmentID string `json:"reviewed_assignment_id,omitempty"`
	ReviewVerdict        string `json:"review_verdict,omitempty"`
	ReviewRisk           string `json:"review_risk,omitempty"`
}

type DesiredAgent struct {
	Kind     string   `json:"kind,omitempty"`
	SkillIDs []string `json:"skill_ids,omitempty"`
}

type Assignment struct {
	ID                 string       `json:"id"`
	ProjectID          string       `json:"project_id"`
	WorkItemID         string       `json:"work_item_id"`
	RoleID             string       `json:"role_id"`
	RootID             string       `json:"root_id,omitempty"`
	ProfileID          string       `json:"profile_id,omitempty"`
	ExecutionProfileID string       `json:"execution_profile_id,omitempty"`
	ExecutionMode      string       `json:"execution_mode"`
	Status             string       `json:"status"`
	DesiredAgent       DesiredAgent `json:"desired_agent,omitempty"`
	ClaimedBy          string       `json:"claimed_by,omitempty"`
	ExecutionRef       string       `json:"execution_ref,omitempty"`
	ContextSnapshotID  string       `json:"context_snapshot_id,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type AssignmentCompatibilityFilter struct {
	Status         string
	ExecutionModes []string
	AgentKind      string
	SkillIDs       []string
	FilterSkills   bool
	Limit          int
}

type AssignmentContext struct {
	ID         string     `json:"id"`
	Project    Project    `json:"project"`
	WorkItem   WorkItem   `json:"work_item"`
	Role       *Role      `json:"role,omitempty"`
	Assignment Assignment `json:"assignment"`
	Warnings   []string   `json:"warnings,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type AssignmentLaunchPacket struct {
	ID               string            `json:"id"`
	Kind             string            `json:"kind"`
	Project          Project           `json:"project"`
	WorkItem         WorkItem          `json:"work_item"`
	Role             *Role             `json:"role,omitempty"`
	Profile          *AgentProfile     `json:"profile,omitempty"`
	ExecutionProfile *ExecutionProfile `json:"execution_profile,omitempty"`
	Skills           []ProjectSkill    `json:"skills,omitempty"`
	Assignment       Assignment        `json:"assignment"`
	Artifacts        []Artifact        `json:"artifacts,omitempty"`
	Evidence         []Evidence        `json:"evidence,omitempty"`
	Reviews          []Review          `json:"reviews,omitempty"`
	Handoffs         []Handoff         `json:"handoffs,omitempty"`
	Memory           []MemoryEntry     `json:"memory,omitempty"`
	MemoryCandidates []MemoryCandidate `json:"memory_candidates,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

type Artifact struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	WorkItemID     string    `json:"work_item_id"`
	AssignmentID   string    `json:"assignment_id,omitempty"`
	Kind           string    `json:"kind"`
	Title          string    `json:"title,omitempty"`
	Body           string    `json:"body"`
	AuthorRoleID   string    `json:"author_role_id,omitempty"`
	ProvenanceKind string    `json:"provenance_kind,omitempty"`
	TrustLabel     string    `json:"trust_label,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Evidence struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	WorkItemID   string    `json:"work_item_id"`
	AssignmentID string    `json:"assignment_id,omitempty"`
	Title        string    `json:"title"`
	Body         string    `json:"body,omitempty"`
	Locator      string    `json:"locator,omitempty"`
	TrustLabel   string    `json:"trust_label,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Review struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	WorkItemID     string    `json:"work_item_id"`
	AssignmentID   string    `json:"assignment_id,omitempty"`
	ReviewerRoleID string    `json:"reviewer_role_id,omitempty"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	Verdict        string    `json:"verdict"`
	Risk           string    `json:"risk,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Handoff struct {
	ID                    string    `json:"id"`
	ProjectID             string    `json:"project_id"`
	WorkItemID            string    `json:"work_item_id"`
	SourceAssignmentID    string    `json:"source_assignment_id,omitempty"`
	SourceRunID           string    `json:"source_run_id,omitempty"`
	SourceChatSessionID   string    `json:"source_chat_session_id,omitempty"`
	SourceMessageID       string    `json:"source_message_id,omitempty"`
	FromRoleID            string    `json:"from_role_id,omitempty"`
	ToRoleID              string    `json:"to_role_id,omitempty"`
	TargetAssignmentID    string    `json:"target_assignment_id,omitempty"`
	TargetWorkItemID      string    `json:"target_work_item_id,omitempty"`
	Title                 string    `json:"title"`
	Body                  string    `json:"body"`
	RecommendedNextAction string    `json:"recommended_next_action,omitempty"`
	LinkedArtifactIDs     []string  `json:"linked_artifact_ids,omitempty"`
	LinkedMemoryIDs       []string  `json:"linked_memory_ids,omitempty"`
	ContextRefs           []string  `json:"context_refs,omitempty"`
	Status                string    `json:"status"`
	ProvenanceKind        string    `json:"provenance_kind,omitempty"`
	TrustLabel            string    `json:"trust_label,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type MemoryEntry struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	TrustLabel string    `json:"trust_label,omitempty"`
	SourceKind string    `json:"source_kind,omitempty"`
	SourceID   string    `json:"source_id,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MemoryCandidate struct {
	ID                  string                     `json:"id"`
	ProjectID           string                     `json:"project_id"`
	Title               string                     `json:"title"`
	Body                string                     `json:"body"`
	SuggestedKind       string                     `json:"suggested_kind,omitempty"`
	SuggestedTrustLabel string                     `json:"suggested_trust_label,omitempty"`
	SuggestedSourceKind string                     `json:"suggested_source_kind,omitempty"`
	SuggestedSourceID   string                     `json:"suggested_source_id,omitempty"`
	SourceRefs          []MemoryCandidateSourceRef `json:"source_refs,omitempty"`
	Status              string                     `json:"status"`
	StatusReason        string                     `json:"status_reason,omitempty"`
	PromotedMemoryID    string                     `json:"promoted_memory_id,omitempty"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
}

type MemoryCandidateSourceRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

type MemoryCandidateFilter struct {
	ProjectID       string
	Status          string
	IncludeResolved bool
}

type MemoryCandidatePromotion struct {
	ProjectID   string
	CandidateID string
	Title       *string
	Body        *string
	TrustLabel  *string
	SourceKind  *string
	SourceID    *string
	Enabled     *bool
}

const (
	WorkStatusReady = "ready"
	WorkStatusDone  = "done"
	PriorityNormal  = "normal"

	ExecutionManual          = "manual"
	ExecutionMCPPull         = "mcp_pull"
	ExecutionExternalAdapter = "external_adapter"
	ExecutionOrchestrated    = "orchestrated"

	AssignmentQueued    = "queued"
	AssignmentClaimed   = "claimed"
	AssignmentRunning   = "running"
	AssignmentReview    = "awaiting_review"
	AssignmentCompleted = "completed"
	AssignmentFailed    = "failed"
	AssignmentCancelled = "cancelled"

	DesiredAgentAny = "any"

	SkillFormatMarkdown  = "skill_md"
	SkillStatusAvailable = "available"
	SkillStatusMissing   = "missing"
	SkillStatusInvalid   = "invalid"
	SkillStatusConflict  = "conflict"
	SkillTrustWorkspace  = "workspace_skill"
	SkillPathAgents      = ".agents/skills"
	SkillPathHecate      = ".hecate/skills"
	SkillPathCairnline   = ".cairnline/skills"

	EvidenceTrustOperator = "operator_provided"

	ReviewVerdictPass     = "pass"
	ReviewVerdictConcerns = "concerns"
	ReviewVerdictBlocked  = "blocked"
	ReviewRiskLow         = "low"
	ReviewRiskMedium      = "medium"
	ReviewRiskHigh        = "high"
	ReviewStatusRecorded  = "recorded"

	MemoryTrustOperator   = "operator_memory"
	MemoryTrustGenerated  = "generated_summary"
	MemorySourceOperator  = "operator"
	MemorySourceGenerated = "generated"

	HandoffStatusOpen       = "open"
	HandoffStatusAccepted   = "accepted"
	HandoffStatusSuperseded = "superseded"
	HandoffStatusDismissed  = "dismissed"

	MemoryCandidatePending  = "pending"
	MemoryCandidatePromoted = "promoted"
	MemoryCandidateRejected = "rejected"

	MemoryCandidateProposed = MemoryCandidatePending

	ProjectOperationsStatusClear     = "clear"
	ProjectOperationsStatusAttention = "attention"

	ProjectSetupStatusReady    = "ready"
	ProjectSetupStatusTodo     = "todo"
	ProjectSetupStatusOptional = "optional"

	ProjectSetupActionSetupProject            = "setup_project"
	ProjectSetupActionCreateWorkItem          = "create_work_item"
	ProjectSetupActionUpdateProject           = "update_project"
	ProjectSetupActionManageContext           = "manage_context"
	ProjectSetupActionManageExecutionProfiles = "manage_execution_profiles"
	ProjectSetupActionManageRoles             = "manage_roles"

	ProjectHealthStatusClear     = "clear"
	ProjectHealthStatusAttention = "attention"

	AssistantProposalSourceAPI       = "api"
	AssistantProposalSourceAssistant = "assistant"

	AssistantProposalStatusProposed     = "proposed"
	AssistantProposalStatusNeedsConfirm = "needs_confirmation"
	AssistantProposalStatusApplied      = "applied"
	AssistantProposalStatusPartial      = "partial"
	AssistantProposalStatusRejected     = "rejected"

	AssistantActionCreateProject         = "create_project"
	AssistantActionUpdateProject         = "update_project"
	AssistantActionAttachProjectRoot     = "attach_project_root"
	AssistantActionRemoveProjectRoot     = "remove_project_root"
	AssistantActionSetProjectDefaults    = "set_project_defaults"
	AssistantActionCreateRole            = "create_role"
	AssistantActionUpdateRole            = "update_role"
	AssistantActionCreateWorkItem        = "create_work_item"
	AssistantActionUpdateWorkItem        = "update_work_item"
	AssistantActionCreateAssignment      = "create_assignment"
	AssistantActionCreateEvidence        = "create_evidence"
	AssistantActionCreateReview          = "create_review"
	AssistantActionCreateHandoff         = "create_handoff"
	AssistantActionUpdateHandoff         = "update_handoff"
	AssistantActionCreateMemoryCandidate = "create_memory_candidate"

	AssistantApplyStatusApplied      = "applied"
	AssistantApplyStatusNeedsConfirm = "needs_confirmation"
	AssistantApplyStatusPartial      = "partial"
	AssistantApplyStatusRejected     = "rejected"

	ProjectOperationKindAssignment      = "assignment"
	ProjectOperationKindCloseoutReady   = "closeout_ready"
	ProjectOperationKindHandoff         = "handoff"
	ProjectOperationKindMemoryCandidate = "memory_candidate"
	ProjectOperationKindMissingEvidence = "missing_evidence"
	ProjectOperationKindReviewFollowUp  = "review_follow_up"
	ProjectOperationKindProjectSetup    = "project_setup"
	ProjectOperationKindProfile         = "profile"
	ProjectOperationKindSkill           = "skill"
	ProjectOperationKindWorkItem        = "work_item"

	ProjectOperationSeverityBlocked = "blocked"
	ProjectOperationSeverityAction  = "action"
	ProjectOperationSeverityActive  = "active"
	ProjectOperationSeverityReady   = "ready"
	ProjectOperationSeverityInfo    = "info"

	ProjectActivityBucketActive    = "active"
	ProjectActivityBucketBlocked   = "blocked"
	ProjectActivityBucketCompleted = "completed"
	ProjectActivityBucketOther     = "other"

	LaunchPacketKindAssignment = "assignment_launch_packet"
)
