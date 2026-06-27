package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hecatehq/cairnline/internal/core"
)

func TestStore_PersistsAssignmentLifecycle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cairnline.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	profile, err := service.CreateAgentProfile(ctx, core.AgentProfile{
		Name:          "Reviewer profile",
		Instructions:  "Review persisted context.",
		ContextPolicy: "include_enabled",
		SkillIDs:      []string{"review"},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	executionProfile, err := service.CreateExecutionProfile(ctx, core.ExecutionProfile{
		Name:           "SQLite execution",
		AgentKind:      "any",
		ModelHint:      "local",
		ProviderHint:   "local",
		ToolsPolicy:    "readonly",
		WritesPolicy:   "block",
		NetworkPolicy:  "block",
		ApprovalPolicy: "require",
		AdapterOptions: map[string]any{"mode": "test"},
	})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, core.Project{
		Name:                      "Persistent project",
		Description:               "Survives process restart.",
		DefaultRootID:             "root_main",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
		Roots: []core.Root{{
			ID:     "root_main",
			Path:   "/tmp/example",
			Kind:   "workspace",
			Active: true,
		}},
		ContextSources: []core.Source{{
			ID:             "src_agents",
			Kind:           "workspace_instruction",
			Title:          "AGENTS.md",
			Locator:        "AGENTS.md",
			Enabled:        true,
			Format:         "agents_md",
			Scope:          "workspace",
			TrustLabel:     "workspace_guidance",
			SourceCategory: "instructions",
			Metadata:       map[string]string{"root_id": "root_main"},
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skill, err := service.CreateProjectSkill(ctx, core.ProjectSkill{
		ProjectID:   project.ID,
		ID:          "review",
		Title:       "Review skill",
		Description: "Review work with evidence.",
		Format:      core.SkillFormatMarkdown,
		Status:      core.SkillStatusAvailable,
		TrustLabel:  core.SkillTrustWorkspace,
		SourceRefs:  []string{"src_agents"},
	})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:                 project.ID,
		Name:                      "Reviewer",
		Instructions:              "Review the durable trail.",
		DefaultProfileID:          profile.ID,
		DefaultExecutionProfileID: executionProfile.ID,
		DefaultSkillIDs:           []string{"review", "evidence"},
		DefaultExecutionMode:      core.ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{
		ProjectID:       project.ID,
		Title:           "Check persistence",
		Brief:           "Create, claim, complete, reopen.",
		ReviewerRoleIDs: []string{role.ID},
		RootID:          "root_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		RoleID:             role.ID,
		RootID:             "root_main",
		ExecutionProfileID: executionProfile.ID,
		ExecutionMode:      core.ExecutionMCPPull,
		DesiredAgent: core.DesiredAgent{
			Kind:     "any",
			SkillIDs: []string{"review"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	assignment, err = service.UpdateAssignment(ctx, core.Assignment{
		ProjectID:          project.ID,
		ID:                 assignment.ID,
		WorkItemID:         work.ID,
		RoleID:             role.ID,
		RootID:             "root_main",
		ProfileID:          profile.ID,
		ExecutionProfileID: executionProfile.ID,
		ExecutionMode:      core.ExecutionMCPPull,
		Status:             core.AssignmentQueued,
		DesiredAgent: core.DesiredAgent{
			Kind:     "any",
			SkillIDs: []string{"review", "evidence"},
		},
		ContextSnapshotID: "ctx-prep",
	})
	if err != nil {
		t.Fatalf("UpdateAssignment() error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	artifactRecord, err := service.CreateArtifact(ctx, core.Artifact{
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		Kind:           "decision_note",
		Title:          "Decision note",
		Body:           "Persist generic collaboration artifacts.",
		AuthorRoleID:   role.ID,
		ProvenanceKind: "operator",
		TrustLabel:     "operator_reviewed",
	})
	if err != nil {
		t.Fatalf("CreateArtifact() error = %v", err)
	}
	evidenceRecord, err := service.CreateEvidence(ctx, core.Evidence{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Output link",
		Locator:      "https://example.test/report",
	})
	if err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	reviewRecord, err := service.CreateReview(ctx, core.Review{
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		ReviewerRoleID: role.ID,
		Title:          "Review pass",
		Body:           "The persistence flow works.",
		Verdict:        core.ReviewVerdictPass,
		Risk:           core.ReviewRiskLow,
	})
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, core.Handoff{
		ProjectID:             project.ID,
		WorkItemID:            work.ID,
		SourceAssignmentID:    assignment.ID,
		SourceRunID:           "run-1",
		FromRoleID:            role.ID,
		ToRoleID:              role.ID,
		TargetAssignmentID:    assignment.ID,
		TargetWorkItemID:      work.ID,
		Title:                 "Next pass",
		Body:                  "Use the persisted context.",
		RecommendedNextAction: "Inspect the launch packet.",
		LinkedArtifactIDs:     []string{evidenceRecord.ID, reviewRecord.ID},
		LinkedMemoryIDs:       []string{"mem_later"},
		ContextRefs:           []string{"ctx_1"},
		Status:                core.HandoffStatusAccepted,
		ProvenanceKind:        "operator",
		TrustLabel:            "operator_reviewed",
	}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ProjectID:           project.ID,
		Title:               "Persistence lesson",
		Body:                "Cairnline stores collaboration artifacts in SQLite.",
		SuggestedTrustLabel: "test",
		SuggestedSourceKind: "assignment",
		SuggestedSourceID:   assignment.ID,
	}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	memoryEntry, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
		ProjectID:  project.ID,
		Title:      "Persisted memory",
		Body:       "Accepted memory is available to assignment launch packets.",
		SourceKind: core.MemorySourceOperator,
		SourceID:   assignment.ID,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, core.AssignmentCompleted, "run-1"); err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen Open() error = %v", err)
	}
	defer reopened.Close()
	reopenedService := core.NewService(reopened)

	projects, err := reopenedService.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID || len(projects[0].Roots) != 1 || len(projects[0].ContextSources) != 1 {
		t.Fatalf("projects = %+v, want persisted project metadata", projects)
	}
	if projects[0].DefaultRootID != "root_main" {
		t.Fatalf("default root = %q, want root_main", projects[0].DefaultRootID)
	}
	if projects[0].DefaultProfileID != profile.ID || projects[0].DefaultExecutionProfileID != executionProfile.ID {
		t.Fatalf("project defaults = %q/%q, want persisted profile and execution profile defaults", projects[0].DefaultProfileID, projects[0].DefaultExecutionProfileID)
	}
	source := projects[0].ContextSources[0]
	if source.Locator != "AGENTS.md" || source.Format != "agents_md" || source.Scope != "workspace" || source.SourceCategory != "instructions" || source.Metadata["root_id"] != "root_main" {
		t.Fatalf("source = %+v, want persisted context source metadata", source)
	}
	profiles, err := reopenedService.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != profile.ID || len(profiles[0].SkillIDs) != 1 {
		t.Fatalf("profiles = %+v, want persisted profile metadata", profiles)
	}
	executionProfiles, err := reopenedService.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() error = %v", err)
	}
	if len(executionProfiles) != 1 || executionProfiles[0].ID != executionProfile.ID || executionProfiles[0].AdapterOptions["mode"] != "test" {
		t.Fatalf("execution profiles = %+v, want persisted execution profile metadata", executionProfiles)
	}
	roles, err := reopenedService.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != role.ID || roles[0].DefaultExecutionProfileID != executionProfile.ID || len(roles[0].DefaultSkillIDs) != 2 {
		t.Fatalf("roles = %+v, want persisted role metadata", roles)
	}
	skills, err := reopenedService.ListProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListProjectSkills() error = %v", err)
	}
	if len(skills) != 1 || skills[0].ID != skill.ID || len(skills[0].SourceRefs) != 1 || !skills[0].Enabled {
		t.Fatalf("skills = %+v, want persisted enabled project skill", skills)
	}
	assignments, err := reopenedService.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != assignment.ID || assignments[0].Status != core.AssignmentCompleted || assignments[0].ExecutionRef != "run-1" {
		t.Fatalf("assignments = %+v, want completed assignment", assignments)
	}
	if assignments[0].RootID != "root_main" || assignments[0].ProfileID != profile.ID || assignments[0].ContextSnapshotID != "ctx-prep" || len(assignments[0].DesiredAgent.SkillIDs) != 2 {
		t.Fatalf("assignment metadata = %+v, want persisted root/profile/context/desired-agent update", assignments[0])
	}
	packet, err := reopenedService.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() error = %v", err)
	}
	if packet.Role == nil || packet.Role.ID != role.ID || packet.WorkItem.ID != work.ID {
		t.Fatalf("packet = %+v, want persisted context metadata", packet)
	}
	if packet.WorkItem.RootID != "root_main" || packet.Assignment.RootID != "root_main" {
		t.Fatalf("packet roots work=%q assignment=%q, want root_main", packet.WorkItem.RootID, packet.Assignment.RootID)
	}
	artifacts, err := reopenedService.ListArtifacts(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ID != artifactRecord.ID || artifacts[0].Kind != "decision_note" || artifacts[0].AuthorRoleID != role.ID {
		t.Fatalf("artifacts = %+v, want persisted generic artifact", artifacts)
	}
	gotArtifact, err := reopenedService.GetArtifact(ctx, project.ID, work.ID, artifactRecord.ID)
	if err != nil {
		t.Fatalf("GetArtifact() error = %v", err)
	}
	if gotArtifact.ID != artifactRecord.ID || gotArtifact.Body != artifactRecord.Body {
		t.Fatalf("GetArtifact() = %+v, want persisted artifact", gotArtifact)
	}
	evidenceItems, err := reopenedService.ListEvidence(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListEvidence() error = %v", err)
	}
	if len(evidenceItems) != 1 || evidenceItems[0].Title != "Output link" || evidenceItems[0].AssignmentID != assignment.ID || evidenceItems[0].TrustLabel != core.EvidenceTrustOperator {
		t.Fatalf("evidence = %+v, want persisted evidence", evidenceItems)
	}
	gotEvidence, err := reopenedService.GetEvidence(ctx, project.ID, work.ID, evidenceRecord.ID)
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if gotEvidence.ID != evidenceRecord.ID || gotEvidence.Locator != evidenceRecord.Locator {
		t.Fatalf("GetEvidence() = %+v, want persisted evidence", gotEvidence)
	}
	reviews, err := reopenedService.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 1 || reviews[0].AssignmentID != assignment.ID || reviews[0].Verdict != core.ReviewVerdictPass {
		t.Fatalf("reviews = %+v, want persisted review", reviews)
	}
	gotReview, err := reopenedService.GetReview(ctx, project.ID, work.ID, reviewRecord.ID)
	if err != nil {
		t.Fatalf("GetReview() error = %v", err)
	}
	if gotReview.ID != reviewRecord.ID || gotReview.ReviewerRoleID != role.ID || gotReview.Risk != core.ReviewRiskLow {
		t.Fatalf("GetReview() = %+v, want persisted review", gotReview)
	}
	handoffs, err := reopenedService.ListHandoffs(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListHandoffs() error = %v", err)
	}
	if len(handoffs) != 1 || handoffs[0].Status != core.HandoffStatusAccepted || handoffs[0].SourceAssignmentID != assignment.ID || handoffs[0].TargetAssignmentID != assignment.ID || handoffs[0].RecommendedNextAction == "" || len(handoffs[0].LinkedArtifactIDs) != 2 || len(handoffs[0].LinkedMemoryIDs) != 1 || len(handoffs[0].ContextRefs) != 1 {
		t.Fatalf("handoffs = %+v, want persisted handoff", handoffs)
	}
	memory, err := reopenedService.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(memory) != 1 || memory[0].Status != core.MemoryCandidatePending || memory[0].SuggestedSourceID != assignment.ID {
		t.Fatalf("memory candidates = %+v, want persisted candidate", memory)
	}
	memoryEntries, err := reopenedService.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries() error = %v", err)
	}
	if len(memoryEntries) != 1 || memoryEntries[0].ID != memoryEntry.ID || memoryEntries[0].TrustLabel != core.MemoryTrustOperator {
		t.Fatalf("memory entries = %+v, want persisted accepted memory", memoryEntries)
	}
	setup, err := reopenedService.ProjectSetupReadiness(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectSetupReadiness() error = %v", err)
	}
	if setup.ShowOnboarding || !setup.SetupStarted || setup.Summary.WorkItemCount != 1 || setup.Summary.RoleCount != 1 || setup.Summary.SkillCount != 1 || setup.Summary.ExecutionProfileCount != 1 {
		t.Fatalf("setup readiness = %+v, want persisted configured setup", setup)
	}
	health, err := reopenedService.ProjectHealth(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectHealth() error = %v", err)
	}
	if health.Status != core.ProjectHealthStatusAttention || health.Summary.PendingMemoryCandidateCount != 1 || health.Summary.ProjectSkillIssueCount == 0 {
		t.Fatalf("health = %+v, want pending memory and missing skill attention", health)
	}
	launchPacket, err := reopenedService.AssignmentLaunchPacket(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentLaunchPacket() error = %v", err)
	}
	if launchPacket.Kind != core.LaunchPacketKindAssignment || launchPacket.Assignment.ID != assignment.ID || launchPacket.Role == nil || launchPacket.Role.ID != role.ID {
		t.Fatalf("launch packet = %+v, want persisted launch packet metadata", launchPacket)
	}
	if launchPacket.Assignment.RootID != "root_main" {
		t.Fatalf("launch packet assignment root = %q, want root_main", launchPacket.Assignment.RootID)
	}
	if launchPacket.Profile == nil || launchPacket.Profile.ID != profile.ID || launchPacket.ExecutionProfile == nil || launchPacket.ExecutionProfile.ID != executionProfile.ID {
		t.Fatalf("launch packet = %+v, want persisted resolved profiles", launchPacket)
	}
	if len(launchPacket.Skills) != 1 || launchPacket.Skills[0].ID != skill.ID {
		t.Fatalf("launch packet skills = %+v, want persisted resolved skill", launchPacket.Skills)
	}
	if len(launchPacket.Memory) != 1 || launchPacket.Memory[0].ID != memoryEntry.ID {
		t.Fatalf("launch packet memory = %+v, want persisted accepted memory", launchPacket.Memory)
	}
	if len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 1 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}
}

