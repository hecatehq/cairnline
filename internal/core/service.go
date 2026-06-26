package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxSkillMetadataBytes = 64 * 1024

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	return s.store.ListProjects(ctx)
}

func (s *Service) CreateProject(ctx context.Context, input Project) (Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Project{}, errors.Join(ErrInvalid, errors.New("project name is required"))
	}
	now := s.now()
	item := Project{
		ID:             firstNonEmpty(strings.TrimSpace(input.ID), newID("proj")),
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		Roots:          normalizeRoots(input.Roots),
		ContextSources: input.ContextSources,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.store.CreateProject(ctx, item)
}

func (s *Service) UpdateProject(ctx context.Context, input Project) (Project, error) {
	id := strings.TrimSpace(input.ID)
	name := strings.TrimSpace(input.Name)
	if id == "" {
		return Project{}, errors.Join(ErrInvalid, errors.New("project id is required"))
	}
	if name == "" {
		return Project{}, errors.Join(ErrInvalid, errors.New("project name is required"))
	}
	existing, err := s.store.GetProject(ctx, id)
	if err != nil {
		return Project{}, err
	}
	item := Project{
		ID:             id,
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		Roots:          normalizeRoots(input.Roots),
		ContextSources: input.ContextSources,
		CreatedAt:      existing.CreatedAt,
		UpdatedAt:      s.now(),
	}
	return s.store.UpdateProject(ctx, item)
}

func (s *Service) ListProjectSkills(ctx context.Context, projectID string) ([]ProjectSkill, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListProjectSkills(ctx, projectID)
}

func (s *Service) GetProjectSkill(ctx context.Context, projectID, id string) (ProjectSkill, error) {
	projectID = strings.TrimSpace(projectID)
	id = normalizeSkillID(id)
	if projectID == "" {
		return ProjectSkill{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return ProjectSkill{}, errors.Join(ErrInvalid, errors.New("skill id is required"))
	}
	return s.store.GetProjectSkill(ctx, projectID, id)
}

func (s *Service) CreateProjectSkill(ctx context.Context, input ProjectSkill) (ProjectSkill, error) {
	item, err := s.normalizeProjectSkill(input, true)
	if err != nil {
		return ProjectSkill{}, err
	}
	if _, err := s.store.GetProject(ctx, item.ProjectID); err != nil {
		return ProjectSkill{}, err
	}
	return s.store.CreateProjectSkill(ctx, item)
}

func (s *Service) UpdateProjectSkill(ctx context.Context, input ProjectSkill) (ProjectSkill, error) {
	item, err := s.normalizeProjectSkill(input, false)
	if err != nil {
		return ProjectSkill{}, err
	}
	existing, err := s.store.GetProjectSkill(ctx, item.ProjectID, item.ID)
	if err != nil {
		return ProjectSkill{}, err
	}
	item.CreatedAt = existing.CreatedAt
	if item.DiscoveredAt.IsZero() {
		item.DiscoveredAt = existing.DiscoveredAt
	}
	return s.store.UpdateProjectSkill(ctx, item)
}

func (s *Service) DiscoverProjectSkills(ctx context.Context, projectID string) ([]ProjectSkill, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	existingItems, err := s.store.ListProjectSkills(ctx, projectID)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]ProjectSkill, len(existingItems))
	for _, item := range existingItems {
		existing[item.ID] = item
	}
	now := s.now()
	discovered := make(map[string]ProjectSkill)
	for _, root := range project.Roots {
		if !root.Active || strings.TrimSpace(root.Path) == "" {
			continue
		}
		rootPath := filepath.Clean(root.Path)
		for _, base := range []string{SkillPathAgents, SkillPathCairnline} {
			basePath := filepath.Join(rootPath, filepath.FromSlash(base))
			entries, err := os.ReadDir(basePath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				skillPath := filepath.Join(basePath, entry.Name(), "SKILL.md")
				metadata, err := readSkillMetadata(skillPath)
				if err != nil {
					continue
				}
				relPath, err := filepath.Rel(rootPath, skillPath)
				if err != nil {
					relPath = skillPath
				}
				id := normalizeSkillID(entry.Name())
				if id == "" {
					id = entry.Name()
				}
				title := firstNonEmpty(metadata.name, metadata.title, entry.Name())
				item := ProjectSkill{
					ID:           id,
					ProjectID:    projectID,
					Title:        title,
					Description:  metadata.description,
					Path:         filepath.ToSlash(relPath),
					RootID:       root.ID,
					Format:       SkillFormatMarkdown,
					Enabled:      true,
					Status:       SkillStatusAvailable,
					TrustLabel:   SkillTrustWorkspace,
					DiscoveredAt: now,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				if prior, ok := discovered[id]; ok {
					prior.Status = SkillStatusConflict
					prior.Warnings = appendUnique(prior.Warnings, "skill id is declared by multiple paths: "+prior.Path+" and "+item.Path)
					discovered[id] = prior
					item.Status = SkillStatusConflict
					item.Warnings = prior.Warnings
				} else {
					discovered[id] = item
				}
			}
		}
	}
	for id, item := range discovered {
		if prior, ok := existing[id]; ok {
			item.Enabled = prior.Enabled
			item.Title = firstNonEmpty(prior.Title, item.Title)
			item.Description = firstNonEmpty(prior.Description, item.Description)
			item.TrustLabel = firstNonEmpty(prior.TrustLabel, item.TrustLabel)
			item.SourceRefs = prior.SourceRefs
			item.CreatedAt = prior.CreatedAt
			if item.CreatedAt.IsZero() {
				item.CreatedAt = now
			}
			if _, err := s.store.UpdateProjectSkill(ctx, item); err != nil {
				return nil, err
			}
			continue
		}
		if err := s.createDiscoveredProjectSkill(ctx, item); err != nil {
			return nil, err
		}
	}
	for id, prior := range existing {
		if _, ok := discovered[id]; ok {
			continue
		}
		if prior.Status == SkillStatusMissing {
			continue
		}
		prior.Status = SkillStatusMissing
		prior.Warnings = appendUnique(prior.Warnings, "skill was not found during latest discovery")
		prior.UpdatedAt = now
		if _, err := s.store.UpdateProjectSkill(ctx, prior); err != nil {
			return nil, err
		}
	}
	return s.store.ListProjectSkills(ctx, projectID)
}

