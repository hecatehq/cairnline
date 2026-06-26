package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

func RegisterTools(server *mcp.Server, service *core.Service) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: mcp.BoolPtr(true)}

	server.RegisterTool(mcp.Tool{
		Name:        "projects.list",
		Title:       "List projects",
		Description: "List durable project coordination spaces.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Annotations: readOnly,
	}, listProjects(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.create",
		Title:       "Create project",
		Description: "Create a rootless or workspace-backed project coordination space.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"name":{"type":"string","minLength":1},
				"description":{"type":"string"},
				"roots":{"type":"array","items":{"type":"object","properties":{
					"id":{"type":"string"},
					"path":{"type":"string","minLength":1},
					"kind":{"type":"string"},
					"git_remote":{"type":"string"},
					"git_branch":{"type":"string"},
					"active":{"type":"boolean"}
				},"required":["path"]}}
			},
			"required":["name"]
		}`),
	}, createProject(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.update",
		Title:       "Update project",
		Description: "Patch durable project metadata, roots, or context source metadata.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","minLength":1},
				"name":{"type":"string"},
				"description":{"type":"string"},
				"roots":{"type":"array","items":{"type":"object","properties":{
					"id":{"type":"string"},
					"path":{"type":"string","minLength":1},
					"kind":{"type":"string"},
					"git_remote":{"type":"string"},
					"git_branch":{"type":"string"},
					"active":{"type":"boolean"}
				},"required":["path"]}}
			},
			"required":["id"]
		}`),
	}, updateProject(service))

	server.RegisterTool(mcp.Tool{
		Name:        "profiles.list",
		Title:       "List agent profiles",
		Description: "List portable agent behavior and context-policy profiles.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Annotations: readOnly,
	}, listAgentProfiles(service))

	server.RegisterTool(mcp.Tool{
		Name:        "profiles.create",
		Title:       "Create agent profile",
		Description: "Create a portable agent behavior and context-policy profile.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"name":{"type":"string","minLength":1},
				"description":{"type":"string"},
				"instructions":{"type":"string"},
				"context_policy":{"type":"string"},
				"memory_policy":{"type":"string"},
				"source_policy":{"type":"string"},
				"skill_ids":{"type":"array","items":{"type":"string"}}
			},
			"required":["name"]
		}`),
	}, createAgentProfile(service))

	server.RegisterTool(mcp.Tool{
		Name:        "profiles.update",
		Title:       "Update agent profile",
		Description: "Replace a portable agent behavior and context-policy profile.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","minLength":1},
				"name":{"type":"string","minLength":1},
				"description":{"type":"string"},
				"instructions":{"type":"string"},
				"context_policy":{"type":"string"},
				"memory_policy":{"type":"string"},
				"source_policy":{"type":"string"},
				"skill_ids":{"type":"array","items":{"type":"string"}}
			},
			"required":["id","name"]
		}`),
	}, updateAgentProfile(service))

	server.RegisterTool(mcp.Tool{
		Name:        "execution_profiles.list",
		Title:       "List execution profiles",
		Description: "List optional host/runtime-specific execution hints.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Annotations: readOnly,
	}, listExecutionProfiles(service))

	server.RegisterTool(mcp.Tool{
		Name:        "execution_profiles.create",
		Title:       "Create execution profile",
		Description: "Create optional host/runtime-specific execution hints.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"name":{"type":"string","minLength":1},
				"description":{"type":"string"},
				"agent_kind":{"type":"string"},
				"model_hint":{"type":"string"},
				"provider_hint":{"type":"string"},
				"tools_policy":{"type":"string"},
				"writes_policy":{"type":"string"},
				"network_policy":{"type":"string"},
				"approval_policy":{"type":"string"},
				"adapter_options":{"type":"object"}
			},
			"required":["name"]
		}`),
	}, createExecutionProfile(service))

	server.RegisterTool(mcp.Tool{
		Name:        "execution_profiles.update",
		Title:       "Update execution profile",
		Description: "Replace optional host/runtime-specific execution hints.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","minLength":1},
				"name":{"type":"string","minLength":1},
				"description":{"type":"string"},
				"agent_kind":{"type":"string"},
				"model_hint":{"type":"string"},
				"provider_hint":{"type":"string"},
				"tools_policy":{"type":"string"},
				"writes_policy":{"type":"string"},
				"network_policy":{"type":"string"},
				"approval_policy":{"type":"string"},
				"adapter_options":{"type":"object"}
			},
			"required":["id","name"]
		}`),
	}, updateExecutionProfile(service))

	server.RegisterTool(mcp.Tool{
		Name:        "skills.list",
		Title:       "List project skills",
		Description: "List project-scoped skill metadata records.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listProjectSkills(service))

	server.RegisterTool(mcp.Tool{
		Name:        "skills.discover",
		Title:       "Discover project skills",
		Description: "Discover skill metadata from active project roots without reading or injecting skill bodies.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
	}, discoverProjectSkills(service))

	server.RegisterTool(mcp.Tool{
		Name:        "skills.create",
		Title:       "Create project skill metadata",
		Description: "Create a project-scoped skill metadata record. Metadata does not grant tools, writes, network, or approvals.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1},
				"title":{"type":"string"},
				"description":{"type":"string"},
				"path":{"type":"string"},
				"root_id":{"type":"string"},
				"format":{"type":"string","enum":["skill_md"]},
				"enabled":{"type":"boolean"},
				"status":{"type":"string","enum":["available","missing","invalid","conflict"]},
				"trust_label":{"type":"string"},
				"source_refs":{"type":"array","items":{"type":"string"}},
				"warnings":{"type":"array","items":{"type":"string"}}
			},
			"required":["project_id","id"]
		}`),
	}, createProjectSkill(service))

	server.RegisterTool(mcp.Tool{
		Name:        "skills.update",
		Title:       "Update project skill metadata",
		Description: "Replace a project-scoped skill metadata record.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1},
				"title":{"type":"string"},
				"description":{"type":"string"},
				"path":{"type":"string"},
				"root_id":{"type":"string"},
				"format":{"type":"string","enum":["skill_md"]},
				"enabled":{"type":"boolean"},
				"status":{"type":"string","enum":["available","missing","invalid","conflict"]},
				"trust_label":{"type":"string"},
				"source_refs":{"type":"array","items":{"type":"string"}},
				"warnings":{"type":"array","items":{"type":"string"}}
			},
			"required":["project_id","id"]
		}`),
	}, updateProjectSkill(service))

	server.RegisterTool(mcp.Tool{
		Name:        "work_items.list",
		Title:       "List work items",
		Description: "List work items for a project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listWorkItems(service))

	server.RegisterTool(mcp.Tool{
		Name:        "work_items.create",
		Title:       "Create work item",
		Description: "Create a reviewable work item under a project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"title":{"type":"string","minLength":1},
				"brief":{"type":"string"},
				"owner_role_id":{"type":"string"},
				"root_id":{"type":"string"}
			},
			"required":["project_id","title"]
		}`),
	}, createWorkItem(service))

	server.RegisterTool(mcp.Tool{
		Name:        "work_items.update",
		Title:       "Update work item",
		Description: "Patch a reviewable work item under a project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1},
				"title":{"type":"string"},
				"brief":{"type":"string"},
				"status":{"type":"string"},
				"priority":{"type":"string"},
				"owner_role_id":{"type":"string"},
				"reviewer_role_ids":{"type":"array","items":{"type":"string"}},
				"root_id":{"type":"string"}
			},
			"required":["project_id","id"]
		}`),
	}, updateWorkItem(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roles.list",
		Title:       "List roles",
		Description: "List project roles.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listRoles(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roles.create",
		Title:       "Create role",
		Description: "Create a project-native responsibility such as implementer, reviewer, or researcher.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"name":{"type":"string","minLength":1},
				"description":{"type":"string"},
				"instructions":{"type":"string"},
				"default_profile_id":{"type":"string"},
				"default_skill_ids":{"type":"array","items":{"type":"string"}},
				"default_execution_mode":{"type":"string","enum":["manual","mcp_pull","external_adapter","orchestrated"]}
			},
			"required":["project_id","name"]
		}`),
	}, createRole(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roles.update",
		Title:       "Update role",
		Description: "Patch a project-native responsibility.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1},
				"name":{"type":"string"},
				"description":{"type":"string"},
				"instructions":{"type":"string"},
				"default_profile_id":{"type":"string"},
				"default_skill_ids":{"type":"array","items":{"type":"string"}},
				"default_execution_mode":{"type":"string","enum":["manual","mcp_pull","external_adapter","orchestrated"]}
			},
			"required":["project_id","id"]
		}`),
	}, updateRole(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.list",
		Title:       "List assignments",
		Description: "List coordination assignments for a project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listAssignments(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.get",
		Title:       "Get assignment",
		Description: "Get one coordination assignment by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1}
			},
			"required":["project_id","assignment_id"]
		}`),
		Annotations: readOnly,
	}, getAssignment(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.next",
		Title:       "Next compatible assignments",
		Description: "List queued assignments compatible with an MCP-pull agent before claiming.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"agent_kind":{"type":"string"},
				"skill_ids":{"type":"array","items":{"type":"string"}},
				"execution_modes":{"type":"array","items":{"type":"string","enum":["manual","mcp_pull","external_adapter","orchestrated"]}},
				"status":{"type":"string","enum":["queued","claimed","running","awaiting_review","completed","failed","cancelled"]},
				"limit":{"type":"integer","minimum":1}
			},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, nextAssignments(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.create",
		Title:       "Create assignment",
		Description: "Create a queued assignment for a work item and role.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"role_id":{"type":"string","minLength":1},
				"profile_id":{"type":"string"},
				"execution_profile_id":{"type":"string"},
				"execution_mode":{"type":"string","enum":["manual","mcp_pull","external_adapter","orchestrated"]},
				"desired_agent_kind":{"type":"string"},
				"skill_ids":{"type":"array","items":{"type":"string"}}
			},
			"required":["project_id","work_item_id","role_id"]
		}`),
	}, createAssignment(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.claim",
		Title:       "Claim assignment",
		Description: "Atomically claim a queued assignment for an MCP-capable agent or operator.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1},
				"claimed_by":{"type":"string","minLength":1}
			},
			"required":["project_id","assignment_id","claimed_by"]
		}`),
	}, claimAssignment(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.update_status",
		Title:       "Update assignment progress status",
		Description: "Mark a claimed assignment running or awaiting review.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1},
				"status":{"type":"string","enum":["running","awaiting_review"]},
				"execution_ref":{"type":"string"}
			},
			"required":["project_id","assignment_id","status"]
		}`),
	}, updateAssignmentStatus(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.context",
		Title:       "Assignment context",
		Description: "Return the project, work item, role, and assignment metadata for a claimed or queued assignment.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1}
			},
			"required":["project_id","assignment_id"]
		}`),
		Annotations: readOnly,
	}, assignmentContext(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.launch_packet",
		Title:       "Assignment launch packet",
		Description: "Return a structured launch packet for manual, MCP-pull, or orchestrator handoff.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1}
			},
			"required":["project_id","assignment_id"]
		}`),
		Annotations: readOnly,
	}, assignmentLaunchPacket(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.complete",
		Title:       "Complete assignment",
		Description: "Mark an assignment completed, failed, cancelled, or awaiting review.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1},
				"status":{"type":"string","enum":["completed","failed","cancelled","awaiting_review"]},
				"execution_ref":{"type":"string"}
			},
			"required":["project_id","assignment_id"]
		}`),
	}, completeAssignment(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assignments.delete",
		Title:       "Delete assignment",
		Description: "Delete an assignment and assignment-linked reviews.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1}
			},
			"required":["project_id","assignment_id"]
		}`),
	}, deleteAssignment(service))

	server.RegisterTool(mcp.Tool{
		Name:        "evidence.record",
		Title:       "Record evidence",
		Description: "Record proof, output, or an external locator for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"title":{"type":"string","minLength":1},
				"body":{"type":"string"},
				"locator":{"type":"string"},
				"trust_label":{"type":"string"}
			},
			"required":["project_id","work_item_id","title"]
		}`),
	}, recordEvidence(service))

	server.RegisterTool(mcp.Tool{
		Name:        "reviews.record",
		Title:       "Record review",
		Description: "Record a structured review verdict for a work item or assignment.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string"},
				"reviewer_role_id":{"type":"string"},
				"title":{"type":"string"},
				"body":{"type":"string","minLength":1},
				"verdict":{"type":"string","enum":["pass","concerns","blocked"]},
				"risk":{"type":"string","enum":["low","medium","high"]}
			},
			"required":["project_id","work_item_id","body","verdict"]
		}`),
	}, recordReview(service))

	server.RegisterTool(mcp.Tool{
		Name:        "handoffs.create",
		Title:       "Create handoff",
		Description: "Create a structured handoff for another role, agent, or operator.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"from_role_id":{"type":"string"},
				"to_role_id":{"type":"string"},
				"title":{"type":"string","minLength":1},
				"body":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id","title","body"]
		}`),
	}, createHandoff(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_candidates.create",
		Title:       "Create memory candidate",
		Description: "Create a proposed durable memory entry awaiting explicit approval.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"title":{"type":"string","minLength":1},
				"body":{"type":"string","minLength":1},
				"trust_label":{"type":"string"},
				"source_ref":{"type":"string"}
			},
			"required":["project_id","title","body"]
		}`),
	}, createMemoryCandidate(service))
}

