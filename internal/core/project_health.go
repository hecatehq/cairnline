package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const projectHealthAttentionLimit = 5

func (s *Service) ProjectHealth(ctx context.Context, projectID string) (ProjectHealth, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ProjectHealth{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	setup, err := s.ProjectSetupReadiness(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	operations, err := s.ProjectOperationsBrief(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	roles, err := s.store.ListRoles(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	profiles, err := s.store.ListAgentProfiles(ctx)
	if err != nil {
		return ProjectHealth{}, err
	}
	skills, err := s.store.ListProjectSkills(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	assignments, err := s.store.ListAssignments(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	workItems, err := s.store.ListWorkItems(ctx, projectID)
	if err != nil {
		return ProjectHealth{}, err
	}
	memoryEntries, err := s.store.ListMemoryEntries(ctx, projectID, true)
	if err != nil {
		return ProjectHealth{}, err
	}
	memoryCandidates, err := s.store.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: projectID, IncludeResolved: true})
	if err != nil {
		return ProjectHealth{}, err
	}
	handoffs, reviews, err := s.projectHealthArtifacts(ctx, projectID, workItems)
	if err != nil {
		return ProjectHealth{}, err
	}

	summary := projectHealthSummary(project, setup, operations, memoryEntries, memoryCandidates, handoffs, reviews)
	attention := make([]ProjectHealthAttentionItem, 0, len(operations.Items)+4)
	if setup.ShowOnboarding {
		attention = append(attention, ProjectHealthAttentionItem{
			ID:          healthItemID(project.ID, "setup"),
			ProjectID:   project.ID,
			Kind:        ProjectOperationKindProjectSetup,
			Severity:    ProjectOperationSeverityAction,
			Status:      ProjectSetupStatusTodo,
			Title:       "Set up project",
			Detail:      "Add project context, roles, and the first work item before coordinating execution.",
			ActionKind:  ProjectSetupActionSetupProject,
			ActionLabel: "Set up project",
			UpdatedAt:   project.UpdatedAt,
		})
	}
	for _, item := range operations.Items {
		attention = append(attention, projectOperationHealthAttention(project.ID, item))
	}
	missingProfiles := missingProjectHealthProfileReferences(roles, assignments, profiles)
	summary.MissingProfileReferenceCount = len(missingProfiles)
	if len(missingProfiles) > 0 {
		attention = append(attention, ProjectHealthAttentionItem{
			ID:          healthItemID(project.ID, "profiles", "missing"),
			ProjectID:   project.ID,
			Kind:        ProjectOperationKindProfile,
			Severity:    ProjectOperationSeverityBlocked,
			Status:      "missing",
			Title:       "Agent profile reference missing",
			Detail:      "Role or assignment defaults reference " + summarizeIDs(missingProfiles) + ".",
			ActionKind:  "update_profiles",
			ActionLabel: "Review profiles",
		})
	}
	skillIssues := projectHealthSkillIssues(roles, assignments, profiles, skills)
	summary.ProjectSkillIssueCount = len(skillIssues)
	if len(skillIssues) > 0 {
		attention = append(attention, ProjectHealthAttentionItem{
			ID:          healthItemID(project.ID, "skills"),
			ProjectID:   project.ID,
			Kind:        ProjectOperationKindSkill,
			Severity:    ProjectOperationSeverityAction,
			Status:      "needs_review",
			Title:       "Project skills need review",
			Detail:      strings.Join(skillIssues, "; ") + ".",
			ActionKind:  "update_skills",
			ActionLabel: "Review skills",
		})
	}
	if !setup.ShowOnboarding && summary.EnabledMemoryCount == 0 && summary.EnabledContextSourceCount == 0 {
		attention = append(attention, ProjectHealthAttentionItem{
			ID:          healthItemID(project.ID, "context"),
			ProjectID:   project.ID,
			Kind:        ProjectOperationKindProjectSetup,
			Severity:    ProjectOperationSeverityAction,
			Status:      ProjectSetupStatusTodo,
			Title:       "No project memory or context sources enabled",
			Detail:      "Project-scoped context is empty for future assignment context packets.",
			ActionKind:  ProjectSetupActionManageContext,
			ActionLabel: "Add context",
		})
	}

	attention = uniqueProjectHealthAttention(attention)
	availableAttentionCount := len(attention)
	if len(attention) > projectHealthAttentionLimit {
		attention = append([]ProjectHealthAttentionItem(nil), attention[:projectHealthAttentionLimit]...)
	}
	summary.AttentionCount = len(attention)
	summary.AvailableAttentionCount = availableAttentionCount
	summary.OmittedAttentionCount = availableAttentionCount - len(attention)
	summary.AttentionLimit = projectHealthAttentionLimit

	status := ProjectHealthStatusClear
	title := "Project is clear"
	detail := "No project coordination items need attention."
	if availableAttentionCount > 0 {
		status = ProjectHealthStatusAttention
		title = "Project needs attention"
		detail = fmt.Sprintf("%d project coordination item%s need%s operator attention.", availableAttentionCount, pluralSuffix(availableAttentionCount), pluralVerb(availableAttentionCount))
	}
	return ProjectHealth{
		ProjectID: project.ID,
		Status:    status,
		Title:     title,
		Detail:    detail,
		Summary:   summary,
		Attention: attention,
		CreatedAt: s.now(),
	}, nil
}

func (s *Service) projectHealthArtifacts(ctx context.Context, projectID string, workItems []WorkItem) ([]Handoff, []Review, error) {
	handoffs := make([]Handoff, 0)
	reviews := make([]Review, 0)
	for _, workItem := range workItems {
		workHandoffs, err := s.store.ListHandoffs(ctx, projectID, workItem.ID)
		if err != nil {
			return nil, nil, err
		}
		handoffs = append(handoffs, workHandoffs...)
		workReviews, err := s.store.ListReviews(ctx, projectID, workItem.ID)
		if err != nil {
			return nil, nil, err
		}
		reviews = append(reviews, workReviews...)
	}
	return handoffs, reviews, nil
}

func projectHealthSummary(project Project, setup ProjectSetupReadiness, operations ProjectOperationsBrief, entries []MemoryEntry, candidates []MemoryCandidate, handoffs []Handoff, reviews []Review) ProjectHealthSummary {
	summary := ProjectHealthSummary{
		MissingProjectRoot:     !projectHasActiveRoot(project),
		HasExecutionProfile:    setup.Summary.HasExecutionProfile,
		ActiveAssignmentCount:  operations.Counts.ActiveAssignments,
		BlockedAssignmentCount: operations.Counts.BlockedAssignments,
	}
	for _, check := range setup.Checks {
		if check.Status == ProjectSetupStatusTodo {
			summary.SetupTodoCount++
		}
	}
	for _, entry := range entries {
		summary.SavedMemoryCount++
		if entry.Enabled {
			summary.EnabledMemoryCount++
		}
	}
	for _, source := range project.ContextSources {
		if source.Enabled {
			summary.EnabledContextSourceCount++
		}
	}
	for _, candidate := range candidates {
		switch candidate.Status {
		case MemoryCandidatePending:
			summary.PendingMemoryCandidateCount++
		case MemoryCandidatePromoted:
			summary.PromotedMemoryCandidateCount++
		case MemoryCandidateRejected:
			summary.RejectedMemoryCandidateCount++
		}
	}
	for _, handoff := range handoffs {
		switch handoff.Status {
		case HandoffStatusOpen:
			summary.OpenHandoffCount++
		case HandoffStatusAccepted:
			summary.AcceptedHandoffCount++
		case HandoffStatusSuperseded:
			summary.SupersededHandoffCount++
		case HandoffStatusDismissed:
			summary.DismissedHandoffCount++
		}
	}
	for _, review := range reviews {
		if reviewRequiresFollowUp(review) {
			summary.ReviewFollowUpCount++
		}
		switch review.Verdict {
		case ReviewVerdictBlocked:
			summary.BlockedReviewCount++
		case ReviewVerdictConcerns:
			summary.ChangesRequestedReviewCount++
		}
	}
	return summary
}

func projectOperationHealthAttention(projectID string, item ProjectOperationItem) ProjectHealthAttentionItem {
	actionKind, actionLabel := projectOperationHealthAction(item)
	attention := ProjectHealthAttentionItem{
		ID:                healthItemID(projectID, item.Kind, item.WorkItemID, item.AssignmentID, item.ArtifactID, item.MemoryCandidateID),
		ProjectID:         projectID,
		Kind:              item.Kind,
		Severity:          item.Severity,
		Status:            item.Status,
		Title:             item.Title,
		Detail:            item.Detail,
		ActionKind:        actionKind,
		ActionLabel:       actionLabel,
		WorkItemID:        item.WorkItemID,
		AssignmentID:      item.AssignmentID,
		ArtifactID:        item.ArtifactID,
		MemoryCandidateID: item.MemoryCandidateID,
		UpdatedAt:         item.UpdatedAt,
	}
	if item.Kind == ProjectOperationKindHandoff {
		attention.HandoffID = item.ArtifactID
		attention.ArtifactID = ""
	}
	return attention
}

func projectOperationHealthAction(item ProjectOperationItem) (string, string) {
	switch item.Kind {
	case ProjectOperationKindAssignment:
		switch item.Status {
		case AssignmentQueued:
			return "claim_assignment", "Start assignment"
		case AssignmentClaimed, AssignmentRunning, AssignmentReview:
			return "inspect_assignment", "Inspect assignment"
		default:
			return "resolve_assignment", "Resolve assignment"
		}
	case ProjectOperationKindMissingEvidence:
		return "record_evidence", "Record evidence"
	case ProjectOperationKindReviewFollowUp:
		return "create_handoff", "Create follow-up"
	case ProjectOperationKindHandoff:
		return "resolve_handoff", "Resolve handoff"
	case ProjectOperationKindMemoryCandidate:
		return "review_memory_candidate", "Review memory"
	case ProjectOperationKindCloseoutReady:
		return "closeout_work_item", "Close out work"
	case ProjectOperationKindWorkItem:
		return "create_assignment", "Create assignment"
	default:
		return "inspect_project", "Inspect"
	}
}

func missingProjectHealthProfileReferences(roles []Role, assignments []Assignment, profiles []AgentProfile) []string {
	profileIDs := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if id := strings.TrimSpace(profile.ID); id != "" {
			profileIDs[id] = struct{}{}
		}
	}
	missing := make([]string, 0)
	addMissing := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := profileIDs[id]; ok {
			return
		}
		missing = appendUnique(missing, id)
	}
	for _, role := range roles {
		addMissing(role.DefaultProfileID)
	}
	for _, assignment := range assignments {
		addMissing(assignment.ProfileID)
	}
	sort.Strings(missing)
	return missing
}

func projectHealthSkillIssues(roles []Role, assignments []Assignment, profiles []AgentProfile, skills []ProjectSkill) []string {
	skillsByID := make(map[string]ProjectSkill, len(skills))
	for _, skill := range skills {
		if id := normalizeSkillID(skill.ID); id != "" {
			skillsByID[id] = skill
		}
	}
	referenced := referencedProjectHealthSkillIDs(roles, assignments, profiles)
	unresolved := make([]string, 0)
	disabled := make([]string, 0)
	unavailable := make([]string, 0)
	for _, skillID := range referenced {
		skill, ok := skillsByID[skillID]
		if !ok {
			unresolved = append(unresolved, skillID)
			continue
		}
		if !skill.Enabled {
			disabled = append(disabled, skillID)
		}
		if skill.Status != SkillStatusAvailable {
			unavailable = append(unavailable, skillID)
		}
	}
	issues := make([]string, 0, 3)
	if len(unresolved) > 0 {
		issues = append(issues, "unresolved: "+summarizeIDs(unresolved))
	}
	if len(disabled) > 0 {
		issues = append(issues, "disabled: "+summarizeIDs(disabled))
	}
	if len(unavailable) > 0 {
		issues = append(issues, "unavailable: "+summarizeIDs(unavailable))
	}
	return issues
}

func referencedProjectHealthSkillIDs(roles []Role, assignments []Assignment, profiles []AgentProfile) []string {
	referenced := make([]string, 0)
	relevantProfileIDs := make(map[string]struct{})
	addSkill := func(id string) {
		id = normalizeSkillID(id)
		if id != "" {
			referenced = appendUnique(referenced, id)
		}
	}
	for _, role := range roles {
		for _, skillID := range role.DefaultSkillIDs {
			addSkill(skillID)
		}
		if id := strings.TrimSpace(role.DefaultProfileID); id != "" {
			relevantProfileIDs[id] = struct{}{}
		}
	}
	for _, assignment := range assignments {
		for _, skillID := range assignment.DesiredAgent.SkillIDs {
			addSkill(skillID)
		}
		if id := strings.TrimSpace(assignment.ProfileID); id != "" {
			relevantProfileIDs[id] = struct{}{}
		}
	}
	for _, profile := range profiles {
		if _, ok := relevantProfileIDs[strings.TrimSpace(profile.ID)]; !ok {
			continue
		}
		for _, skillID := range profile.SkillIDs {
			addSkill(skillID)
		}
	}
	sort.Strings(referenced)
	return referenced
}

func uniqueProjectHealthAttention(items []ProjectHealthAttentionItem) []ProjectHealthAttentionItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]ProjectHealthAttentionItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func summarizeIDs(ids []string) string {
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			unique = appendUnique(unique, id)
		}
	}
	sort.Strings(unique)
	shown := unique
	if len(shown) > 3 {
		shown = shown[:3]
	}
	if len(unique) <= 3 {
		return strings.Join(shown, ", ")
	}
	return strings.Join(shown, ", ") + ", and " + fmt.Sprintf("%d", len(unique)-3) + " more"
}

func healthItemID(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, ":")
}
