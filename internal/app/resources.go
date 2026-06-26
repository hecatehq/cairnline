package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

const (
	resourcePrefix = "cairnline://projects/"
	jsonMimeType   = "application/json"
)

func RegisterResources(server *mcp.Server, service *core.Service) {
	server.RegisterResourceProvider(projectResources(service), readProjectResource(service))
}

func projectResources(service *core.Service) mcp.ResourceProvider {
	return func(ctx context.Context) ([]mcp.Resource, error) {
		projects, err := service.ListProjects(ctx)
		if err != nil {
			return nil, err
		}
		var resources []mcp.Resource
		for _, project := range projects {
			projectURI := projectResourceURI(project.ID)
			resources = append(resources, mcp.Resource{
				URI:         projectURI,
				Name:        "project/" + project.ID,
				Title:       project.Name,
				Description: "Project coordination summary.",
				MimeType:    jsonMimeType,
			})
			workItems, err := service.ListWorkItems(ctx, project.ID)
			if err != nil {
				return nil, err
			}
			for _, workItem := range workItems {
				resources = append(resources, mcp.Resource{
					URI:         workItemResourceURI(project.ID, workItem.ID),
					Name:        "work_item/" + workItem.ID,
					Title:       workItem.Title,
					Description: "Work item with collaboration artifacts.",
					MimeType:    jsonMimeType,
				})
			}
			assignments, err := service.ListAssignments(ctx, project.ID)
			if err != nil {
				return nil, err
			}
			for _, assignment := range assignments {
				resources = append(resources, mcp.Resource{
					URI:         assignmentResourceURI(project.ID, assignment.ID),
					Name:        "assignment/" + assignment.ID,
					Title:       "Assignment " + assignment.ID,
					Description: "Assignment context metadata.",
					MimeType:    jsonMimeType,
				})
				resources = append(resources, mcp.Resource{
					URI:         assignmentLaunchPacketResourceURI(project.ID, assignment.ID),
					Name:        "assignment_launch_packet/" + assignment.ID,
					Title:       "Launch packet " + assignment.ID,
					Description: "Assignment launch packet for manual or MCP-pull execution.",
					MimeType:    jsonMimeType,
				})
			}
		}
		return resources, nil
	}
}

func readProjectResource(service *core.Service) mcp.ResourceReader {
	return func(ctx context.Context, uri string) (mcp.ReadResourceResult, bool, error) {
		ref, ok := parseProjectResourceURI(uri)
		if !ok {
			return mcp.ReadResourceResult{}, false, nil
		}
		switch ref.kind {
		case "project":
			payload, err := buildProjectResource(ctx, service, ref.projectID)
			return jsonResource(uri, payload, err)
		case "work_item":
			payload, err := buildWorkItemResource(ctx, service, ref.projectID, ref.id)
			return jsonResource(uri, payload, err)
		case "assignment":
			payload, err := service.AssignmentContext(ctx, ref.projectID, ref.id)
			return jsonResource(uri, payload, err)
		case "assignment_launch_packet":
			payload, err := service.AssignmentLaunchPacket(ctx, ref.projectID, ref.id)
			return jsonResource(uri, payload, err)
		default:
			return mcp.ReadResourceResult{}, false, nil
		}
	}
}

type projectResourceRef struct {
	kind      string
	projectID string
	id        string
}

type projectResourcePayload struct {
	Project     core.Project                `json:"project"`
	Operations  core.ProjectOperationsBrief `json:"operations"`
	Activity    core.ProjectActivity        `json:"activity"`
	Roles       []core.Role                 `json:"roles,omitempty"`
	Skills      []core.ProjectSkill         `json:"skills,omitempty"`
	Memory      []core.MemoryEntry          `json:"memory,omitempty"`
	WorkItems   []core.WorkItem             `json:"work_items,omitempty"`
	Assignments []core.Assignment           `json:"assignments,omitempty"`
}

type workItemResourcePayload struct {
	ProjectID   string            `json:"project_id"`
	WorkItem    core.WorkItem     `json:"work_item"`
	Assignments []core.Assignment `json:"assignments,omitempty"`
	Evidence    []core.Evidence   `json:"evidence,omitempty"`
	Reviews     []core.Review     `json:"reviews,omitempty"`
	Handoffs    []core.Handoff    `json:"handoffs,omitempty"`
}

