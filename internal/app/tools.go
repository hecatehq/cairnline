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
		Name:        "projects.get",
		Title:       "Get project",
		Description: "Return one durable project coordination space.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string","minLength":1}},
			"required":["id"]
		}`),
		Annotations: readOnly,
	}, getProject(service))

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
				},"required":["path"]}},
				"default_root_id":{"type":"string"},
				"default_profile_id":{"type":"string"},
				"default_execution_profile_id":{"type":"string"},
				"context_sources":{"type":"array","items":{"type":"object","properties":{
					"id":{"type":"string"},
					"kind":{"type":"string"},
					"title":{"type":"string"},
					"locator":{"type":"string"},
					"enabled":{"type":"boolean"},
					"format":{"type":"string"},
					"scope":{"type":"string"},
					"trust_label":{"type":"string"},
					"source_category":{"type":"string"},
					"metadata":{"type":"object","additionalProperties":{"type":"string"}}
				}}}
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
				},"required":["path"]}},
				"default_root_id":{"type":"string"},
				"default_profile_id":{"type":"string"},
				"default_execution_profile_id":{"type":"string"},
				"context_sources":{"type":"array","items":{"type":"object","properties":{
					"id":{"type":"string"},
					"kind":{"type":"string"},
					"title":{"type":"string"},
					"locator":{"type":"string"},
					"enabled":{"type":"boolean"},
					"format":{"type":"string"},
					"scope":{"type":"string"},
					"trust_label":{"type":"string"},
					"source_category":{"type":"string"},
					"metadata":{"type":"object","additionalProperties":{"type":"string"}}
				}}}
			},
			"required":["id"]
		}`),
	}, updateProject(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.delete",
		Title:       "Delete project",
		Description: "Delete a project and its project-scoped coordination records. Global profiles and execution profiles are not deleted.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string","minLength":1}},
			"required":["id"]
		}`),
	}, deleteProject(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roots.list",
		Title:       "List project roots",
		Description: "List project root metadata. Cairnline does not create, delete, or inspect local directories through this tool.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listRoots(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roots.create",
		Title:       "Create project root",
		Description: "Attach project root metadata without creating folders, worktrees, or Git state.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string"},
				"path":{"type":"string","minLength":1},
				"kind":{"type":"string"},
				"git_remote":{"type":"string"},
				"git_branch":{"type":"string"},
				"active":{"type":"boolean"}
			},
			"required":["project_id","path"]
		}`),
	}, createRoot(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roots.update",
		Title:       "Update project root",
		Description: "Patch project root metadata without touching the local filesystem.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"root_id":{"type":"string","minLength":1},
				"path":{"type":"string"},
				"kind":{"type":"string"},
				"git_remote":{"type":"string"},
				"git_branch":{"type":"string"},
				"active":{"type":"boolean"}
			},
			"required":["project_id","root_id"]
		}`),
	}, updateRoot(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roots.delete",
		Title:       "Delete project root",
		Description: "Remove one project root metadata record without deleting local files or Git worktrees.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"root_id":{"type":"string","minLength":1}
			},
			"required":["project_id","root_id"]
		}`),
	}, deleteRoot(service))

	server.RegisterTool(mcp.Tool{
		Name:        "context_sources.list",
		Title:       "List context sources",
		Description: "List metadata-only project context sources such as guidance files, notes, URLs, or operator-provided references.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listContextSources(service))

	server.RegisterTool(mcp.Tool{
		Name:        "context_sources.create",
		Title:       "Create context source",
		Description: "Create a metadata-only project context source. Cairnline stores the locator but does not fetch or inject source content.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string"},
				"kind":{"type":"string"},
				"title":{"type":"string"},
				"locator":{"type":"string","minLength":1},
				"enabled":{"type":"boolean"},
				"format":{"type":"string"},
				"scope":{"type":"string"},
				"trust_label":{"type":"string"},
				"source_category":{"type":"string"},
				"metadata":{"type":"object","additionalProperties":{"type":"string"}}
			},
			"required":["project_id","locator"]
		}`),
	}, createContextSource(service))

	server.RegisterTool(mcp.Tool{
		Name:        "context_sources.update",
		Title:       "Update context source",
		Description: "Patch a metadata-only project context source without replacing the whole project record.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"source_id":{"type":"string","minLength":1},
				"kind":{"type":"string"},
				"title":{"type":"string"},
				"locator":{"type":"string"},
				"enabled":{"type":"boolean"},
				"format":{"type":"string"},
				"scope":{"type":"string"},
				"trust_label":{"type":"string"},
				"source_category":{"type":"string"},
				"metadata":{"type":"object","additionalProperties":{"type":"string"}}
			},
			"required":["project_id","source_id"]
		}`),
	}, updateContextSource(service))

	server.RegisterTool(mcp.Tool{
		Name:        "context_sources.delete",
		Title:       "Delete context source",
		Description: "Delete one project context-source metadata record without touching local files or external URLs.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"source_id":{"type":"string","minLength":1}
			},
			"required":["project_id","source_id"]
		}`),
	}, deleteContextSource(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.operations_brief",
		Title:       "Project operations brief",
		Description: "Return a read-only project operations summary for operator attention and next-action routing.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, projectOperationsBrief(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.setup_readiness",
		Title:       "Project setup readiness",
		Description: "Return a read-only onboarding checklist for a project coordination space.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, projectSetupReadiness(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.health",
		Title:       "Project health",
		Description: "Return read-only project attention, setup, context, handoff, review, and assignment signals.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, projectHealth(service))

	server.RegisterTool(mcp.Tool{
		Name:        "projects.activity",
		Title:       "Project activity",
		Description: "Return a read-only project activity projection grouped by assignment state.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string","minLength":1}},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, projectActivity(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assistant.propose",
		Title:       "Create assistant proposal",
		Description: "Persist a typed proposal record for operator-confirmed apply. This mutates the proposal ledger only, not project coordination records.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"project_id":{"type":"string"},
				"title":{"type":"string","minLength":1},
				"summary":{"type":"string"},
				"warnings":{"type":"array","items":{"type":"string"}},
				"source":{"type":"string"},
				"actions":{"type":"array","items":{"type":"object"},"minItems":1}
			},
			"required":["title","actions"]
		}`),
	}, assistantPropose(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assistant.proposals.list",
		Title:       "List assistant proposals",
		Description: "List durable assistant proposal records, optionally scoped to a project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"project_id":{"type":"string"}}
		}`),
		Annotations: readOnly,
	}, listAssistantProposals(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assistant.proposals.get",
		Title:       "Get assistant proposal",
		Description: "Fetch one durable assistant proposal record.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string","minLength":1}},
			"required":["id"]
		}`),
		Annotations: readOnly,
	}, getAssistantProposal(service))

	server.RegisterTool(mcp.Tool{
		Name:        "assistant.apply",
		Title:       "Apply assistant proposal",
		Description: "Apply a confirmed typed project proposal record. Assignments are coordination records only and are not auto-dispatched.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"proposal_id":{"type":"string"},
				"proposal":{"type":"object"},
				"confirm":{"type":"boolean"}
			}
		}`),
	}, assistantApply(service))

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
		Name:        "profiles.delete",
		Title:       "Delete agent profile",
		Description: "Delete a portable agent behavior and context-policy profile.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string","minLength":1}},
			"required":["id"]
		}`),
	}, deleteAgentProfile(service))

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
		Name:        "execution_profiles.delete",
		Title:       "Delete execution profile",
		Description: "Delete optional host/runtime-specific execution hints.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string","minLength":1}},
			"required":["id"]
		}`),
	}, deleteExecutionProfile(service))

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
				"suggested_tools":{"type":"array","items":{"type":"string"}},
				"required_permissions":{
					"type":"object",
					"properties":{
						"tools":{"type":"boolean"},
						"writes":{"type":"boolean"},
						"network":{"type":"boolean"}
					}
				},
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
				"suggested_tools":{"type":"array","items":{"type":"string"}},
				"required_permissions":{
					"type":"object",
					"properties":{
						"tools":{"type":"boolean"},
						"writes":{"type":"boolean"},
						"network":{"type":"boolean"}
					}
				},
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
		Name:        "work_items.get",
		Title:       "Get work item",
		Description: "Return one work item for a project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1}
			},
			"required":["project_id","id"]
		}`),
		Annotations: readOnly,
	}, getWorkItem(service))

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
		Name:        "work_items.delete",
		Title:       "Delete work item",
		Description: "Delete a work item and its assignments, evidence, reviews, and handoffs.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1}
			},
			"required":["project_id","id"]
		}`),
	}, deleteWorkItem(service))

	server.RegisterTool(mcp.Tool{
		Name:        "work_items.closeout_readiness",
		Title:       "Work item closeout readiness",
		Description: "Return the read-only closeout readiness summary for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id"]
		}`),
		Annotations: readOnly,
	}, workItemCloseoutReadiness(service))

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
				"default_execution_profile_id":{"type":"string"},
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
				"default_execution_profile_id":{"type":"string"},
				"default_skill_ids":{"type":"array","items":{"type":"string"}},
				"default_execution_mode":{"type":"string","enum":["manual","mcp_pull","external_adapter","orchestrated"]}
			},
			"required":["project_id","id"]
		}`),
	}, updateRole(service))

	server.RegisterTool(mcp.Tool{
		Name:        "roles.delete",
		Title:       "Delete role",
		Description: "Delete an unreferenced project-native responsibility.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"id":{"type":"string","minLength":1}
			},
			"required":["project_id","id"]
		}`),
	}, deleteRole(service))

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
				"root_id":{"type":"string"},
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
		Name:        "assignments.update",
		Title:       "Update assignment metadata",
		Description: "Update assignment coordination metadata such as work item, role, root, profiles, execution mode, desired agent, status, and context refs.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"role_id":{"type":"string","minLength":1},
				"root_id":{"type":"string"},
				"profile_id":{"type":"string"},
				"execution_profile_id":{"type":"string"},
				"execution_mode":{"type":"string","enum":["manual","mcp_pull","external_adapter","orchestrated"]},
				"desired_agent_kind":{"type":"string"},
				"skill_ids":{"type":"array","items":{"type":"string"}},
				"status":{"type":"string","enum":["queued","claimed","running","awaiting_review","completed","failed","cancelled"]},
				"execution_ref":{"type":"string"},
				"context_snapshot_id":{"type":"string"}
			},
			"required":["project_id","assignment_id","work_item_id","role_id"]
		}`),
	}, updateAssignment(service))

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
		Name:        "assignments.release",
		Title:       "Release assignment claim",
		Description: "Release a claimed assignment back to queued state before execution starts.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string","minLength":1},
				"claimed_by":{"type":"string","minLength":1}
			},
			"required":["project_id","assignment_id","claimed_by"]
		}`),
	}, releaseAssignment(service))

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
		Name:        "artifacts.list",
		Title:       "List collaboration artifacts",
		Description: "List generic collaboration artifacts recorded for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id"]
		}`),
		Annotations: readOnly,
	}, listArtifacts(service))

	server.RegisterTool(mcp.Tool{
		Name:        "artifacts.get",
		Title:       "Get collaboration artifact",
		Description: "Get one generic collaboration artifact by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"artifact_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id","artifact_id"]
		}`),
		Annotations: readOnly,
	}, getArtifact(service))

	server.RegisterTool(mcp.Tool{
		Name:        "artifacts.create",
		Title:       "Create collaboration artifact",
		Description: "Record a generic collaboration artifact such as a brief, decision note, or handoff summary.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string"},
				"kind":{"type":"string","minLength":1},
				"title":{"type":"string"},
				"body":{"type":"string","minLength":1},
				"author_role_id":{"type":"string"},
				"provenance_kind":{"type":"string"},
				"trust_label":{"type":"string"}
			},
			"required":["project_id","work_item_id","kind","body"]
		}`),
	}, createArtifact(service))

	server.RegisterTool(mcp.Tool{
		Name:        "evidence.list",
		Title:       "List evidence",
		Description: "List proof, output, or external locators recorded for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id"]
		}`),
		Annotations: readOnly,
	}, listEvidence(service))

	server.RegisterTool(mcp.Tool{
		Name:        "evidence.get",
		Title:       "Get evidence",
		Description: "Get one evidence record by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"evidence_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id","evidence_id"]
		}`),
		Annotations: readOnly,
	}, getEvidence(service))

	server.RegisterTool(mcp.Tool{
		Name:        "evidence.record",
		Title:       "Record evidence",
		Description: "Record proof, output, or an external locator for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"assignment_id":{"type":"string"},
				"title":{"type":"string","minLength":1},
				"body":{"type":"string"},
				"locator":{"type":"string"},
				"source_kind":{"type":"string"},
				"external_id":{"type":"string"},
				"provider":{"type":"string"},
				"trust_label":{"type":"string"}
			},
			"required":["project_id","work_item_id","title"]
		}`),
	}, recordEvidence(service))

	server.RegisterTool(mcp.Tool{
		Name:        "reviews.list",
		Title:       "List reviews",
		Description: "List structured review records for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id"]
		}`),
		Annotations: readOnly,
	}, listReviews(service))

	server.RegisterTool(mcp.Tool{
		Name:        "reviews.get",
		Title:       "Get review",
		Description: "Get one structured review record by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"review_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id","review_id"]
		}`),
		Annotations: readOnly,
	}, getReview(service))

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
				"verdict":{"type":"string","enum":["approved","changes_requested","blocked","risk"]},
				"risk":{"type":"string","enum":["low","medium","high","unknown"]}
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
				"source_assignment_id":{"type":"string"},
				"source_run_id":{"type":"string"},
				"source_chat_session_id":{"type":"string"},
				"source_message_id":{"type":"string"},
				"from_role_id":{"type":"string"},
				"to_role_id":{"type":"string"},
				"target_assignment_id":{"type":"string"},
				"target_work_item_id":{"type":"string"},
				"title":{"type":"string","minLength":1},
				"body":{"type":"string","minLength":1},
				"recommended_next_action":{"type":"string"},
				"linked_artifact_ids":{"type":"array","items":{"type":"string"}},
				"linked_memory_ids":{"type":"array","items":{"type":"string"}},
				"context_refs":{"type":"array","items":{"type":"string"}},
				"status":{"type":"string","enum":["open","accepted","superseded","dismissed"]},
				"provenance_kind":{"type":"string"},
				"trust_label":{"type":"string"}
			},
			"required":["project_id","work_item_id","title","body"]
		}`),
	}, createHandoff(service))

	server.RegisterTool(mcp.Tool{
		Name:        "handoffs.list",
		Title:       "List handoffs",
		Description: "List structured handoffs for a work item.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id"]
		}`),
		Annotations: readOnly,
	}, listHandoffs(service))

	server.RegisterTool(mcp.Tool{
		Name:        "handoffs.get",
		Title:       "Get handoff",
		Description: "Get one structured handoff by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"handoff_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id","handoff_id"]
		}`),
		Annotations: readOnly,
	}, getHandoff(service))

	server.RegisterTool(mcp.Tool{
		Name:        "handoffs.update",
		Title:       "Update handoff",
		Description: "Patch handoff text, roles, source/target refs, or status.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"handoff_id":{"type":"string","minLength":1},
				"source_assignment_id":{"type":"string"},
				"source_run_id":{"type":"string"},
				"source_chat_session_id":{"type":"string"},
				"source_message_id":{"type":"string"},
				"from_role_id":{"type":"string"},
				"to_role_id":{"type":"string"},
				"target_assignment_id":{"type":"string"},
				"target_work_item_id":{"type":"string"},
				"title":{"type":"string"},
				"body":{"type":"string"},
				"recommended_next_action":{"type":"string"},
				"linked_artifact_ids":{"type":"array","items":{"type":"string"}},
				"linked_memory_ids":{"type":"array","items":{"type":"string"}},
				"context_refs":{"type":"array","items":{"type":"string"}},
				"status":{"type":"string","enum":["open","accepted","superseded","dismissed"]},
				"provenance_kind":{"type":"string"},
				"trust_label":{"type":"string"}
			},
			"required":["project_id","work_item_id","handoff_id"]
		}`),
	}, updateHandoff(service))

	server.RegisterTool(mcp.Tool{
		Name:        "handoffs.update_status",
		Title:       "Update handoff status",
		Description: "Transition a handoff status without changing its text or refs.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"handoff_id":{"type":"string","minLength":1},
				"status":{"type":"string","enum":["open","accepted","superseded","dismissed"]}
			},
			"required":["project_id","work_item_id","handoff_id","status"]
		}`),
	}, updateHandoffStatus(service))

	server.RegisterTool(mcp.Tool{
		Name:        "handoffs.delete",
		Title:       "Delete handoff",
		Description: "Delete a handoff record without deleting linked work.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"work_item_id":{"type":"string","minLength":1},
				"handoff_id":{"type":"string","minLength":1}
			},
			"required":["project_id","work_item_id","handoff_id"]
		}`),
	}, deleteHandoff(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_entries.list",
		Title:       "List memory entries",
		Description: "List accepted project memory entries.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"include_disabled":{"type":"boolean"}
			},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listMemoryEntries(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_entries.get",
		Title:       "Get memory entry",
		Description: "Get one accepted project memory entry by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"memory_id":{"type":"string","minLength":1}
			},
			"required":["project_id","memory_id"]
		}`),
		Annotations: readOnly,
	}, getMemoryEntry(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_entries.create",
		Title:       "Create memory entry",
		Description: "Create an accepted project memory entry.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"title":{"type":"string","minLength":1},
				"body":{"type":"string","minLength":1},
				"trust_label":{"type":"string"},
				"source_kind":{"type":"string"},
				"source_id":{"type":"string"}
			},
			"required":["project_id","title","body"]
		}`),
	}, createMemoryEntry(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_entries.update",
		Title:       "Update memory entry",
		Description: "Patch an accepted project memory entry.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"memory_id":{"type":"string","minLength":1},
				"title":{"type":"string"},
				"body":{"type":"string"},
				"trust_label":{"type":"string"},
				"source_kind":{"type":"string"},
				"source_id":{"type":"string"},
				"enabled":{"type":"boolean"}
			},
			"required":["project_id","memory_id"]
		}`),
	}, updateMemoryEntry(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_entries.delete",
		Title:       "Delete memory entry",
		Description: "Delete an accepted project memory entry.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"memory_id":{"type":"string","minLength":1}
			},
			"required":["project_id","memory_id"]
		}`),
	}, deleteMemoryEntry(service))

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
				"suggested_kind":{"type":"string"},
				"suggested_trust_label":{"type":"string"},
				"suggested_source_kind":{"type":"string"},
				"suggested_source_id":{"type":"string"},
				"source_refs":{"type":"array","items":{"type":"object","properties":{
					"kind":{"type":"string","minLength":1},
					"id":{"type":"string","minLength":1},
					"title":{"type":"string"},
					"url":{"type":"string"}
				},"required":["kind","id"]}}
			},
			"required":["project_id","title","body"]
		}`),
	}, createMemoryCandidate(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_candidates.list",
		Title:       "List memory candidates",
		Description: "List project memory candidates. Pending candidates are returned by default.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"include_resolved":{"type":"boolean"},
				"status":{"type":"string","enum":["pending","promoted","rejected"]}
			},
			"required":["project_id"]
		}`),
		Annotations: readOnly,
	}, listMemoryCandidates(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_candidates.get",
		Title:       "Get memory candidate",
		Description: "Get one project memory candidate by id.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"candidate_id":{"type":"string","minLength":1}
			},
			"required":["project_id","candidate_id"]
		}`),
		Annotations: readOnly,
	}, getMemoryCandidate(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_candidates.promote",
		Title:       "Promote memory candidate",
		Description: "Promote a pending memory candidate into an accepted project memory entry.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"candidate_id":{"type":"string","minLength":1},
				"title":{"type":"string"},
				"body":{"type":"string"},
				"trust_label":{"type":"string"},
				"source_kind":{"type":"string"},
				"source_id":{"type":"string"},
				"enabled":{"type":"boolean"}
			},
			"required":["project_id","candidate_id"]
		}`),
	}, promoteMemoryCandidate(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_candidates.reject",
		Title:       "Reject memory candidate",
		Description: "Reject a pending memory candidate without creating accepted memory.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"candidate_id":{"type":"string","minLength":1},
				"reason":{"type":"string"}
			},
			"required":["project_id","candidate_id"]
		}`),
	}, rejectMemoryCandidate(service))

	server.RegisterTool(mcp.Tool{
		Name:        "memory_candidates.delete",
		Title:       "Delete memory candidate",
		Description: "Delete a project memory candidate.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"project_id":{"type":"string","minLength":1},
				"candidate_id":{"type":"string","minLength":1}
			},
			"required":["project_id","candidate_id"]
		}`),
	}, deleteMemoryCandidate(service))
}