func (s *Service) ListAgentProfiles(ctx context.Context) ([]AgentProfile, error) {
	return s.store.ListAgentProfiles(ctx)
}

func (s *Service) CreateAgentProfile(ctx context.Context, input AgentProfile) (AgentProfile, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return AgentProfile{}, errors.Join(ErrInvalid, errors.New("profile name is required"))
	}
	now := s.now()
	item := AgentProfile{
		ID:            firstNonEmpty(strings.TrimSpace(input.ID), newID("profile")),
		Name:          name,
		Description:   strings.TrimSpace(input.Description),
		Instructions:  strings.TrimSpace(input.Instructions),
		ContextPolicy: strings.TrimSpace(input.ContextPolicy),
		MemoryPolicy:  strings.TrimSpace(input.MemoryPolicy),
		SourcePolicy:  strings.TrimSpace(input.SourcePolicy),
		SkillIDs:      compactStrings(input.SkillIDs),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.store.CreateAgentProfile(ctx, item)
}

func (s *Service) UpdateAgentProfile(ctx context.Context, input AgentProfile) (AgentProfile, error) {
	id := strings.TrimSpace(input.ID)
	name := strings.TrimSpace(input.Name)
	if id == "" {
		return AgentProfile{}, errors.Join(ErrInvalid, errors.New("profile id is required"))
	}
	if name == "" {
		return AgentProfile{}, errors.Join(ErrInvalid, errors.New("profile name is required"))
	}
	existing, err := s.store.GetAgentProfile(ctx, id)
	if err != nil {
		return AgentProfile{}, err
	}
	item := AgentProfile{
		ID:            id,
		Name:          name,
		Description:   strings.TrimSpace(input.Description),
		Instructions:  strings.TrimSpace(input.Instructions),
		ContextPolicy: strings.TrimSpace(input.ContextPolicy),
		MemoryPolicy:  strings.TrimSpace(input.MemoryPolicy),
		SourcePolicy:  strings.TrimSpace(input.SourcePolicy),
		SkillIDs:      compactStrings(input.SkillIDs),
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     s.now(),
	}
	return s.store.UpdateAgentProfile(ctx, item)
}

func (s *Service) ListExecutionProfiles(ctx context.Context) ([]ExecutionProfile, error) {
	return s.store.ListExecutionProfiles(ctx)
}

