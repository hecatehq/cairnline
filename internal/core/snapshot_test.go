package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestServiceSnapshotRoundTripMemoryStore(t *testing.T) {
	ctx := context.Background()
	source := NewService(NewMemoryStore())
	fixture := createSnapshotFixture(t, ctx, source)

	exported, err := source.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	if exported.Version != SnapshotVersion || exported.ExportedAt.IsZero() {
		t.Fatalf("snapshot header = version %d exported_at %s, want current version and export time", exported.Version, exported.ExportedAt)
	}

	target := NewService(NewMemoryStore())
	imported, err := target.ImportSnapshot(ctx, exported)
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if !snapshotsEqualIgnoringExportedAt(exported, imported) {
		t.Fatalf("imported snapshot mismatch\nexported: %+v\nimported: %+v", exported, imported)
	}
	if _, err := target.GetWorkItem(ctx, fixture.projectID, fixture.proposalWorkID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkItem(proposal action) error = %v, want ErrNotFound so proposal import does not replay actions", err)
	}

	importedAgain, err := target.ImportSnapshot(ctx, exported)
	if err != nil {
		t.Fatalf("ImportSnapshot(idempotent) error = %v", err)
	}
	if !snapshotsEqualIgnoringExportedAt(exported, importedAgain) {
		t.Fatalf("idempotent imported snapshot mismatch\nexported: %+v\nimported: %+v", exported, importedAgain)
	}
}

func TestServiceSnapshotImportPreservesTargetOnlyAssignmentHistory(t *testing.T) {
	ctx := context.Background()
	source := NewService(NewMemoryStore())
	fixture := createSnapshotFixture(t, ctx, source)
	snapshot, err := source.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	target := NewService(NewMemoryStore())
	if _, err := target.ImportSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("ImportSnapshot(first) error = %v", err)
	}
	if _, err := target.CreateArtifact(ctx, Artifact{
		ID:           "art_target_only",
		ProjectID:    fixture.projectID,
		WorkItemID:   "work_snapshot",
		AssignmentID: "asgn_snapshot",
		Kind:         "decision_note",
		Title:        "Target-only decision",
		Body:         "Keep this target-only assignment history.",
	}); err != nil {
		t.Fatalf("CreateArtifact(target only) error = %v", err)
	}
	if _, err := target.CreateReview(ctx, Review{
		ID:             "rev_target_only",
		ProjectID:      fixture.projectID,
		WorkItemID:     "work_snapshot",
		AssignmentID:   "asgn_snapshot",
		ReviewerRoleID: "role_architect",
		Title:          "Target-only review",
		Body:           "Keep this target-only review history.",
		Verdict:        ReviewVerdictPass,
	}); err != nil {
		t.Fatalf("CreateReview(target only) error = %v", err)
	}
	if _, err := target.ImportSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("ImportSnapshot(second) error = %v", err)
	}
	artifacts, err := target.ListArtifacts(ctx, fixture.projectID, "work_snapshot")
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}
	if !artifactIDPresent(artifacts, "art_target_only") {
		t.Fatalf("artifacts = %+v, want target-only assignment history preserved", artifacts)
	}
	reviews, err := target.ListReviews(ctx, fixture.projectID, "work_snapshot")
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if !reviewIDPresent(reviews, "rev_target_only") {
		t.Fatalf("reviews = %+v, want target-only assignment history preserved", reviews)
	}
}

