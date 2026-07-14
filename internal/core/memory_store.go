package core

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

type MemoryStore struct {
	mu          sync.Mutex
	projects    map[string]Project
	skills      map[string]map[string]ProjectSkill
	workItems   map[string]map[string]WorkItem
	roles       map[string]map[string]Role
	assignments map[string]map[string]Assignment
	artifacts   map[string]map[string]Artifact
	evidence    map[string]map[string]Evidence
	reviews     map[string]map[string]Review
	handoffs    map[string]map[string]Handoff
	entries     map[string]map[string]MemoryEntry
	memory      map[string]map[string]MemoryCandidate
	assistant   map[string]AssistantProposalRecord
	receipts    map[handoffCommandReceiptKey]handoffCommandReceipt
}

type handoffCommandReceiptKey struct {
	projectID      string
	operation      string
	idempotencyKey string
}

type handoffCommandReceipt struct {
	requestHash string
	result      HandoffFollowUpResult
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		projects:    make(map[string]Project),
		skills:      make(map[string]map[string]ProjectSkill),
		workItems:   make(map[string]map[string]WorkItem),
		roles:       make(map[string]map[string]Role),
		assignments: make(map[string]map[string]Assignment),
		artifacts:   make(map[string]map[string]Artifact),
		evidence:    make(map[string]map[string]Evidence),
		reviews:     make(map[string]map[string]Review),
		handoffs:    make(map[string]map[string]Handoff),
		entries:     make(map[string]map[string]MemoryEntry),
		memory:      make(map[string]map[string]MemoryCandidate),
		assistant:   make(map[string]AssistantProposalRecord),
		receipts:    make(map[handoffCommandReceiptKey]handoffCommandReceipt),
	}
}

func (s *MemoryStore) ListProjects(ctx context.Context) ([]Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]Project, 0, len(s.projects))
	for _, item := range s.projects {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b Project) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetProject(ctx context.Context, id string) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.projects[id]
	if !ok {
		return Project{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateProject(ctx context.Context, project Project) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[project.ID]; ok {
		return Project{}, ErrDuplicate
	}
	s.projects[project.ID] = project
	return project, nil
}

func (s *MemoryStore) UpdateProject(ctx context.Context, project Project) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[project.ID]; !ok {
		return Project{}, ErrNotFound
	}
	s.projects[project.ID] = project
	return project, nil
}

func (s *MemoryStore) DeleteProject(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[id]; !ok {
		return ErrNotFound
	}
	delete(s.projects, id)
	delete(s.skills, id)
	delete(s.workItems, id)
	delete(s.roles, id)
	delete(s.assignments, id)
	delete(s.artifacts, id)
	delete(s.evidence, id)
	delete(s.reviews, id)
	delete(s.handoffs, id)
	for key, receipt := range s.receipts {
		if receipt.result.Handoff.ProjectID == id {
			delete(s.receipts, key)
		}
	}
	delete(s.entries, id)
	delete(s.memory, id)
	for proposalID, proposal := range s.assistant {
		if proposal.ProjectID == id {
			delete(s.assistant, proposalID)
		}
	}
	return nil
}

func (s *MemoryStore) ListProjectSkills(ctx context.Context, projectID string) ([]ProjectSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	skillsByID := s.skills[projectID]
	items := make([]ProjectSkill, 0, len(skillsByID))
	for _, item := range skillsByID {
		items = append(items, cloneProjectSkill(item))
	}
	slices.SortFunc(items, func(a, b ProjectSkill) int {
		return compareString(a.ID, b.ID)
	})
	return items, nil
}

func (s *MemoryStore) GetProjectSkill(ctx context.Context, projectID, id string) (ProjectSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return ProjectSkill{}, ErrNotFound
	}
	item, ok := s.skills[projectID][id]
	if !ok {
		return ProjectSkill{}, ErrNotFound
	}
	return cloneProjectSkill(item), nil
}

func (s *MemoryStore) CreateProjectSkill(ctx context.Context, skill ProjectSkill) (ProjectSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[skill.ProjectID]; !ok {
		return ProjectSkill{}, ErrNotFound
	}
	if s.skills[skill.ProjectID] == nil {
		s.skills[skill.ProjectID] = make(map[string]ProjectSkill)
	}
	if _, ok := s.skills[skill.ProjectID][skill.ID]; ok {
		return ProjectSkill{}, ErrDuplicate
	}
	s.skills[skill.ProjectID][skill.ID] = cloneProjectSkill(skill)
	return cloneProjectSkill(skill), nil
}

func (s *MemoryStore) UpdateProjectSkill(ctx context.Context, skill ProjectSkill) (ProjectSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[skill.ProjectID]; !ok {
		return ProjectSkill{}, ErrNotFound
	}
	if _, ok := s.skills[skill.ProjectID][skill.ID]; !ok {
		return ProjectSkill{}, ErrNotFound
	}
	s.skills[skill.ProjectID][skill.ID] = cloneProjectSkill(skill)
	return cloneProjectSkill(skill), nil
}

