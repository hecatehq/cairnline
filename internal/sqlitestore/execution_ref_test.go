package sqlitestore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
)

func seedExecutionRefTestAssignment(t *testing.T, ctx context.Context, service *core.Service) (core.Project, core.Assignment) {
	t.Helper()
	project, err := service.CreateProject(ctx, core.Project{Name: "Execution ref project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Portable execution"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	return project, assignment
}

func TestStore_AssignmentExecutionRefRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cairnline.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	project, assignment := seedExecutionRefTestAssignment(t, ctx, service)

	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	ref := core.ExecutionRef{
		Kind:             "task_run",
		TaskID:           "task_sqlite",
		RunID:            "run_sqlite",
		SessionID:        "sess_sqlite",
		TraceID:          "trace_sqlite",
		PendingApprovals: 2,
	}
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, core.AssignmentAwaitingApproval, ref); err != nil {
		t.Fatalf("UpdateAssignmentStatus(awaiting_approval) error = %v", err)
	}

	// Reopen so the readback exercises persisted state, not process memory.
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	got, err := reopened.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if got.Status != core.AssignmentAwaitingApproval {
		t.Fatalf("status = %q, want awaiting_approval to persist without clamping", got.Status)
	}
	if got.ExecutionRef != ref {
		t.Fatalf("execution ref = %+v, want full structured ref %+v", got.ExecutionRef, ref)
	}
}

func TestStore_AssignmentLegacyExecutionRefDecodes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cairnline.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	project, assignment := seedExecutionRefTestAssignment(t, ctx, service)

	// Rows written before the structured ref stored one opaque host string.
	if _, err := store.db.ExecContext(ctx, `UPDATE assignments SET execution_ref = ? WHERE project_id = ? AND id = ?`, "legacy-run-7", project.ID, assignment.ID); err != nil {
		t.Fatalf("seed legacy execution_ref: %v", err)
	}
	got, err := store.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if got.ExecutionRef != (core.ExecutionRef{RunID: "legacy-run-7"}) {
		t.Fatalf("legacy execution ref = %+v, want run id legacy-run-7", got.ExecutionRef)
	}
}
