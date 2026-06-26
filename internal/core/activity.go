package core

import (
	"context"
	"errors"
	"sort"
	"strings"
)

func (s *Service) ProjectActivity(ctx context.Context, projectID string) (ProjectActivity, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ProjectActivity{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if _, err := s.store.GetProject(ctx, projectID); err != nil {
		return ProjectActivity{}, err
	}
	workItems, err := s.store.ListWorkItems(ctx, projectID)
	if err != nil {
		return ProjectActivity{}, err
	}
	roles, err := s.store.ListRoles(ctx, projectID)
	if err != nil {
		return ProjectActivity{}, err
	}
	assignments, err := s.store.ListAssignments(ctx, projectID)
	if err != nil {
		return ProjectActivity{}, err
	}
	workItemsByID := make(map[string]WorkItem, len(workItems))
	for _, item := range workItems {
		workItemsByID[item.ID] = item
	}
	rolesByID := make(map[string]Role, len(roles))
	for _, role := range roles {
		rolesByID[role.ID] = role
	}
	activity := ProjectActivity{
		ProjectID: projectID,
		CreatedAt: s.now(),
	}
	for _, assignment := range assignments {
		item := projectActivityItem(assignment, workItemsByID[assignment.WorkItemID], rolesByID[assignment.RoleID])
		activity.Counts.Assignments++
		countProjectActivityStatus(&activity.Counts, item.Status)
		switch item.Bucket {
		case ProjectActivityBucketActive:
			activity.Counts.Active++
			activity.Buckets.Active = append(activity.Buckets.Active, item)
		case ProjectActivityBucketBlocked:
			activity.Counts.Blocked++
			activity.Buckets.Blocked = append(activity.Buckets.Blocked, item)
		case ProjectActivityBucketCompleted:
			activity.Buckets.Completed = append(activity.Buckets.Completed, item)
		default:
			activity.Buckets.Other = append(activity.Buckets.Other, item)
		}
		activity.Items = append(activity.Items, item)
		activity.Buckets.Recent = append(activity.Buckets.Recent, item)
	}
	sortProjectActivityItems(activity.Items)
	sortProjectActivityItems(activity.Buckets.Active)
	sortProjectActivityItems(activity.Buckets.Blocked)
	sortProjectActivityItems(activity.Buckets.Completed)
	sortProjectActivityItems(activity.Buckets.Other)
	sortProjectActivityItems(activity.Buckets.Recent)
	return activity, nil
}

func projectActivityItem(assignment Assignment, workItem WorkItem, role Role) ProjectActivityItem {
	return ProjectActivityItem{
		Bucket:           projectActivityBucket(assignment.Status),
		AssignmentID:     assignment.ID,
		WorkItemID:       assignment.WorkItemID,
		WorkItemTitle:    workItem.Title,
		RoleID:           assignment.RoleID,
		RoleName:         role.Name,
		RootID:           assignment.RootID,
		Status:           strings.TrimSpace(assignment.Status),
		ExecutionMode:    assignment.ExecutionMode,
		DesiredAgentKind: assignment.DesiredAgent.Kind,
		ExecutionRef:     assignment.ExecutionRef,
		CreatedAt:        assignment.CreatedAt,
		UpdatedAt:        assignment.UpdatedAt,
	}
}

func projectActivityBucket(status string) string {
	switch strings.TrimSpace(status) {
	case AssignmentQueued, AssignmentClaimed, AssignmentRunning, AssignmentReview:
		return ProjectActivityBucketActive
	case AssignmentFailed, AssignmentCancelled:
		return ProjectActivityBucketBlocked
	case AssignmentCompleted:
		return ProjectActivityBucketCompleted
	default:
		return ProjectActivityBucketOther
	}
}

func countProjectActivityStatus(counts *ProjectActivityCounts, status string) {
	switch strings.TrimSpace(status) {
	case AssignmentQueued:
		counts.Queued++
	case AssignmentClaimed:
		counts.Claimed++
	case AssignmentRunning:
		counts.Running++
	case AssignmentReview:
		counts.AwaitingReview++
	case AssignmentCompleted:
		counts.Completed++
	case AssignmentFailed:
		counts.Failed++
	case AssignmentCancelled:
		counts.Cancelled++
	default:
		counts.Other++
	}
}

func sortProjectActivityItems(items []ProjectActivityItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].AssignmentID < items[j].AssignmentID
	})
}
