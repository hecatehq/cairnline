package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
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
	project, err := service.CreateProject(ctx, core.Project{
		Name:          "Persistent project",
		Description:   "Survives process restart.",
		DefaultRootID: "root_main",
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
		ProjectID:      project.ID,
		ID:             "review",
		Title:          "Review skill",
		Description:    "Review work with evidence.",
		Format:         core.SkillFormatMarkdown,
		SuggestedTools: []string{"git.diff", "file.read"},
		RequiredPermissions: core.RequiredPermissions{
			Tools:  boolPointerForStoreTest(true),
			Writes: boolPointerForStoreTest(false),
		},
		Status:     core.SkillStatusAvailable,
		TrustLabel: core.SkillTrustWorkspace,
		SourceRefs: []string{"src_agents"},
	})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:            project.ID,
		Name:                 "Reviewer",
		Instructions:         "Review the durable trail.",
		DefaultSkillIDs:      []string{"review", "evidence"},
		DefaultExecutionMode: core.ExecutionMCPPull,
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
		ProjectID:     project.ID,
		WorkItemID:    work.ID,
		RoleID:        role.ID,
		RootID:        "root_main",
		ExecutionMode: core.ExecutionMCPPull,
		DesiredAgent: core.DesiredAgent{
			Kind:     "any",
			SkillIDs: []string{"review"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	assignment, err = service.UpdateQueuedAssignment(ctx, project.ID, assignment.ID, core.QueuedAssignmentUpdate{
		Expected:          assignment.Coordination(),
		ExpectedUpdatedAt: assignment.UpdatedAt,
		Replacement: core.AssignmentCoordination{
			WorkItemID:    work.ID,
			RoleID:        role.ID,
			RootID:        "root_main",
			ExecutionMode: core.ExecutionMCPPull,
			DesiredAgent: core.DesiredAgent{
				Kind:     "any",
				SkillIDs: []string{"review", "evidence"},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateQueuedAssignment() error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	assignment, err = service.PrepareAssignment(ctx, project.ID, assignment.ID, core.AssignmentPreparation{ClaimedBy: "agent-a", ContextSnapshotID: "ctx-prep"})
	if err != nil {
		t.Fatalf("PrepareAssignment() error = %v", err)
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
		SourceKind:   "pull_request",
		ExternalID:   "PR 42",
		Provider:     "github",
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
		Verdict:        core.ReviewVerdictApproved,
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
	if _, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, core.AssignmentCompleted, core.ExecutionRef{RunID: "run-1"}); err != nil {
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
	source := projects[0].ContextSources[0]
	if source.Locator != "AGENTS.md" || source.Format != "agents_md" || source.Scope != "workspace" || source.SourceCategory != "instructions" || source.Metadata["root_id"] != "root_main" {
		t.Fatalf("source = %+v, want persisted context source metadata", source)
	}
	roles, err := reopenedService.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != role.ID || len(roles[0].DefaultSkillIDs) != 2 {
		t.Fatalf("roles = %+v, want persisted role metadata", roles)
	}
	skills, err := reopenedService.ListProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListProjectSkills() error = %v", err)
	}
	if len(skills) != 1 || skills[0].ID != skill.ID || len(skills[0].SourceRefs) != 1 || !skills[0].Enabled {
		t.Fatalf("skills = %+v, want persisted enabled project skill", skills)
	}
	if strings.Join(skills[0].SuggestedTools, ",") != "file.read,git.diff" || skills[0].RequiredPermissions.Tools == nil || !*skills[0].RequiredPermissions.Tools || skills[0].RequiredPermissions.Writes == nil || *skills[0].RequiredPermissions.Writes {
		t.Fatalf("skill capability metadata = %+v / %+v, want persisted tools and permissions", skills[0].SuggestedTools, skills[0].RequiredPermissions)
	}
	assignments, err := reopenedService.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != assignment.ID || assignments[0].Status != core.AssignmentCompleted || assignments[0].ExecutionRef.RunID != "run-1" {
		t.Fatalf("assignments = %+v, want completed assignment", assignments)
	}
	if assignments[0].StartedAt.IsZero() || assignments[0].CompletedAt.IsZero() {
		t.Fatalf("assignment timestamps = started:%s completed:%s, want persisted lifecycle timestamps", assignments[0].StartedAt, assignments[0].CompletedAt)
	}
	if assignments[0].RootID != "root_main" || assignments[0].ContextSnapshotID != "ctx-prep" || len(assignments[0].DesiredAgent.SkillIDs) != 2 {
		t.Fatalf("assignment metadata = %+v, want persisted root/context/desired-agent update", assignments[0])
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
	if len(evidenceItems) != 1 || evidenceItems[0].Title != "Output link" || evidenceItems[0].AssignmentID != assignment.ID || evidenceItems[0].TrustLabel != core.EvidenceTrustOperator || evidenceItems[0].SourceKind != "pull_request" || evidenceItems[0].ExternalID != "PR 42" || evidenceItems[0].Provider != "github" {
		t.Fatalf("evidence = %+v, want persisted evidence", evidenceItems)
	}
	gotEvidence, err := reopenedService.GetEvidence(ctx, project.ID, work.ID, evidenceRecord.ID)
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if gotEvidence.ID != evidenceRecord.ID || gotEvidence.Locator != evidenceRecord.Locator || gotEvidence.SourceKind != "pull_request" || gotEvidence.ExternalID != "PR 42" || gotEvidence.Provider != "github" {
		t.Fatalf("GetEvidence() = %+v, want persisted evidence", gotEvidence)
	}
	reviews, err := reopenedService.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 1 || reviews[0].AssignmentID != assignment.ID || reviews[0].Verdict != core.ReviewVerdictApproved {
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
	if setup.ShowOnboarding || !setup.SetupStarted || setup.Summary.WorkItemCount != 1 || setup.Summary.RoleCount != 1 || setup.Summary.SkillCount != 1 {
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
	if len(launchPacket.Skills) != 1 || launchPacket.Skills[0].ID != skill.ID {
		t.Fatalf("launch packet skills = %+v, want persisted resolved skill", launchPacket.Skills)
	}
	if strings.Join(launchPacket.Skills[0].SuggestedTools, ",") != "file.read,git.diff" {
		t.Fatalf("launch packet skill = %+v, want capability metadata", launchPacket.Skills[0])
	}
	if len(launchPacket.Memory) != 1 || launchPacket.Memory[0].ID != memoryEntry.ID {
		t.Fatalf("launch packet memory = %+v, want persisted accepted memory", launchPacket.Memory)
	}
	if len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 1 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}
}

func TestStore_ReleaseAssignmentClearsClaimForRetry(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "release-assignment.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Release"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Retry start"})
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
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	startedAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	if err := store.DeleteAssignment(ctx, project.ID, assignment.ID); err != nil {
		t.Fatalf("DeleteAssignment(claim metadata fixture) error = %v", err)
	}
	if _, err := store.CreateAssignment(ctx, core.Assignment{
		ProjectID:         project.ID,
		ID:                assignment.ID,
		WorkItemID:        work.ID,
		RoleID:            role.ID,
		ExecutionMode:     core.ExecutionMCPPull,
		Status:            core.AssignmentClaimed,
		ClaimedBy:         "agent-a",
		ExecutionRef:      core.ExecutionRef{RunID: "run-pre-dispatch"},
		ContextSnapshotID: "ctx-pre-dispatch",
		StartedAt:         startedAt,
		CompletedAt:       startedAt.Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateAssignment(claim metadata fixture) error = %v", err)
	}
	if _, err := service.ReleaseAssignment(ctx, project.ID, assignment.ID, "agent-b"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("ReleaseAssignment(wrong claimer) error = %v, want ErrConflict", err)
	}
	released, err := service.ReleaseAssignment(ctx, project.ID, assignment.ID, "agent-a")
	if err != nil {
		t.Fatalf("ReleaseAssignment() error = %v", err)
	}
	if released.Status != core.AssignmentQueued || released.ClaimedBy != "" || !released.ExecutionRef.Empty() || released.ContextSnapshotID != "" || !released.StartedAt.IsZero() || !released.CompletedAt.IsZero() {
		t.Fatalf("released assignment = %+v, want queued with claim/runtime refs cleared", released)
	}
	reclaimed, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-b")
	if err != nil {
		t.Fatalf("ClaimAssignment(after release) error = %v", err)
	}
	if reclaimed.Status != core.AssignmentClaimed || reclaimed.ClaimedBy != "agent-b" {
		t.Fatalf("reclaimed assignment = %+v, want claimed by agent-b", reclaimed)
	}
}

func TestStore_ContextSourceMutationsPersist(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "context-sources.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Context sources"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	_, created, err := service.CreateContextSource(ctx, project.ID, core.Source{
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
	})
	if err != nil {
		t.Fatalf("CreateContextSource() error = %v", err)
	}
	createdAt := created.CreatedAt
	if _, _, err := service.UpdateContextSource(ctx, project.ID, "src_agents", core.Source{
		Kind:       "url",
		Title:      "Design brief",
		Locator:    "https://example.invalid/design",
		Enabled:    false,
		Format:     "url",
		TrustLabel: "operator_source",
	}); err != nil {
		t.Fatalf("UpdateContextSource() error = %v", err)
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
	source, err := reopenedService.GetContextSource(ctx, project.ID, "src_agents")
	if err != nil {
		t.Fatalf("GetContextSource() after reopen error = %v", err)
	}
	if source.Title != "Design brief" || source.Enabled || source.Locator != "https://example.invalid/design" || !source.CreatedAt.Equal(createdAt) {
		t.Fatalf("source after reopen = %+v, want updated metadata with original created time", source)
	}
	project, deleted, err := reopenedService.DeleteContextSource(ctx, project.ID, "src_agents")
	if err != nil {
		t.Fatalf("DeleteContextSource() error = %v", err)
	}
	if deleted.ID != "src_agents" || len(project.ContextSources) != 0 {
		t.Fatalf("deleted source=%+v project=%+v, want source removed", deleted, project)
	}
}

func TestStore_RootMutationsPersist(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "roots.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Roots"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, _, err := service.CreateRoot(ctx, project.ID, core.Root{
		ID:        "root_main",
		Path:      "/workspace/main",
		Kind:      "git",
		GitRemote: "https://github.com/hecatehq/hecate",
		GitBranch: "main",
		Active:    true,
	}); err != nil {
		t.Fatalf("CreateRoot(main) error = %v", err)
	}
	if _, _, err := service.CreateRoot(ctx, project.ID, core.Root{
		ID:     "root_other",
		Path:   "/workspace/other",
		Kind:   "folder",
		Active: true,
	}); err != nil {
		t.Fatalf("CreateRoot(other) error = %v", err)
	}
	if _, _, err := service.UpdateRoot(ctx, project.ID, "root_main", core.Root{
		Path:      "/workspace/.worktrees/root-api",
		Kind:      "git_worktree",
		GitBranch: "feature/root-api",
		Active:    false,
	}); err != nil {
		t.Fatalf("UpdateRoot() error = %v", err)
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
	root, err := reopenedService.GetRoot(ctx, project.ID, "root_main")
	if err != nil {
		t.Fatalf("GetRoot() after reopen error = %v", err)
	}
	if root.Path != "/workspace/.worktrees/root-api" || root.Kind != "git_worktree" || root.GitBranch != "feature/root-api" || root.Active {
		t.Fatalf("root after reopen = %+v, want updated inactive worktree metadata", root)
	}
	project, deleted, err := reopenedService.DeleteRoot(ctx, project.ID, "root_main")
	if err != nil {
		t.Fatalf("DeleteRoot() error = %v", err)
	}
	if deleted.ID != "root_main" || len(project.Roots) != 1 || project.Roots[0].ID != "root_other" || project.DefaultRootID != "root_other" {
		t.Fatalf("deleted root=%+v project roots=%+v default=%q, want remaining root defaulted", deleted, project.Roots, project.DefaultRootID)
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

	project, err := service.CreateProject(ctx, core.Project{Name: "Delete me"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Delete scoped rows"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.CreateProjectSkill(ctx, core.ProjectSkill{ProjectID: project.ID, ID: "review", Title: "Review", Format: core.SkillFormatMarkdown, Status: core.SkillStatusAvailable, TrustLabel: core.SkillTrustWorkspace}); err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, core.Evidence{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Title: "Evidence", Locator: "https://example.test/evidence"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Body: "Pass", Verdict: core.ReviewVerdictApproved}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, core.Handoff{ProjectID: project.ID, WorkItemID: work.ID, SourceAssignmentID: assignment.ID, ToRoleID: role.ID, Title: "Handoff", Body: "Follow up"})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := store.AcceptHandoffWithFollowUp(ctx, core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         project.ID,
		WorkItemID:        work.ID,
		HandoffID:         handoff.ID,
		ExpectedUpdatedAt: handoff.UpdatedAt,
		IdempotencyKey:    "delete-project-receipt",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}, "asgn_delete_follow_up", time.Now); err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
	}
	var receiptCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM command_receipts WHERE project_id = ?`, project.ID).Scan(&receiptCount); err != nil {
		t.Fatalf("count command receipts before delete error = %v", err)
	}
	if receiptCount != 1 {
		t.Fatalf("command receipt count before delete = %d, want 1", receiptCount)
	}
	if _, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{ProjectID: project.ID, Title: "Accepted memory", Body: "Remember this"}); err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{ProjectID: project.ID, Title: "Candidate", Body: "Review this"}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	// DeleteProject explicitly removes every project-scoped table rather than
	// relying on connection-local foreign-key cascade enforcement.
	if _, err := store.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign keys error = %v", err)
	}
	if err := service.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM command_receipts WHERE project_id = ?`, project.ID).Scan(&receiptCount); err != nil {
		t.Fatalf("count command receipts after delete error = %v", err)
	}
	if receiptCount != 0 {
		t.Fatalf("command receipt count after delete = %d, want 0", receiptCount)
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
}

func TestStore_DeleteRoleAllowsHistoricalAssignments(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "delete-roles.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)

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
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Keep historical assignment"})
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
	if err := service.DeleteRole(ctx, project.ID, role.ID); err != nil {
		t.Fatalf("DeleteRole(referenced) error = %v", err)
	}
	context, err := service.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() after role delete error = %v", err)
	}
	if context.Assignment.RoleID != role.ID || context.Role != nil || !containsString(context.Warnings, "assignment role was not found") {
		t.Fatalf("assignment context after role delete = %+v, want durable assignment with missing-role warning", context)
	}
	roles, err := service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("roles after deletes = %+v, want none", roles)
	}
	if err := service.DeleteRole(ctx, project.ID, role.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("DeleteRole(deleted) error = %v, want ErrNotFound", err)
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
	if _, err := service.CreateReview(ctx, core.Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: deletedAssignment.ID, Body: "Delete with assignment.", Verdict: core.ReviewVerdictApproved}); err != nil {
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
	if !handoff.StatusChangedAt.Equal(handoff.CreatedAt) {
		t.Fatalf("handoff status_changed_at = %s, want created_at %s", handoff.StatusChangedAt, handoff.CreatedAt)
	}
	got, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff() error = %v", err)
	}
	acceptedAt := got.UpdatedAt.Add(time.Minute)
	acceptedStatus := core.HandoffStatusAccepted
	acceptedTitle := "Accepted for review"
	linkedMemoryIDs := []string{"mem_1"}
	updated, err := store.UpdateHandoff(ctx, project.ID, work.ID, handoff.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: got.UpdatedAt,
		Patch: core.HandoffPatch{
			Status:          &acceptedStatus,
			Title:           &acceptedTitle,
			LinkedMemoryIDs: &linkedMemoryIDs,
		},
	}, func() time.Time { return acceptedAt })
	if err != nil {
		t.Fatalf("UpdateHandoff() error = %v", err)
	}
	if updated.Status != core.HandoffStatusAccepted || updated.Title != "Accepted for review" || len(updated.LinkedMemoryIDs) != 1 {
		t.Fatalf("updated handoff = %+v, want accepted replacement metadata", updated)
	}
	if !updated.StatusChangedAt.Equal(acceptedAt) {
		t.Fatalf("updated status_changed_at = %s, want status update time %s", updated.StatusChangedAt, acceptedAt)
	}
	reloaded, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff(updated) error = %v", err)
	}
	if !reloaded.StatusChangedAt.Equal(acceptedAt) {
		t.Fatalf("reloaded status_changed_at = %s, want %s", reloaded.StatusChangedAt, acceptedAt)
	}
	importedCreatedAt := reloaded.CreatedAt.Add(-time.Hour)
	importedStatusChangedAt := reloaded.StatusChangedAt.Add(15 * time.Minute)
	reloaded.Status = core.HandoffStatusSuperseded
	reloaded.CreatedAt = importedCreatedAt
	reloaded.UpdatedAt = acceptedAt.Add(15 * time.Minute)
	reloaded.StatusChangedAt = importedStatusChangedAt
	importedUpdate, err := store.RestoreHandoffSnapshot(ctx, reloaded)
	if err != nil {
		t.Fatalf("UpdateHandoff(imported status change) error = %v", err)
	}
	if !importedUpdate.CreatedAt.Equal(importedCreatedAt) || !importedUpdate.StatusChangedAt.Equal(importedStatusChangedAt) {
		t.Fatalf("imported timestamps = created %s / status %s, want %s / %s", importedUpdate.CreatedAt, importedUpdate.StatusChangedAt, importedCreatedAt, importedStatusChangedAt)
	}
	reloaded, err = service.GetHandoff(ctx, project.ID, work.ID, handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff(imported status change) error = %v", err)
	}
	if !reloaded.StatusChangedAt.Equal(importedStatusChangedAt) {
		t.Fatalf("reloaded imported status_changed_at = %s, want %s", reloaded.StatusChangedAt, importedStatusChangedAt)
	}
	if _, err := service.UpdateHandoffStatus(ctx, project.ID, work.ID, handoff.ID, core.HandoffStatusUpdate{ExpectedUpdatedAt: reloaded.UpdatedAt, Status: "unsupported"}); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("UpdateHandoffStatus(unsupported) error = %v, want ErrInvalid", err)
	}
	if err := service.DeleteHandoff(ctx, project.ID, work.ID, handoff.ID, core.HandoffDelete{ExpectedUpdatedAt: importedUpdate.UpdatedAt}); err != nil {
		t.Fatalf("DeleteHandoff() error = %v", err)
	}
	if _, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetHandoff(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStore_HandoffCompareAndSetTransitions(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "handoff-cas.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	fixture := seedHandoffAuthorityFixture(t, store, core.ExecutionMCPPull)
	original := fixture.handoff

	accepted := core.HandoffStatusAccepted
	fixedNow := func() time.Time { return original.UpdatedAt }
	statusUpdate, err := store.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, original.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: original.UpdatedAt,
		Patch:             core.HandoffPatch{Status: &accepted},
	}, fixedNow)
	if err != nil {
		t.Fatalf("UpdateHandoff(status) error = %v", err)
	}
	if !statusUpdate.UpdatedAt.After(original.UpdatedAt) {
		t.Fatalf("status update token = %s, want after %s", statusUpdate.UpdatedAt, original.UpdatedAt)
	}
	if !statusUpdate.StatusChangedAt.Equal(statusUpdate.UpdatedAt) {
		t.Fatalf("status_changed_at = %s, want transition token %s", statusUpdate.StatusChangedAt, statusUpdate.UpdatedAt)
	}

	staleTitle := "Stale editor"
	if _, err := store.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, original.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: original.UpdatedAt,
		Patch:             core.HandoffPatch{Title: &staleTitle},
	}, fixedNow); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateHandoff(stale content) error = %v, want ErrConflict", err)
	}
	if err := store.DeleteHandoff(ctx, fixture.project.ID, fixture.work.ID, original.ID, core.HandoffDelete{ExpectedUpdatedAt: original.UpdatedAt}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("DeleteHandoff(stale) error = %v, want ErrConflict", err)
	}

	newTitle := "Authoritative editor"
	contentUpdate, err := store.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, original.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: statusUpdate.UpdatedAt,
		Patch:             core.HandoffPatch{Title: &newTitle},
	}, func() time.Time { return statusUpdate.UpdatedAt })
	if err != nil {
		t.Fatalf("UpdateHandoff(content) error = %v", err)
	}
	if !contentUpdate.UpdatedAt.After(statusUpdate.UpdatedAt) {
		t.Fatalf("content update token = %s, want after %s", contentUpdate.UpdatedAt, statusUpdate.UpdatedAt)
	}
	if !contentUpdate.StatusChangedAt.Equal(statusUpdate.StatusChangedAt) {
		t.Fatalf("content update status_changed_at = %s, want unchanged %s", contentUpdate.StatusChangedAt, statusUpdate.StatusChangedAt)
	}
	noOp, err := store.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, original.ID, core.HandoffUpdate{ExpectedUpdatedAt: contentUpdate.UpdatedAt}, fixedNow)
	if err != nil {
		t.Fatalf("UpdateHandoff(no-op) error = %v", err)
	}
	if !noOp.UpdatedAt.Equal(contentUpdate.UpdatedAt) {
		t.Fatalf("no-op token = %s, want unchanged %s", noOp.UpdatedAt, contentUpdate.UpdatedAt)
	}
	if err := store.DeleteHandoff(ctx, fixture.project.ID, fixture.work.ID, original.ID, core.HandoffDelete{ExpectedUpdatedAt: contentUpdate.UpdatedAt}); err != nil {
		t.Fatalf("DeleteHandoff(current) error = %v", err)
	}
}