func listProjects(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		items, err := service.ListProjects(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No projects yet.")}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Projects (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: %s", item.ID, item.Name)
			if item.Description != "" {
				fmt.Fprintf(&b, " — %s", item.Description)
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String())}, nil
	}
}

type rootArgs struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	GitRemote string `json:"git_remote"`
	GitBranch string `json:"git_branch"`
	Active    *bool  `json:"active"`
}

func toCoreRoots(input []rootArgs) []core.Root {
	roots := make([]core.Root, 0, len(input))
	for _, root := range input {
		active := true
		if root.Active != nil {
			active = *root.Active
		}
		roots = append(roots, core.Root{
			ID:        root.ID,
			Path:      root.Path,
			Kind:      root.Kind,
			GitRemote: root.GitRemote,
			GitBranch: root.GitBranch,
			Active:    active,
		})
	}
	return roots
}

func createProject(service *core.Service) mcp.ToolHandler {
	type args struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		Roots       []rootArgs `json:"roots"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateProject(ctx, core.Project{
			Name:        input.Name,
			Description: input.Description,
			Roots:       toCoreRoots(input.Roots),
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created project %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func updateProject(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID          string      `json:"id"`
		Name        *string     `json:"name"`
		Description *string     `json:"description"`
		Roots       *[]rootArgs `json:"roots"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		projects, err := service.ListProjects(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		var existing core.Project
		for _, project := range projects {
			if project.ID == input.ID {
				existing = project
				break
			}
		}
		if existing.ID == "" {
			return mcp.CallToolResult{}, core.ErrNotFound
		}
		if input.Name != nil {
			existing.Name = *input.Name
		}
		if input.Description != nil {
			existing.Description = *input.Description
		}
		if input.Roots != nil {
			existing.Roots = toCoreRoots(*input.Roots)
		}
		item, err := service.UpdateProject(ctx, existing)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated project %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func listAgentProfiles(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		items, err := service.ListAgentProfiles(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No agent profiles yet.")}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Agent profiles (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: %s", item.ID, item.Name)
			if item.Description != "" {
				fmt.Fprintf(&b, " — %s", item.Description)
			}
			if len(item.SkillIDs) > 0 {
				fmt.Fprintf(&b, " skills=%s", strings.Join(item.SkillIDs, ","))
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String())}, nil
	}
}

