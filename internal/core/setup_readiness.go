package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) ProjectSetupReadiness(ctx context.Context, projectID string) (ProjectSetupReadiness, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ProjectSetupReadiness{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return ProjectSetupReadiness{}, err
	}
	workItems, err := s.store.ListWorkItems(ctx, projectID)
	if err != nil {
		return ProjectSetupReadiness{}, err
	}
	roles, err := s.store.ListRoles(ctx, projectID)
	if err != nil {
		return ProjectSetupReadiness{}, err
	}
	skills, err := s.store.ListProjectSkills(ctx, projectID)
	if err != nil {
		return ProjectSetupReadiness{}, err
	}
	executionProfiles, err := s.store.ListExecutionProfiles(ctx)
	if err != nil {
		return ProjectSetupReadiness{}, err
	}
	memoryEntries, err := s.store.ListMemoryEntries(ctx, projectID, true)
	if err != nil {
		return ProjectSetupReadiness{}, err
	}
	memoryCandidates, err := s.store.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: projectID})
	if err != nil {
		return ProjectSetupReadiness{}, err
	}

	summary := projectSetupReadinessSummary(project, workItems, roles, skills, executionProfiles, memoryEntries, memoryCandidates)
	setupStarted := summary.EnabledContextSourceCount > 0 ||
		summary.RoleCount > 0 ||
		summary.SkillCount > 0 ||
		summary.ExecutionProfileCount > 0 ||
		summary.SavedMemoryCount > 0 ||
		summary.PendingMemoryCandidateCount > 0
	return ProjectSetupReadiness{
		ProjectID:      project.ID,
		ShowOnboarding: summary.WorkItemCount == 0 && !setupStarted,
		SetupStarted:   setupStarted,
		FirstWorkReady: summary.WorkItemCount == 0 && setupStarted,
		Summary:        summary,
		PrimaryAction:  projectSetupReadinessAction(ProjectSetupActionSetupProject, project.ID, "Set up project"),
		Checks:         projectSetupReadinessChecks(project, summary),
		CreatedAt:      s.now(),
	}, nil
}

func projectSetupReadinessSummary(project Project, workItems []WorkItem, roles []Role, skills []ProjectSkill, executionProfiles []ExecutionProfile, memoryEntries []MemoryEntry, memoryCandidates []MemoryCandidate) ProjectSetupReadinessSummary {
	summary := ProjectSetupReadinessSummary{
		WorkItemCount:         len(workItems),
		RoleCount:             len(roles),
		SkillCount:            len(skills),
		ExecutionProfileCount: len(executionProfiles),
		HasPurpose:            strings.TrimSpace(project.Description) != "",
		HasActiveRoot:         projectHasActiveRoot(project),
		HasExecutionProfile:   len(executionProfiles) > 0,
	}
	for _, source := range project.ContextSources {
		if source.Enabled {
			summary.EnabledContextSourceCount++
		}
	}
	summary.SavedMemoryCount = len(memoryEntries)
	for _, candidate := range memoryCandidates {
		if candidate.Status == MemoryCandidatePending {
			summary.PendingMemoryCandidateCount++
		}
	}
	return summary
}

func projectSetupReadinessChecks(project Project, summary ProjectSetupReadinessSummary) []ProjectSetupReadinessCheck {
	projectID := project.ID
	hasContext := summary.EnabledContextSourceCount > 0 || summary.SkillCount > 0 || summary.SavedMemoryCount > 0 || summary.PendingMemoryCandidateCount > 0
	return []ProjectSetupReadinessCheck{
		projectSetupCheck("purpose", "Project purpose", strings.TrimSpace(project.Description), summary.HasPurpose, false, projectSetupReadinessAction(ProjectSetupActionUpdateProject, projectID, "Add purpose"), "Add a short purpose."),
		projectSetupWorkspaceCheck(project),
		projectSetupCheck("execution_profiles", "Execution profiles", projectSetupExecutionProfileDetail(summary), summary.HasExecutionProfile, true, projectSetupReadinessAction(ProjectSetupActionManageExecutionProfiles, projectID, "Add execution profile"), "Optional; add one when an orchestrator or host needs runtime hints."),
		projectSetupCheck("sources_memory", "Sources and memory", projectSetupGuidanceDetail(summary, project), hasContext, false, projectSetupReadinessAction(ProjectSetupActionManageContext, projectID, "Add context"), ""),
		projectSetupCheck("roles", "Roles", projectSetupRoleDetail(summary), summary.RoleCount > 0, false, projectSetupReadinessAction(ProjectSetupActionManageRoles, projectID, "Create role"), ""),
		projectSetupCheck("first_work_item", "First work item", projectSetupFirstWorkDetail(summary), summary.WorkItemCount > 0, false, projectSetupReadinessAction(ProjectSetupActionCreateWorkItem, projectID, "Create work item"), ""),
	}
}

