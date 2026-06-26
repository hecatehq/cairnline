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

	project, err := service.CreateProject(ctx, cairnline.Project{Name: "Embedded project"})
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
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, cairnline.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
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

	claimed, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "hecate")
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if claimed.Status != cairnline.AssignmentClaimed {
		t.Fatalf("claimed assignment = %+v, want claimed status", claimed)
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
