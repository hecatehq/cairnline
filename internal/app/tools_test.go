package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"evidence.record","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","title":"Test output","locator":"file://report.md"}}}` + "\n",
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

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"handoffs.create","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","from_role_id":"` + role.ID + `","to_role_id":"` + role.ID + `","title":"Next pass","body":"Use the recorded evidence."}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created handoff handoff_") {
		t.Fatalf("create handoff response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"memory_candidates.create","arguments":{"project_id":"` + project.ID + `","title":"Review convention","body":"Reviews should cite evidence.","source_ref":"` + assignmentID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created memory candidate memcand_") {
		t.Fatalf("create memory candidate response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"assignments.launch_packet","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `"}}}` + "\n",
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
	if len(packet.Evidence) != 1 || len(packet.Reviews) != 1 || len(packet.Handoffs) != 1 || len(packet.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(packet.Evidence), len(packet.Reviews), len(packet.Handoffs), len(packet.MemoryCandidates))
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"assignments.complete","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `","execution_ref":"run-1"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated assignment "+assignmentID+": completed") {
		t.Fatalf("complete assignment response = %s", output.String())
	}

	evidence, err := service.ListEvidence(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListEvidence() error = %v", err)
	}
	reviews, err := service.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	handoffs, err := service.ListHandoffs(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListHandoffs() error = %v", err)
	}
	memory, err := service.ListMemoryCandidates(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(evidence) != 1 || len(reviews) != 1 || len(handoffs) != 1 || len(memory) != 1 {
		t.Fatalf("artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(evidence), len(reviews), len(handoffs), len(memory))
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":16,"method":"resources/list"}` + "\n",
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
		`{"jsonrpc":"2.0","id":17,"method":"resources/read","params":{"uri":"` + projectURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() project resources/read error = %v", err)
	}
	projectResourceText := readSingleResourceText(t, output.Bytes())
	if !strings.Contains(projectResourceText, `"project"`) || !strings.Contains(projectResourceText, `"roles"`) || !strings.Contains(projectResourceText, `"assignments"`) {
		t.Fatalf("project resources/read text = %s", projectResourceText)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":18,"method":"resources/read","params":{"uri":"` + launchURI + `"}}` + "\n",
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

func TestMCPTools_GetAndDeleteAssignment(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")
	project, err := service.CreateProject(ctx, core.Project{Name: "Cleanup"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Delete assignment"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{
		ProjectID:    project.ID,
		WorkItemID:   work.ID,
		AssignmentID: assignment.ID,
		Body:         "Delete with assignment.",
		Verdict:      core.ReviewVerdictPass,
	}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}

	input := toolRequest(t, 1, "assignments.get", map[string]any{
		"project_id":    project.ID,
		"assignment_id": assignment.ID,
	})
	var output bytes.Buffer
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get assignment error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Assignment (1):") || !strings.Contains(got, assignment.ID) {
		t.Fatalf("get assignment response = %s", got)
	}
	var getResponse struct {
		Result struct {
			StructuredContent core.Assignment `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &getResponse); err != nil {
		t.Fatalf("get assignment response did not unmarshal: %v\n%s", err, output.String())
	}
	if getResponse.Result.StructuredContent.ID != assignment.ID {
		t.Fatalf("structured assignment = %+v, want %s", getResponse.Result.StructuredContent, assignment.ID)
	}

	input = toolRequest(t, 2, "assignments.delete", map[string]any{
		"project_id":    project.ID,
		"assignment_id": assignment.ID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() delete assignment error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted assignment "+assignment.ID) {
		t.Fatalf("delete assignment response = %s", output.String())
	}
	if _, err := service.GetAssignment(ctx, project.ID, assignment.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetAssignment(deleted) error = %v, want ErrNotFound", err)
	}
	reviews, err := service.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("reviews = %+v, want deleted assignment review removed", reviews)
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