func createAgentProfile(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Instructions  string   `json:"instructions"`
		ContextPolicy string   `json:"context_policy"`
		MemoryPolicy  string   `json:"memory_policy"`
		SourcePolicy  string   `json:"source_policy"`
		SkillIDs      []string `json:"skill_ids"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateAgentProfile(ctx, core.AgentProfile{
			ID:            input.ID,
			Name:          input.Name,
			Description:   input.Description,
			Instructions:  input.Instructions,
			ContextPolicy: input.ContextPolicy,
			MemoryPolicy:  input.MemoryPolicy,
			SourcePolicy:  input.SourcePolicy,
			SkillIDs:      input.SkillIDs,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created agent profile %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func updateAgentProfile(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Instructions  string   `json:"instructions"`
		ContextPolicy string   `json:"context_policy"`
		MemoryPolicy  string   `json:"memory_policy"`
		SourcePolicy  string   `json:"source_policy"`
		SkillIDs      []string `json:"skill_ids"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.UpdateAgentProfile(ctx, core.AgentProfile{
			ID:            input.ID,
			Name:          input.Name,
			Description:   input.Description,
			Instructions:  input.Instructions,
			ContextPolicy: input.ContextPolicy,
			MemoryPolicy:  input.MemoryPolicy,
			SourcePolicy:  input.SourcePolicy,
			SkillIDs:      input.SkillIDs,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated agent profile %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func listExecutionProfiles(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		items, err := service.ListExecutionProfiles(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No execution profiles yet.")}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Execution profiles (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: %s", item.ID, item.Name)
			if item.AgentKind != "" {
				fmt.Fprintf(&b, " agent=%s", item.AgentKind)
			}
			if item.ProviderHint != "" || item.ModelHint != "" {
				fmt.Fprintf(&b, " model=%s/%s", item.ProviderHint, item.ModelHint)
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String())}, nil
	}
}

func createExecutionProfile(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID             string         `json:"id"`
		Name           string         `json:"name"`
		Description    string         `json:"description"`
		AgentKind      string         `json:"agent_kind"`
		ModelHint      string         `json:"model_hint"`
		ProviderHint   string         `json:"provider_hint"`
		ToolsPolicy    string         `json:"tools_policy"`
		WritesPolicy   string         `json:"writes_policy"`
		NetworkPolicy  string         `json:"network_policy"`
		ApprovalPolicy string         `json:"approval_policy"`
		AdapterOptions map[string]any `json:"adapter_options"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateExecutionProfile(ctx, core.ExecutionProfile{
			ID:             input.ID,
			Name:           input.Name,
			Description:    input.Description,
			AgentKind:      input.AgentKind,
			ModelHint:      input.ModelHint,
			ProviderHint:   input.ProviderHint,
			ToolsPolicy:    input.ToolsPolicy,
			WritesPolicy:   input.WritesPolicy,
			NetworkPolicy:  input.NetworkPolicy,
			ApprovalPolicy: input.ApprovalPolicy,
			AdapterOptions: input.AdapterOptions,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created execution profile %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func updateExecutionProfile(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID             string         `json:"id"`
		Name           string         `json:"name"`
		Description    string         `json:"description"`
		AgentKind      string         `json:"agent_kind"`
		ModelHint      string         `json:"model_hint"`
		ProviderHint   string         `json:"provider_hint"`
		ToolsPolicy    string         `json:"tools_policy"`
		WritesPolicy   string         `json:"writes_policy"`
		NetworkPolicy  string         `json:"network_policy"`
		ApprovalPolicy string         `json:"approval_policy"`
		AdapterOptions map[string]any `json:"adapter_options"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.UpdateExecutionProfile(ctx, core.ExecutionProfile{
			ID:             input.ID,
			Name:           input.Name,
			Description:    input.Description,
			AgentKind:      input.AgentKind,
			ModelHint:      input.ModelHint,
			ProviderHint:   input.ProviderHint,
			ToolsPolicy:    input.ToolsPolicy,
			WritesPolicy:   input.WritesPolicy,
			NetworkPolicy:  input.NetworkPolicy,
			ApprovalPolicy: input.ApprovalPolicy,
			AdapterOptions: input.AdapterOptions,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated execution profile %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func listProjectSkills(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListProjectSkills(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatProjectSkills("Project skills", items)),
			StructuredContent: items,
		}, nil
	}
}

func discoverProjectSkills(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.DiscoverProjectSkills(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatProjectSkills("Discovered project skills", items)),
			StructuredContent: items,
		}, nil
	}
}