func listProjects(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		items, err := service.ListProjects(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if items == nil {
			items = []core.Project{}
		}
		if len(items) == 0 {
			return mcp.CallToolResult{
				Content:           mcp.TextContent("No projects yet."),
				StructuredContent: items,
			}, nil
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
		return mcp.CallToolResult{
			Content:           mcp.TextContent(b.String()),
			StructuredContent: items,
		}, nil
	}
}

func getProject(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetProject(ctx, input.ID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		var detail strings.Builder
		fmt.Fprintf(&detail, "Project %s: %s", item.ID, item.Name)
		if item.Description != "" {
			fmt.Fprintf(&detail, " — %s", item.Description)
		}
		if len(item.Roots) > 0 {
			fmt.Fprintf(&detail, "\nRoots: %d", len(item.Roots))
		}
		if len(item.ContextSources) > 0 {
			fmt.Fprintf(&detail, "\nContext sources: %d", len(item.ContextSources))
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(detail.String()),
			StructuredContent: item,
		}, nil
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

type sourceArgs struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Title          string            `json:"title"`
	Locator        string            `json:"locator"`
	Enabled        *bool             `json:"enabled"`
	Format         string            `json:"format"`
	Scope          string            `json:"scope"`
	TrustLabel     string            `json:"trust_label"`
	SourceCategory string            `json:"source_category"`
	Metadata       map[string]string `json:"metadata"`
}

func toCoreSources(input []sourceArgs) []core.Source {
	sources := make([]core.Source, 0, len(input))
	for _, source := range input {
		enabled := true
		if source.Enabled != nil {
			enabled = *source.Enabled
		}
		sources = append(sources, core.Source{
			ID:             source.ID,
			Kind:           source.Kind,
			Title:          source.Title,
			Locator:        source.Locator,
			Enabled:        enabled,
			Format:         source.Format,
			Scope:          source.Scope,
			TrustLabel:     source.TrustLabel,
			SourceCategory: source.SourceCategory,
			Metadata:       source.Metadata,
		})
	}
	return sources
}

func createProject(service *core.Service) mcp.ToolHandler {
	type args struct {
		Name                      string       `json:"name"`
		Description               string       `json:"description"`
		Roots                     []rootArgs   `json:"roots"`
		DefaultRootID             string       `json:"default_root_id"`
		DefaultProfileID          string       `json:"default_profile_id"`
		DefaultExecutionProfileID string       `json:"default_execution_profile_id"`
		ContextSources            []sourceArgs `json:"context_sources"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateProject(ctx, core.Project{
			Name:                      input.Name,
			Description:               input.Description,
			Roots:                     toCoreRoots(input.Roots),
			DefaultRootID:             input.DefaultRootID,
			DefaultProfileID:          input.DefaultProfileID,
			DefaultExecutionProfileID: input.DefaultExecutionProfileID,
			ContextSources:            toCoreSources(input.ContextSources),
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
		ID                        string        `json:"id"`
		Name                      *string       `json:"name"`
		Description               *string       `json:"description"`
		Roots                     *[]rootArgs   `json:"roots"`
		DefaultRootID             *string       `json:"default_root_id"`
		DefaultProfileID          *string       `json:"default_profile_id"`
		DefaultExecutionProfileID *string       `json:"default_execution_profile_id"`
		ContextSources            *[]sourceArgs `json:"context_sources"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		existing, err := service.GetProject(ctx, input.ID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if input.Name != nil {
			existing.Name = *input.Name
		}
		if input.Description != nil {
			existing.Description = *input.Description
		}
		if input.Roots != nil {
			existing.Roots = toCoreRoots(*input.Roots)
			if input.DefaultRootID == nil && !rootIDExists(existing.DefaultRootID, existing.Roots) {
				existing.DefaultRootID = ""
			}
		}
		if input.DefaultRootID != nil {
			existing.DefaultRootID = *input.DefaultRootID
		}
		if input.DefaultProfileID != nil {
			existing.DefaultProfileID = *input.DefaultProfileID
		}
		if input.DefaultExecutionProfileID != nil {
			existing.DefaultExecutionProfileID = *input.DefaultExecutionProfileID
		}
		if input.ContextSources != nil {
			existing.ContextSources = toCoreSources(*input.ContextSources)
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

func deleteProject(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteProject(ctx, input.ID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted project %s", strings.TrimSpace(input.ID))),
		}, nil
	}
}

func listRoots(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListRoots(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{
				Content:           mcp.TextContent("No roots for project " + strings.TrimSpace(input.ProjectID) + "."),
				StructuredContent: items,
			}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Roots for %s (%d):\n", strings.TrimSpace(input.ProjectID), len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: %s", item.ID, item.Path)
			if item.Kind != "" {
				fmt.Fprintf(&b, " [%s]", item.Kind)
			}
			if item.GitBranch != "" {
				fmt.Fprintf(&b, " branch=%s", item.GitBranch)
			}
			if !item.Active {
				b.WriteString(" inactive")
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(b.String()),
			StructuredContent: items,
		}, nil
	}
}

func createRoot(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input struct {
			ProjectID string `json:"project_id"`
			ID        string `json:"id"`
			Path      string `json:"path"`
			Kind      string `json:"kind"`
			GitRemote string `json:"git_remote"`
			GitBranch string `json:"git_branch"`
			Active    *bool  `json:"active"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		_, item, err := service.CreateRoot(ctx, input.ProjectID, core.Root{
			ID:        input.ID,
			Path:      input.Path,
			Kind:      input.Kind,
			GitRemote: input.GitRemote,
			GitBranch: input.GitBranch,
			Active:    active,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Created root %s for project %s", item.ID, strings.TrimSpace(input.ProjectID))),
			StructuredContent: item,
		}, nil
	}
}

func updateRoot(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input struct {
			ProjectID string  `json:"project_id"`
			RootID    string  `json:"root_id"`
			Path      *string `json:"path"`
			Kind      *string `json:"kind"`
			GitRemote *string `json:"git_remote"`
			GitBranch *string `json:"git_branch"`
			Active    *bool   `json:"active"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetRoot(ctx, input.ProjectID, input.RootID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if input.Path != nil {
			item.Path = *input.Path
		}
		if input.Kind != nil {
			item.Kind = *input.Kind
		}
		if input.GitRemote != nil {
			item.GitRemote = *input.GitRemote
		}
		if input.GitBranch != nil {
			item.GitBranch = *input.GitBranch
		}
		if input.Active != nil {
			item.Active = *input.Active
		}
		_, item, err = service.UpdateRoot(ctx, input.ProjectID, input.RootID, item)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Updated root %s for project %s", item.ID, strings.TrimSpace(input.ProjectID))),
			StructuredContent: item,
		}, nil
	}
}

func deleteRoot(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		RootID    string `json:"root_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		_, deleted, err := service.DeleteRoot(ctx, input.ProjectID, input.RootID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Deleted root %s for project %s", deleted.ID, strings.TrimSpace(input.ProjectID))),
			StructuredContent: deleted,
		}, nil
	}
}

func listContextSources(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListContextSources(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{
				Content:           mcp.TextContent("No context sources for project " + strings.TrimSpace(input.ProjectID) + "."),
				StructuredContent: items,
			}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Context sources for %s (%d):\n", strings.TrimSpace(input.ProjectID), len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: %s", item.ID, firstNonEmpty(item.Title, item.Locator))
			if item.Kind != "" {
				fmt.Fprintf(&b, " [%s]", item.Kind)
			}
			if !item.Enabled {
				b.WriteString(" disabled")
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(b.String()),
			StructuredContent: items,
		}, nil
	}
}

func createContextSource(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input struct {
			ProjectID      string            `json:"project_id"`
			ID             string            `json:"id"`
			Kind           string            `json:"kind"`
			Title          string            `json:"title"`
			Locator        string            `json:"locator"`
			Enabled        *bool             `json:"enabled"`
			Format         string            `json:"format"`
			Scope          string            `json:"scope"`
			TrustLabel     string            `json:"trust_label"`
			SourceCategory string            `json:"source_category"`
			Metadata       map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		_, item, err := service.CreateContextSource(ctx, input.ProjectID, core.Source{
			ID:             input.ID,
			Kind:           input.Kind,
			Title:          input.Title,
			Locator:        input.Locator,
			Enabled:        enabled,
			Format:         input.Format,
			Scope:          input.Scope,
			TrustLabel:     input.TrustLabel,
			SourceCategory: input.SourceCategory,
			Metadata:       input.Metadata,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Created context source %s for project %s", item.ID, strings.TrimSpace(input.ProjectID))),
			StructuredContent: item,
		}, nil
	}
}

func updateContextSource(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input struct {
			ProjectID      string            `json:"project_id"`
			SourceID       string            `json:"source_id"`
			Kind           *string           `json:"kind"`
			Title          *string           `json:"title"`
			Locator        *string           `json:"locator"`
			Enabled        *bool             `json:"enabled"`
			Format         *string           `json:"format"`
			Scope          *string           `json:"scope"`
			TrustLabel     *string           `json:"trust_label"`
			SourceCategory *string           `json:"source_category"`
			Metadata       map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetContextSource(ctx, input.ProjectID, input.SourceID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if input.Kind != nil {
			item.Kind = *input.Kind
		}
		if input.Title != nil {
			item.Title = *input.Title
		}
		if input.Locator != nil {
			item.Locator = *input.Locator
		}
		if input.Enabled != nil {
			item.Enabled = *input.Enabled
		}
		if input.Format != nil {
			item.Format = *input.Format
		}
		if input.Scope != nil {
			item.Scope = *input.Scope
		}
		if input.TrustLabel != nil {
			item.TrustLabel = *input.TrustLabel
		}
		if input.SourceCategory != nil {
			item.SourceCategory = *input.SourceCategory
		}
		if input.Metadata != nil {
			item.Metadata = input.Metadata
		}
		_, item, err = service.UpdateContextSource(ctx, input.ProjectID, input.SourceID, item)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Updated context source %s for project %s", item.ID, strings.TrimSpace(input.ProjectID))),
			StructuredContent: item,
		}, nil
	}
}

func deleteContextSource(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		SourceID  string `json:"source_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		_, deleted, err := service.DeleteContextSource(ctx, input.ProjectID, input.SourceID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Deleted context source %s for project %s", deleted.ID, strings.TrimSpace(input.ProjectID))),
			StructuredContent: deleted,
		}, nil
	}
}