func TestStore_DeleteProjectCascadesProjectScopedRows(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "project-delete.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)

	profile, err := service.CreateAgentProfile(ctx, core.AgentProfile{Name: "Global profile"})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	execution, err := service.CreateExecutionProfile(ctx, core.ExecutionProfile{Name: "Global execution"})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, core.Project{Name: "Delete me"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer", DefaultProfileID: profile.ID})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Delete scoped rows"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID, ExecutionProfileID: execution.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.CreateProjectSkill(ctx, core.ProjectSkill{ProjectID: project.ID, ID: "review", Title: "Review", Format: core.SkillFormatMarkdown, Status: core.SkillStatusAvailable, TrustLabel: core.SkillTrustWorkspace}); err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, core.Evidence{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Title: "Evidence", Locator: "https://example.test/evidence"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Body: "Pass", Verdict: core.ReviewVerdictPass}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, core.Handoff{ProjectID: project.ID, WorkItemID: work.ID, SourceAssignmentID: assignment.ID, Title: "Handoff", Body: "Follow up"}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{ProjectID: project.ID, Title: "Accepted memory", Body: "Remember this"}); err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{ProjectID: project.ID, Title: "Candidate", Body: "Review this"}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	if err := service.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := service.GetProject(ctx, project.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetProject() after delete error = %v, want ErrNotFound", err)
	}
	if _, err := service.ListWorkItems(ctx, project.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ListWorkItems() after delete error = %v, want ErrNotFound", err)
	}
	if _, err := service.ListMemoryEntries(ctx, project.ID, false); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ListMemoryEntries() after delete error = %v, want ErrNotFound", err)
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

func TestStore_DeleteProfilesAndRoles(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "delete-profiles-roles.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)

	profile, err := service.CreateAgentProfile(ctx, core.AgentProfile{Name: "Temporary profile"})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	executionProfile, err := service.CreateExecutionProfile(ctx, core.ExecutionProfile{Name: "Temporary execution"})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	if err := service.DeleteAgentProfile(ctx, profile.ID); err != nil {
		t.Fatalf("DeleteAgentProfile() error = %v", err)
	}
	if err := service.DeleteExecutionProfile(ctx, executionProfile.ID); err != nil {
		t.Fatalf("DeleteExecutionProfile() error = %v", err)
	}
	profiles, err := service.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() error = %v", err)
	}
	executionProfiles, err := service.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() error = %v", err)
	}
	if len(profiles) != 0 || len(executionProfiles) != 0 {
		t.Fatalf("profiles = %+v execution = %+v, want deleted globals removed", profiles, executionProfiles)
	}
	if err := service.DeleteAgentProfile(ctx, profile.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("DeleteAgentProfile(deleted) error = %v, want ErrNotFound", err)
	}
	if err := service.DeleteExecutionProfile(ctx, executionProfile.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("DeleteExecutionProfile(deleted) error = %v, want ErrNotFound", err)
	}

	project, err := service.CreateProject(ctx, core.Project{Name: "Role cleanup"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	spareRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Spare reviewer"})
	if err != nil {
		t.Fatalf("CreateRole(spare) error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Keep referenced role"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if err := service.DeleteRole(ctx, project.ID, spareRole.ID); err != nil {
		t.Fatalf("DeleteRole(spare) error = %v", err)
	}
	if err := service.DeleteRole(ctx, project.ID, role.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("DeleteRole(referenced) error = %v, want ErrConflict", err)
	}
	if err := service.DeleteAssignment(ctx, project.ID, assignment.ID); err != nil {
		t.Fatalf("DeleteAssignment() error = %v", err)
	}
	if err := service.DeleteRole(ctx, project.ID, role.ID); err != nil {
		t.Fatalf("DeleteRole(unreferenced) error = %v", err)
	}
	roles, err := service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("roles after deletes = %+v, want none", roles)
	}
}

func TestStore_DeleteWorkItemAndAssignmentScope(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "work-assignment-delete.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)

	project, err := service.CreateProject(ctx, core.Project{Name: "Cleanup"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Delete scoped work"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	keepWork, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Keep this work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(keep) error = %v", err)
	}
	deletedAssignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(deleted) error = %v", err)
	}
	keptAssignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: keepWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(keep) error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: deletedAssignment.ID, Body: "Delete with assignment.", Verdict: core.ReviewVerdictPass}); err != nil {
		t.Fatalf("CreateReview(deleted assignment) error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, core.Handoff{ProjectID: project.ID, WorkItemID: work.ID, TargetWorkItemID: keepWork.ID, Title: "Handoff", Body: "Continue elsewhere"}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	memory, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{ProjectID: project.ID, Title: "Keep project memory", Body: "Project-level candidate stays.", SuggestedSourceID: deletedAssignment.ID})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	if err := service.DeleteAssignment(ctx, project.ID, deletedAssignment.ID); err != nil {
		t.Fatalf("DeleteAssignment() error = %v", err)
	}
	if _, err := service.GetAssignment(ctx, project.ID, deletedAssignment.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetAssignment(deleted) error = %v, want ErrNotFound", err)
	}
	if reviews, err := service.ListReviews(ctx, project.ID, work.ID); err != nil || len(reviews) != 0 {
		t.Fatalf("reviews after assignment delete = %+v error=%v, want none", reviews, err)
	}

	if err := service.DeleteWorkItem(ctx, project.ID, work.ID); err != nil {
		t.Fatalf("DeleteWorkItem() error = %v", err)
	}
	if _, err := service.GetWorkItem(ctx, project.ID, work.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetWorkItem() after delete error = %v, want ErrNotFound", err)
	}
	assignments, err := service.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() after work delete error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != keptAssignment.ID {
		t.Fatalf("assignments after work delete = %+v, want kept assignment only", assignments)
	}
	candidates, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() after work delete error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != memory.ID {
		t.Fatalf("memory candidates after work delete = %+v, want project-level memory preserved", candidates)
	}
}