func TestStore_AcceptHandoffWithFollowUpIsAtomicAndDurablyIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "handoff-follow-up.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	fixture := seedHandoffAuthorityFixture(t, store, core.ExecutionManual)
	command := core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         fixture.project.ID,
		WorkItemID:        fixture.work.ID,
		HandoffID:         fixture.handoff.ID,
		ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
		IdempotencyKey:    "follow-up-retry",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}
	fixedNow := fixture.handoff.UpdatedAt
	result, err := store.AcceptHandoffWithFollowUp(ctx, command, "asgn_follow_up", func() time.Time { return fixedNow })
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
	}
	if result.Outcome != core.HandoffFollowUpCreated || result.Replayed {
		t.Fatalf("first result = %+v, want newly created outcome", result)
	}
	if result.Handoff.Status != core.HandoffStatusAccepted || result.Handoff.TargetAssignmentID != result.Assignment.ID || result.Handoff.TargetWorkItemID != fixture.work.ID {
		t.Fatalf("accepted handoff = %+v, want linked follow-up", result.Handoff)
	}
	if !result.Handoff.UpdatedAt.After(fixture.handoff.UpdatedAt) {
		t.Fatalf("handoff token = %s, want after %s", result.Handoff.UpdatedAt, fixture.handoff.UpdatedAt)
	}
	if result.Assignment.ID != "asgn_follow_up" || result.Assignment.Status != core.AssignmentQueued || result.Assignment.ExecutionMode != core.ExecutionManual || result.Assignment.DesiredAgent.Kind != "human" || len(result.Assignment.DesiredAgent.SkillIDs) != 2 {
		t.Fatalf("follow-up assignment = %+v, want queued manual role defaults", result.Assignment)
	}
	if result.Assignment.RootID != "" || result.Assignment.ClaimedBy != "" || !result.Assignment.ExecutionRef.Empty() || result.Assignment.ContextSnapshotID != "" || !result.Assignment.StartedAt.IsZero() || !result.Assignment.CompletedAt.IsZero() {
		t.Fatalf("rootless follow-up execution state = %+v, want portable queued coordination only", result.Assignment)
	}

	claimed, err := store.ClaimAssignment(ctx, fixture.project.ID, result.Assignment.ID, "operator-a", func() time.Time { return result.Assignment.UpdatedAt.Add(time.Second) })
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	currentTitle := "Continue with current evidence"
	currentHandoff, err := store.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: result.Handoff.UpdatedAt,
		Patch:             core.HandoffPatch{Title: &currentTitle},
	}, func() time.Time { return result.Handoff.UpdatedAt.Add(2 * time.Second) })
	if err != nil {
		t.Fatalf("UpdateHandoff(after follow-up) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer reopened.Close()

	replayed, err := reopened.AcceptHandoffWithFollowUp(ctx, command, "asgn_retry_must_not_exist", time.Now)
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(replay) error = %v", err)
	}
	if !replayed.Replayed || replayed.Outcome != result.Outcome || replayed.Handoff.Title != currentTitle || !replayed.Handoff.UpdatedAt.Equal(currentHandoff.UpdatedAt) || replayed.Assignment.ID != result.Assignment.ID || replayed.Assignment.Status != core.AssignmentClaimed || replayed.Assignment.ClaimedBy != claimed.ClaimedBy || !replayed.Assignment.UpdatedAt.Equal(claimed.UpdatedAt) {
		t.Fatalf("replayed result = %+v, want original outcome/identity with current claimed assignment", replayed)
	}
	assignments, err := reopened.ListAssignments(ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != result.Assignment.ID {
		t.Fatalf("assignments after replay = %+v, want exactly one follow-up", assignments)
	}

	changedRequest := command
	changedRequest.ExpectedUpdatedAt = result.Handoff.UpdatedAt
	if _, err := reopened.AcceptHandoffWithFollowUp(ctx, changedRequest, "asgn_changed_request", time.Now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("AcceptHandoffWithFollowUp(reused key with changed request) error = %v, want ErrConflict", err)
	}
	staleNewKey := command
	staleNewKey.IdempotencyKey = "stale-new-command"
	if _, err := reopened.AcceptHandoffWithFollowUp(ctx, staleNewKey, "asgn_stale_request", time.Now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("AcceptHandoffWithFollowUp(stale new key) error = %v, want ErrConflict", err)
	}
	dismissed := core.HandoffStatusDismissed
	if _, err := reopened.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: replayed.Handoff.UpdatedAt,
		Patch:             core.HandoffPatch{Status: &dismissed},
	}, time.Now); err != nil {
		t.Fatalf("UpdateHandoff(dismiss after receipt) error = %v", err)
	}
	if _, err := reopened.AcceptHandoffWithFollowUp(ctx, command, "asgn_receipt_must_not_override_close", time.Now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("AcceptHandoffWithFollowUp(replay after dismissal) error = %v, want ErrConflict", err)
	}
}

