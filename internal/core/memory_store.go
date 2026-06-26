package core

import (
	"context"
	"slices"
	"sync"
	"time"
)

type MemoryStore struct {
	mu          sync.Mutex
	projects    map[string]Project
	agent       map[string]AgentProfile
	execution   map[string]ExecutionProfile
	skills      map[string]map[string]ProjectSkill
	workItems   map[string]map[string]WorkItem
	roles       map[string]map[string]Role
	assignments map[string]map[string]Assignment
	evidence    map[string]map[string]Evidence
	reviews     map[string]map[string]Review
	handoffs    map[string]map[string]Handoff
	memory      map[string]map[string]MemoryCandidate
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		projects:    make(map[string]Project),
		agent:       make(map[string]AgentProfile),
		execution:   make(map[string]ExecutionProfile),
		skills:      make(map[string]map[string]ProjectSkill),
		workItems:   make(map[string]map[string]WorkItem),
		roles:       make(map[string]map[string]Role),
		assignments: make(map[string]map[string]Assignment),
		evidence:    make(map[string]map[string]Evidence),
		reviews:     make(map[string]map[string]Review),
		handoffs:    make(map[string]map[string]Handoff),
		memory:      make(map[string]map[string]MemoryCandidate),
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

func (s *MemoryStore) ListAgentProfiles(ctx context.Context) ([]AgentProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]AgentProfile, 0, len(s.agent))
	for _, item := range s.agent {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b AgentProfile) int {
		return compareString(a.Name, b.Name)
	})
	return items, nil
}

func (s *MemoryStore) GetAgentProfile(ctx context.Context, id string) (AgentProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.agent[id]
	if !ok {
		return AgentProfile{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateAgentProfile(ctx context.Context, profile AgentProfile) (AgentProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agent[profile.ID]; ok {
		return AgentProfile{}, ErrDuplicate
	}
	s.agent[profile.ID] = profile
	return profile, nil
}

func (s *MemoryStore) UpdateAgentProfile(ctx context.Context, profile AgentProfile) (AgentProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agent[profile.ID]; !ok {
		return AgentProfile{}, ErrNotFound
	}
	s.agent[profile.ID] = profile
	return profile, nil
}

func (s *MemoryStore) ListExecutionProfiles(ctx context.Context) ([]ExecutionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]ExecutionProfile, 0, len(s.execution))
	for _, item := range s.execution {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b ExecutionProfile) int {
		return compareString(a.Name, b.Name)
	})
	return items, nil
}

func (s *MemoryStore) GetExecutionProfile(ctx context.Context, id string) (ExecutionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.execution[id]
	if !ok {
		return ExecutionProfile{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) CreateExecutionProfile(ctx context.Context, profile ExecutionProfile) (ExecutionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.execution[profile.ID]; ok {
		return ExecutionProfile{}, ErrDuplicate
	}
	s.execution[profile.ID] = profile
	return profile, nil
}

func (s *MemoryStore) UpdateExecutionProfile(ctx context.Context, profile ExecutionProfile) (ExecutionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.execution[profile.ID]; !ok {
		return ExecutionProfile{}, ErrNotFound
	}
	s.execution[profile.ID] = profile
	return profile, nil
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
		items = append(items, item)
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
	return item, nil
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
	s.skills[skill.ProjectID][skill.ID] = skill
	return skill, nil
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
	s.skills[skill.ProjectID][skill.ID] = skill
	return skill, nil
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

func (s *MemoryStore) ListAssignments(ctx context.Context, projectID string) ([]Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	assignmentsByID := s.assignments[projectID]
	items := make([]Assignment, 0, len(assignmentsByID))
	for _, item := range assignmentsByID {
		items = append(items, item)
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
	return item, nil
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
	s.assignments[assignment.ProjectID][assignment.ID] = assignment
	return assignment, nil
}

func (s *MemoryStore) UpdateAssignment(ctx context.Context, assignment Assignment) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[assignment.ProjectID]; !ok {
		return Assignment{}, ErrNotFound
	}
	if _, ok := s.assignments[assignment.ProjectID][assignment.ID]; !ok {
		return Assignment{}, ErrNotFound
	}
	s.assignments[assignment.ProjectID][assignment.ID] = assignment
	return assignment, nil
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
	stamp := time.Now().UTC()
	if now != nil {
		stamp = now()
	}
	item.Status = AssignmentClaimed
	item.ClaimedBy = claimedBy
	item.UpdatedAt = stamp
	s.assignments[projectID][id] = item
	return item, nil
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

func (s *MemoryStore) CreateEvidence(ctx context.Context, evidence Evidence) (Evidence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(evidence.ProjectID, evidence.WorkItemID); err != nil {
		return Evidence{}, err
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
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b Handoff) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) CreateHandoff(ctx context.Context, handoff Handoff) (Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireWorkItemLocked(handoff.ProjectID, handoff.WorkItemID); err != nil {
		return Handoff{}, err
	}
	if handoff.FromRoleID != "" {
		if _, ok := s.roles[handoff.ProjectID][handoff.FromRoleID]; !ok {
			return Handoff{}, ErrNotFound
		}
	}
	if handoff.ToRoleID != "" {
		if _, ok := s.roles[handoff.ProjectID][handoff.ToRoleID]; !ok {
			return Handoff{}, ErrNotFound
		}
	}
	if s.handoffs[handoff.ProjectID] == nil {
		s.handoffs[handoff.ProjectID] = make(map[string]Handoff)
	}
	if _, ok := s.handoffs[handoff.ProjectID][handoff.ID]; ok {
		return Handoff{}, ErrDuplicate
	}
	s.handoffs[handoff.ProjectID][handoff.ID] = handoff
	return handoff, nil
}

func (s *MemoryStore) ListMemoryCandidates(ctx context.Context, projectID string) ([]MemoryCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	items := make([]MemoryCandidate, 0, len(s.memory[projectID]))
	for _, item := range s.memory[projectID] {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b MemoryCandidate) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return items, nil
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

func (s *MemoryStore) requireWorkItemLocked(projectID, workItemID string) error {
	if _, ok := s.projects[projectID]; !ok {
		return ErrNotFound
	}
	if _, ok := s.workItems[projectID][workItemID]; !ok {
		return ErrNotFound
	}
	return nil
}