func TestStore_HandoffLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "handoffs.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)

	project, err := service.CreateProject(ctx, core.Project{Name: "Handoff flow"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	fromRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole(from) error = %v", err)
	}
	toRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole(to) error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Ship handoff"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, core.Handoff{
		ProjectID:         project.ID,
		WorkItemID:        work.ID,
		FromRoleID:        fromRole.ID,
		ToRoleID:          toRole.ID,
		Title:             "Ready for review",
		Body:              "Implementation is ready.",
		LinkedArtifactIDs: []string{"evidence_1"},
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	got, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff() error = %v", err)
	}
	got.Status = core.HandoffStatusAccepted
	got.Title = "Accepted for review"
	got.LinkedMemoryIDs = []string{"mem_1"}
	updated, err := service.UpdateHandoff(ctx, got)
	if err != nil {
		t.Fatalf("UpdateHandoff() error = %v", err)
	}
	if updated.Status != core.HandoffStatusAccepted || updated.Title != "Accepted for review" || len(updated.LinkedMemoryIDs) != 1 {
		t.Fatalf("updated handoff = %+v, want accepted replacement metadata", updated)
	}
	if _, err := service.UpdateHandoffStatus(ctx, project.ID, work.ID, handoff.ID, "unsupported"); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("UpdateHandoffStatus(unsupported) error = %v, want ErrInvalid", err)
	}
	if err := service.DeleteHandoff(ctx, project.ID, work.ID, handoff.ID); err != nil {
		t.Fatalf("DeleteHandoff() error = %v", err)
	}
	if _, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetHandoff(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStore_MemoryCandidateDecisionLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "candidate.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Candidates"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	candidate, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ProjectID:           project.ID,
		Title:               "Generated convention",
		Body:                "Use durable memory only after review.",
		SuggestedKind:       "note",
		SuggestedTrustLabel: core.MemoryTrustGenerated,
		SuggestedSourceKind: core.MemorySourceGenerated,
		SuggestedSourceID:   "run_1",
		SourceRefs: []core.MemoryCandidateSourceRef{{
			Kind: "task_run",
			ID:   "run_1",
		}},
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	promoted, entry, err := service.PromoteMemoryCandidate(ctx, core.MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: candidate.ID,
	})
	if err != nil {
		t.Fatalf("PromoteMemoryCandidate() error = %v", err)
	}
	if promoted.Status != core.MemoryCandidatePromoted || promoted.PromotedMemoryID != entry.ID {
		t.Fatalf("promoted candidate = %+v entry=%+v, want promoted linked entry", promoted, entry)
	}
	retried, retriedEntry, err := service.PromoteMemoryCandidate(ctx, core.MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: candidate.ID,
	})
	if err != nil {
		t.Fatalf("PromoteMemoryCandidate(retry) error = %v", err)
	}
	if retried.PromotedMemoryID != entry.ID || retriedEntry.ID != entry.ID {
		t.Fatalf("retry candidate=%+v entry=%+v, want idempotent promoted entry", retried, retriedEntry)
	}

	rejectCandidate, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ProjectID: project.ID,
		Title:     "Speculative convention",
		Body:      "Maybe skip tests.",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(reject) error = %v", err)
	}
	rejected, err := service.RejectMemoryCandidate(ctx, project.ID, rejectCandidate.ID, "Speculative.")
	if err != nil {
		t.Fatalf("RejectMemoryCandidate() error = %v", err)
	}
	if rejected.Status != core.MemoryCandidateRejected || rejected.StatusReason != "Speculative." {
		t.Fatalf("rejected candidate = %+v, want rejected reason", rejected)
	}
	if _, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID, Status: "bogus"}); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("ListMemoryCandidates(invalid status) error = %v, want ErrInvalid", err)
	}
	pending, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(pending) error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending candidates = %+v, want none after resolution", pending)
	}
	resolved, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID, IncludeResolved: true})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(resolved) error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved candidates = %+v, want promoted and rejected", resolved)
	}
	if err := service.DeleteMemoryCandidate(ctx, project.ID, rejectCandidate.ID); err != nil {
		t.Fatalf("DeleteMemoryCandidate() error = %v", err)
	}
	if _, err := service.GetMemoryCandidate(ctx, project.ID, rejectCandidate.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetMemoryCandidate(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStore_MemoryEntryLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Memory"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	entry, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
		ProjectID:  project.ID,
		Title:      "Keep reviews concrete",
		Body:       "Accepted review memory should cite evidence.",
		TrustLabel: core.MemoryTrustGenerated,
		SourceKind: core.MemorySourceGenerated,
		SourceID:   "memcand_1",
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	got, err := service.GetMemoryEntry(ctx, project.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetMemoryEntry() error = %v", err)
	}
	if got.ID != entry.ID || got.SourceKind != core.MemorySourceGenerated || !got.Enabled {
		t.Fatalf("got memory = %+v, want created enabled entry", got)
	}

	got.Enabled = false
	got.Title = "Keep review memory concrete"
	updated, err := service.UpdateMemoryEntry(ctx, got)
	if err != nil {
		t.Fatalf("UpdateMemoryEntry() error = %v", err)
	}
	if updated.Enabled || updated.Title != "Keep review memory concrete" || updated.CreatedAt.IsZero() {
		t.Fatalf("updated memory = %+v, want disabled updated entry preserving created_at", updated)
	}
	enabled, err := service.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries(enabled) error = %v", err)
	}
	if len(enabled) != 0 {
		t.Fatalf("enabled entries = %+v, want disabled entry omitted", enabled)
	}
	all, err := service.ListMemoryEntries(ctx, project.ID, true)
	if err != nil {
		t.Fatalf("ListMemoryEntries(all) error = %v", err)
	}
	if len(all) != 1 || all[0].ID != entry.ID {
		t.Fatalf("all entries = %+v, want disabled entry included", all)
	}

	if err := service.DeleteMemoryEntry(ctx, project.ID, entry.ID); err != nil {
		t.Fatalf("DeleteMemoryEntry() error = %v", err)
	}
	if _, err := service.GetMemoryEntry(ctx, project.ID, entry.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetMemoryEntry(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStore_MemoryListsUseHecateCompatibleOrder(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory_order.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Memory order"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	base := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	enabled, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
		ID:         "mem_enabled",
		ProjectID:  project.ID,
		Title:      "Enabled memory",
		Body:       "Enabled entries sort before disabled entries.",
		TrustLabel: core.MemoryTrustOperator,
		SourceKind: core.MemorySourceOperator,
		CreatedAt:  base,
		UpdatedAt:  base,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry(enabled) error = %v", err)
	}
	disabled, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
		ID:         "mem_disabled",
		ProjectID:  project.ID,
		Title:      "Disabled memory",
		Body:       "Disabled entries remain inspectable but sort after enabled entries.",
		TrustLabel: core.MemoryTrustOperator,
		SourceKind: core.MemorySourceOperator,
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

	pending, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ID:                  "memcand_pending",
		ProjectID:           project.ID,
		Title:               "Pending candidate",
		Body:                "Pending candidates need operator review.",
		SuggestedTrustLabel: core.MemoryTrustGenerated,
		SuggestedSourceKind: core.MemorySourceGenerated,
		CreatedAt:           base,
		UpdatedAt:           base,
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(pending) error = %v", err)
	}
	rejected, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ID:                  "memcand_rejected",
		ProjectID:           project.ID,
		Title:               "Rejected candidate",
		Body:                "Resolved candidates sort after pending candidates.",
		SuggestedTrustLabel: core.MemoryTrustGenerated,
		SuggestedSourceKind: core.MemorySourceGenerated,
		CreatedAt:           base.Add(time.Minute),
		UpdatedAt:           base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(rejected) error = %v", err)
	}
	rejected.Status = core.MemoryCandidateRejected
	rejected.StatusReason = "Not durable."
	rejected.UpdatedAt = base.Add(2 * time.Minute)
	if _, err := service.UpdateMemoryCandidate(ctx, rejected); err != nil {
		t.Fatalf("UpdateMemoryCandidate(rejected) error = %v", err)
	}
	candidates, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID, IncludeResolved: true})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 2 || candidates[0].ID != pending.ID || candidates[1].ID != rejected.ID {
		t.Fatalf("candidates = %+v, want pending candidate before newer rejected candidate", candidates)
	}
}

