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
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
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
		RootID:          "root_feature",
	})
	if err != nil {
		t.Fatalf("UpdateWorkItem() error = %v", err)
	}
	if updatedWork.Title != "Updated work" || updatedWork.OwnerRoleID != role.ID || len(updatedWork.ReviewerRoleIDs) != 1 || updatedWork.RootID != "root_feature" {
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
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Test output",
		Locator:      "file://report.md",
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
	if len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 1 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}
	if launchPacket.Evidence[0].AssignmentID != assignment.ID {
		t.Fatalf("launch packet evidence = %+v, want assignment-scoped evidence", launchPacket.Evidence[0])
	}

	completed, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, AssignmentCompleted, "run-123")
	if err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	if completed.Status != AssignmentCompleted || completed.ExecutionRef != "run-123" {
		t.Fatalf("completed assignment = %+v, want completed with execution ref", completed)
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
		Verdict:      ReviewVerdictConcerns,
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
		Verdict:      ReviewVerdictConcerns,
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
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
