package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestService_CreateRootlessProjectAndWorkItem(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{
		Name:                      "Research notes",
		Description:               "Coordinate interview synthesis.",
		DefaultProfileID:          " profile_research ",
		DefaultExecutionProfileID: " exec_local ",
		ContextSources: []Source{{
			ID:             " src_agents ",
			Kind:           " workspace_instruction ",
			Title:          " AGENTS.md ",
			Locator:        " AGENTS.md ",
			Enabled:        true,
			Format:         " agents_md ",
			Scope:          " workspace ",
			TrustLabel:     " workspace_guidance ",
			SourceCategory: " instructions ",
			Metadata: map[string]string{
				" root_id ": " root_main ",
				"":          "ignored",
			},
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.ID == "" || len(project.Roots) != 0 {
		t.Fatalf("project = %+v, want generated id and no roots", project)
	}
	if project.DefaultProfileID != "profile_research" || project.DefaultExecutionProfileID != "exec_local" {
		t.Fatalf("project defaults = %q/%q, want trimmed profile and execution profile defaults", project.DefaultProfileID, project.DefaultExecutionProfileID)
	}
	if len(project.ContextSources) != 1 {
		t.Fatalf("context sources = %+v, want one normalized source", project.ContextSources)
	}
	source := project.ContextSources[0]
	if source.ID != "src_agents" || source.Kind != "workspace_instruction" || source.Title != "AGENTS.md" || source.Locator != "AGENTS.md" {
		t.Fatalf("source identity = %+v, want normalized source fields", source)
	}
	if !source.Enabled || source.Format != "agents_md" || source.Scope != "workspace" || source.TrustLabel != "workspace_guidance" || source.SourceCategory != "instructions" {
		t.Fatalf("source metadata = %+v, want normalized metadata fields", source)
	}
	if source.Metadata["root_id"] != "root_main" || len(source.Metadata) != 1 {
		t.Fatalf("source metadata map = %+v, want trimmed root_id only", source.Metadata)
	}
	if source.CreatedAt.IsZero() || source.UpdatedAt.IsZero() {
		t.Fatalf("source timestamps = %+v, want set timestamps", source)
	}

	updatedProject, err := service.UpdateProject(ctx, Project{
		ID:                        project.ID,
		Name:                      project.Name,
		Description:               project.Description,
		DefaultProfileID:          "profile_synthesis",
		DefaultExecutionProfileID: "exec_review",
		ContextSources: []Source{{
			ID:             "src_agents",
			Kind:           "workspace_instruction",
			Title:          "Repository guidance",
			Locator:        "AGENTS.md",
			Enabled:        false,
			Format:         "agents_md",
			Scope:          "workspace",
			TrustLabel:     "workspace_guidance",
			SourceCategory: "instructions",
			Metadata:       map[string]string{"root_id": "root_main", "source": "manual"},
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	updatedSource := updatedProject.ContextSources[0]
	if updatedSource.CreatedAt.IsZero() || !updatedSource.CreatedAt.Equal(source.CreatedAt) || updatedSource.Title != "Repository guidance" || updatedSource.Enabled {
		t.Fatalf("updated source = %+v, want preserved created_at and replacement metadata", updatedSource)
	}
	if updatedSource.Metadata["source"] != "manual" || updatedSource.Metadata["root_id"] != "root_main" {
		t.Fatalf("updated source metadata = %+v, want replacement metadata", updatedSource.Metadata)
	}
	if updatedProject.DefaultProfileID != "profile_synthesis" || updatedProject.DefaultExecutionProfileID != "exec_review" {
		t.Fatalf("updated defaults = %q/%q, want updated profile and execution profile defaults", updatedProject.DefaultProfileID, updatedProject.DefaultExecutionProfileID)
	}

	item, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Summarize interview themes",
		Brief:     "Turn notes into a reviewable theme summary.",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	if item.OwnerRoleID != "" || item.Status != WorkStatusReady || item.Priority != PriorityNormal {
		t.Fatalf("work item = %+v, want ownerless ready normal item", item)
	}

	items, err := service.ListWorkItems(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListWorkItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("items = %+v, want created item", items)
	}
}

func TestService_DeleteProjectCascadesProjectScopedState(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{Name: "Global profile"})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	execution, err := service.CreateExecutionProfile(ctx, ExecutionProfile{Name: "Global execution"})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, Project{Name: "Delete me"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if got, err := service.GetProject(ctx, project.ID); err != nil || got.ID != project.ID {
		t.Fatalf("GetProject() = %+v, %v; want created project", got, err)
	}
	skill, err := service.CreateProjectSkill(ctx, ProjectSkill{
		ProjectID:  project.ID,
		ID:         "review",
		Title:      "Review",
		Format:     SkillFormatMarkdown,
		Status:     SkillStatusAvailable,
		TrustLabel: SkillTrustWorkspace,
	})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Reviewer", DefaultProfileID: profile.ID})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Delete scoped rows"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		RoleID:             role.ID,
		ExecutionProfileID: execution.ID,
		DesiredAgent:       DesiredAgent{Kind: DesiredAgentAny, SkillIDs: []string{skill.ID}},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Title: "Evidence", Locator: "https://example.test/evidence"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Body: "Pass", Verdict: ReviewVerdictApproved}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{ProjectID: project.ID, WorkItemID: work.ID, SourceAssignmentID: assignment.ID, Title: "Handoff", Body: "Follow up"}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := service.CreateMemoryEntry(ctx, MemoryEntry{ProjectID: project.ID, Title: "Accepted memory", Body: "Remember this"}); err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{ProjectID: project.ID, Title: "Candidate", Body: "Review this"}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	if err := service.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := service.GetProject(ctx, project.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProject() after delete error = %v, want ErrNotFound", err)
	}
	for name, check := range map[string]func() error{
		"skills": func() error {
			_, err := service.ListProjectSkills(ctx, project.ID)
			return err
		},
		"roles": func() error {
			_, err := service.ListRoles(ctx, project.ID)
			return err
		},
		"work items": func() error {
			_, err := service.ListWorkItems(ctx, project.ID)
			return err
		},
		"assignments": func() error {
			_, err := service.ListAssignments(ctx, project.ID)
			return err
		},
		"memory entries": func() error {
			_, err := service.ListMemoryEntries(ctx, project.ID, false)
			return err
		},
		"memory candidates": func() error {
			_, err := service.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID})
			return err
		},
	} {
		if err := check(); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s after project delete error = %v, want ErrNotFound", name, err)
		}
	}

	profiles, err := service.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() error = %v", err)
	}
	executionProfiles, err := service.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != profile.ID || len(executionProfiles) != 1 || executionProfiles[0].ID != execution.ID {
		t.Fatalf("global profiles = %+v execution = %+v, want preserved globals", profiles, executionProfiles)
	}
}

func TestService_DeleteWorkItemAndAssignmentScope(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{Name: "Cleanup"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Delete scoped work"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	keepWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Keep this work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(keep) error = %v", err)
	}
	deletedAssignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(deleted) error = %v", err)
	}
	keptAssignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: keepWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(keep) error = %v", err)
	}
	if _, err := service.CreateReview(ctx, Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: deletedAssignment.ID, Body: "Delete with assignment.", Verdict: ReviewVerdictApproved}); err != nil {
		t.Fatalf("CreateReview(deleted assignment) error = %v", err)
	}
	if _, err := service.CreateReview(ctx, Review{ProjectID: project.ID, WorkItemID: keepWork.ID, AssignmentID: keptAssignment.ID, Body: "Keep.", Verdict: ReviewVerdictApproved}); err != nil {
		t.Fatalf("CreateReview(kept assignment) error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{ProjectID: project.ID, WorkItemID: work.ID, Title: "Evidence", Locator: "file://evidence.md"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{ProjectID: project.ID, WorkItemID: work.ID, TargetWorkItemID: keepWork.ID, Title: "Handoff", Body: "Continue elsewhere"}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	memory, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{ProjectID: project.ID, Title: "Keep project memory", Body: "Project-level candidate stays.", SuggestedSourceID: deletedAssignment.ID})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	if err := service.DeleteAssignment(ctx, project.ID, deletedAssignment.ID); err != nil {
		t.Fatalf("DeleteAssignment() error = %v", err)
	}
	if _, err := service.GetAssignment(ctx, project.ID, deletedAssignment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAssignment(deleted) error = %v, want ErrNotFound", err)
	}
	reviews, err := service.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() after assignment delete error = %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("reviews after assignment delete = %+v, want deleted assignment review removed", reviews)
	}

	if err := service.DeleteWorkItem(ctx, project.ID, work.ID); err != nil {
		t.Fatalf("DeleteWorkItem() error = %v", err)
	}
	if _, err := service.GetWorkItem(ctx, project.ID, work.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkItem() after delete error = %v, want ErrNotFound", err)
	}
	assignments, err := service.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() after work delete error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != keptAssignment.ID {
		t.Fatalf("assignments after work delete = %+v, want kept assignment only", assignments)
	}
	candidates, err := service.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() after work delete error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != memory.ID {
		t.Fatalf("memory candidates after work delete = %+v, want project-level memory preserved", candidates)
	}
}