func (s *Service) CreateExecutionProfile(ctx context.Context, input ExecutionProfile) (ExecutionProfile, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ExecutionProfile{}, errors.Join(ErrInvalid, errors.New("execution profile name is required"))
	}
	now := s.now()
	item := ExecutionProfile{
		ID:             firstNonEmpty(strings.TrimSpace(input.ID), newID("execprof")),
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		AgentKind:      strings.TrimSpace(input.AgentKind),
		ModelHint:      strings.TrimSpace(input.ModelHint),
		ProviderHint:   strings.TrimSpace(input.ProviderHint),
		ToolsPolicy:    strings.TrimSpace(input.ToolsPolicy),
		WritesPolicy:   strings.TrimSpace(input.WritesPolicy),
		NetworkPolicy:  strings.TrimSpace(input.NetworkPolicy),
		ApprovalPolicy: strings.TrimSpace(input.ApprovalPolicy),
		AdapterOptions: input.AdapterOptions,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.store.CreateExecutionProfile(ctx, item)
}

func (s *Service) UpdateExecutionProfile(ctx context.Context, input ExecutionProfile) (ExecutionProfile, error) {
	id := strings.TrimSpace(input.ID)
	name := strings.TrimSpace(input.Name)
	if id == "" {
		return ExecutionProfile{}, errors.Join(ErrInvalid, errors.New("execution profile id is required"))
	}
	if name == "" {
		return ExecutionProfile{}, errors.Join(ErrInvalid, errors.New("execution profile name is required"))
	}
	existing, err := s.store.GetExecutionProfile(ctx, id)
	if err != nil {
		return ExecutionProfile{}, err
	}
	item := ExecutionProfile{
		ID:             id,
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		AgentKind:      strings.TrimSpace(input.AgentKind),
		ModelHint:      strings.TrimSpace(input.ModelHint),
		ProviderHint:   strings.TrimSpace(input.ProviderHint),
		ToolsPolicy:    strings.TrimSpace(input.ToolsPolicy),
		WritesPolicy:   strings.TrimSpace(input.WritesPolicy),
		NetworkPolicy:  strings.TrimSpace(input.NetworkPolicy),
		ApprovalPolicy: strings.TrimSpace(input.ApprovalPolicy),
		AdapterOptions: input.AdapterOptions,
		CreatedAt:      existing.CreatedAt,
		UpdatedAt:      s.now(),
	}
	return s.store.UpdateExecutionProfile(ctx, item)
}

func (s *Service) ListWorkItems(ctx context.Context, projectID string) ([]WorkItem, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListWorkItems(ctx, projectID)
}

func (s *Service) CreateWorkItem(ctx context.Context, input WorkItem) (WorkItem, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	title := strings.TrimSpace(input.Title)
	if projectID == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if title == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("work item title is required"))
	}
	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = PriorityNormal
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = WorkStatusReady
	}
	now := s.now()
	item := WorkItem{
		ID:              firstNonEmpty(strings.TrimSpace(input.ID), newID("work")),
		ProjectID:       projectID,
		Title:           title,
		Brief:           strings.TrimSpace(input.Brief),
		Status:          status,
		Priority:        priority,
		OwnerRoleID:     strings.TrimSpace(input.OwnerRoleID),
		ReviewerRoleIDs: input.ReviewerRoleIDs,
		RootID:          strings.TrimSpace(input.RootID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return s.store.CreateWorkItem(ctx, item)
}

func (s *Service) UpdateWorkItem(ctx context.Context, input WorkItem) (WorkItem, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.ID)
	title := strings.TrimSpace(input.Title)
	if projectID == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if title == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("work item title is required"))
	}
	existing, err := s.store.GetWorkItem(ctx, projectID, id)
	if err != nil {
		return WorkItem{}, err
	}
	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = PriorityNormal
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = WorkStatusReady
	}
	item := WorkItem{
		ID:              id,
		ProjectID:       projectID,
		Title:           title,
		Brief:           strings.TrimSpace(input.Brief),
		Status:          status,
		Priority:        priority,
		OwnerRoleID:     strings.TrimSpace(input.OwnerRoleID),
		ReviewerRoleIDs: compactStrings(input.ReviewerRoleIDs),
		RootID:          strings.TrimSpace(input.RootID),
		CreatedAt:       existing.CreatedAt,
		UpdatedAt:       s.now(),
	}
	return s.store.UpdateWorkItem(ctx, item)
}

func (s *Service) ListRoles(ctx context.Context, projectID string) ([]Role, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListRoles(ctx, projectID)
}

