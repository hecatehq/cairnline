package sqlitestore

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hecatehq/cairnline/internal/core"
)

func TestStore_SnapshotImportExportRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snapshot.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	snapshot := sqliteSnapshotFixture()
	imported, err := service.ImportSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if !sqliteSnapshotsEqualIgnoringExportedAt(snapshot, imported) {
		t.Fatalf("imported snapshot mismatch\nsnapshot: %+v\nimported: %+v", snapshot, imported)
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
	reopenedSnapshot, err := reopenedService.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot(reopened) error = %v", err)
	}
	if !sqliteSnapshotsEqualIgnoringExportedAt(snapshot, reopenedSnapshot) {
		t.Fatalf("reopened snapshot mismatch\nsnapshot: %+v\nreopened: %+v", snapshot, reopenedSnapshot)
	}
	if _, err := reopenedService.GetWorkItem(ctx, "proj_sqlite_snapshot", "work_from_proposal"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetWorkItem(proposal action) error = %v, want ErrNotFound so proposal import does not replay actions", err)
	}
}

func TestStore_SnapshotImportPreservesTargetOnlyAssignmentHistory(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "snapshot-target-history.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	snapshot := sqliteSnapshotFixture()
	if _, err := service.ImportSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("ImportSnapshot(first) error = %v", err)
	}
	if _, err := service.CreateArtifact(ctx, core.Artifact{
		ID:           "art_sqlite_target_only",
		ProjectID:    "proj_sqlite_snapshot",
		WorkItemID:   "work_sqlite",
		AssignmentID: "asgn_sqlite",
		Kind:         "decision_note",
		Title:        "Target-only SQLite decision",
		Body:         "Keep this target-only assignment history.",
	}); err != nil {
		t.Fatalf("CreateArtifact(target only) error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{
		ID:             "rev_sqlite_target_only",
		ProjectID:      "proj_sqlite_snapshot",
		WorkItemID:     "work_sqlite",
		AssignmentID:   "asgn_sqlite",
		ReviewerRoleID: "role_sqlite",
		Title:          "Target-only SQLite review",
		Body:           "Keep this target-only review history.",
		Verdict:        core.ReviewVerdictPass,
	}); err != nil {
		t.Fatalf("CreateReview(target only) error = %v", err)
	}
	if _, err := service.ImportSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("ImportSnapshot(second) error = %v", err)
	}
	artifacts, err := service.ListArtifacts(ctx, "proj_sqlite_snapshot", "work_sqlite")
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}
	foundArtifact := false
	for _, artifact := range artifacts {
		foundArtifact = foundArtifact || artifact.ID == "art_sqlite_target_only"
	}
	if !foundArtifact {
		t.Fatalf("artifacts = %+v, want target-only assignment history preserved", artifacts)
	}
	reviews, err := service.ListReviews(ctx, "proj_sqlite_snapshot", "work_sqlite")
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	foundReview := false
	for _, review := range reviews {
		foundReview = foundReview || review.ID == "rev_sqlite_target_only"
	}
	if !foundReview {
		t.Fatalf("reviews = %+v, want target-only assignment history preserved", reviews)
	}
}

