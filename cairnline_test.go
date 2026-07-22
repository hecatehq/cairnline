package cairnline_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/hecatehq/cairnline"
)

func TestPublicAPIEmbedsCoordinationCore(t *testing.T) {
	ctx := context.Background()
	service := cairnline.NewMemoryService()

	project, err := service.CreateProject(ctx, cairnline.Project{
		Name: "Embedded project",
		Roots: []cairnline.Root{{
			ID:     "root_main",
			Path:   "/workspace/example",
			Kind:   "local",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, cairnline.Role{
		ProjectID:            project.ID,
		Name:                 "Implementer",
		DefaultExecutionMode: cairnline.ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, cairnline.WorkItem{
		ProjectID: project.ID,
		Title:     "Wire embedded API",
		RootID:    "root_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, cairnline.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
		RootID:     "root_main",
		DesiredAgent: cairnline.DesiredAgent{
			Kind: cairnline.DesiredAgentAny,
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if assignment.Status != cairnline.AssignmentQueued || assignment.ExecutionMode != cairnline.ExecutionMCPPull {
		t.Fatalf("assignment = %+v, want queued mcp_pull assignment", assignment)
	}
	if assignment.RootID != "root_main" {
		t.Fatalf("assignment root = %q, want root_main", assignment.RootID)
	}

	claimed, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "hecate")
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if claimed.Status != cairnline.AssignmentClaimed {
		t.Fatalf("claimed assignment = %+v, want claimed status", claimed)
	}

	brief, err := service.ProjectOperationsBrief(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectOperationsBrief() error = %v", err)
	}
	var typedBrief cairnline.ProjectOperationsBrief = brief
	if typedBrief.Status != cairnline.ProjectOperationsStatusAttention || typedBrief.Next == nil || typedBrief.Next.Kind != cairnline.ProjectOperationKindAssignment || typedBrief.Next.Severity != cairnline.ProjectOperationSeverityActive {
		t.Fatalf("operations brief = %+v, want public active assignment attention item", typedBrief)
	}

	activity, err := service.ProjectActivity(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectActivity() error = %v", err)
	}
	var typedActivity cairnline.ProjectActivity = activity
	if typedActivity.Counts.Active != 1 || len(typedActivity.Buckets.Active) != 1 || typedActivity.Buckets.Active[0].Bucket != cairnline.ProjectActivityBucketActive {
		t.Fatalf("activity = %+v, want public active assignment bucket", typedActivity)
	}
}

func TestPublicAPIExposesFencedAssignmentClaims(t *testing.T) {
	if cairnline.ProjectOperationActionRecoverClaim != "recover_assignment_claim" {
		t.Fatalf("ProjectOperationActionRecoverClaim = %q, want stable public action", cairnline.ProjectOperationActionRecoverClaim)
	}
	ctx := context.Background()
	service := cairnline.NewService(cairnline.NewMemoryStore(), cairnline.WithAssignmentClaimLeaseTTL(90*time.Second))
	project, err := service.CreateProject(ctx, cairnline.Project{Name: "Portable claim"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, cairnline.Role{ProjectID: project.ID, Name: "Worker"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, cairnline.WorkItem{ProjectID: project.ID, Title: "Fence worker"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, cairnline.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	claimed, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "portable-worker")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	var claim *cairnline.AssignmentClaimLease = claimed.Claim
	if claim == nil || claim.ID == "" || claim.ExpiresAt.Sub(claim.AcquiredAt) != 90*time.Second {
		t.Fatalf("claim = %+v, want public 90-second fence", claim)
	}
	running, err := service.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, cairnline.AssignmentRunning, cairnline.ExecutionRef{}, claim.ID)
	if err != nil {
		t.Fatalf("UpdateAssignmentStatusWithClaim() error = %v", err)
	}
	if running.Claim == nil || running.Claim.ID != claim.ID || !running.Claim.ExpiresAt.IsZero() {
		t.Fatalf("running claim = %+v, want retained public fence", running.Claim)
	}
}

func TestPublicAPIOpensSQLiteStore(t *testing.T) {
	ctx := context.Background()
	service, store, err := cairnline.NewSQLiteService(ctx, filepath.Join(t.TempDir(), "cairnline.db"))
	if err != nil {
		t.Fatalf("NewSQLiteService() error = %v", err)
	}
	defer store.Close()

	project, err := service.CreateProject(ctx, cairnline.Project{Name: "SQLite project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	projects, err := service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("projects = %+v, want persisted project", projects)
	}

	_, err = service.CreateWorkItem(ctx, cairnline.WorkItem{
		ProjectID: "proj_missing",
		Title:     "Missing project",
	})
	if !errors.Is(err, cairnline.ErrNotFound) {
		t.Fatalf("CreateWorkItem() error = %v, want ErrNotFound", err)
	}
}

func TestPublicAPIExposesSnapshotMigrationContract(t *testing.T) {
	ctx := context.Background()
	source := cairnline.NewMemoryService()
	project, err := source.CreateProject(ctx, cairnline.Project{Name: "Snapshot public API"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := source.CreateWorkItem(ctx, cairnline.WorkItem{ProjectID: project.ID, Title: "Snapshot work"}); err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	snapshot, err := source.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	var typedSnapshot cairnline.Snapshot = snapshot
	if typedSnapshot.Version != cairnline.SnapshotVersion || len(typedSnapshot.Projects) != 1 || len(typedSnapshot.WorkItems) != 1 {
		t.Fatalf("snapshot = %+v, want public snapshot with project and work item", typedSnapshot)
	}

	target := cairnline.NewMemoryService()
	imported, err := target.ImportSnapshot(ctx, typedSnapshot)
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if imported.Version != cairnline.SnapshotVersion || len(imported.Projects) != 1 || imported.Projects[0].ID != project.ID {
		t.Fatalf("imported snapshot = %+v, want public snapshot import", imported)
	}
}

func TestPublicAPIExposesAssistantProposalLedger(t *testing.T) {
	ctx := context.Background()
	service := cairnline.NewMemoryService()
	project, err := service.CreateProject(ctx, cairnline.Project{Name: "Assistant project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	record, err := service.CreateAssistantProposal(ctx, cairnline.AssistantProposal{
		ID:        "prop_public",
		ProjectID: project.ID,
		Title:     "Create public work",
		Warnings:  []string{"operator should confirm scope"},
		Actions: []cairnline.AssistantAction{
			{
				Kind:   cairnline.AssistantActionAttachProjectRoot,
				Target: cairnline.AssistantTarget{ProjectID: project.ID},
				Root:   &cairnline.Root{ID: "root_public", Path: "/workspace/public", Active: true},
			},
			{
				Kind:    cairnline.AssistantActionSetProjectDefaults,
				Project: &cairnline.Project{ID: project.ID, DefaultRootID: "root_public"},
			},
			{
				Kind: cairnline.AssistantActionCreateWorkItem,
				WorkItem: &cairnline.WorkItem{
					ID:        "work_public",
					ProjectID: project.ID,
					Title:     "Public API work",
					RootID:    "root_public",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssistantProposal() error = %v", err)
	}
	var typedRecord cairnline.AssistantProposalRecord = record
	if typedRecord.Status != cairnline.AssistantProposalStatusProposed {
		t.Fatalf("proposal status = %q, want public proposed constant", typedRecord.Status)
	}
	if len(typedRecord.Proposal.Warnings) != 1 || typedRecord.Proposal.Warnings[0] != "operator should confirm scope" {
		t.Fatalf("proposal warnings = %+v, want public warnings preserved", typedRecord.Proposal.Warnings)
	}
	result, err := service.ApplyAssistantProposalRecord(ctx, typedRecord.ID, true)
	if err != nil {
		t.Fatalf("ApplyAssistantProposalRecord() error = %v", err)
	}
	var typedResult cairnline.AssistantApplyResult = result
	if typedResult.Status != cairnline.AssistantApplyStatusApplied || !typedResult.Applied {
		t.Fatalf("apply result = %+v, want public applied status", typedResult)
	}
	if typedResult.AppliedActionCount != 3 || len(typedResult.Actions) != 3 || typedResult.Actions[0].RootID != "root_public" || typedResult.Actions[1].RootID != "root_public" {
		t.Fatalf("apply action refs = %+v, want public root action refs", typedResult.Actions)
	}
	applied, err := service.GetAssistantProposal(ctx, typedRecord.ID)
	if err != nil {
		t.Fatalf("GetAssistantProposal() error = %v", err)
	}
	if len(applied.ApplyAttempts) != 1 {
		t.Fatalf("apply attempts = %+v, want one public attempt", applied.ApplyAttempts)
	}
	var typedAttempt cairnline.AssistantApplyAttempt = applied.ApplyAttempts[0]
	if typedAttempt.ProposalID != typedRecord.ID || typedAttempt.Status != cairnline.AssistantApplyStatusApplied {
		t.Fatalf("typed attempt = %+v, want public attempt alias", typedAttempt)
	}

	imported, err := service.ImportAssistantProposalRecord(ctx, cairnline.AssistantProposalRecord{
		ID:        "prop_import_public",
		ProjectID: project.ID,
		Source:    cairnline.AssistantProposalSourceAssistant,
		Proposal: cairnline.AssistantProposal{
			ID:        "prop_import_public",
			ProjectID: project.ID,
			Title:     "Imported public proposal",
			Warnings:  []string{"imported warning"},
			Actions: []cairnline.AssistantAction{{
				Kind:     cairnline.AssistantActionCreateWorkItem,
				WorkItem: &cairnline.WorkItem{ID: "work_import_public", ProjectID: project.ID, Title: "Imported work"},
			}},
		},
		Status: cairnline.AssistantProposalStatusRejected,
		LatestResult: &cairnline.AssistantApplyResult{
			ProposalID:       "prop_import_public",
			Status:           cairnline.AssistantApplyStatusRejected,
			TotalActionCount: 1,
		},
	})
	if err != nil {
		t.Fatalf("ImportAssistantProposalRecord() error = %v", err)
	}
	if imported.Status != cairnline.AssistantProposalStatusRejected || imported.LatestResult == nil {
		t.Fatalf("imported public proposal = %+v, want rejected imported ledger state", imported)
	}
	if len(imported.Proposal.Warnings) != 1 || imported.Proposal.Warnings[0] != "imported warning" {
		t.Fatalf("imported warnings = %+v, want public imported warnings preserved", imported.Proposal.Warnings)
	}
}