func projectSetupCheck(id, label, detail string, done, optional bool, action ProjectSetupReadinessAction, fallbackDetail string) ProjectSetupReadinessCheck {
	status := ProjectSetupStatusTodo
	if done {
		status = ProjectSetupStatusReady
	} else if optional {
		status = ProjectSetupStatusOptional
	}
	check := ProjectSetupReadinessCheck{
		ID:       id,
		Label:    label,
		Detail:   strings.TrimSpace(detail),
		Status:   status,
		Optional: optional,
	}
	if check.Detail == "" {
		check.Detail = fallbackDetail
	}
	if !done && !optional {
		check.Action = &action
	}
	return check
}

func projectSetupWorkspaceCheck(project Project) ProjectSetupReadinessCheck {
	if root, ok := projectActiveRoot(project); ok {
		return ProjectSetupReadinessCheck{
			ID:       "workspace_source",
			Label:    "Workspace source",
			Detail:   root.Path,
			Status:   ProjectSetupStatusReady,
			Optional: true,
		}
	}
	return ProjectSetupReadinessCheck{
		ID:       "workspace_source",
		Label:    "Workspace source",
		Detail:   "Optional; attach files when this project needs them.",
		Status:   ProjectSetupStatusOptional,
		Optional: true,
	}
}

func projectActiveRoot(project Project) (Root, bool) {
	for _, root := range project.Roots {
		if root.Active && strings.TrimSpace(root.Path) != "" {
			return root, true
		}
	}
	return Root{}, false
}

func projectHasActiveRoot(project Project) bool {
	_, ok := projectActiveRoot(project)
	return ok
}

func projectSetupExecutionProfileDetail(summary ProjectSetupReadinessSummary) string {
	if summary.ExecutionProfileCount > 0 {
		return projectSetupCountLabel(summary.ExecutionProfileCount, "execution profile")
	}
	return "Optional; add host/runtime hints when execution will be launched by an orchestrator."
}

func projectSetupGuidanceDetail(summary ProjectSetupReadinessSummary, project Project) string {
	parts := make([]string, 0, 4)
	if summary.EnabledContextSourceCount > 0 {
		parts = append(parts, projectSetupCountLabel(summary.EnabledContextSourceCount, "source"))
	}
	if summary.SkillCount > 0 {
		parts = append(parts, projectSetupCountLabel(summary.SkillCount, "skill"))
	}
	if summary.SavedMemoryCount > 0 {
		parts = append(parts, projectSetupCountLabel(summary.SavedMemoryCount, "memory"))
	}
	if summary.PendingMemoryCandidateCount > 0 {
		parts = append(parts, projectSetupCountLabel(summary.PendingMemoryCandidateCount, "candidate"))
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}
	if projectHasActiveRoot(project) {
		return "Discover workspace guidance or add project memory before assigning work."
	}
	return "Add context sources or memory when useful for this project."
}

func projectSetupRoleDetail(summary ProjectSetupReadinessSummary) string {
	if summary.RoleCount > 0 {
		return projectSetupCountLabel(summary.RoleCount, "role")
	}
	return "Create responsibilities such as operator, implementer, reviewer, researcher, or designer."
}

func projectSetupFirstWorkDetail(summary ProjectSetupReadinessSummary) string {
	if summary.WorkItemCount > 0 {
		return projectSetupCountLabel(summary.WorkItemCount, "work item")
	}
	return "Create the first reviewable task after setup."
}

func projectSetupCountLabel(count int, singular string) string {
	if count == 1 {
		return "1 " + singular
	}
	plural := singular + "s"
	if strings.HasSuffix(singular, "y") {
		plural = strings.TrimSuffix(singular, "y") + "ies"
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func projectSetupReadinessAction(kind, projectID, label string) ProjectSetupReadinessAction {
	return ProjectSetupReadinessAction{
		Kind:      kind,
		ProjectID: projectID,
		Label:     label,
	}
}