func sqliteSnapshotFixture() core.Snapshot {
	base := time.Date(2026, 6, 28, 15, 0, 0, 0, time.UTC)
	return core.Snapshot{
		Version:    core.SnapshotVersion,
		ExportedAt: base.Add(30 * time.Minute),
		Projects: []core.Project{{
			ID:            "proj_sqlite_snapshot",
			Name:          "SQLite snapshot",
			Description:   "Persistent snapshot fixture.",
			DefaultRootID: "root_main",
			Roots: []core.Root{{
				ID:        "root_main",
				Path:      "/tmp/sqlite-snapshot",
				Kind:      "workspace",
				GitRemote: "https://example.test/sqlite.git",
				GitBranch: "main",
				Active:    true,
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
				CreatedAt:      base,
				UpdatedAt:      base.Add(time.Minute),
			}},
			CreatedAt: base,
			UpdatedAt: base.Add(time.Minute),
		}},
		ProjectSkills: []core.ProjectSkill{{
			ID:             "sqlite-skill",
			ProjectID:      "proj_sqlite_snapshot",
			Title:          "SQLite skill",
			Description:    "A persisted skill record.",
			Path:           ".agents/skills/sqlite/SKILL.md",
			RootID:         "root_main",
			Format:         core.SkillFormatMarkdown,
			SuggestedTools: []string{"file.read"},
			RequiredPermissions: core.RequiredPermissions{
				Tools:   boolPointerForSnapshotStoreTest(true),
				Writes:  boolPointerForSnapshotStoreTest(false),
				Network: boolPointerForSnapshotStoreTest(false),
			},
			Enabled:      true,
			Status:       core.SkillStatusAvailable,
			TrustLabel:   core.SkillTrustWorkspace,
			SourceRefs:   []string{"src_agents"},
			Warnings:     []string{"metadata only"},
			DiscoveredAt: base.Add(2 * time.Minute),
			CreatedAt:    base.Add(2 * time.Minute),
			UpdatedAt:    base.Add(2 * time.Minute),
		}},
		Roles: []core.Role{{
			ID:                   "role_sqlite",
			ProjectID:            "proj_sqlite_snapshot",
			Name:                 "SQLite role",
			Description:          "Owns the migration check.",
			Instructions:         "Preserve coordination records.",
			DefaultSkillIDs:      []string{"sqlite-skill"},
			DefaultExecutionMode: core.ExecutionMCPPull,
		}},
		WorkItems: []core.WorkItem{{
			ID:              "work_sqlite",
			ProjectID:       "proj_sqlite_snapshot",
			Title:           "Persist snapshot",
			Brief:           "Round-trip a snapshot through SQLite.",
			Status:          core.WorkStatusReady,
			Priority:        core.PriorityNormal,
			OwnerRoleID:     "role_sqlite",
			ReviewerRoleIDs: []string{"role_sqlite"},
			RootID:          "root_main",
			CreatedAt:       base.Add(3 * time.Minute),
			UpdatedAt:       base.Add(4 * time.Minute),
		}},
		Assignments: []core.Assignment{{
			ID:                "asgn_sqlite",
			ProjectID:         "proj_sqlite_snapshot",
			WorkItemID:        "work_sqlite",
			RoleID:            "role_sqlite",
			RootID:            "root_main",
			ExecutionMode:     core.ExecutionMCPPull,
			Status:            core.AssignmentCompleted,
			DesiredAgent:      core.DesiredAgent{Kind: core.DesiredAgentAny, SkillIDs: []string{"sqlite-skill"}},
			ClaimedBy:         "agent:sqlite",
			ExecutionRef:      core.ExecutionRef{Kind: "task_run", TaskID: "task_sqlite", RunID: "run_sqlite", TraceID: "trace_sqlite", PendingApprovals: 1},
			ContextSnapshotID: "ctx_sqlite",
			CreatedAt:         base.Add(5 * time.Minute),
			UpdatedAt:         base.Add(8 * time.Minute),
			StartedAt:         base.Add(6 * time.Minute),
			CompletedAt:       base.Add(8 * time.Minute),
		}},
		Artifacts: []core.Artifact{{
			ID:             "art_sqlite",
			ProjectID:      "proj_sqlite_snapshot",
			WorkItemID:     "work_sqlite",
			AssignmentID:   "asgn_sqlite",
			Kind:           "decision_note",
			Title:          "SQLite artifact",
			Body:           "Snapshot import stores generic artifacts.",
			AuthorRoleID:   "role_sqlite",
			ProvenanceKind: "operator",
			TrustLabel:     "operator_reviewed",
			CreatedAt:      base.Add(9 * time.Minute),
			UpdatedAt:      base.Add(9 * time.Minute),
		}},
		Evidence: []core.Evidence{{
			ID:           "ev_sqlite",
			ProjectID:    "proj_sqlite_snapshot",
			WorkItemID:   "work_sqlite",
			AssignmentID: "asgn_sqlite",
			Title:        "SQLite evidence",
			Locator:      "https://example.test/sqlite-evidence",
			SourceKind:   "pull_request",
			ExternalID:   "PR 8",
			Provider:     "github",
			TrustLabel:   core.EvidenceTrustOperator,
			CreatedAt:    base.Add(10 * time.Minute),
			UpdatedAt:    base.Add(10 * time.Minute),
		}},
		Reviews: []core.Review{{
			ID:             "rev_sqlite",
			ProjectID:      "proj_sqlite_snapshot",
			WorkItemID:     "work_sqlite",
			AssignmentID:   "asgn_sqlite",
			ReviewerRoleID: "role_sqlite",
			Title:          "SQLite review",
			Body:           "Snapshot import stores review metadata.",
			Verdict:        core.ReviewVerdictApproved,
			Risk:           core.ReviewRiskLow,
			Status:         core.ReviewStatusRecorded,
			CreatedAt:      base.Add(11 * time.Minute),
			UpdatedAt:      base.Add(11 * time.Minute),
		}},
		Handoffs: []core.Handoff{{
			ID:                    "handoff_sqlite",
			ProjectID:             "proj_sqlite_snapshot",
			WorkItemID:            "work_sqlite",
			SourceAssignmentID:    "asgn_sqlite",
			SourceRunID:           "run_sqlite",
			FromRoleID:            "role_sqlite",
			ToRoleID:              "role_sqlite",
			TargetAssignmentID:    "asgn_sqlite",
			TargetWorkItemID:      "work_sqlite",
			Title:                 "SQLite handoff",
			Body:                  "Use this after import.",
			RecommendedNextAction: "Continue from the snapshot.",
			LinkedArtifactIDs:     []string{"art_sqlite", "ev_sqlite", "rev_sqlite"},
			LinkedMemoryIDs:       []string{"mem_sqlite"},
			ContextRefs:           []string{"ctx_sqlite"},
			Status:                core.HandoffStatusAccepted,
			ProvenanceKind:        "operator",
			TrustLabel:            "operator_reviewed",
			CreatedAt:             base.Add(12 * time.Minute),
			UpdatedAt:             base.Add(13 * time.Minute),
			StatusChangedAt:       base.Add(12*time.Minute + 30*time.Second),
		}},
		MemoryEntries: []core.MemoryEntry{{
			ID:         "mem_sqlite",
			ProjectID:  "proj_sqlite_snapshot",
			Title:      "SQLite memory",
			Body:       "Snapshots include disabled memory entries.",
			TrustLabel: core.MemoryTrustOperator,
			SourceKind: core.MemorySourceOperator,
			SourceID:   "asgn_sqlite",
			Enabled:    false,
			CreatedAt:  base.Add(14 * time.Minute),
			UpdatedAt:  base.Add(15 * time.Minute),
		}},
		MemoryCandidates: []core.MemoryCandidate{{
			ID:                  "memcand_sqlite",
			ProjectID:           "proj_sqlite_snapshot",
			Title:               "SQLite candidate",
			Body:                "Snapshots include resolved candidates.",
			SuggestedKind:       "project_lesson",
			SuggestedTrustLabel: core.MemoryTrustGenerated,
			SuggestedSourceKind: "assignment",
			SuggestedSourceID:   "asgn_sqlite",
			SourceRefs:          []core.MemoryCandidateSourceRef{{Kind: "assignment", ID: "asgn_sqlite", Title: "SQLite assignment"}},
			Status:              core.MemoryCandidateRejected,
			StatusReason:        "not durable enough",
			CreatedAt:           base.Add(16 * time.Minute),
			UpdatedAt:           base.Add(17 * time.Minute),
		}},
		AssistantProposals: []core.AssistantProposalRecord{{
			ID:        "prop_sqlite",
			ProjectID: "proj_sqlite_snapshot",
			Source:    core.AssistantProposalSourceAssistant,
			Proposal: core.AssistantProposal{
				ID:                   "prop_sqlite",
				ProjectID:            "proj_sqlite_snapshot",
				Title:                "Would create work",
				Summary:              "Snapshot import should not replay this proposal.",
				Source:               core.AssistantProposalSourceAssistant,
				RequiresConfirmation: true,
				Actions: []core.AssistantAction{{
					Kind:     core.AssistantActionCreateWorkItem,
					WorkItem: &core.WorkItem{ID: "work_from_proposal", ProjectID: "proj_sqlite_snapshot", Title: "Should not exist"},
				}},
				CreatedAt: base.Add(18 * time.Minute),
			},
			Status:    core.AssistantProposalStatusProposed,
			CreatedAt: base.Add(18 * time.Minute),
			UpdatedAt: base.Add(18 * time.Minute),
		}},
	}
}

func sqliteSnapshotsEqualIgnoringExportedAt(a, b core.Snapshot) bool {
	a.ExportedAt = time.Time{}
	b.ExportedAt = time.Time{}
	return reflect.DeepEqual(a, b)
}

func boolPointerForSnapshotStoreTest(value bool) *bool {
	return &value
}