func createProjectSkill(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		input, err := decodeCreateProjectSkillArgs(raw)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		item, err := service.CreateProjectSkill(ctx, input)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Created project skill %s: %s", item.ID, item.Title)),
			StructuredContent: item,
		}, nil
	}
}

func updateProjectSkill(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		input, err := decodeUpdateProjectSkillArgs(ctx, service, raw)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		item, err := service.UpdateProjectSkill(ctx, input)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Updated project skill %s: %s", item.ID, item.Title)),
			StructuredContent: item,
		}, nil
	}
}

func listWorkItems(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListWorkItems(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No work items yet.")}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Work items (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: [%s] %s", item.ID, item.Status, item.Title)
			if item.Brief != "" {
				fmt.Fprintf(&b, " — %s", item.Brief)
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String())}, nil
	}
}

func createWorkItem(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID   string `json:"project_id"`
		Title       string `json:"title"`
		Brief       string `json:"brief"`
		OwnerRoleID string `json:"owner_role_id"`
		RootID      string `json:"root_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateWorkItem(ctx, core.WorkItem{
			ProjectID:   input.ProjectID,
			Title:       input.Title,
			Brief:       input.Brief,
			OwnerRoleID: input.OwnerRoleID,
			RootID:      input.RootID,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created work item %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func updateWorkItem(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID       string    `json:"project_id"`
		ID              string    `json:"id"`
		Title           *string   `json:"title"`
		Brief           *string   `json:"brief"`
		Status          *string   `json:"status"`
		Priority        *string   `json:"priority"`
		OwnerRoleID     *string   `json:"owner_role_id"`
		ReviewerRoleIDs *[]string `json:"reviewer_role_ids"`
		RootID          *string   `json:"root_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		workItems, err := service.ListWorkItems(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		var existing core.WorkItem
		for _, item := range workItems {
			if item.ID == input.ID {
				existing = item
				break
			}
		}
		if existing.ID == "" {
			return mcp.CallToolResult{}, core.ErrNotFound
		}
		if input.Title != nil {
			existing.Title = *input.Title
		}
		if input.Brief != nil {
			existing.Brief = *input.Brief
		}
		if input.Status != nil {
			existing.Status = *input.Status
		}
		if input.Priority != nil {
			existing.Priority = *input.Priority
		}
		if input.OwnerRoleID != nil {
			existing.OwnerRoleID = *input.OwnerRoleID
		}
		if input.ReviewerRoleIDs != nil {
			existing.ReviewerRoleIDs = *input.ReviewerRoleIDs
		}
		if input.RootID != nil {
			existing.RootID = *input.RootID
		}
		item, err := service.UpdateWorkItem(ctx, existing)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated work item %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func listRoles(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListRoles(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No roles yet.")}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Roles (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: %s", item.ID, item.Name)
			if item.Description != "" {
				fmt.Fprintf(&b, " — %s", item.Description)
			}
			if item.DefaultExecutionMode != "" {
				fmt.Fprintf(&b, " (%s)", item.DefaultExecutionMode)
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String())}, nil
	}
}