func rootIDExists(id string, roots []core.Root) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, root := range roots {
		if root.ID == id {
			return true
		}
	}
	return false
}

func projectOperationsBrief(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		brief, err := service.ProjectOperationsBrief(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatProjectOperationsBrief(brief)),
			StructuredContent: brief,
		}, nil
	}
}

func projectSetupReadiness(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		readiness, err := service.ProjectSetupReadiness(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatProjectSetupReadiness(readiness)),
			StructuredContent: readiness,
		}, nil
	}
}

func projectHealth(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		health, err := service.ProjectHealth(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatProjectHealth(health)),
			StructuredContent: health,
		}, nil
	}
}

func projectActivity(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		activity, err := service.ProjectActivity(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatProjectActivity(activity)),
			StructuredContent: activity,
		}, nil
	}
}

func assistantPropose(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input core.AssistantProposal
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		record, err := service.CreateAssistantProposal(ctx, input)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatAssistantProposalRecord(record)),
			StructuredContent: record,
		}, nil
	}
}

func listAssistantProposals(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &input); err != nil {
				return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
			}
		}
		items, err := service.ListAssistantProposals(ctx, input.ProjectID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No assistant proposals yet."), StructuredContent: items}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Assistant proposals (%d):\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&b, "- %s: [%s] %s (%d actions)\n", item.ID, item.Status, item.Proposal.Title, len(item.Proposal.Actions))
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String()), StructuredContent: items}, nil
	}
}