func TestStore_AcceptHandoffWithFollowUpSerializesSameKey(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "handoff-follow-up-race.db")
	firstStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	defer firstStore.Close()
	fixture := seedHandoffAuthorityFixture(t, firstStore, core.ExecutionMCPPull)
	secondStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer secondStore.Close()
	command := core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         fixture.project.ID,
		WorkItemID:        fixture.work.ID,
		HandoffID:         fixture.handoff.ID,
		ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
		IdempotencyKey:    "concurrent-retry",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}
	type commandResult struct {
		result core.HandoffFollowUpResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan commandResult, 2)
	for index, candidate := range []*Store{firstStore, secondStore} {
		index, candidate := index, candidate
		go func() {
			<-start
			result, err := candidate.AcceptHandoffWithFollowUp(ctx, command, fmt.Sprintf("asgn_racer_%d", index), func() time.Time { return fixture.handoff.UpdatedAt })
			results <- commandResult{result: result, err: err}
		}()
	}
	close(start)
	first := <-results
	second := <-results
	for index, result := range []commandResult{first, second} {
		if result.err != nil {
			t.Fatalf("concurrent result %d error = %v", index, result.err)
		}
	}
	if first.result.Assignment.ID != second.result.Assignment.ID || first.result.Outcome != core.HandoffFollowUpCreated || second.result.Outcome != core.HandoffFollowUpCreated || first.result.Replayed == second.result.Replayed {
		t.Fatalf("concurrent results = %+v / %+v, want one creation and one replay of same identity", first.result, second.result)
	}
	assignments, err := firstStore.ListAssignments(ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("assignments after concurrent retry = %+v, want exactly one", assignments)
	}
}

