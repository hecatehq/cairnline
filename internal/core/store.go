package core

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrInvalid   = errors.New("invalid")
	ErrDuplicate = errors.New("duplicate")
	ErrConflict  = errors.New("conflict")
)

type Store interface {
	ListProjects(ctx context.Context) ([]Project, error)
	GetProject(ctx context.Context, id string) (Project, error)
	CreateProject(ctx context.Context, project Project) (Project, error)
	UpdateProject(ctx context.Context, project Project) (Project, error)
	DeleteProject(ctx context.Context, id string) error

	ListAgentProfiles(ctx context.Context) ([]AgentProfile, error)
	GetAgentProfile(ctx context.Context, id string) (AgentProfile, error)
	CreateAgentProfile(ctx context.Context, profile AgentProfile) (AgentProfile, error)
	UpdateAgentProfile(ctx context.Context, profile AgentProfile) (AgentProfile, error)

	ListExecutionProfiles(ctx context.Context) ([]ExecutionProfile, error)
	GetExecutionProfile(ctx context.Context, id string) (ExecutionProfile, error)
	CreateExecutionProfile(ctx context.Context, profile ExecutionProfile) (ExecutionProfile, error)
	UpdateExecutionProfile(ctx context.Context, profile ExecutionProfile) (ExecutionProfile, error)

	ListProjectSkills(ctx context.Context, projectID string) ([]ProjectSkill, error)
	GetProjectSkill(ctx context.Context, projectID, id string) (ProjectSkill, error)
	CreateProjectSkill(ctx context.Context, skill ProjectSkill) (ProjectSkill, error)
	UpdateProjectSkill(ctx context.Context, skill ProjectSkill) (ProjectSkill, error)

	ListWorkItems(ctx context.Context, projectID string) ([]WorkItem, error)
	GetWorkItem(ctx context.Context, projectID, id string) (WorkItem, error)
	CreateWorkItem(ctx context.Context, item WorkItem) (WorkItem, error)
	UpdateWorkItem(ctx context.Context, item WorkItem) (WorkItem, error)
	DeleteWorkItem(ctx context.Context, projectID, id string) error

	ListRoles(ctx context.Context, projectID string) ([]Role, error)
	GetRole(ctx context.Context, projectID, id string) (Role, error)
	CreateRole(ctx context.Context, role Role) (Role, error)
	UpdateRole(ctx context.Context, role Role) (Role, error)

	ListAssignments(ctx context.Context, projectID string) ([]Assignment, error)
	GetAssignment(ctx context.Context, projectID, id string) (Assignment, error)
	CreateAssignment(ctx context.Context, assignment Assignment) (Assignment, error)
	UpdateAssignment(ctx context.Context, assignment Assignment) (Assignment, error)
	ClaimAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (Assignment, error)
	DeleteAssignment(ctx context.Context, projectID, id string) error

	ListEvidence(ctx context.Context, projectID, workItemID string) ([]Evidence, error)
	CreateEvidence(ctx context.Context, evidence Evidence) (Evidence, error)

	ListReviews(ctx context.Context, projectID, workItemID string) ([]Review, error)
	CreateReview(ctx context.Context, review Review) (Review, error)

	ListHandoffs(ctx context.Context, projectID, workItemID string) ([]Handoff, error)
	GetHandoff(ctx context.Context, projectID, workItemID, id string) (Handoff, error)
	CreateHandoff(ctx context.Context, handoff Handoff) (Handoff, error)
	UpdateHandoff(ctx context.Context, handoff Handoff) (Handoff, error)
	DeleteHandoff(ctx context.Context, projectID, workItemID, id string) error

	ListMemoryEntries(ctx context.Context, projectID string, includeDisabled bool) ([]MemoryEntry, error)
	GetMemoryEntry(ctx context.Context, projectID, id string) (MemoryEntry, error)
	CreateMemoryEntry(ctx context.Context, entry MemoryEntry) (MemoryEntry, error)
	UpdateMemoryEntry(ctx context.Context, entry MemoryEntry) (MemoryEntry, error)
	DeleteMemoryEntry(ctx context.Context, projectID, id string) error

	ListMemoryCandidates(ctx context.Context, filter MemoryCandidateFilter) ([]MemoryCandidate, error)
	GetMemoryCandidate(ctx context.Context, projectID, id string) (MemoryCandidate, error)
	CreateMemoryCandidate(ctx context.Context, candidate MemoryCandidate) (MemoryCandidate, error)
	UpdateMemoryCandidate(ctx context.Context, candidate MemoryCandidate) (MemoryCandidate, error)
	DeleteMemoryCandidate(ctx context.Context, projectID, id string) error
	PromoteMemoryCandidate(ctx context.Context, projectID, id string, entry MemoryEntry) (MemoryCandidate, MemoryEntry, error)
}
