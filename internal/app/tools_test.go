package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
)

func TestMCPTools_CreateProjectAndWorkItem(t *testing.T) {
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects.create","arguments":{"name":"Research notes","description":"Coordinate synthesis."}}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created project proj_") {
		t.Fatalf("create project response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"projects.list","arguments":{}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Research notes") {
		t.Fatalf("list projects response = %s", output.String())
	}

	projects, err := service.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %+v, want one project", projects)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"projects.update","arguments":{"id":"` + projects[0].ID + `","description":"Updated synthesis coordination."}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated project "+projects[0].ID) {
		t.Fatalf("update project response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"work_items.create","arguments":{"project_id":"` + projects[0].ID + `","title":"Summarize interviews","brief":"Produce themes."}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created work item work_") {
		t.Fatalf("create work item response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"work_items.list","arguments":{"project_id":"` + projects[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Summarize interviews") {
		t.Fatalf("list work items response = %s", output.String())
	}

	workItems, err := service.ListWorkItems(context.Background(), projects[0].ID)
	if err != nil {
		t.Fatalf("ListWorkItems() error = %v", err)
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"work_items.update","arguments":{"project_id":"` + projects[0].ID + `","id":"` + workItems[0].ID + `","brief":"Updated themes."}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated work item "+workItems[0].ID) {
		t.Fatalf("update work item response = %s", output.String())
	}
	workItems, err = service.ListWorkItems(context.Background(), projects[0].ID)
	if err != nil {
		t.Fatalf("ListWorkItems() after update error = %v", err)
	}
	if workItems[0].Title != "Summarize interviews" || workItems[0].Brief != "Updated themes." {
		t.Fatalf("updated work item = %+v, want patch preserving title and replacing brief", workItems[0])
	}
}

func TestMCPTools_AssignmentPullLifecycle(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"profiles.create","arguments":{"id":"profile_reviewer","name":"Reviewer profile","instructions":"Review with evidence.","skill_ids":["review"]}}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created agent profile profile_reviewer") {
		t.Fatalf("create profile response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"execution_profiles.create","arguments":{"id":"exec_local","name":"Local reviewer","agent_kind":"any","provider_hint":"local","model_hint":"local-small","tools_policy":"readonly","writes_policy":"block","network_policy":"block","approval_policy":"require","adapter_options":{"mode":"test"}}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created execution profile exec_local") {
		t.Fatalf("create execution profile response = %s", output.String())
	}

	project, err := service.CreateProject(ctx, core.Project{
		Name: "Dogfood",
		Roots: []core.Root{{
			ID:     "root_main",
			Path:   "/workspace/dogfood",
			Kind:   "local",
			Active: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:        project.ID,
		Name:             "Reviewer",
		Instructions:     "Review evidence.",
		DefaultProfileID: "profile_reviewer",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"roles.update","arguments":{"project_id":"` + project.ID + `","id":"` + role.ID + `","name":"Senior reviewer","default_skill_ids":["review"]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated role "+role.ID+": Senior reviewer") {
		t.Fatalf("update role response = %s", output.String())
	}
	updatedRoles, err := service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(updatedRoles) != 1 || updatedRoles[0].DefaultProfileID != "profile_reviewer" || updatedRoles[0].Name != "Senior reviewer" {
		t.Fatalf("updated roles = %+v, want patch preserving default profile", updatedRoles)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{
		ProjectID: project.ID,
		Title:     "Review MCP pull",
		Brief:     "Prove assignment claim and completion.",
		RootID:    "root_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"assignments.create","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","role_id":"` + role.ID + `","root_id":"root_main","execution_profile_id":"exec_local","desired_agent_kind":"any","skill_ids":["review"]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created assignment asgn_") {
		t.Fatalf("create assignment response = %s", output.String())
	}

	assignments, err := service.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("assignments = %+v, want one assignment", assignments)
	}
	if assignments[0].RootID != "root_main" {
		t.Fatalf("assignment root = %q, want root_main", assignments[0].RootID)
	}
	assignmentID := assignments[0].ID

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"assignments.next","arguments":{"project_id":"` + project.ID + `","agent_kind":"any","skill_ids":["review"]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Compatible assignments") || !strings.Contains(output.String(), assignmentID) {
		t.Fatalf("next assignments response = %s", output.String())
	}
	var nextResponse struct {
		Result struct {
			StructuredContent []core.Assignment `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &nextResponse); err != nil {
		t.Fatalf("next assignments response did not unmarshal: %v\n%s", err, output.String())
	}
	if len(nextResponse.Result.StructuredContent) != 1 || nextResponse.Result.StructuredContent[0].ID != assignmentID {
		t.Fatalf("next assignments = %+v, want assignment %s", nextResponse.Result.StructuredContent, assignmentID)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"assignments.claim","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `","claimed_by":"agent-a"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Claimed assignment "+assignmentID+" by agent-a") {
		t.Fatalf("claim assignment response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"assignments.update_status","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `","status":"running","execution_ref":"run-1"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated assignment "+assignmentID+": running") {
		t.Fatalf("update assignment status response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"assignments.next","arguments":{"project_id":"` + project.ID + `","agent_kind":"any","skill_ids":["review"]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if strings.Contains(output.String(), assignmentID) {
		t.Fatalf("next assignments after claim response = %s, want claimed assignment omitted", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"assignments.context","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Work item: Review MCP pull") || !strings.Contains(got, "Role: Senior reviewer") {
		t.Fatalf("assignment context response = %s", got)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"evidence.record","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","assignment_id":"` + assignmentID + `","title":"Test output","locator":"file://report.md"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Recorded evidence ev_") {
		t.Fatalf("record evidence response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"reviews.record","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","assignment_id":"` + assignmentID + `","reviewer_role_id":"` + role.ID + `","body":"Looks good.","verdict":"pass","risk":"low"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Recorded review rev_") || !strings.Contains(output.String(), "verdict=pass") {
		t.Fatalf("record review response = %s", output.String())
	}
	reviews, err := service.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() after record error = %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("reviews after record = %+v, want one review", reviews)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"handoffs.create","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","source_assignment_id":"` + assignmentID + `","source_run_id":"run-1","from_role_id":"` + role.ID + `","to_role_id":"` + role.ID + `","target_assignment_id":"` + assignmentID + `","target_work_item_id":"` + work.ID + `","title":"Next pass","body":"Use the recorded evidence.","recommended_next_action":"Inspect the launch packet.","linked_artifact_ids":["` + reviews[0].ID + `"],"linked_memory_ids":["mem_later"],"context_refs":["ctx_1"],"status":"accepted","provenance_kind":"operator","trust_label":"operator_reviewed"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created handoff handoff_") || !strings.Contains(output.String(), "status=accepted") {
		t.Fatalf("create handoff response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"memory_entries.create","arguments":{"project_id":"` + project.ID + `","title":"Accepted review convention","body":"Accepted memory should be visible in launch packets.","source_kind":"operator","source_id":"` + assignmentID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create memory entry error = %v", err)
	}
	if !strings.Contains(output.String(), "Created memory entry mem_") {
		t.Fatalf("create memory entry response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"memory_candidates.create","arguments":{"project_id":"` + project.ID + `","title":"Review convention","body":"Reviews should cite evidence.","suggested_source_kind":"assignment","suggested_source_id":"` + assignmentID + `","source_refs":[{"kind":"assignment","id":"` + assignmentID + `","title":"Assignment"}]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create memory candidate error = %v", err)
	}
	if !strings.Contains(output.String(), "Created memory candidate memcand_") {
		t.Fatalf("create memory candidate response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"assignments.launch_packet","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Launch packet launch_") || !strings.Contains(output.String(), `"structuredContent"`) {
		t.Fatalf("launch packet response = %s", output.String())
	}
	var launchResponse struct {
		Result struct {
			StructuredContent core.AssignmentLaunchPacket `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &launchResponse); err != nil {
		t.Fatalf("launch packet response did not unmarshal: %v\n%s", err, output.String())
	}
	packet := launchResponse.Result.StructuredContent
	if packet.Kind != core.LaunchPacketKindAssignment || packet.Assignment.ID != assignmentID || packet.Role == nil || packet.Role.ID != role.ID {
		t.Fatalf("launch packet = %+v, want structured assignment packet", packet)
	}
	if packet.Profile == nil || packet.Profile.ID != "profile_reviewer" || packet.ExecutionProfile == nil || packet.ExecutionProfile.ID != "exec_local" {
		t.Fatalf("launch packet = %+v, want resolved profile metadata", packet)
	}
	if len(packet.Memory) != 1 || packet.Memory[0].Title != "Accepted review convention" {
		t.Fatalf("launch packet memory = %+v, want accepted memory entry", packet.Memory)
	}
	if len(packet.Evidence) != 1 || len(packet.Reviews) != 1 || len(packet.Handoffs) != 1 || len(packet.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(packet.Evidence), len(packet.Reviews), len(packet.Handoffs), len(packet.MemoryCandidates))
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"assignments.complete","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `","execution_ref":"run-1"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated assignment "+assignmentID+": completed") {
		t.Fatalf("complete assignment response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"work_items.closeout_readiness","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() closeout readiness error = %v", err)
	}
	if !strings.Contains(output.String(), "Closeout readiness "+work.ID+": ready") || !strings.Contains(output.String(), "1/1 complete") {
		t.Fatalf("closeout readiness response = %s", output.String())
	}
	var readinessResponse struct {
		Result struct {
			StructuredContent core.WorkItemCloseoutReadiness `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &readinessResponse); err != nil {
		t.Fatalf("closeout readiness response did not unmarshal: %v\n%s", err, output.String())
	}
	if !readinessResponse.Result.StructuredContent.Ready || readinessResponse.Result.StructuredContent.CompletedAssignments != 1 {
		t.Fatalf("closeout readiness = %+v, want ready with completed assignment", readinessResponse.Result.StructuredContent)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":18,"method":"tools/call","params":{"name":"projects.operations_brief","arguments":{"project_id":"` + project.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() operations brief error = %v", err)
	}
	if !strings.Contains(output.String(), "Operations brief "+project.ID+": attention") || !strings.Contains(output.String(), "memory_candidates=1") || !strings.Contains(output.String(), "closeout_ready=1") {
		t.Fatalf("operations brief response = %s", output.String())
	}
	var operationsResponse struct {
		Result struct {
			StructuredContent core.ProjectOperationsBrief `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &operationsResponse); err != nil {
		t.Fatalf("operations brief response did not unmarshal: %v\n%s", err, output.String())
	}
	operations := operationsResponse.Result.StructuredContent
	if operations.Status != core.ProjectOperationsStatusAttention || operations.Next == nil || operations.Next.Kind != core.ProjectOperationKindMemoryCandidate || operations.Counts.CloseoutReady != 1 {
		t.Fatalf("operations brief = %+v, want memory candidate next and closeout-ready count", operations)
	}

	evidence, err := service.ListEvidence(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListEvidence() error = %v", err)
	}
	handoffs, err := service.ListHandoffs(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListHandoffs() error = %v", err)
	}
	if len(handoffs) != 1 || handoffs[0].Status != core.HandoffStatusAccepted || handoffs[0].SourceAssignmentID != assignmentID || handoffs[0].TargetAssignmentID != assignmentID || len(handoffs[0].LinkedArtifactIDs) != 1 || len(handoffs[0].ContextRefs) != 1 {
		t.Fatalf("handoffs = %+v, want metadata from MCP tool", handoffs)
	}
	memory, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	memoryEntries, err := service.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries() error = %v", err)
	}
	if len(evidence) != 1 || len(reviews) != 1 || len(handoffs) != 1 || len(memoryEntries) != 1 || len(memory) != 1 {
		t.Fatalf("artifact counts evidence=%d reviews=%d handoffs=%d memory_entries=%d memory_candidates=%d, want all one", len(evidence), len(reviews), len(handoffs), len(memoryEntries), len(memory))
	}
	if evidence[0].AssignmentID != assignmentID {
		t.Fatalf("evidence = %+v, want assignment-scoped evidence from MCP tool", evidence[0])
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":19,"method":"resources/list"}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() resources/list error = %v", err)
	}
	projectURI := "cairnline://projects/" + project.ID
	launchURI := projectURI + "/assignments/" + assignmentID + "/launch-packet"
	if got := output.String(); !strings.Contains(got, projectURI) || !strings.Contains(got, launchURI) {
		t.Fatalf("resources/list response = %s, want project and launch packet resources", got)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":20,"method":"resources/read","params":{"uri":"` + projectURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() project resources/read error = %v", err)
	}
	projectResourceText := readSingleResourceText(t, output.Bytes())
	if !strings.Contains(projectResourceText, `"project"`) || !strings.Contains(projectResourceText, `"operations"`) || !strings.Contains(projectResourceText, `"roles"`) || !strings.Contains(projectResourceText, `"assignments"`) || !strings.Contains(projectResourceText, `"memory"`) {
		t.Fatalf("project resources/read text = %s", projectResourceText)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":21,"method":"resources/read","params":{"uri":"` + launchURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() launch resources/read error = %v", err)
	}
	resourceResponse := readSingleResourceResponse(t, output.Bytes())
	if resourceResponse.URI != launchURI || resourceResponse.MimeType != "application/json" {
		t.Fatalf("launch resource content = %+v, want JSON launch resource", resourceResponse)
	}
	var packetFromResource core.AssignmentLaunchPacket
	if err := json.Unmarshal([]byte(resourceResponse.Text), &packetFromResource); err != nil {
		t.Fatalf("launch resource text did not unmarshal: %v\n%s", err, resourceResponse.Text)
	}
	if packetFromResource.Assignment.ID != assignmentID || packetFromResource.Kind != core.LaunchPacketKindAssignment {
		t.Fatalf("launch resource packet = %+v, want assignment launch packet", packetFromResource)
	}
}

func TestMCPTools_MemoryEntryLifecycle(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")
	project, err := service.CreateProject(ctx, core.Project{Name: "Memory project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	var output bytes.Buffer
	input := toolRequest(t, 1, "memory_entries.create", map[string]any{
		"project_id":  project.ID,
		"title":       "Accepted convention",
		"body":        "Use accepted memory only when explicitly saved.",
		"trust_label": core.MemoryTrustGenerated,
		"source_kind": core.MemorySourceGenerated,
		"source_id":   "memcand_1",
	})
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create memory entry error = %v", err)
	}
	if !strings.Contains(output.String(), "Created memory entry mem_") {
		t.Fatalf("create memory entry response = %s", output.String())
	}
	entries, err := service.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v, want one created entry", entries)
	}
	memoryID := entries[0].ID

	input = toolRequest(t, 2, "memory_entries.list", map[string]any{"project_id": project.ID})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list memory entries error = %v", err)
	}
	var listResponse struct {
		Result struct {
			StructuredContent []core.MemoryEntry `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &listResponse); err != nil {
		t.Fatalf("list response did not unmarshal: %v\n%s", err, output.String())
	}
	if len(listResponse.Result.StructuredContent) != 1 || listResponse.Result.StructuredContent[0].ID != memoryID {
		t.Fatalf("listed memory entries = %+v, want created entry", listResponse.Result.StructuredContent)
	}

	input = toolRequest(t, 3, "memory_entries.get", map[string]any{
		"project_id": project.ID,
		"memory_id":  memoryID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get memory entry error = %v", err)
	}
	var getResponse struct {
		Result struct {
			StructuredContent core.MemoryEntry `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &getResponse); err != nil {
		t.Fatalf("get response did not unmarshal: %v\n%s", err, output.String())
	}
	if getResponse.Result.StructuredContent.ID != memoryID || getResponse.Result.StructuredContent.TrustLabel != core.MemoryTrustGenerated {
		t.Fatalf("got memory entry = %+v, want created generated entry", getResponse.Result.StructuredContent)
	}

	input = toolRequest(t, 4, "memory_entries.update", map[string]any{
		"project_id": project.ID,
		"memory_id":  memoryID,
		"title":      "Disabled convention",
		"enabled":    false,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() update memory entry error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated memory entry "+memoryID) {
		t.Fatalf("update memory entry response = %s", output.String())
	}

	input = toolRequest(t, 5, "memory_entries.list", map[string]any{"project_id": project.ID})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list enabled memory entries error = %v", err)
	}
	if !strings.Contains(output.String(), "No memory entries.") {
		t.Fatalf("enabled memory entries response = %s, want disabled entry omitted", output.String())
	}

	input = toolRequest(t, 6, "memory_entries.list", map[string]any{
		"project_id":       project.ID,
		"include_disabled": true,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list all memory entries error = %v", err)
	}
	if !strings.Contains(output.String(), "Disabled convention") || !strings.Contains(output.String(), "disabled") {
		t.Fatalf("all memory entries response = %s, want disabled entry included", output.String())
	}

	input = toolRequest(t, 7, "memory_entries.delete", map[string]any{
		"project_id": project.ID,
		"memory_id":  memoryID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() delete memory entry error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted memory entry "+memoryID) {
		t.Fatalf("delete memory entry response = %s", output.String())
	}
	entries, err = service.ListMemoryEntries(ctx, project.ID, true)
	if err != nil {
		t.Fatalf("ListMemoryEntries(all after delete) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries after delete = %+v, want none", entries)
	}
}

func TestMCPTools_MemoryCandidateDecisionLifecycle(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")
	project, err := service.CreateProject(ctx, core.Project{Name: "Candidate project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	var output bytes.Buffer
	input := toolRequest(t, 1, "memory_candidates.create", map[string]any{
		"project_id":            project.ID,
		"title":                 "Generated lesson",
		"body":                  "Generated summaries should be reviewed before becoming memory.",
		"suggested_kind":        "note",
		"suggested_trust_label": core.MemoryTrustGenerated,
		"suggested_source_kind": core.MemorySourceGenerated,
		"suggested_source_id":   "run_1",
		"source_refs": []map[string]any{{
			"kind":  "task_run",
			"id":    "run_1",
			"title": "Task run",
		}},
	})
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create memory candidate error = %v", err)
	}
	if !strings.Contains(output.String(), "Created memory candidate memcand_") {
		t.Fatalf("create memory candidate response = %s", output.String())
	}
	candidates, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want one pending candidate", candidates)
	}
	candidateID := candidates[0].ID

	input = toolRequest(t, 2, "memory_candidates.list", map[string]any{"project_id": project.ID})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list memory candidates error = %v", err)
	}
	var listResponse struct {
		Result struct {
			StructuredContent []core.MemoryCandidate `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &listResponse); err != nil {
		t.Fatalf("list response did not unmarshal: %v\n%s", err, output.String())
	}
	if len(listResponse.Result.StructuredContent) != 1 || listResponse.Result.StructuredContent[0].ID != candidateID || len(listResponse.Result.StructuredContent[0].SourceRefs) != 1 {
		t.Fatalf("listed candidates = %+v, want created candidate with source ref", listResponse.Result.StructuredContent)
	}

	input = toolRequest(t, 3, "memory_candidates.get", map[string]any{
		"project_id":   project.ID,
		"candidate_id": candidateID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get memory candidate error = %v", err)
	}
	if !strings.Contains(output.String(), "status=pending") || !strings.Contains(output.String(), "refs=1") {
		t.Fatalf("get memory candidate response = %s, want pending source-ref summary", output.String())
	}

	promotedTitle := "Reviewed generated lesson"
	input = toolRequest(t, 4, "memory_candidates.promote", map[string]any{
		"project_id":   project.ID,
		"candidate_id": candidateID,
		"title":        promotedTitle,
		"trust_label":  core.MemoryTrustOperator,
		"source_kind":  core.MemorySourceOperator,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() promote memory candidate error = %v", err)
	}
	if !strings.Contains(output.String(), "Promoted memory candidate "+candidateID+" to memory entry mem_") {
		t.Fatalf("promote memory candidate response = %s", output.String())
	}
	entries, err := service.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Title != promotedTitle || entries[0].TrustLabel != core.MemoryTrustOperator {
		t.Fatalf("memory entries = %+v, want promoted accepted memory", entries)
	}

	input = toolRequest(t, 5, "memory_candidates.list", map[string]any{"project_id": project.ID})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list pending memory candidates error = %v", err)
	}
	if !strings.Contains(output.String(), "No memory candidates.") {
		t.Fatalf("pending candidates response = %s, want promoted candidate omitted", output.String())
	}

	input = toolRequest(t, 6, "memory_candidates.list", map[string]any{
		"project_id":       project.ID,
		"include_resolved": true,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list resolved memory candidates error = %v", err)
	}
	if !strings.Contains(output.String(), "status=promoted") || !strings.Contains(output.String(), "promoted_memory="+entries[0].ID) {
		t.Fatalf("resolved candidates response = %s, want promoted candidate", output.String())
	}

	input = toolRequest(t, 7, "memory_candidates.create", map[string]any{
		"project_id": project.ID,
		"title":      "Speculative lesson",
		"body":       "Maybe skip validation.",
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create reject candidate error = %v", err)
	}
	candidates, err = service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(after create reject) error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("pending candidates after create reject = %+v, want one", candidates)
	}
	rejectID := candidates[0].ID
	input = toolRequest(t, 8, "memory_candidates.reject", map[string]any{
		"project_id":   project.ID,
		"candidate_id": rejectID,
		"reason":       "Not durable.",
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() reject memory candidate error = %v", err)
	}
	if !strings.Contains(output.String(), "Rejected memory candidate "+rejectID) {
		t.Fatalf("reject memory candidate response = %s", output.String())
	}
	rejected, err := service.GetMemoryCandidate(ctx, project.ID, rejectID)
	if err != nil {
		t.Fatalf("GetMemoryCandidate(rejected) error = %v", err)
	}
	if rejected.Status != core.MemoryCandidateRejected || rejected.StatusReason != "Not durable." {
		t.Fatalf("rejected candidate = %+v, want rejected reason", rejected)
	}

	input = toolRequest(t, 9, "memory_candidates.delete", map[string]any{
		"project_id":   project.ID,
		"candidate_id": rejectID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() delete memory candidate error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted memory candidate "+rejectID) {
		t.Fatalf("delete memory candidate response = %s", output.String())
	}
}

func TestMCPTools_ProjectSkillsRegistry(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")
	root := t.TempDir()
	skillDir := filepath.Join(root, ".agents", "skills", "backend")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: Backend\n---\n# Backend body\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var output bytes.Buffer
	input := toolRequest(t, 1, "projects.create", map[string]any{
		"name": "Skills project",
		"roots": []map[string]any{{
			"id":     "root_main",
			"path":   root,
			"active": true,
		}},
	})
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create project error = %v", err)
	}
	projects, err := service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || len(projects[0].Roots) != 1 || !projects[0].Roots[0].Active {
		t.Fatalf("projects = %+v, want active workspace root from MCP create", projects)
	}
	projectID := projects[0].ID

	input = toolRequest(t, 2, "skills.discover", map[string]any{"project_id": projectID})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() discover skills error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Discovered project skills") || !strings.Contains(got, "backend") {
		t.Fatalf("discover skills response = %s", got)
	}
	var discoverResponse struct {
		Result struct {
			StructuredContent []core.ProjectSkill `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &discoverResponse); err != nil {
		t.Fatalf("discover response did not unmarshal: %v\n%s", err, output.String())
	}
	if len(discoverResponse.Result.StructuredContent) != 1 || discoverResponse.Result.StructuredContent[0].ID != "backend" {
		t.Fatalf("structured discovered skills = %+v, want backend", discoverResponse.Result.StructuredContent)
	}

	input = toolRequest(t, 3, "skills.update", map[string]any{
		"project_id": projectID,
		"id":         "backend",
		"title":      "Backend disabled",
		"enabled":    false,
		"status":     core.SkillStatusAvailable,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() update skill error = %v", err)
	}
	skills, err := service.ListProjectSkills(ctx, projectID)
	if err != nil {
		t.Fatalf("ListProjectSkills() error = %v", err)
	}
	if len(skills) != 1 || skills[0].Enabled || skills[0].Title != "Backend disabled" || skills[0].Path != ".agents/skills/backend/SKILL.md" {
		t.Fatalf("skills after update = %+v, want disabled patch preserving discovered path", skills)
	}

	input = toolRequest(t, 4, "skills.list", map[string]any{"project_id": projectID})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list skills error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Project skills") || !strings.Contains(got, "disabled") {
		t.Fatalf("list skills response = %s", got)
	}
}

func toolRequest(t *testing.T, id int, name string, arguments any) *strings.Reader {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	})
	if err != nil {
		t.Fatalf("Marshal tool request error = %v", err)
	}
	return strings.NewReader(string(raw) + "\n")
}

type resourceTextContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

func readSingleResourceText(t *testing.T, raw []byte) string {
	t.Helper()
	return readSingleResourceResponse(t, raw).Text
}

func readSingleResourceResponse(t *testing.T, raw []byte) resourceTextContent {
	t.Helper()
	var response struct {
		Result struct {
			Contents []resourceTextContent `json:"contents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("resource response did not unmarshal: %v\n%s", err, string(raw))
	}
	if len(response.Result.Contents) != 1 {
		t.Fatalf("resource contents = %+v, want one", response.Result.Contents)
	}
	return response.Result.Contents[0]
}