func getAssistantProposal(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		record, err := service.GetAssistantProposal(ctx, input.ID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatAssistantProposalRecord(record)),
			StructuredContent: record,
		}, nil
	}
}

func assistantApply(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProposalID string                 `json:"proposal_id"`
		Proposal   core.AssistantProposal `json:"proposal"`
		Confirm    bool                   `json:"confirm"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		var result core.AssistantApplyResult
		var err error
		if strings.TrimSpace(input.ProposalID) != "" {
			result, err = service.ApplyAssistantProposalRecord(ctx, input.ProposalID, input.Confirm)
		} else {
			result, err = service.ApplyAssistantProposal(ctx, input.Proposal, input.Confirm)
		}
		if err != nil && result.ProposalID == "" {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatAssistantApplyResult(result)),
			StructuredContent: result,
			IsError:           err != nil,
		}, nil
	}
}

func listAgentProfiles(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		items, err := service.ListAgentProfiles(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if items == nil {
			items = []core.AgentProfile{}
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No agent profiles yet."), StructuredContent: items}, nil
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
		return mcp.CallToolResult{Content: mcp.TextContent(b.String()), StructuredContent: items}, nil
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

func deleteAgentProfile(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteAgentProfile(ctx, input.ID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted agent profile %s", strings.TrimSpace(input.ID))),
		}, nil
	}
}

func listExecutionProfiles(service *core.Service) mcp.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		items, err := service.ListExecutionProfiles(ctx)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if items == nil {
			items = []core.ExecutionProfile{}
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No execution profiles yet."), StructuredContent: items}, nil
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
		return mcp.CallToolResult{Content: mcp.TextContent(b.String()), StructuredContent: items}, nil
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

func deleteExecutionProfile(service *core.Service) mcp.ToolHandler {
	type args struct {
		ID string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteExecutionProfile(ctx, input.ID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted execution profile %s", strings.TrimSpace(input.ID))),
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
		if items == nil {
			items = []core.WorkItem{}
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No work items yet."), StructuredContent: items}, nil
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
		return mcp.CallToolResult{Content: mcp.TextContent(b.String()), StructuredContent: items}, nil
	}
}

func getWorkItem(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		ID        string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetWorkItem(ctx, input.ProjectID, input.ID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		var detail strings.Builder
		fmt.Fprintf(&detail, "Work item %s: [%s] %s", item.ID, item.Status, item.Title)
		if item.Brief != "" {
			fmt.Fprintf(&detail, " — %s", item.Brief)
		}
		if item.RootID != "" {
			fmt.Fprintf(&detail, "\nRoot: %s", item.RootID)
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(detail.String()),
			StructuredContent: item,
		}, nil
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
		existing, err := service.GetWorkItem(ctx, input.ProjectID, input.ID)
		if err != nil {
			return mcp.CallToolResult{}, err
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

func deleteWorkItem(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		ID        string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteWorkItem(ctx, input.ProjectID, input.ID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted work item %s", strings.TrimSpace(input.ID))),
		}, nil
	}
}

func workItemCloseoutReadiness(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		readiness, err := service.WorkItemCloseoutReadiness(ctx, input.ProjectID, input.WorkItemID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatWorkItemCloseoutReadiness(readiness)),
			StructuredContent: readiness,
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
		if items == nil {
			items = []core.Role{}
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No roles yet."), StructuredContent: items}, nil
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
			if item.DefaultExecutionProfileID != "" {
				fmt.Fprintf(&b, " exec=%s", item.DefaultExecutionProfileID)
			}
			b.WriteByte('\n')
		}
		return mcp.CallToolResult{Content: mcp.TextContent(b.String()), StructuredContent: items}, nil
	}
}

func createRole(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID                 string   `json:"project_id"`
		Name                      string   `json:"name"`
		Description               string   `json:"description"`
		Instructions              string   `json:"instructions"`
		DefaultProfileID          string   `json:"default_profile_id"`
		DefaultExecutionProfileID string   `json:"default_execution_profile_id"`
		DefaultSkillIDs           []string `json:"default_skill_ids"`
		DefaultExecutionMode      string   `json:"default_execution_mode"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateRole(ctx, core.Role{
			ProjectID:                 input.ProjectID,
			Name:                      input.Name,
			Description:               input.Description,
			Instructions:              input.Instructions,
			DefaultProfileID:          input.DefaultProfileID,
			DefaultExecutionProfileID: input.DefaultExecutionProfileID,
			DefaultSkillIDs:           input.DefaultSkillIDs,
			DefaultExecutionMode:      input.DefaultExecutionMode,
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
		ProjectID                 string    `json:"project_id"`
		ID                        string    `json:"id"`
		Name                      *string   `json:"name"`
		Description               *string   `json:"description"`
		Instructions              *string   `json:"instructions"`
		DefaultProfileID          *string   `json:"default_profile_id"`
		DefaultExecutionProfileID *string   `json:"default_execution_profile_id"`
		DefaultSkillIDs           *[]string `json:"default_skill_ids"`
		DefaultExecutionMode      *string   `json:"default_execution_mode"`
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
		if input.DefaultExecutionProfileID != nil {
			existing.DefaultExecutionProfileID = *input.DefaultExecutionProfileID
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

func deleteRole(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		ID        string `json:"id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteRole(ctx, input.ProjectID, input.ID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted role %s", strings.TrimSpace(input.ID))),
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
		if items == nil {
			items = []core.Assignment{}
		}
		if len(items) == 0 {
			return mcp.CallToolResult{Content: mcp.TextContent("No assignments yet."), StructuredContent: items}, nil
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
		return mcp.CallToolResult{Content: mcp.TextContent(b.String()), StructuredContent: items}, nil
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

func updateAssignment(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID          string   `json:"project_id"`
		AssignmentID       string   `json:"assignment_id"`
		WorkItemID         string   `json:"work_item_id"`
		RoleID             string   `json:"role_id"`
		RootID             string   `json:"root_id"`
		ProfileID          string   `json:"profile_id"`
		ExecutionProfileID string   `json:"execution_profile_id"`
		ExecutionMode      string   `json:"execution_mode"`
		DesiredAgentKind   string   `json:"desired_agent_kind"`
		SkillIDs           []string `json:"skill_ids"`
		Status             string   `json:"status"`
		ExecutionRef       string   `json:"execution_ref"`
		ContextSnapshotID  string   `json:"context_snapshot_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.UpdateAssignment(ctx, core.Assignment{
			ProjectID:          input.ProjectID,
			ID:                 input.AssignmentID,
			WorkItemID:         input.WorkItemID,
			RoleID:             input.RoleID,
			RootID:             input.RootID,
			ProfileID:          input.ProfileID,
			ExecutionProfileID: input.ExecutionProfileID,
			ExecutionMode:      input.ExecutionMode,
			Status:             input.Status,
			DesiredAgent: core.DesiredAgent{
				Kind:     input.DesiredAgentKind,
				SkillIDs: input.SkillIDs,
			},
			ExecutionRef:      input.ExecutionRef,
			ContextSnapshotID: input.ContextSnapshotID,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		detail := fmt.Sprintf("Updated assignment %s: work=%s role=%s status=%s", item.ID, item.WorkItemID, item.RoleID, item.Status)
		if item.ExecutionMode != "" {
			detail += " mode=" + item.ExecutionMode
		}
		if item.RootID != "" {
			detail += " root=" + item.RootID
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(detail),
			StructuredContent: item,
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

func releaseAssignment(service *core.Service) mcp.ToolHandler {
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
		item, err := service.ReleaseAssignment(ctx, input.ProjectID, input.AssignmentID, input.ClaimedBy)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Released assignment %s", item.ID)),
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
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatAssignmentContext(item)),
			StructuredContent: item,
		}, nil
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
			Content: mcp.TextContent(fmt.Sprintf("Deleted assignment %s", strings.TrimSpace(input.AssignmentID))),
		}, nil
	}
}

func listArtifacts(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListArtifacts(ctx, input.ProjectID, input.WorkItemID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatArtifacts("Artifacts", items)),
			StructuredContent: items,
		}, nil
	}
}