func TestStore_AcceptHandoffWithFollowUpDifferentKeysCompeteOnToken(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "handoff-follow-up-compete.db")
	firstStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	defer firstStore.Close()
	fixture := seedHandoffAuthorityFixture(t, firstStore, core.ExecutionMCPPull)
	secondStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer secondStore.Close()
	baseCommand := core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         fixture.project.ID,
		WorkItemID:        fixture.work.ID,
		HandoffID:         fixture.handoff.ID,
		ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}
	type commandResult struct {
		result core.HandoffFollowUpResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan commandResult, 2)
	for index, candidate := range []*Store{firstStore, secondStore} {
		index, candidate := index, candidate
		go func() {
			<-start
			command := baseCommand
			command.IdempotencyKey = fmt.Sprintf("distinct-command-%d", index)
			result, err := candidate.AcceptHandoffWithFollowUp(ctx, command, fmt.Sprintf("asgn_distinct_%d", index), func() time.Time { return fixture.handoff.UpdatedAt })
			results <- commandResult{result: result, err: err}
		}()
	}
	close(start)
	first := <-results
	second := <-results
	successes := 0
	conflicts := 0
	var createdID string
	for _, result := range []commandResult{first, second} {
		switch {
		case result.err == nil:
			successes++
			createdID = result.result.Assignment.ID
		case errors.Is(result.err, core.ErrConflict):
			conflicts++
		default:
			t.Fatalf("different-key race error = %v, want nil or ErrConflict", result.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("different-key race results = %+v / %+v, want one success and one conflict", first, second)
	}
	assignments, err := firstStore.ListAssignments(ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != createdID {
		t.Fatalf("assignments after different-key race = %+v, want sole winner %s", assignments, createdID)
	}
}

func TestStore_HandoffUpdateAndDeleteSerializeAcrossHandles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "handoff-update-delete-race.db")
	firstStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	defer firstStore.Close()
	fixture := seedHandoffAuthorityFixture(t, firstStore, core.ExecutionMCPPull)
	secondStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer secondStore.Close()

	start := make(chan struct{})
	updateResult := make(chan error, 1)
	deleteResult := make(chan error, 1)
	updatedTitle := "Concurrent authoritative edit"
	go func() {
		<-start
		_, err := firstStore.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID, core.HandoffUpdate{
			ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
			Patch:             core.HandoffPatch{Title: &updatedTitle},
		}, func() time.Time { return fixture.handoff.UpdatedAt })
		updateResult <- err
	}()
	go func() {
		<-start
		deleteResult <- secondStore.DeleteHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID, core.HandoffDelete{ExpectedUpdatedAt: fixture.handoff.UpdatedAt})
	}()
	close(start)
	updateErr := <-updateResult
	deleteErr := <-deleteResult
	if updateErr == nil {
		if !errors.Is(deleteErr, core.ErrConflict) {
			t.Fatalf("delete after winning update error = %v, want ErrConflict", deleteErr)
		}
		current, err := firstStore.GetHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff(after update win) error = %v", err)
		}
		if current.Title != updatedTitle || !current.UpdatedAt.After(fixture.handoff.UpdatedAt) {
			t.Fatalf("handoff after update win = %+v, want monotonic authoritative edit", current)
		}
		return
	}
	if deleteErr != nil {
		t.Fatalf("update/delete race errors = update %v / delete %v, want exactly one success", updateErr, deleteErr)
	}
	if !errors.Is(updateErr, core.ErrNotFound) && !errors.Is(updateErr, core.ErrConflict) {
		t.Fatalf("update after winning delete error = %v, want ErrNotFound or ErrConflict", updateErr)
	}
	if _, err := firstStore.GetHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetHandoff(after delete win) error = %v, want ErrNotFound", err)
	}
}