func createRole(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID            string   `json:"project_id"`
		Name                 string   `json:"name"`
		Description          string   `json:"description"`
		Instructions         string   `json:"instructions"`
		DefaultProfileID     string   `json:"default_profile_id"`
		DefaultSkillIDs      []string `json:"default_skill_ids"`
		DefaultExecutionMode string   `json:"default_execution_mode"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateRole(ctx, core.Role{
			ProjectID:            input.ProjectID,
			Name:                 input.Name,
			Description:          input.Description,
			Instructions:         input.Instructions,
			DefaultProfileID:     input.DefaultProfileID,
			DefaultSkillIDs:      input.DefaultSkillIDs,
			DefaultExecutionMode: input.DefaultExecutionMode,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created role %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func updateRole(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID            string    `json:"project_id"`
		ID                   string    `json:"id"`
		Name                 *string   `json:"name"`
		Description          *string   `json:"description"`
		Instructions         *string   `json:"instructions"`
		DefaultProfileID     *string   `json:"default_profile_id"`
		DefaultSkillIDs      *[]string `json:"default_skill_ids"`
		DefaultExecutionMode *string   `json:"default_execution_mode"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		roles, err := service.ListRoles(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		var existing core.Role
		for _, role := range roles {
			if role.ID == input.ID {
				existing = role
				break
			}
		}
		if existing.ID == "" {
			return mcp.CallToolResult{}, core.ErrNotFound
		}
		if input.Name != nil {
			existing.Name = *input.Name
		}
		if input.Description != nil {
			existing.Description = *input.Description
		}
		if input.Instructions != nil {
			existing.Instructions = *input.Instructions
		}
		if input.DefaultProfileID != nil {
			existing.DefaultProfileID = *input.DefaultProfileID
		}
		if input.DefaultSkillIDs != nil {
			existing.DefaultSkillIDs = *input.DefaultSkillIDs
		}
		if input.DefaultExecutionMode != nil {
			existing.DefaultExecutionMode = *input.DefaultExecutionMode
		}
		item, err := service.UpdateRole(ctx, existing)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated role %s: %s", item.ID, item.Name)),
		}, nil
	}
}

