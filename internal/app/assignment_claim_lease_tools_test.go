package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hecatehq/cairnline/internal/core"
)

type assignmentClaimToolResponse struct {
	Result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	} `json:"result"`
}

func callAssignmentClaimTool(t *testing.T, server interface {
	Serve(context.Context, io.Reader, io.Writer) error
}, name string, args map[string]any) assignmentClaimToolResponse {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	})
	if err != nil {
		t.Fatalf("Marshal(%s) error = %v", name, err)
	}
	var output bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(string(body)+"\n"), &output); err != nil {
		t.Fatalf("Serve(%s) error = %v", name, err)
	}
	var response assignmentClaimToolResponse
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("decode %s response: %v\n%s", name, err, output.String())
	}
	return response
}

func TestMCPTools_AssignmentClaimLeaseContract(t *testing.T) {
	ctx := context.Background()
	store := core.NewMemoryStore()
	service := core.NewService(store, core.WithAssignmentClaimLeaseTTL(1500*time.Millisecond))
	server := NewServer(service, "lease-test")
	project, err := service.CreateProject(ctx, core.Project{Name: "MCP claim leases"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Worker"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Lease through MCP"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	claimResponse := callAssignmentClaimTool(t, server, "assignments.claim", map[string]any{
		"project_id": project.ID, "assignment_id": assignment.ID, "claimed_by": "mcp-worker",
	})
	if claimResponse.Result.IsError {
		t.Fatalf("claim response = %+v, want success", claimResponse.Result)
	}
	var claimed core.Assignment
	if err := json.Unmarshal(claimResponse.Result.StructuredContent, &claimed); err != nil {
		t.Fatalf("decode claim structured content: %v", err)
	}
	if claimed.Claim == nil || claimed.Claim.ID == "" || claimed.Claim.ExpiresAt.Sub(claimed.Claim.AcquiredAt) != 2*time.Second {
		t.Fatalf("claimed assignment = %+v, want rounded two-second lease", claimed)
	}
	if len(claimResponse.Result.Content) == 0 || !strings.Contains(claimResponse.Result.Content[0].Text, claimed.Claim.ID) {
		t.Fatalf("claim fallback content = %+v, want claim id %q", claimResponse.Result.Content, claimed.Claim.ID)
	}
	expiryText := claimed.Claim.ExpiresAt.Format(time.RFC3339Nano)
	for _, read := range []struct {
		name string
		args map[string]any
	}{
		{name: "assignments.get", args: map[string]any{"project_id": project.ID, "assignment_id": assignment.ID}},
		{name: "assignments.list", args: map[string]any{"project_id": project.ID}},
	} {
		response := callAssignmentClaimTool(t, server, read.name, read.args)
		if response.Result.IsError || len(response.Result.Content) == 0 || !strings.Contains(response.Result.Content[0].Text, "claim_id="+claimed.Claim.ID) || !strings.Contains(response.Result.Content[0].Text, "claim_expires_at="+expiryText) {
			t.Fatalf("%s fallback content = %+v, want recoverable claim id and expiry", read.name, response.Result)
		}
	}

	renewResponse := callAssignmentClaimTool(t, server, "assignments.renew_claim", map[string]any{
		"project_id": project.ID, "assignment_id": assignment.ID, "claim_id": claimed.Claim.ID,
	})
	if renewResponse.Result.IsError {
		t.Fatalf("renew response = %+v, want success", renewResponse.Result)
	}
	var renewed core.Assignment
	if err := json.Unmarshal(renewResponse.Result.StructuredContent, &renewed); err != nil {
		t.Fatalf("decode renew structured content: %v", err)
	}
	if !renewed.UpdatedAt.Equal(claimed.UpdatedAt) || renewed.Claim == nil || renewed.Claim.ExpiresAt.Before(claimed.Claim.ExpiresAt) {
		t.Fatalf("renewed assignment = %+v, want unchanged activity revision and extended claim", renewed)
	}

	wrongResponse := callAssignmentClaimTool(t, server, "assignments.prepare", map[string]any{
		"project_id": project.ID, "assignment_id": assignment.ID, "claim_id": "claim_stale", "context_snapshot_id": "ctx-stale",
	})
	if !wrongResponse.Result.IsError || !strings.Contains(string(wrongResponse.Result.StructuredContent), `"code":"conflict"`) {
		t.Fatalf("stale prepare response = %+v, want typed conflict", wrongResponse.Result)
	}

	startResponse := callAssignmentClaimTool(t, server, "assignments.update_status", map[string]any{
		"project_id": project.ID, "assignment_id": assignment.ID, "claim_id": claimed.Claim.ID, "status": "running",
	})
	if startResponse.Result.IsError {
		t.Fatalf("start response = %+v, want success", startResponse.Result)
	}
	var running core.Assignment
	if err := json.Unmarshal(startResponse.Result.StructuredContent, &running); err != nil {
		t.Fatalf("decode start structured content: %v", err)
	}
	if running.Claim == nil || running.Claim.ID != claimed.Claim.ID || !running.Claim.ExpiresAt.IsZero() {
		t.Fatalf("running assignment = %+v, want fence retained and reservation expiry retired", running)
	}
	runningRead := callAssignmentClaimTool(t, server, "assignments.get", map[string]any{
		"project_id": project.ID, "assignment_id": assignment.ID,
	})
	if runningRead.Result.IsError || len(runningRead.Result.Content) == 0 || !strings.Contains(runningRead.Result.Content[0].Text, "claim_fence="+claimed.Claim.ID) || strings.Contains(runningRead.Result.Content[0].Text, "claim_expires_at=") {
		t.Fatalf("running assignment fallback content = %+v, want retained fence without lease expiry", runningRead.Result)
	}

	capabilities := callAssignmentClaimTool(t, server, "coordination.capabilities", map[string]any{})
	if capabilities.Result.IsError || !strings.Contains(string(capabilities.Result.StructuredContent), `"ttl_seconds":2`) || !strings.Contains(string(capabilities.Result.StructuredContent), `"recovery_tool":"assignments.recover_claim"`) {
		t.Fatalf("capabilities response = %+v, want effective lease contract", capabilities.Result)
	}

	recoverable, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment(recoverable) error = %v", err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	recoverable.CreatedAt = past
	recoverable.UpdatedAt = past
	if _, err := store.RestoreAssignmentSnapshot(ctx, recoverable); err != nil {
		t.Fatalf("RestoreAssignmentSnapshot(expired fixture) error = %v", err)
	}
	if _, err := store.ClaimAssignmentWithLease(ctx, project.ID, recoverable.ID, "crashed-worker", core.AssignmentClaimLease{ID: "claim_expired"}, time.Second, func() time.Time { return past.Add(time.Second) }); err != nil {
		t.Fatalf("ClaimAssignmentWithLease(expired fixture) error = %v", err)
	}
	recoverResponse := callAssignmentClaimTool(t, server, "assignments.recover_claim", map[string]any{
		"project_id": project.ID, "assignment_id": recoverable.ID, "expected_claim_id": "claim_expired",
	})
	if recoverResponse.Result.IsError {
		t.Fatalf("recover response = %+v, want success", recoverResponse.Result)
	}
	var recovered core.Assignment
	if err := json.Unmarshal(recoverResponse.Result.StructuredContent, &recovered); err != nil {
		t.Fatalf("decode recovery structured content: %v", err)
	}
	if recovered.Status != core.AssignmentQueued || recovered.Claim != nil || recovered.ClaimedBy != "" {
		t.Fatalf("recovered assignment = %+v, want queued without claim", recovered)
	}
	completeWithoutFence := callAssignmentClaimTool(t, server, "assignments.complete", map[string]any{
		"project_id": project.ID, "assignment_id": recoverable.ID, "status": "completed",
	})
	if !completeWithoutFence.Result.IsError || !strings.Contains(string(completeWithoutFence.Result.StructuredContent), `"code":"invalid"`) {
		t.Fatalf("complete without fence response = %+v, want typed invalid error", completeWithoutFence.Result)
	}
	stillQueued, err := service.GetAssignment(ctx, project.ID, recoverable.ID)
	if err != nil {
		t.Fatalf("GetAssignment(after unfenced completion) error = %v", err)
	}
	if stillQueued.Status != core.AssignmentQueued || stillQueued.Claim != nil {
		t.Fatalf("assignment after unfenced completion = %+v, want unchanged queued state", stillQueued)
	}
}
