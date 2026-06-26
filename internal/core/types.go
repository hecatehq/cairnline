package core

import "time"

type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Roots          []Root    `json:"roots,omitempty"`
	ContextSources []Source  `json:"context_sources,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Locator    string `json:"locator,omitempty"`
	Enabled    bool   `json:"enabled"`
	TrustLabel string `json:"trust_label,omitempty"`
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
	ID                   string   `json:"id"`
	ProjectID            string   `json:"project_id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	Instructions         string   `json:"instructions,omitempty"`
	DefaultProfileID     string   `json:"default_profile_id,omitempty"`
	DefaultSkillIDs      []string `json:"default_skill_ids,omitempty"`
	DefaultExecutionMode string   `json:"default_execution_mode,omitempty"`
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
	Evidence         []Evidence        `json:"evidence,omitempty"`
	Reviews          []Review          `json:"reviews,omitempty"`
	Handoffs         []Handoff         `json:"handoffs,omitempty"`
	MemoryCandidates []MemoryCandidate `json:"memory_candidates,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

type Evidence struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	WorkItemID string    `json:"work_item_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body,omitempty"`
	Locator    string    `json:"locator,omitempty"`
	TrustLabel string    `json:"trust_label,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	WorkItemID string    `json:"work_item_id"`
	FromRoleID string    `json:"from_role_id,omitempty"`
	ToRoleID   string    `json:"to_role_id,omitempty"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MemoryCandidate struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	TrustLabel string    `json:"trust_label,omitempty"`
	SourceRef  string    `json:"source_ref,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

const (
	WorkStatusReady = "ready"
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
	SkillPathCairnline   = ".cairnline/skills"

	EvidenceTrustOperator = "operator_provided"

	ReviewVerdictPass     = "pass"
	ReviewVerdictConcerns = "concerns"
	ReviewVerdictBlocked  = "blocked"
	ReviewRiskLow         = "low"
	ReviewRiskMedium      = "medium"
	ReviewRiskHigh        = "high"
	ReviewStatusRecorded  = "recorded"

	HandoffStatusPending    = "pending"
	HandoffStatusAccepted   = "accepted"
	HandoffStatusSuperseded = "superseded"
	HandoffStatusDismissed  = "dismissed"
	HandoffStatusOpen       = HandoffStatusPending

	MemoryCandidateProposed = "proposed"

	LaunchPacketKindAssignment = "assignment_launch_packet"
)