func listAssignments(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListAssignments(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No assignments yet.")}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Assignments (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: [%s] work=%s role=%s mode=%s", item.ID, item.Status, item.WorkItemID, item.RoleID, item.ExecutionMode)
			if item.RootID != "" {
				fmt.Fprintf(&b, " root=%s", item.RootID)
			}
			if item.ClaimedBy != "" {
				fmt.Fprintf(&b, " claimed_by=%s", item.ClaimedBy)
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String())}, nil
	}
}

func getAssignment(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetAssignment(ctx, input.ProjectID, input.AssignmentID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatAssignments("Assignment", []core.Assignment{item})),
			StructuredContent: item,
		}, nil
	}
}

func nextAssignments(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID      string    `json:"project_id"`
		AgentKind      string    `json:"agent_kind"`
		SkillIDs       *[]string `json:"skill_ids"`
		ExecutionModes []string  `json:"execution_modes"`
		Status         string    `json:"status"`
		Limit          int       `json:"limit"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		var skillIDs []string
		filterSkills := input.SkillIDs != nil
		if input.SkillIDs != nil {
			skillIDs = *input.SkillIDs
		}
		items, err := service.ListCompatibleAssignments(ctx, input.ProjectID, core.AssignmentCompatibilityFilter{
			Status:         input.Status,
			ExecutionModes: input.ExecutionModes,
			AgentKind:      input.AgentKind,
			SkillIDs:       skillIDs,
			FilterSkills:   filterSkills,
			Limit:          input.Limit,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatAssignments("Compatible assignments", items)),
			StructuredContent: items,
		}, nil
	}
}

func createAssignment(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID          string   `json:"project_id"`
		WorkItemID         string   `json:"work_item_id"`
		RoleID             string   `json:"role_id"`
		RootID             string   `json:"root_id"`
		ProfileID          string   `json:"profile_id"`
		ExecutionProfileID string   `json:"execution_profile_id"`
		ExecutionMode      string   `json:"execution_mode"`
		DesiredAgentKind   string   `json:"desired_agent_kind"`
		SkillIDs           []string `json:"skill_ids"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateAssignment(ctx, core.Assignment{
			ProjectID:          input.ProjectID,
			WorkItemID:         input.WorkItemID,
			RoleID:             input.RoleID,
			RootID:             input.RootID,
			ProfileID:          input.ProfileID,
			ExecutionProfileID: input.ExecutionProfileID,
			ExecutionMode:      input.ExecutionMode,
			DesiredAgent: core.DesiredAgent{
				Kind:     input.DesiredAgentKind,
				SkillIDs: input.SkillIDs,
			},
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		detail := fmt.Sprintf("Created assignment %s: work=%s role=%s mode=%s", item.ID, item.WorkItemID, item.RoleID, item.ExecutionMode)
		if item.RootID != "" {
			detail += " root=" + item.RootID
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(detail),
		}, nil
	}
}

func claimAssignment(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
		ClaimedBy    string `json:"claimed_by"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.ClaimAssignment(ctx, input.ProjectID, input.AssignmentID, input.ClaimedBy)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Claimed assignment %s by %s", item.ID, item.ClaimedBy)),
		}, nil
	}
}

func updateAssignmentStatus(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
		Status       string `json:"status"`
		ExecutionRef string `json:"execution_ref"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.UpdateAssignmentStatus(ctx, input.ProjectID, input.AssignmentID, input.Status, input.ExecutionRef)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated assignment %s: %s", item.ID, item.Status)),
		}, nil
	}
}

