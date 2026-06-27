package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxGuidanceMetadataBytes = 64 * 1024
	maxSkillMetadataBytes    = 64 * 1024
)

type skillDiscoveryBase struct {
	path       string
	sourceRefs []string
}

type Service struct {
	store       Store
	now         func() time.Time
	assistantMu sync.Mutex
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	return s.store.ListProjects(ctx)
}

func (s *Service) GetProject(ctx context.Context, id string) (Project, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Project{}, errors.Join(ErrInvalid, errors.New("project id is required"))
	}
	return s.store.GetProject(ctx, id)
}

func (s *Service) CreateProject(ctx context.Context, input Project) (Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Project{}, errors.Join(ErrInvalid, errors.New("project name is required"))
	}
	roots := normalizeRoots(input.Roots)
	defaultRootID, err := normalizeDefaultRootID(input.DefaultRootID, roots)
	if err != nil {
		return Project{}, err
	}
	now := s.now()
	item := Project{
		ID:                        firstNonEmpty(strings.TrimSpace(input.ID), newID("proj")),
		Name:                      name,
		Description:               strings.TrimSpace(input.Description),
		Roots:                     roots,
		DefaultRootID:             defaultRootID,
		DefaultProfileID:          strings.TrimSpace(input.DefaultProfileID),
		DefaultExecutionProfileID: strings.TrimSpace(input.DefaultExecutionProfileID),
		ContextSources:            normalizeSources(input.ContextSources, nil, now),
		CreatedAt:                 now,
		UpdatedAt:                 now,
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
	roots := normalizeRoots(input.Roots)
	defaultRootID, err := normalizeDefaultRootID(input.DefaultRootID, roots)
	if err != nil {
		return Project{}, err
	}
	now := s.now()
	item := Project{
		ID:                        id,
		Name:                      name,
		Description:               strings.TrimSpace(input.Description),
		Roots:                     roots,
		DefaultRootID:             defaultRootID,
		DefaultProfileID:          strings.TrimSpace(input.DefaultProfileID),
		DefaultExecutionProfileID: strings.TrimSpace(input.DefaultExecutionProfileID),
		ContextSources:            normalizeSources(input.ContextSources, existingSourcesByID(existing.ContextSources), now),
		CreatedAt:                 existing.CreatedAt,
		UpdatedAt:                 now,
	}
	return s.store.UpdateProject(ctx, item)
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("project id is required"))
	}
	return s.store.DeleteProject(ctx, id)
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
		for _, base := range projectSkillDiscoveryBases(rootPath, root, project.ContextSources) {
			basePath := filepath.Join(rootPath, filepath.FromSlash(base.path))
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
					SourceRefs:   compactStrings(base.sourceRefs),
					DiscoveredAt: now,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				if prior, ok := discovered[id]; ok {
					prior.Status = SkillStatusConflict
					prior.Warnings = appendUnique(prior.Warnings, "skill id is declared by multiple paths: "+prior.Path+" and "+item.Path)
					for _, ref := range item.SourceRefs {
						prior.SourceRefs = appendUnique(prior.SourceRefs, ref)
					}
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

func (s *Service) DeleteAgentProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("profile id is required"))
	}
	return s.store.DeleteAgentProfile(ctx, id)
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

func (s *Service) DeleteExecutionProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("execution profile id is required"))
	}
	return s.store.DeleteExecutionProfile(ctx, id)
}

func (s *Service) ListWorkItems(ctx context.Context, projectID string) ([]WorkItem, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListWorkItems(ctx, projectID)
}

func (s *Service) GetWorkItem(ctx context.Context, projectID, id string) (WorkItem, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	return s.store.GetWorkItem(ctx, projectID, id)
}

func (s *Service) CreateWorkItem(ctx context.Context, input WorkItem) (WorkItem, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	title := strings.TrimSpace(input.Title)
	rootID := strings.TrimSpace(input.RootID)
	if projectID == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if title == "" {
		return WorkItem{}, errors.Join(ErrInvalid, errors.New("work item title is required"))
	}
	if err := s.validateProjectRoot(ctx, projectID, rootID); err != nil {
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
		RootID:          rootID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return s.store.CreateWorkItem(ctx, item)
}

func (s *Service) UpdateWorkItem(ctx context.Context, input WorkItem) (WorkItem, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.ID)
	title := strings.TrimSpace(input.Title)
	rootID := strings.TrimSpace(input.RootID)
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
	if err := s.validateProjectRoot(ctx, projectID, rootID); err != nil {
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
		RootID:          rootID,
		CreatedAt:       existing.CreatedAt,
		UpdatedAt:       s.now(),
	}
	return s.store.UpdateWorkItem(ctx, item)
}

