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
	DeleteRole(ctx context.Context, projectID, id string) error

	ListAssignments(ctx context.Context, projectID string) ([]Assignment, error)
	GetAssignment(ctx context.Context, projectID, id string) (Assignment, error)
	CreateAssignment(ctx context.Context, assignment Assignment) (Assignment, error)
	RestoreAssignmentSnapshot(ctx context.Context, assignment Assignment) (Assignment, error)
	UpdateQueuedAssignment(ctx context.Context, projectID, id string, update QueuedAssignmentUpdate, now func() time.Time) (Assignment, error)
	ClaimAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (Assignment, error)
	PrepareAssignment(ctx context.Context, projectID, id string, preparation AssignmentPreparation, now func() time.Time) (Assignment, error)
	ReleaseAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (Assignment, error)
	UpdateAssignmentStatus(ctx context.Context, projectID, id, status string, executionRef ExecutionRef, now func() time.Time) (Assignment, error)
	CompleteAssignment(ctx context.Context, projectID, id, status string, executionRef ExecutionRef, now func() time.Time) (Assignment, error)
	DeleteAssignment(ctx context.Context, projectID, id string) error

	ListArtifacts(ctx context.Context, projectID, workItemID string) ([]Artifact, error)
	GetArtifact(ctx context.Context, projectID, workItemID, id string) (Artifact, error)
	CreateArtifact(ctx context.Context, artifact Artifact) (Artifact, error)

	ListEvidence(ctx context.Context, projectID, workItemID string) ([]Evidence, error)
	GetEvidence(ctx context.Context, projectID, workItemID, id string) (Evidence, error)
	CreateEvidence(ctx context.Context, evidence Evidence) (Evidence, error)

	ListReviews(ctx context.Context, projectID, workItemID string) ([]Review, error)
	GetReview(ctx context.Context, projectID, workItemID, id string) (Review, error)
	CreateReview(ctx context.Context, review Review) (Review, error)

	ListHandoffs(ctx context.Context, projectID, workItemID string) ([]Handoff, error)
	GetHandoff(ctx context.Context, projectID, workItemID, id string) (Handoff, error)
	CreateHandoff(ctx context.Context, handoff Handoff) (Handoff, error)
	RestoreHandoffSnapshot(ctx context.Context, handoff Handoff) (Handoff, error)
	UpdateHandoff(ctx context.Context, projectID, workItemID, id string, update HandoffUpdate, now func() time.Time) (Handoff, error)
	DeleteHandoff(ctx context.Context, projectID, workItemID, id string, deletion HandoffDelete) error
	AcceptHandoffWithFollowUp(ctx context.Context, command AcceptHandoffWithFollowUpCommand, newAssignmentID string, now func() time.Time) (HandoffFollowUpResult, error)

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

	ListAssistantProposals(ctx context.Context, projectID string) ([]AssistantProposalRecord, error)
	GetAssistantProposal(ctx context.Context, id string) (AssistantProposalRecord, error)
	CreateAssistantProposal(ctx context.Context, record AssistantProposalRecord) (AssistantProposalRecord, error)
	UpdateAssistantProposal(ctx context.Context, record AssistantProposalRecord) (AssistantProposalRecord, error)
}