func assignmentContext(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.AssignmentContext(ctx, input.ProjectID, input.AssignmentID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{Content: mcp.TextContent(formatAssignmentContext(item))}, nil
	}
}

func assignmentLaunchPacket(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		packet, err := service.AssignmentLaunchPacket(ctx, input.ProjectID, input.AssignmentID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatLaunchPacketSummary(packet)),
			StructuredContent: packet,
		}, nil
	}
}

func completeAssignment(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
		Status       string `json:"status"`
		ExecutionRef string `json:"execution_ref"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CompleteAssignment(ctx, input.ProjectID, input.AssignmentID, input.Status, input.ExecutionRef)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated assignment %s: %s", item.ID, item.Status)),
		}, nil
	}
}

func deleteAssignment(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		AssignmentID string `json:"assignment_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteAssignment(ctx, input.ProjectID, input.AssignmentID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted assignment %s", input.AssignmentID)),
		}, nil
	}
}

func recordEvidence(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		Locator    string `json:"locator"`
		TrustLabel string `json:"trust_label"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateEvidence(ctx, core.Evidence{
			ProjectID:  input.ProjectID,
			WorkItemID: input.WorkItemID,
			Title:      input.Title,
			Body:       input.Body,
			Locator:    input.Locator,
			TrustLabel: input.TrustLabel,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Recorded evidence %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func recordReview(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID      string `json:"project_id"`
		WorkItemID     string `json:"work_item_id"`
		AssignmentID   string `json:"assignment_id"`
		ReviewerRoleID string `json:"reviewer_role_id"`
		Title          string `json:"title"`
		Body           string `json:"body"`
		Verdict        string `json:"verdict"`
		Risk           string `json:"risk"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateReview(ctx, core.Review{
			ProjectID:      input.ProjectID,
			WorkItemID:     input.WorkItemID,
			AssignmentID:   input.AssignmentID,
			ReviewerRoleID: input.ReviewerRoleID,
			Title:          input.Title,
			Body:           input.Body,
			Verdict:        input.Verdict,
			Risk:           input.Risk,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Recorded review %s: verdict=%s risk=%s", item.ID, item.Verdict, firstNonEmpty(item.Risk, "unset"))),
		}, nil
	}
}

func createHandoff(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		FromRoleID string `json:"from_role_id"`
		ToRoleID   string `json:"to_role_id"`
		Title      string `json:"title"`
		Body       string `json:"body"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateHandoff(ctx, core.Handoff{
			ProjectID:  input.ProjectID,
			WorkItemID: input.WorkItemID,
			FromRoleID: input.FromRoleID,
			ToRoleID:   input.ToRoleID,
			Title:      input.Title,
			Body:       input.Body,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created handoff %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func createMemoryCandidate(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		TrustLabel string `json:"trust_label"`
		SourceRef  string `json:"source_ref"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
			ProjectID:  input.ProjectID,
			Title:      input.Title,
			Body:       input.Body,
			TrustLabel: input.TrustLabel,
			SourceRef:  input.SourceRef,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created memory candidate %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func formatAssignmentContext(item core.AssignmentContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Assignment context %s\n", item.ID)
	fmt.Fprintf(&b, "Project: %s (%s)\n", item.Project.Name, item.Project.ID)
	fmt.Fprintf(&b, "Work item: %s (%s)\n", item.WorkItem.Title, item.WorkItem.ID)
	if item.WorkItem.Brief != "" {
		fmt.Fprintf(&b, "Brief: %s\n", item.WorkItem.Brief)
	}
	if item.Role != nil {
		fmt.Fprintf(&b, "Role: %s (%s)\n", item.Role.Name, item.Role.ID)
		if item.Role.Instructions != "" {
			fmt.Fprintf(&b, "Role instructions: %s\n", item.Role.Instructions)
		}
	} else {
		fmt.Fprintf(&b, "Role: %s\n", item.Assignment.RoleID)
	}
	fmt.Fprintf(&b, "Assignment: %s [%s]\n", item.Assignment.ID, item.Assignment.Status)
	fmt.Fprintf(&b, "Execution mode: %s\n", item.Assignment.ExecutionMode)
	if item.Assignment.DesiredAgent.Kind != "" {
		fmt.Fprintf(&b, "Desired agent: %s\n", item.Assignment.DesiredAgent.Kind)
	}
	if len(item.Assignment.DesiredAgent.SkillIDs) > 0 {
		fmt.Fprintf(&b, "Skill IDs: %s\n", strings.Join(item.Assignment.DesiredAgent.SkillIDs, ", "))
	}
	for _, warning := range item.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", warning)
	}
	return b.String()
}