func getArtifact(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		ArtifactID string `json:"artifact_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetArtifact(ctx, input.ProjectID, input.WorkItemID, input.ArtifactID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatArtifacts("Artifacts", []core.Artifact{item})),
			StructuredContent: item,
		}, nil
	}
}

func createArtifact(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID      string `json:"project_id"`
		WorkItemID     string `json:"work_item_id"`
		AssignmentID   string `json:"assignment_id"`
		Kind           string `json:"kind"`
		Title          string `json:"title"`
		Body           string `json:"body"`
		AuthorRoleID   string `json:"author_role_id"`
		ProvenanceKind string `json:"provenance_kind"`
		TrustLabel     string `json:"trust_label"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateArtifact(ctx, core.Artifact{
			ProjectID:      input.ProjectID,
			WorkItemID:     input.WorkItemID,
			AssignmentID:   input.AssignmentID,
			Kind:           input.Kind,
			Title:          input.Title,
			Body:           input.Body,
			AuthorRoleID:   input.AuthorRoleID,
			ProvenanceKind: input.ProvenanceKind,
			TrustLabel:     input.TrustLabel,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Created artifact %s: %s", item.ID, item.Title)),
			StructuredContent: item,
		}, nil
	}
}

func listEvidence(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListEvidence(ctx, input.ProjectID, input.WorkItemID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatEvidence("Evidence", items)),
			StructuredContent: items,
		}, nil
	}
}

func getEvidence(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		EvidenceID string `json:"evidence_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetEvidence(ctx, input.ProjectID, input.WorkItemID, input.EvidenceID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatEvidence("Evidence", []core.Evidence{item})),
			StructuredContent: item,
		}, nil
	}
}

func recordEvidence(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID    string `json:"project_id"`
		WorkItemID   string `json:"work_item_id"`
		AssignmentID string `json:"assignment_id"`
		Title        string `json:"title"`
		Body         string `json:"body"`
		Locator      string `json:"locator"`
		SourceKind   string `json:"source_kind"`
		ExternalID   string `json:"external_id"`
		Provider     string `json:"provider"`
		TrustLabel   string `json:"trust_label"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateEvidence(ctx, core.Evidence{
			ProjectID:    input.ProjectID,
			WorkItemID:   input.WorkItemID,
			AssignmentID: input.AssignmentID,
			Title:        input.Title,
			Body:         input.Body,
			Locator:      input.Locator,
			SourceKind:   input.SourceKind,
			ExternalID:   input.ExternalID,
			Provider:     input.Provider,
			TrustLabel:   input.TrustLabel,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Recorded evidence %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func listReviews(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListReviews(ctx, input.ProjectID, input.WorkItemID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatReviews("Reviews", items)),
			StructuredContent: items,
		}, nil
	}
}

func getReview(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		ReviewID   string `json:"review_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetReview(ctx, input.ProjectID, input.WorkItemID, input.ReviewID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatReviews("Review", []core.Review{item})),
			StructuredContent: item,
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
		ProjectID             string   `json:"project_id"`
		WorkItemID            string   `json:"work_item_id"`
		SourceAssignmentID    string   `json:"source_assignment_id"`
		SourceRunID           string   `json:"source_run_id"`
		SourceChatSessionID   string   `json:"source_chat_session_id"`
		SourceMessageID       string   `json:"source_message_id"`
		FromRoleID            string   `json:"from_role_id"`
		ToRoleID              string   `json:"to_role_id"`
		TargetAssignmentID    string   `json:"target_assignment_id"`
		TargetWorkItemID      string   `json:"target_work_item_id"`
		Title                 string   `json:"title"`
		Body                  string   `json:"body"`
		RecommendedNextAction string   `json:"recommended_next_action"`
		LinkedArtifactIDs     []string `json:"linked_artifact_ids"`
		LinkedMemoryIDs       []string `json:"linked_memory_ids"`
		ContextRefs           []string `json:"context_refs"`
		Status                string   `json:"status"`
		ProvenanceKind        string   `json:"provenance_kind"`
		TrustLabel            string   `json:"trust_label"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateHandoff(ctx, core.Handoff{
			ProjectID:             input.ProjectID,
			WorkItemID:            input.WorkItemID,
			SourceAssignmentID:    input.SourceAssignmentID,
			SourceRunID:           input.SourceRunID,
			SourceChatSessionID:   input.SourceChatSessionID,
			SourceMessageID:       input.SourceMessageID,
			FromRoleID:            input.FromRoleID,
			ToRoleID:              input.ToRoleID,
			TargetAssignmentID:    input.TargetAssignmentID,
			TargetWorkItemID:      input.TargetWorkItemID,
			Title:                 input.Title,
			Body:                  input.Body,
			RecommendedNextAction: input.RecommendedNextAction,
			LinkedArtifactIDs:     input.LinkedArtifactIDs,
			LinkedMemoryIDs:       input.LinkedMemoryIDs,
			ContextRefs:           input.ContextRefs,
			Status:                input.Status,
			ProvenanceKind:        input.ProvenanceKind,
			TrustLabel:            input.TrustLabel,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		detail := fmt.Sprintf("Created handoff %s: %s", item.ID, item.Title)
		if item.Status != "" {
			detail += " status=" + item.Status
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(detail),
		}, nil
	}
}

func listHandoffs(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListHandoffs(ctx, input.ProjectID, input.WorkItemID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatHandoffs("Handoffs", items)),
			StructuredContent: items,
		}, nil
	}
}

func getHandoff(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		HandoffID  string `json:"handoff_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetHandoff(ctx, input.ProjectID, input.WorkItemID, input.HandoffID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatHandoffs("Handoff", []core.Handoff{item})),
			StructuredContent: item,
		}, nil
	}
}

func updateHandoff(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID             string    `json:"project_id"`
		WorkItemID            string    `json:"work_item_id"`
		HandoffID             string    `json:"handoff_id"`
		SourceAssignmentID    *string   `json:"source_assignment_id"`
		SourceRunID           *string   `json:"source_run_id"`
		SourceChatSessionID   *string   `json:"source_chat_session_id"`
		SourceMessageID       *string   `json:"source_message_id"`
		FromRoleID            *string   `json:"from_role_id"`
		ToRoleID              *string   `json:"to_role_id"`
		TargetAssignmentID    *string   `json:"target_assignment_id"`
		TargetWorkItemID      *string   `json:"target_work_item_id"`
		Title                 *string   `json:"title"`
		Body                  *string   `json:"body"`
		RecommendedNextAction *string   `json:"recommended_next_action"`
		LinkedArtifactIDs     *[]string `json:"linked_artifact_ids"`
		LinkedMemoryIDs       *[]string `json:"linked_memory_ids"`
		ContextRefs           *[]string `json:"context_refs"`
		Status                *string   `json:"status"`
		ProvenanceKind        *string   `json:"provenance_kind"`
		TrustLabel            *string   `json:"trust_label"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		existing, err := service.GetHandoff(ctx, input.ProjectID, input.WorkItemID, input.HandoffID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if input.SourceAssignmentID != nil {
			existing.SourceAssignmentID = *input.SourceAssignmentID
		}
		if input.SourceRunID != nil {
			existing.SourceRunID = *input.SourceRunID
		}
		if input.SourceChatSessionID != nil {
			existing.SourceChatSessionID = *input.SourceChatSessionID
		}
		if input.SourceMessageID != nil {
			existing.SourceMessageID = *input.SourceMessageID
		}
		if input.FromRoleID != nil {
			existing.FromRoleID = *input.FromRoleID
		}
		if input.ToRoleID != nil {
			existing.ToRoleID = *input.ToRoleID
		}
		if input.TargetAssignmentID != nil {
			existing.TargetAssignmentID = *input.TargetAssignmentID
		}
		if input.TargetWorkItemID != nil {
			existing.TargetWorkItemID = *input.TargetWorkItemID
		}
		if input.Title != nil {
			existing.Title = *input.Title
		}
		if input.Body != nil {
			existing.Body = *input.Body
		}
		if input.RecommendedNextAction != nil {
			existing.RecommendedNextAction = *input.RecommendedNextAction
		}
		if input.LinkedArtifactIDs != nil {
			existing.LinkedArtifactIDs = *input.LinkedArtifactIDs
		}
		if input.LinkedMemoryIDs != nil {
			existing.LinkedMemoryIDs = *input.LinkedMemoryIDs
		}
		if input.ContextRefs != nil {
			existing.ContextRefs = *input.ContextRefs
		}
		if input.Status != nil {
			existing.Status = *input.Status
		}
		if input.ProvenanceKind != nil {
			existing.ProvenanceKind = *input.ProvenanceKind
		}
		if input.TrustLabel != nil {
			existing.TrustLabel = *input.TrustLabel
		}
		item, err := service.UpdateHandoff(ctx, existing)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Updated handoff %s: %s [%s]", item.ID, item.Title, item.Status)),
			StructuredContent: item,
		}, nil
	}
}