func TestStore_AcceptHandoffWithFollowUpDerivesCrossWorkRootAndRoleDefaults(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "handoff-follow-up-cross-work.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{
		Name: "Cross-work follow-up",
		Roots: []core.Root{
			{ID: "root_source", Path: "/tmp/source", Kind: "workspace", Active: true},
			{ID: "root_target", Path: "/tmp/target", Kind: "workspace", Active: true},
		},
		DefaultRootID: "root_source",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:            project.ID,
		Name:                 "Cross-work owner",
		DefaultSkillIDs:      []string{"investigate", "report"},
		DefaultExecutionMode: core.ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	sourceWork, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Source work", RootID: "root_source"})
	if err != nil {
		t.Fatalf("CreateWorkItem(source) error = %v", err)
	}
	targetWork, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Target work", RootID: "root_target"})
	if err != nil {
		t.Fatalf("CreateWorkItem(target) error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, core.Handoff{
		ProjectID:        project.ID,
		WorkItemID:       sourceWork.ID,
		ToRoleID:         role.ID,
		TargetWorkItemID: targetWork.ID,
		Title:            "Continue in target work",
		Body:             "Use the target work root and role defaults.",
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	result, err := store.AcceptHandoffWithFollowUp(ctx, core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         project.ID,
		WorkItemID:        sourceWork.ID,
		HandoffID:         handoff.ID,
		ExpectedUpdatedAt: handoff.UpdatedAt,
		IdempotencyKey:    "cross-work-root",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}, "asgn_cross_work", func() time.Time { return handoff.UpdatedAt })
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
	}
	if result.Assignment.WorkItemID != targetWork.ID || result.Assignment.RootID != "root_target" || result.Assignment.RoleID != role.ID || result.Assignment.ExecutionMode != core.ExecutionMCPPull || result.Assignment.DesiredAgent.Kind != core.DesiredAgentAny || !slices.Equal(result.Assignment.DesiredAgent.SkillIDs, role.DefaultSkillIDs) {
		t.Fatalf("cross-work assignment = %+v, want target root and role defaults", result.Assignment)
	}
	if result.Handoff.TargetWorkItemID != targetWork.ID || result.Handoff.TargetAssignmentID != result.Assignment.ID {
		t.Fatalf("cross-work handoff = %+v, want explicit target links", result.Handoff)
	}
}

func TestStore_AcceptHandoffWithFollowUpRejectsRemovedTargetRoot(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "handoff-follow-up-removed-root.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{
		Name:          "Removed target root",
		Roots:         []core.Root{{ID: "root_target", Path: "/tmp/target", Kind: "workspace", Active: true}},
		DefaultRootID: "root_target",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Owner"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Stale rooted work", RootID: "root_target"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, core.Handoff{ProjectID: project.ID, WorkItemID: work.ID, ToRoleID: role.ID, Title: "Continue", Body: "Do not copy a stale root."})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, _, err := service.DeleteRoot(ctx, project.ID, "root_target"); err != nil {
		t.Fatalf("DeleteRoot() error = %v", err)
	}
	_, err = store.AcceptHandoffWithFollowUp(ctx, core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         project.ID,
		WorkItemID:        work.ID,
		HandoffID:         handoff.ID,
		ExpectedUpdatedAt: handoff.UpdatedAt,
		IdempotencyKey:    "removed-root",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}, "asgn_must_not_exist", time.Now)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v, want ErrNotFound", err)
	}
	assignments, err := store.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("assignments = %+v, want none after removed-root rejection", assignments)
	}
}

func TestStore_AcceptHandoffWithFollowUpLinksExistingAssignment(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "handoff-follow-up-existing.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	fixture := seedHandoffAuthorityFixture(t, store, core.ExecutionMCPPull)
	service := core.NewService(store)
	existing, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:     fixture.project.ID,
		WorkItemID:    fixture.work.ID,
		RoleID:        fixture.role.ID,
		ExecutionMode: core.ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	existing, err = service.CompleteAssignment(ctx, fixture.project.ID, existing.ID, core.AssignmentCompleted, core.ExecutionRef{})
	if err != nil {
		t.Fatalf("CompleteAssignment(existing) error = %v", err)
	}
	linked, err := store.UpdateHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID, core.HandoffUpdate{
		ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
		Patch:             core.HandoffPatch{TargetAssignmentID: &existing.ID},
	}, func() time.Time { return fixture.handoff.UpdatedAt.Add(time.Second) })
	if err != nil {
		t.Fatalf("UpdateHandoff(link target) error = %v", err)
	}
	command := core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         fixture.project.ID,
		WorkItemID:        fixture.work.ID,
		HandoffID:         fixture.handoff.ID,
		ExpectedUpdatedAt: linked.UpdatedAt,
		IdempotencyKey:    "link-existing",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}
	result, err := store.AcceptHandoffWithFollowUp(ctx, command, "asgn_must_not_be_created", func() time.Time { return linked.UpdatedAt.Add(time.Second) })
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
	}
	if result.Outcome != core.HandoffFollowUpLinkedExisting || result.Assignment.ID != existing.ID || result.Assignment.Status != core.AssignmentCompleted || result.Handoff.TargetWorkItemID != existing.WorkItemID || result.Handoff.Status != core.HandoffStatusAccepted {
		t.Fatalf("linked result = %+v, want accepted current completed assignment", result)
	}
	assignments, err := store.ListAssignments(ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != existing.ID {
		t.Fatalf("assignments after linking = %+v, want only existing assignment", assignments)
	}

	alreadyCommand := command
	alreadyCommand.ExpectedUpdatedAt = result.Handoff.UpdatedAt
	alreadyCommand.IdempotencyKey = "already-linked"
	already, err := store.AcceptHandoffWithFollowUp(ctx, alreadyCommand, "asgn_still_must_not_be_created", func() time.Time { return result.Handoff.UpdatedAt.Add(time.Second) })
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(already linked) error = %v", err)
	}
	if already.Outcome != core.HandoffFollowUpAlreadySatisfied || already.Assignment.ID != existing.ID || !already.Handoff.UpdatedAt.Equal(result.Handoff.UpdatedAt) {
		t.Fatalf("already-linked result = %+v, want unchanged authoritative link", already)
	}
}