func buildProjectResource(ctx context.Context, service *core.Service, projectID string) (projectResourcePayload, error) {
	projects, err := service.ListProjects(ctx)
	if err != nil {
		return projectResourcePayload{}, err
	}
	var project core.Project
	for _, item := range projects {
		if item.ID == projectID {
			project = item
			break
		}
	}
	if project.ID == "" {
		return projectResourcePayload{}, core.ErrNotFound
	}
	roles, err := service.ListRoles(ctx, projectID)
	if err != nil {
		return projectResourcePayload{}, err
	}
	skills, err := service.ListProjectSkills(ctx, projectID)
	if err != nil {
		return projectResourcePayload{}, err
	}
	workItems, err := service.ListWorkItems(ctx, projectID)
	if err != nil {
		return projectResourcePayload{}, err
	}
	memoryEntries, err := service.ListMemoryEntries(ctx, projectID, false)
	if err != nil {
		return projectResourcePayload{}, err
	}
	assignments, err := service.ListAssignments(ctx, projectID)
	if err != nil {
		return projectResourcePayload{}, err
	}
	operations, err := service.ProjectOperationsBrief(ctx, projectID)
	if err != nil {
		return projectResourcePayload{}, err
	}
	activity, err := service.ProjectActivity(ctx, projectID)
	if err != nil {
		return projectResourcePayload{}, err
	}
	return projectResourcePayload{
		Project:     project,
		Operations:  operations,
		Activity:    activity,
		Roles:       roles,
		Skills:      skills,
		Memory:      memoryEntries,
		WorkItems:   workItems,
		Assignments: assignments,
	}, nil
}

func buildWorkItemResource(ctx context.Context, service *core.Service, projectID, workItemID string) (workItemResourcePayload, error) {
	workItems, err := service.ListWorkItems(ctx, projectID)
	if err != nil {
		return workItemResourcePayload{}, err
	}
	var workItem core.WorkItem
	for _, item := range workItems {
		if item.ID == workItemID {
			workItem = item
			break
		}
	}
	if workItem.ID == "" {
		return workItemResourcePayload{}, core.ErrNotFound
	}
	assignments, err := service.ListAssignments(ctx, projectID)
	if err != nil {
		return workItemResourcePayload{}, err
	}
	assignments = filterAssignmentsByWorkItem(assignments, workItemID)
	evidence, err := service.ListEvidence(ctx, projectID, workItemID)
	if err != nil {
		return workItemResourcePayload{}, err
	}
	reviews, err := service.ListReviews(ctx, projectID, workItemID)
	if err != nil {
		return workItemResourcePayload{}, err
	}
	handoffs, err := service.ListHandoffs(ctx, projectID, workItemID)
	if err != nil {
		return workItemResourcePayload{}, err
	}
	return workItemResourcePayload{
		ProjectID:   projectID,
		WorkItem:    workItem,
		Assignments: assignments,
		Evidence:    evidence,
		Reviews:     reviews,
		Handoffs:    handoffs,
	}, nil
}

func filterAssignmentsByWorkItem(assignments []core.Assignment, workItemID string) []core.Assignment {
	var out []core.Assignment
	for _, item := range assignments {
		if item.WorkItemID == workItemID {
			out = append(out, item)
		}
	}
	return out
}

func jsonResource(uri string, payload any, err error) (mcp.ReadResourceResult, bool, error) {
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return mcp.ReadResourceResult{}, false, nil
		}
		return mcp.ReadResourceResult{}, true, err
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.ReadResourceResult{}, true, err
	}
	return mcp.ReadResourceResult{
		Contents: []mcp.ResourceContent{{
			URI:      uri,
			MimeType: jsonMimeType,
			Text:     string(raw),
		}},
	}, true, nil
}

func projectResourceURI(projectID string) string {
	return resourcePrefix + projectID
}

func workItemResourceURI(projectID, workItemID string) string {
	return resourcePrefix + projectID + "/work-items/" + workItemID
}

func assignmentResourceURI(projectID, assignmentID string) string {
	return resourcePrefix + projectID + "/assignments/" + assignmentID
}

func assignmentLaunchPacketResourceURI(projectID, assignmentID string) string {
	return assignmentResourceURI(projectID, assignmentID) + "/launch-packet"
}

func parseProjectResourceURI(uri string) (projectResourceRef, bool) {
	if !strings.HasPrefix(uri, resourcePrefix) {
		return projectResourceRef{}, false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(uri, resourcePrefix), "/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		return projectResourceRef{kind: "project", projectID: parts[0]}, true
	}
	if len(parts) == 3 && parts[0] != "" && parts[1] == "work-items" && parts[2] != "" {
		return projectResourceRef{kind: "work_item", projectID: parts[0], id: parts[2]}, true
	}
	if len(parts) == 3 && parts[0] != "" && parts[1] == "assignments" && parts[2] != "" {
		return projectResourceRef{kind: "assignment", projectID: parts[0], id: parts[2]}, true
	}
	if len(parts) == 4 && parts[0] != "" && parts[1] == "assignments" && parts[2] != "" && parts[3] == "launch-packet" {
		return projectResourceRef{kind: "assignment_launch_packet", projectID: parts[0], id: parts[2]}, true
	}
	return projectResourceRef{}, false
}