func (s *Service) DeleteWorkItem(ctx context.Context, projectID, id string) error {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	return s.store.DeleteWorkItem(ctx, projectID, id)
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
	defaultExecutionProfileID := strings.TrimSpace(input.DefaultExecutionProfileID)
	if defaultExecutionProfileID != "" {
		if _, err := s.store.GetExecutionProfile(ctx, defaultExecutionProfileID); err != nil {
			return Role{}, err
		}
	}
	executionMode, err := normalizeExecutionMode(input.DefaultExecutionMode, true)
	if err != nil {
		return Role{}, err
	}
	item := Role{
		ID:                        firstNonEmpty(strings.TrimSpace(input.ID), newID("role")),
		ProjectID:                 projectID,
		Name:                      name,
		Description:               strings.TrimSpace(input.Description),
		Instructions:              strings.TrimSpace(input.Instructions),
		DefaultProfileID:          defaultProfileID,
		DefaultExecutionProfileID: defaultExecutionProfileID,
		DefaultSkillIDs:           compactStrings(input.DefaultSkillIDs),
		DefaultExecutionMode:      executionMode,
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
	defaultExecutionProfileID := strings.TrimSpace(input.DefaultExecutionProfileID)
	if defaultExecutionProfileID != "" {
		if _, err := s.store.GetExecutionProfile(ctx, defaultExecutionProfileID); err != nil {
			return Role{}, err
		}
	}
	executionMode, err := normalizeExecutionMode(input.DefaultExecutionMode, true)
	if err != nil {
		return Role{}, err
	}
	item := Role{
		ID:                        id,
		ProjectID:                 projectID,
		Name:                      name,
		Description:               strings.TrimSpace(input.Description),
		Instructions:              strings.TrimSpace(input.Instructions),
		DefaultProfileID:          defaultProfileID,
		DefaultExecutionProfileID: defaultExecutionProfileID,
		DefaultSkillIDs:           compactStrings(input.DefaultSkillIDs),
		DefaultExecutionMode:      executionMode,
	}
	return s.store.UpdateRole(ctx, item)
}

func (s *Service) DeleteRole(ctx context.Context, projectID, id string) error {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("role_id is required"))
	}
	return s.store.DeleteRole(ctx, projectID, id)
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
	rootID := strings.TrimSpace(input.RootID)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if roleID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("role_id is required"))
	}
	if err := s.validateProjectRoot(ctx, projectID, rootID); err != nil {
		return Assignment{}, err
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
	desiredAgent := normalizeDesiredAgent(input.DesiredAgent)
	now := s.now()
	item := Assignment{
		ID:                 firstNonEmpty(strings.TrimSpace(input.ID), newID("asgn")),
		ProjectID:          projectID,
		WorkItemID:         workItemID,
		RoleID:             roleID,
		RootID:             rootID,
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

func (s *Service) UpdateAssignment(ctx context.Context, input Assignment) (Assignment, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.ID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	roleID := strings.TrimSpace(input.RoleID)
	rootID := strings.TrimSpace(input.RootID)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	if workItemID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if roleID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("role_id is required"))
	}
	existing, err := s.store.GetAssignment(ctx, projectID, id)
	if err != nil {
		return Assignment{}, err
	}
	if _, err := s.store.GetWorkItem(ctx, projectID, workItemID); err != nil {
		return Assignment{}, err
	}
	if _, err := s.store.GetRole(ctx, projectID, roleID); err != nil {
		return Assignment{}, err
	}
	if err := s.validateProjectRoot(ctx, projectID, rootID); err != nil {
		return Assignment{}, err
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
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = existing.Status
	}
	if !isAssignmentStatus(status) {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment status is invalid"))
	}
	claimedBy := strings.TrimSpace(input.ClaimedBy)
	if claimedBy == "" {
		claimedBy = existing.ClaimedBy
	}
	item := Assignment{
		ID:                 id,
		ProjectID:          projectID,
		WorkItemID:         workItemID,
		RoleID:             roleID,
		RootID:             rootID,
		ProfileID:          profileID,
		ExecutionProfileID: executionProfileID,
		ExecutionMode:      executionMode,
		Status:             status,
		DesiredAgent:       normalizeDesiredAgent(input.DesiredAgent),
		ClaimedBy:          claimedBy,
		ExecutionRef:       strings.TrimSpace(input.ExecutionRef),
		ContextSnapshotID:  strings.TrimSpace(input.ContextSnapshotID),
		CreatedAt:          existing.CreatedAt,
		UpdatedAt:          s.now(),
	}
	return s.store.UpdateAssignment(ctx, item)
}

