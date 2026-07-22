package core

import (
	"errors"
	"strings"
)

// ValidateAssignmentProgressStatus validates a fenced in-progress transition.
func ValidateAssignmentProgressStatus(status string) error {
	if !isProgressAssignmentStatus(status) {
		return errors.Join(ErrInvalid, errors.New("assignment status must be running, awaiting_approval, or awaiting_review"))
	}
	return nil
}

// ValidateAssignmentCompletionStatus validates a fenced completion transition.
func ValidateAssignmentCompletionStatus(status string) error {
	if !isCompletionAssignmentStatus(status) {
		return errors.Join(ErrInvalid, errors.New("assignment status must be completed, failed, cancelled, or awaiting_review"))
	}
	return nil
}

// ValidateAssignmentClaimState rejects persisted claim/fence combinations that
// cannot be produced by Cairnline's assignment lifecycle. Built-in stores call
// it before writes, and snapshot import preflights every assignment before
// mutating any store.
func ValidateAssignmentClaimState(assignment Assignment) error {
	claim := assignment.Claim
	if claim == nil {
		return nil
	}
	if strings.TrimSpace(claim.ID) == "" {
		return errors.Join(ErrInvalid, errors.New("assignment claim id is required"))
	}
	if claim.ID != strings.TrimSpace(claim.ID) {
		return errors.Join(ErrInvalid, errors.New("assignment claim id must not contain surrounding whitespace"))
	}
	if strings.TrimSpace(assignment.ClaimedBy) == "" {
		return errors.Join(ErrInvalid, errors.New("assignment claimed_by is required when claim is present"))
	}
	if assignment.ClaimedBy != strings.TrimSpace(assignment.ClaimedBy) {
		return errors.Join(ErrInvalid, errors.New("assignment claimed_by must not contain surrounding whitespace"))
	}
	if claim.AcquiredAt.IsZero() {
		return errors.Join(ErrInvalid, errors.New("assignment claim acquired_at is required"))
	}
	if assignment.Status == AssignmentClaimed {
		if claim.ExpiresAt.IsZero() {
			return errors.Join(ErrInvalid, errors.New("claimed assignment claim expires_at is required"))
		}
		if !claim.ExpiresAt.After(claim.AcquiredAt) {
			return errors.Join(ErrInvalid, errors.New("assignment claim expires_at must be after acquired_at"))
		}
		return nil
	}
	switch assignment.Status {
	case AssignmentQueued:
		return errors.Join(ErrInvalid, errors.New("queued assignment cannot retain a claim fence"))
	case AssignmentRunning, AssignmentAwaitingApproval, AssignmentReview, AssignmentCompleted, AssignmentFailed, AssignmentCancelled:
		// These lifecycle states retain the fence after retiring the pre-start
		// reservation expiry.
	default:
		return errors.Join(ErrInvalid, errors.New("assignment claim fence has an unsupported status"))
	}
	if !claim.ExpiresAt.IsZero() {
		return errors.Join(ErrInvalid, errors.New("assignment claim expires_at must be empty after claimed status"))
	}
	return nil
}