func (s *Service) CreateRole(ctx context.Context, input Role) (Role, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	name := strings.TrimSpace(input.Name)
	if projectID == "" {
		return Role{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if name == "" {
		return Role{}, errors.Join(ErrInvalid, errors.New("role name is required"))
	}
	defaultProfileID := strings.TrimSpace(input.DefaultProfileID)
	if defaultProfileID != "" {
		if _, err := s.store.GetAgentProfile(ctx, defaultProfileID); err != nil {
			return Role{}, err
		}
	}
	executionMode, err := normalizeExecutionMode(input.DefaultExecutionMode, true)
	if err != nil {
		return Role{}, err
	}
	item := Role{
		ID:                   firstNonEmpty(strings.TrimSpace(input.ID), newID("role")),
		ProjectID:            projectID,
		Name:                 name,
		Description:          strings.TrimSpace(input.Description),
		Instructions:         strings.TrimSpace(input.Instructions),
		DefaultProfileID:     defaultProfileID,
		DefaultSkillIDs:      compactStrings(input.DefaultSkillIDs),
		DefaultExecutionMode: executionMode,
	}
	return s.store.CreateRole(ctx, item)
}

func (s *Service) UpdateRole(ctx context.Context, input Role) (Role, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.ID)
	name := strings.TrimSpace(input.Name)
	if projectID == "" {
		return Role{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return Role{}, errors.Join(ErrInvalid, errors.New("role_id is required"))
	}
	if name == "" {
		return Role{}, errors.Join(ErrInvalid, errors.New("role name is required"))
	}
	if _, err := s.store.GetRole(ctx, projectID, id); err != nil {
		return Role{}, err
	}
	defaultProfileID := strings.TrimSpace(input.DefaultProfileID)
	if defaultProfileID != "" {
		if _, err := s.store.GetAgentProfile(ctx, defaultProfileID); err != nil {
			return Role{}, err
		}
	}
	executionMode, err := normalizeExecutionMode(input.DefaultExecutionMode, true)
	if err != nil {
		return Role{}, err
	}
	item := Role{
		ID:                   id,
		ProjectID:            projectID,
		Name:                 name,
		Description:          strings.TrimSpace(input.Description),
		Instructions:         strings.TrimSpace(input.Instructions),
		DefaultProfileID:     defaultProfileID,
		DefaultSkillIDs:      compactStrings(input.DefaultSkillIDs),
		DefaultExecutionMode: executionMode,
	}
	return s.store.UpdateRole(ctx, item)
}

func (s *Service) ListAssignments(ctx context.Context, projectID string) ([]Assignment, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListAssignments(ctx, projectID)
}

func (s *Service) ListCompatibleAssignments(ctx context.Context, projectID string, filter AssignmentCompatibilityFilter) ([]Assignment, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	status := strings.TrimSpace(filter.Status)
	if status == "" {
		status = AssignmentQueued
	}
	executionModes := compactStrings(filter.ExecutionModes)
	if len(executionModes) == 0 {
		executionModes = []string{ExecutionMCPPull}
	}
	for _, mode := range executionModes {
		if _, err := normalizeExecutionMode(mode, false); err != nil {
			return nil, err
		}
	}
	agentKind := strings.TrimSpace(filter.AgentKind)
	skillSet := make(map[string]struct{}, len(filter.SkillIDs))
	for _, id := range compactStrings(filter.SkillIDs) {
		normalizedID := normalizeSkillID(id)
		if normalizedID == "" {
			continue
		}
		skillSet[normalizedID] = struct{}{}
	}
	items, err := s.store.ListAssignments(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var out []Assignment
	for _, item := range items {
		if item.Status != status {
			continue
		}
		if !containsStringValue(executionModes, item.ExecutionMode) {
			continue
		}
		if !assignmentAgentKindMatches(item.DesiredAgent.Kind, agentKind) {
			continue
		}
		if filter.FilterSkills && !assignmentSkillsMatch(item.DesiredAgent.SkillIDs, skillSet) {
			continue
		}
		out = append(out, item)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *Service) CreateAssignment(ctx context.Context, input Assignment) (Assignment, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	roleID := strings.TrimSpace(input.RoleID)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if roleID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("role_id is required"))
	}
	profileID := strings.TrimSpace(input.ProfileID)
	if profileID != "" {
		if _, err := s.store.GetAgentProfile(ctx, profileID); err != nil {
			return Assignment{}, err
		}
	}
	executionProfileID := strings.TrimSpace(input.ExecutionProfileID)
	if executionProfileID != "" {
		if _, err := s.store.GetExecutionProfile(ctx, executionProfileID); err != nil {
			return Assignment{}, err
		}
	}
	executionMode, err := normalizeExecutionMode(input.ExecutionMode, false)
	if err != nil {
		return Assignment{}, err
	}
	desiredAgent := input.DesiredAgent
	desiredAgent.Kind = strings.TrimSpace(desiredAgent.Kind)
	if desiredAgent.Kind == "" {
		desiredAgent.Kind = DesiredAgentAny
	}
	desiredAgent.SkillIDs = compactStrings(desiredAgent.SkillIDs)
	now := s.now()
	item := Assignment{
		ID:                 firstNonEmpty(strings.TrimSpace(input.ID), newID("asgn")),
		ProjectID:          projectID,
		WorkItemID:         workItemID,
		RoleID:             roleID,
		ProfileID:          profileID,
		ExecutionProfileID: executionProfileID,
		ExecutionMode:      executionMode,
		Status:             AssignmentQueued,
		DesiredAgent:       desiredAgent,
		ExecutionRef:       strings.TrimSpace(input.ExecutionRef),
		ContextSnapshotID:  strings.TrimSpace(input.ContextSnapshotID),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	return s.store.CreateAssignment(ctx, item)
}

func (s *Service) ClaimAssignment(ctx context.Context, projectID, id, claimedBy string) (Assignment, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	claimedBy = strings.TrimSpace(claimedBy)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	if claimedBy == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("claimed_by is required"))
	}
	return s.store.ClaimAssignment(ctx, projectID, id, claimedBy, s.now)
}

func (s *Service) UpdateAssignmentStatus(ctx context.Context, projectID, id, status, executionRef string) (Assignment, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	if !isProgressAssignmentStatus(status) {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment status must be running or awaiting_review"))
	}
	item, err := s.store.GetAssignment(ctx, projectID, id)
	if err != nil {
		return Assignment{}, err
	}
	if isTerminalAssignmentStatus(item.Status) {
		return Assignment{}, ErrConflict
	}
	if item.Status == AssignmentQueued {
		return Assignment{}, ErrConflict
	}
	item.Status = status
	if trimmed := strings.TrimSpace(executionRef); trimmed != "" {
		item.ExecutionRef = trimmed
	}
	item.UpdatedAt = s.now()
	return s.store.UpdateAssignment(ctx, item)
}

func (s *Service) AssignmentContext(ctx context.Context, projectID, id string) (AssignmentContext, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return AssignmentContext{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return AssignmentContext{}, errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return AssignmentContext{}, err
	}
	assignment, err := s.store.GetAssignment(ctx, projectID, id)
	if err != nil {
		return AssignmentContext{}, err
	}
	workItem, err := s.store.GetWorkItem(ctx, projectID, assignment.WorkItemID)
	if err != nil {
		return AssignmentContext{}, err
	}
	var warnings []string
	var role *Role
	if assignment.RoleID != "" {
		foundRole, err := s.store.GetRole(ctx, projectID, assignment.RoleID)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return AssignmentContext{}, err
			}
			warnings = append(warnings, "assignment role was not found")
		} else {
			role = &foundRole
		}
	}
	return AssignmentContext{
		ID:         newID("ctx"),
		Project:    project,
		WorkItem:   workItem,
		Role:       role,
		Assignment: assignment,
		Warnings:   warnings,
		CreatedAt:  s.now(),
	}, nil
}