func artifactIDPresent(items []Artifact, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func reviewIDPresent(items []Review, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func TestServiceSnapshotRejectsUnsupportedVersion(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	if _, err := service.ImportSnapshot(ctx, Snapshot{Version: SnapshotVersion + 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ImportSnapshot(unsupported version) error = %v, want ErrInvalid", err)
	}
}

type snapshotFixtureIDs struct {
	projectID      string
	proposalWorkID string
}

func createSnapshotFixture(t *testing.T, ctx context.Context, service *Service) snapshotFixtureIDs {
	t.Helper()

	base := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	project, err := service.CreateProject(ctx, Project{
		ID:            "proj_snapshot",
		Name:          "Snapshot project",
		Description:   "Portable migration fixture.",
		DefaultRootID: "root_main",
		Roots: []Root{{
			ID:        "root_main",
			Path:      "/tmp/cairnline-snapshot",
			Kind:      "workspace",
			GitRemote: "https://example.test/repo.git",
			GitBranch: "main",
			Active:    true,
		}},
		ContextSources: []Source{{
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
	if _, err := service.CreateProjectSkill(ctx, ProjectSkill{
		ProjectID:      project.ID,
		ID:             "planning",
		Title:          "Planning",
		Description:    "Break down project work.",
		Path:           ".agents/skills/planning/SKILL.md",
		RootID:         "root_main",
		Format:         SkillFormatMarkdown,
		SuggestedTools: []string{"project.read", "work.create"},
		RequiredPermissions: RequiredPermissions{
			Tools:   boolPointerForSnapshotTest(true),
			Writes:  boolPointerForSnapshotTest(false),
			Network: boolPointerForSnapshotTest(false),
		},
		Status:       SkillStatusAvailable,
		TrustLabel:   SkillTrustWorkspace,
		SourceRefs:   []string{"src_agents"},
		Warnings:     []string{"metadata only"},
		DiscoveredAt: base.Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ID:                   "role_architect",
		ProjectID:            project.ID,
		Name:                 "Architect",
		Description:          "Owns project shape.",
		Instructions:         "Design before dispatch.",
		DefaultSkillIDs:      []string{"planning"},
		DefaultExecutionMode: ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{
		ID:              "work_snapshot",
		ProjectID:       project.ID,
		Title:           "Validate snapshot migration",
		Brief:           "Export and import project coordination state.",
		Priority:        PriorityNormal,
		OwnerRoleID:     role.ID,
		ReviewerRoleIDs: []string{role.ID},
		RootID:          "root_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ID:                "asgn_snapshot",
		ProjectID:         project.ID,
		WorkItemID:        work.ID,
		RoleID:            role.ID,
		RootID:            "root_main",
		ExecutionMode:     ExecutionMCPPull,
		DesiredAgent:      DesiredAgent{Kind: DesiredAgentAny, SkillIDs: []string{"planning"}},
		ContextSnapshotID: "ctx_snapshot",
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	assignment.Status = AssignmentCompleted
	assignment.ClaimedBy = "agent:test"
	assignment.ExecutionRef = ExecutionRef{Kind: "task_run", TaskID: "task_snapshot", RunID: "run_snapshot", SessionID: "sess_snapshot", TraceID: "trace_snapshot", PendingApprovals: 2}
	assignment.StartedAt = base.Add(2 * time.Minute)
	assignment.CompletedAt = base.Add(4 * time.Minute)
	assignment.UpdatedAt = assignment.CompletedAt
	if _, err := service.store.RestoreAssignmentSnapshot(ctx, assignment); err != nil {
		t.Fatalf("store.RestoreAssignmentSnapshot() error = %v", err)
	}
	if _, err := service.CreateArtifact(ctx, Artifact{
		ID:             "art_snapshot",
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		Kind:           "decision_note",
		Title:          "Snapshot decision",
		Body:           "Use Cairnline snapshots for migration rehearsal.",
		AuthorRoleID:   role.ID,
		ProvenanceKind: "operator",
		TrustLabel:     "operator_reviewed",
		CreatedAt:      base.Add(5 * time.Minute),
		UpdatedAt:      base.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateArtifact() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, Evidence{
		ID:           "ev_snapshot",
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Title:        "Snapshot evidence",
		Locator:      "https://example.test/evidence",
		SourceKind:   "pull_request",
		ExternalID:   "PR 7",
		Provider:     "github",
		CreatedAt:    base.Add(6 * time.Minute),
		UpdatedAt:    base.Add(6 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, Review{
		ID:             "rev_snapshot",
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		ReviewerRoleID: role.ID,
		Title:          "Snapshot review",
		Body:           "Migration fixture preserves review metadata.",
		Verdict:        ReviewVerdictChangesRequested,
		Risk:           ReviewRiskMedium,
		CreatedAt:      base.Add(7 * time.Minute),
		UpdatedAt:      base.Add(7 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, Handoff{
		ID:                    "handoff_snapshot",
		ProjectID:             project.ID,
		WorkItemID:            work.ID,
		SourceAssignmentID:    assignment.ID,
		SourceRunID:           "run_snapshot",
		SourceChatSessionID:   "chat_snapshot",
		SourceMessageID:       "msg_snapshot",
		FromRoleID:            role.ID,
		ToRoleID:              role.ID,
		TargetAssignmentID:    assignment.ID,
		TargetWorkItemID:      work.ID,
		Title:                 "Snapshot handoff",
		Body:                  "Continue from imported state.",
		RecommendedNextAction: "Review the imported snapshot.",
		LinkedArtifactIDs:     []string{"art_snapshot", "ev_snapshot", "rev_snapshot"},
		LinkedMemoryIDs:       []string{"mem_snapshot"},
		ContextRefs:           []string{"ctx_snapshot"},
		Status:                HandoffStatusAccepted,
		ProvenanceKind:        "operator",
		TrustLabel:            "operator_reviewed",
		CreatedAt:             base.Add(8 * time.Minute),
		UpdatedAt:             base.Add(9 * time.Minute),
		StatusChangedAt:       base.Add(8*time.Minute + 30*time.Second),
	}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	entry, err := service.CreateMemoryEntry(ctx, MemoryEntry{
		ID:         "mem_snapshot",
		ProjectID:  project.ID,
		Title:      "Snapshot memory",
		Body:       "Migration snapshots preserve durable memory.",
		TrustLabel: MemoryTrustOperator,
		SourceKind: MemorySourceOperator,
		SourceID:   assignment.ID,
		CreatedAt:  base.Add(10 * time.Minute),
		UpdatedAt:  base.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	entry.Enabled = false
	entry.UpdatedAt = base.Add(11 * time.Minute)
	if _, err := service.UpdateMemoryEntry(ctx, entry); err != nil {
		t.Fatalf("UpdateMemoryEntry() error = %v", err)
	}
	candidate, err := service.CreateMemoryCandidate(ctx, MemoryCandidate{
		ID:                  "memcand_snapshot",
		ProjectID:           project.ID,
		Title:               "Snapshot candidate",
		Body:                "Candidate state should survive import.",
		SuggestedKind:       "project_lesson",
		SuggestedTrustLabel: MemoryTrustGenerated,
		SuggestedSourceKind: "assignment",
		SuggestedSourceID:   assignment.ID,
		SourceRefs:          []MemoryCandidateSourceRef{{Kind: "assignment", ID: assignment.ID, Title: "Completed assignment"}},
		CreatedAt:           base.Add(12 * time.Minute),
		UpdatedAt:           base.Add(12 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	candidate.Status = MemoryCandidateRejected
	candidate.StatusReason = "not durable enough"
	candidate.UpdatedAt = base.Add(13 * time.Minute)
	if _, err := service.UpdateMemoryCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpdateMemoryCandidate() error = %v", err)
	}

	proposalWorkID := "work_from_proposal"
	if _, err := service.ImportAssistantProposalRecord(ctx, AssistantProposalRecord{
		ID:        "prop_snapshot",
		ProjectID: project.ID,
		Source:    AssistantProposalSourceAssistant,
		Proposal: AssistantProposal{
			ID:                   "prop_snapshot",
			ProjectID:            project.ID,
			Title:                "Would create more work",
			Summary:              "This proposal is history only in a snapshot import.",
			Source:               AssistantProposalSourceAssistant,
			RequiresConfirmation: true,
			CreatedAt:            base.Add(14 * time.Minute),
			Actions: []AssistantAction{{
				Kind:     AssistantActionCreateWorkItem,
				WorkItem: &WorkItem{ID: proposalWorkID, ProjectID: project.ID, Title: "Should not be replayed"},
			}},
		},
		Status:    AssistantProposalStatusProposed,
		CreatedAt: base.Add(14 * time.Minute),
		UpdatedAt: base.Add(14 * time.Minute),
	}); err != nil {
		t.Fatalf("ImportAssistantProposalRecord() error = %v", err)
	}
	return snapshotFixtureIDs{projectID: project.ID, proposalWorkID: proposalWorkID}
}

func snapshotsEqualIgnoringExportedAt(a, b Snapshot) bool {
	a.ExportedAt = time.Time{}
	b.ExportedAt = time.Time{}
	return reflect.DeepEqual(a, b)
}

func boolPointerForSnapshotTest(value bool) *bool {
	return &value
}
