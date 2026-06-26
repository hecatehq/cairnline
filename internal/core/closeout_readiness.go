package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) WorkItemCloseoutReadiness(ctx context.Context, projectID, workItemID string) (WorkItemCloseoutReadiness, error) {
	projectID = strings.TrimSpace(projectID)
	workItemID = strings.TrimSpace(workItemID)
	if projectID == "" {
		return WorkItemCloseoutReadiness{}, errors.Join(ErrInvalid, errors.New("project_id is required"))
	}
	if workItemID == "" {
		return WorkItemCloseoutReadiness{}, errors.Join(ErrInvalid, errors.New("work_item_id is required"))
	}
	workItem, err := s.store.GetWorkItem(ctx, projectID, workItemID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	assignments, err := s.store.ListAssignments(ctx, projectID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	assignments = filterAssignmentsByWorkItem(assignments, workItemID)
	evidence, err := s.store.ListEvidence(ctx, projectID, workItemID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	reviews, err := s.store.ListReviews(ctx, projectID, workItemID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	handoffs, err := s.store.ListHandoffs(ctx, projectID, workItemID)
	if err != nil {
		return WorkItemCloseoutReadiness{}, err
	}
	return EvaluateWorkItemCloseoutReadiness(workItem, assignments, evidence, reviews, handoffs), nil
}

func EvaluateWorkItemCloseoutReadiness(workItem WorkItem, assignments []Assignment, evidence []Evidence, reviews []Review, handoffs []Handoff) WorkItemCloseoutReadiness {
	readiness := WorkItemCloseoutReadiness{
		ProjectID:            workItem.ProjectID,
		WorkItemID:           workItem.ID,
		Status:               "ready",
		Title:                "Ready to mark done",
		Detail:               "Assignments, evidence, handoffs, and review follow-up are clear. The operator can mark this work item done.",
		AssignmentCount:      len(assignments),
		CompletedAssignments: 0,
	}
	assignmentsByID := assignmentsByID(assignments)
	closed := workItemClosed(workItem.Status)
	for _, assignment := range assignments {
		status := assignmentCloseoutStatus(assignment)
		if status != AssignmentCompleted {
			continue
		}
		readiness.CompletedAssignments++
		if !closed && !assignmentHasCloseoutEvidence(assignment, evidence) {
			readiness.MissingEvidenceAssignmentIDs = append(readiness.MissingEvidenceAssignmentIDs, assignment.ID)
		}
	}
	if closed {
		readiness.Status = "done"
		readiness.Title = "Work item is done"
		readiness.Detail = "This work item has already been marked done by the operator."
		return readiness
	}
	activeAssignments := countAssignments(assignments, func(status string) bool {
		return isActiveCloseoutAssignmentStatus(status)
	})
	failedAssignments := countAssignments(assignments, func(status string) bool {
		return status == AssignmentFailed
	})
	cancelledAssignments := countAssignments(assignments, func(status string) bool {
		return status == AssignmentCancelled
	})
	unresolvedAssignments := countAssignments(assignments, func(status string) bool {
		return !isActiveCloseoutAssignmentStatus(status) && status != AssignmentCompleted && status != AssignmentFailed && status != AssignmentCancelled
	})
	pendingHandoffs := 0
	for _, handoff := range handoffs {
		if strings.TrimSpace(handoff.Status) == HandoffStatusOpen {
			pendingHandoffs++
			readiness.OpenHandoffIDs = append(readiness.OpenHandoffIDs, handoff.ID)
		}
	}
	if activeAssignments > 0 {
		readiness.Blockers = append(readiness.Blockers, readinessPlural(activeAssignments, "assignment is still active", "assignments are still active"))
	}
	if failedAssignments > 0 {
		readiness.Blockers = append(readiness.Blockers, readinessPlural(failedAssignments, "assignment failed", "assignments failed"))
	}
	if cancelledAssignments > 0 {
		readiness.Blockers = append(readiness.Blockers, readinessPlural(cancelledAssignments, "assignment was cancelled", "assignments were cancelled"))
	}
	if unresolvedAssignments > 0 {
		readiness.Blockers = append(readiness.Blockers, readinessPlural(unresolvedAssignments, "assignment is not complete", "assignments are not complete"))
	}
	if pendingHandoffs > 0 {
		readiness.Blockers = append(readiness.Blockers, readinessPlural(pendingHandoffs, "handoff is open", "handoffs are open"))
	}
	if len(readiness.MissingEvidenceAssignmentIDs) > 0 {
		readiness.Blockers = append(readiness.Blockers, readinessPlural(len(readiness.MissingEvidenceAssignmentIDs), "completed assignment is missing evidence", "completed assignments are missing evidence"))
	}
	if len(assignments) == 0 {
		readiness.Warnings = append(readiness.Warnings, "No assignments are linked to this work item; closeout is manual.")
	}
	for _, review := range reviews {
		blocker := reviewFollowUpBlocker(review, handoffs, assignmentsByID)
		if blocker == "" {
			continue
		}
		readiness.ReviewFollowUpArtifactIDs = append(readiness.ReviewFollowUpArtifactIDs, review.ID)
		readiness.ReviewFollowUps = append(readiness.ReviewFollowUps, ReviewFollowUpReadiness{
			ArtifactID:           review.ID,
			Title:                firstNonEmpty(review.Title, review.ID),
			Status:               "needs_path",
			Blocker:              blocker,
			ReviewedAssignmentID: review.AssignmentID,
			ReviewVerdict:        review.Verdict,
			ReviewRisk:           review.Risk,
		})
		readiness.Blockers = append(readiness.Blockers, blocker)
	}
	readiness.ReviewFollowUpCount = len(readiness.ReviewFollowUpArtifactIDs)
	readiness.Blockers = uniqueReadinessStrings(readiness.Blockers)
	readiness.Warnings = uniqueReadinessStrings(readiness.Warnings)
	if len(readiness.Blockers) > 0 {
		readiness.Status = "blocked"
		readiness.Title = "Closeout is blocked"
		readiness.Detail = "Resolve the listed assignment, evidence, handoff, or review follow-up items before marking this work done."
	}
	readiness.Ready = readiness.Status == "ready"
	return readiness
}

func filterAssignmentsByWorkItem(assignments []Assignment, workItemID string) []Assignment {
	out := make([]Assignment, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.WorkItemID == workItemID {
			out = append(out, assignment)
		}
	}
	return out
}

func assignmentsByID(assignments []Assignment) map[string]Assignment {
	out := make(map[string]Assignment, len(assignments))
	for _, assignment := range assignments {
		out[assignment.ID] = assignment
	}
	return out
}

func assignmentCloseoutStatus(assignment Assignment) string {
	return strings.TrimSpace(assignment.Status)
}

func isActiveCloseoutAssignmentStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case AssignmentQueued, AssignmentClaimed, AssignmentRunning, AssignmentReview:
		return true
	default:
		return false
	}
}

func assignmentHasCloseoutEvidence(assignment Assignment, evidence []Evidence) bool {
	for _, item := range evidence {
		assignmentID := strings.TrimSpace(item.AssignmentID)
		if assignmentID == "" || assignmentID == assignment.ID {
			return true
		}
	}
	return false
}

func countAssignments(assignments []Assignment, predicate func(string) bool) int {
	count := 0
	for _, assignment := range assignments {
		if predicate(assignmentCloseoutStatus(assignment)) {
			count++
		}
	}
	return count
}

func workItemClosed(status string) bool {
	switch strings.TrimSpace(status) {
	case WorkStatusDone, "cancelled":
		return true
	default:
		return false
	}
}

func reviewRequiresFollowUp(review Review) bool {
	switch strings.TrimSpace(review.Verdict) {
	case ReviewVerdictConcerns, ReviewVerdictBlocked:
		return true
	default:
		return false
	}
}

func reviewFollowUpBlocker(review Review, handoffs []Handoff, assignments map[string]Assignment) string {
	if !reviewRequiresFollowUp(review) {
		return ""
	}
	title := firstNonEmpty(review.Title, review.ID)
	linked := make([]Handoff, 0)
	for _, handoff := range handoffs {
		for _, artifactID := range handoff.LinkedArtifactIDs {
			if strings.TrimSpace(artifactID) == review.ID {
				linked = append(linked, handoff)
				break
			}
		}
	}
	if len(linked) == 0 {
		return fmt.Sprintf("Review follow-up %q is not triaged", title)
	}
	hasTargetAssignment := false
	hasCompletedTarget := false
	hasDismissedOrSuperseded := false
	for _, handoff := range linked {
		switch strings.TrimSpace(handoff.Status) {
		case HandoffStatusOpen:
			return fmt.Sprintf("Review follow-up %q has an open handoff", title)
		case HandoffStatusDismissed, HandoffStatusSuperseded:
			hasDismissedOrSuperseded = true
		}
		targetAssignmentID := strings.TrimSpace(handoff.TargetAssignmentID)
		if targetAssignmentID == "" {
			continue
		}
		hasTargetAssignment = true
		if assignment, ok := assignments[targetAssignmentID]; ok && assignmentCloseoutStatus(assignment) == AssignmentCompleted {
			hasCompletedTarget = true
		}
	}
	if hasCompletedTarget {
		return ""
	}
	if hasTargetAssignment {
		return fmt.Sprintf("Review follow-up %q assignment is not completed", title)
	}
	if hasDismissedOrSuperseded {
		return ""
	}
	return fmt.Sprintf("Review follow-up %q is not triaged", title)
}

func readinessPlural(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func uniqueReadinessStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