func (s *Service) GetAssignment(ctx context.Context, projectID, id string) (Assignment, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return Assignment{}, errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	return s.store.GetAssignment(ctx, projectID, id)
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

func (s *Service) DeleteAssignment(ctx context.Context, projectID, id string) error {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("assignment_id is required"))
	}
	return s.store.DeleteAssignment(ctx, projectID, id)
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
	artifacts, err := s.store.ListArtifacts(ctx, packetContext.Project.ID, packetContext.WorkItem.ID)
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
	memoryEntries, err := s.store.ListMemoryEntries(ctx, packetContext.Project.ID, false)
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	memoryCandidates, err := s.store.ListMemoryCandidates(ctx, MemoryCandidateFilter{
		ProjectID: packetContext.Project.ID,
	})
	if err != nil {
		return AssignmentLaunchPacket{}, err
	}
	warnings := append([]string(nil), packetContext.Warnings...)
	var profile *AgentProfile
	profileID := firstNonEmpty(packetContext.Assignment.ProfileID, profileIDFromRole(packetContext.Role), packetContext.Project.DefaultProfileID)
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
	executionProfileID := firstNonEmpty(packetContext.Assignment.ExecutionProfileID, executionProfileIDFromRole(packetContext.Role), packetContext.Project.DefaultExecutionProfileID)
	if executionProfileID != "" {
		resolvedExecutionProfile, err := s.store.GetExecutionProfile(ctx, executionProfileID)
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
		Artifacts:        artifacts,
		Evidence:         evidence,
		Reviews:          reviews,
		Handoffs:         handoffs,
		Memory:           memoryEntries,
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

func (s *Service) ListArtifacts(ctx context.Context, projectID, workItemID string) ([]Artifact, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	return s.store.ListArtifacts(ctx, projectID, workItemID)
}

func (s *Service) GetArtifact(ctx context.Context, projectID, workItemID, id string) (Artifact, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if id == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("artifact_id is required"))
	}
	return s.store.GetArtifact(ctx, projectID, workItemID, id)
}

