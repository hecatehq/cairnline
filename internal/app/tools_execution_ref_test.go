package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
)

func TestMCPTools_StructuredExecutionRefAndAwaitingApproval(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")

	project, err := service.CreateProject(ctx, core.Project{Name: "Approval project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Gated change"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	claimed, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "agent-a")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	claimID := claimed.Claim.ID
	if _, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{ProjectID: project.ID, Title: "Timeout invariant", Body: "Never lower the gateway timeout."}); err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"assignments.update_status","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignment.ID + `","claim_id":"` + claimID + `","status":"awaiting_approval","execution_ref":{"kind":"task_run","task_id":"task-1","run_id":"run-1","session_id":"sess-1","trace_id":"trace-1","pending_approvals":2}}}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() update_status error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated assignment "+assignment.ID+": awaiting_approval") {
		t.Fatalf("update_status response = %s", output.String())
	}
	got, err := service.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	want := core.ExecutionRef{Kind: "task_run", TaskID: "task-1", RunID: "run-1", SessionID: "sess-1", TraceID: "trace-1", PendingApprovals: 2}
	if got.Status != core.AssignmentAwaitingApproval || got.ExecutionRef != want {
		t.Fatalf("assignment after tool call = %+v, want awaiting_approval with structured ref", got)
	}

	// A bare-string execution_ref is the pre-structured contract; it fails
	// argument decode as a tool error and leaves the assignment untouched.
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"assignments.update_status","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignment.ID + `","claim_id":"` + claimID + `","status":"running","execution_ref":"legacy-run-9"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() bare-string update_status error = %v", err)
	}
	if !strings.Contains(output.String(), "invalid arguments") {
		t.Fatalf("bare-string execution_ref response = %s, want invalid-arguments tool error", output.String())
	}
	got, err = service.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() after rejected ref error = %v", err)
	}
	if got.Status != core.AssignmentAwaitingApproval || got.ExecutionRef != want {
		t.Fatalf("assignment after rejected ref = %+v, want prior awaiting_approval state preserved", got)
	}

	// Resume with a structured ref so the context read below sees running
	// state driven by the supported shape.
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, core.AssignmentRunning, core.ExecutionRef{RunID: "run-9"}); err != nil {
		t.Fatalf("UpdateAssignmentStatus(resume) error = %v", err)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"assignments.context","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignment.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() context error = %v", err)
	}
	if !strings.Contains(output.String(), "Memory: Timeout invariant") {
		t.Fatalf("assignment context response = %s, want memory entry title in text summary", output.String())
	}
	var contextResponse struct {
		Result struct {
			StructuredContent core.AssignmentContext `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &contextResponse); err != nil {
		t.Fatalf("decode assignment context response: %v\n%s", err, output.String())
	}
	packet := contextResponse.Result.StructuredContent
	if len(packet.Memory) != 1 || packet.Memory[0].Title != "Timeout invariant" || packet.Memory[0].Body != "Never lower the gateway timeout." {
		t.Fatalf("structured context memory = %+v, want the enabled memory entry with body", packet.Memory)
	}
	if packet.Assignment.ExecutionRef != (core.ExecutionRef{RunID: "run-9"}) {
		t.Fatalf("structured context assignment ref = %+v, want structured execution ref", packet.Assignment.ExecutionRef)
	}
}