func updateHandoffStatus(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		HandoffID  string `json:"handoff_id"`
		Status     string `json:"status"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.UpdateHandoffStatus(ctx, input.ProjectID, input.WorkItemID, input.HandoffID, input.Status)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Updated handoff %s: %s", item.ID, item.Status)),
			StructuredContent: item,
		}, nil
	}
}

func deleteHandoff(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		WorkItemID string `json:"work_item_id"`
		HandoffID  string `json:"handoff_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteHandoff(ctx, input.ProjectID, input.WorkItemID, input.HandoffID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted handoff %s", strings.TrimSpace(input.HandoffID))),
		}, nil
	}
}

func listMemoryEntries(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID       string `json:"project_id"`
		IncludeDisabled bool   `json:"include_disabled"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListMemoryEntries(ctx, input.ProjectID, input.IncludeDisabled)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatMemoryEntries("Memory entries", items)),
			StructuredContent: items,
		}, nil
	}
}

func getMemoryEntry(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		MemoryID  string `json:"memory_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetMemoryEntry(ctx, input.ProjectID, input.MemoryID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatMemoryEntries("Memory entry", []core.MemoryEntry{item})),
			StructuredContent: item,
		}, nil
	}
}

func createMemoryEntry(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string `json:"project_id"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		TrustLabel string `json:"trust_label"`
		SourceKind string `json:"source_kind"`
		SourceID   string `json:"source_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
			ProjectID:  input.ProjectID,
			Title:      input.Title,
			Body:       input.Body,
			TrustLabel: input.TrustLabel,
			SourceKind: input.SourceKind,
			SourceID:   input.SourceID,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created memory entry %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func updateMemoryEntry(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID  string  `json:"project_id"`
		MemoryID   string  `json:"memory_id"`
		Title      *string `json:"title"`
		Body       *string `json:"body"`
		TrustLabel *string `json:"trust_label"`
		SourceKind *string `json:"source_kind"`
		SourceID   *string `json:"source_id"`
		Enabled    *bool   `json:"enabled"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		existing, err := service.GetMemoryEntry(ctx, input.ProjectID, input.MemoryID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		if input.Title != nil {
			existing.Title = *input.Title
		}
		if input.Body != nil {
			existing.Body = *input.Body
		}
		if input.TrustLabel != nil {
			existing.TrustLabel = *input.TrustLabel
		}
		if input.SourceKind != nil {
			existing.SourceKind = *input.SourceKind
		}
		if input.SourceID != nil {
			existing.SourceID = *input.SourceID
		}
		if input.Enabled != nil {
			existing.Enabled = *input.Enabled
		}
		item, err := service.UpdateMemoryEntry(ctx, existing)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Updated memory entry %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func deleteMemoryEntry(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID string `json:"project_id"`
		MemoryID  string `json:"memory_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteMemoryEntry(ctx, input.ProjectID, input.MemoryID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted memory entry %s", input.MemoryID)),
		}, nil
	}
}

type memoryCandidateSourceRefArgs struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func createMemoryCandidate(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID           string                         `json:"project_id"`
		Title               string                         `json:"title"`
		Body                string                         `json:"body"`
		SuggestedKind       string                         `json:"suggested_kind"`
		SuggestedTrustLabel string                         `json:"suggested_trust_label"`
		SuggestedSourceKind string                         `json:"suggested_source_kind"`
		SuggestedSourceID   string                         `json:"suggested_source_id"`
		SourceRefs          []memoryCandidateSourceRefArgs `json:"source_refs"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
			ProjectID:           input.ProjectID,
			Title:               input.Title,
			Body:                input.Body,
			SuggestedKind:       input.SuggestedKind,
			SuggestedTrustLabel: input.SuggestedTrustLabel,
			SuggestedSourceKind: input.SuggestedSourceKind,
			SuggestedSourceID:   input.SuggestedSourceID,
			SourceRefs:          toCoreMemoryCandidateSourceRefs(input.SourceRefs),
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Created memory candidate %s: %s", item.ID, item.Title)),
		}, nil
	}
}

func listMemoryCandidates(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID       string `json:"project_id"`
		IncludeResolved bool   `json:"include_resolved"`
		Status          string `json:"status"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		items, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{
			ProjectID:       input.ProjectID,
			Status:          input.Status,
			IncludeResolved: input.IncludeResolved,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatMemoryCandidates("Memory candidates", items)),
			StructuredContent: items,
		}, nil
	}
}

func getMemoryCandidate(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID   string `json:"project_id"`
		CandidateID string `json:"candidate_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.GetMemoryCandidate(ctx, input.ProjectID, input.CandidateID)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(formatMemoryCandidates("Memory candidate", []core.MemoryCandidate{item})),
			StructuredContent: item,
		}, nil
	}
}

func promoteMemoryCandidate(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID   string  `json:"project_id"`
		CandidateID string  `json:"candidate_id"`
		Title       *string `json:"title"`
		Body        *string `json:"body"`
		TrustLabel  *string `json:"trust_label"`
		SourceKind  *string `json:"source_kind"`
		SourceID    *string `json:"source_id"`
		Enabled     *bool   `json:"enabled"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		candidate, entry, err := service.PromoteMemoryCandidate(ctx, core.MemoryCandidatePromotion{
			ProjectID:   input.ProjectID,
			CandidateID: input.CandidateID,
			Title:       input.Title,
			Body:        input.Body,
			TrustLabel:  input.TrustLabel,
			SourceKind:  input.SourceKind,
			SourceID:    input.SourceID,
			Enabled:     input.Enabled,
		})
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Promoted memory candidate %s to memory entry %s", candidate.ID, entry.ID)),
			StructuredContent: candidate,
		}, nil
	}
}

