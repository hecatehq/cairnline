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
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects.create","arguments":{"name":"Research notes","description":"Coordinate synthesis.","default_profile_id":"profile_research","default_execution_profile_id":"exec_local","context_sources":[{"id":"src_agents","kind":"workspace_instruction","title":"AGENTS.md","locator":"AGENTS.md","format":"agents_md","scope":"workspace","trust_label":"workspace_guidance","source_category":"instructions","metadata":{"root_id":"root_main"}}]}}}` + "\n",
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
	if projects[0].DefaultProfileID != "profile_research" || projects[0].DefaultExecutionProfileID != "exec_local" {
		t.Fatalf("project defaults = %q/%q, want MCP-created defaults", projects[0].DefaultProfileID, projects[0].DefaultExecutionProfileID)
	}
	if len(projects[0].ContextSources) != 1 || projects[0].ContextSources[0].Format != "agents_md" || projects[0].ContextSources[0].Metadata["root_id"] != "root_main" {
		t.Fatalf("project sources = %+v, want MCP-created source metadata", projects[0].ContextSources)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"projects.get","arguments":{"id":"` + projects[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Project "+projects[0].ID+": Research notes") || !strings.Contains(output.String(), `"structuredContent"`) {
		t.Fatalf("get project response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"projects.update","arguments":{"id":"` + projects[0].ID + `","description":"Updated synthesis coordination.","default_profile_id":"profile_writer","default_execution_profile_id":"exec_review","context_sources":[{"id":"src_agents","kind":"workspace_instruction","title":"Repository guidance","locator":"AGENTS.md","enabled":false,"format":"agents_md","scope":"workspace","trust_label":"workspace_guidance","source_category":"instructions","metadata":{"root_id":"root_main","source":"manual"}}]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated project "+projects[0].ID) {
		t.Fatalf("update project response = %s", output.String())
	}
	projects, err = service.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() after update error = %v", err)
	}
	updatedSource := projects[0].ContextSources[0]
	if updatedSource.Title != "Repository guidance" || updatedSource.Enabled || updatedSource.Metadata["source"] != "manual" {
		t.Fatalf("updated project source = %+v, want MCP-updated source metadata", updatedSource)
	}
	if projects[0].DefaultProfileID != "profile_writer" || projects[0].DefaultExecutionProfileID != "exec_review" {
		t.Fatalf("updated project defaults = %q/%q, want MCP-updated defaults", projects[0].DefaultProfileID, projects[0].DefaultExecutionProfileID)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"work_items.create","arguments":{"project_id":"` + projects[0].ID + `","title":"Summarize interviews","brief":"Produce themes."}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Created work item work_") {
		t.Fatalf("create work item response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"work_items.list","arguments":{"project_id":"` + projects[0].ID + `"}}}` + "\n",
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
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"work_items.get","arguments":{"project_id":"` + projects[0].ID + `","id":"` + workItems[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Work item "+workItems[0].ID+": [ready] Summarize interviews") || !strings.Contains(output.String(), `"structuredContent"`) {
		t.Fatalf("get work item response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"work_items.update","arguments":{"project_id":"` + projects[0].ID + `","id":"` + workItems[0].ID + `","brief":"Updated themes."}}}` + "\n",
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

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"work_items.delete","arguments":{"project_id":"` + projects[0].ID + `","id":"` + workItems[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted work item "+workItems[0].ID) {
		t.Fatalf("delete work item response = %s", output.String())
	}
	if _, err := service.GetWorkItem(context.Background(), projects[0].ID, workItems[0].ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetWorkItem(deleted) error = %v, want ErrNotFound", err)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"projects.delete","arguments":{"id":"` + projects[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted project "+projects[0].ID) {
		t.Fatalf("delete project response = %s", output.String())
	}
}

func TestMCPTools_DeleteProfilesAndRoles(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"profiles.create","arguments":{"id":"profile_temp","name":"Temporary profile"}}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve(profile create) error = %v", err)
	}
	if !strings.Contains(output.String(), "Created agent profile profile_temp") {
		t.Fatalf("create profile response = %s", output.String())
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"profiles.delete","arguments":{"id":"profile_temp"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve(profile delete) error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted agent profile profile_temp") {
		t.Fatalf("delete profile response = %s", output.String())
	}
	profiles, err := service.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() error = %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles after delete = %+v, want none", profiles)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"execution_profiles.create","arguments":{"id":"exec_temp","name":"Temporary execution","agent_kind":"any"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve(execution create) error = %v", err)
	}
	if !strings.Contains(output.String(), "Created execution profile exec_temp") {
		t.Fatalf("create execution profile response = %s", output.String())
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"execution_profiles.delete","arguments":{"id":"exec_temp"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve(execution delete) error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted execution profile exec_temp") {
		t.Fatalf("delete execution profile response = %s", output.String())
	}
	executionProfiles, err := service.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() error = %v", err)
	}
	if len(executionProfiles) != 0 {
		t.Fatalf("execution profiles after delete = %+v, want none", executionProfiles)
	}

	project, err := service.CreateProject(ctx, core.Project{Name: "Delete role"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"roles.create","arguments":{"project_id":"` + project.ID + `","name":"Temporary role"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve(role create) error = %v", err)
	}
	roles, err := service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("roles after create = %+v, want one role", roles)
	}
	if !strings.Contains(output.String(), "Created role "+roles[0].ID+": Temporary role") {
		t.Fatalf("create role response = %s", output.String())
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"roles.delete","arguments":{"project_id":"` + project.ID + `","id":"` + roles[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve(role delete) error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted role "+roles[0].ID) {
		t.Fatalf("delete role response = %s", output.String())
	}
	roles, err = service.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() after delete error = %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("roles after delete = %+v, want none", roles)
	}
}

func TestMCPTools_AssistantProposalApply(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")
	project, err := service.CreateProject(ctx, core.Project{Name: "Assistant MCP"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	proposal := core.AssistantProposal{
		ID:        "prop_mcp",
		ProjectID: project.ID,
		Title:     "Queue first assignment",
		Warnings:  []string{"review before apply"},
		Actions: []core.AssistantAction{
			{
				Kind:   core.AssistantActionAttachProjectRoot,
				Target: core.AssistantTarget{ProjectID: project.ID},
				Root:   &core.Root{ID: "root_mcp", Path: "/workspace/mcp", Kind: "local", Active: true},
			},
			{
				Kind: core.AssistantActionCreateRole,
				Role: &core.Role{ID: "role_mcp", ProjectID: project.ID, Name: "Operator"},
			},
			{
				Kind:     core.AssistantActionCreateWorkItem,
				WorkItem: &core.WorkItem{ID: "work_mcp", ProjectID: project.ID, Title: "MCP proposed work"},
			},
			{
				Kind: core.AssistantActionCreateAssignment,
				Assignment: &core.Assignment{
					ID:            "asgn_mcp",
					ProjectID:     project.ID,
					WorkItemID:    "work_mcp",
					RoleID:        "role_mcp",
					ExecutionMode: core.ExecutionMCPPull,
				},
			},
		},
	}
	proposePayload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "assistant.propose",
			"arguments": proposal,
		},
	})
	if err != nil {
		t.Fatalf("marshal propose payload: %v", err)
	}
	var output bytes.Buffer
	if err := server.Serve(ctx, bytes.NewReader(append(proposePayload, '\n')), &output); err != nil {
		t.Fatalf("Serve(propose) error = %v", err)
	}
	if !strings.Contains(output.String(), "Assistant proposal prop_mcp") || !strings.Contains(output.String(), "Warning: review before apply") || !strings.Contains(output.String(), "requires_confirmation=true") || !strings.Contains(output.String(), `"structuredContent"`) {
		t.Fatalf("propose response = %s", output.String())
	}
	var proposeResponse struct {
		Result struct {
			StructuredContent core.AssistantProposalRecord `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &proposeResponse); err != nil {
		t.Fatalf("decode propose response: %v\n%s", err, output.String())
	}
	if proposeResponse.Result.StructuredContent.Status != core.AssistantProposalStatusProposed || proposeResponse.Result.StructuredContent.Proposal.ID != "prop_mcp" {
		t.Fatalf("proposal record = %+v, want proposed prop_mcp", proposeResponse.Result.StructuredContent)
	}
	if len(proposeResponse.Result.StructuredContent.Proposal.Warnings) != 1 || proposeResponse.Result.StructuredContent.Proposal.Warnings[0] != "review before apply" {
		t.Fatalf("proposal warnings = %+v, want warning in structured proposal", proposeResponse.Result.StructuredContent.Proposal.Warnings)
	}

	listPayload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "list-proposals",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "assistant.proposals.list",
			"arguments": map[string]any{
				"project_id": project.ID,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal proposal list payload: %v", err)
	}
	output.Reset()
	if err := server.Serve(ctx, bytes.NewReader(append(listPayload, '\n')), &output); err != nil {
		t.Fatalf("Serve(proposals.list) error = %v", err)
	}
	if !strings.Contains(output.String(), "Assistant proposals (1):") || !strings.Contains(output.String(), "prop_mcp") {
		t.Fatalf("proposal list response = %s", output.String())
	}

	getPayload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "get-proposal",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "assistant.proposals.get",
			"arguments": map[string]any{
				"id": proposeResponse.Result.StructuredContent.ID,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal proposal get payload: %v", err)
	}
	output.Reset()
	if err := server.Serve(ctx, bytes.NewReader(append(getPayload, '\n')), &output); err != nil {
		t.Fatalf("Serve(proposals.get) error = %v", err)
	}
	if !strings.Contains(output.String(), "Assistant proposal prop_mcp") {
		t.Fatalf("proposal get response = %s", output.String())
	}

	applyPayload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "assistant.apply",
			"arguments": map[string]any{
				"proposal_id": proposeResponse.Result.StructuredContent.ID,
				"confirm":     true,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal apply payload: %v", err)
	}
	output.Reset()
	if err := server.Serve(ctx, bytes.NewReader(append(applyPayload, '\n')), &output); err != nil {
		t.Fatalf("Serve(apply) error = %v", err)
	}
	if !strings.Contains(output.String(), "Assistant apply prop_mcp: applied") || !strings.Contains(output.String(), "actions=4/4") || !strings.Contains(output.String(), "root=root_mcp") {
		t.Fatalf("apply response = %s", output.String())
	}
	var applyResponse struct {
		Result struct {
			StructuredContent core.AssistantApplyResult `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &applyResponse); err != nil {
		t.Fatalf("decode apply response: %v\n%s", err, output.String())
	}
	if !applyResponse.Result.StructuredContent.Applied || applyResponse.Result.StructuredContent.AppliedActionCount != 4 || applyResponse.Result.StructuredContent.Actions[0].RootID != "root_mcp" {
		t.Fatalf("apply result = %+v, want full confirmed apply", applyResponse.Result.StructuredContent)
	}
	assignment, err := service.GetAssignment(ctx, project.ID, "asgn_mcp")
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if assignment.Status != core.AssignmentQueued {
		t.Fatalf("assignment = %+v, want queued", assignment)
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
	if project.DefaultRootID != "root_main" {
		t.Fatalf("default root = %q, want root_main", project.DefaultRootID)
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"projects.update","arguments":{"id":"` + project.ID + `","roots":[{"id":"root_review","path":"/workspace/dogfood-review","kind":"git_worktree"}]}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated project "+project.ID+": Dogfood") {
		t.Fatalf("update project response = %s", output.String())
	}
	projects, err := service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].DefaultRootID != "root_review" {
		t.Fatalf("projects = %+v, want default root to follow replacement roots", projects)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:                 project.ID,
		Name:                      "Reviewer",
		Instructions:              "Review evidence.",
		DefaultProfileID:          "profile_reviewer",
		DefaultExecutionProfileID: "exec_local",
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
	if len(updatedRoles) != 1 || updatedRoles[0].DefaultProfileID != "profile_reviewer" || updatedRoles[0].DefaultExecutionProfileID != "exec_local" || updatedRoles[0].Name != "Senior reviewer" {
		t.Fatalf("updated roles = %+v, want patch preserving default profiles", updatedRoles)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{
		ProjectID: project.ID,
		Title:     "Review MCP pull",
		Brief:     "Prove assignment claim and completion.",
		RootID:    "root_review",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"assignments.create","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","role_id":"` + role.ID + `","root_id":"root_review","execution_profile_id":"exec_local","desired_agent_kind":"any","skill_ids":["review"]}}}` + "\n",
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
	if assignments[0].RootID != "root_review" {
		t.Fatalf("assignment root = %q, want root_review", assignments[0].RootID)
	}
	assignmentID := assignments[0].ID
	updatedWork, err := service.CreateWorkItem(ctx, core.WorkItem{
		ProjectID: project.ID,
		Title:     "Review MCP pull updated",
		Brief:     "Retargeted work item.",
		RootID:    "root_review",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem(updated) error = %v", err)
	}
	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":40,"method":"tools/call","params":{"name":"assignments.update","arguments":{"project_id":"` + project.ID + `","assignment_id":"` + assignmentID + `","work_item_id":"` + updatedWork.ID + `","role_id":"` + role.ID + `","root_id":"root_review","profile_id":"profile_reviewer","execution_profile_id":"exec_local","execution_mode":"mcp_pull","desired_agent_kind":"codex","skill_ids":["review","backend","backend"],"status":"queued","execution_ref":"chat-1","context_snapshot_id":"ctx-1"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() update assignment error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated assignment "+assignmentID) || !strings.Contains(output.String(), `"structuredContent"`) {
		t.Fatalf("update assignment response = %s", output.String())
	}
	var updateAssignmentResponse struct {
		Result struct {
			StructuredContent core.Assignment `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &updateAssignmentResponse); err != nil {
		t.Fatalf("update assignment response did not unmarshal: %v\n%s", err, output.String())
	}
	updatedAssignment := updateAssignmentResponse.Result.StructuredContent
	if updatedAssignment.WorkItemID != updatedWork.ID || updatedAssignment.ExecutionMode != core.ExecutionMCPPull || updatedAssignment.ProfileID != "profile_reviewer" || updatedAssignment.ExecutionProfileID != "exec_local" || updatedAssignment.ContextSnapshotID != "ctx-1" {
		t.Fatalf("updated assignment = %+v, want retargeted assignment metadata", updatedAssignment)
	}
	if updatedAssignment.DesiredAgent.Kind != "codex" || len(updatedAssignment.DesiredAgent.SkillIDs) != 2 || updatedAssignment.DesiredAgent.SkillIDs[1] != "backend" {
		t.Fatalf("updated assignment desired agent = %+v, want normalized desired agent", updatedAssignment.DesiredAgent)
	}
	work = updatedWork

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":41,"method":"tools/call","params":{"name":"artifacts.create","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","assignment_id":"` + assignmentID + `","kind":"decision_note","title":"Decision","body":"Record a generic collaboration artifact.","author_role_id":"` + role.ID + `","provenance_kind":"operator","trust_label":"operator_reviewed"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create artifact error = %v", err)
	}
	if !strings.Contains(output.String(), "Created artifact art_") {
		t.Fatalf("create artifact response = %s", output.String())
	}
	var artifactResponse struct {
		Result struct {
			StructuredContent core.Artifact `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &artifactResponse); err != nil {
		t.Fatalf("create artifact response did not unmarshal: %v\n%s", err, output.String())
	}
	artifact := artifactResponse.Result.StructuredContent
	if artifact.Kind != "decision_note" || artifact.AssignmentID != assignmentID || artifact.AuthorRoleID != role.ID || artifact.TrustLabel != "operator_reviewed" {
		t.Fatalf("created artifact = %+v, want generic artifact metadata", artifact)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"artifacts.list","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list artifacts error = %v", err)
	}
	if !strings.Contains(output.String(), "Artifacts (1):") || !strings.Contains(output.String(), "decision_note") {
		t.Fatalf("list artifacts response = %s", output.String())
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":43,"method":"tools/call","params":{"name":"artifacts.get","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","artifact_id":"` + artifact.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get artifact error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Artifacts (1):") || !strings.Contains(got, artifact.ID) {
		t.Fatalf("get artifact response = %s", got)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"assignments.next","arguments":{"project_id":"` + project.ID + `","agent_kind":"any","skill_ids":["review","backend"]}}}` + "\n",
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
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"assignments.next","arguments":{"project_id":"` + project.ID + `","agent_kind":"any","skill_ids":["review","backend"]}}}` + "\n",
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
	evidence, err := service.ListEvidence(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListEvidence() after record error = %v", err)
	}
	if len(evidence) != 1 {
		t.Fatalf("evidence after record = %+v, want one evidence record", evidence)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":101,"method":"tools/call","params":{"name":"evidence.list","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list evidence error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Evidence (1):") || !strings.Contains(got, evidence[0].ID) || !strings.Contains(got, `"structuredContent"`) {
		t.Fatalf("list evidence response = %s", got)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":102,"method":"tools/call","params":{"name":"evidence.get","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","evidence_id":"` + evidence[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get evidence error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Evidence (1):") || !strings.Contains(got, "file://report.md") {
		t.Fatalf("get evidence response = %s", got)
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
		`{"jsonrpc":"2.0","id":103,"method":"tools/call","params":{"name":"reviews.list","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list reviews error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Reviews (1):") || !strings.Contains(got, reviews[0].ID) || !strings.Contains(got, `"structuredContent"`) {
		t.Fatalf("list reviews response = %s", got)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":104,"method":"tools/call","params":{"name":"reviews.get","arguments":{"project_id":"` + project.ID + `","work_item_id":"` + work.ID + `","review_id":"` + reviews[0].ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get review error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Review (1):") || !strings.Contains(got, "risk=low") {
		t.Fatalf("get review response = %s", got)
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
	if len(packet.Artifacts) != 1 || len(packet.Evidence) != 1 || len(packet.Reviews) != 1 || len(packet.Handoffs) != 1 || len(packet.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts artifacts=%d evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(packet.Artifacts), len(packet.Evidence), len(packet.Reviews), len(packet.Handoffs), len(packet.MemoryCandidates))
	}
	if packet.Artifacts[0].ID != artifact.ID || packet.Artifacts[0].Kind != "decision_note" {
		t.Fatalf("launch packet artifacts = %+v, want generic artifact", packet.Artifacts)
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

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":181,"method":"tools/call","params":{"name":"projects.setup_readiness","arguments":{"project_id":"` + project.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() setup readiness error = %v", err)
	}
	if !strings.Contains(output.String(), "Setup readiness "+project.ID) || !strings.Contains(output.String(), "setup_started=true") {
		t.Fatalf("setup readiness response = %s", output.String())
	}
	var setupResponse struct {
		Result struct {
			StructuredContent core.ProjectSetupReadiness `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &setupResponse); err != nil {
		t.Fatalf("setup readiness response did not unmarshal: %v\n%s", err, output.String())
	}
	setup := setupResponse.Result.StructuredContent
	if setup.ShowOnboarding || !setup.SetupStarted || setup.Summary.WorkItemCount == 0 || setup.Summary.RoleCount == 0 {
		t.Fatalf("setup readiness = %+v, want configured project with work and roles", setup)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":182,"method":"tools/call","params":{"name":"projects.health","arguments":{"project_id":"` + project.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() health error = %v", err)
	}
	if !strings.Contains(output.String(), "Project health "+project.ID+": attention") || !strings.Contains(output.String(), "closeout_ready") {
		t.Fatalf("health response = %s", output.String())
	}
	var healthResponse struct {
		Result struct {
			StructuredContent core.ProjectHealth `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &healthResponse); err != nil {
		t.Fatalf("health response did not unmarshal: %v\n%s", err, output.String())
	}
	health := healthResponse.Result.StructuredContent
	if health.Status != core.ProjectHealthStatusAttention || health.Summary.PendingMemoryCandidateCount != 1 || health.Summary.AttentionCount == 0 {
		t.Fatalf("health = %+v, want pending memory attention", health)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":19,"method":"tools/call","params":{"name":"projects.activity","arguments":{"project_id":"` + project.ID + `"}}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() activity error = %v", err)
	}
	if !strings.Contains(output.String(), "Project activity "+project.ID) || !strings.Contains(output.String(), "completed=1") {
		t.Fatalf("activity response = %s", output.String())
	}
	var activityResponse struct {
		Result struct {
			StructuredContent core.ProjectActivity `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &activityResponse); err != nil {
		t.Fatalf("activity response did not unmarshal: %v\n%s", err, output.String())
	}
	activity := activityResponse.Result.StructuredContent
	if activity.Counts.Assignments != 1 || activity.Counts.Completed != 1 || len(activity.Buckets.Completed) != 1 || activity.Buckets.Completed[0].AssignmentID != assignmentID {
		t.Fatalf("activity = %+v, want completed assignment bucket", activity)
	}

	evidence, err = service.ListEvidence(ctx, project.ID, work.ID)
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
	candidateID := memory[0].ID

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":20,"method":"resources/list"}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() resources/list error = %v", err)
	}
	projectURI := "cairnline://projects/" + project.ID
	workItemURI := projectURI + "/work-items/" + work.ID
	readinessURI := projectURI + "/work-items/" + work.ID + "/closeout-readiness"
	launchURI := projectURI + "/assignments/" + assignmentID + "/launch-packet"
	candidateURI := projectURI + "/memory-candidates/" + candidateID
	if got := output.String(); !strings.Contains(got, projectURI) || !strings.Contains(got, workItemURI) || !strings.Contains(got, readinessURI) || !strings.Contains(got, launchURI) || !strings.Contains(got, candidateURI) {
		t.Fatalf("resources/list response = %s, want project, readiness, launch packet, and memory candidate resources", got)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":21,"method":"resources/read","params":{"uri":"` + projectURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() project resources/read error = %v", err)
	}
	projectResourceText := readSingleResourceText(t, output.Bytes())
	if !strings.Contains(projectResourceText, `"project"`) || !strings.Contains(projectResourceText, `"operations"`) || !strings.Contains(projectResourceText, `"activity"`) || !strings.Contains(projectResourceText, `"roles"`) || !strings.Contains(projectResourceText, `"assignments"`) || !strings.Contains(projectResourceText, `"memory"`) {
		t.Fatalf("project resources/read text = %s", projectResourceText)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":22,"method":"resources/read","params":{"uri":"` + workItemURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() work item resources/read error = %v", err)
	}
	workItemResourceResponse := readSingleResourceResponse(t, output.Bytes())
	if workItemResourceResponse.URI != workItemURI || workItemResourceResponse.MimeType != "application/json" {
		t.Fatalf("work item resource content = %+v, want JSON work item resource", workItemResourceResponse)
	}
	var workItemFromResource workItemResourcePayload
	if err := json.Unmarshal([]byte(workItemResourceResponse.Text), &workItemFromResource); err != nil {
		t.Fatalf("work item resource text did not unmarshal: %v\n%s", err, workItemResourceResponse.Text)
	}
	if len(workItemFromResource.Artifacts) != 1 || workItemFromResource.Artifacts[0].ID != artifact.ID || len(workItemFromResource.Evidence) != 1 || len(workItemFromResource.Reviews) != 1 || len(workItemFromResource.Handoffs) != 1 {
		t.Fatalf("work item resource = %+v, want generic artifact plus collaboration records", workItemFromResource)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":23,"method":"resources/read","params":{"uri":"` + readinessURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() readiness resources/read error = %v", err)
	}
	readinessResourceResponse := readSingleResourceResponse(t, output.Bytes())
	if readinessResourceResponse.URI != readinessURI || readinessResourceResponse.MimeType != "application/json" {
		t.Fatalf("readiness resource content = %+v, want JSON readiness resource", readinessResourceResponse)
	}
	var readinessFromResource core.WorkItemCloseoutReadiness
	if err := json.Unmarshal([]byte(readinessResourceResponse.Text), &readinessFromResource); err != nil {
		t.Fatalf("readiness resource text did not unmarshal: %v\n%s", err, readinessResourceResponse.Text)
	}
	if !readinessFromResource.Ready || readinessFromResource.WorkItemID != work.ID || readinessFromResource.CompletedAssignments != 1 {
		t.Fatalf("readiness resource = %+v, want ready closeout readiness", readinessFromResource)
	}

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":24,"method":"resources/read","params":{"uri":"` + launchURI + `"}}` + "\n",
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

	input = strings.NewReader(
		`{"jsonrpc":"2.0","id":25,"method":"resources/read","params":{"uri":"` + candidateURI + `"}}` + "\n",
	)
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() memory candidate resources/read error = %v", err)
	}
	candidateResourceResponse := readSingleResourceResponse(t, output.Bytes())
	if candidateResourceResponse.URI != candidateURI || candidateResourceResponse.MimeType != "application/json" {
		t.Fatalf("memory candidate resource content = %+v, want JSON memory candidate resource", candidateResourceResponse)
	}
	var candidateFromResource core.MemoryCandidate
	if err := json.Unmarshal([]byte(candidateResourceResponse.Text), &candidateFromResource); err != nil {
		t.Fatalf("memory candidate resource text did not unmarshal: %v\n%s", err, candidateResourceResponse.Text)
	}
	if candidateFromResource.ID != candidateID || candidateFromResource.Status != core.MemoryCandidatePending || len(candidateFromResource.SourceRefs) != 1 {
		t.Fatalf("memory candidate resource = %+v, want pending candidate with provenance", candidateFromResource)
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
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
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

	var output bytes.Buffer
	input := toolRequest(t, 1, "assignments.get", map[string]any{
		"project_id":    project.ID,
		"assignment_id": assignment.ID,
	})
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get assignment error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Assignment (1):") || !strings.Contains(got, assignment.ID) || !strings.Contains(got, `"structuredContent"`) {
		t.Fatalf("get assignment response = %s", got)
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

func TestMCPTools_HandoffLifecycle(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")
	project, err := service.CreateProject(ctx, core.Project{Name: "Handoff project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	fromRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole(from) error = %v", err)
	}
	toRole, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole(to) error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Handoff flow"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	var output bytes.Buffer
	input := toolRequest(t, 1, "handoffs.create", map[string]any{
		"project_id":      project.ID,
		"work_item_id":    work.ID,
		"from_role_id":    fromRole.ID,
		"to_role_id":      toRole.ID,
		"title":           "Ready for review",
		"body":            "Implementation is ready.",
		"context_refs":    []string{"ctx_1"},
		"trust_label":     "operator_reviewed",
		"provenance_kind": "operator",
	})
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() create handoff error = %v", err)
	}
	if !strings.Contains(output.String(), "Created handoff handoff_") {
		t.Fatalf("create handoff response = %s", output.String())
	}
	handoffs, err := service.ListHandoffs(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListHandoffs() error = %v", err)
	}
	if len(handoffs) != 1 || handoffs[0].Status != core.HandoffStatusOpen {
		t.Fatalf("handoffs = %+v, want one open handoff", handoffs)
	}
	handoffID := handoffs[0].ID

	input = toolRequest(t, 2, "handoffs.list", map[string]any{
		"project_id":   project.ID,
		"work_item_id": work.ID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() list handoffs error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Handoffs (1):") || !strings.Contains(got, handoffID) || !strings.Contains(got, `"structuredContent"`) {
		t.Fatalf("list handoffs response = %s", got)
	}

	input = toolRequest(t, 3, "handoffs.get", map[string]any{
		"project_id":   project.ID,
		"work_item_id": work.ID,
		"handoff_id":   handoffID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() get handoff error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "Handoff (1):") || !strings.Contains(got, "Ready for review") {
		t.Fatalf("get handoff response = %s", got)
	}

	input = toolRequest(t, 4, "handoffs.update", map[string]any{
		"project_id":   project.ID,
		"work_item_id": work.ID,
		"handoff_id":   handoffID,
		"title":        "Accepted review handoff",
		"body":         "Reviewer accepted the handoff.",
		"status":       core.HandoffStatusAccepted,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() update handoff error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated handoff "+handoffID+": Accepted review handoff [accepted]") {
		t.Fatalf("update handoff response = %s", output.String())
	}

	input = toolRequest(t, 5, "handoffs.update_status", map[string]any{
		"project_id":   project.ID,
		"work_item_id": work.ID,
		"handoff_id":   handoffID,
		"status":       core.HandoffStatusDismissed,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() update handoff status error = %v", err)
	}
	if !strings.Contains(output.String(), "Updated handoff "+handoffID+": dismissed") {
		t.Fatalf("update handoff status response = %s", output.String())
	}
	updated, err := service.GetHandoff(ctx, project.ID, work.ID, handoffID)
	if err != nil {
		t.Fatalf("GetHandoff() after status error = %v", err)
	}
	if updated.Status != core.HandoffStatusDismissed || updated.Title != "Accepted review handoff" || len(updated.ContextRefs) != 1 {
		t.Fatalf("updated handoff = %+v, want dismissed with text and refs preserved", updated)
	}

	input = toolRequest(t, 6, "handoffs.delete", map[string]any{
		"project_id":   project.ID,
		"work_item_id": work.ID,
		"handoff_id":   handoffID,
	})
	output.Reset()
	if err := server.Serve(ctx, input, &output); err != nil {
		t.Fatalf("Serve() delete handoff error = %v", err)
	}
	if !strings.Contains(output.String(), "Deleted handoff "+handoffID) {
		t.Fatalf("delete handoff response = %s", output.String())
	}
	if _, err := service.GetHandoff(ctx, project.ID, work.ID, handoffID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetHandoff(deleted) error = %v, want ErrNotFound", err)
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