func TestStore_AcceptHandoffWithFollowUpRollsBackAfterAssignmentInsert(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "handoff-follow-up-rollback.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	fixture := seedHandoffAuthorityFixture(t, store, core.ExecutionMCPPull)
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_follow_up_handoff BEFORE UPDATE ON handoffs WHEN NEW.status = 'accepted' BEGIN SELECT RAISE(ABORT, 'forced handoff failure'); END`); err != nil {
		t.Fatalf("create failure trigger error = %v", err)
	}
	command := core.AcceptHandoffWithFollowUpCommand{
		ProjectID:         fixture.project.ID,
		WorkItemID:        fixture.work.ID,
		HandoffID:         fixture.handoff.ID,
		ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
		IdempotencyKey:    "rollback-retry",
		Intent:            core.HandoffFollowUpIntentAcceptAndEnsure,
	}
	if _, err := store.AcceptHandoffWithFollowUp(ctx, command, "asgn_rolled_back", time.Now); err == nil {
		t.Fatal("AcceptHandoffWithFollowUp() error = nil, want forced failure")
	}
	assignments, err := store.ListAssignments(ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("assignments after rollback = %+v, want none", assignments)
	}
	reloaded, err := store.GetHandoff(ctx, fixture.project.ID, fixture.work.ID, fixture.handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff() error = %v", err)
	}
	if reloaded.Status != core.HandoffStatusOpen || reloaded.TargetAssignmentID != "" || !reloaded.UpdatedAt.Equal(fixture.handoff.UpdatedAt) {
		t.Fatalf("handoff after rollback = %+v, want untouched open handoff", reloaded)
	}
	var receiptCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM command_receipts WHERE project_id = ?`, fixture.project.ID).Scan(&receiptCount); err != nil {
		t.Fatalf("count receipts error = %v", err)
	}
	if receiptCount != 0 {
		t.Fatalf("receipt count after rollback = %d, want 0", receiptCount)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER reject_follow_up_handoff`); err != nil {
		t.Fatalf("drop failure trigger error = %v", err)
	}
	if _, err := store.AcceptHandoffWithFollowUp(ctx, command, "asgn_after_rollback", time.Now); err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(retry after rollback) error = %v", err)
	}
}

type handoffAuthorityFixture struct {
	project core.Project
	work    core.WorkItem
	role    core.Role
	handoff core.Handoff
}

func seedHandoffAuthorityFixture(t *testing.T, store *Store, executionMode string) handoffAuthorityFixture {
	t.Helper()
	ctx := context.Background()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Handoff authority"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:            project.ID,
		Name:                 "Follow-up owner",
		DefaultSkillIDs:      []string{"triage", "evidence"},
		DefaultExecutionMode: executionMode,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Rootless follow-up"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, core.Handoff{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		ToRoleID:   role.ID,
		Title:      "Continue the work",
		Body:       "Create the next portable assignment.",
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	return handoffAuthorityFixture{project: project, work: work, role: role, handoff: handoff}
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

func TestStore_MigrateMakesAssignmentRoleSoftReference(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "role-fk.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(seed) error = %v", err)
	}
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Role FK migration"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Persist assignment"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close seed store error = %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign keys error = %v", err)
	}
	_, err = db.ExecContext(ctx, `DROP TABLE assignments`)
	if err != nil {
		t.Fatalf("drop assignments error = %v", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE assignments (
		project_id TEXT NOT NULL,
		id TEXT NOT NULL,
		work_item_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		execution_mode TEXT NOT NULL,
		status TEXT NOT NULL,
		desired_agent_json TEXT NOT NULL DEFAULT '{}',
		claimed_by TEXT NOT NULL DEFAULT '',
		execution_ref TEXT NOT NULL DEFAULT '',
		context_snapshot_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (project_id, id),
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
		FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE,
		FOREIGN KEY (project_id, role_id) REFERENCES roles(project_id, id) ON DELETE RESTRICT
	)`)
	if err != nil {
		t.Fatalf("create old assignments table error = %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO assignments (project_id, id, work_item_id, role_id, execution_mode, status, desired_agent_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project.ID, "asgn_legacy", work.ID, role.ID, core.ExecutionMCPPull, core.AssignmentQueued, `{}`, now, now)
	if err != nil {
		t.Fatalf("insert legacy assignment error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close old db error = %v", err)
	}

	store, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(migrate) error = %v", err)
	}
	defer store.Close()
	service = core.NewService(store)
	if err := service.DeleteRole(ctx, project.ID, role.ID); err != nil {
		t.Fatalf("DeleteRole(referenced after migration) error = %v", err)
	}
	context, err := service.AssignmentContext(ctx, project.ID, "asgn_legacy")
	if err != nil {
		t.Fatalf("AssignmentContext() after migrated role delete error = %v", err)
	}
	if context.Assignment.RoleID != role.ID || context.Role != nil || !containsString(context.Warnings, "assignment role was not found") {
		t.Fatalf("assignment context after migrated role delete = %+v, want historical assignment with missing-role warning", context)
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
		"default_root_id": true,
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

func TestStore_AssignmentCompareAndSetTransitions(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "assignment-cas.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Assignment transitions"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	firstRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Researcher"})
	if err != nil {
		t.Fatalf("CreateRole(first) error = %v", err)
	}
	secondRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer", DefaultExecutionMode: core.ExecutionManual})
	if err != nil {
		t.Fatalf("CreateRole(second) error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Coordinate once"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		RoleID:       firstRole.ID,
		DesiredAgent: core.DesiredAgent{Kind: core.DesiredAgentAny, SkillIDs: []string{"research"}},
	})
	if err != nil {
		t.Fatalf("CreateAssignment(edit) error = %v", err)
	}
	updated, err := service.UpdateQueuedAssignment(ctx, project.ID, assignment.ID, core.QueuedAssignmentUpdate{
		Expected:          assignment.Coordination(),
		ExpectedUpdatedAt: assignment.UpdatedAt,
		Replacement: core.AssignmentCoordination{
			WorkItemID:    work.ID,
			RoleID:        secondRole.ID,
			ExecutionMode: core.ExecutionManual,
			DesiredAgent:  core.DesiredAgent{Kind: "human"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateQueuedAssignment() error = %v", err)
	}
	if _, err := service.UpdateQueuedAssignment(ctx, project.ID, assignment.ID, core.QueuedAssignmentUpdate{
		Expected:          assignment.Coordination(),
		ExpectedUpdatedAt: assignment.UpdatedAt,
		Replacement: core.AssignmentCoordination{
			WorkItemID:    work.ID,
			RoleID:        firstRole.ID,
			ExecutionMode: core.ExecutionMCPPull,
		},
	}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateQueuedAssignment(stale editor) error = %v, want ErrConflict", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if _, err := service.UpdateQueuedAssignment(ctx, project.ID, assignment.ID, core.QueuedAssignmentUpdate{
		Expected:          updated.Coordination(),
		ExpectedUpdatedAt: updated.UpdatedAt,
		Replacement: core.AssignmentCoordination{
			WorkItemID:    work.ID,
			RoleID:        firstRole.ID,
			ExecutionMode: core.ExecutionMCPPull,
		},
	}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateQueuedAssignment(after claim) error = %v, want ErrConflict", err)
	}
	current, err := service.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if current.Status != core.AssignmentClaimed || current.ClaimedBy != "agent-a" || current.RoleID != secondRole.ID {
		t.Fatalf("assignment after stale update = %+v, want claim and metadata preserved", current)
	}
	contextual, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:         project.ID,
		WorkItemID:        work.ID,
		RoleID:            firstRole.ID,
		ContextSnapshotID: "ctx-already-prepared",
	})
	if err != nil {
		t.Fatalf("CreateAssignment(contextual) error = %v", err)
	}
	if _, err := service.UpdateQueuedAssignment(ctx, project.ID, contextual.ID, core.QueuedAssignmentUpdate{
		Expected:          contextual.Coordination(),
		ExpectedUpdatedAt: contextual.UpdatedAt,
		Replacement: core.AssignmentCoordination{
			WorkItemID:    work.ID,
			RoleID:        secondRole.ID,
			ExecutionMode: core.ExecutionManual,
		},
	}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateQueuedAssignment(context already prepared) error = %v, want ErrConflict", err)
	}

	cancelled, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: firstRole.ID, ExecutionMode: core.ExecutionManual})
	if err != nil {
		t.Fatalf("CreateAssignment(cancelled) error = %v", err)
	}
	cancelled, err = service.CompleteAssignment(ctx, project.ID, cancelled.ID, core.AssignmentCancelled, core.ExecutionRef{})
	if err != nil {
		t.Fatalf("CompleteAssignment(cancelled) error = %v", err)
	}
	if !cancelled.StartedAt.IsZero() || cancelled.CompletedAt.IsZero() {
		t.Fatalf("queued cancellation timestamps = started:%s completed:%s, want finish without fabricated start", cancelled.StartedAt, cancelled.CompletedAt)
	}
	claimedOnly, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: firstRole.ID, ExecutionMode: core.ExecutionManual})
	if err != nil {
		t.Fatalf("CreateAssignment(claimed completion) error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, claimedOnly.ID, "operator-a"); err != nil {
		t.Fatalf("ClaimAssignment(claimed completion) error = %v", err)
	}
	claimedOnly, err = service.CompleteAssignment(ctx, project.ID, claimedOnly.ID, core.AssignmentCompleted, core.ExecutionRef{})
	if err != nil {
		t.Fatalf("CompleteAssignment(claimed completion) error = %v", err)
	}
	if claimedOnly.StartedAt.IsZero() || claimedOnly.CompletedAt.IsZero() {
		t.Fatalf("claimed completion timestamps = started:%s completed:%s, want both recorded", claimedOnly.StartedAt, claimedOnly.CompletedAt)
	}

	terminal, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: firstRole.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(terminal) error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, terminal.ID, "agent-b"); err != nil {
		t.Fatalf("ClaimAssignment(terminal) error = %v", err)
	}
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, terminal.ID, core.AssignmentRunning, core.ExecutionRef{}); err != nil {
		t.Fatalf("UpdateAssignmentStatus(terminal) error = %v", err)
	}
	var wins atomic.Int32
	var conflicts atomic.Int32
	var wg sync.WaitGroup
	for _, status := range []string{core.AssignmentCompleted, core.AssignmentFailed} {
		wg.Add(1)
		go func(status string) {
			defer wg.Done()
			item, err := service.CompleteAssignment(ctx, project.ID, terminal.ID, status, core.ExecutionRef{})
			switch {
			case err == nil:
				if item.Status != status {
					t.Errorf("CompleteAssignment(%s) returned status %q", status, item.Status)
				}
				wins.Add(1)
			case errors.Is(err, core.ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("CompleteAssignment(%s) unexpected error = %v", status, err)
			}
		}(status)
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != 1 {
		t.Fatalf("terminal transition wins=%d conflicts=%d, want one of each", wins.Load(), conflicts.Load())
	}
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, terminal.ID, core.AssignmentRunning, core.ExecutionRef{}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateAssignmentStatus(after terminal) error = %v, want ErrConflict", err)
	}

	progressRace, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: firstRole.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(progress race) error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, progressRace.ID, "agent-c"); err != nil {
		t.Fatalf("ClaimAssignment(progress race) error = %v", err)
	}
	start := make(chan struct{})
	type transitionResult struct {
		assignment core.Assignment
		err        error
	}
	progressResult := make(chan transitionResult, 1)
	completeResult := make(chan transitionResult, 1)
	go func() {
		<-start
		item, err := service.UpdateAssignmentStatus(ctx, project.ID, progressRace.ID, core.AssignmentReview, core.ExecutionRef{})
		progressResult <- transitionResult{assignment: item, err: err}
	}()
	go func() {
		<-start
		item, err := service.CompleteAssignment(ctx, project.ID, progressRace.ID, core.AssignmentCompleted, core.ExecutionRef{})
		completeResult <- transitionResult{assignment: item, err: err}
	}()
	close(start)
	completedResult := <-completeResult
	if completedResult.err != nil {
		t.Fatalf("CompleteAssignment(progress race) error = %v", completedResult.err)
	}
	if completedResult.assignment.Status != core.AssignmentCompleted {
		t.Fatalf("CompleteAssignment(progress race) returned status = %q, want completed", completedResult.assignment.Status)
	}
	progressedResult := <-progressResult
	if progressedResult.err != nil && !errors.Is(progressedResult.err, core.ErrConflict) {
		t.Fatalf("UpdateAssignmentStatus(progress race) error = %v, want nil or ErrConflict", progressedResult.err)
	}
	if progressedResult.err == nil && progressedResult.assignment.Status != core.AssignmentReview {
		t.Fatalf("UpdateAssignmentStatus(progress race) returned status = %q, want awaiting_review", progressedResult.assignment.Status)
	}
	progressRace, err = service.GetAssignment(ctx, project.ID, progressRace.ID)
	if err != nil {
		t.Fatalf("GetAssignment(progress race) error = %v", err)
	}
	if progressRace.Status != core.AssignmentCompleted {
		t.Fatalf("progress race status = %q, want completed", progressRace.Status)
	}
	if progressRace.StartedAt.After(progressRace.CompletedAt) || progressRace.CompletedAt.After(progressRace.UpdatedAt) {
		t.Fatalf("progress race timestamps = started:%s completed:%s updated:%s, want monotone lifecycle", progressRace.StartedAt, progressRace.CompletedAt, progressRace.UpdatedAt)
	}
}