func (s *MemoryStore) ListWorkItems(ctx context.Context, projectID string) ([]WorkItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	itemsByID := s.workItems[projectID]
	items := make([]WorkItem, 0, len(itemsByID))
	for _, item := range itemsByID {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b WorkItem) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetWorkItem(ctx context.Context, projectID, id string) (WorkItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return WorkItem{}, ErrNotFound
	}
	item, ok := s.workItems[projectID][id]
	if !ok {
		return WorkItem{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateWorkItem(ctx context.Context, item WorkItem) (WorkItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[item.ProjectID]; !ok {
		return WorkItem{}, ErrNotFound
	}
	if s.workItems[item.ProjectID] == nil {
		s.workItems[item.ProjectID] = make(map[string]WorkItem)
	}
	if _, ok := s.workItems[item.ProjectID][item.ID]; ok {
		return WorkItem{}, ErrDuplicate
	}
	s.workItems[item.ProjectID][item.ID] = item
	return item, nil
}

func (s *MemoryStore) UpdateWorkItem(ctx context.Context, item WorkItem) (WorkItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[item.ProjectID]; !ok {
		return WorkItem{}, ErrNotFound
	}
	if _, ok := s.workItems[item.ProjectID][item.ID]; !ok {
		return WorkItem{}, ErrNotFound
	}
	s.workItems[item.ProjectID][item.ID] = item
	return item, nil
}

func (s *MemoryStore) DeleteWorkItem(ctx context.Context, projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.workItems[projectID][id]; !ok {
		return ErrNotFound
	}
	delete(s.workItems[projectID], id)
	for assignmentID, item := range s.assignments[projectID] {
		if item.WorkItemID == id {
			delete(s.assignments[projectID], assignmentID)
		}
	}
	for evidenceID, item := range s.evidence[projectID] {
		if item.WorkItemID == id {
			delete(s.evidence[projectID], evidenceID)
		}
	}
	for artifactID, item := range s.artifacts[projectID] {
		if item.WorkItemID == id {
			delete(s.artifacts[projectID], artifactID)
		}
	}
	for reviewID, item := range s.reviews[projectID] {
		if item.WorkItemID == id {
			delete(s.reviews[projectID], reviewID)
		}
	}
	for handoffID, item := range s.handoffs[projectID] {
		if item.WorkItemID == id || item.TargetWorkItemID == id {
			delete(s.handoffs[projectID], handoffID)
		}
	}
	return nil
}

func (s *MemoryStore) ListRoles(ctx context.Context, projectID string) ([]Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	rolesByID := s.roles[projectID]
	items := make([]Role, 0, len(rolesByID))
	for _, item := range rolesByID {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b Role) int {
		return compareString(a.Name, b.Name)
	})
	return items, nil
}

func (s *MemoryStore) GetRole(ctx context.Context, projectID, id string) (Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Role{}, ErrNotFound
	}
	item, ok := s.roles[projectID][id]
	if !ok {
		return Role{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateRole(ctx context.Context, role Role) (Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[role.ProjectID]; !ok {
		return Role{}, ErrNotFound
	}
	if s.roles[role.ProjectID] == nil {
		s.roles[role.ProjectID] = make(map[string]Role)
	}
	if _, ok := s.roles[role.ProjectID][role.ID]; ok {
		return Role{}, ErrDuplicate
	}
	s.roles[role.ProjectID][role.ID] = role
	return role, nil
}

func (s *MemoryStore) UpdateRole(ctx context.Context, role Role) (Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[role.ProjectID]; !ok {
		return Role{}, ErrNotFound
	}
	if _, ok := s.roles[role.ProjectID][role.ID]; !ok {
		return Role{}, ErrNotFound
	}
	s.roles[role.ProjectID][role.ID] = role
	return role, nil
}

func (s *MemoryStore) DeleteRole(ctx context.Context, projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.roles[projectID][id]; !ok {
		return ErrNotFound
	}
	delete(s.roles[projectID], id)
	return nil
}

func (s *MemoryStore) ListAssignments(ctx context.Context, projectID string) ([]Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	assignmentsByID := s.assignments[projectID]
	items := make([]Assignment, 0, len(assignmentsByID))
	for _, item := range assignmentsByID {
		items = append(items, cloneAssignment(item))
	}
	slices.SortFunc(items, func(a, b Assignment) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetAssignment(ctx context.Context, projectID, id string) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	item, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	return cloneAssignment(item), nil
}

func (s *MemoryStore) CreateAssignment(ctx context.Context, assignment Assignment) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[assignment.ProjectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.workItems[assignment.ProjectID][assignment.WorkItemID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if assignment.RoleID != "" {
		if _, ok := s.roles[assignment.ProjectID][assignment.RoleID]; !ok {
			return Assignment{}, ErrNotFound
		}
	}
	if s.assignments[assignment.ProjectID] == nil {
		s.assignments[assignment.ProjectID] = make(map[string]Assignment)
	}
	if _, ok := s.assignments[assignment.ProjectID][assignment.ID]; ok {
		return Assignment{}, ErrDuplicate
	}
	stored := cloneAssignment(assignment)
	s.assignments[assignment.ProjectID][assignment.ID] = stored
	return cloneAssignment(stored), nil
}

// RestoreAssignmentSnapshot atomically replaces an assignment during offline
// snapshot import. It is intentionally not exposed through Service or MCP.
func (s *MemoryStore) RestoreAssignmentSnapshot(ctx context.Context, assignment Assignment) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[assignment.ProjectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.workItems[assignment.ProjectID][assignment.WorkItemID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.roles[assignment.ProjectID][assignment.RoleID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.assignments[assignment.ProjectID][assignment.ID]; !ok {
		return Assignment{}, ErrNotFound
	}
	stored := cloneAssignment(assignment)
	s.assignments[assignment.ProjectID][assignment.ID] = stored
	return cloneAssignment(stored), nil
}

func (s *MemoryStore) UpdateQueuedAssignment(ctx context.Context, projectID, id string, update QueuedAssignmentUpdate, now func() time.Time) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.workItems[projectID][update.Replacement.WorkItemID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.roles[projectID][update.Replacement.RoleID]; !ok {
		return Assignment{}, ErrNotFound
	}
	current, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	if !queuedAssignmentSnapshotMatches(current, update) {
		return Assignment{}, ErrConflict
	}
	current.WorkItemID = update.Replacement.WorkItemID
	current.RoleID = update.Replacement.RoleID
	current.RootID = update.Replacement.RootID
	current.ExecutionMode = update.Replacement.ExecutionMode
	current.DesiredAgent = cloneDesiredAgent(update.Replacement.DesiredAgent)
	current.UpdatedAt = assignmentTransitionTime(current, now)
	s.assignments[projectID][id] = cloneAssignment(current)
	return cloneAssignment(current), nil
}

func queuedAssignmentSnapshotMatches(current Assignment, update QueuedAssignmentUpdate) bool {
	return current.WorkItemID == update.Expected.WorkItemID &&
		current.RoleID == update.Expected.RoleID &&
		current.RootID == update.Expected.RootID &&
		current.ExecutionMode == update.Expected.ExecutionMode &&
		current.Status == AssignmentQueued &&
		current.ClaimedBy == "" &&
		current.DesiredAgent.Kind == update.Expected.DesiredAgent.Kind &&
		slices.Equal(current.DesiredAgent.SkillIDs, update.Expected.DesiredAgent.SkillIDs) &&
		current.ExecutionRef.Empty() &&
		current.ContextSnapshotID == "" &&
		current.UpdatedAt.Equal(update.ExpectedUpdatedAt) &&
		current.StartedAt.IsZero() &&
		current.CompletedAt.IsZero()
}

func cloneDesiredAgent(agent DesiredAgent) DesiredAgent {
	agent.SkillIDs = append([]string(nil), agent.SkillIDs...)
	return agent
}

func cloneAssignment(assignment Assignment) Assignment {
	assignment.DesiredAgent = cloneDesiredAgent(assignment.DesiredAgent)
	return assignment
}

func assignmentTransitionTime(current Assignment, now func() time.Time) time.Time {
	stamp := time.Now().UTC()
	if now != nil {
		stamp = now()
	}
	if stamp.Before(current.UpdatedAt) {
		stamp = current.UpdatedAt
	}
	if stamp.Before(current.StartedAt) {
		stamp = current.StartedAt
	}
	if !stamp.After(current.UpdatedAt) {
		stamp = current.UpdatedAt.Add(time.Nanosecond)
	}
	return stamp
}

func (s *MemoryStore) ClaimAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	item, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	if item.Status != AssignmentQueued {
		return Assignment{}, ErrConflict
	}
	stamp := assignmentTransitionTime(item, now)
	item.Status = AssignmentClaimed
	item.ClaimedBy = claimedBy
	item.UpdatedAt = stamp
	s.assignments[projectID][id] = cloneAssignment(item)
	return cloneAssignment(item), nil
}

func (s *MemoryStore) PrepareAssignment(ctx context.Context, projectID, id string, preparation AssignmentPreparation, now func() time.Time) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	item, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	if item.Status != AssignmentClaimed || item.ClaimedBy != preparation.ClaimedBy {
		return Assignment{}, ErrConflict
	}
	if !preparation.ExecutionRef.Empty() {
		item.ExecutionRef = preparation.ExecutionRef
	}
	if preparation.ContextSnapshotID != "" {
		item.ContextSnapshotID = preparation.ContextSnapshotID
	}
	item.UpdatedAt = assignmentTransitionTime(item, now)
	s.assignments[projectID][id] = cloneAssignment(item)
	return cloneAssignment(item), nil
}

func (s *MemoryStore) ReleaseAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	item, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	if item.Status != AssignmentClaimed || item.ClaimedBy != claimedBy {
		return Assignment{}, ErrConflict
	}
	stamp := assignmentTransitionTime(item, now)
	item.Status = AssignmentQueued
	item.ClaimedBy = ""
	item.ExecutionRef = ExecutionRef{}
	item.ContextSnapshotID = ""
	item.StartedAt = time.Time{}
	item.CompletedAt = time.Time{}
	item.UpdatedAt = stamp
	s.assignments[projectID][id] = cloneAssignment(item)
	return cloneAssignment(item), nil
}

func (s *MemoryStore) CompleteAssignment(ctx context.Context, projectID, id, status string, executionRef ExecutionRef, now func() time.Time) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	item, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	if isTerminalAssignmentStatus(item.Status) {
		return Assignment{}, ErrConflict
	}
	stamp := assignmentTransitionTime(item, now)
	previousStatus := item.Status
	item.Status = status
	if item.StartedAt.IsZero() && !(previousStatus == AssignmentQueued && status == AssignmentCancelled) {
		item.StartedAt = stamp
	}
	if isTerminalAssignmentStatus(status) && item.CompletedAt.IsZero() {
		item.CompletedAt = stamp
	}
	if !executionRef.Empty() {
		item.ExecutionRef = executionRef
	}
	item.UpdatedAt = stamp
	s.assignments[projectID][id] = cloneAssignment(item)
	return cloneAssignment(item), nil
}

func (s *MemoryStore) UpdateAssignmentStatus(ctx context.Context, projectID, id, status string, executionRef ExecutionRef, now func() time.Time) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	item, ok := s.assignments[projectID][id]
	if !ok {
		return Assignment{}, ErrNotFound
	}
	if item.Status == AssignmentQueued || isTerminalAssignmentStatus(item.Status) {
		return Assignment{}, ErrConflict
	}
	stamp := assignmentTransitionTime(item, now)
	item.Status = status
	if item.StartedAt.IsZero() {
		item.StartedAt = stamp
	}
	if !executionRef.Empty() {
		item.ExecutionRef = executionRef
	}
	item.UpdatedAt = stamp
	s.assignments[projectID][id] = cloneAssignment(item)
	return cloneAssignment(item), nil
}

func (s *MemoryStore) DeleteAssignment(ctx context.Context, projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.assignments[projectID][id]; !ok {
		return ErrNotFound
	}
	delete(s.assignments[projectID], id)
	for artifactID, artifact := range s.artifacts[projectID] {
		if artifact.AssignmentID == id {
			delete(s.artifacts[projectID], artifactID)
		}
	}
	for reviewID, review := range s.reviews[projectID] {
		if review.AssignmentID == id {
			delete(s.reviews[projectID], reviewID)
		}
	}
	return nil
}

func compareString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (s *MemoryStore) ListArtifacts(ctx context.Context, projectID, workItemID string) ([]Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return nil, err
	}
	items := make([]Artifact, 0, len(s.artifacts[projectID]))
	for _, item := range s.artifacts[projectID] {
		if item.WorkItemID == workItemID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b Artifact) int {
		if cmp := a.CreatedAt.Compare(b.CreatedAt); cmp != 0 {
			return cmp
		}
		return compareString(a.ID, b.ID)
	})
	return items, nil
}

func (s *MemoryStore) GetArtifact(ctx context.Context, projectID, workItemID, id string) (Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return Artifact{}, err
	}
	item, ok := s.artifacts[projectID][id]
	if !ok || item.WorkItemID != workItemID {
		return Artifact{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateArtifact(ctx context.Context, artifact Artifact) (Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(artifact.ProjectID, artifact.WorkItemID); err != nil {
		return Artifact{}, err
	}
	if artifact.AssignmentID != "" {
		assignment, ok := s.assignments[artifact.ProjectID][artifact.AssignmentID]
		if !ok || assignment.WorkItemID != artifact.WorkItemID {
			return Artifact{}, ErrNotFound
		}
	}
	if artifact.AuthorRoleID != "" {
		if _, ok := s.roles[artifact.ProjectID][artifact.AuthorRoleID]; !ok {
			return Artifact{}, ErrNotFound
		}
	}
	if s.artifacts[artifact.ProjectID] == nil {
		s.artifacts[artifact.ProjectID] = make(map[string]Artifact)
	}
	if _, ok := s.artifacts[artifact.ProjectID][artifact.ID]; ok {
		return Artifact{}, ErrDuplicate
	}
	s.artifacts[artifact.ProjectID][artifact.ID] = artifact
	return artifact, nil
}

func (s *MemoryStore) ListEvidence(ctx context.Context, projectID, workItemID string) ([]Evidence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return nil, err
	}
	items := make([]Evidence, 0, len(s.evidence[projectID]))
	for _, item := range s.evidence[projectID] {
		if item.WorkItemID == workItemID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b Evidence) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetEvidence(ctx context.Context, projectID, workItemID, id string) (Evidence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return Evidence{}, err
	}
	item, ok := s.evidence[projectID][id]
	if !ok || item.WorkItemID != workItemID {
		return Evidence{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateEvidence(ctx context.Context, evidence Evidence) (Evidence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(evidence.ProjectID, evidence.WorkItemID); err != nil {
		return Evidence{}, err
	}
	if evidence.AssignmentID != "" {
		assignment, ok := s.assignments[evidence.ProjectID][evidence.AssignmentID]
		if !ok || assignment.WorkItemID != evidence.WorkItemID {
			return Evidence{}, ErrNotFound
		}
	}
	if s.evidence[evidence.ProjectID] == nil {
		s.evidence[evidence.ProjectID] = make(map[string]Evidence)
	}
	if _, ok := s.evidence[evidence.ProjectID][evidence.ID]; ok {
		return Evidence{}, ErrDuplicate
	}
	s.evidence[evidence.ProjectID][evidence.ID] = evidence
	return evidence, nil
}

func (s *MemoryStore) ListReviews(ctx context.Context, projectID, workItemID string) ([]Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return nil, err
	}
	items := make([]Review, 0, len(s.reviews[projectID]))
	for _, item := range s.reviews[projectID] {
		if item.WorkItemID == workItemID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b Review) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetReview(ctx context.Context, projectID, workItemID, id string) (Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return Review{}, err
	}
	item, ok := s.reviews[projectID][id]
	if !ok || item.WorkItemID != workItemID {
		return Review{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateReview(ctx context.Context, review Review) (Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(review.ProjectID, review.WorkItemID); err != nil {
		return Review{}, err
	}
	if review.AssignmentID != "" {
		assignment, ok := s.assignments[review.ProjectID][review.AssignmentID]
		if !ok || assignment.WorkItemID != review.WorkItemID {
			return Review{}, ErrNotFound
		}
	}
	if review.ReviewerRoleID != "" {
		if _, ok := s.roles[review.ProjectID][review.ReviewerRoleID]; !ok {
			return Review{}, ErrNotFound
		}
	}
	if s.reviews[review.ProjectID] == nil {
		s.reviews[review.ProjectID] = make(map[string]Review)
	}
	if _, ok := s.reviews[review.ProjectID][review.ID]; ok {
		return Review{}, ErrDuplicate
	}
	s.reviews[review.ProjectID][review.ID] = review
	return review, nil
}

func (s *MemoryStore) ListHandoffs(ctx context.Context, projectID, workItemID string) ([]Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return nil, err
	}
	items := make([]Handoff, 0, len(s.handoffs[projectID]))
	for _, item := range s.handoffs[projectID] {
		if item.WorkItemID == workItemID {
			items = append(items, cloneHandoff(item))
		}
	}
	slices.SortFunc(items, func(a, b Handoff) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetHandoff(ctx context.Context, projectID, workItemID, id string) (Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return Handoff{}, err
	}
	item, ok := s.handoffs[projectID][id]
	if !ok || item.WorkItemID != workItemID {
		return Handoff{}, ErrNotFound
	}
	return cloneHandoff(item), nil
}

func (s *MemoryStore) CreateHandoff(ctx context.Context, handoff Handoff) (Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateHandoffLocked(handoff); err != nil {
		return Handoff{}, err
	}
	if s.handoffs[handoff.ProjectID] == nil {
		s.handoffs[handoff.ProjectID] = make(map[string]Handoff)
	}
	if _, ok := s.handoffs[handoff.ProjectID][handoff.ID]; ok {
		return Handoff{}, ErrDuplicate
	}
	s.handoffs[handoff.ProjectID][handoff.ID] = cloneHandoff(handoff)
	return cloneHandoff(handoff), nil
}

// RestoreHandoffSnapshot atomically replaces a handoff during offline snapshot
// import. Live callers must use the CAS transition methods below.
func (s *MemoryStore) RestoreHandoffSnapshot(ctx context.Context, handoff Handoff) (Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateHandoffLocked(handoff); err != nil {
		return Handoff{}, err
	}
	existing, ok := s.handoffs[handoff.ProjectID][handoff.ID]
	if !ok || existing.WorkItemID != handoff.WorkItemID {
		return Handoff{}, ErrNotFound
	}
	s.handoffs[handoff.ProjectID][handoff.ID] = cloneHandoff(handoff)
	return cloneHandoff(handoff), nil
}

func (s *MemoryStore) UpdateHandoff(ctx context.Context, projectID, workItemID, id string, update HandoffUpdate, now func() time.Time) (Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return Handoff{}, err
	}
	current, ok := s.handoffs[projectID][id]
	if !ok || current.WorkItemID != workItemID {
		return Handoff{}, ErrNotFound
	}
	if !current.UpdatedAt.Equal(update.ExpectedUpdatedAt) {
		return Handoff{}, ErrConflict
	}
	replacement := update.Patch.Apply(cloneHandoff(current))
	if current.SameContent(replacement) {
		return cloneHandoff(current), nil
	}
	if err := s.validateHandoffLocked(replacement); err != nil {
		return Handoff{}, err
	}
	replacement.CreatedAt = current.CreatedAt
	replacement.UpdatedAt = handoffTransitionTime(current, now)
	replacement.StatusChangedAt = current.StatusChangedAt
	if replacement.StatusChangedAt.IsZero() {
		replacement.StatusChangedAt = current.CreatedAt
	}
	if replacement.Status != current.Status {
		replacement.StatusChangedAt = replacement.UpdatedAt
	}
	s.handoffs[projectID][id] = cloneHandoff(replacement)
	return cloneHandoff(replacement), nil
}

func (s *MemoryStore) DeleteHandoff(ctx context.Context, projectID, workItemID, id string, deletion HandoffDelete) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(projectID, workItemID); err != nil {
		return err
	}
	item, ok := s.handoffs[projectID][id]
	if !ok || item.WorkItemID != workItemID {
		return ErrNotFound
	}
	if !item.UpdatedAt.Equal(deletion.ExpectedUpdatedAt) {
		return ErrConflict
	}
	delete(s.handoffs[projectID], id)
	return nil
}

func (s *MemoryStore) AcceptHandoffWithFollowUp(ctx context.Context, command AcceptHandoffWithFollowUpCommand, newAssignmentID string, now func() time.Time) (HandoffFollowUpResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receiptKey := handoffCommandReceiptKey{
		projectID:      command.ProjectID,
		operation:      "accept_handoff_with_follow_up",
		idempotencyKey: command.IdempotencyKey,
	}
	requestHash := command.RequestHash()
	if receipt, ok := s.receipts[receiptKey]; ok {
		if receipt.requestHash != requestHash {
			return HandoffFollowUpResult{}, ErrConflict
		}
		result := cloneHandoffFollowUpResult(receipt.result)
		currentHandoff, handoffOK := s.handoffs[command.ProjectID][result.Handoff.ID]
		currentAssignment, assignmentOK := s.assignments[command.ProjectID][result.Assignment.ID]
		if !handoffOK || !assignmentOK ||
			currentHandoff.Status != HandoffStatusAccepted ||
			currentHandoff.TargetAssignmentID != currentAssignment.ID ||
			currentHandoff.TargetWorkItemID != currentAssignment.WorkItemID ||
			currentHandoff.ToRoleID != currentAssignment.RoleID {
			return HandoffFollowUpResult{}, ErrConflict
		}
		result.Handoff = cloneHandoff(currentHandoff)
		result.Assignment = cloneAssignment(currentAssignment)
		result.Replayed = true
		return result, nil
	}
	current, ok := s.handoffs[command.ProjectID][command.HandoffID]
	if !ok || current.WorkItemID != command.WorkItemID {
		return HandoffFollowUpResult{}, ErrNotFound
	}
	if !current.UpdatedAt.Equal(command.ExpectedUpdatedAt) {
		return HandoffFollowUpResult{}, ErrConflict
	}
	if current.Status == HandoffStatusDismissed || current.Status == HandoffStatusSuperseded {
		return HandoffFollowUpResult{}, ErrConflict
	}
	if current.ToRoleID == "" {
		return HandoffFollowUpResult{}, errors.Join(ErrInvalid, errors.New("handoff to_role_id is required"))
	}
	role, ok := s.roles[command.ProjectID][current.ToRoleID]
	if !ok {
		return HandoffFollowUpResult{}, ErrNotFound
	}

	assignment := Assignment{}
	outcome := HandoffFollowUpCreated
	if current.TargetAssignmentID != "" {
		linked, ok := s.assignments[command.ProjectID][current.TargetAssignmentID]
		if !ok {
			return HandoffFollowUpResult{}, ErrConflict
		}
		if linked.RoleID != current.ToRoleID || (current.TargetWorkItemID != "" && linked.WorkItemID != current.TargetWorkItemID) {
			return HandoffFollowUpResult{}, ErrConflict
		}
		if _, ok := s.workItems[command.ProjectID][linked.WorkItemID]; !ok {
			return HandoffFollowUpResult{}, ErrConflict
		}
		assignment = cloneAssignment(linked)
		if current.Status == HandoffStatusAccepted && current.TargetWorkItemID == linked.WorkItemID {
			outcome = HandoffFollowUpAlreadySatisfied
		} else {
			outcome = HandoffFollowUpLinkedExisting
		}
		current.TargetWorkItemID = linked.WorkItemID
	} else {
		targetWorkItemID := current.TargetWorkItemID
		if targetWorkItemID == "" {
			targetWorkItemID = current.WorkItemID
		}
		workItem, ok := s.workItems[command.ProjectID][targetWorkItemID]
		if !ok {
			return HandoffFollowUpResult{}, ErrNotFound
		}
		if workItem.RootID != "" {
			project, ok := s.projects[command.ProjectID]
			if !ok || !projectHasRoot(project, workItem.RootID) {
				return HandoffFollowUpResult{}, ErrNotFound
			}
		}
		executionMode := role.DefaultExecutionMode
		if executionMode == "" {
			executionMode = ExecutionMCPPull
		}
		desiredKind := DesiredAgentAny
		if executionMode == ExecutionManual {
			desiredKind = "human"
		}
		stamp := time.Now().UTC()
		if now != nil {
			stamp = now()
		}
		assignment = Assignment{
			ID:            newAssignmentID,
			ProjectID:     command.ProjectID,
			WorkItemID:    targetWorkItemID,
			RoleID:        current.ToRoleID,
			RootID:        workItem.RootID,
			ExecutionMode: executionMode,
			Status:        AssignmentQueued,
			DesiredAgent: DesiredAgent{
				Kind:     desiredKind,
				SkillIDs: append([]string(nil), role.DefaultSkillIDs...),
			},
			CreatedAt: stamp,
			UpdatedAt: stamp,
		}
		if s.assignments[command.ProjectID] == nil {
			s.assignments[command.ProjectID] = make(map[string]Assignment)
		}
		if _, exists := s.assignments[command.ProjectID][assignment.ID]; exists {
			return HandoffFollowUpResult{}, ErrDuplicate
		}
		s.assignments[command.ProjectID][assignment.ID] = cloneAssignment(assignment)
		current.TargetAssignmentID = assignment.ID
		current.TargetWorkItemID = targetWorkItemID
	}

	if outcome != HandoffFollowUpAlreadySatisfied {
		previousStatus := current.Status
		current.Status = HandoffStatusAccepted
		current.UpdatedAt = handoffTransitionTime(current, now)
		if previousStatus != current.Status {
			current.StatusChangedAt = current.UpdatedAt
		}
		s.handoffs[command.ProjectID][command.HandoffID] = cloneHandoff(current)
	}
	result := HandoffFollowUpResult{
		Handoff:    cloneHandoff(current),
		Assignment: cloneAssignment(assignment),
		Outcome:    outcome,
	}
	s.receipts[receiptKey] = handoffCommandReceipt{requestHash: requestHash, result: cloneHandoffFollowUpResult(result)}
	return result, nil
}

func cloneHandoff(handoff Handoff) Handoff {
	handoff.LinkedArtifactIDs = append([]string(nil), handoff.LinkedArtifactIDs...)
	handoff.LinkedMemoryIDs = append([]string(nil), handoff.LinkedMemoryIDs...)
	handoff.ContextRefs = append([]string(nil), handoff.ContextRefs...)
	return handoff
}

func cloneHandoffFollowUpResult(result HandoffFollowUpResult) HandoffFollowUpResult {
	result.Handoff = cloneHandoff(result.Handoff)
	result.Assignment = cloneAssignment(result.Assignment)
	return result
}

func handoffTransitionTime(current Handoff, now func() time.Time) time.Time {
	stamp := time.Now().UTC()
	if now != nil {
		stamp = now()
	}
	for _, floor := range []time.Time{current.CreatedAt, current.UpdatedAt, current.StatusChangedAt} {
		if stamp.Before(floor) {
			stamp = floor
		}
	}
	if !stamp.After(current.UpdatedAt) {
		stamp = current.UpdatedAt.Add(time.Nanosecond)
	}
	return stamp
}

func projectHasRoot(project Project, rootID string) bool {
	for _, root := range project.Roots {
		if root.ID == rootID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) validateHandoffLocked(handoff Handoff) error {
	if err := s.requireWorkItemLocked(handoff.ProjectID, handoff.WorkItemID); err != nil {
		return err
	}
	if handoff.FromRoleID != "" {
		if _, ok := s.roles[handoff.ProjectID][handoff.FromRoleID]; !ok {
			return ErrNotFound
		}
	}
	if handoff.ToRoleID != "" {
		if _, ok := s.roles[handoff.ProjectID][handoff.ToRoleID]; !ok {
			return ErrNotFound
		}
	}
	if handoff.SourceAssignmentID != "" {
		if assignment, ok := s.assignments[handoff.ProjectID][handoff.SourceAssignmentID]; !ok || assignment.WorkItemID != handoff.WorkItemID {
			return ErrNotFound
		}
	}
	if handoff.TargetAssignmentID != "" {
		assignment, ok := s.assignments[handoff.ProjectID][handoff.TargetAssignmentID]
		if !ok {
			return ErrNotFound
		}
		if handoff.TargetWorkItemID != "" && assignment.WorkItemID != handoff.TargetWorkItemID {
			return ErrNotFound
		}
	}
	if handoff.TargetWorkItemID != "" {
		if _, ok := s.workItems[handoff.ProjectID][handoff.TargetWorkItemID]; !ok {
			return ErrNotFound
		}
	}
	return nil
}

func (s *MemoryStore) ListMemoryEntries(ctx context.Context, projectID string, includeDisabled bool) ([]MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	items := make([]MemoryEntry, 0, len(s.entries[projectID]))
	for _, item := range s.entries[projectID] {
		if !includeDisabled && !item.Enabled {
			continue
		}
		items = append(items, item)
	}
	slices.SortFunc(items, compareMemoryEntries)
	return items, nil
}

func (s *MemoryStore) GetMemoryEntry(ctx context.Context, projectID, id string) (MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return MemoryEntry{}, ErrNotFound
	}
	item, ok := s.entries[projectID][id]
	if !ok {
		return MemoryEntry{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateMemoryEntry(ctx context.Context, entry MemoryEntry) (MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[entry.ProjectID]; !ok {
		return MemoryEntry{}, ErrNotFound
	}
	if s.entries[entry.ProjectID] == nil {
		s.entries[entry.ProjectID] = make(map[string]MemoryEntry)
	}
	if _, ok := s.entries[entry.ProjectID][entry.ID]; ok {
		return MemoryEntry{}, ErrDuplicate
	}
	s.entries[entry.ProjectID][entry.ID] = entry
	return entry, nil
}

func (s *MemoryStore) UpdateMemoryEntry(ctx context.Context, entry MemoryEntry) (MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[entry.ProjectID]; !ok {
		return MemoryEntry{}, ErrNotFound
	}
	existing, ok := s.entries[entry.ProjectID][entry.ID]
	if !ok {
		return MemoryEntry{}, ErrNotFound
	}
	entry.CreatedAt = existing.CreatedAt
	s.entries[entry.ProjectID][entry.ID] = entry
	return entry, nil
}

func (s *MemoryStore) DeleteMemoryEntry(ctx context.Context, projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.entries[projectID][id]; !ok {
		return ErrNotFound
	}
	delete(s.entries[projectID], id)
	return nil
}

func (s *MemoryStore) ListMemoryCandidates(ctx context.Context, filter MemoryCandidateFilter) ([]MemoryCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	projectID := filter.ProjectID
	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	items := make([]MemoryCandidate, 0, len(s.memory[projectID]))
	for _, item := range s.memory[projectID] {
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.Status == "" && !filter.IncludeResolved && item.Status != MemoryCandidatePending {
			continue
		}
		items = append(items, item)
	}
	slices.SortFunc(items, compareMemoryCandidates)
	return items, nil
}

func compareMemoryEntries(a, b MemoryEntry) int {
	if a.Enabled != b.Enabled {
		if a.Enabled {
			return -1
		}
		return 1
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	}
	if a.Title != b.Title {
		return compareStrings(a.Title, b.Title)
	}
	return compareStrings(a.ID, b.ID)
}

func compareMemoryCandidates(a, b MemoryCandidate) int {
	if a.Status != b.Status {
		return memoryCandidateSortRank(a.Status) - memoryCandidateSortRank(b.Status)
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	}
	if a.Title != b.Title {
		return compareStrings(a.Title, b.Title)
	}
	return compareStrings(a.ID, b.ID)
}

func memoryCandidateSortRank(status string) int {
	switch status {
	case MemoryCandidatePending:
		return 0
	case MemoryCandidatePromoted:
		return 1
	case MemoryCandidateRejected:
		return 2
	default:
		return 3
	}
}

func compareStrings(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func (s *MemoryStore) GetMemoryCandidate(ctx context.Context, projectID, id string) (MemoryCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return MemoryCandidate{}, ErrNotFound
	}
	item, ok := s.memory[projectID][id]
	if !ok {
		return MemoryCandidate{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateMemoryCandidate(ctx context.Context, candidate MemoryCandidate) (MemoryCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[candidate.ProjectID]; !ok {
		return MemoryCandidate{}, ErrNotFound
	}
	if s.memory[candidate.ProjectID] == nil {
		s.memory[candidate.ProjectID] = make(map[string]MemoryCandidate)
	}
	if _, ok := s.memory[candidate.ProjectID][candidate.ID]; ok {
		return MemoryCandidate{}, ErrDuplicate
	}
	s.memory[candidate.ProjectID][candidate.ID] = candidate
	return candidate, nil
}

func (s *MemoryStore) UpdateMemoryCandidate(ctx context.Context, candidate MemoryCandidate) (MemoryCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[candidate.ProjectID]; !ok {
		return MemoryCandidate{}, ErrNotFound
	}
	existing, ok := s.memory[candidate.ProjectID][candidate.ID]
	if !ok {
		return MemoryCandidate{}, ErrNotFound
	}
	candidate.CreatedAt = existing.CreatedAt
	s.memory[candidate.ProjectID][candidate.ID] = candidate
	return candidate, nil
}

func (s *MemoryStore) DeleteMemoryCandidate(ctx context.Context, projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.memory[projectID][id]; !ok {
		return ErrNotFound
	}
	delete(s.memory[projectID], id)
	return nil
}

func (s *MemoryStore) PromoteMemoryCandidate(ctx context.Context, projectID, id string, entry MemoryEntry) (MemoryCandidate, MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return MemoryCandidate{}, MemoryEntry{}, ErrNotFound
	}
	candidate, ok := s.memory[projectID][id]
	if !ok {
		return MemoryCandidate{}, MemoryEntry{}, ErrNotFound
	}
	if candidate.Status == MemoryCandidatePromoted && candidate.PromotedMemoryID != "" {
		promoted, ok := s.entries[projectID][candidate.PromotedMemoryID]
		if !ok {
			return MemoryCandidate{}, MemoryEntry{}, ErrNotFound
		}
		return candidate, promoted, nil
	}
	if candidate.Status != MemoryCandidatePending {
		return MemoryCandidate{}, MemoryEntry{}, ErrConflict
	}
	if s.entries[projectID] == nil {
		s.entries[projectID] = make(map[string]MemoryEntry)
	}
	if _, ok := s.entries[projectID][entry.ID]; ok {
		return MemoryCandidate{}, MemoryEntry{}, ErrDuplicate
	}
	s.entries[projectID][entry.ID] = entry
	candidate.Status = MemoryCandidatePromoted
	candidate.StatusReason = ""
	candidate.PromotedMemoryID = entry.ID
	candidate.UpdatedAt = entry.UpdatedAt
	s.memory[projectID][id] = candidate
	return candidate, entry, nil
}

func (s *MemoryStore) ListAssistantProposals(ctx context.Context, projectID string) ([]AssistantProposalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]AssistantProposalRecord, 0)
	for _, item := range s.assistant {
		if projectID != "" && item.ProjectID != projectID {
			continue
		}
		items = append(items, cloneAssistantProposalRecord(item))
	}
	slices.SortFunc(items, func(a, b AssistantProposalRecord) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetAssistantProposal(ctx context.Context, id string) (AssistantProposalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.assistant[id]
	if !ok {
		return AssistantProposalRecord{}, ErrNotFound
	}
	return cloneAssistantProposalRecord(item), nil
}

func (s *MemoryStore) CreateAssistantProposal(ctx context.Context, record AssistantProposalRecord) (AssistantProposalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record = normalizeAssistantProposalRecord(record, record.CreatedAt)
	if err := validateAssistantProposalRecord(record); err != nil {
		return AssistantProposalRecord{}, err
	}
	if _, ok := s.assistant[record.ID]; ok {
		return AssistantProposalRecord{}, ErrDuplicate
	}
	s.assistant[record.ID] = cloneAssistantProposalRecord(record)
	return cloneAssistantProposalRecord(record), nil
}

func (s *MemoryStore) UpdateAssistantProposal(ctx context.Context, record AssistantProposalRecord) (AssistantProposalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.assistant[record.ID]
	if !ok {
		return AssistantProposalRecord{}, ErrNotFound
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = existing.CreatedAt
	}
	record = normalizeAssistantProposalRecord(record, record.UpdatedAt)
	if err := validateAssistantProposalRecord(record); err != nil {
		return AssistantProposalRecord{}, err
	}
	s.assistant[record.ID] = cloneAssistantProposalRecord(record)
	return cloneAssistantProposalRecord(record), nil
}

func (s *MemoryStore) requireWorkItemLocked(projectID, workItemID string) error {
	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.workItems[projectID][workItemID]; !ok {
		return ErrNotFound
	}
	return nil
}

func cloneProjectSkill(item ProjectSkill) ProjectSkill {
	item.SuggestedTools = append([]string(nil), item.SuggestedTools...)
	item.RequiredPermissions = cloneRequiredPermissions(item.RequiredPermissions)
	item.SourceRefs = append([]string(nil), item.SourceRefs...)
	item.Warnings = append([]string(nil), item.Warnings...)
	return item
}
