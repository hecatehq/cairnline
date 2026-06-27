package cairnline_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

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
		Actions: []cairnline.AssistantAction{{
			Kind: cairnline.AssistantActionCreateWorkItem,
			WorkItem: &cairnline.WorkItem{
				ID:        "work_public",
				ProjectID: project.ID,
				Title:     "Public API work",
			},
		}},
	})
	if err != nil {
		t.Fatalf("CreateAssistantProposal() error = %v", err)
	}
	var typedRecord cairnline.AssistantProposalRecord = record
	if typedRecord.Status != cairnline.AssistantProposalStatusProposed {
		t.Fatalf("proposal status = %q, want public proposed constant", typedRecord.Status)
	}
	result, err := service.ApplyAssistantProposalRecord(ctx, typedRecord.ID, true)
	if err != nil {
		t.Fatalf("ApplyAssistantProposalRecord() error = %v", err)
	}
	var typedResult cairnline.AssistantApplyResult = result
	if typedResult.Status != cairnline.AssistantApplyStatusApplied || !typedResult.Applied {
		t.Fatalf("apply result = %+v, want public applied status", typedResult)
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
}