func rejectMemoryCandidate(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID   string `json:"project_id"`
		CandidateID string `json:"candidate_id"`
		Reason      string `json:"reason"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		item, err := service.RejectMemoryCandidate(ctx, input.ProjectID, input.CandidateID, input.Reason)
		if err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content:           mcp.TextContent(fmt.Sprintf("Rejected memory candidate %s", item.ID)),
			StructuredContent: item,
		}, nil
	}
}

func deleteMemoryCandidate(service *core.Service) mcp.ToolHandler {
	type args struct {
		ProjectID   string `json:"project_id"`
		CandidateID string `json:"candidate_id"`
	}
	return func(ctx context.Context, raw json.RawMessage) (mcp.CallToolResult, error) {
		var input args
		if err := json.Unmarshal(raw, &input); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := service.DeleteMemoryCandidate(ctx, input.ProjectID, input.CandidateID); err != nil {
			return mcp.CallToolResult{}, err
		}
		return mcp.CallToolResult{
			Content: mcp.TextContent(fmt.Sprintf("Deleted memory candidate %s", input.CandidateID)),
		}, nil
	}
}

func toCoreMemoryCandidateSourceRefs(input []memoryCandidateSourceRefArgs) []core.MemoryCandidateSourceRef {
	out := make([]core.MemoryCandidateSourceRef, 0, len(input))
	for _, ref := range input {
		out = append(out, core.MemoryCandidateSourceRef{
			Kind:  ref.Kind,
			ID:    ref.ID,
			Title: ref.Title,
			URL:   ref.URL,
		})
	}
	return out
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
	fmt.Fprintf(&b, "Skills: %d; artifacts: %d; evidence: %d; reviews: %d; handoffs: %d; memory: %d; memory candidates: %d\n", len(packet.Skills), len(packet.Artifacts), len(packet.Evidence), len(packet.Reviews), len(packet.Handoffs), len(packet.Memory), len(packet.MemoryCandidates))
	for _, warning := range packet.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", warning)
	}
	return b.String()
}

type projectSkillArgs struct {
	ProjectID           string                    `json:"project_id"`
	ID                  string                    `json:"id"`
	Title               *string                   `json:"title"`
	Description         *string                   `json:"description"`
	Path                *string                   `json:"path"`
	RootID              *string                   `json:"root_id"`
	Format              *string                   `json:"format"`
	SuggestedTools      *[]string                 `json:"suggested_tools"`
	RequiredPermissions *core.RequiredPermissions `json:"required_permissions"`
	Enabled             *bool                     `json:"enabled"`
	Status              *string                   `json:"status"`
	TrustLabel          *string                   `json:"trust_label"`
	SourceRefs          *[]string                 `json:"source_refs"`
	Warnings            *[]string                 `json:"warnings"`
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
		ProjectID:           input.ProjectID,
		ID:                  input.ID,
		Title:               stringValue(input.Title),
		Description:         stringValue(input.Description),
		Path:                stringValue(input.Path),
		RootID:              stringValue(input.RootID),
		Format:              stringValue(input.Format),
		SuggestedTools:      stringSliceValue(input.SuggestedTools),
		RequiredPermissions: requiredPermissionsValue(input.RequiredPermissions),
		Enabled:             enabled,
		Status:              stringValue(input.Status),
		TrustLabel:          stringValue(input.TrustLabel),
		SourceRefs:          stringSliceValue(input.SourceRefs),
		Warnings:            stringSliceValue(input.Warnings),
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
	if input.SuggestedTools != nil {
		existing.SuggestedTools = *input.SuggestedTools
	}
	if input.RequiredPermissions != nil {
		existing.RequiredPermissions = *input.RequiredPermissions
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

func requiredPermissionsValue(value *core.RequiredPermissions) core.RequiredPermissions {
	if value == nil {
		return core.RequiredPermissions{}
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
		if len(item.SuggestedTools) > 0 {
			fmt.Fprintf(&b, " tools=%s", strings.Join(item.SuggestedTools, ","))
		}
		if permissions := formatRequiredPermissions(item.RequiredPermissions); permissions != "" {
			fmt.Fprintf(&b, " permissions=%s", permissions)
		}
		if len(item.Warnings) > 0 {
			fmt.Fprintf(&b, " warnings=%s", strings.Join(item.Warnings, "; "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatRequiredPermissions(permissions core.RequiredPermissions) string {
	if permissions.Empty() {
		return ""
	}
	parts := make([]string, 0, 3)
	if permissions.Tools != nil {
		parts = append(parts, fmt.Sprintf("tools:%t", *permissions.Tools))
	}
	if permissions.Writes != nil {
		parts = append(parts, fmt.Sprintf("writes:%t", *permissions.Writes))
	}
	if permissions.Network != nil {
		parts = append(parts, fmt.Sprintf("network:%t", *permissions.Network))
	}
	return strings.Join(parts, ",")
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

func formatArtifacts(title string, items []core.Artifact) string {
	if len(items) == 0 {
		return "No artifacts."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: [%s]", item.ID, item.Kind)
		if item.Title != "" {
			fmt.Fprintf(&b, " %s", item.Title)
		}
		if item.AssignmentID != "" {
			fmt.Fprintf(&b, " assignment=%s", item.AssignmentID)
		}
		if item.AuthorRoleID != "" {
			fmt.Fprintf(&b, " author_role=%s", item.AuthorRoleID)
		}
		if item.TrustLabel != "" {
			fmt.Fprintf(&b, " trust=%s", item.TrustLabel)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatEvidence(title string, items []core.Evidence) string {
	if len(items) == 0 {
		return "No evidence."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: %s", item.ID, item.Title)
		if item.AssignmentID != "" {
			fmt.Fprintf(&b, " assignment=%s", item.AssignmentID)
		}
		if item.Locator != "" {
			fmt.Fprintf(&b, " locator=%s", item.Locator)
		}
		if item.SourceKind != "" {
			fmt.Fprintf(&b, " source=%s", item.SourceKind)
		}
		if item.ExternalID != "" {
			fmt.Fprintf(&b, " external_id=%s", item.ExternalID)
		}
		if item.Provider != "" {
			fmt.Fprintf(&b, " provider=%s", item.Provider)
		}
		if item.TrustLabel != "" {
			fmt.Fprintf(&b, " trust=%s", item.TrustLabel)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatReviews(title string, items []core.Review) string {
	if len(items) == 0 {
		return "No reviews."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: [%s] %s", item.ID, item.Verdict, item.Title)
		if item.Risk != "" {
			fmt.Fprintf(&b, " risk=%s", item.Risk)
		}
		if item.AssignmentID != "" {
			fmt.Fprintf(&b, " assignment=%s", item.AssignmentID)
		}
		if item.ReviewerRoleID != "" {
			fmt.Fprintf(&b, " reviewer=%s", item.ReviewerRoleID)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatHandoffs(title string, items []core.Handoff) string {
	if len(items) == 0 {
		return "No handoffs."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: [%s] %s", item.ID, item.Status, item.Title)
		if item.FromRoleID != "" {
			fmt.Fprintf(&b, " from=%s", item.FromRoleID)
		}
		if item.ToRoleID != "" {
			fmt.Fprintf(&b, " to=%s", item.ToRoleID)
		}
		if item.TargetAssignmentID != "" {
			fmt.Fprintf(&b, " target_assignment=%s", item.TargetAssignmentID)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatWorkItemCloseoutReadiness(readiness core.WorkItemCloseoutReadiness) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closeout readiness %s: %s\n", readiness.WorkItemID, readiness.Status)
	fmt.Fprintf(&b, "%s\n", readiness.Title)
	if readiness.Detail != "" {
		fmt.Fprintf(&b, "%s\n", readiness.Detail)
	}
	fmt.Fprintf(&b, "Assignments: %d/%d complete\n", readiness.CompletedAssignments, readiness.AssignmentCount)
	if len(readiness.Blockers) > 0 {
		b.WriteString("Blockers:\n")
		for _, blocker := range readiness.Blockers {
			fmt.Fprintf(&b, "- %s\n", blocker)
		}
	}
	if len(readiness.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, warning := range readiness.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	if len(readiness.ReviewFollowUps) > 0 {
		b.WriteString("Review follow-ups:\n")
		for _, followUp := range readiness.ReviewFollowUps {
			fmt.Fprintf(&b, "- %s: %s\n", followUp.ArtifactID, followUp.Blocker)
		}
	}
	if len(readiness.MissingEvidenceAssignmentIDs) > 0 {
		fmt.Fprintf(&b, "Missing evidence assignments: %s\n", strings.Join(readiness.MissingEvidenceAssignmentIDs, ", "))
	}
	return b.String()
}

func formatProjectSetupReadiness(readiness core.ProjectSetupReadiness) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Setup readiness %s\n", readiness.ProjectID)
	fmt.Fprintf(&b, "show_onboarding=%t setup_started=%t first_work_ready=%t\n", readiness.ShowOnboarding, readiness.SetupStarted, readiness.FirstWorkReady)
	fmt.Fprintf(&b, "Summary: work_items=%d roles=%d skills=%d execution_profiles=%d sources=%d memory=%d candidates=%d purpose=%t active_root=%t\n",
		readiness.Summary.WorkItemCount,
		readiness.Summary.RoleCount,
		readiness.Summary.SkillCount,
		readiness.Summary.ExecutionProfileCount,
		readiness.Summary.EnabledContextSourceCount,
		readiness.Summary.SavedMemoryCount,
		readiness.Summary.PendingMemoryCandidateCount,
		readiness.Summary.HasPurpose,
		readiness.Summary.HasActiveRoot,
	)
	if readiness.PrimaryAction.Kind != "" {
		fmt.Fprintf(&b, "Primary action: %s (%s)\n", readiness.PrimaryAction.Label, readiness.PrimaryAction.Kind)
	}
	if len(readiness.Checks) > 0 {
		b.WriteString("Checks:\n")
		for _, check := range readiness.Checks {
			fmt.Fprintf(&b, "- [%s] %s", check.Status, check.Label)
			if check.Optional {
				b.WriteString(" optional")
			}
			if check.Detail != "" {
				fmt.Fprintf(&b, " — %s", check.Detail)
			}
			if check.Action != nil {
				fmt.Fprintf(&b, " action=%s", check.Action.Kind)
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatProjectHealth(health core.ProjectHealth) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project health %s: %s\n", health.ProjectID, health.Status)
	fmt.Fprintf(&b, "%s\n", health.Title)
	if health.Detail != "" {
		fmt.Fprintf(&b, "%s\n", health.Detail)
	}
	fmt.Fprintf(&b, "Counts: attention=%d/%d omitted=%d setup_todo=%d active=%d blocked=%d memory_candidates=%d review_follow_ups=%d open_handoffs=%d profile_refs=%d skill_issues=%d\n",
		health.Summary.AttentionCount,
		health.Summary.AvailableAttentionCount,
		health.Summary.OmittedAttentionCount,
		health.Summary.SetupTodoCount,
		health.Summary.ActiveAssignmentCount,
		health.Summary.BlockedAssignmentCount,
		health.Summary.PendingMemoryCandidateCount,
		health.Summary.ReviewFollowUpCount,
		health.Summary.OpenHandoffCount,
		health.Summary.MissingProfileReferenceCount,
		health.Summary.ProjectSkillIssueCount,
	)
	if len(health.Attention) > 0 {
		b.WriteString("Attention:\n")
		for _, item := range health.Attention {
			fmt.Fprintf(&b, "- [%s/%s]", item.Kind, item.Severity)
			if item.Status != "" {
				fmt.Fprintf(&b, " %s", item.Status)
			}
			fmt.Fprintf(&b, " %s", item.Title)
			if item.WorkItemID != "" {
				fmt.Fprintf(&b, " work=%s", item.WorkItemID)
			}
			if item.AssignmentID != "" {
				fmt.Fprintf(&b, " assignment=%s", item.AssignmentID)
			}
			if item.ArtifactID != "" {
				fmt.Fprintf(&b, " artifact=%s", item.ArtifactID)
			}
			if item.HandoffID != "" {
				fmt.Fprintf(&b, " handoff=%s", item.HandoffID)
			}
			if item.MemoryCandidateID != "" {
				fmt.Fprintf(&b, " memory_candidate=%s", item.MemoryCandidateID)
			}
			if item.ActionKind != "" {
				fmt.Fprintf(&b, " action=%s", item.ActionKind)
			}
			if item.Detail != "" {
				fmt.Fprintf(&b, " — %s", item.Detail)
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatProjectOperationsBrief(brief core.ProjectOperationsBrief) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Operations brief %s: %s\n", brief.ProjectID, brief.Status)
	fmt.Fprintf(&b, "%s\n", brief.Title)
	if brief.Detail != "" {
		fmt.Fprintf(&b, "%s\n", brief.Detail)
	}
	fmt.Fprintf(&b, "Counts: work_items=%d open=%d assignments=%d active=%d blocked=%d memory_candidates=%d review_follow_ups=%d missing_evidence=%d open_handoffs=%d closeout_ready=%d\n",
		brief.Counts.WorkItems,
		brief.Counts.OpenWorkItems,
		brief.Counts.Assignments,
		brief.Counts.ActiveAssignments,
		brief.Counts.BlockedAssignments,
		brief.Counts.PendingMemoryCandidates,
		brief.Counts.ReviewFollowUps,
		brief.Counts.MissingEvidence,
		brief.Counts.OpenHandoffs,
		brief.Counts.CloseoutReady,
	)
	if brief.Next != nil {
		fmt.Fprintf(&b, "Next: [%s/%s] %s\n", brief.Next.Kind, brief.Next.Severity, brief.Next.Title)
	}
	if len(brief.Items) > 0 {
		b.WriteString("Items:\n")
		for _, item := range brief.Items {
			fmt.Fprintf(&b, "- [%s/%s]", item.Kind, item.Severity)
			if item.Status != "" {
				fmt.Fprintf(&b, " %s", item.Status)
			}
			fmt.Fprintf(&b, " %s", item.Title)
			if item.WorkItemID != "" {
				fmt.Fprintf(&b, " work=%s", item.WorkItemID)
			}
			if item.AssignmentID != "" {
				fmt.Fprintf(&b, " assignment=%s", item.AssignmentID)
			}
			if item.ArtifactID != "" {
				fmt.Fprintf(&b, " artifact=%s", item.ArtifactID)
			}
			if item.MemoryCandidateID != "" {
				fmt.Fprintf(&b, " memory_candidate=%s", item.MemoryCandidateID)
			}
			if item.Detail != "" {
				fmt.Fprintf(&b, " — %s", item.Detail)
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatProjectActivity(activity core.ProjectActivity) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project activity %s\n", activity.ProjectID)
	fmt.Fprintf(&b, "Counts: assignments=%d active=%d blocked=%d queued=%d claimed=%d running=%d awaiting_review=%d completed=%d failed=%d cancelled=%d other=%d\n",
		activity.Counts.Assignments,
		activity.Counts.Active,
		activity.Counts.Blocked,
		activity.Counts.Queued,
		activity.Counts.Claimed,
		activity.Counts.Running,
		activity.Counts.AwaitingReview,
		activity.Counts.Completed,
		activity.Counts.Failed,
		activity.Counts.Cancelled,
		activity.Counts.Other,
	)
	formatProjectActivityBucket(&b, "Active", activity.Buckets.Active)
	formatProjectActivityBucket(&b, "Blocked", activity.Buckets.Blocked)
	formatProjectActivityBucket(&b, "Completed", activity.Buckets.Completed)
	formatProjectActivityBucket(&b, "Other", activity.Buckets.Other)
	return b.String()
}

func formatProjectActivityBucket(b *strings.Builder, title string, items []core.ProjectActivityItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, item := range items {
		fmt.Fprintf(b, "- %s: [%s] work=%s", item.AssignmentID, item.Status, item.WorkItemID)
		if item.WorkItemTitle != "" {
			fmt.Fprintf(b, " title=%q", item.WorkItemTitle)
		}
		if item.RoleID != "" {
			fmt.Fprintf(b, " role=%s", item.RoleID)
		}
		if item.ExecutionMode != "" {
			fmt.Fprintf(b, " mode=%s", item.ExecutionMode)
		}
		b.WriteByte('\n')
	}
}

func formatAssistantProposal(proposal core.AssistantProposal) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Assistant proposal %s: %s\n", proposal.ID, proposal.Title)
	if proposal.ProjectID != "" {
		fmt.Fprintf(&b, "Project: %s\n", proposal.ProjectID)
	}
	if proposal.Summary != "" {
		fmt.Fprintf(&b, "%s\n", proposal.Summary)
	}
	for _, warning := range proposal.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", warning)
	}
	fmt.Fprintf(&b, "requires_confirmation=%t actions=%d\n", proposal.RequiresConfirmation, len(proposal.Actions))
	for idx, action := range proposal.Actions {
		fmt.Fprintf(&b, "%d. %s", idx+1, action.Kind)
		if action.Title != "" {
			fmt.Fprintf(&b, " — %s", action.Title)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatAssistantProposalRecord(record core.AssistantProposalRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Assistant proposal %s: [%s] %s\n", record.ID, record.Status, record.Proposal.Title)
	if record.ProjectID != "" {
		fmt.Fprintf(&b, "Project: %s\n", record.ProjectID)
	}
	if record.Proposal.Summary != "" {
		fmt.Fprintf(&b, "%s\n", record.Proposal.Summary)
	}
	for _, warning := range record.Proposal.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", warning)
	}
	fmt.Fprintf(&b, "source=%s requires_confirmation=%t actions=%d attempts=%d\n", record.Source, record.Proposal.RequiresConfirmation, len(record.Proposal.Actions), len(record.ApplyAttempts))
	if record.LatestResult != nil {
		fmt.Fprintf(&b, "latest=%s applied=%t actions=%d/%d\n", record.LatestResult.Status, record.LatestResult.Applied, record.LatestResult.AppliedActionCount, record.LatestResult.TotalActionCount)
	}
	for idx, action := range record.Proposal.Actions {
		fmt.Fprintf(&b, "%d. %s", idx+1, action.Kind)
		if action.Title != "" {
			fmt.Fprintf(&b, " — %s", action.Title)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatAssistantApplyResult(result core.AssistantApplyResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Assistant apply %s: %s\n", result.ProposalID, result.Status)
	fmt.Fprintf(&b, "confirmed=%t applied=%t actions=%d/%d\n", result.Confirmed, result.Applied, result.AppliedActionCount, result.TotalActionCount)
	if result.FailedActionIndex != nil {
		fmt.Fprintf(&b, "failed_action_index=%d\n", *result.FailedActionIndex)
	}
	for idx, action := range result.Actions {
		fmt.Fprintf(&b, "%d. [%s] %s", idx+1, action.Status, action.Kind)
		refs := assistantActionResultRefs(action)
		if len(refs) > 0 {
			fmt.Fprintf(&b, " %s", strings.Join(refs, " "))
		}
		if action.Error != "" {
			fmt.Fprintf(&b, " — %s", action.Error)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func assistantActionResultRefs(action core.AssistantActionResult) []string {
	refs := make([]string, 0, 7)
	if action.ProjectID != "" {
		refs = append(refs, "project="+action.ProjectID)
	}
	if action.RootID != "" {
		refs = append(refs, "root="+action.RootID)
	}
	if action.RoleID != "" {
		refs = append(refs, "role="+action.RoleID)
	}
	if action.WorkItemID != "" {
		refs = append(refs, "work="+action.WorkItemID)
	}
	if action.AssignmentID != "" {
		refs = append(refs, "assignment="+action.AssignmentID)
	}
	if action.ArtifactID != "" {
		refs = append(refs, "artifact="+action.ArtifactID)
	}
	if action.HandoffID != "" {
		refs = append(refs, "handoff="+action.HandoffID)
	}
	if action.MemoryCandidateID != "" {
		refs = append(refs, "memory_candidate="+action.MemoryCandidateID)
	}
	return refs
}

func formatMemoryEntries(title string, items []core.MemoryEntry) string {
	if len(items) == 0 {
		return "No memory entries."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: %s", item.ID, item.Title)
		if !item.Enabled {
			b.WriteString(" disabled")
		}
		if item.TrustLabel != "" {
			fmt.Fprintf(&b, " trust=%s", item.TrustLabel)
		}
		if item.SourceKind != "" {
			fmt.Fprintf(&b, " source=%s", item.SourceKind)
			if item.SourceID != "" {
				fmt.Fprintf(&b, ":%s", item.SourceID)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatMemoryCandidates(title string, items []core.MemoryCandidate) string {
	if len(items) == 0 {
		return "No memory candidates."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "- %s: %s status=%s", item.ID, item.Title, item.Status)
		if item.SuggestedTrustLabel != "" {
			fmt.Fprintf(&b, " trust=%s", item.SuggestedTrustLabel)
		}
		if item.SuggestedSourceKind != "" {
			fmt.Fprintf(&b, " source=%s", item.SuggestedSourceKind)
			if item.SuggestedSourceID != "" {
				fmt.Fprintf(&b, ":%s", item.SuggestedSourceID)
			}
		}
		if item.PromotedMemoryID != "" {
			fmt.Fprintf(&b, " promoted_memory=%s", item.PromotedMemoryID)
		}
		if item.StatusReason != "" {
			fmt.Fprintf(&b, " reason=%q", item.StatusReason)
		}
		if len(item.SourceRefs) > 0 {
			fmt.Fprintf(&b, " refs=%d", len(item.SourceRefs))
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