func TestStore_AssignmentTransitionTimestampPreventsQueuedABA(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "assignment-transition-token.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Assignment transition token"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Operator"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Do not accept stale edits"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	original, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:     project.ID,
		WorkItemID:    work.ID,
		RoleID:        role.ID,
		ExecutionMode: core.ExecutionManual,
		DesiredAgent:  core.DesiredAgent{Kind: "human", SkillIDs: []string{"review"}},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	fixedNow := func() time.Time { return original.UpdatedAt }
	claimed, err := store.ClaimAssignment(ctx, project.ID, original.ID, "operator-a", fixedNow)
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	released, err := store.ReleaseAssignment(ctx, project.ID, original.ID, "operator-a", fixedNow)
	if err != nil {
		t.Fatalf("ReleaseAssignment() error = %v", err)
	}
	if !claimed.UpdatedAt.After(original.UpdatedAt) || !released.UpdatedAt.After(claimed.UpdatedAt) {
		t.Fatalf("transition timestamps = original:%s claimed:%s released:%s, want strict advancement", original.UpdatedAt, claimed.UpdatedAt, released.UpdatedAt)
	}
	if _, err := store.UpdateQueuedAssignment(ctx, project.ID, original.ID, core.QueuedAssignmentUpdate{
		Expected:          original.Coordination(),
		ExpectedUpdatedAt: original.UpdatedAt,
		Replacement:       original.Coordination(),
	}, fixedNow); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateQueuedAssignment(stale after claim/release) error = %v, want ErrConflict", err)
	}
}

