package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestExecutionRef_JSONRoundTrip(t *testing.T) {
	ref := ExecutionRef{
		Kind:             "task_run",
		TaskID:           "task_1",
		RunID:            "run_1",
		SessionID:        "sess_1",
		TraceID:          "trace_1",
		PendingApprovals: 3,
	}
	raw, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded ExecutionRef
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded != ref {
		t.Fatalf("round trip = %+v, want %+v", decoded, ref)
	}
}

func TestExecutionRef_RejectsBareStringWire(t *testing.T) {
	// The pre-structured contract stored one opaque string. It is not
	// silently reinterpreted: decode fails and the ref stays zero, so a
	// caller sees an error instead of a fabricated ref.
	var decoded ExecutionRef
	if err := json.Unmarshal([]byte(`"run-legacy-1"`), &decoded); err == nil {
		t.Fatalf("Unmarshal(bare string) error = nil, want decode failure")
	}
	if !decoded.Empty() {
		t.Fatalf("decoded ref after failure = %+v, want zero ref", decoded)
	}
}

func TestExecutionRef_OmittedWhenEmpty(t *testing.T) {
	raw, err := json.Marshal(Assignment{ID: "asgn_1"})
	if err != nil {
		t.Fatalf("Marshal(assignment) error = %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("Unmarshal(wire) error = %v", err)
	}
	if _, ok := wire["execution_ref"]; ok {
		t.Fatalf("assignment wire = %s, want execution_ref omitted when empty", raw)
	}
}

func newExecutionRefTestAssignment(t *testing.T, ctx context.Context, service *Service) (Project, WorkItem, Role, Assignment) {
	t.Helper()
	project, err := service.CreateProject(ctx, Project{Name: "Execution ref project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Portable execution"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	return project, work, role, assignment
}

func TestService_UpdateAssignmentStatusAwaitingApproval(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, _, _, assignment := newExecutionRefTestAssignment(t, ctx, service)

	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	ref := ExecutionRef{Kind: "task_run", TaskID: "task_1", RunID: "run_1", TraceID: "trace_1", PendingApprovals: 2}
	blocked, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, AssignmentAwaitingApproval, ref)
	if err != nil {
		t.Fatalf("UpdateAssignmentStatus(awaiting_approval) error = %v", err)
	}
	if blocked.Status != AssignmentAwaitingApproval || blocked.ExecutionRef != ref {
		t.Fatalf("blocked assignment = %+v, want awaiting_approval with full ref", blocked)
	}
	got, err := service.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if got.Status != AssignmentAwaitingApproval || got.ExecutionRef != ref {
		t.Fatalf("readback = %+v, want persisted awaiting_approval and ref", got)
	}

	// Empty ref must not clobber the stored one when the approval resolves.
	resumed, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, AssignmentRunning, ExecutionRef{})
	if err != nil {
		t.Fatalf("UpdateAssignmentStatus(running) error = %v", err)
	}
	if resumed.Status != AssignmentRunning || resumed.ExecutionRef != ref {
		t.Fatalf("resumed assignment = %+v, want running with preserved ref", resumed)
	}

	if _, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, AssignmentAwaitingApproval, ExecutionRef{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CompleteAssignment(awaiting_approval) error = %v, want ErrInvalid", err)
	}
}

func TestService_AssignmentContextIncludesEnabledMemory(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, _, _, assignment := newExecutionRefTestAssignment(t, ctx, service)

	enabled, err := service.CreateMemoryEntry(ctx, MemoryEntry{ProjectID: project.ID, Title: "Timeout invariant", Body: "Never lower the gateway timeout."})
	if err != nil {
		t.Fatalf("CreateMemoryEntry(enabled) error = %v", err)
	}
	disabled, err := service.CreateMemoryEntry(ctx, MemoryEntry{ProjectID: project.ID, Title: "Stale note", Body: "Superseded guidance."})
	if err != nil {
		t.Fatalf("CreateMemoryEntry(disabled) error = %v", err)
	}
	disabled.Enabled = false
	if _, err := service.UpdateMemoryEntry(ctx, disabled); err != nil {
		t.Fatalf("UpdateMemoryEntry(disable) error = %v", err)
	}

	packet, err := service.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() error = %v", err)
	}
	if len(packet.Memory) != 1 || packet.Memory[0].ID != enabled.ID || packet.Memory[0].Body != "Never lower the gateway timeout." {
		t.Fatalf("context memory = %+v, want only the enabled entry with body", packet.Memory)
	}
}

func TestService_ProjectActivityCountsAwaitingApproval(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	project, _, _, assignment := newExecutionRefTestAssignment(t, ctx, service)

	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if _, err := service.UpdateAssignmentStatus(ctx, project.ID, assignment.ID, AssignmentAwaitingApproval, ExecutionRef{RunID: "run_1", PendingApprovals: 1}); err != nil {
		t.Fatalf("UpdateAssignmentStatus(awaiting_approval) error = %v", err)
	}
	activity, err := service.ProjectActivity(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectActivity() error = %v", err)
	}
	if activity.Counts.AwaitingApproval != 1 || activity.Counts.Blocked != 1 || activity.Counts.Active != 0 {
		t.Fatalf("activity counts = %+v, want one blocked awaiting_approval assignment", activity.Counts)
	}
	if len(activity.Buckets.Blocked) != 1 || activity.Buckets.Blocked[0].Status != AssignmentAwaitingApproval {
		t.Fatalf("blocked bucket = %+v, want awaiting_approval item", activity.Buckets.Blocked)
	}
	if activity.Buckets.Blocked[0].ExecutionRef.RunID != "run_1" || activity.Buckets.Blocked[0].ExecutionRef.PendingApprovals != 1 {
		t.Fatalf("blocked item ref = %+v, want structured execution ref", activity.Buckets.Blocked[0].ExecutionRef)
	}
}
