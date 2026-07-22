package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Service) ProjectOperationsBrief(ctx context.Context, projectID string) (ProjectOperationsBrief, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ProjectOperationsBrief{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if _, err := s.store.GetProject(ctx, projectID); err != nil {
		return ProjectOperationsBrief{}, err
	}
	workItems, err := s.store.ListWorkItems(ctx, projectID)
	if err != nil {
		return ProjectOperationsBrief{}, err
	}
	assignments, err := s.store.ListAssignments(ctx, projectID)
	if err != nil {
		return ProjectOperationsBrief{}, err
	}
	memoryCandidates, err := s.store.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: projectID})
	if err != nil {
		return ProjectOperationsBrief{}, err
	}
	assignmentsByWorkItem := make(map[string][]Assignment)
	for _, assignment := range assignments {
		assignmentsByWorkItem[assignment.WorkItemID] = append(assignmentsByWorkItem[assignment.WorkItemID], assignment)
	}
	now := s.now()
	brief := ProjectOperationsBrief{
		ProjectID: projectID,
		Status:    ProjectOperationsStatusClear,
		Title:     "No project operations need attention",
		Detail:    "There are no active assignments, pending memory candidates, review follow-ups, or closeout blockers.",
		Counts: ProjectOperationsCounts{
			WorkItems:   len(workItems),
			Assignments: len(assignments),
		},
		CreatedAt: now,
	}
	for _, candidate := range memoryCandidates {
		if candidate.Status != MemoryCandidatePending {
			continue
		}
		brief.Counts.PendingMemoryCandidates++
		brief.Items = append(brief.Items, ProjectOperationItem{
			Kind:              ProjectOperationKindMemoryCandidate,
			Severity:          ProjectOperationSeverityAction,
			Status:            candidate.Status,
			Title:             "Review memory candidate: " + firstNonEmpty(candidate.Title, candidate.ID),
			Detail:            "Promote or reject the proposed durable memory entry.",
			MemoryCandidateID: candidate.ID,
			UpdatedAt:         candidate.UpdatedAt,
		})
	}
	for _, workItem := range workItems {
		if !workItemClosed(workItem.Status) {
			brief.Counts.OpenWorkItems++
		}
		workAssignments := assignmentsByWorkItem[workItem.ID]
		readiness, err := s.evaluateWorkItemOperationsReadiness(ctx, workItem, workAssignments)
		if err != nil {
			return ProjectOperationsBrief{}, err
		}
		for _, assignment := range workAssignments {
			item, ok := projectAssignmentOperationItem(workItem, assignment, now)
			if !ok {
				continue
			}
			switch item.Severity {
			case ProjectOperationSeverityBlocked:
				brief.Counts.BlockedAssignments++
			case ProjectOperationSeverityActive:
				brief.Counts.ActiveAssignments++
			}
			brief.Items = append(brief.Items, item)
		}
		for _, assignmentID := range readiness.MissingEvidenceAssignmentIDs {
			brief.Counts.MissingEvidence++
			brief.Items = append(brief.Items, ProjectOperationItem{
				Kind:         ProjectOperationKindMissingEvidence,
				Severity:     ProjectOperationSeverityBlocked,
				Status:       "missing",
				Title:        "Record evidence for " + workItem.Title,
				Detail:       "A completed assignment must have evidence before closeout.",
				WorkItemID:   workItem.ID,
				AssignmentID: assignmentID,
				UpdatedAt:    workItem.UpdatedAt,
			})
		}
		for _, followUp := range readiness.ReviewFollowUps {
			brief.Counts.ReviewFollowUps++
			brief.Items = append(brief.Items, ProjectOperationItem{
				Kind:         ProjectOperationKindReviewFollowUp,
				Severity:     ProjectOperationSeverityBlocked,
				Status:       followUp.Status,
				Title:        "Triage review follow-up: " + firstNonEmpty(followUp.Title, followUp.ArtifactID),
				Detail:       followUp.Blocker,
				WorkItemID:   workItem.ID,
				AssignmentID: followUp.ReviewedAssignmentID,
				ArtifactID:   followUp.ArtifactID,
				UpdatedAt:    workItem.UpdatedAt,
			})
		}
		for _, handoffID := range readiness.OpenHandoffIDs {
			brief.Counts.OpenHandoffs++
			brief.Items = append(brief.Items, ProjectOperationItem{
				Kind:       ProjectOperationKindHandoff,
				Severity:   ProjectOperationSeverityBlocked,
				Status:     HandoffStatusOpen,
				Title:      "Resolve open handoff for " + workItem.Title,
				Detail:     "The handoff must be accepted, dismissed, or superseded before closeout.",
				WorkItemID: workItem.ID,
				ArtifactID: handoffID,
				UpdatedAt:  workItem.UpdatedAt,
			})
		}
		if !workItemClosed(workItem.Status) && readiness.Ready && readiness.AssignmentCount > 0 {
			brief.Counts.CloseoutReady++
			brief.Items = append(brief.Items, ProjectOperationItem{
				Kind:       ProjectOperationKindCloseoutReady,
				Severity:   ProjectOperationSeverityReady,
				Status:     readiness.Status,
				Title:      "Close out " + workItem.Title,
				Detail:     readiness.Detail,
				WorkItemID: workItem.ID,
				UpdatedAt:  workItem.UpdatedAt,
			})
		}
		if !workItemClosed(workItem.Status) && len(workAssignments) == 0 {
			brief.Items = append(brief.Items, ProjectOperationItem{
				Kind:       ProjectOperationKindWorkItem,
				Severity:   ProjectOperationSeverityInfo,
				Status:     workItem.Status,
				Title:      "Plan or assign " + workItem.Title,
				Detail:     "This open work item has no assignments yet.",
				WorkItemID: workItem.ID,
				UpdatedAt:  workItem.UpdatedAt,
			})
		}
	}
	sort.SliceStable(brief.Items, func(i, j int) bool {
		left := brief.Items[i]
		right := brief.Items[j]
		leftRank := projectOperationRank(left)
		rightRank := projectOperationRank(right)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if !left.UpdatedAt.Equal(right.UpdatedAt) {
			return left.UpdatedAt.After(right.UpdatedAt)
		}
		return left.Title < right.Title
	})
	if len(brief.Items) > 0 {
		brief.Status = ProjectOperationsStatusAttention
		brief.Title = "Project operations need attention"
		brief.Detail = fmt.Sprintf("%d project operation item%s need%s operator attention.", len(brief.Items), pluralSuffix(len(brief.Items)), pluralVerb(len(brief.Items)))
		next := brief.Items[0]
		brief.Next = &next
	}
	return brief, nil
}