func TestStore_MigrateAddsAssignmentRootID(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE assignments (
		project_id TEXT NOT NULL,
		id TEXT NOT NULL,
		work_item_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		profile_id TEXT NOT NULL DEFAULT '',
		execution_profile_id TEXT NOT NULL DEFAULT '',
		execution_mode TEXT NOT NULL,
		status TEXT NOT NULL,
		desired_agent_json TEXT NOT NULL DEFAULT '{}',
		claimed_by TEXT NOT NULL DEFAULT '',
		execution_ref TEXT NOT NULL DEFAULT '',
		context_snapshot_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (project_id, id)
	)`)
	if err != nil {
		t.Fatalf("create old assignments table error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close old db error = %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	rows, err := store.db.QueryContext(ctx, `PRAGMA table_info(assignments)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info error = %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info error = %v", err)
		}
		if name == "root_id" {
			found = true
			if columnType != "TEXT" || notNull != 1 {
				t.Fatalf("root_id column type=%q notNull=%d, want TEXT NOT NULL", columnType, notNull)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows error = %v", err)
	}
	if !found {
		t.Fatalf("assignments root_id column was not added")
	}
}

func TestStore_MigrateAddsProjectDefaultColumns(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old-project.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		roots_json TEXT NOT NULL DEFAULT '[]',
		context_sources_json TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create old projects table error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close old db error = %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	rows, err := store.db.QueryContext(ctx, `PRAGMA table_info(projects)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info error = %v", err)
	}
	defer rows.Close()
	found := map[string]bool{}
	want := map[string]bool{
		"default_root_id":              true,
		"default_profile_id":           true,
		"default_execution_profile_id": true,
	}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info error = %v", err)
		}
		if want[name] {
			found[name] = true
			if columnType != "TEXT" || notNull != 1 {
				t.Fatalf("%s column type=%q notNull=%d, want TEXT NOT NULL", name, columnType, notNull)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows error = %v", err)
	}
	for name := range want {
		if !found[name] {
			t.Fatalf("projects %s column was not added", name)
		}
	}
}

func TestStore_ClaimAssignmentRaceHasOneWinner(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "race.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Race"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Claim once"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
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
			case errors.Is(err, core.ErrConflict):
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

func TestStore_CreateAssignmentValidatesReferences(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "validation.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Validation"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Needs role"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	_, err = service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     "role_missing",
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("CreateAssignment() error = %v, want ErrNotFound", err)
	}
}

func TestStore_PersistsAssistantProposalLedger(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "assistant-ledger.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Assistant ledger"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	record, err := service.CreateAssistantProposal(ctx, core.AssistantProposal{
		ID:        "prop_sqlite",
		ProjectID: project.ID,
		Title:     "Create persisted work",
		Warnings:  []string{"persist warning"},
		Actions: []core.AssistantAction{
			{
				Kind: core.AssistantActionCreateRole,
				Role: &core.Role{
					ID:        "role_sqlite",
					ProjectID: project.ID,
					Name:      "Operator",
				},
			},
			{
				Kind: core.AssistantActionCreateWorkItem,
				WorkItem: &core.WorkItem{
					ID:        "work_sqlite",
					ProjectID: project.ID,
					Title:     "Persist proposal apply",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssistantProposal() error = %v", err)
	}
	if _, err := service.ApplyAssistantProposalRecord(ctx, record.ID, true); err != nil {
		t.Fatalf("ApplyAssistantProposalRecord() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen Open() error = %v", err)
	}
	defer reopened.Close()
	reopenedService := core.NewService(reopened)
	records, err := reopenedService.ListAssistantProposals(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssistantProposals() error = %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID || records[0].Status != core.AssistantProposalStatusApplied || records[0].LatestResult == nil || len(records[0].ApplyAttempts) != 1 || records[0].AppliedAt == nil || len(records[0].Proposal.Warnings) != 1 || records[0].Proposal.Warnings[0] != "persist warning" {
		t.Fatalf("records = %+v, want persisted applied proposal ledger", records)
	}
	if _, err := reopenedService.ApplyAssistantProposalRecord(ctx, record.ID, true); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("reapply after reopen error = %v, want ErrConflict", err)
	}
}

func TestStore_CreateReviewValidatesAssignmentWorkItem(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "review-validation.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Review validation"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	workA, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "A"})
	if err != nil {
		t.Fatalf("CreateWorkItem(A) error = %v", err)
	}
	workB, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "B"})
	if err != nil {
		t.Fatalf("CreateWorkItem(B) error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: workA.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	_, err = service.CreateReview(ctx, core.Review{
		ProjectID:    project.ID,
		WorkItemID:   workB.ID,
		AssignmentID: assignment.ID,
		Body:         "Assignment belongs to another work item.",
		Verdict:      core.ReviewVerdictBlocked,
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("CreateReview() error = %v, want ErrNotFound", err)
	}
}