func TestStore_CompleteAssignmentCanSubmitQueuedOrClaimedForReview(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "assignment-review.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Review submission"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Submit for review"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	for _, initial := range []string{core.AssignmentQueued, core.AssignmentClaimed} {
		assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID, ExecutionMode: core.ExecutionManual})
		if err != nil {
			t.Fatalf("CreateAssignment(%s) error = %v", initial, err)
		}
		if initial == core.AssignmentClaimed {
			if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "operator-a"); err != nil {
				t.Fatalf("ClaimAssignment() error = %v", err)
			}
		}
		review, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, core.AssignmentReview, core.ExecutionRef{})
		if err != nil {
			t.Fatalf("CompleteAssignment(%s to review) error = %v", initial, err)
		}
		if review.Status != core.AssignmentReview || review.StartedAt.IsZero() || !review.CompletedAt.IsZero() {
			t.Fatalf("%s to review assignment = %+v, want started nonterminal review", initial, review)
		}
	}
}

func TestStore_AssignmentTransitionSurvivesCancelledRequest(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "assignment-cancelled-transition.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Cancelled transition"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Operator"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Keep the store usable"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	cancelledCtx, cancel := context.WithCancel(ctx)
	transition, _, err := store.beginAssignmentTransition(cancelledCtx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("beginAssignmentTransition() error = %v", err)
	}
	cancel()
	if err := transition.Rollback(); err != nil {
		t.Fatalf("Rollback() after cancellation error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "operator-a"); err != nil {
		t.Fatalf("ClaimAssignment() after cancelled transition error = %v", err)
	}
}

func TestStore_DiscardedTransitionConnectionKeepsSafetyPragmas(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "assignment-discarded-connection.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Discarded transition connection"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Operator"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Keep connection safety"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	transition, _, err := store.beginAssignmentTransition(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("beginAssignmentTransition() error = %v", err)
	}
	transition.discard()

	var foreignKeys, busyTimeout int
	if err := store.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("PRAGMA foreign_keys error = %v", err)
	}
	if err := store.db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout error = %v", err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("replacement connection pragmas = foreign_keys:%d busy_timeout:%d, want 1 and 5000", foreignKeys, busyTimeout)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "operator-a"); err != nil {
		t.Fatalf("ClaimAssignment() after discard error = %v", err)
	}
}

func TestStore_AssignmentTransitionsSerializeAcrossStoreHandles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "assignment-multi-store.db")
	firstStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	defer firstStore.Close()
	service := core.NewService(firstStore)
	project, err := service.CreateProject(ctx, core.Project{Name: "Multi-store transition"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Operator"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Claim once"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	secondStore, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer secondStore.Close()
	heldTransition, _, err := firstStore.beginAssignmentTransition(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("beginAssignmentTransition(first handle) error = %v", err)
	}
	if _, err := secondStore.db.ExecContext(ctx, `PRAGMA busy_timeout = 1`); err != nil {
		t.Fatalf("set short busy timeout error = %v", err)
	}
	if _, _, err := secondStore.beginAssignmentTransition(ctx, project.ID, assignment.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("beginAssignmentTransition(locked second handle) error = %v, want ErrConflict", err)
	}
	if err := heldTransition.Rollback(); err != nil {
		t.Fatalf("Rollback(first handle) error = %v", err)
	}
	if _, err := secondStore.db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
		t.Fatalf("restore busy timeout error = %v", err)
	}

	if _, err := firstStore.ClaimAssignment(ctx, project.ID, assignment.ID, "operator-a", func() time.Time { return assignment.UpdatedAt }); err != nil {
		t.Fatalf("first ClaimAssignment() error = %v", err)
	}
	if _, err := secondStore.ClaimAssignment(ctx, project.ID, assignment.ID, "operator-b", func() time.Time { return assignment.UpdatedAt }); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("second ClaimAssignment() error = %v, want ErrConflict", err)
	}
}

func TestStore_CreateRecordsValidateReferences(t *testing.T) {
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
	if _, err := store.CreateAssignment(ctx, core.Assignment{
		ID:         "asgn_direct_missing_role",
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     "role_missing",
	}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.CreateAssignment(missing role) error = %v, want ErrNotFound", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID: project.ID,
		Name:      "Reviewer",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment(valid) error = %v", err)
	}
	if _, err := store.CreateAssignment(ctx, core.Assignment{
		ID:         "asgn_direct_missing_work",
		ProjectID:  project.ID,
		WorkItemID: "work_missing",
		RoleID:     role.ID,
	}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.CreateAssignment(missing work item) error = %v, want ErrNotFound", err)
	}
	if _, err := store.CreateEvidence(ctx, core.Evidence{
		ID:         "ev_direct_missing_work",
		ProjectID:  project.ID,
		WorkItemID: "work_missing",
		Title:      "Missing work evidence",
		Locator:    "file://missing.md",
	}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.CreateEvidence(missing work) error = %v, want ErrNotFound", err)
	}
	otherWork, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Other evidence work"})
	if err != nil {
		t.Fatalf("CreateWorkItem(other) error = %v", err)
	}
	otherAssignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: otherWork.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(other) error = %v", err)
	}
	if _, err := store.CreateEvidence(ctx, core.Evidence{
		ID:           "ev_direct_wrong_assignment",
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: otherAssignment.ID,
		Title:        "Wrong assignment evidence",
		Locator:      "file://wrong.md",
	}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.CreateEvidence(wrong assignment) error = %v, want ErrNotFound", err)
	}
	if _, err := store.CreateEvidence(ctx, core.Evidence{
		ID:           "ev_direct_valid",
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Valid direct evidence",
		Locator:      "file://valid.md",
	}); err != nil {
		t.Fatalf("store.CreateEvidence(valid) error = %v", err)
	}
	if _, err := store.CreateReview(ctx, core.Review{
		ID:         "rev_direct_missing_work",
		ProjectID:  project.ID,
		WorkItemID: "work_missing",
		Body:       "Missing work review.",
		Verdict:    core.ReviewVerdictApproved,
		Status:     core.ReviewStatusRecorded,
	}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.CreateReview(missing work) error = %v, want ErrNotFound", err)
	}
	if _, err := store.CreateHandoff(ctx, core.Handoff{
		ID:         "handoff_direct_missing_work",
		ProjectID:  project.ID,
		WorkItemID: "work_missing",
		Title:      "Missing work handoff",
		Body:       "Missing work handoff should be rejected.",
		Status:     core.HandoffStatusOpen,
	}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.CreateHandoff(missing work) error = %v, want ErrNotFound", err)
	}
	if _, err := store.UpdateHandoff(ctx, project.ID, "work_missing", "handoff_direct_missing_work", core.HandoffUpdate{}, time.Now); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("store.UpdateHandoff(missing work) error = %v, want ErrNotFound", err)
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

func boolPointerForStoreTest(value bool) *bool {
	return &value
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