func (s *Service) evaluateWorkItemOperationsReadiness(ctx context.Context, workItem WorkItem, assignments []Assignment) (WorkItemCloseoutReadiness, error) {
	evidence, err := s.store.ListEvidence(ctx, workItem.ProjectID, workItem.ID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	reviews, err := s.store.ListReviews(ctx, workItem.ProjectID, workItem.ID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	handoffs, err := s.store.ListHandoffs(ctx, workItem.ProjectID, workItem.ID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	return EvaluateWorkItemCloseoutReadiness(workItem, assignments, evidence, reviews, handoffs), nil
}

func projectAssignmentOperationItem(workItem WorkItem, assignment Assignment, now time.Time) (ProjectOperationItem, bool) {
	status := assignmentCloseoutStatus(assignment)
	switch status {
	case AssignmentQueued:
		return ProjectOperationItem{
			Kind:         ProjectOperationKindAssignment,
			Severity:     ProjectOperationSeverityBlocked,
			Status:       status,
			Title:        "Start queued assignment for " + workItem.Title,
			Detail:       "The assignment is queued and needs an operator or compatible agent to claim it.",
			WorkItemID:   workItem.ID,
			AssignmentID: assignment.ID,
			UpdatedAt:    assignment.UpdatedAt,
		}, true
	case AssignmentAwaitingApproval:
		return ProjectOperationItem{
			Kind:         ProjectOperationKindAssignment,
			Severity:     ProjectOperationSeverityBlocked,
			Status:       status,
			Title:        "Resolve pending approval for " + workItem.Title,
			Detail:       "The assignment is paused until a pending approval is resolved on the executing host.",
			WorkItemID:   workItem.ID,
			AssignmentID: assignment.ID,
			UpdatedAt:    assignment.UpdatedAt,
		}, true
	case AssignmentFailed, AssignmentCancelled:
		return ProjectOperationItem{
			Kind:         ProjectOperationKindAssignment,
			Severity:     ProjectOperationSeverityBlocked,
			Status:       status,
			Title:        "Resolve " + status + " assignment for " + workItem.Title,
			Detail:       "The assignment must be retried, replaced, or explicitly accepted before closeout.",
			WorkItemID:   workItem.ID,
			AssignmentID: assignment.ID,
			UpdatedAt:    assignment.UpdatedAt,
		}, true
	case AssignmentClaimed:
		if assignment.Claim != nil && !assignment.Claim.ExpiresAt.IsZero() && !assignment.Claim.ExpiresAt.After(now) {
			return ProjectOperationItem{
				Kind:         ProjectOperationKindAssignment,
				Severity:     ProjectOperationSeverityBlocked,
				Status:       status,
				Title:        "Recover expired claim for " + workItem.Title,
				Detail:       "The pre-start claim expired. Reconcile any prepared host resources, then recover the assignment claim.",
				ActionKind:   ProjectOperationActionRecoverClaim,
				ActionLabel:  "Recover claim",
				WorkItemID:   workItem.ID,
				AssignmentID: assignment.ID,
				UpdatedAt:    assignment.UpdatedAt,
			}, true
		}
		fallthrough
	case AssignmentRunning, AssignmentReview:
		return ProjectOperationItem{
			Kind:         ProjectOperationKindAssignment,
			Severity:     ProjectOperationSeverityActive,
			Status:       status,
			Title:        "Continue assignment for " + workItem.Title,
			Detail:       "The assignment is not terminal yet.",
			WorkItemID:   workItem.ID,
			AssignmentID: assignment.ID,
			UpdatedAt:    assignment.UpdatedAt,
		}, true
	default:
		return ProjectOperationItem{}, false
	}
}

func projectOperationRank(item ProjectOperationItem) int {
	switch item.Kind {
	case ProjectOperationKindAssignment:
		if item.Severity == ProjectOperationSeverityBlocked {
			return 10
		}
		return 50
	case ProjectOperationKindMissingEvidence:
		return 20
	case ProjectOperationKindReviewFollowUp:
		return 30
	case ProjectOperationKindHandoff:
		return 35
	case ProjectOperationKindMemoryCandidate:
		return 40
	case ProjectOperationKindCloseoutReady:
		return 60
	case ProjectOperationKindWorkItem:
		return 70
	default:
		return 100
	}
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func pluralVerb(count int) string {
	if count == 1 {
		return "s"
	}
	return ""
}