func (s *Service) AssignmentLaunchPacket(ctx context.Context, projectID, id string) (AssignmentLaunchPacket, error) {
	packetContext, err := s.AssignmentContext(ctx, projectID, id)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	evidence, err := s.store.ListEvidence(ctx, packetContext.Project.ID, packetContext.WorkItem.ID)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	reviews, err := s.store.ListReviews(ctx, packetContext.Project.ID, packetContext.WorkItem.ID)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	handoffs, err := s.store.ListHandoffs(ctx, packetContext.Project.ID, packetContext.WorkItem.ID)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	memoryCandidates, err := s.store.ListMemoryCandidates(ctx, packetContext.Project.ID)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	warnings := append([]string(nil), packetContext.Warnings...)
	var profile *AgentProfile
	profileID := firstNonEmpty(packetContext.Assignment.ProfileID, profileIDFromRole(packetContext.Role))
	if profileID != "" {
		resolvedProfile, err := s.store.GetAgentProfile(ctx, profileID)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return AssignmentLaunchPacket{}, err
			}
			warnings = append(warnings, "assignment profile was not found")
		} else {
			profile = &resolvedProfile
		}
	}
	var executionProfile *ExecutionProfile
	if packetContext.Assignment.ExecutionProfileID != "" {
		resolvedExecutionProfile, err := s.store.GetExecutionProfile(ctx, packetContext.Assignment.ExecutionProfileID)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return AssignmentLaunchPacket{}, err
			}
			warnings = append(warnings, "assignment execution profile was not found")
		} else {
			executionProfile = &resolvedExecutionProfile
		}
	}
	resolvedSkills, skillWarnings, err := s.resolveLaunchPacketSkills(ctx, packetContext.Project.ID, packetContext.Assignment, packetContext.Role, profile)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	warnings = append(warnings, skillWarnings...)
	return AssignmentLaunchPacket{
		ID:               newID("launch"),
		Kind:             LaunchPacketKindAssignment,
		Project:          packetContext.Project,
		WorkItem:         packetContext.WorkItem,
		Role:             packetContext.Role,
		Profile:          profile,
		ExecutionProfile: executionProfile,
		Skills:           resolvedSkills,
		Assignment:       packetContext.Assignment,
		Evidence:         evidence,
		Reviews:          reviews,
		Handoffs:         handoffs,
		MemoryCandidates: memoryCandidates,
		Warnings:         warnings,
		CreatedAt:        s.now(),
	}, nil
}

