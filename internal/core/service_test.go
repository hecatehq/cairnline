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
)

func TestService_CreateRootlessProjectAndWorkItem(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	project, err := service.CreateProject(ctx, Project{
		Name:        "Research notes",
		Description: "Coordinate interview synthesis.",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.ID == "" || len(project.Roots) != 0 {
		t.Fatalf("project = %+v, want generated id and no roots", project)
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

func TestService_GetAndDeleteProjectCascadesProjectScopedState(t *testing.T) {
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
	got, err := service.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if got.ID != project.ID || got.Name != "Delete me" {
		t.Fatalf("GetProject() = %+v, want created project", got)
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
	if _, err := service.CreateEvidence(ctx, Evidence{ProjectID: project.ID, WorkItemID: work.ID, Title: "Evidence", Locator: "https://example.test/evidence"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Body: "Pass", Verdict: ReviewVerdictPass}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{ProjectID: project.ID, WorkItemID: work.ID, Title: "Handoff", Body: "Follow up"}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{ProjectID: project.ID, Title: "Memory", Body: "Remember this"}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	if err := service.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := service.GetProject(ctx, project.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProject() after delete error = %v, want ErrNotFound", err)
	}
	assertGone := func(name string, count int, err error) {
		t.Helper()
		if errors.Is(err, ErrNotFound) {
			return
		}
		if err != nil {
			t.Fatalf("%s list error = %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0 after project delete", name, count)
		}
	}
	skills, err := service.ListProjectSkills(ctx, project.ID)
	assertGone("skills", len(skills), err)
	roles, err := service.ListRoles(ctx, project.ID)
	assertGone("roles", len(roles), err)
	workItems, err := service.ListWorkItems(ctx, project.ID)
	assertGone("work items", len(workItems), err)
	assignments, err := service.ListAssignments(ctx, project.ID)
	assertGone("assignments", len(assignments), err)
	evidence, err := service.ListEvidence(ctx, project.ID, work.ID)
	assertGone("evidence", len(evidence), err)
	reviews, err := service.ListReviews(ctx, project.ID, work.ID)
	assertGone("reviews", len(reviews), err)
	handoffs, err := service.ListHandoffs(ctx, project.ID, work.ID)
	assertGone("handoffs", len(handoffs), err)
	memory, err := service.ListMemoryCandidates(ctx, project.ID)
	assertGone("memory candidates", len(memory), err)

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
	if err := service.DeleteProject(ctx, project.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteProject() second error = %v, want ErrNotFound", err)
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
}

func TestService_ProjectSkillsDiscoveryAndLaunchResolution(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, SkillPathAgents, "backend", `---
name: Backend implementer
description: Work on backend code with tests.
---
# Backend skill

Body should not be stored in the skill registry.
`)

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
	if len(skills) != 1 {
		t.Fatalf("skills = %+v, want one discovered skill", skills)
	}
	skill := skills[0]
	if skill.ID != "backend" || skill.Title != "Backend implementer" || skill.Description != "Work on backend code with tests." || skill.Path != ".agents/skills/backend/SKILL.md" {
		t.Fatalf("skill = %+v, want parsed metadata and relative path", skill)
	}
	if strings.Contains(skill.Description, "Body should not be stored") {
		t.Fatalf("skill description includes body content: %+v", skill)
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
	if _, err := service.UpdateProjectSkill(ctx, skill); err != nil {
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

func TestService_ProjectSkillsDiscoveryMarksConflicts(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	root := t.TempDir()
	writeSkill(t, root, SkillPathAgents, "backend", "# Backend\n")
	writeSkill(t, root, SkillPathCairnline, "backend", "# Backend duplicate\n")
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
	if updatedProject.CreatedAt.IsZero() || updatedProject.UpdatedAt.Before(updatedProject.CreatedAt) {
		t.Fatalf("updated project timestamps = created %s updated %s", updatedProject.CreatedAt, updatedProject.UpdatedAt)
	}

	profile, err := service.CreateAgentProfile(ctx, AgentProfile{Name: "Default profile"})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ProjectID:        project.ID,
		Name:             "Implementer",
		DefaultProfileID: profile.ID,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	updatedRole, err := service.UpdateRole(ctx, Role{
		ProjectID:            project.ID,
		ID:                   role.ID,
		Name:                 "Senior implementer",
		Description:          "Owns implementation.",
		DefaultProfileID:     profile.ID,
		DefaultSkillIDs:      []string{"backend", "backend"},
		DefaultExecutionMode: ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if updatedRole.Name != "Senior implementer" || len(updatedRole.DefaultSkillIDs) != 1 || updatedRole.DefaultExecutionMode != ExecutionMCPPull {
		t.Fatalf("updated role = %+v, want replacement defaults", updatedRole)
	}

	work, err := service.CreateWorkItem(ctx, WorkItem{
		ProjectID: project.ID,
		Title:     "Original work",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
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
		RootID:          "root_main",
	})
	if err != nil {
		t.Fatalf("UpdateWorkItem() error = %v", err)
	}
	if updatedWork.Title != "Updated work" || updatedWork.OwnerRoleID != role.ID || len(updatedWork.ReviewerRoleIDs) != 1 || updatedWork.RootID != "root_main" {
		t.Fatalf("updated work item = %+v, want replacement metadata", updatedWork)
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
		ProjectID:        project.ID,
		Name:             "Reviewer",
		Instructions:     "Check the evidence and record blockers.",
		DefaultProfileID: profile.ID,
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
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		RoleID:             role.ID,
		ExecutionProfileID: executionProfile.ID,
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

	packet, err := service.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() error = %v", err)
	}
	if packet.Project.ID != project.ID || packet.WorkItem.ID != work.ID || packet.Role == nil || packet.Role.ID != role.ID {
		t.Fatalf("assignment context = %+v, want project/work/role metadata", packet)
	}

	evidence, err := service.CreateEvidence(ctx, Evidence{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		Title:      "Test output",
		Locator:    "file://report.md",
	})
	if err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if evidence.TrustLabel != EvidenceTrustOperator {
		t.Fatalf("evidence = %+v, want default trust label", evidence)
	}
	review, err := service.CreateReview(ctx, Review{
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		ReviewerRoleID: role.ID,
		Body:           "Looks good with one note.",
		Verdict:        ReviewVerdictPass,
		Risk:           ReviewRiskLow,
	})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if review.Title != "Review" || review.Status != ReviewStatusRecorded {
		t.Fatalf("review = %+v, want default title and recorded status", review)
	}
	handoff, err := service.CreateHandoff(ctx, Handoff{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		FromRoleID: role.ID,
		ToRoleID:   role.ID,
		Title:      "Follow-up",
		Body:       "Carry this into the next pass.",
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if handoff.Status != HandoffStatusOpen {
		t.Fatalf("handoff = %+v, want open status", handoff)
	}
	memory, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ProjectID: project.ID,
		Title:     "Project convention",
		Body:      "Reviews should include concrete evidence.",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	if memory.Status != MemoryCandidateProposed {
		t.Fatalf("memory candidate = %+v, want proposed status", memory)
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
	if len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 1 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}

	completed, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, AssignmentCompleted, "run-123")
	if err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	if completed.Status != AssignmentCompleted || completed.ExecutionRef != "run-123" {
		t.Fatalf("completed assignment = %+v, want completed with execution ref", completed)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
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