func (s *Service) CreateArtifact(ctx context.Context, input Artifact) (Artifact, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	kind := strings.TrimSpace(input.Kind)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if kind == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("artifact kind is required"))
	}
	if body == "" {
		return Artifact{}, errors.Join(ErrInvalid, errors.New("artifact body is required"))
	}
	assignmentID := strings.TrimSpace(input.AssignmentID)
	if assignmentID != "" {
		assignment, err := s.store.GetAssignment(ctx, projectID, assignmentID)
		if err != nil {
			return Artifact{}, err
		}
		if assignment.WorkItemID != workItemID {
			return Artifact{}, errors.Join(ErrNotFound, errors.New("assignment_id was not found in work item"))
		}
	}
	authorRoleID := strings.TrimSpace(input.AuthorRoleID)
	if authorRoleID != "" {
		if _, err := s.store.GetRole(ctx, projectID, authorRoleID); err != nil {
			return Artifact{}, err
		}
	}
	now := s.now()
	item := Artifact{
		ID:             firstNonEmpty(strings.TrimSpace(input.ID), newID("art")),
		ProjectID:      projectID,
		WorkItemID:     workItemID,
		AssignmentID:   assignmentID,
		Kind:           kind,
		Title:          strings.TrimSpace(input.Title),
		Body:           body,
		AuthorRoleID:   authorRoleID,
		ProvenanceKind: strings.TrimSpace(input.ProvenanceKind),
		TrustLabel:     strings.TrimSpace(input.TrustLabel),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.store.CreateArtifact(ctx, item)
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

func (s *Service) GetEvidence(ctx context.Context, projectID, workItemID, id string) (Evidence, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if id == "" {
		return Evidence{}, errors.Join(ErrInvalid, errors.New("evidence_id is required"))
	}
	return s.store.GetEvidence(ctx, projectID, workItemID, id)
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
	assignmentID := strings.TrimSpace(input.AssignmentID)
	if assignmentID != "" {
		assignment, err := s.store.GetAssignment(ctx, projectID, assignmentID)
		if err != nil {
			return Evidence{}, err
		}
		if assignment.WorkItemID != workItemID {
			return Evidence{}, errors.Join(ErrNotFound, errors.New("assignment_id was not found in work item"))
		}
	}
	trustLabel := strings.TrimSpace(input.TrustLabel)
	if trustLabel == "" {
		trustLabel = EvidenceTrustOperator
	}
	now := s.now()
	item := Evidence{
		ID:           firstNonEmpty(strings.TrimSpace(input.ID), newID("ev")),
		ProjectID:    projectID,
		WorkItemID:   workItemID,
		AssignmentID: assignmentID,
		Title:        title,
		Body:         body,
		Locator:      locator,
		TrustLabel:   trustLabel,
		CreatedAt:    now,
		UpdatedAt:    now,
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

func (s *Service) GetReview(ctx context.Context, projectID, workItemID, id string) (Review, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return Review{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Review{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if id == "" {
		return Review{}, errors.Join(ErrInvalid, errors.New("review_id is required"))
	}
	return s.store.GetReview(ctx, projectID, workItemID, id)
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

func (s *Service) GetHandoff(ctx context.Context, projectID, workItemID, id string) (Handoff, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if id == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff_id is required"))
	}
	return s.store.GetHandoff(ctx, projectID, workItemID, id)
}

func (s *Service) CreateHandoff(ctx context.Context, input Handoff) (Handoff, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = HandoffStatusOpen
	}
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
	if !isHandoffStatus(status) {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff status is invalid"))
	}
	now := s.now()
	item := Handoff{
		ID:                    firstNonEmpty(strings.TrimSpace(input.ID), newID("handoff")),
		ProjectID:             projectID,
		WorkItemID:            workItemID,
		SourceAssignmentID:    strings.TrimSpace(input.SourceAssignmentID),
		SourceRunID:           strings.TrimSpace(input.SourceRunID),
		SourceChatSessionID:   strings.TrimSpace(input.SourceChatSessionID),
		SourceMessageID:       strings.TrimSpace(input.SourceMessageID),
		FromRoleID:            strings.TrimSpace(input.FromRoleID),
		ToRoleID:              strings.TrimSpace(input.ToRoleID),
		TargetAssignmentID:    strings.TrimSpace(input.TargetAssignmentID),
		TargetWorkItemID:      strings.TrimSpace(input.TargetWorkItemID),
		Title:                 title,
		Body:                  body,
		RecommendedNextAction: strings.TrimSpace(input.RecommendedNextAction),
		LinkedArtifactIDs:     compactStrings(input.LinkedArtifactIDs),
		LinkedMemoryIDs:       compactStrings(input.LinkedMemoryIDs),
		ContextRefs:           compactStrings(input.ContextRefs),
		Status:                status,
		ProvenanceKind:        strings.TrimSpace(input.ProvenanceKind),
		TrustLabel:            strings.TrimSpace(input.TrustLabel),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	return s.store.CreateHandoff(ctx, item)
}

func (s *Service) UpdateHandoff(ctx context.Context, input Handoff) (Handoff, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	workItemID := strings.TrimSpace(input.WorkItemID)
	id := strings.TrimSpace(input.ID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = HandoffStatusOpen
	}
	if projectID == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if id == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff_id is required"))
	}
	if title == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff title is required"))
	}
	if body == "" {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff body is required"))
	}
	if !isHandoffStatus(status) {
		return Handoff{}, errors.Join(ErrInvalid, errors.New("handoff status is invalid"))
	}
	existing, err := s.store.GetHandoff(ctx, projectID, workItemID, id)
	if err != nil {
		return Handoff{}, err
	}
	item := Handoff{
		ID:                    id,
		ProjectID:             projectID,
		WorkItemID:            workItemID,
		SourceAssignmentID:    strings.TrimSpace(input.SourceAssignmentID),
		SourceRunID:           strings.TrimSpace(input.SourceRunID),
		SourceChatSessionID:   strings.TrimSpace(input.SourceChatSessionID),
		SourceMessageID:       strings.TrimSpace(input.SourceMessageID),
		FromRoleID:            strings.TrimSpace(input.FromRoleID),
		ToRoleID:              strings.TrimSpace(input.ToRoleID),
		TargetAssignmentID:    strings.TrimSpace(input.TargetAssignmentID),
		TargetWorkItemID:      strings.TrimSpace(input.TargetWorkItemID),
		Title:                 title,
		Body:                  body,
		RecommendedNextAction: strings.TrimSpace(input.RecommendedNextAction),
		LinkedArtifactIDs:     compactStrings(input.LinkedArtifactIDs),
		LinkedMemoryIDs:       compactStrings(input.LinkedMemoryIDs),
		ContextRefs:           compactStrings(input.ContextRefs),
		Status:                status,
		ProvenanceKind:        strings.TrimSpace(input.ProvenanceKind),
		TrustLabel:            strings.TrimSpace(input.TrustLabel),
		CreatedAt:             existing.CreatedAt,
		UpdatedAt:             s.now(),
	}
	return s.store.UpdateHandoff(ctx, item)
}

func (s *Service) UpdateHandoffStatus(ctx context.Context, projectID, workItemID, id, status string) (Handoff, error) {
	existing, err := s.GetHandoff(ctx, projectID, workItemID, id)
	if err != nil {
		return Handoff{}, err
	}
	existing.Status = status
	return s.UpdateHandoff(ctx, existing)
}

func (s *Service) DeleteHandoff(ctx context.Context, projectID, workItemID, id string) error {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("handoff_id is required"))
	}
	return s.store.DeleteHandoff(ctx, projectID, workItemID, id)
}

func (s *Service) ListMemoryEntries(ctx context.Context, projectID string, includeDisabled bool) ([]MemoryEntry, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	return s.store.ListMemoryEntries(ctx, projectID, includeDisabled)
}

func (s *Service) GetMemoryEntry(ctx context.Context, projectID, id string) (MemoryEntry, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory_id is required"))
	}
	return s.store.GetMemoryEntry(ctx, projectID, id)
}

