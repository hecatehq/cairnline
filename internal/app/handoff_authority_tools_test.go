package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

func TestHandoffAuthorityTools_DescriptorsRequireCASAndDescribeAtomicRetry(t *testing.T) {
	server := NewServer(core.NewService(core.NewMemoryStore()), "test")
	var output bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &output); err != nil {
		t.Fatalf("Serve(tools/list) error = %v", err)
	}
	var envelope struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("Unmarshal(tools/list) error = %v", err)
	}
	tools := make(map[string]mcp.Tool, len(envelope.Result.Tools))
	for _, tool := range envelope.Result.Tools {
		tools[tool.Name] = tool
	}
	for _, name := range []string{"handoffs.update", "handoffs.update_status", "handoffs.delete"} {
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(tools[name].InputSchema, &schema); err != nil {
			t.Fatalf("Unmarshal(%s schema) error = %v", name, err)
		}
		requiresToken := false
		for _, field := range schema.Required {
			if field == "expected_updated_at" {
				requiresToken = true
				break
			}
		}
		if !requiresToken {
			t.Fatalf("%s required fields = %v, want expected_updated_at", name, schema.Required)
		}
	}
	atomicTool := tools["handoffs.accept_with_follow_up"]
	if atomicTool.Annotations == nil || atomicTool.Annotations.IdempotentHint == nil || !*atomicTool.Annotations.IdempotentHint || atomicTool.Annotations.DestructiveHint == nil || !*atomicTool.Annotations.DestructiveHint {
		t.Fatalf("accept_with_follow_up annotations = %+v, want idempotent and destructive confirmation hints", atomicTool.Annotations)
	}
	if schema := string(atomicTool.InputSchema); !strings.Contains(schema, core.HandoffFollowUpIntentAcceptAndEnsure) || !strings.Contains(schema, `"idempotency_key"`) {
		t.Fatalf("accept_with_follow_up schema = %s, want explicit intent and idempotency key", schema)
	}
}

func TestHandoffAuthorityTools_ConflictAndIdempotentAtomicFollowUp(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "test")
	project, err := service.CreateProject(ctx, core.Project{Name: "Atomic handoff"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer", DefaultExecutionMode: core.ExecutionMCPPull, DefaultSkillIDs: []string{"review"}})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Review"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, core.Handoff{ProjectID: project.ID, WorkItemID: work.ID, ToRoleID: role.ID, Title: "Review next", Body: "Continue from the evidence."})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}

	var output bytes.Buffer
	missingToken := toolRequest(t, 1, "handoffs.update_status", map[string]any{
		"project_id": project.ID, "work_item_id": work.ID, "handoff_id": handoff.ID, "status": core.HandoffStatusAccepted,
	})
	if err := server.Serve(ctx, missingToken, &output); err != nil {
		t.Fatalf("Serve(missing token) error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, `"code":"invalid"`) || !strings.Contains(got, "expected_updated_at is required") {
		t.Fatalf("missing-token response = %s", got)
	}

	originalToken := handoff.UpdatedAt.Format(time.RFC3339Nano)
	output.Reset()
	patch := toolRequest(t, 2, "handoffs.update", map[string]any{
		"project_id": project.ID, "work_item_id": work.ID, "handoff_id": handoff.ID,
		"expected_updated_at": originalToken, "title": "Review the evidence",
	})
	if err := server.Serve(ctx, patch, &output); err != nil {
		t.Fatalf("Serve(patch) error = %v", err)
	}
	current, err := service.GetHandoff(ctx, project.ID, work.ID, handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff() error = %v", err)
	}

	output.Reset()
	stale := toolRequest(t, 3, "handoffs.update_status", map[string]any{
		"project_id": project.ID, "work_item_id": work.ID, "handoff_id": handoff.ID,
		"expected_updated_at": originalToken, "status": core.HandoffStatusAccepted,
	})
	if err := server.Serve(ctx, stale, &output); err != nil {
		t.Fatalf("Serve(stale status) error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, `"code":"conflict"`) {
		t.Fatalf("stale response = %s, want structured conflict", got)
	}

	arguments := map[string]any{
		"project_id": project.ID, "work_item_id": work.ID, "handoff_id": handoff.ID,
		"expected_updated_at": current.UpdatedAt.Format(time.RFC3339Nano),
		"idempotency_key":     "operator-action-1", "intent": core.HandoffFollowUpIntentAcceptAndEnsure,
	}
	output.Reset()
	if err := server.Serve(ctx, toolRequest(t, 4, "handoffs.accept_with_follow_up", arguments), &output); err != nil {
		t.Fatalf("Serve(atomic follow-up) error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, `"outcome":"created"`) || !strings.Contains(got, `"status":"accepted"`) || !strings.Contains(got, `"structuredContent"`) {
		t.Fatalf("atomic response = %s", got)
	}
	assignments, err := service.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].Status != core.AssignmentQueued || !assignments[0].ExecutionRef.Empty() || assignments[0].ClaimedBy != "" {
		t.Fatalf("assignments = %+v, want one pristine queued follow-up", assignments)
	}
	assignmentID := assignments[0].ID

	output.Reset()
	if err := server.Serve(ctx, toolRequest(t, 5, "handoffs.accept_with_follow_up", arguments), &output); err != nil {
		t.Fatalf("Serve(replay) error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, `"replayed":true`) || !strings.Contains(got, assignmentID) {
		t.Fatalf("replay response = %s, want same assignment and replay marker", got)
	}
	assignments, err = service.ListAssignments(ctx, project.ID)
	if err != nil || len(assignments) != 1 || assignments[0].ID != assignmentID {
		t.Fatalf("assignments after replay = %+v, %v; want one original assignment", assignments, err)
	}
}