func formatLaunchPacketSummary(packet core.AssignmentLaunchPacket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Launch packet %s\n", packet.ID)
	fmt.Fprintf(&b, "Project: %s (%s)\n", packet.Project.Name, packet.Project.ID)
	fmt.Fprintf(&b, "Work item: %s (%s)\n", packet.WorkItem.Title, packet.WorkItem.ID)
	if packet.Role != nil {
		fmt.Fprintf(&b, "Role: %s (%s)\n", packet.Role.Name, packet.Role.ID)
	}
	fmt.Fprintf(&b, "Assignment: %s [%s] mode=%s\n", packet.Assignment.ID, packet.Assignment.Status, packet.Assignment.ExecutionMode)
	fmt.Fprintf(&b, "Skills: %d; evidence: %d; reviews: %d; handoffs: %d; memory candidates: %d\n", len(packet.Skills), len(packet.Evidence), len(packet.Reviews), len(packet.Handoffs), len(packet.MemoryCandidates))
	for _, warning := range packet.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", warning)
	}
	return b.String()
}

type projectSkillArgs struct {
	ProjectID   string    `json:"project_id"`
	ID          string    `json:"id"`
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Path        *string   `json:"path"`
	RootID      *string   `json:"root_id"`
	Format      *string   `json:"format"`
	Enabled     *bool     `json:"enabled"`
	Status      *string   `json:"status"`
	TrustLabel  *string   `json:"trust_label"`
	SourceRefs  *[]string `json:"source_refs"`
	Warnings    *[]string `json:"warnings"`
}

func decodeCreateProjectSkillArgs(raw json.RawMessage) (core.ProjectSkill, error) {
	var input projectSkillArgs
	if err := json.Unmarshal(raw, &input); err != nil {
		return core.ProjectSkill{}, fmt.Errorf("invalid arguments: %w", err)
	}
	enabled := false
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return core.ProjectSkill{
		ProjectID:   input.ProjectID,
		ID:          input.ID,
		Title:       stringValue(input.Title),
		Description: stringValue(input.Description),
		Path:        stringValue(input.Path),
		RootID:      stringValue(input.RootID),
		Format:      stringValue(input.Format),
		Enabled:     enabled,
		Status:      stringValue(input.Status),
		TrustLabel:  stringValue(input.TrustLabel),
		SourceRefs:  stringSliceValue(input.SourceRefs),
		Warnings:    stringSliceValue(input.Warnings),
	}, nil
}

func decodeUpdateProjectSkillArgs(ctx context.Context, service *core.Service, raw json.RawMessage) (core.ProjectSkill, error) {
	var input projectSkillArgs
	if err := json.Unmarshal(raw, &input); err != nil {
		return core.ProjectSkill{}, fmt.Errorf("invalid arguments: %w", err)
	}
	existing, err := service.GetProjectSkill(ctx, input.ProjectID, input.ID)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	if input.Title != nil {
		existing.Title = *input.Title
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.Path != nil {
		existing.Path = *input.Path
	}
	if input.RootID != nil {
		existing.RootID = *input.RootID
	}
	if input.Format != nil {
		existing.Format = *input.Format
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.Status != nil {
		existing.Status = *input.Status
	}
	if input.TrustLabel != nil {
		existing.TrustLabel = *input.TrustLabel
	}
	if input.SourceRefs != nil {
		existing.SourceRefs = *input.SourceRefs
	}
	if input.Warnings != nil {
		existing.Warnings = *input.Warnings
	}
	return existing, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringSliceValue(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

func formatProjectSkills(title string, items []core.ProjectSkill) string {
	if len(items) == 0 {
		return "No project skills yet."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: %s [%s]", item.ID, item.Title, item.Status)
		if !item.Enabled {
			b.WriteString(" disabled")
		}
		if item.Path != "" {
			fmt.Fprintf(&b, " path=%s", item.Path)
		}
		if len(item.Warnings) > 0 {
			fmt.Fprintf(&b, " warnings=%s", strings.Join(item.Warnings, "; "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatAssignments(title string, items []core.Assignment) string {
	if len(items) == 0 {
		return "No compatible assignments."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: [%s] work=%s role=%s mode=%s", item.ID, item.Status, item.WorkItemID, item.RoleID, item.ExecutionMode)
		if item.RootID != "" {
			fmt.Fprintf(&b, " root=%s", item.RootID)
		}
		if item.DesiredAgent.Kind != "" {
			fmt.Fprintf(&b, " desired=%s", item.DesiredAgent.Kind)
		}
		if len(item.DesiredAgent.SkillIDs) > 0 {
			fmt.Fprintf(&b, " skills=%s", strings.Join(item.DesiredAgent.SkillIDs, ","))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