func (s *Service) CompleteAssignment(ctx context.Context, projectID, id, status, executionRef string) (Assignment, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = AssignmentCompleted
	}
	if !isCompletionAssignmentStatus(status) {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment status must be completed, failed, cancelled, or awaiting_review"))
	}
	item, err := s.store.GetAssignment(ctx, projectID, id)
	if err != nil {
		return Assignment{}, err
	}
	if isTerminalAssignmentStatus(item.Status) {
		return Assignment{}, ErrConflict
	}
	item.Status = status
	if trimmed := strings.TrimSpace(executionRef); trimmed != "" {
		item.ExecutionRef = trimmed
	}
	item.UpdatedAt = s.now()
	return s.store.UpdateAssignment(ctx, item)
}

func (s *Service) ListEvidence(ctx context.Context, projectID, workItemID string) ([]Evidence, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	return s.store.ListEvidence(ctx, projectID, workItemID)
}

func (s *Service) CreateEvidence(ctx context.Context, input Evidence) (Evidence, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	locator := strings.TrimSpace(input.Locator)
	if projectID == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if title == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("evidence title is required"))
	}
	if body == "" && locator == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("evidence body or locator is required"))
	}
	trustLabel := strings.TrimSpace(input.TrustLabel)
	if trustLabel == "" {
		trustLabel = EvidenceTrustOperator
	}
	now := s.now()
	item := Evidence{
		ID:         firstNonEmpty(strings.TrimSpace(input.ID), newID("ev")),
		ProjectID:  projectID,
		WorkItemID: workItemID,
		Title:      title,
		Body:       body,
		Locator:    locator,
		TrustLabel: trustLabel,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return s.store.CreateEvidence(ctx, item)
}

func (s *Service) ListReviews(ctx context.Context, projectID, workItemID string) ([]Review, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	return s.store.ListReviews(ctx, projectID, workItemID)
}