func (s *Service) CreateMemoryEntry(ctx context.Context, input MemoryEntry) (MemoryEntry, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if title == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory title is required"))
	}
	if body == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory body is required"))
	}
	trustLabel := strings.TrimSpace(input.TrustLabel)
	if trustLabel == "" {
		trustLabel = MemoryTrustOperator
	}
	createdAt, updatedAt := importedTimestamps(input.CreatedAt, input.UpdatedAt, s.now())
	item := MemoryEntry{
		ID:         firstNonEmpty(strings.TrimSpace(input.ID), newID("mem")),
		ProjectID:  projectID,
		Title:      title,
		Body:       body,
		TrustLabel: trustLabel,
		SourceKind: strings.TrimSpace(input.SourceKind),
		SourceID:   strings.TrimSpace(input.SourceID),
		Enabled:    true,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
	return s.store.CreateMemoryEntry(ctx, item)
}

func (s *Service) UpdateMemoryEntry(ctx context.Context, input MemoryEntry) (MemoryEntry, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.ID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory_id is required"))
	}
	if title == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory title is required"))
	}
	if body == "" {
		return MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory body is required"))
	}
	trustLabel := strings.TrimSpace(input.TrustLabel)
	if trustLabel == "" {
		trustLabel = MemoryTrustOperator
	}
	existing, err := s.store.GetMemoryEntry(ctx, projectID, id)
	if err != nil {
		return MemoryEntry{}, err
	}
	updatedAt := input.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now()
	}
	item := MemoryEntry{
		ID:         id,
		ProjectID:  projectID,
		Title:      title,
		Body:       body,
		TrustLabel: trustLabel,
		SourceKind: strings.TrimSpace(input.SourceKind),
		SourceID:   strings.TrimSpace(input.SourceID),
		Enabled:    input.Enabled,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  updatedAt,
	}
	return s.store.UpdateMemoryEntry(ctx, item)
}

func (s *Service) DeleteMemoryEntry(ctx context.Context, projectID, id string) error {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("memory_id is required"))
	}
	return s.store.DeleteMemoryEntry(ctx, projectID, id)
}

func (s *Service) ListMemoryCandidates(ctx context.Context, filter MemoryCandidateFilter) ([]MemoryCandidate, error) {
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.ProjectID == "" {
		return nil, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if filter.Status != "" && !isMemoryCandidateStatus(filter.Status) {
		return nil, errors.Join(ErrInvalid, errors.New("memory candidate status is invalid"))
	}
	return s.store.ListMemoryCandidates(ctx, filter)
}

func (s *Service) GetMemoryCandidate(ctx context.Context, projectID, id string) (MemoryCandidate, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory_candidate_id is required"))
	}
	return s.store.GetMemoryCandidate(ctx, projectID, id)
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
	suggestedTrustLabel := firstNonEmpty(strings.TrimSpace(input.SuggestedTrustLabel), MemoryTrustGenerated)
	suggestedSourceKind := firstNonEmpty(strings.TrimSpace(input.SuggestedSourceKind), MemorySourceGenerated)
	createdAt, updatedAt := importedTimestamps(input.CreatedAt, input.UpdatedAt, s.now())
	item := MemoryCandidate{
		ID:                  firstNonEmpty(strings.TrimSpace(input.ID), newID("memcand")),
		ProjectID:           projectID,
		Title:               title,
		Body:                body,
		SuggestedKind:       strings.TrimSpace(input.SuggestedKind),
		SuggestedTrustLabel: suggestedTrustLabel,
		SuggestedSourceKind: suggestedSourceKind,
		SuggestedSourceID:   strings.TrimSpace(input.SuggestedSourceID),
		SourceRefs:          normalizeMemoryCandidateSourceRefs(input.SourceRefs),
		Status:              MemoryCandidatePending,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
	return s.store.CreateMemoryCandidate(ctx, item)
}

func (s *Service) UpdateMemoryCandidate(ctx context.Context, input MemoryCandidate) (MemoryCandidate, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.ID)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if projectID == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory_candidate_id is required"))
	}
	if title == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory candidate title is required"))
	}
	if body == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory candidate body is required"))
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = MemoryCandidatePending
	}
	if !isMemoryCandidateStatus(status) {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory candidate status is invalid"))
	}
	existing, err := s.store.GetMemoryCandidate(ctx, projectID, id)
	if err != nil {
		return MemoryCandidate{}, err
	}
	updatedAt := input.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now()
	}
	item := MemoryCandidate{
		ID:                  id,
		ProjectID:           projectID,
		Title:               title,
		Body:                body,
		SuggestedKind:       strings.TrimSpace(input.SuggestedKind),
		SuggestedTrustLabel: firstNonEmpty(strings.TrimSpace(input.SuggestedTrustLabel), MemoryTrustGenerated),
		SuggestedSourceKind: firstNonEmpty(strings.TrimSpace(input.SuggestedSourceKind), MemorySourceGenerated),
		SuggestedSourceID:   strings.TrimSpace(input.SuggestedSourceID),
		SourceRefs:          normalizeMemoryCandidateSourceRefs(input.SourceRefs),
		Status:              status,
		StatusReason:        strings.TrimSpace(input.StatusReason),
		PromotedMemoryID:    strings.TrimSpace(input.PromotedMemoryID),
		CreatedAt:           existing.CreatedAt,
		UpdatedAt:           updatedAt,
	}
	if item.Status != MemoryCandidatePromoted {
		item.PromotedMemoryID = ""
	}
	if item.Status == MemoryCandidatePending {
		item.StatusReason = ""
	}
	return s.store.UpdateMemoryCandidate(ctx, item)
}

