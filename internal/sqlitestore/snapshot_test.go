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

func sqliteSnapshotFixture() core.Snapshot {
	base := time.Date(2026, 6, 28, 15, 0, 0, 0, time.UTC)
	return core.Snapshot{
		Version:    core.SnapshotVersion,
		ExportedAt: base.Add(30 * time.Minute),
		AgentProfiles: []core.AgentProfile{{
			ID:            "profile_sqlite",
			Name:          "SQLite profile",
			Description:   "Snapshot profile.",
			Instructions:  "Use durable context.",
			ContextPolicy: "include_enabled",
			MemoryPolicy:  "visible_only",
			SourcePolicy:  "include_enabled",
			SkillIDs:      []string{"sqlite-skill"},
			CreatedAt:     base,
			UpdatedAt:     base.Add(time.Minute),
		}},
		ExecutionProfiles: []core.ExecutionProfile{{
			ID:             "exec_sqlite",
			Name:           "SQLite execution",
			Description:    "Portable execution profile.",
			AgentKind:      "any",
			ModelHint:      "local",
			ProviderHint:   "local",
			ToolsPolicy:    "readonly",
			WritesPolicy:   "block",
			NetworkPolicy:  "block",
			ApprovalPolicy: "require",
			AdapterOptions: map[string]any{"driver": "mcp_pull"},
			CreatedAt:      base,
			UpdatedAt:      base.Add(time.Minute),
		}},
		Projects: []core.Project{{
			ID:                        "proj_sqlite_snapshot",
			Name:                      "SQLite snapshot",
			Description:               "Persistent snapshot fixture.",
			DefaultRootID:             "root_main",
			DefaultProfileID:          "profile_sqlite",
			DefaultExecutionProfileID: "exec_sqlite",
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
			ID:                        "role_sqlite",
			ProjectID:                 "proj_sqlite_snapshot",
			Name:                      "SQLite role",
			Description:               "Owns the migration check.",
			Instructions:              "Preserve coordination records.",
			DefaultProfileID:          "profile_sqlite",
			DefaultExecutionProfileID: "exec_sqlite",
			DefaultSkillIDs:           []string{"sqlite-skill"},
			DefaultExecutionMode:      core.ExecutionMCPPull,
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
			ID:                 "asgn_sqlite",
			ProjectID:          "proj_sqlite_snapshot",
			WorkItemID:         "work_sqlite",
			RoleID:             "role_sqlite",
			RootID:             "root_main",
			ProfileID:          "profile_sqlite",
			ExecutionProfileID: "exec_sqlite",
			ExecutionMode:      core.ExecutionMCPPull,
			Status:             core.AssignmentCompleted,
			DesiredAgent:       core.DesiredAgent{Kind: core.DesiredAgentAny, SkillIDs: []string{"sqlite-skill"}},
			ClaimedBy:          "agent:sqlite",
			ExecutionRef:       "run_sqlite",
			ContextSnapshotID:  "ctx_sqlite",
			CreatedAt:          base.Add(5 * time.Minute),
			UpdatedAt:          base.Add(8 * time.Minute),
			StartedAt:          base.Add(6 * time.Minute),
			CompletedAt:        base.Add(8 * time.Minute),
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