func (s *Service) CreateReview(ctx context.Context, input Review) (Review, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return Review{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Review{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if body == "" {
		return Review{}, errors.Join(ErrInvalid, errors.New("review body is required"))
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Review"
	}
	verdict, err := normalizeReviewVerdict(input.Verdict)
	if err != nil {
		return Review{}, err
	}
	risk, err := normalizeReviewRisk(input.Risk)
	if err != nil {
		return Review{}, err
	}
	now := s.now()
	item := Review{
		ID:             firstNonEmpty(strings.TrimSpace(input.ID), newID("rev")),
		ProjectID:      projectID,
		WorkItemID:     workItemID,
		AssignmentID:   strings.TrimSpace(input.AssignmentID),
		ReviewerRoleID: strings.TrimSpace(input.ReviewerRoleID),
		Title:          title,
		Body:           body,
		Verdict:        verdict,
		Risk:           risk,
		Status:         ReviewStatusRecorded,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.store.CreateReview(ctx, item)
}

func (s *Service) ListHandoffs(ctx context.Context, projectID, workItemID string) ([]Handoff, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	return s.store.ListHandoffs(ctx, projectID, workItemID)
}

func (s *Service) CreateHandoff(ctx context.Context, input Handoff) (Handoff, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if title == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff title is required"))
	}
	if body == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff body is required"))
	}
	now := s.now()
	item := Handoff{
		ID:         firstNonEmpty(strings.TrimSpace(input.ID), newID("handoff")),
		ProjectID:  projectID,
		WorkItemID: workItemID,
		FromRoleID: strings.TrimSpace(input.FromRoleID),
		ToRoleID:   strings.TrimSpace(input.ToRoleID),
		Title:      title,
		Body:       body,
		Status:     HandoffStatusOpen,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return s.store.CreateHandoff(ctx, item)
}

func (s *Service) ListMemoryCandidates(ctx context.Context, projectID string) ([]MemoryCandidate, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListMemoryCandidates(ctx, projectID)
}

func (s *Service) CreateMemoryCandidate(ctx context.Context, input MemoryCandidate) (MemoryCandidate, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if title == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory candidate title is required"))
	}
	if body == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory candidate body is required"))
	}
	trustLabel := strings.TrimSpace(input.TrustLabel)
	if trustLabel == "" {
		trustLabel = EvidenceTrustOperator
	}
	now := s.now()
	item := MemoryCandidate{
		ID:         firstNonEmpty(strings.TrimSpace(input.ID), newID("memcand")),
		ProjectID:  projectID,
		Title:      title,
		Body:       body,
		Status:     MemoryCandidateProposed,
		TrustLabel: trustLabel,
		SourceRef:  strings.TrimSpace(input.SourceRef),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return s.store.CreateMemoryCandidate(ctx, item)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeRoots(values []Root) []Root {
	if len(values) == 0 {
		return nil
	}
	out := make([]Root, 0, len(values))
	for _, value := range values {
		path := strings.TrimSpace(value.Path)
		if path == "" {
			continue
		}
		id := strings.TrimSpace(value.ID)
		if id == "" {
			id = newID("root")
		}
		out = append(out, Root{
			ID:        id,
			Path:      filepath.Clean(path),
			Kind:      strings.TrimSpace(value.Kind),
			GitRemote: strings.TrimSpace(value.GitRemote),
			GitBranch: strings.TrimSpace(value.GitBranch),
			Active:    value.Active,
		})
	}
	return out
}

func (s *Service) normalizeProjectSkill(input ProjectSkill, creating bool) (ProjectSkill, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := normalizeSkillID(input.ID)
	if projectID == "" {
		return ProjectSkill{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return ProjectSkill{}, errors.Join(ErrInvalid, errors.New("skill id is required"))
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = id
	}
	format := strings.TrimSpace(input.Format)
	if format == "" {
		format = SkillFormatMarkdown
	}
	if format != SkillFormatMarkdown {
		return ProjectSkill{}, errors.Join(ErrInvalid, errors.New("unsupported skill format"))
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = SkillStatusAvailable
	}
	if !validSkillStatus(status) {
		return ProjectSkill{}, errors.Join(ErrInvalid, errors.New("unsupported skill status"))
	}
	trustLabel := strings.TrimSpace(input.TrustLabel)
	if trustLabel == "" {
		trustLabel = SkillTrustWorkspace
	}
	path := strings.TrimSpace(input.Path)
	if path != "" {
		path = filepath.ToSlash(filepath.Clean(path))
	}
	now := s.now()
	createdAt := input.CreatedAt
	if creating || createdAt.IsZero() {
		createdAt = now
	}
	discoveredAt := input.DiscoveredAt
	if discoveredAt.IsZero() && strings.TrimSpace(input.Path) != "" {
		discoveredAt = now
	}
	enabled := input.Enabled
	if creating {
		enabled = true
	}
	return ProjectSkill{
		ID:           id,
		ProjectID:    projectID,
		Title:        title,
		Description:  strings.TrimSpace(input.Description),
		Path:         path,
		RootID:       strings.TrimSpace(input.RootID),
		Format:       format,
		Enabled:      enabled,
		Status:       status,
		TrustLabel:   trustLabel,
		SourceRefs:   compactStrings(input.SourceRefs),
		Warnings:     compactStrings(input.Warnings),
		DiscoveredAt: discoveredAt,
		CreatedAt:    createdAt,
		UpdatedAt:    now,
	}, nil
}

func (s *Service) createDiscoveredProjectSkill(ctx context.Context, item ProjectSkill) error {
	item.Enabled = true
	_, err := s.store.CreateProjectSkill(ctx, item)
	return err
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsStringValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func normalizeSkillID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == ' ' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func assignmentAgentKindMatches(desiredKind, agentKind string) bool {
	desiredKind = strings.TrimSpace(desiredKind)
	agentKind = strings.TrimSpace(agentKind)
	if desiredKind == "" || desiredKind == DesiredAgentAny || agentKind == "" || agentKind == DesiredAgentAny {
		return true
	}
	return desiredKind == agentKind
}

func assignmentSkillsMatch(required []string, available map[string]struct{}) bool {
	for _, id := range compactStrings(required) {
		normalizedID := normalizeSkillID(id)
		if normalizedID == "" {
			return false
		}
		if _, ok := available[normalizedID]; !ok {
			return false
		}
	}
	return true
}

func validSkillStatus(value string) bool {
	switch value {
	case SkillStatusAvailable, SkillStatusMissing, SkillStatusInvalid, SkillStatusConflict:
		return true
	default:
		return false
	}
}

type skillFileMetadata struct {
	name        string
	title       string
	description string
}

func readSkillMetadata(path string) (skillFileMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return skillFileMetadata{}, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxSkillMetadataBytes))
	if err != nil {
		return skillFileMetadata{}, err
	}
	return parseSkillMetadata(string(raw)), nil
}

func parseSkillMetadata(body string) skillFileMetadata {
	var metadata skillFileMetadata
	lines := strings.Split(body, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "---" {
				start = i + 1
				break
			}
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "name":
				metadata.name = value
			case "title":
				metadata.title = value
			case "description":
				metadata.description = value
			}
		}
	}
	for _, line := range lines[start:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			metadata.title = firstNonEmpty(metadata.title, strings.TrimSpace(strings.TrimPrefix(line, "# ")))
			break
		}
	}
	return metadata
}