func (s *Service) PromoteMemoryCandidate(ctx context.Context, input MemoryCandidatePromotion) (MemoryCandidate, MemoryEntry, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	id := strings.TrimSpace(input.CandidateID)
	if projectID == "" {
		return MemoryCandidate{}, MemoryEntry{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return MemoryCandidate{}, MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory_candidate_id is required"))
	}
	candidate, err := s.store.GetMemoryCandidate(ctx, projectID, id)
	if err != nil {
		return MemoryCandidate{}, MemoryEntry{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	title := candidate.Title
	if input.Title != nil {
		title = *input.Title
	}
	body := candidate.Body
	if input.Body != nil {
		body = *input.Body
	}
	trustLabel := firstNonEmpty(candidate.SuggestedTrustLabel, MemoryTrustGenerated)
	if input.TrustLabel != nil {
		trustLabel = *input.TrustLabel
	}
	sourceKind := firstNonEmpty(candidate.SuggestedSourceKind, MemorySourceGenerated)
	if input.SourceKind != nil {
		sourceKind = *input.SourceKind
	}
	sourceID := candidate.SuggestedSourceID
	if input.SourceID != nil {
		sourceID = *input.SourceID
	}
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" {
		return MemoryCandidate{}, MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory title is required"))
	}
	if body == "" {
		return MemoryCandidate{}, MemoryEntry{}, errors.Join(ErrInvalid, errors.New("memory body is required"))
	}
	now := s.now()
	entry := MemoryEntry{
		ID:         newID("mem"),
		ProjectID:  projectID,
		Title:      title,
		Body:       body,
		TrustLabel: strings.TrimSpace(trustLabel),
		SourceKind: strings.TrimSpace(sourceKind),
		SourceID:   strings.TrimSpace(sourceID),
		Enabled:    enabled,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if entry.TrustLabel == "" {
		entry.TrustLabel = MemoryTrustGenerated
	}
	if entry.SourceKind == "" {
		entry.SourceKind = MemorySourceGenerated
	}
	return s.store.PromoteMemoryCandidate(ctx, projectID, id, entry)
}

func (s *Service) RejectMemoryCandidate(ctx context.Context, projectID, id, reason string) (MemoryCandidate, error) {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return MemoryCandidate{}, errors.Join(ErrInvalid, errors.New("memory_candidate_id is required"))
	}
	candidate, err := s.store.GetMemoryCandidate(ctx, projectID, id)
	if err != nil {
		return MemoryCandidate{}, err
	}
	if candidate.Status != MemoryCandidatePending {
		return MemoryCandidate{}, ErrConflict
	}
	candidate.Status = MemoryCandidateRejected
	candidate.StatusReason = strings.TrimSpace(reason)
	candidate.PromotedMemoryID = ""
	candidate.UpdatedAt = s.now()
	return s.store.UpdateMemoryCandidate(ctx, candidate)
}

func (s *Service) DeleteMemoryCandidate(ctx context.Context, projectID, id string) error {
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if id == "" {
		return errors.Join(ErrInvalid, errors.New("memory_candidate_id is required"))
	}
	return s.store.DeleteMemoryCandidate(ctx, projectID, id)
}

func normalizeMemoryCandidateSourceRefs(refs []MemoryCandidateSourceRef) []MemoryCandidateSourceRef {
	out := make([]MemoryCandidateSourceRef, 0, len(refs))
	for _, ref := range refs {
		kind := strings.TrimSpace(ref.Kind)
		id := strings.TrimSpace(ref.ID)
		if kind == "" || id == "" {
			continue
		}
		out = append(out, MemoryCandidateSourceRef{
			Kind:  kind,
			ID:    id,
			Title: strings.TrimSpace(ref.Title),
			URL:   strings.TrimSpace(ref.URL),
		})
	}
	return out
}

func isMemoryCandidateStatus(status string) bool {
	switch status {
	case MemoryCandidatePending, MemoryCandidatePromoted, MemoryCandidateRejected:
		return true
	default:
		return false
	}
}

func importedTimestamps(createdAt, updatedAt, fallback time.Time) (time.Time, time.Time) {
	if fallback.IsZero() {
		fallback = time.Now().UTC()
	}
	if createdAt.IsZero() {
		createdAt = fallback
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return createdAt, updatedAt
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

func normalizeSources(values []Source, existing map[string]Source, now time.Time) []Source {
	if len(values) == 0 {
		return nil
	}
	out := make([]Source, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value.ID)
		kind := strings.TrimSpace(value.Kind)
		title := strings.TrimSpace(value.Title)
		locator := strings.TrimSpace(value.Locator)
		if id == "" && kind == "" && title == "" && locator == "" {
			continue
		}
		if id == "" {
			id = newID("src")
		}
		createdAt := value.CreatedAt
		if createdAt.IsZero() {
			if prior, ok := existing[id]; ok {
				createdAt = prior.CreatedAt
			}
		}
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := value.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		out = append(out, Source{
			ID:             id,
			Kind:           kind,
			Title:          title,
			Locator:        locator,
			Enabled:        value.Enabled,
			Format:         strings.TrimSpace(value.Format),
			Scope:          strings.TrimSpace(value.Scope),
			TrustLabel:     strings.TrimSpace(value.TrustLabel),
			SourceCategory: strings.TrimSpace(value.SourceCategory),
			Metadata:       normalizeStringMap(value.Metadata),
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}
	return out
}

func existingSourcesByID(values []Source) map[string]Source {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]Source, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value.ID)
		if id == "" {
			continue
		}
		out[id] = value
	}
	return out
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDefaultRootID(value string, roots []Root) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if len(roots) == 0 {
			return "", nil
		}
		return roots[0].ID, nil
	}
	for _, root := range roots {
		if root.ID == value {
			return value, nil
		}
	}
	return "", errors.Join(ErrNotFound, errors.New("default_root_id was not found in project roots"))
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
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := input.UpdatedAt
	if creating {
		if updatedAt.IsZero() {
			updatedAt = createdAt
		}
	} else {
		updatedAt = now
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
		UpdatedAt:    updatedAt,
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

func projectSkillDiscoveryBases(rootPath string, root Root, sources []Source) []skillDiscoveryBase {
	bases := []skillDiscoveryBase{
		{path: SkillPathAgents},
		{path: SkillPathHecate},
		{path: SkillPathCairnline},
	}
	seen := map[string]int{
		SkillPathAgents:    0,
		SkillPathHecate:    1,
		SkillPathCairnline: 2,
	}
	for _, source := range sources {
		if !skillGuidanceSourceForRoot(source, root.ID) {
			continue
		}
		body, ok := readGuidanceSource(rootPath, source)
		if !ok {
			continue
		}
		for _, dir := range skillBaseDirsFromGuidance(source.Locator, body) {
			if shouldSkipSkillDiscoveryPath(dir) {
				continue
			}
			if idx, ok := seen[dir]; ok {
				bases[idx].sourceRefs = appendUnique(bases[idx].sourceRefs, source.ID)
				continue
			}
			seen[dir] = len(bases)
			bases = append(bases, skillDiscoveryBase{
				path:       dir,
				sourceRefs: compactStrings([]string{source.ID}),
			})
		}
	}
	return bases
}

func skillGuidanceSourceForRoot(source Source, rootID string) bool {
	if !source.Enabled {
		return false
	}
	locator, ok := cleanSkillDiscoveryRelativePath(source.Locator)
	if !ok || shouldSkipSkillDiscoveryPath(locator) {
		return false
	}
	if source.Metadata != nil {
		sourceRootID := strings.TrimSpace(source.Metadata["root_id"])
		if sourceRootID != "" && rootID != "" && sourceRootID != rootID {
			return false
		}
	}
	if source.Kind == "workspace_instruction" || source.Format == "agents_md" || source.Format == "claude_md" {
		return true
	}
	switch strings.ToLower(path.Base(locator)) {
	case "agents.md", "claude.md", "claude.local.md":
		return true
	default:
		return false
	}
}

func readGuidanceSource(rootPath string, source Source) (string, bool) {
	locator, ok := cleanSkillDiscoveryRelativePath(source.Locator)
	if !ok {
		return "", false
	}
	filePath := filepath.Join(rootPath, filepath.FromSlash(locator))
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() || info.Size() > maxGuidanceMetadataBytes {
		return "", false
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxGuidanceMetadataBytes))
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func skillBaseDirsFromGuidance(sourceLocator, body string) []string {
	sourceDir := path.Dir(filepath.ToSlash(strings.TrimSpace(sourceLocator)))
	if sourceDir == "." {
		sourceDir = ""
	}
	seen := make(map[string]struct{})
	var out []string
	for _, token := range guidancePathTokens(body) {
		for _, dir := range skillBaseDirsFromToken(sourceDir, token) {
			if _, ok := seen[dir]; ok {
				continue
			}
			seen[dir] = struct{}{}
			out = append(out, dir)
		}
	}
	return out
}

func guidancePathTokens(body string) []string {
	var out []string
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		token := strings.Trim(builder.String(), "`'\"()[]{}<>.,;:")
		builder.Reset()
		if token != "" {
			out = append(out, token)
		}
	}
	for _, r := range body {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-', r == '/', r == '*', r == '@':
			builder.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

func skillBaseDirsFromToken(sourceDir, token string) []string {
	token = strings.TrimSpace(strings.TrimPrefix(token, "@"))
	if token == "" || strings.Contains(token, "://") || strings.HasPrefix(token, "#") {
		return nil
	}
	token = filepath.ToSlash(token)
	token = strings.TrimPrefix(token, "./")
	if path.IsAbs(token) {
		return nil
	}
	cleaned := path.Clean(token)
	if sourceDir != "" && !strings.HasPrefix(cleaned, ".agents/") && !strings.HasPrefix(cleaned, ".hecate/") && !strings.HasPrefix(cleaned, ".cairnline/") {
		cleaned = path.Clean(path.Join(sourceDir, cleaned))
	}
	cleaned, ok := cleanSkillDiscoveryRelativePath(cleaned)
	if !ok {
		return nil
	}
	lower := strings.ToLower(cleaned)
	switch {
	case strings.Contains(lower, "/*/skill.md"):
		idx := strings.Index(lower, "/*/skill.md")
		base := cleaned[:idx]
		if base != "" && base != "." {
			return []string{base}
		}
	case strings.HasSuffix(lower, "/skill.md"):
		skillDir := path.Dir(cleaned)
		base := path.Dir(skillDir)
		if base != "." && base != "/" {
			return []string{base}
		}
	case strings.HasSuffix(lower, "/skills/readme.md"):
		return []string{path.Dir(cleaned)}
	case strings.HasSuffix(lower, "/skills"):
		return []string{cleaned}
	default:
		if idx := strings.Index(lower, "/skills/"); idx >= 0 {
			return []string{cleaned[:idx+len("/skills")]}
		}
	}
	return nil
}

func cleanSkillDiscoveryRelativePath(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.Contains(trimmed, "://") || filepath.IsAbs(trimmed) {
		return "", false
	}
	rel := filepath.ToSlash(trimmed)
	rel = strings.TrimPrefix(rel, "./")
	if path.IsAbs(rel) {
		return "", false
	}
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

func shouldSkipSkillDiscoveryPath(rel string) bool {
	cleaned, ok := cleanSkillDiscoveryRelativePath(rel)
	if !ok {
		return false
	}
	return cleaned == ".worktrees" || strings.HasPrefix(cleaned, ".worktrees/") ||
		cleaned == ".claude/worktrees" || strings.HasPrefix(cleaned, ".claude/worktrees/")
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

func isAssignmentStatus(status string) bool {
	switch status {
	case AssignmentQueued, AssignmentClaimed, AssignmentRunning, AssignmentReview, AssignmentCompleted, AssignmentFailed, AssignmentCancelled:
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

func normalizeDesiredAgent(input DesiredAgent) DesiredAgent {
	desiredAgent := input
	desiredAgent.Kind = strings.TrimSpace(desiredAgent.Kind)
	if desiredAgent.Kind == "" {
		desiredAgent.Kind = DesiredAgentAny
	}
	desiredAgent.SkillIDs = compactStrings(desiredAgent.SkillIDs)
	return desiredAgent
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

func isHandoffStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case HandoffStatusOpen, HandoffStatusAccepted, HandoffStatusSuperseded, HandoffStatusDismissed:
		return true
	default:
		return false
	}
}

func profileIDFromRole(role *Role) string {
	if role == nil {
		return ""
	}
	return role.DefaultProfileID
}

func executionProfileIDFromRole(role *Role) string {
	if role == nil {
		return ""
	}
	return role.DefaultExecutionProfileID
}

func (s *Service) validateProjectRoot(ctx context.Context, projectID, rootID string) error {
	rootID = strings.TrimSpace(rootID)
	if rootID == "" {
		return nil
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	for _, root := range project.Roots {
		if root.ID == rootID {
			return nil
		}
	}
	return errors.Join(ErrNotFound, errors.New("root_id was not found in project"))
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