func TestService_HandoffLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{Name: "Handoff flow"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	fromRole, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole(from) error = %v", err)
	}
	toRole, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole(to) error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Ship handoff"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	handoff, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:             project.ID,
		WorkItemID:            work.ID,
		FromRoleID:            fromRole.ID,
		ToRoleID:              toRole.ID,
		Title:                 "Ready for review",
		Body:                  "Implementation is ready.",
		RecommendedNextAction: "Review it",
		LinkedArtifactIDs:     []string{"evidence_1"},
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if handoff.Status != HandoffStatusOpen {
		t.Fatalf("handoff status = %q, want open", handoff.Status)
	}
	if !handoff.StatusChangedAt.Equal(handoff.CreatedAt) {
		t.Fatalf("handoff status_changed_at = %s, want created_at %s", handoff.StatusChangedAt, handoff.CreatedAt)
	}
	got, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff() error = %v", err)
	}
	acceptedAt := time.Date(2026, 6, 3, 12, 30, 0, 0, time.UTC)
	got.Title = "Accepted for review"
	got.Body = "Reviewer accepted the handoff."
	got.Status = HandoffStatusAccepted
	got.LinkedMemoryIDs = []string{"mem_1"}
	got.UpdatedAt = acceptedAt
	updated, err := service.UpdateHandoff(ctx, got)
	if err != nil {
		t.Fatalf("UpdateHandoff() error = %v", err)
	}
	if updated.Status != HandoffStatusAccepted || updated.Title != "Accepted for review" || len(updated.LinkedMemoryIDs) != 1 {
		t.Fatalf("updated handoff = %+v, want accepted replacement metadata", updated)
	}
	if !updated.StatusChangedAt.Equal(acceptedAt) {
		t.Fatalf("updated status_changed_at = %s, want status update time %s", updated.StatusChangedAt, acceptedAt)
	}
	contentEditAt := time.Date(2026, 6, 3, 12, 35, 0, 0, time.UTC)
	updated.Body = "Review context clarified."
	updated.UpdatedAt = contentEditAt
	contentUpdated, err := service.UpdateHandoff(ctx, updated)
	if err != nil {
		t.Fatalf("UpdateHandoff(content only) error = %v", err)
	}
	if !contentUpdated.StatusChangedAt.Equal(acceptedAt) {
		t.Fatalf("content-only status_changed_at = %s, want preserved %s", contentUpdated.StatusChangedAt, acceptedAt)
	}
	superseded, err := service.UpdateHandoffStatus(ctx, project.ID, work.ID, handoff.ID, HandoffStatusSuperseded)
	if err != nil {
		t.Fatalf("UpdateHandoffStatus() error = %v", err)
	}
	if superseded.Status != HandoffStatusSuperseded || superseded.Title != "Accepted for review" {
		t.Fatalf("status-updated handoff = %+v, want superseded with text preserved", superseded)
	}
	if !superseded.StatusChangedAt.After(acceptedAt) {
		t.Fatalf("superseded status_changed_at = %s, want after %s", superseded.StatusChangedAt, acceptedAt)
	}
	if _, err := service.UpdateHandoffStatus(ctx, project.ID, work.ID, handoff.ID, "unsupported"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateHandoffStatus(unsupported) error = %v, want ErrInvalid", err)
	}
	if err := service.DeleteHandoff(ctx, project.ID, work.ID, handoff.ID); err != nil {
		t.Fatalf("DeleteHandoff() error = %v", err)
	}
	if _, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetHandoff(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestService_HandoffImportPreservesTimestamps(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{Name: "Imported handoff"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Replay handoff"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	createdAt := time.Date(2026, 6, 3, 12, 6, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 3, 12, 7, 0, 0, time.UTC)
	statusChangedAt := time.Date(2026, 6, 3, 12, 6, 30, 0, time.UTC)
	handoff, err := service.CreateHandoff(ctx, Handoff{
		ID:              "handoff_imported",
		ProjectID:       project.ID,
		WorkItemID:      work.ID,
		Title:           "Imported handoff",
		Body:            "Keep source timestamps.",
		Status:          HandoffStatusOpen,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		StatusChangedAt: statusChangedAt,
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if !handoff.CreatedAt.Equal(createdAt) || !handoff.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("handoff timestamps = %s/%s, want %s/%s", handoff.CreatedAt, handoff.UpdatedAt, createdAt, updatedAt)
	}
	if !handoff.StatusChangedAt.Equal(statusChangedAt) {
		t.Fatalf("handoff status_changed_at = %s, want %s", handoff.StatusChangedAt, statusChangedAt)
	}

	nextUpdatedAt := time.Date(2026, 6, 3, 12, 8, 0, 0, time.UTC)
	handoff.Body = "Updated source summary."
	handoff.UpdatedAt = nextUpdatedAt
	updated, err := service.UpdateHandoff(ctx, handoff)
	if err != nil {
		t.Fatalf("UpdateHandoff() error = %v", err)
	}
	if !updated.CreatedAt.Equal(createdAt) || !updated.UpdatedAt.Equal(nextUpdatedAt) {
		t.Fatalf("updated handoff timestamps = %s/%s, want %s/%s", updated.CreatedAt, updated.UpdatedAt, createdAt, nextUpdatedAt)
	}
	if !updated.StatusChangedAt.Equal(statusChangedAt) {
		t.Fatalf("updated handoff status_changed_at = %s, want preserved %s", updated.StatusChangedAt, statusChangedAt)
	}

	importedStatusChangedAt := time.Date(2026, 6, 3, 12, 8, 30, 0, time.UTC)
	updated.Status = HandoffStatusAccepted
	updated.UpdatedAt = nextUpdatedAt.Add(time.Minute)
	updated.StatusChangedAt = importedStatusChangedAt
	statusUpdated, err := service.UpdateHandoff(ctx, updated)
	if err != nil {
		t.Fatalf("UpdateHandoff(imported status change) error = %v", err)
	}
	if !statusUpdated.StatusChangedAt.Equal(importedStatusChangedAt) {
		t.Fatalf("imported status_changed_at = %s, want %s", statusUpdated.StatusChangedAt, importedStatusChangedAt)
	}
}

func TestService_CreateWorkItemValidatesProject(t *testing.T) {
	service := NewService(NewMemoryStore())

	_, err := service.CreateWorkItem(context.Background(), WorkItem{
		ProjectID: "proj_missing",
		Title:     "Do work",
	})
	if err == nil {
		t.Fatal("CreateWorkItem() error = nil, want error")
	}
}

func TestService_ProfileLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{
		Name:          "Implementer",
		Instructions:  "Make focused changes.",
		ContextPolicy: "include_enabled",
		SkillIDs:      []string{"backend", "backend"},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	updatedProfile, err := service.UpdateAgentProfile(ctx, AgentProfile{
		ID:            profile.ID,
		Name:          "Senior implementer",
		Instructions:  "Make focused, tested changes.",
		ContextPolicy: "include_enabled",
		SkillIDs:      []string{"backend", "tests"},
	})
	if err != nil {
		t.Fatalf("UpdateAgentProfile() error = %v", err)
	}
	if updatedProfile.CreatedAt.IsZero() || !updatedProfile.UpdatedAt.After(updatedProfile.CreatedAt) && !updatedProfile.UpdatedAt.Equal(updatedProfile.CreatedAt) {
		t.Fatalf("updated profile timestamps = created %s updated %s", updatedProfile.CreatedAt, updatedProfile.UpdatedAt)
	}
	if updatedProfile.Name != "Senior implementer" || len(updatedProfile.SkillIDs) != 2 {
		t.Fatalf("updated profile = %+v, want replacement values", updatedProfile)
	}

	execution, err := service.CreateExecutionProfile(ctx, ExecutionProfile{
		Name:           "Local execution",
		AgentKind:      "any",
		ModelHint:      "local",
		ProviderHint:   "local",
		AdapterOptions: map[string]any{"tier": "dev"},
	})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	updatedExecution, err := service.UpdateExecutionProfile(ctx, ExecutionProfile{
		ID:             execution.ID,
		Name:           "Cloud execution",
		AgentKind:      "any",
		ModelHint:      "frontier",
		ProviderHint:   "cloud",
		ApprovalPolicy: "require",
	})
	if err != nil {
		t.Fatalf("UpdateExecutionProfile() error = %v", err)
	}
	if updatedExecution.Name != "Cloud execution" || updatedExecution.ProviderHint != "cloud" {
		t.Fatalf("updated execution profile = %+v, want replacement values", updatedExecution)
	}

	if err := service.DeleteAgentProfile(ctx, updatedProfile.ID); err != nil {
		t.Fatalf("DeleteAgentProfile() error = %v", err)
	}
	profiles, err := service.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() after delete error = %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles after delete = %+v, want none", profiles)
	}
	if err := service.DeleteAgentProfile(ctx, updatedProfile.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteAgentProfile(deleted) error = %v, want ErrNotFound", err)
	}

	if err := service.DeleteExecutionProfile(ctx, updatedExecution.ID); err != nil {
		t.Fatalf("DeleteExecutionProfile() error = %v", err)
	}
	executionProfiles, err := service.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() after delete error = %v", err)
	}
	if len(executionProfiles) != 0 {
		t.Fatalf("execution profiles after delete = %+v, want none", executionProfiles)
	}
	if err := service.DeleteExecutionProfile(ctx, updatedExecution.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteExecutionProfile(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestService_DeleteRoleAllowsHistoricalAssignments(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{Name: "Role cleanup"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	spareRole, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Spare reviewer"})
	if err != nil {
		t.Fatalf("CreateRole(spare) error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Keep historical assignments"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	if err := service.DeleteRole(ctx, project.ID, spareRole.ID); err != nil {
		t.Fatalf("DeleteRole(spare) error = %v", err)
	}
	roles, err := service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() after spare delete error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != role.ID {
		t.Fatalf("roles after spare delete = %+v, want referenced role only", roles)
	}

	if err := service.DeleteRole(ctx, project.ID, role.ID); err != nil {
		t.Fatalf("DeleteRole(referenced) error = %v", err)
	}
	roles, err = service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() after referenced delete error = %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("roles after referenced delete = %+v, want none", roles)
	}
	context, err := service.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() after role delete error = %v", err)
	}
	if context.Assignment.RoleID != role.ID || context.Role != nil || !containsString(context.Warnings, "assignment role was not found") {
		t.Fatalf("assignment context after role delete = %+v, want durable assignment with missing-role warning", context)
	}
	if err := service.DeleteRole(ctx, project.ID, role.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteRole(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestService_ProjectSkillsDiscoveryAndLaunchResolution(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, SkillPathAgents, "backend", `---
name: Backend implementer
description: Work on backend code with tests.
hecate:
  suggested_tools:
    - git.diff
    - file.read
  required_permissions:
    tools: true
    writes: false
    network: false
---
# Backend skill

Body should not be stored in the skill registry.
`)
	writeSkill(t, root, SkillPathHecate, "qa", "# QA reviewer\n")
	writeSkill(t, root, SkillPathCairnline, "planning", "# Planning\n")

	project, err := service.CreateProject(ctx, Project{
		Name: "Skill project",
		Roots: []Root{{
			ID:     "root_main",
			Path:   root,
			Kind:   "workspace",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skills, err := service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() error = %v", err)
	}
	if len(skills) != 3 {
		t.Fatalf("skills = %+v, want three discovered skills", skills)
	}
	skill := findProjectSkillForTest(skills, "backend")
	if skill == nil {
		t.Fatalf("skills = %+v, missing backend skill", skills)
	}
	if skill.ID != "backend" || skill.Title != "Backend implementer" || skill.Description != "Work on backend code with tests." || skill.Path != ".agents/skills/backend/SKILL.md" {
		t.Fatalf("skill = %+v, want parsed metadata and relative path", skill)
	}
	if strings.Join(skill.SuggestedTools, ",") != "file.read,git.diff" {
		t.Fatalf("suggested tools = %+v, want sorted parsed capability hints", skill.SuggestedTools)
	}
	if skill.RequiredPermissions.Tools == nil || !*skill.RequiredPermissions.Tools || skill.RequiredPermissions.Writes == nil || *skill.RequiredPermissions.Writes || skill.RequiredPermissions.Network == nil || *skill.RequiredPermissions.Network {
		t.Fatalf("required permissions = %+v, want parsed nullable booleans", skill.RequiredPermissions)
	}
	if strings.Contains(skill.Description, "Body should not be stored") {
		t.Fatalf("skill description includes body content: %+v", skill)
	}
	qa := findProjectSkillForTest(skills, "qa")
	if qa == nil || qa.Path != ".hecate/skills/qa/SKILL.md" || qa.Title != "QA reviewer" {
		t.Fatalf("qa skill = %+v, want Hecate-compatible discovered skill", qa)
	}
	planning := findProjectSkillForTest(skills, "planning")
	if planning == nil || planning.Path != ".cairnline/skills/planning/SKILL.md" || planning.Title != "Planning" {
		t.Fatalf("planning skill = %+v, want Cairnline-native discovered skill", planning)
	}

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{
		Name:     "Implementer profile",
		SkillIDs: []string{"backend", "missing"},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ProjectID:        project.ID,
		Name:             "Implementer",
		DefaultProfileID: profile.ID,
		DefaultSkillIDs:  []string{"backend"},
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Use skill metadata",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
		DesiredAgent: DesiredAgent{
			Kind:     DesiredAgentAny,
			SkillIDs: []string{"backend"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	packet, err := service.AssignmentLaunchPacket(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentLaunchPacket() error = %v", err)
	}
	if len(packet.Skills) != 1 || packet.Skills[0].ID != "backend" {
		t.Fatalf("launch packet skills = %+v, want resolved backend skill", packet.Skills)
	}
	if !containsString(packet.Warnings, "skill was not found: missing") {
		t.Fatalf("launch packet warnings = %+v, want missing skill warning", packet.Warnings)
	}

	skill.Enabled = false
	if _, err := service.UpdateProjectSkill(ctx, *skill); err != nil {
		t.Fatalf("UpdateProjectSkill() error = %v", err)
	}
	packet, err = service.AssignmentLaunchPacket(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentLaunchPacket() after disable error = %v", err)
	}
	if len(packet.Skills) != 0 || !containsString(packet.Warnings, "skill is disabled: backend") {
		t.Fatalf("launch packet after disabled skill = skills %+v warnings %+v, want disabled warning", packet.Skills, packet.Warnings)
	}
}

func TestService_CreateProjectSkillPreservesImportedTimestamps(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	service.now = func() time.Time {
		return time.Date(2026, 6, 27, 20, 0, 0, 0, time.UTC)
	}
	project, err := service.CreateProject(ctx, Project{Name: "Imported skill project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	discoveredAt := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 22, 11, 0, 0, 0, time.UTC)

	skill, err := service.CreateProjectSkill(ctx, ProjectSkill{
		ProjectID:    project.ID,
		ID:           "backend",
		Title:        "Backend",
		Path:         ".agents/skills/backend/SKILL.md",
		Format:       SkillFormatMarkdown,
		Status:       SkillStatusAvailable,
		TrustLabel:   SkillTrustWorkspace,
		DiscoveredAt: discoveredAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	if !skill.DiscoveredAt.Equal(discoveredAt) || !skill.CreatedAt.Equal(createdAt) || !skill.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("skill timestamps = discovered:%s created:%s updated:%s, want imported timestamps", skill.DiscoveredAt, skill.CreatedAt, skill.UpdatedAt)
	}
}

func TestService_ProjectSkillsDiscoveryUsesGuidanceLinkedRoots(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, "docs-ai/skills", "research", "# Researcher\n")
	writeSkill(t, root, "docs/guidance/local-skills", "release", "# Release Manager\n")
	writeSkill(t, root, ".worktrees/refactor/docs-ai/skills", "worktree", "# Worktree Skill\n")
	writeSkill(t, root, "ignored/skills", "disabled", "# Disabled\n")
	writeProjectTestFile(t, root, "AGENTS.md", "Use @docs-ai/skills/research/SKILL.md and `.worktrees/refactor/docs-ai/skills/worktree/SKILL.md`.\n")
	writeProjectTestFile(t, root, "docs/guidance/CLAUDE.md", "Use local-skills/release/SKILL.md and https://example.com/skills/cloud/SKILL.md.\n")
	writeProjectTestFile(t, root, "OTHER.md", "Use ignored/skills/wrong-root/SKILL.md.\n")
	writeProjectTestFile(t, root, "DISABLED.md", "Use ignored/skills/disabled/SKILL.md.\n")

	project, err := service.CreateProject(ctx, Project{
		Name: "Guidance skills",
		Roots: []Root{{
			ID:     "root_main",
			Path:   root,
			Kind:   "workspace",
			Active: true,
		}},
		ContextSources: []Source{
			{
				ID:      "src_agents",
				Kind:    "workspace_instruction",
				Title:   "AGENTS.md",
				Locator: "AGENTS.md",
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
			{
				ID:      "src_claude",
				Title:   "CLAUDE.md",
				Locator: "docs/guidance/CLAUDE.md",
				Enabled: true,
				Format:  "claude_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
			{
				ID:      "src_wrong_root",
				Kind:    "workspace_instruction",
				Title:   "OTHER.md",
				Locator: "OTHER.md",
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "other_root",
				},
			},
			{
				ID:      "src_disabled",
				Kind:    "workspace_instruction",
				Title:   "DISABLED.md",
				Locator: "DISABLED.md",
				Enabled: false,
				Format:  "agents_md",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skills, err := service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() error = %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("skills = %+v, want two guidance-linked skills", skills)
	}
	research := findProjectSkillForTest(skills, "research")
	if research == nil || research.Path != "docs-ai/skills/research/SKILL.md" || !containsString(research.SourceRefs, "src_agents") {
		t.Fatalf("research skill = %+v, want AGENTS.md-linked skill with source ref", research)
	}
	release := findProjectSkillForTest(skills, "release")
	if release == nil || release.Path != "docs/guidance/local-skills/release/SKILL.md" || !containsString(release.SourceRefs, "src_claude") {
		t.Fatalf("release skill = %+v, want CLAUDE.md-linked skill with source ref", release)
	}
	if skipped := findProjectSkillForTest(skills, "worktree"); skipped != nil {
		t.Fatalf("worktree skill = %+v, want nested worktree link skipped", skipped)
	}
	if disabled := findProjectSkillForTest(skills, "disabled"); disabled != nil {
		t.Fatalf("disabled-source skill = %+v, want disabled guidance source skipped", disabled)
	}
}

func TestService_ProjectSkillsDiscoveryUsesCommonAgentGuidanceSources(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, "skills/gemini", "research", "# Research\n")
	writeSkill(t, root, ".cursor/rules/project-skills", "frontend", "# Frontend\n")
	writeSkill(t, root, ".github/instructions/local-skills", "review", "# Review\n")
	writeSkill(t, root, ".devin/rules/local-skills", "ops", "# Ops\n")
	writeSkill(t, root, ".windsurf/rules/local-skills", "design", "# Design\n")
	writeProjectTestFile(t, root, "GEMINI.md", "Use skills/gemini/research/SKILL.md.\n")
	writeProjectTestFile(t, root, ".cursor/rules/project.mdc", "Use project-skills/frontend/SKILL.md.\n")
	writeProjectTestFile(t, root, ".github/instructions/project.instructions.md", "Use local-skills/review/SKILL.md.\n")
	writeProjectTestFile(t, root, ".devin/rules/project.md", "Use local-skills/ops/SKILL.md.\n")
	writeProjectTestFile(t, root, ".windsurf/rules/project.md", "Use local-skills/design/SKILL.md.\n")

	project, err := service.CreateProject(ctx, Project{
		Name: "Common agent guidance",
		Roots: []Root{{
			ID:     "root_main",
			Path:   root,
			Active: true,
		}},
		ContextSources: []Source{
			{ID: "src_gemini", Title: "GEMINI.md", Locator: "GEMINI.md", Enabled: true, Format: "gemini_md", Metadata: map[string]string{"root_id": "root_main"}},
			{ID: "src_cursor", Title: "Cursor rule", Locator: ".cursor/rules/project.mdc", Enabled: true},
			{ID: "src_github", Title: "GitHub instruction", Locator: ".github/instructions/project.instructions.md", Enabled: true},
			{ID: "src_devin", Title: "Devin rule", Locator: ".devin/rules/project.md", Enabled: true},
			{ID: "src_windsurf", Title: "Windsurf rule", Locator: ".windsurf/rules/project.md", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skills, err := service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() error = %v", err)
	}
	assertSkillSource := func(id, sourceRef string) {
		t.Helper()
		skill := findProjectSkillForTest(skills, id)
		if skill == nil {
			t.Fatalf("skills = %+v, missing %s", skills, id)
		}
		if !containsString(skill.SourceRefs, sourceRef) {
			t.Fatalf("%s source refs = %+v, want %s", id, skill.SourceRefs, sourceRef)
		}
	}
	assertSkillSource("research", "src_gemini")
	assertSkillSource("frontend", "src_cursor")
	assertSkillSource("review", "src_github")
	assertSkillSource("ops", "src_devin")
	assertSkillSource("design", "src_windsurf")
}

func TestService_ProjectSkillsDiscoveryRejectsUnsafeGuidancePaths(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, "docs-ai/skills", "safe", "# Safe Skill\n")
	writeSkill(t, root, ".worktrees/refactor/docs-ai/skills", "worktree", "# Worktree Skill\n")
	writeSkill(t, root, ".claude/worktrees/refactor/docs-ai/skills", "claude-worktree", "# Claude Worktree Skill\n")
	writeSkill(t, root, "absolute-source/skills", "absolute-source", "# Absolute Source Skill\n")
	writeProjectTestFile(t, root, "AGENTS.md", strings.Join([]string{
		"Use docs-ai/skills/safe/SKILL.md.",
		"Ignore ../outside/skills/escape/SKILL.md.",
		"Ignore /tmp/outside/skills/absolute/SKILL.md.",
		"Ignore https://example.com/skills/cloud/SKILL.md.",
		"Ignore .worktrees/refactor/docs-ai/skills/worktree/SKILL.md.",
		"Ignore .claude/worktrees/refactor/docs-ai/skills/claude-worktree/SKILL.md.",
	}, "\n"))
	writeProjectTestFile(t, root, "ABSOLUTE.md", "Use absolute-source/skills/absolute-source/SKILL.md.\n")
	writeProjectTestFile(t, root, ".worktrees/refactor/AGENTS.md", "Use docs-ai/skills/worktree-source/SKILL.md.\n")

	project, err := service.CreateProject(ctx, Project{
		Name: "Unsafe guidance paths",
		Roots: []Root{{
			ID:     "root_main",
			Path:   root,
			Kind:   "workspace",
			Active: true,
		}},
		ContextSources: []Source{
			{
				ID:      "src_agents",
				Kind:    "workspace_instruction",
				Title:   "AGENTS.md",
				Locator: "AGENTS.md",
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
			{
				ID:      "src_absolute",
				Kind:    "workspace_instruction",
				Title:   "Absolute guidance",
				Locator: filepath.Join(root, "ABSOLUTE.md"),
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
			{
				ID:      "src_parent",
				Kind:    "workspace_instruction",
				Title:   "Parent guidance",
				Locator: "../outside/AGENTS.md",
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
			{
				ID:      "src_url",
				Kind:    "workspace_instruction",
				Title:   "Remote guidance",
				Locator: "https://example.com/AGENTS.md",
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
			{
				ID:      "src_worktree",
				Kind:    "workspace_instruction",
				Title:   "Worktree guidance",
				Locator: ".worktrees/refactor/AGENTS.md",
				Enabled: true,
				Format:  "agents_md",
				Metadata: map[string]string{
					"root_id": "root_main",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skills, err := service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("skills = %+v, want only the safe guidance-linked skill", skills)
	}
	if safe := findProjectSkillForTest(skills, "safe"); safe == nil || safe.Path != "docs-ai/skills/safe/SKILL.md" || !containsString(safe.SourceRefs, "src_agents") {
		t.Fatalf("safe skill = %+v, want AGENTS.md-linked skill with source ref", safe)
	}
	for _, id := range []string{"worktree", "claude-worktree", "absolute-source"} {
		if skill := findProjectSkillForTest(skills, id); skill != nil {
			t.Fatalf("unsafe skill %q = %+v, want skipped", id, skill)
		}
	}
}

func TestService_ProjectSkillsRediscoveryPreservesOperatorOverridesAndRefreshesSourceRefs(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, "docs-ai/skills", "backend", `---
name: Backend implementer
description: Original description.
hecate:
  suggested_tools: [git.diff]
  required_permissions:
    writes: false
---
# Backend
`)
	writeProjectTestFile(t, root, "AGENTS.md", "Use docs-ai/skills/backend/SKILL.md.\n")
	writeProjectTestFile(t, root, "CLAUDE.md", "Use docs-ai/skills/backend/SKILL.md.\n")

	project, err := service.CreateProject(ctx, Project{
		Name: "Rediscover skills",
		Roots: []Root{{
			ID:     "root_main",
			Path:   root,
			Kind:   "workspace",
			Active: true,
		}},
		ContextSources: []Source{{
			ID:      "src_agents",
			Kind:    "workspace_instruction",
			Title:   "AGENTS.md",
			Locator: "AGENTS.md",
			Enabled: true,
			Format:  "agents_md",
			Metadata: map[string]string{
				"root_id": "root_main",
			},
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skills, err := service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() error = %v", err)
	}
	backend := findProjectSkillForTest(skills, "backend")
	if backend == nil {
		t.Fatalf("skills = %+v, missing backend skill", skills)
	}
	if !containsString(backend.SourceRefs, "src_agents") {
		t.Fatalf("backend source refs = %+v, want initial AGENTS.md provenance", backend.SourceRefs)
	}
	backend.Enabled = false
	backend.Title = "Operator backend"
	backend.Description = "Operator-curated description."
	backend.TrustLabel = "operator_reviewed"
	if _, err := service.UpdateProjectSkill(ctx, *backend); err != nil {
		t.Fatalf("UpdateProjectSkill() error = %v", err)
	}

	project.ContextSources = []Source{{
		ID:      "src_claude",
		Kind:    "workspace_instruction",
		Title:   "CLAUDE.md",
		Locator: "CLAUDE.md",
		Enabled: true,
		Format:  "claude_md",
		Metadata: map[string]string{
			"root_id": "root_main",
		},
	}}
	if _, err := service.UpdateProject(ctx, project); err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	writeSkill(t, root, "docs-ai/skills", "backend", `---
name: Backend implementer
description: Refreshed description.
cairnline:
  suggested_tools: [file.read, git.diff]
  required_permissions:
    writes: true
---
# Backend
`)
	skills, err = service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() rediscover error = %v", err)
	}
	backend = findProjectSkillForTest(skills, "backend")
	if backend == nil {
		t.Fatalf("skills = %+v, missing backend skill after rediscovery", skills)
	}
	if backend.Enabled || backend.Title != "Operator backend" || backend.Description != "Operator-curated description." || backend.TrustLabel != "operator_reviewed" {
		t.Fatalf("backend after rediscovery = %+v, want operator overrides preserved", backend)
	}
	if !containsString(backend.SourceRefs, "src_claude") || containsString(backend.SourceRefs, "src_agents") {
		t.Fatalf("backend source refs = %+v, want refreshed CLAUDE.md provenance only", backend.SourceRefs)
	}
	if strings.Join(backend.SuggestedTools, ",") != "file.read,git.diff" || backend.RequiredPermissions.Writes == nil || !*backend.RequiredPermissions.Writes {
		t.Fatalf("backend capability hints = %+v / %+v, want refreshed source-derived metadata", backend.SuggestedTools, backend.RequiredPermissions)
	}
}

func TestService_ProjectSkillsDiscoveryMarksConflicts(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, SkillPathAgents, "backend", "# Backend\n")
	writeSkill(t, root, SkillPathHecate, "backend", "# Backend duplicate\n")
	project, err := service.CreateProject(ctx, Project{
		Name: "Conflict project",
		Roots: []Root{{
			ID:     "root_main",
			Path:   root,
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skills, err := service.DiscoverProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("DiscoverProjectSkills() error = %v", err)
	}
	if len(skills) != 1 || skills[0].Status != SkillStatusConflict || len(skills[0].Warnings) == 0 {
		t.Fatalf("skills = %+v, want one conflict record with warning", skills)
	}
}

func TestService_ProjectSkillsMemoryStoreDoesNotAliasCapabilityMetadata(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Skill metadata"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	toolsAllowed := true
	created, err := service.CreateProjectSkill(ctx, ProjectSkill{
		ProjectID:      project.ID,
		ID:             "backend",
		Title:          "Backend",
		Format:         SkillFormatMarkdown,
		SuggestedTools: []string{"git.diff"},
		RequiredPermissions: RequiredPermissions{
			Tools: &toolsAllowed,
		},
		Status: SkillStatusAvailable,
	})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}

	created.SuggestedTools[0] = "shell.exec"
	*created.RequiredPermissions.Tools = false
	got, err := service.GetProjectSkill(ctx, project.ID, "backend")
	if err != nil {
		t.Fatalf("GetProjectSkill() error = %v", err)
	}
	if strings.Join(got.SuggestedTools, ",") != "git.diff" || got.RequiredPermissions.Tools == nil || !*got.RequiredPermissions.Tools {
		t.Fatalf("stored skill after mutating create response = %+v / %+v, want original capability metadata", got.SuggestedTools, got.RequiredPermissions)
	}

	got.SuggestedTools[0] = "file.write"
	*got.RequiredPermissions.Tools = false
	listed, err := service.ListProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListProjectSkills() error = %v", err)
	}
	if len(listed) != 1 || strings.Join(listed[0].SuggestedTools, ",") != "git.diff" || listed[0].RequiredPermissions.Tools == nil || !*listed[0].RequiredPermissions.Tools {
		t.Fatalf("listed skill after mutating get response = %+v, want original capability metadata", listed)
	}
}

func TestService_UpdateProjectRoleAndWorkItem(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{
		Name: "Draft project",
		Roots: []Root{{
			ID:     "root_main",
			Path:   "/tmp/project",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.DefaultRootID != "root_main" {
		t.Fatalf("default root = %q, want root_main", project.DefaultRootID)
	}
	updatedProject, err := service.UpdateProject(ctx, Project{
		ID:          project.ID,
		Name:        "Ready project",
		Description: "Updated description.",
		Roots: []Root{{
			ID:     "root_main",
			Path:   "/tmp/project",
			Active: false,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if updatedProject.Name != "Ready project" || updatedProject.Description != "Updated description." || len(updatedProject.Roots) != 1 || updatedProject.Roots[0].Active {
		t.Fatalf("updated project = %+v, want replacement metadata and inactive root", updatedProject)
	}
	if updatedProject.DefaultRootID != "root_main" {
		t.Fatalf("updated default root = %q, want root_main", updatedProject.DefaultRootID)
	}
	if updatedProject.CreatedAt.IsZero() || updatedProject.UpdatedAt.Before(updatedProject.CreatedAt) {
		t.Fatalf("updated project timestamps = created %s updated %s", updatedProject.CreatedAt, updatedProject.UpdatedAt)
	}
	updatedProject, err = service.UpdateProject(ctx, Project{
		ID:   project.ID,
		Name: "Retargeted project",
		Roots: []Root{{
			ID:     "root_feature",
			Path:   "/tmp/project-feature",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProject(new roots) error = %v", err)
	}
	if updatedProject.DefaultRootID != "root_feature" {
		t.Fatalf("retargeted default root = %q, want root_feature", updatedProject.DefaultRootID)
	}
	if _, err := service.UpdateProject(ctx, Project{
		ID:            project.ID,
		Name:          "Broken default",
		DefaultRootID: "root_missing",
		Roots: []Root{{
			ID:   "root_main",
			Path: "/tmp/project",
		}},
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateProject(missing default root) error = %v, want ErrNotFound", err)
	}

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{Name: "Default profile"})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	executionProfile, err := service.CreateExecutionProfile(ctx, ExecutionProfile{Name: "Local execution"})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ProjectID:                 project.ID,
		Name:                      "Implementer",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if role.DefaultExecutionProfileID != executionProfile.ID {
		t.Fatalf("role = %+v, want default execution profile", role)
	}
	updatedRole, err := service.UpdateRole(ctx, Role{
		ProjectID:                 project.ID,
		ID:                        role.ID,
		Name:                      "Senior implementer",
		Description:               "Owns implementation.",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
		DefaultSkillIDs:           []string{"backend", "backend"},
		DefaultExecutionMode:      ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if updatedRole.Name != "Senior implementer" || len(updatedRole.DefaultSkillIDs) != 1 || updatedRole.DefaultExecutionMode != ExecutionMCPPull || updatedRole.DefaultExecutionProfileID != executionProfile.ID {
		t.Fatalf("updated role = %+v, want replacement defaults", updatedRole)
	}
	if _, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Missing execution", DefaultExecutionProfileID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateRole(missing execution profile) error = %v, want ErrNotFound", err)
	}

	work, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Original work",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	reviewedWork, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID:       project.ID,
		Title:           "Reviewed work",
		OwnerRoleID:     " " + role.ID + " ",
		ReviewerRoleIDs: []string{role.ID, role.ID, " "},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem(reviewed) error = %v", err)
	}
	if reviewedWork.OwnerRoleID != role.ID || len(reviewedWork.ReviewerRoleIDs) != 1 || reviewedWork.ReviewerRoleIDs[0] != role.ID {
		t.Fatalf("reviewed work item = %+v, want trimmed owner and deduped reviewer roles", reviewedWork)
	}
	updatedWork, err := service.UpdateWorkItem(ctx, WorkItem{
		ProjectID:       project.ID,
		ID:              work.ID,
		Title:           "Updated work",
		Brief:           "Updated brief.",
		Status:          WorkStatusReady,
		Priority:        PriorityNormal,
		OwnerRoleID:     role.ID,
		ReviewerRoleIDs: []string{role.ID, role.ID},
		RootID:          "root_feature",
	})
	if err != nil {
		t.Fatalf("UpdateWorkItem() error = %v", err)
	}
	if updatedWork.Title != "Updated work" || updatedWork.OwnerRoleID != role.ID || len(updatedWork.ReviewerRoleIDs) != 1 || updatedWork.RootID != "root_feature" {
		t.Fatalf("updated work item = %+v, want replacement metadata", updatedWork)
	}
	if _, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID:   project.ID,
		Title:       "Missing owner",
		OwnerRoleID: "missing",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateWorkItem(missing owner role) error = %v, want ErrNotFound", err)
	}
	if _, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID:       project.ID,
		Title:           "Missing reviewer",
		ReviewerRoleIDs: []string{role.ID, "missing"},
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateWorkItem(missing reviewer role) error = %v, want ErrNotFound", err)
	}
	if _, err := service.UpdateWorkItem(ctx, WorkItem{
		ProjectID:       project.ID,
		ID:              work.ID,
		Title:           "Missing update reviewer",
		ReviewerRoleIDs: []string{"missing"},
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateWorkItem(missing reviewer role) error = %v, want ErrNotFound", err)
	}
}

func TestService_ContextSourceMutations(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Sources"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	project, created, err := service.CreateContextSource(ctx, project.ID, Source{
		ID:             " src_agents ",
		Kind:           " workspace_instruction ",
		Title:          " AGENTS.md ",
		Locator:        " AGENTS.md ",
		Enabled:        true,
		Format:         " agents_md ",
		Scope:          " workspace ",
		TrustLabel:     " workspace_guidance ",
		SourceCategory: " instructions ",
		Metadata:       map[string]string{" root_id ": " root_main "},
	})
	if err != nil {
		t.Fatalf("CreateContextSource() error = %v", err)
	}
	if created.ID != "src_agents" || created.Kind != "workspace_instruction" || created.Locator != "AGENTS.md" || !created.Enabled || created.Metadata["root_id"] != "root_main" {
		t.Fatalf("created source = %+v, want normalized source metadata", created)
	}
	if len(project.ContextSources) != 1 {
		t.Fatalf("project sources after create = %+v, want one source", project.ContextSources)
	}
	createdAt := created.CreatedAt

	if _, _, err := service.CreateContextSource(ctx, project.ID, Source{ID: "src_agents", Locator: "README.md"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("CreateContextSource(duplicate) error = %v, want ErrDuplicate", err)
	}
	if _, _, err := service.CreateContextSource(ctx, project.ID, Source{ID: "src_empty"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateContextSource(empty locator) error = %v, want ErrInvalid", err)
	}

	project, updated, err := service.UpdateContextSource(ctx, project.ID, "src_agents", Source{
		Kind:           "url",
		Title:          "Design brief",
		Locator:        "https://example.invalid/design",
		Enabled:        false,
		Format:         "url",
		TrustLabel:     "operator_source",
		SourceCategory: "operator_source",
	})
	if err != nil {
		t.Fatalf("UpdateContextSource() error = %v", err)
	}
	if updated.ID != "src_agents" || updated.Kind != "url" || updated.Enabled || updated.Locator != "https://example.invalid/design" {
		t.Fatalf("updated source = %+v, want replacement metadata with stable id", updated)
	}
	if !updated.CreatedAt.Equal(createdAt) || !updated.UpdatedAt.After(createdAt) {
		t.Fatalf("updated source timestamps = created %s updated %s, want original created and newer updated", updated.CreatedAt, updated.UpdatedAt)
	}
	if len(project.ContextSources) != 1 {
		t.Fatalf("project sources after update = %+v, want one source", project.ContextSources)
	}

	got, err := service.GetContextSource(ctx, project.ID, "src_agents")
	if err != nil {
		t.Fatalf("GetContextSource() error = %v", err)
	}
	if got.Title != "Design brief" {
		t.Fatalf("GetContextSource() = %+v, want updated source", got)
	}
	sources, err := service.ListContextSources(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListContextSources() error = %v", err)
	}
	if len(sources) != 1 || sources[0].ID != "src_agents" {
		t.Fatalf("ListContextSources() = %+v, want updated source", sources)
	}

	if _, _, err := service.UpdateContextSource(ctx, project.ID, "src_missing", Source{Locator: "README.md"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateContextSource(missing) error = %v, want ErrNotFound", err)
	}
	project, deleted, err := service.DeleteContextSource(ctx, project.ID, "src_agents")
	if err != nil {
		t.Fatalf("DeleteContextSource() error = %v", err)
	}
	if deleted.ID != "src_agents" || len(project.ContextSources) != 0 {
		t.Fatalf("deleted source=%+v project sources=%+v, want source removed", deleted, project.ContextSources)
	}
	if _, err := service.GetContextSource(ctx, project.ID, "src_agents"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetContextSource(deleted) error = %v, want ErrNotFound", err)
	}
	if _, _, err := service.DeleteContextSource(ctx, project.ID, "src_agents"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteContextSource(missing) error = %v, want ErrNotFound", err)
	}
}

func TestService_RootMutations(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Roots"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	project, created, err := service.CreateRoot(ctx, project.ID, Root{
		ID:        " root_main ",
		Path:      " /workspace/main/../main ",
		Kind:      " git ",
		GitRemote: " https://github.com/hecatehq/hecate ",
		GitBranch: " main ",
		Active:    true,
	})
	if err != nil {
		t.Fatalf("CreateRoot() error = %v", err)
	}
	if created.ID != "root_main" || created.Path != "/workspace/main" || created.Kind != "git" || created.GitRemote != "https://github.com/hecatehq/hecate" || created.GitBranch != "main" || !created.Active {
		t.Fatalf("created root = %+v, want normalized root metadata", created)
	}
	if len(project.Roots) != 1 || project.DefaultRootID != "root_main" {
		t.Fatalf("project roots/default after create = %+v default=%q, want created root as default", project.Roots, project.DefaultRootID)
	}

	if _, _, err := service.CreateRoot(ctx, project.ID, Root{ID: "root_main", Path: "/workspace/other"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("CreateRoot(duplicate) error = %v, want ErrDuplicate", err)
	}
	if _, _, err := service.CreateRoot(ctx, project.ID, Root{ID: "root_empty"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateRoot(empty path) error = %v, want ErrInvalid", err)
	}

	project, updated, err := service.UpdateRoot(ctx, project.ID, "root_main", Root{
		Path:      "/workspace/.worktrees/feature",
		Kind:      "git_worktree",
		GitBranch: "feature/root-api",
		Active:    false,
	})
	if err != nil {
		t.Fatalf("UpdateRoot() error = %v", err)
	}
	if updated.ID != "root_main" || updated.Path != "/workspace/.worktrees/feature" || updated.Kind != "git_worktree" || updated.GitBranch != "feature/root-api" || updated.Active {
		t.Fatalf("updated root = %+v, want replacement metadata with stable id", updated)
	}
	if len(project.Roots) != 1 || project.DefaultRootID != "root_main" {
		t.Fatalf("project roots/default after update = %+v default=%q, want stable default root", project.Roots, project.DefaultRootID)
	}

	got, err := service.GetRoot(ctx, project.ID, "root_main")
	if err != nil {
		t.Fatalf("GetRoot() error = %v", err)
	}
	if got.Path != "/workspace/.worktrees/feature" {
		t.Fatalf("GetRoot() = %+v, want updated root", got)
	}
	roots, err := service.ListRoots(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoots() error = %v", err)
	}
	if len(roots) != 1 || roots[0].ID != "root_main" {
		t.Fatalf("ListRoots() = %+v, want updated root", roots)
	}
	if _, _, err := service.UpdateRoot(ctx, project.ID, "root_missing", Root{Path: "/workspace/new"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateRoot(missing) error = %v, want ErrNotFound", err)
	}

	project, _, err = service.CreateRoot(ctx, project.ID, Root{ID: "root_other", Path: "/workspace/other", Active: true})
	if err != nil {
		t.Fatalf("CreateRoot(other) error = %v", err)
	}
	project, deleted, err := service.DeleteRoot(ctx, project.ID, "root_main")
	if err != nil {
		t.Fatalf("DeleteRoot() error = %v", err)
	}
	if deleted.ID != "root_main" || len(project.Roots) != 1 || project.Roots[0].ID != "root_other" || project.DefaultRootID != "root_other" {
		t.Fatalf("deleted root=%+v project roots=%+v default=%q, want deleted root removed and remaining root defaulted", deleted, project.Roots, project.DefaultRootID)
	}
	if _, err := service.GetRoot(ctx, project.ID, "root_main"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRoot(deleted) error = %v, want ErrNotFound", err)
	}
	if _, _, err := service.DeleteRoot(ctx, project.ID, "root_main"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteRoot(missing) error = %v, want ErrNotFound", err)
	}
}

func TestService_AssignmentLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{
		Name:          "Reviewer profile",
		Instructions:  "Prefer concise findings with evidence.",
		ContextPolicy: "include_enabled",
		SkillIDs:      []string{"review", "review"},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	if len(profile.SkillIDs) != 1 || profile.SkillIDs[0] != "review" {
		t.Fatalf("profile = %+v, want deduped skill ids", profile)
	}
	executionProfile, err := service.CreateExecutionProfile(ctx, ExecutionProfile{
		Name:           "Local reviewer",
		AgentKind:      "any",
		ModelHint:      "local-small",
		ProviderHint:   "local",
		ToolsPolicy:    "readonly",
		WritesPolicy:   "block",
		NetworkPolicy:  "block",
		ApprovalPolicy: "require",
		AdapterOptions: map[string]any{"profile": "reviewer"},
	})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, Project{Name: "Cairnline"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ProjectID:                 project.ID,
		Name:                      "Reviewer",
		Instructions:              "Check the evidence and record blockers.",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Review assignment flow",
		Brief:     "Verify the MCP-pull lifecycle.",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
		DesiredAgent: DesiredAgent{
			Kind:     "any",
			SkillIDs: []string{"review", "review"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if assignment.Status != AssignmentQueued || assignment.ExecutionMode != ExecutionMCPPull {
		t.Fatalf("assignment = %+v, want queued mcp_pull assignment", assignment)
	}
	if got := assignment.DesiredAgent.SkillIDs; len(got) != 1 || got[0] != "review" {
		t.Fatalf("skill ids = %+v, want deduped review id", got)
	}
	implementer, err := service.CreateRole(ctx, Role{
		ProjectID:                 project.ID,
		Name:                      "Implementer",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
	})
	if err != nil {
		t.Fatalf("CreateRole(implementer) error = %v", err)
	}
	updatedWork, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Updated assignment target",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem(updated target) error = %v", err)
	}
	updatedAssignment, err := service.UpdateAssignment(ctx, Assignment{
		ProjectID:          project.ID,
		ID:                 assignment.ID,
		WorkItemID:         updatedWork.ID,
		RoleID:             implementer.ID,
		ProfileID:          profile.ID,
		ExecutionProfileID: executionProfile.ID,
		ExecutionMode:      ExecutionExternalAdapter,
		Status:             AssignmentQueued,
		DesiredAgent: DesiredAgent{
			Kind:     "codex",
			SkillIDs: []string{"review", "backend", "backend"},
		},
		ExecutionRef:      "chat-1",
		ContextSnapshotID: "ctx-1",
	})
	if err != nil {
		t.Fatalf("UpdateAssignment() error = %v", err)
	}
	if updatedAssignment.WorkItemID != updatedWork.ID || updatedAssignment.RoleID != implementer.ID || updatedAssignment.ExecutionMode != ExecutionExternalAdapter || updatedAssignment.ProfileID != profile.ID || updatedAssignment.ExecutionProfileID != executionProfile.ID {
		t.Fatalf("updated assignment = %+v, want replacement metadata", updatedAssignment)
	}
	if updatedAssignment.CreatedAt != assignment.CreatedAt {
		t.Fatalf("updated assignment created_at = %s, want original %s", updatedAssignment.CreatedAt, assignment.CreatedAt)
	}
	if updatedAssignment.DesiredAgent.Kind != "codex" || len(updatedAssignment.DesiredAgent.SkillIDs) != 2 || updatedAssignment.DesiredAgent.SkillIDs[1] != "backend" || updatedAssignment.ExecutionRef != "chat-1" || updatedAssignment.ContextSnapshotID != "ctx-1" {
		t.Fatalf("updated assignment desired/context = %+v/%q/%q, want normalized desired agent and refs", updatedAssignment.DesiredAgent, updatedAssignment.ExecutionRef, updatedAssignment.ContextSnapshotID)
	}
	if _, err := service.UpdateAssignment(ctx, Assignment{
		ProjectID:     project.ID,
		ID:            assignment.ID,
		WorkItemID:    "missing",
		RoleID:        implementer.ID,
		ExecutionMode: ExecutionMCPPull,
		Status:        AssignmentQueued,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateAssignment(missing work item) error = %v, want ErrNotFound", err)
	}
	if _, err := service.UpdateAssignment(ctx, Assignment{
		ProjectID:     project.ID,
		ID:            assignment.ID,
		WorkItemID:    updatedWork.ID,
		RoleID:        implementer.ID,
		ExecutionMode: ExecutionMCPPull,
		Status:        "paused",
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateAssignment(invalid status) error = %v, want ErrInvalid", err)
	}
	work = updatedWork
	role = implementer
	assignment = updatedAssignment
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, AssignmentRunning, "run-queued"); !errors.Is(err, ErrConflict) {
		t.Fatalf("UpdateAssignmentStatus() before claim error = %v, want ErrConflict", err)
	}

	claimed, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a")
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if claimed.Status != AssignmentClaimed || claimed.ClaimedBy != "agent-a" {
		t.Fatalf("claimed assignment = %+v, want claimed by agent-a", claimed)
	}
	running, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, AssignmentRunning, "run-123")
	if err != nil {
		t.Fatalf("UpdateAssignmentStatus() error = %v", err)
	}
	if running.Status != AssignmentRunning || running.ExecutionRef != "run-123" {
		t.Fatalf("running assignment = %+v, want running with execution ref", running)
	}
	if running.StartedAt.IsZero() || !running.CompletedAt.IsZero() {
		t.Fatalf("running assignment timestamps = started:%s completed:%s, want started only", running.StartedAt, running.CompletedAt)
	}
	runningUpdated, err := service.UpdateAssignment(ctx, Assignment{
		ProjectID:         project.ID,
		ID:                assignment.ID,
		WorkItemID:        work.ID,
		RoleID:            role.ID,
		ExecutionMode:     ExecutionOrchestrated,
		Status:            AssignmentRunning,
		DesiredAgent:      DesiredAgent{Kind: "hecate", SkillIDs: []string{"review"}},
		ExecutionRef:      "run-456",
		ContextSnapshotID: "ctx-running",
	})
	if err != nil {
		t.Fatalf("UpdateAssignment(running) error = %v", err)
	}
	if runningUpdated.Status != AssignmentRunning || runningUpdated.ClaimedBy != "agent-a" || runningUpdated.ExecutionMode != ExecutionOrchestrated || runningUpdated.ExecutionRef != "run-456" || runningUpdated.ContextSnapshotID != "ctx-running" {
		t.Fatalf("running updated assignment = %+v, want metadata update preserving claim", runningUpdated)
	}
	if !runningUpdated.StartedAt.Equal(running.StartedAt) || !runningUpdated.CompletedAt.IsZero() {
		t.Fatalf("running updated timestamps = started:%s completed:%s, want preserved started timestamp", runningUpdated.StartedAt, runningUpdated.CompletedAt)
	}

	packet, err := service.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() error = %v", err)
	}
	if packet.Project.ID != project.ID || packet.WorkItem.ID != work.ID || packet.Role == nil || packet.Role.ID != role.ID {
		t.Fatalf("assignment context = %+v, want project/work/role metadata", packet)
	}

	artifact, err := service.CreateArtifact(ctx, Artifact{
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		Kind:           "decision_note",
		Title:          "Decision",
		Body:           "Keep the portable artifact in the launch packet.",
		AuthorRoleID:   role.ID,
		ProvenanceKind: "operator",
		TrustLabel:     "operator_reviewed",
	})
	if err != nil {
		t.Fatalf("CreateArtifact() error = %v", err)
	}
	gotArtifact, err := service.GetArtifact(ctx, project.ID, work.ID, artifact.ID)
	if err != nil {
		t.Fatalf("GetArtifact() error = %v", err)
	}
	if gotArtifact.Kind != "decision_note" || gotArtifact.AssignmentID != assignment.ID || gotArtifact.AuthorRoleID != role.ID {
		t.Fatalf("GetArtifact() = %+v, want recorded generic artifact", gotArtifact)
	}
	evidence, err := service.CreateEvidence(ctx, Evidence{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Test output",
		Locator:      "file://report.md",
		SourceKind:   "pull_request",
		ExternalID:   "PR 42",
		Provider:     "github",
	})
	if err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if evidence.TrustLabel != EvidenceTrustOperator {
		t.Fatalf("evidence = %+v, want default trust label", evidence)
	}
	gotEvidence, err := service.GetEvidence(ctx, project.ID, work.ID, evidence.ID)
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if gotEvidence.ID != evidence.ID || gotEvidence.AssignmentID != assignment.ID || gotEvidence.Locator != "file://report.md" || gotEvidence.SourceKind != "pull_request" || gotEvidence.ExternalID != "PR 42" || gotEvidence.Provider != "github" {
		t.Fatalf("GetEvidence() = %+v, want recorded evidence", gotEvidence)
	}
	review, err := service.CreateReview(ctx, Review{
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		ReviewerRoleID: role.ID,
		Body:           "Looks good with one note.",
		Verdict:        ReviewVerdictApproved,
		Risk:           ReviewRiskLow,
	})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if review.Title != "Review" || review.Status != ReviewStatusRecorded {
		t.Fatalf("review = %+v, want default title and recorded status", review)
	}
	gotReview, err := service.GetReview(ctx, project.ID, work.ID, review.ID)
	if err != nil {
		t.Fatalf("GetReview() error = %v", err)
	}
	if gotReview.ID != review.ID || gotReview.AssignmentID != assignment.ID || gotReview.ReviewerRoleID != role.ID {
		t.Fatalf("GetReview() = %+v, want recorded review", gotReview)
	}
	riskReview, err := service.CreateReview(ctx, Review{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Body:         "Risk accepted by operator.",
		Verdict:      ReviewVerdictRisk,
		Risk:         ReviewRiskUnknown,
	})
	if err != nil {
		t.Fatalf("CreateReview(risk/unknown) error = %v", err)
	}
	if riskReview.Verdict != ReviewVerdictRisk || riskReview.Risk != ReviewRiskUnknown {
		t.Fatalf("risk review = %+v, want Hecate-compatible risk/unknown values", riskReview)
	}
	if _, err := service.CreateReview(ctx, Review{ProjectID: project.ID, WorkItemID: work.ID, Body: "Legacy verdict.", Verdict: "pass"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateReview(legacy pass) error = %v, want ErrInvalid", err)
	}
	handoff, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:             project.ID,
		WorkItemID:            work.ID,
		SourceAssignmentID:    assignment.ID,
		SourceRunID:           "run-123",
		FromRoleID:            role.ID,
		ToRoleID:              role.ID,
		TargetAssignmentID:    assignment.ID,
		TargetWorkItemID:      work.ID,
		Title:                 "Follow-up",
		Body:                  "Carry this into the next pass.",
		RecommendedNextAction: "Verify the recorded evidence.",
		LinkedArtifactIDs:     []string{evidence.ID, review.ID, review.ID},
		ContextRefs:           []string{"ctx_1"},
		Status:                HandoffStatusAccepted,
		ProvenanceKind:        "operator",
		TrustLabel:            "operator_reviewed",
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if handoff.Status != HandoffStatusAccepted || handoff.SourceAssignmentID != assignment.ID || handoff.TargetAssignmentID != assignment.ID || handoff.RecommendedNextAction == "" || len(handoff.LinkedArtifactIDs) != 2 || len(handoff.ContextRefs) != 1 {
		t.Fatalf("handoff = %+v, want metadata preserved", handoff)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		Title:      "Invalid status",
		Body:       "Invalid handoff status should be rejected.",
		Status:     "paused",
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateHandoff(invalid status) error = %v, want ErrInvalid", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		SourceAssignmentID: "missing",
		Title:              "Missing source",
		Body:               "Missing source assignment should be rejected.",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateHandoff(missing source assignment) error = %v, want ErrNotFound", err)
	}
	otherWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Different follow-up target"})
	if err != nil {
		t.Fatalf("CreateWorkItem(other) error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		TargetAssignmentID: assignment.ID,
		TargetWorkItemID:   otherWork.ID,
		Title:              "Mismatched target",
		Body:               "Target assignment and work item should agree.",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateHandoff(mismatched target) error = %v, want ErrNotFound", err)
	}
	memory, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ProjectID:           project.ID,
		Title:               "Project convention",
		Body:                "Reviews should include concrete evidence.",
		SuggestedTrustLabel: MemoryTrustGenerated,
		SuggestedSourceKind: MemorySourceGenerated,
		SuggestedSourceID:   assignment.ID,
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	if memory.Status != MemoryCandidatePending || memory.SuggestedTrustLabel != MemoryTrustGenerated {
		t.Fatalf("memory candidate = %+v, want pending generated status", memory)
	}
	memoryEntry, err := service.CreateMemoryEntry(ctx, MemoryEntry{
		ProjectID:  project.ID,
		Title:      "Review memory",
		Body:       "Reviews should include concrete evidence.",
		SourceKind: MemorySourceOperator,
		SourceID:   assignment.ID,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if !memoryEntry.Enabled || memoryEntry.TrustLabel != MemoryTrustOperator {
		t.Fatalf("memory entry = %+v, want enabled operator memory", memoryEntry)
	}
	launchPacket, err := service.AssignmentLaunchPacket(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentLaunchPacket() error = %v", err)
	}
	if launchPacket.Kind != LaunchPacketKindAssignment || launchPacket.Project.ID != project.ID || launchPacket.WorkItem.ID != work.ID || launchPacket.Role == nil || launchPacket.Role.ID != role.ID {
		t.Fatalf("launch packet = %+v, want project/work/role packet", launchPacket)
	}
	if launchPacket.Profile == nil || launchPacket.Profile.ID != profile.ID || launchPacket.ExecutionProfile == nil || launchPacket.ExecutionProfile.ID != executionProfile.ID {
		t.Fatalf("launch packet = %+v, want resolved profile metadata", launchPacket)
	}
	if len(launchPacket.Memory) != 1 || launchPacket.Memory[0].ID != memoryEntry.ID {
		t.Fatalf("launch packet memory = %+v, want accepted project memory", launchPacket.Memory)
	}
	if len(launchPacket.Artifacts) != 1 || len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 2 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts artifacts=%d evidence=%d reviews=%d handoffs=%d memory=%d, want one artifact/evidence/handoff/memory candidate and two reviews", len(launchPacket.Artifacts), len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}
	if launchPacket.Artifacts[0].ID != artifact.ID || launchPacket.Artifacts[0].Kind != "decision_note" {
		t.Fatalf("launch packet artifacts = %+v, want generic artifact", launchPacket.Artifacts)
	}
	if launchPacket.Evidence[0].AssignmentID != assignment.ID {
		t.Fatalf("launch packet evidence = %+v, want assignment-scoped evidence", launchPacket.Evidence[0])
	}
	if launchPacket.Evidence[0].SourceKind != "pull_request" || launchPacket.Evidence[0].ExternalID != "PR 42" || launchPacket.Evidence[0].Provider != "github" {
		t.Fatalf("launch packet evidence = %+v, want source/provider/external metadata", launchPacket.Evidence[0])
	}
	if !reviewIDExistsForTest(launchPacket.Reviews, review.ID) || !reviewIDExistsForTest(launchPacket.Reviews, riskReview.ID) {
		t.Fatalf("launch packet reviews = %+v, want approved and risk reviews", launchPacket.Reviews)
	}

	completed, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, AssignmentCompleted, "run-123")
	if err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	if completed.Status != AssignmentCompleted || completed.ExecutionRef != "run-123" {
		t.Fatalf("completed assignment = %+v, want completed with execution ref", completed)
	}
	if !completed.StartedAt.Equal(running.StartedAt) || completed.CompletedAt.IsZero() {
		t.Fatalf("completed timestamps = started:%s completed:%s, want preserved start and completed timestamp", completed.StartedAt, completed.CompletedAt)
	}
}

func TestService_CollaborationImportPreservesTimestamps(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{Name: "Cairnline"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Review imported collaboration"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	createdAt := time.Date(2026, 6, 3, 12, 3, 30, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 3, 12, 6, 30, 0, time.UTC)
	artifact, err := service.CreateArtifact(ctx, Artifact{
		ID:           "art_imported_decision",
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Kind:         "decision_note",
		Title:        "Imported decision",
		Body:         "Keep the imported timestamp.",
		AuthorRoleID: role.ID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	})
	if err != nil {
		t.Fatalf("CreateArtifact() error = %v", err)
	}
	if !artifact.CreatedAt.Equal(createdAt) || !artifact.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("artifact timestamps = %s/%s, want %s/%s", artifact.CreatedAt, artifact.UpdatedAt, createdAt, updatedAt)
	}
	gotArtifact, err := service.GetArtifact(ctx, project.ID, work.ID, artifact.ID)
	if err != nil {
		t.Fatalf("GetArtifact() error = %v", err)
	}
	if !gotArtifact.CreatedAt.Equal(createdAt) || !gotArtifact.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("stored artifact timestamps = %s/%s, want %s/%s", gotArtifact.CreatedAt, gotArtifact.UpdatedAt, createdAt, updatedAt)
	}

	evidence, err := service.CreateEvidence(ctx, Evidence{
		ID:           "ev_imported",
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Imported evidence",
		Locator:      "https://example.invalid/evidence",
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	})
	if err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if !evidence.CreatedAt.Equal(createdAt) || !evidence.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("evidence timestamps = %s/%s, want %s/%s", evidence.CreatedAt, evidence.UpdatedAt, createdAt, updatedAt)
	}
	gotEvidence, err := service.GetEvidence(ctx, project.ID, work.ID, evidence.ID)
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if !gotEvidence.CreatedAt.Equal(createdAt) || !gotEvidence.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("stored evidence timestamps = %s/%s, want %s/%s", gotEvidence.CreatedAt, gotEvidence.UpdatedAt, createdAt, updatedAt)
	}

	review, err := service.CreateReview(ctx, Review{
		ID:             "rev_imported",
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		ReviewerRoleID: role.ID,
		Title:          "Imported review",
		Body:           "Review timestamp should survive import.",
		Verdict:        ReviewVerdictApproved,
		Risk:           ReviewRiskLow,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if !review.CreatedAt.Equal(createdAt) || !review.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("review timestamps = %s/%s, want %s/%s", review.CreatedAt, review.UpdatedAt, createdAt, updatedAt)
	}
	gotReview, err := service.GetReview(ctx, project.ID, work.ID, review.ID)
	if err != nil {
		t.Fatalf("GetReview() error = %v", err)
	}
	if !gotReview.CreatedAt.Equal(createdAt) || !gotReview.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("stored review timestamps = %s/%s, want %s/%s", gotReview.CreatedAt, gotReview.UpdatedAt, createdAt, updatedAt)
	}
}

func TestService_AssignmentLaunchPacketUsesProjectDefaults(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{
		Name:     "Project default profile",
		SkillIDs: []string{"research"},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	executionProfile, err := service.CreateExecutionProfile(ctx, ExecutionProfile{
		Name:      "Project default execution",
		AgentKind: "any",
	})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, Project{
		Name:                      "Project defaults",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := service.CreateProjectSkill(ctx, ProjectSkill{
		ProjectID:  project.ID,
		ID:         "research",
		Title:      "Research",
		Format:     SkillFormatMarkdown,
		Enabled:    true,
		Status:     SkillStatusAvailable,
		TrustLabel: SkillTrustWorkspace,
	}); err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ProjectID: project.ID,
		Name:      "Researcher",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Investigate project defaults",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	packet, err := service.AssignmentLaunchPacket(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentLaunchPacket() error = %v", err)
	}
	if packet.Profile == nil || packet.Profile.ID != profile.ID {
		t.Fatalf("launch packet profile = %+v, want project default profile", packet.Profile)
	}
	if packet.ExecutionProfile == nil || packet.ExecutionProfile.ID != executionProfile.ID {
		t.Fatalf("launch packet execution profile = %+v, want project default execution profile", packet.ExecutionProfile)
	}
	if len(packet.Skills) != 1 || packet.Skills[0].ID != "research" {
		t.Fatalf("launch packet skills = %+v, want skills from project default profile", packet.Skills)
	}
}

func TestService_WorkItemCloseoutReadiness(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Closeout project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Close out work"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, AssignmentCompleted, "run-1"); err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}

	readiness, err := service.WorkItemCloseoutReadiness(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("WorkItemCloseoutReadiness() error = %v", err)
	}
	if readiness.Ready || readiness.Status != "blocked" || len(readiness.MissingEvidenceAssignmentIDs) != 1 || readiness.MissingEvidenceAssignmentIDs[0] != assignment.ID {
		t.Fatalf("readiness = %+v, want missing evidence blocker", readiness)
	}

	otherWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Other work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(other) error = %v", err)
	}
	otherAssignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: otherWork.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment(other) error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: otherAssignment.ID,
		Title:        "Wrong assignment evidence",
		Locator:      "file://wrong.md",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateEvidence(wrong assignment) error = %v, want ErrNotFound", err)
	}

	if _, err := service.CreateEvidence(ctx, Evidence{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Test output",
		Locator:      "file://report.md",
	}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	readiness, err = service.WorkItemCloseoutReadiness(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("WorkItemCloseoutReadiness() with evidence error = %v", err)
	}
	if !readiness.Ready || readiness.CompletedAssignments != 1 || readiness.AssignmentCount != 1 {
		t.Fatalf("readiness with evidence = %+v, want ready closeout", readiness)
	}
}

func TestService_WorkItemCloseoutReadinessManualAndActiveStates(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Manual closeout"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Coordinator"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	manualWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Manual work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(manual) error = %v", err)
	}
	readiness, err := service.WorkItemCloseoutReadiness(ctx, project.ID, manualWork.ID)
	if err != nil {
		t.Fatalf("WorkItemCloseoutReadiness(manual) error = %v", err)
	}
	if !readiness.Ready || len(readiness.Warnings) != 1 || readiness.AssignmentCount != 0 {
		t.Fatalf("manual readiness = %+v, want ready manual warning", readiness)
	}

	activeWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Active work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(active) error = %v", err)
	}
	if _, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: activeWork.ID, RoleID: role.ID}); err != nil {
		t.Fatalf("CreateAssignment(active) error = %v", err)
	}
	readiness, err = service.WorkItemCloseoutReadiness(ctx, project.ID, activeWork.ID)
	if err != nil {
		t.Fatalf("WorkItemCloseoutReadiness(active) error = %v", err)
	}
	if readiness.Ready || readiness.Status != "blocked" || !containsString(readiness.Blockers, "1 assignment is still active") {
		t.Fatalf("active readiness = %+v, want active assignment blocker", readiness)
	}
}

func TestService_WorkItemCloseoutReadinessReviewFollowUp(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Review closeout"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Review follow-up"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, AssignmentCompleted, "run-1"); err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Title: "Evidence", Locator: "file://report.md"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	review, err := service.CreateReview(ctx, Review{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Architecture review",
		Body:         "Needs follow-up.",
		Verdict:      ReviewVerdictChangesRequested,
		Risk:         ReviewRiskMedium,
	})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	readiness, err := service.WorkItemCloseoutReadiness(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("WorkItemCloseoutReadiness() error = %v", err)
	}
	if readiness.Ready || len(readiness.ReviewFollowUps) != 1 || readiness.ReviewFollowUps[0].ArtifactID != review.ID || readiness.ReviewFollowUps[0].Blocker == "" {
		t.Fatalf("readiness review follow-up = %+v, want typed blocker", readiness)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		TargetWorkItemID:   work.ID,
		Title:              "Dismiss review follow-up",
		Body:               "Operator accepted the risk.",
		LinkedArtifactIDs:  []string{review.ID},
		Status:             HandoffStatusDismissed,
		TargetAssignmentID: "",
	}); err != nil {
		t.Fatalf("CreateHandoff(dismissed) error = %v", err)
	}
	readiness, err = service.WorkItemCloseoutReadiness(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("WorkItemCloseoutReadiness() after dismissed handoff error = %v", err)
	}
	if !readiness.Ready || readiness.ReviewFollowUpCount != 0 {
		t.Fatalf("readiness after dismissed follow-up = %+v, want ready closeout", readiness)
	}
}

func TestService_ProjectOperationsBrief(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Operations"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Operator"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ProjectID: project.ID,
		Title:     "Testing convention",
		Body:      "Record durable test lessons.",
	}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	activeWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Active work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(active) error = %v", err)
	}
	if _, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: activeWork.ID, RoleID: role.ID}); err != nil {
		t.Fatalf("CreateAssignment(active) error = %v", err)
	}
	failedWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Failed work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(failed) error = %v", err)
	}
	failedAssignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: failedWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(failed) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, failedAssignment.ID, AssignmentFailed, "run-failed"); err != nil {
		t.Fatalf("CompleteAssignment(failed) error = %v", err)
	}
	missingEvidenceWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Needs evidence"})
	if err != nil {
		t.Fatalf("CreateWorkItem(missing evidence) error = %v", err)
	}
	missingEvidenceAssignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: missingEvidenceWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(missing evidence) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, missingEvidenceAssignment.ID, AssignmentCompleted, "run-missing"); err != nil {
		t.Fatalf("CompleteAssignment(missing evidence) error = %v", err)
	}
	openHandoff, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:  project.ID,
		WorkItemID: missingEvidenceWork.ID,
		Title:      "Follow-up path",
		Body:       "Operator needs to decide the next path.",
		Status:     HandoffStatusOpen,
	})
	if err != nil {
		t.Fatalf("CreateHandoff(open) error = %v", err)
	}
	reviewWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Needs review follow-up"})
	if err != nil {
		t.Fatalf("CreateWorkItem(review) error = %v", err)
	}
	reviewAssignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: reviewWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(review) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, reviewAssignment.ID, AssignmentCompleted, "run-review"); err != nil {
		t.Fatalf("CompleteAssignment(review) error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{ProjectID: project.ID, WorkItemID: reviewWork.ID, AssignmentID: reviewAssignment.ID, Title: "Review evidence", Locator: "file://review.md"}); err != nil {
		t.Fatalf("CreateEvidence(review) error = %v", err)
	}
	review, err := service.CreateReview(ctx, Review{
		ProjectID:    project.ID,
		WorkItemID:   reviewWork.ID,
		AssignmentID: reviewAssignment.ID,
		Title:        "Risk review",
		Body:         "Needs a follow-up path.",
		Verdict:      ReviewVerdictChangesRequested,
		Risk:         ReviewRiskMedium,
	})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	closeoutWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Ready work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(closeout) error = %v", err)
	}
	closeoutAssignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: closeoutWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(closeout) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, closeoutAssignment.ID, AssignmentCompleted, "run-ready"); err != nil {
		t.Fatalf("CompleteAssignment(closeout) error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{ProjectID: project.ID, WorkItemID: closeoutWork.ID, AssignmentID: closeoutAssignment.ID, Title: "Ready evidence", Locator: "file://ready.md"}); err != nil {
		t.Fatalf("CreateEvidence(closeout) error = %v", err)
	}
	unassignedWork, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Unassigned work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(unassigned) error = %v", err)
	}

	brief, err := service.ProjectOperationsBrief(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectOperationsBrief() error = %v", err)
	}
	if brief.Status != ProjectOperationsStatusAttention || brief.Next == nil || brief.Next.AssignmentID != failedAssignment.ID {
		t.Fatalf("brief next = %+v, want failed assignment first in attention brief", brief.Next)
	}
	if brief.Counts.WorkItems != 6 || brief.Counts.OpenWorkItems != 6 || brief.Counts.Assignments != 5 {
		t.Fatalf("brief counts = %+v, want six open work items and five assignments", brief.Counts)
	}
	if brief.Counts.ActiveAssignments != 0 || brief.Counts.BlockedAssignments != 2 || brief.Counts.PendingMemoryCandidates != 1 || brief.Counts.MissingEvidence != 1 || brief.Counts.ReviewFollowUps != 1 || brief.Counts.OpenHandoffs != 1 || brief.Counts.CloseoutReady != 1 {
		t.Fatalf("brief counts = %+v, want queued+failed blocked and memory/evidence/review/handoff/closeout coverage", brief.Counts)
	}
	if !containsOperation(brief.Items, ProjectOperationKindReviewFollowUp, reviewWork.ID, review.ID) {
		t.Fatalf("brief items = %+v, want review follow-up item for %s", brief.Items, review.ID)
	}
	if !containsOperation(brief.Items, ProjectOperationKindMissingEvidence, missingEvidenceWork.ID, missingEvidenceAssignment.ID) {
		t.Fatalf("brief items = %+v, want missing evidence item for %s", brief.Items, missingEvidenceAssignment.ID)
	}
	if !containsOperation(brief.Items, ProjectOperationKindHandoff, missingEvidenceWork.ID, openHandoff.ID) {
		t.Fatalf("brief items = %+v, want open handoff item for %s", brief.Items, openHandoff.ID)
	}
	if !containsOperation(brief.Items, ProjectOperationKindCloseoutReady, closeoutWork.ID, "") {
		t.Fatalf("brief items = %+v, want closeout ready item for %s", brief.Items, closeoutWork.ID)
	}
	if !containsOperation(brief.Items, ProjectOperationKindWorkItem, unassignedWork.ID, "") {
		t.Fatalf("brief items = %+v, want unassigned work item for %s", brief.Items, unassignedWork.ID)
	}
}

func TestService_ProjectSetupReadiness(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Setup"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	readiness, err := service.ProjectSetupReadiness(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectSetupReadiness(pristine) error = %v", err)
	}
	if !readiness.ShowOnboarding || readiness.SetupStarted || readiness.FirstWorkReady {
		t.Fatalf("pristine readiness = %+v, want onboarding without setup started", readiness)
	}
	if readiness.Summary.WorkItemCount != 0 || readiness.Summary.RoleCount != 0 || readiness.Summary.HasPurpose || readiness.Summary.HasActiveRoot {
		t.Fatalf("pristine summary = %+v, want empty setup", readiness.Summary)
	}
	if readiness.PrimaryAction.Kind != ProjectSetupActionSetupProject {
		t.Fatalf("primary action = %+v, want setup project", readiness.PrimaryAction)
	}
	if check := setupCheckByID(readiness.Checks, "workspace_source"); check.Status != ProjectSetupStatusOptional || !check.Optional {
		t.Fatalf("workspace check = %+v, want optional rootless project", check)
	}
	if check := setupCheckByID(readiness.Checks, "purpose"); check.Status != ProjectSetupStatusTodo || check.Action == nil || check.Action.Kind != ProjectSetupActionUpdateProject {
		t.Fatalf("purpose check = %+v, want todo update action", check)
	}

	project.Description = "Coordinate setup."
	project.Roots = []Root{{ID: "root", Path: t.TempDir(), Kind: "workspace", Active: true}}
	project.DefaultRootID = "root"
	project.ContextSources = []Source{{Kind: "workspace_instruction", Title: "AGENTS.md", Enabled: true}}
	if _, err := service.UpdateProject(ctx, project); err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if _, err := service.CreateExecutionProfile(ctx, ExecutionProfile{Name: "Local execution"}); err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	if _, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Reviewer"}); err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if _, err := service.CreateProjectSkill(ctx, ProjectSkill{ProjectID: project.ID, ID: "review", Title: "Review", Format: SkillFormatMarkdown, Status: SkillStatusAvailable, TrustLabel: SkillTrustWorkspace}); err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	if _, err := service.CreateMemoryEntry(ctx, MemoryEntry{ProjectID: project.ID, Title: "Memory", Body: "Remember setup."}); err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{ProjectID: project.ID, Title: "Candidate", Body: "Review me."}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	readiness, err = service.ProjectSetupReadiness(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectSetupReadiness(configured) error = %v", err)
	}
	if readiness.ShowOnboarding || !readiness.SetupStarted || !readiness.FirstWorkReady {
		t.Fatalf("configured readiness = %+v, want setup started and first work ready", readiness)
	}
	if readiness.Summary.RoleCount != 1 || readiness.Summary.SkillCount != 1 || readiness.Summary.ExecutionProfileCount != 1 || readiness.Summary.EnabledContextSourceCount != 1 || readiness.Summary.SavedMemoryCount != 1 || readiness.Summary.PendingMemoryCandidateCount != 1 {
		t.Fatalf("configured summary = %+v, want all setup counts", readiness.Summary)
	}
	for _, id := range []string{"purpose", "workspace_source", "execution_profiles", "sources_memory", "roles"} {
		if check := setupCheckByID(readiness.Checks, id); check.Status != ProjectSetupStatusReady {
			t.Fatalf("check %s = %+v, want ready", id, check)
		}
	}
	if check := setupCheckByID(readiness.Checks, "first_work_item"); check.Status != ProjectSetupStatusTodo || check.Action == nil || check.Action.Kind != ProjectSetupActionCreateWorkItem {
		t.Fatalf("first work check = %+v, want create work action", check)
	}
	if _, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "First work"}); err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	readiness, err = service.ProjectSetupReadiness(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectSetupReadiness(after work) error = %v", err)
	}
	if readiness.FirstWorkReady || readiness.Summary.WorkItemCount != 1 {
		t.Fatalf("after-work readiness = %+v, want first work no longer ready-to-create", readiness)
	}
}

func TestService_ProjectHealth(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{
		Name:           "Health",
		Description:    "Track attention.",
		ContextSources: []Source{{Kind: "workspace_instruction", Title: "AGENTS.md", Enabled: true}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ProjectID:       project.ID,
		Name:            "Reviewer",
		DefaultSkillIDs: []string{"missing-skill", "disabled-skill"},
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	disabledSkill, err := service.CreateProjectSkill(ctx, ProjectSkill{ProjectID: project.ID, ID: "disabled-skill", Title: "Disabled", Format: SkillFormatMarkdown, Status: SkillStatusAvailable, TrustLabel: SkillTrustWorkspace})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	disabledSkill.Enabled = false
	if _, err := service.UpdateProjectSkill(ctx, disabledSkill); err != nil {
		t.Fatalf("UpdateProjectSkill(disabled) error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{ProjectID: project.ID, Title: "Candidate", Body: "Check this."}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Review work"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	queued, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID, DesiredAgent: DesiredAgent{SkillIDs: []string{"missing-skill"}}})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	completed, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(completed) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, completed.ID, AssignmentCompleted, "run-complete"); err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	review, err := service.CreateReview(ctx, Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: completed.ID, Title: "Needs follow-up", Body: "Please follow up.", Verdict: ReviewVerdictChangesRequested})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{ProjectID: project.ID, WorkItemID: work.ID, Title: "Open handoff", Body: "Decide next path.", LinkedArtifactIDs: []string{review.ID}}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}

	health, err := service.ProjectHealth(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectHealth() error = %v", err)
	}
	if health.Status != ProjectHealthStatusAttention || health.Summary.AttentionCount == 0 || health.Summary.AvailableAttentionCount < health.Summary.AttentionCount {
		t.Fatalf("health = %+v, want attention status", health)
	}
	if health.Summary.BlockedAssignmentCount != 1 || health.Summary.PendingMemoryCandidateCount != 1 || health.Summary.OpenHandoffCount != 1 || health.Summary.ReviewFollowUpCount != 1 {
		t.Fatalf("health summary = %+v, want assignment/memory/handoff/review counts", health.Summary)
	}
	if health.Summary.MissingProfileReferenceCount != 0 || health.Summary.ProjectSkillIssueCount == 0 {
		t.Fatalf("health summary = %+v, want skill issues without service-created missing profiles", health.Summary)
	}
	if !containsHealthAttention(health.Attention, ProjectOperationKindAssignment, queued.ID) {
		t.Fatalf("health attention = %+v, want queued assignment", health.Attention)
	}
}

func TestService_ProjectHealthIncludesProjectDefaultProfileReferences(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{
		Name:             "Missing project default",
		DefaultProfileID: "missing_profile",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	health, err := service.ProjectHealth(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectHealth() error = %v", err)
	}
	if health.Summary.MissingProfileReferenceCount != 1 {
		t.Fatalf("health summary = %+v, want one missing project default profile", health.Summary)
	}
	if !containsHealthAttentionDetail(health.Attention, ProjectOperationKindProfile, "missing_profile") {
		t.Fatalf("health attention = %+v, want missing project default profile detail", health.Attention)
	}
}

func TestService_AssistantProposalApply(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Assistant"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	proposal := AssistantProposal{
		ID:        "prop_assistant",
		ProjectID: project.ID,
		Title:     "Set up first work",
		Actions: []AssistantAction{
			{
				Kind: AssistantActionCreateRole,
				Role: &Role{
					ID:        "role_operator",
					ProjectID: project.ID,
					Name:      "Operator",
				},
			},
			{
				Kind: AssistantActionCreateWorkItem,
				WorkItem: &WorkItem{
					ID:        "work_first",
					ProjectID: project.ID,
					Title:     "First reviewable task",
					Brief:     "Prove the proposal path.",
				},
			},
			{
				Kind: AssistantActionCreateAssignment,
				Assignment: &Assignment{
					ID:            "asgn_first",
					ProjectID:     project.ID,
					WorkItemID:    "work_first",
					RoleID:        "role_operator",
					ExecutionMode: ExecutionMCPPull,
					DesiredAgent:  DesiredAgent{Kind: DesiredAgentAny},
				},
			},
			{
				Kind: AssistantActionCreateMemoryCandidate,
				MemoryCandidate: &MemoryCandidate{
					ID:        "memcand_first",
					ProjectID: project.ID,
					Title:     "Project setup lesson",
					Body:      "The first work item came from a confirmed proposal.",
				},
			},
		},
	}
	normalized, err := service.AssistantPropose(ctx, proposal)
	if err != nil {
		t.Fatalf("AssistantPropose() error = %v", err)
	}
	if normalized.ID != proposal.ID || !normalized.RequiresConfirmation || len(normalized.Actions) != 4 {
		t.Fatalf("normalized proposal = %+v, want confirmed four-action proposal", normalized)
	}
	if _, err := service.ApplyAssistantProposal(ctx, proposal, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("ApplyAssistantProposal(unconfirmed) error = %v, want ErrConflict", err)
	}
	applied, err := service.ApplyAssistantProposal(ctx, proposal, true)
	if err != nil {
		t.Fatalf("ApplyAssistantProposal() error = %v result=%+v", err, applied)
	}
	if !applied.Applied || applied.Status != AssistantApplyStatusApplied || applied.AppliedActionCount != 4 {
		t.Fatalf("applied result = %+v, want full apply", applied)
	}
	assignment, err := service.GetAssignment(ctx, project.ID, "asgn_first")
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if assignment.Status != AssignmentQueued || assignment.ExecutionRef != "" || assignment.ClaimedBy != "" {
		t.Fatalf("assignment = %+v, want queued coordination record without execution binding", assignment)
	}
	candidates, err := service.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != "memcand_first" {
		t.Fatalf("candidates = %+v, want proposal-created memory candidate", candidates)
	}
	reapplied, err := service.ApplyAssistantProposal(ctx, proposal, true)
	if !errors.Is(err, ErrDuplicate) || reapplied.Status != AssistantApplyStatusRejected || reapplied.AppliedActionCount != 0 || reapplied.FailedActionIndex == nil || *reapplied.FailedActionIndex != 0 {
		t.Fatalf("reapply result=%+v error=%v, want duplicate rejected at first action", reapplied, err)
	}
}

func TestService_AssistantProposalApplyProjectRootActions(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{
		Name: "Root actions",
		Roots: []Root{{
			ID:     "root_main",
			Path:   "/workspace/main",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	proposal := AssistantProposal{
		ID:        "prop_roots",
		ProjectID: project.ID,
		Title:     "Adjust project roots",
		Actions: []AssistantAction{
			{
				Kind:   AssistantActionAttachProjectRoot,
				Target: AssistantTarget{ProjectID: project.ID},
				Root: &Root{
					ID:        "root_worktree",
					Path:      "/workspace/worktree",
					Kind:      "git_worktree",
					GitBranch: "feature/root-actions",
					Active:    true,
				},
			},
			{
				Kind:    AssistantActionSetProjectDefaults,
				Project: &Project{ID: project.ID, DefaultRootID: "root_worktree"},
			},
			{
				Kind:   AssistantActionRemoveProjectRoot,
				Target: AssistantTarget{ProjectID: project.ID, RootID: "root_main"},
			},
		},
	}
	normalized, err := service.AssistantPropose(ctx, proposal)
	if err != nil {
		t.Fatalf("AssistantPropose() error = %v", err)
	}
	if normalized.ProjectID != project.ID || len(normalized.Actions) != 3 || normalized.Actions[0].Root == nil || normalized.Actions[0].Root.Path != "/workspace/worktree" {
		t.Fatalf("normalized proposal = %+v, want root action metadata", normalized)
	}
	applied, err := service.ApplyAssistantProposal(ctx, proposal, true)
	if err != nil {
		t.Fatalf("ApplyAssistantProposal() error = %v result=%+v", err, applied)
	}
	if !applied.Applied || applied.AppliedActionCount != 3 || len(applied.Actions) != 3 {
		t.Fatalf("applied = %+v, want three applied root actions", applied)
	}
	if applied.Actions[0].RootID != "root_worktree" || applied.Actions[1].RootID != "root_worktree" || applied.Actions[2].RootID != "root_main" {
		t.Fatalf("applied action refs = %+v, want root ids for attach/default/remove", applied.Actions)
	}
	updated, err := service.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if updated.DefaultRootID != "root_worktree" || len(updated.Roots) != 1 || updated.Roots[0].ID != "root_worktree" || updated.Roots[0].GitBranch != "feature/root-actions" {
		t.Fatalf("updated project = %+v, want worktree root as default after root actions", updated)
	}
	if result, err := service.ApplyAssistantProposal(ctx, AssistantProposal{
		ID:        "prop_remove_missing",
		ProjectID: project.ID,
		Title:     "Remove missing root",
		Actions: []AssistantAction{{
			Kind:   AssistantActionRemoveProjectRoot,
			Target: AssistantTarget{ProjectID: project.ID, RootID: "root_missing"},
		}},
	}, true); !errors.Is(err, ErrNotFound) || result.Status != AssistantApplyStatusRejected || result.AppliedActionCount != 0 {
		t.Fatalf("remove missing root result=%+v error=%v, want rejected ErrNotFound", result, err)
	}
}

func TestService_AssistantProposalRecordLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Assistant ledger"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	proposal := AssistantProposal{
		ID:        "prop_record",
		ProjectID: project.ID,
		Title:     "Queue reviewable work",
		Warnings:  []string{"confirm operator intent", "confirm operator intent", " "},
		Actions: []AssistantAction{
			{
				Kind: AssistantActionCreateRole,
				Role: &Role{
					ID:        "role_record",
					ProjectID: project.ID,
					Name:      "Operator",
				},
			},
			{
				Kind: AssistantActionCreateWorkItem,
				WorkItem: &WorkItem{
					ID:        "work_record",
					ProjectID: project.ID,
					Title:     "Record-backed work",
				},
			},
			{
				Kind: AssistantActionCreateAssignment,
				Assignment: &Assignment{
					ID:            "asgn_record",
					ProjectID:     project.ID,
					WorkItemID:    "work_record",
					RoleID:        "role_record",
					ExecutionMode: ExecutionMCPPull,
				},
			},
		},
	}
	record, err := service.CreateAssistantProposal(ctx, proposal)
	if err != nil {
		t.Fatalf("CreateAssistantProposal() error = %v", err)
	}
	if record.ID != proposal.ID || record.Status != AssistantProposalStatusProposed || record.ProjectID != project.ID || len(record.ApplyAttempts) != 0 {
		t.Fatalf("record = %+v, want proposed project-scoped record", record)
	}
	if len(record.Proposal.Warnings) != 1 || record.Proposal.Warnings[0] != "confirm operator intent" {
		t.Fatalf("record warnings = %+v, want compacted proposal warnings", record.Proposal.Warnings)
	}
	listed, err := service.ListAssistantProposals(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssistantProposals() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != record.ID || len(listed[0].Proposal.Warnings) != 1 {
		t.Fatalf("listed = %+v, want proposal record", listed)
	}
	needsConfirm, err := service.ApplyAssistantProposalRecord(ctx, record.ID, false)
	if !errors.Is(err, ErrConflict) || needsConfirm.Status != AssistantApplyStatusNeedsConfirm || needsConfirm.Applied {
		t.Fatalf("unconfirmed apply result=%+v error=%v, want confirmation conflict", needsConfirm, err)
	}
	afterConfirmGate, err := service.GetAssistantProposal(ctx, record.ID)
	if err != nil {
		t.Fatalf("GetAssistantProposal() after confirm gate error = %v", err)
	}
	if afterConfirmGate.Status != AssistantProposalStatusNeedsConfirm || afterConfirmGate.LatestResult == nil || len(afterConfirmGate.ApplyAttempts) != 1 {
		t.Fatalf("record after confirm gate = %+v, want one needs-confirmation attempt", afterConfirmGate)
	}
	applied, err := service.ApplyAssistantProposalRecord(ctx, record.ID, true)
	if err != nil {
		t.Fatalf("ApplyAssistantProposalRecord() error = %v result=%+v", err, applied)
	}
	if !applied.Applied || applied.Status != AssistantApplyStatusApplied || applied.AppliedActionCount != 3 {
		t.Fatalf("applied = %+v, want full record apply", applied)
	}
	afterApply, err := service.GetAssistantProposal(ctx, record.ID)
	if err != nil {
		t.Fatalf("GetAssistantProposal() after apply error = %v", err)
	}
	if afterApply.Status != AssistantProposalStatusApplied || afterApply.LatestResult == nil || !afterApply.LatestResult.Applied || len(afterApply.ApplyAttempts) != 2 || afterApply.AppliedAt == nil {
		t.Fatalf("record after apply = %+v, want applied ledger state", afterApply)
	}
	reapplied, err := service.ApplyAssistantProposalRecord(ctx, record.ID, true)
	if !errors.Is(err, ErrConflict) || !reapplied.Applied || reapplied.Status != AssistantApplyStatusApplied {
		t.Fatalf("reapply result=%+v error=%v, want already-applied conflict with latest result", reapplied, err)
	}
}

func TestService_AssistantProposalRecordCapturesPartialFailure(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Assistant partial"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	record, err := service.CreateAssistantProposal(ctx, AssistantProposal{
		ID:        "prop_partial",
		ProjectID: project.ID,
		Title:     "Partially apply",
		Actions: []AssistantAction{
			{
				Kind: AssistantActionCreateRole,
				Role: &Role{
					ID:        "role_partial",
					ProjectID: project.ID,
					Name:      "Operator",
				},
			},
			{
				Kind: AssistantActionCreateAssignment,
				Assignment: &Assignment{
					ID:            "asgn_partial",
					ProjectID:     project.ID,
					WorkItemID:    "work_missing",
					RoleID:        "role_partial",
					ExecutionMode: ExecutionMCPPull,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssistantProposal() error = %v", err)
	}
	result, err := service.ApplyAssistantProposalRecord(ctx, record.ID, true)
	if !errors.Is(err, ErrNotFound) || result.Status != AssistantApplyStatusPartial || result.AppliedActionCount != 1 || result.FailedActionIndex == nil || *result.FailedActionIndex != 1 {
		t.Fatalf("partial result=%+v error=%v, want one committed action and failed second action", result, err)
	}
	roles, err := service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() after partial apply error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != "role_partial" {
		t.Fatalf("roles after partial apply = %+v, want role_partial", roles)
	}
	if _, err := service.GetAssignment(ctx, project.ID, "asgn_partial"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAssignment() after partial apply error = %v, want ErrNotFound", err)
	}
	afterApply, err := service.GetAssistantProposal(ctx, record.ID)
	if err != nil {
		t.Fatalf("GetAssistantProposal() after partial apply error = %v", err)
	}
	if afterApply.Status != AssistantProposalStatusPartial || afterApply.LatestResult == nil || len(afterApply.ApplyAttempts) != 1 || afterApply.ApplyAttempts[0].ErrorMessage == "" {
		t.Fatalf("record after partial = %+v, want partial ledger state with error", afterApply)
	}
}

func TestService_ImportAssistantProposalRecordPreservesLedgerWithoutApplying(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Assistant import"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	appliedAt := time.Date(2026, 6, 27, 8, 15, 0, 0, time.UTC)
	record, err := service.ImportAssistantProposalRecord(ctx, AssistantProposalRecord{
		ID:        "prop_import",
		ProjectID: project.ID,
		Source:    AssistantProposalSourceAssistant,
		Proposal: AssistantProposal{
			ID:        "prop_import",
			ProjectID: project.ID,
			Title:     "Imported proposal",
			Warnings:  []string{"imported proposal warning"},
			Actions: []AssistantAction{{
				Kind: AssistantActionCreateWorkItem,
				WorkItem: &WorkItem{
					ID:        "work_imported",
					ProjectID: project.ID,
					Title:     "Should not be applied during import",
				},
			}},
		},
		Status: AssistantProposalStatusApplied,
		LatestResult: &AssistantApplyResult{
			ProposalID:         "prop_import",
			Status:             AssistantApplyStatusApplied,
			Applied:            true,
			Confirmed:          true,
			TotalActionCount:   1,
			AppliedActionCount: 1,
		},
		ApplyAttempts: []AssistantApplyAttempt{{
			ID:         "paatt_import",
			ProposalID: "prop_import",
			Status:     AssistantApplyStatusApplied,
			Confirmed:  true,
			Result: AssistantApplyResult{
				ProposalID:         "prop_import",
				Status:             AssistantApplyStatusApplied,
				Applied:            true,
				Confirmed:          true,
				TotalActionCount:   1,
				AppliedActionCount: 1,
			},
			CreatedAt: appliedAt,
		}},
		CreatedAt: appliedAt.Add(-time.Hour),
		UpdatedAt: appliedAt,
		AppliedAt: &appliedAt,
	})
	if err != nil {
		t.Fatalf("ImportAssistantProposalRecord() error = %v", err)
	}
	if record.Status != AssistantProposalStatusApplied || record.LatestResult == nil || len(record.ApplyAttempts) != 1 || record.AppliedAt == nil {
		t.Fatalf("imported record = %+v, want applied ledger state", record)
	}
	if len(record.Proposal.Warnings) != 1 || record.Proposal.Warnings[0] != "imported proposal warning" {
		t.Fatalf("imported warnings = %+v, want imported warnings preserved", record.Proposal.Warnings)
	}
	if _, err := service.GetWorkItem(ctx, project.ID, "work_imported"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkItem() after import error = %v, want ErrNotFound because import must not apply actions", err)
	}
	if result, err := service.ApplyAssistantProposalRecord(ctx, record.ID, true); !errors.Is(err, ErrConflict) || !result.Applied {
		t.Fatalf("ApplyAssistantProposalRecord() after imported applied record result=%+v error=%v, want replay conflict with latest result", result, err)
	}
	record.Status = AssistantProposalStatusRejected
	record.LatestResult = &AssistantApplyResult{
		ProposalID:       record.ID,
		Status:           AssistantApplyStatusRejected,
		TotalActionCount: 1,
	}
	record.AppliedAt = nil
	updated, err := service.ImportAssistantProposalRecord(ctx, record)
	if err != nil {
		t.Fatalf("ImportAssistantProposalRecord(update) error = %v", err)
	}
	if updated.Status != AssistantProposalStatusRejected || updated.AppliedAt != nil || updated.LatestResult == nil || updated.LatestResult.Status != AssistantApplyStatusRejected {
		t.Fatalf("updated imported record = %+v, want rejected imported state", updated)
	}
}

func TestService_AssistantProposalRejectsExecutionBoundAssignment(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Assistant safety"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	proposal := AssistantProposal{
		Title: "Unsafe assignment",
		Actions: []AssistantAction{{
			Kind: AssistantActionCreateAssignment,
			Assignment: &Assignment{
				ProjectID:     project.ID,
				WorkItemID:    "work_missing",
				RoleID:        "role_missing",
				Status:        AssignmentRunning,
				ExecutionRef:  "run_unsafe",
				ExecutionMode: ExecutionMCPPull,
			},
		}},
	}
	if _, err := service.AssistantPropose(ctx, proposal); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AssistantPropose(unsafe assignment) error = %v, want ErrInvalid", err)
	}
}

func TestService_ProjectActivity(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Activity"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Track assignments"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	queued, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(queued) error = %v", err)
	}
	claimed, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(claimed) error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, claimed.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	running, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(running) error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, running.ID, "agent-b"); err != nil {
		t.Fatalf("ClaimAssignment(running) error = %v", err)
	}
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, running.ID, AssignmentRunning, "run-1"); err != nil {
		t.Fatalf("UpdateAssignmentStatus(running) error = %v", err)
	}
	completed, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(completed) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, completed.ID, AssignmentCompleted, "run-2"); err != nil {
		t.Fatalf("CompleteAssignment(completed) error = %v", err)
	}
	failed, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(failed) error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, failed.ID, AssignmentFailed, "run-3"); err != nil {
		t.Fatalf("CompleteAssignment(failed) error = %v", err)
	}

	activity, err := service.ProjectActivity(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectActivity() error = %v", err)
	}
	if activity.Counts.Assignments != 5 || activity.Counts.Queued != 1 || activity.Counts.Claimed != 1 || activity.Counts.Running != 1 || activity.Counts.Completed != 1 || activity.Counts.Failed != 1 {
		t.Fatalf("activity counts = %+v, want status counts", activity.Counts)
	}
	if activity.Counts.Active != 2 || activity.Counts.Blocked != 2 || len(activity.Buckets.Active) != 2 || len(activity.Buckets.Blocked) != 2 || len(activity.Buckets.Completed) != 1 || len(activity.Buckets.Recent) != 5 {
		t.Fatalf("activity buckets = counts %+v buckets %+v, want active/blocked/completed/recent", activity.Counts, activity.Buckets)
	}
	if !containsActivity(activity.Items, ProjectActivityBucketBlocked, queued.ID, work.ID, role.Name) ||
		!containsActivity(activity.Items, ProjectActivityBucketActive, claimed.ID, work.ID, role.Name) ||
		!containsActivity(activity.Items, ProjectActivityBucketActive, running.ID, work.ID, role.Name) ||
		!containsActivity(activity.Items, ProjectActivityBucketCompleted, completed.ID, work.ID, role.Name) ||
		!containsActivity(activity.Items, ProjectActivityBucketBlocked, failed.ID, work.ID, role.Name) {
		t.Fatalf("activity items = %+v, want resolved work and role metadata", activity.Items)
	}
}

func TestService_MemoryCandidateDecisionLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Candidate decisions"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	promoteCandidate, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ProjectID:           project.ID,
		Title:               "Generated review lesson",
		Body:                "Review handoffs should cite concrete evidence.",
		SuggestedKind:       "note",
		SuggestedTrustLabel: MemoryTrustGenerated,
		SuggestedSourceKind: MemorySourceGenerated,
		SuggestedSourceID:   "run_1",
		SourceRefs: []MemoryCandidateSourceRef{{
			Kind:  "task_run",
			ID:    "run_1",
			Title: "Task run",
		}},
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(promote) error = %v", err)
	}
	if promoteCandidate.Status != MemoryCandidatePending || len(promoteCandidate.SourceRefs) != 1 {
		t.Fatalf("promote candidate = %+v, want pending candidate with source ref", promoteCandidate)
	}
	overriddenTitle := "Reviewed review lesson"
	trust := MemoryTrustOperator
	sourceKind := MemorySourceOperator
	promoted, entry, err := service.PromoteMemoryCandidate(ctx, MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: promoteCandidate.ID,
		Title:       &overriddenTitle,
		TrustLabel:  &trust,
		SourceKind:  &sourceKind,
	})
	if err != nil {
		t.Fatalf("PromoteMemoryCandidate() error = %v", err)
	}
	if promoted.Status != MemoryCandidatePromoted || promoted.PromotedMemoryID != entry.ID {
		t.Fatalf("promoted candidate = %+v entry=%+v, want promoted candidate linked to entry", promoted, entry)
	}
	if entry.Title != overriddenTitle || entry.TrustLabel != MemoryTrustOperator || !entry.Enabled {
		t.Fatalf("promoted entry = %+v, want override title/operator trust/enabled", entry)
	}
	retried, retriedEntry, err := service.PromoteMemoryCandidate(ctx, MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: promoteCandidate.ID,
	})
	if err != nil {
		t.Fatalf("PromoteMemoryCandidate(retry) error = %v", err)
	}
	if retried.PromotedMemoryID != entry.ID || retriedEntry.ID != entry.ID {
		t.Fatalf("retried promote candidate=%+v entry=%+v, want idempotent promoted memory %s", retried, retriedEntry, entry.ID)
	}

	rejectCandidate, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ProjectID: project.ID,
		Title:     "Speculative convention",
		Body:      "Maybe skip all tests.",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(reject) error = %v", err)
	}
	rejected, err := service.RejectMemoryCandidate(ctx, project.ID, rejectCandidate.ID, "Not durable project knowledge.")
	if err != nil {
		t.Fatalf("RejectMemoryCandidate() error = %v", err)
	}
	if rejected.Status != MemoryCandidateRejected || rejected.StatusReason != "Not durable project knowledge." {
		t.Fatalf("rejected candidate = %+v, want rejected reason", rejected)
	}
	if _, _, err := service.PromoteMemoryCandidate(ctx, MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: rejectCandidate.ID,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("PromoteMemoryCandidate(rejected) error = %v, want ErrConflict", err)
	}

	pending, err := service.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(pending) error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending candidates = %+v, want resolved candidates omitted", pending)
	}
	all, err := service.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID, IncludeResolved: true})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(all) error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all candidates = %+v, want promoted and rejected candidates", all)
	}
	if err := service.DeleteMemoryCandidate(ctx, project.ID, rejectCandidate.ID); err != nil {
		t.Fatalf("DeleteMemoryCandidate() error = %v", err)
	}
	if _, err := service.GetMemoryCandidate(ctx, project.ID, rejectCandidate.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMemoryCandidate(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestService_MemoryEntryImportPreservesTimestamps(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	service.now = func() time.Time {
		return time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	}
	project, err := service.CreateProject(ctx, Project{Name: "Imported memory"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	createdAt := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 21, 9, 30, 0, 0, time.UTC)

	entry, err := service.CreateMemoryEntry(ctx, MemoryEntry{
		ID:         "mem_imported",
		ProjectID:  project.ID,
		Title:      "Imported memory",
		Body:       "Memory mirrored from an existing coordination store.",
		TrustLabel: MemoryTrustOperator,
		SourceKind: MemorySourceOperator,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if !entry.CreatedAt.Equal(createdAt) || !entry.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("entry timestamps = created:%s updated:%s, want imported timestamps", entry.CreatedAt, entry.UpdatedAt)
	}
	if !entry.Enabled {
		t.Fatalf("entry = %+v, want ordinary create default enabled", entry)
	}

	importedUpdateAt := time.Date(2026, 6, 22, 11, 0, 0, 0, time.UTC)
	entry.Enabled = false
	entry.UpdatedAt = importedUpdateAt
	updated, err := service.UpdateMemoryEntry(ctx, entry)
	if err != nil {
		t.Fatalf("UpdateMemoryEntry() error = %v", err)
	}
	if updated.Enabled || !updated.CreatedAt.Equal(createdAt) || !updated.UpdatedAt.Equal(importedUpdateAt) {
		t.Fatalf("updated entry = %+v, want disabled entry preserving imported timestamps", updated)
	}
}

func TestService_MemoryCandidateImportPreservesTimestampsAndResolvedState(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	service.now = func() time.Time {
		return time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	}
	project, err := service.CreateProject(ctx, Project{Name: "Imported memory candidates"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	createdAt := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 21, 9, 30, 0, 0, time.UTC)

	candidate, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ID:                  "memcand_imported",
		ProjectID:           project.ID,
		Title:               "Imported candidate",
		Body:                "Candidate mirrored from an existing coordination store.",
		SuggestedKind:       "project_pattern",
		SuggestedTrustLabel: MemoryTrustGenerated,
		SuggestedSourceKind: MemorySourceGenerated,
		SourceRefs: []MemoryCandidateSourceRef{{
			Kind:  "handoff",
			ID:    "handoff_import",
			Title: "Imported handoff",
		}},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	if candidate.Status != MemoryCandidatePending || !candidate.CreatedAt.Equal(createdAt) || !candidate.UpdatedAt.Equal(updatedAt) || len(candidate.SourceRefs) != 1 {
		t.Fatalf("candidate = %+v, want pending imported metadata and timestamps", candidate)
	}

	importedUpdateAt := time.Date(2026, 6, 22, 11, 0, 0, 0, time.UTC)
	candidate.Status = MemoryCandidateRejected
	candidate.StatusReason = "Not durable enough."
	candidate.PromotedMemoryID = ""
	candidate.SuggestedKind = ""
	candidate.SourceRefs = nil
	candidate.UpdatedAt = importedUpdateAt
	updated, err := service.UpdateMemoryCandidate(ctx, candidate)
	if err != nil {
		t.Fatalf("UpdateMemoryCandidate() error = %v", err)
	}
	if updated.Status != MemoryCandidateRejected || updated.StatusReason != "Not durable enough." || updated.PromotedMemoryID != "" {
		t.Fatalf("updated candidate = %+v, want imported rejected state", updated)
	}
	if !updated.CreatedAt.Equal(createdAt) || !updated.UpdatedAt.Equal(importedUpdateAt) {
		t.Fatalf("updated candidate timestamps = created:%s updated:%s, want imported timestamps", updated.CreatedAt, updated.UpdatedAt)
	}
	if updated.SuggestedKind != "" || len(updated.SourceRefs) != 0 {
		t.Fatalf("updated candidate provenance = kind:%q refs:%+v, want imported cleared provenance", updated.SuggestedKind, updated.SourceRefs)
	}
}

func TestService_MemoryListsUseHecateCompatibleOrder(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Memory order"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	base := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	enabled, err := service.CreateMemoryEntry(ctx, MemoryEntry{
		ID:         "mem_enabled",
		ProjectID:  project.ID,
		Title:      "Enabled memory",
		Body:       "Enabled entries sort before disabled entries.",
		TrustLabel: MemoryTrustOperator,
		SourceKind: MemorySourceOperator,
		CreatedAt:  base,
		UpdatedAt:  base,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry(enabled) error = %v", err)
	}
	disabled, err := service.CreateMemoryEntry(ctx, MemoryEntry{
		ID:         "mem_disabled",
		ProjectID:  project.ID,
		Title:      "Disabled memory",
		Body:       "Disabled entries remain inspectable but sort after enabled entries.",
		TrustLabel: MemoryTrustOperator,
		SourceKind: MemorySourceOperator,
		CreatedAt:  base.Add(time.Minute),
		UpdatedAt:  base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry(disabled) error = %v", err)
	}
	disabled.Enabled = false
	disabled.UpdatedAt = base.Add(2 * time.Minute)
	if _, err := service.UpdateMemoryEntry(ctx, disabled); err != nil {
		t.Fatalf("UpdateMemoryEntry(disabled) error = %v", err)
	}
	entries, err := service.ListMemoryEntries(ctx, project.ID, true)
	if err != nil {
		t.Fatalf("ListMemoryEntries() error = %v", err)
	}
	if len(entries) != 2 || entries[0].ID != enabled.ID || entries[1].ID != disabled.ID {
		t.Fatalf("entries = %+v, want enabled memory before newer disabled memory", entries)
	}

	pending, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ID:                  "memcand_pending",
		ProjectID:           project.ID,
		Title:               "Pending candidate",
		Body:                "Pending candidates need operator review.",
		SuggestedTrustLabel: MemoryTrustGenerated,
		SuggestedSourceKind: MemorySourceGenerated,
		CreatedAt:           base,
		UpdatedAt:           base,
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(pending) error = %v", err)
	}
	rejected, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ID:                  "memcand_rejected",
		ProjectID:           project.ID,
		Title:               "Rejected candidate",
		Body:                "Resolved candidates sort after pending candidates.",
		SuggestedTrustLabel: MemoryTrustGenerated,
		SuggestedSourceKind: MemorySourceGenerated,
		CreatedAt:           base.Add(time.Minute),
		UpdatedAt:           base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(rejected) error = %v", err)
	}
	rejected.Status = MemoryCandidateRejected
	rejected.StatusReason = "Not durable."
	rejected.UpdatedAt = base.Add(2 * time.Minute)
	if _, err := service.UpdateMemoryCandidate(ctx, rejected); err != nil {
		t.Fatalf("UpdateMemoryCandidate(rejected) error = %v", err)
	}
	candidates, err := service.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID, IncludeResolved: true})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 2 || candidates[0].ID != pending.ID || candidates[1].ID != rejected.ID {
		t.Fatalf("candidates = %+v, want pending candidate before newer rejected candidate", candidates)
	}
}

func TestService_MemoryEntryLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Memory flow"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	entry, err := service.CreateMemoryEntry(ctx, MemoryEntry{
		ProjectID:  project.ID,
		Title:      "Keep reviews tight",
		Body:       "Review notes should cite evidence.",
		SourceKind: MemorySourceGenerated,
		SourceID:   "handoff_1",
		TrustLabel: MemoryTrustGenerated,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if entry.ID == "" || !entry.Enabled || entry.TrustLabel != MemoryTrustGenerated {
		t.Fatalf("entry = %+v, want generated id and enabled generated memory", entry)
	}
	got, err := service.GetMemoryEntry(ctx, project.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetMemoryEntry() error = %v", err)
	}
	if got.ID != entry.ID || got.SourceID != "handoff_1" {
		t.Fatalf("got memory = %+v, want created entry", got)
	}
	got.Title = "Keep reviews concrete"
	got.Body = "Review notes should cite exact evidence."
	got.Enabled = false
	updated, err := service.UpdateMemoryEntry(ctx, got)
	if err != nil {
		t.Fatalf("UpdateMemoryEntry() error = %v", err)
	}
	if updated.Title != "Keep reviews concrete" || updated.Enabled {
		t.Fatalf("updated memory = %+v, want disabled replacement metadata", updated)
	}
	enabledOnly, err := service.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries(enabled) error = %v", err)
	}
	if len(enabledOnly) != 0 {
		t.Fatalf("enabled memory = %+v, want disabled entry omitted", enabledOnly)
	}
	allEntries, err := service.ListMemoryEntries(ctx, project.ID, true)
	if err != nil {
		t.Fatalf("ListMemoryEntries(all) error = %v", err)
	}
	if len(allEntries) != 1 || allEntries[0].ID != entry.ID {
		t.Fatalf("all memory = %+v, want disabled entry included", allEntries)
	}
	if _, err := service.UpdateMemoryEntry(ctx, MemoryEntry{
		ProjectID: project.ID,
		ID:        entry.ID,
		Title:     "",
		Body:      "Missing title.",
		Enabled:   true,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateMemoryEntry(invalid) error = %v, want ErrInvalid", err)
	}
	if err := service.DeleteMemoryEntry(ctx, project.ID, entry.ID); err != nil {
		t.Fatalf("DeleteMemoryEntry() error = %v", err)
	}
	if _, err := service.GetMemoryEntry(ctx, project.ID, entry.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMemoryEntry(deleted) error = %v, want ErrNotFound", err)
	}
	if err := service.DeleteMemoryEntry(ctx, project.ID, entry.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteMemoryEntry(second) error = %v, want ErrNotFound", err)
	}
}

func TestService_ListCompatibleAssignmentsFiltersQueuedWork(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Queue"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Implement backend"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	match, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:     project.ID,
		WorkItemID:    work.ID,
		RoleID:        role.ID,
		ExecutionMode: ExecutionMCPPull,
		DesiredAgent: DesiredAgent{
			Kind:     "codex",
			SkillIDs: []string{"backend"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() match error = %v", err)
	}
	if _, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:     project.ID,
		WorkItemID:    work.ID,
		RoleID:        role.ID,
		ExecutionMode: ExecutionMCPPull,
		DesiredAgent: DesiredAgent{
			Kind:     "claude",
			SkillIDs: []string{"backend"},
		},
	}); err != nil {
		t.Fatalf("CreateAssignment() kind mismatch error = %v", err)
	}
	if _, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:     project.ID,
		WorkItemID:    work.ID,
		RoleID:        role.ID,
		ExecutionMode: ExecutionMCPPull,
		DesiredAgent: DesiredAgent{
			Kind:     "codex",
			SkillIDs: []string{"frontend"},
		},
	}); err != nil {
		t.Fatalf("CreateAssignment() skill mismatch error = %v", err)
	}

	items, err := service.ListCompatibleAssignments(ctx, project.ID, AssignmentCompatibilityFilter{
		AgentKind:    "codex",
		SkillIDs:     []string{"backend", "review"},
		FilterSkills: true,
	})
	if err != nil {
		t.Fatalf("ListCompatibleAssignments() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != match.ID {
		t.Fatalf("compatible assignments = %+v, want only matching assignment", items)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, match.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	items, err = service.ListCompatibleAssignments(ctx, project.ID, AssignmentCompatibilityFilter{
		AgentKind:    "codex",
		SkillIDs:     []string{"backend"},
		FilterSkills: true,
	})
	if err != nil {
		t.Fatalf("ListCompatibleAssignments() after claim error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("compatible assignments after claim = %+v, want none", items)
	}
}

func writeSkill(t *testing.T, root, base, id, body string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(base), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeProjectTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func findProjectSkillForTest(skills []ProjectSkill, id string) *ProjectSkill {
	for index := range skills {
		if skills[index].ID == id {
			return &skills[index]
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func reviewIDExistsForTest(values []Review, want string) bool {
	for _, value := range values {
		if value.ID == want {
			return true
		}
	}
	return false
}

func containsOperation(items []ProjectOperationItem, kind, workItemID, refID string) bool {
	for _, item := range items {
		if item.Kind != kind || item.WorkItemID != workItemID {
			continue
		}
		if refID == "" || item.AssignmentID == refID || item.ArtifactID == refID || item.MemoryCandidateID == refID {
			return true
		}
	}
	return false
}

func setupCheckByID(checks []ProjectSetupReadinessCheck, id string) ProjectSetupReadinessCheck {
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	return ProjectSetupReadinessCheck{}
}

func containsHealthAttention(items []ProjectHealthAttentionItem, kind, refID string) bool {
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if refID == "" || item.AssignmentID == refID || item.ArtifactID == refID || item.HandoffID == refID || item.MemoryCandidateID == refID {
			return true
		}
	}
	return false
}

func containsHealthAttentionDetail(items []ProjectHealthAttentionItem, kind, detail string) bool {
	for _, item := range items {
		if item.Kind == kind && strings.Contains(item.Detail, detail) {
			return true
		}
	}
	return false
}

func containsActivity(items []ProjectActivityItem, bucket, assignmentID, workItemID, roleName string) bool {
	for _, item := range items {
		if item.Bucket == bucket && item.AssignmentID == assignmentID && item.WorkItemID == workItemID && item.RoleName == roleName {
			return true
		}
	}
	return false
}

func TestService_ClaimAssignmentRaceHasOneWinner(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Race"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Claim once"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	const contenders = 20
	var wins atomic.Int32
	var conflicts atomic.Int32
	var wg sync.WaitGroup
	for i := range contenders {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-"+string(rune('a'+i)))
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("ClaimAssignment() unexpected error = %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != contenders-1 {
		t.Fatalf("wins=%d conflicts=%d, want one winner and %d conflicts", wins.Load(), conflicts.Load(), contenders-1)
	}
}

func TestService_ReleaseAssignmentClearsClaimForRetry(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Release"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Retry start"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	claimed, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a")
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	startedAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(time.Minute)
	claimed, err = service.UpdateAssignment(ctx, Assignment{
		ProjectID:         project.ID,
		ID:                assignment.ID,
		WorkItemID:        work.ID,
		RoleID:            role.ID,
		ExecutionMode:     ExecutionMCPPull,
		Status:            AssignmentClaimed,
		ClaimedBy:         claimed.ClaimedBy,
		ExecutionRef:      "run-pre-dispatch",
		ContextSnapshotID: "ctx-pre-dispatch",
		StartedAt:         startedAt,
		CompletedAt:       completedAt,
	})
	if err != nil {
		t.Fatalf("UpdateAssignment(claim metadata) error = %v", err)
	}
	if _, err := service.ReleaseAssignment(ctx, project.ID, assignment.ID, "agent-b"); !errors.Is(err, ErrConflict) {
		t.Fatalf("ReleaseAssignment(wrong claimer) error = %v, want ErrConflict", err)
	}
	released, err := service.ReleaseAssignment(ctx, project.ID, assignment.ID, "agent-a")
	if err != nil {
		t.Fatalf("ReleaseAssignment() error = %v", err)
	}
	if released.Status != AssignmentQueued || released.ClaimedBy != "" || released.ExecutionRef != "" || released.ContextSnapshotID != "" || !released.StartedAt.IsZero() || !released.CompletedAt.IsZero() {
		t.Fatalf("released assignment = %+v, want queued with claim/runtime refs cleared", released)
	}
	reclaimed, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-b")
	if err != nil {
		t.Fatalf("ClaimAssignment(after release) error = %v", err)
	}
	if reclaimed.Status != AssignmentClaimed || reclaimed.ClaimedBy != "agent-b" {
		t.Fatalf("reclaimed assignment = %+v, want claimed by agent-b", reclaimed)
	}
	running, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, AssignmentRunning, "run-started")
	if err != nil {
		t.Fatalf("UpdateAssignmentStatus(running) error = %v", err)
	}
	if _, err := service.ReleaseAssignment(ctx, project.ID, assignment.ID, "agent-b"); !errors.Is(err, ErrConflict) {
		t.Fatalf("ReleaseAssignment(running) error = %v, want ErrConflict", err)
	}
	if running.Status != AssignmentRunning || running.ExecutionRef != "run-started" {
		t.Fatalf("running assignment = %+v, want running state preserved", running)
	}
}

func TestService_CreateAssignmentValidatesRole(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Validation"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Needs role"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	_, err = service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     "role_missing",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateAssignment() error = %v, want ErrNotFound", err)
	}
}

func TestService_ValidatesProjectRootReferences(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{
		Name: "Roots",
		Roots: []Root{{
			ID:     "root_main",
			Path:   "/workspace/main",
			Kind:   "local",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := service.CreateProject(ctx, Project{
		Name:          "Broken default root",
		DefaultRootID: "root_missing",
		Roots: []Root{{
			ID:   "root_main",
			Path: "/workspace/main",
		}},
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateProject(missing default root) error = %v, want ErrNotFound", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Use valid root",
		RootID:    "root_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() valid root error = %v", err)
	}
	if work.RootID != "root_main" {
		t.Fatalf("work root = %q, want root_main", work.RootID)
	}
	if _, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Use missing root",
		RootID:    "root_missing",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateWorkItem() missing root error = %v, want ErrNotFound", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
		RootID:     "root_main",
	})
	if err != nil {
		t.Fatalf("CreateAssignment() valid root error = %v", err)
	}
	if assignment.RootID != "root_main" {
		t.Fatalf("assignment root = %q, want root_main", assignment.RootID)
	}
	if _, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
		RootID:     "root_missing",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateAssignment() missing root error = %v, want ErrNotFound", err)
	}
}

func TestService_CreateReviewValidatesVerdict(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, err := service.CreateProject(ctx, Project{Name: "Reviews"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Review me"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	_, err = service.CreateReview(ctx, Review{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		Body:       "Missing verdict.",
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateReview() error = %v, want ErrInvalid", err)
	}
}