func normalizeExecutionMode(value string, allowEmpty bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if allowEmpty {
			return "", nil
		}
		return ExecutionMCPPull, nil
	}
	switch value {
	case ExecutionManual, ExecutionMCPPull, ExecutionExternalAdapter, ExecutionOrchestrated:
		return value, nil
	default:
		return "", errors.Join(ErrInvalid, errors.New("unsupported execution_mode"))
	}
}

func isCompletionAssignmentStatus(status string) bool {
	switch status {
	case AssignmentCompleted, AssignmentFailed, AssignmentCancelled, AssignmentReview:
		return true
	default:
		return false
	}
}

func isTerminalAssignmentStatus(status string) bool {
	switch status {
	case AssignmentCompleted, AssignmentFailed, AssignmentCancelled:
		return true
	default:
		return false
	}
}

func isProgressAssignmentStatus(status string) bool {
	switch status {
	case AssignmentRunning, AssignmentReview:
		return true
	default:
		return false
	}
}

func normalizeReviewVerdict(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case ReviewVerdictPass, ReviewVerdictConcerns, ReviewVerdictBlocked:
		return value, nil
	case "":
		return "", errors.Join(ErrInvalid, errors.New("review verdict is required"))
	default:
		return "", errors.Join(ErrInvalid, errors.New("unsupported review verdict"))
	}
}

func normalizeReviewRisk(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "", ReviewRiskLow, ReviewRiskMedium, ReviewRiskHigh:
		return value, nil
	default:
		return "", errors.Join(ErrInvalid, errors.New("unsupported review risk"))
	}
}

func profileIDFromRole(role *Role) string {
	if role == nil {
		return ""
	}
	return role.DefaultProfileID
}

func (s *Service) resolveLaunchPacketSkills(ctx context.Context, projectID string, assignment Assignment, role *Role, profile *AgentProfile) ([]ProjectSkill, []string, error) {
	var requested []string
	requested = append(requested, assignment.DesiredAgent.SkillIDs...)
	if role != nil {
		requested = append(requested, role.DefaultSkillIDs...)
	}
	if profile != nil {
		requested = append(requested, profile.SkillIDs...)
	}
	requested = compactStrings(requested)
	if len(requested) == 0 {
		return nil, nil, nil
	}
	var skills []ProjectSkill
	var warnings []string
	for _, id := range requested {
		normalizedID := normalizeSkillID(id)
		if normalizedID == "" {
			warnings = appendUnique(warnings, "skill id is invalid: "+id)
			continue
		}
		skill, err := s.store.GetProjectSkill(ctx, projectID, normalizedID)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return nil, nil, err
			}
			warnings = appendUnique(warnings, "skill was not found: "+normalizedID)
			continue
		}
		if !skill.Enabled {
			warnings = appendUnique(warnings, "skill is disabled: "+normalizedID)
			continue
		}
		if skill.Status != SkillStatusAvailable {
			warnings = appendUnique(warnings, "skill is not available: "+normalizedID+" ("+skill.Status+")")
			continue
		}
		skills = append(skills, skill)
	}
	return skills, warnings, nil
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
