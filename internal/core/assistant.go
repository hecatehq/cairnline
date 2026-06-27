package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) AssistantPropose(ctx context.Context, input AssistantProposal) (AssistantProposal, error) {
	proposal := normalizeAssistantProposal(input, s.now)
	if proposal.Title == "" {
		return AssistantProposal{}, errors.Join(ErrInvalid, errors.New("proposal title is required"))
	}
	if len(proposal.Actions) == 0 {
		return AssistantProposal{}, errors.Join(ErrInvalid, errors.New("proposal actions are required"))
	}
	for idx, action := range proposal.Actions {
		if err := validateAssistantAction(action); err != nil {
			return AssistantProposal{}, fmt.Errorf("action %d: %w", idx, err)
		}
	}
	return proposal, nil
}

func (s *Service) ListAssistantProposals(ctx context.Context, projectID string) ([]AssistantProposalRecord, error) {
	return s.store.ListAssistantProposals(ctx, strings.TrimSpace(projectID))
}

func (s *Service) GetAssistantProposal(ctx context.Context, id string) (AssistantProposalRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AssistantProposalRecord{}, errors.Join(ErrInvalid, errors.New("proposal id is required"))
	}
	return s.store.GetAssistantProposal(ctx, id)
}

func (s *Service) CreateAssistantProposal(ctx context.Context, input AssistantProposal) (AssistantProposalRecord, error) {
	proposal, err := s.AssistantPropose(ctx, input)
	if err != nil {
		return AssistantProposalRecord{}, err
	}
	record := normalizeAssistantProposalRecord(AssistantProposalRecord{
		ID:        proposal.ID,
		ProjectID: proposal.ProjectID,
		Source:    proposal.Source,
		Proposal:  proposal,
		Status:    AssistantProposalStatusProposed,
	}, s.now())
	return s.store.CreateAssistantProposal(ctx, record)
}

func (s *Service) ApplyAssistantProposal(ctx context.Context, input AssistantProposal, confirmed bool) (AssistantApplyResult, error) {
	proposal, err := s.AssistantPropose(ctx, input)
	if err != nil {
		return AssistantApplyResult{}, err
	}
	if proposal.RequiresConfirmation && !confirmed {
		return AssistantApplyResult{
			ProposalID:       proposal.ID,
			Status:           AssistantApplyStatusNeedsConfirm,
			Confirmed:        false,
			TotalActionCount: len(proposal.Actions),
		}, ErrConflict
	}
	result := AssistantApplyResult{
		ProposalID:       proposal.ID,
		Status:           AssistantApplyStatusApplied,
		Confirmed:        confirmed,
		TotalActionCount: len(proposal.Actions),
	}
	for idx, action := range proposal.Actions {
		applied, err := s.applyAssistantAction(ctx, action)
		result.Actions = append(result.Actions, applied)
		if err != nil {
			failedIndex := idx
			applied.Status = AssistantApplyStatusRejected
			applied.Error = err.Error()
			result.Actions[len(result.Actions)-1] = applied
			result.Status = AssistantApplyStatusPartial
			if result.AppliedActionCount == 0 {
				result.Status = AssistantApplyStatusRejected
			}
			result.FailedActionIndex = &failedIndex
			return result, err
		}
		result.AppliedActionCount++
	}
	result.Applied = true
	return result, nil
}

func (s *Service) ApplyAssistantProposalRecord(ctx context.Context, id string, confirmed bool) (AssistantApplyResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AssistantApplyResult{}, errors.Join(ErrInvalid, errors.New("proposal id is required"))
	}

	s.assistantMu.Lock()
	defer s.assistantMu.Unlock()

	record, err := s.store.GetAssistantProposal(ctx, id)
	if err != nil {
		return AssistantApplyResult{}, err
	}
	if record.LatestResult != nil && record.LatestResult.Applied {
		return cloneAssistantApplyResult(*record.LatestResult), ErrConflict
	}
	if record.Status == AssistantProposalStatusApplied {
		if record.LatestResult != nil {
			return cloneAssistantApplyResult(*record.LatestResult), ErrConflict
		}
		return AssistantApplyResult{ProposalID: record.ID, Status: AssistantApplyStatusApplied, Applied: true}, ErrConflict
	}

	result, applyErr := s.ApplyAssistantProposal(ctx, record.Proposal, confirmed)
	attempt := assistantApplyAttemptForResult(newID("paatt"), confirmed, result, applyErr, s.now())
	record = applyAssistantResultToProposalRecord(record, result, s.now())
	record.ApplyAttempts = append(record.ApplyAttempts, attempt)
	if _, err := s.store.UpdateAssistantProposal(ctx, record); err != nil {
		return result, err
	}
	return result, applyErr
}

func normalizeAssistantProposal(input AssistantProposal, now func() time.Time) AssistantProposal {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = now()
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = AssistantProposalSourceAPI
	}
	proposal := AssistantProposal{
		ID:                   firstNonEmpty(strings.TrimSpace(input.ID), newID("prop")),
		ProjectID:            strings.TrimSpace(input.ProjectID),
		Title:                strings.TrimSpace(input.Title),
		Summary:              strings.TrimSpace(input.Summary),
		Source:               source,
		RequiresConfirmation: true,
		CreatedAt:            createdAt,
		Actions:              make([]AssistantAction, 0, len(input.Actions)),
	}
	for _, action := range input.Actions {
		proposal.Actions = append(proposal.Actions, normalizeAssistantAction(action))
	}
	return proposal
}

func normalizeAssistantProposalRecord(input AssistantProposalRecord, now time.Time) AssistantProposalRecord {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	proposal := cloneAssistantProposal(input.Proposal)
	if proposal.ID == "" {
		proposal.ID = strings.TrimSpace(input.ID)
	}
	if proposal.ProjectID == "" {
		proposal.ProjectID = strings.TrimSpace(input.ProjectID)
	}
	if proposal.Source == "" {
		proposal.Source = strings.TrimSpace(input.Source)
	}
	record := AssistantProposalRecord{
		ID:            firstNonEmpty(strings.TrimSpace(input.ID), proposal.ID, newID("prop")),
		ProjectID:     firstNonEmpty(strings.TrimSpace(input.ProjectID), proposal.ProjectID, assistantProposalProjectID(proposal)),
		Source:        firstNonEmpty(strings.TrimSpace(input.Source), proposal.Source, AssistantProposalSourceAPI),
		SourceID:      strings.TrimSpace(input.SourceID),
		Proposal:      proposal,
		Status:        firstNonEmpty(strings.TrimSpace(input.Status), AssistantProposalStatusProposed),
		LatestResult:  cloneAssistantApplyResultPtr(input.LatestResult),
		ApplyAttempts: cloneAssistantApplyAttempts(input.ApplyAttempts),
		CreatedAt:     input.CreatedAt,
		UpdatedAt:     input.UpdatedAt,
		AppliedAt:     cloneTimePtr(input.AppliedAt),
	}
	record.Proposal.ID = record.ID
	record.Proposal.ProjectID = record.ProjectID
	record.Proposal.Source = record.Source
	record.Proposal.RequiresConfirmation = true
	if record.Proposal.CreatedAt.IsZero() {
		record.Proposal.CreatedAt = now
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = record.Proposal.CreatedAt
	}
	if record.UpdatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
		record.UpdatedAt = now
	}
	if record.LatestResult != nil {
		record = applyAssistantResultToProposalRecord(record, *record.LatestResult, record.UpdatedAt)
	}
	return record
}

func normalizeAssistantAction(input AssistantAction) AssistantAction {
	return AssistantAction{
		Kind:            strings.TrimSpace(input.Kind),
		Title:           strings.TrimSpace(input.Title),
		Summary:         strings.TrimSpace(input.Summary),
		Target:          normalizeAssistantTarget(input.Target),
		Project:         cloneProjectPtr(input.Project),
		Role:            cloneRolePtr(input.Role),
		WorkItem:        cloneWorkItemPtr(input.WorkItem),
		Assignment:      cloneAssignmentPtr(input.Assignment),
		Evidence:        cloneEvidencePtr(input.Evidence),
		Review:          cloneReviewPtr(input.Review),
		Handoff:         cloneHandoffPtr(input.Handoff),
		MemoryCandidate: cloneMemoryCandidatePtr(input.MemoryCandidate),
	}
}

func normalizeAssistantTarget(input AssistantTarget) AssistantTarget {
	return AssistantTarget{
		ProjectID:    strings.TrimSpace(input.ProjectID),
		RoleID:       strings.TrimSpace(input.RoleID),
		WorkItemID:   strings.TrimSpace(input.WorkItemID),
		AssignmentID: strings.TrimSpace(input.AssignmentID),
		ArtifactID:   strings.TrimSpace(input.ArtifactID),
		HandoffID:    strings.TrimSpace(input.HandoffID),
	}
}

func validateAssistantAction(action AssistantAction) error {
	if action.Kind == "" {
		return errors.Join(ErrInvalid, errors.New("action kind is required"))
	}
	switch action.Kind {
	case AssistantActionCreateProject:
		return requireAssistantPayload(action.Project, "project")
	case AssistantActionUpdateProject:
		if err := requireAssistantPayload(action.Project, "project"); err != nil {
			return err
		}
		if strings.TrimSpace(action.Project.ID) == "" {
			return errors.Join(ErrInvalid, errors.New("project id is required"))
		}
	case AssistantActionCreateRole:
		return requireAssistantPayload(action.Role, "role")
	case AssistantActionUpdateRole:
		if err := requireAssistantPayload(action.Role, "role"); err != nil {
			return err
		}
		if strings.TrimSpace(action.Role.ID) == "" {
			return errors.Join(ErrInvalid, errors.New("role id is required"))
		}
	case AssistantActionCreateWorkItem:
		return requireAssistantPayload(action.WorkItem, "work_item")
	case AssistantActionUpdateWorkItem:
		if err := requireAssistantPayload(action.WorkItem, "work_item"); err != nil {
			return err
		}
		if strings.TrimSpace(action.WorkItem.ID) == "" {
			return errors.Join(ErrInvalid, errors.New("work item id is required"))
		}
	case AssistantActionCreateAssignment:
		if err := requireAssistantPayload(action.Assignment, "assignment"); err != nil {
			return err
		}
		if err := validateAssistantAssignment(action.Assignment); err != nil {
			return err
		}
	case AssistantActionCreateEvidence:
		return requireAssistantPayload(action.Evidence, "evidence")
	case AssistantActionCreateReview:
		return requireAssistantPayload(action.Review, "review")
	case AssistantActionCreateHandoff:
		return requireAssistantPayload(action.Handoff, "handoff")
	case AssistantActionUpdateHandoff:
		if err := requireAssistantPayload(action.Handoff, "handoff"); err != nil {
			return err
		}
		if strings.TrimSpace(action.Handoff.ID) == "" {
			return errors.Join(ErrInvalid, errors.New("handoff id is required"))
		}
	case AssistantActionCreateMemoryCandidate:
		return requireAssistantPayload(action.MemoryCandidate, "memory_candidate")
	default:
		return errors.Join(ErrInvalid, fmt.Errorf("unknown assistant action kind %q", action.Kind))
	}
	return nil
}

func validateAssistantProposalRecord(record AssistantProposalRecord) error {
	if strings.TrimSpace(record.ID) == "" {
		return errors.Join(ErrInvalid, errors.New("proposal id is required"))
	}
	if strings.TrimSpace(record.Proposal.ID) != record.ID {
		return errors.Join(ErrInvalid, errors.New("proposal record id mismatch"))
	}
	if _, err := (&Service{now: func() time.Time { return record.CreatedAt }}).AssistantPropose(context.Background(), record.Proposal); err != nil {
		return err
	}
	for idx, attempt := range record.ApplyAttempts {
		if err := validateAssistantApplyAttempt(attempt); err != nil {
			return fmt.Errorf("apply attempt %d: %w", idx, err)
		}
	}
	return nil
}

func validateAssistantApplyAttempt(attempt AssistantApplyAttempt) error {
	if strings.TrimSpace(attempt.ID) == "" {
		return errors.Join(ErrInvalid, errors.New("apply attempt id is required"))
	}
	if strings.TrimSpace(attempt.ProposalID) == "" {
		return errors.Join(ErrInvalid, errors.New("apply attempt proposal_id is required"))
	}
	if strings.TrimSpace(attempt.Status) == "" {
		return errors.Join(ErrInvalid, errors.New("apply attempt status is required"))
	}
	return nil
}

func requireAssistantPayload[T any](payload *T, name string) error {
	if payload == nil {
		return errors.Join(ErrInvalid, fmt.Errorf("%s payload is required", name))
	}
	return nil
}

func validateAssistantAssignment(assignment *Assignment) error {
	status := strings.TrimSpace(assignment.Status)
	if status != "" && status != AssignmentQueued {
		return errors.Join(ErrInvalid, errors.New("assistant-created assignments must be queued"))
	}
	if strings.TrimSpace(assignment.ClaimedBy) != "" ||
		strings.TrimSpace(assignment.ExecutionRef) != "" ||
		strings.TrimSpace(assignment.ContextSnapshotID) != "" {
		return errors.Join(ErrInvalid, errors.New("assistant-created assignments cannot bind execution state"))
	}
	return nil
}

func (s *Service) applyAssistantAction(ctx context.Context, action AssistantAction) (AssistantActionResult, error) {
	result := AssistantActionResult{
		Kind:   action.Kind,
		Status: AssistantApplyStatusApplied,
	}
	switch action.Kind {
	case AssistantActionCreateProject:
		item, err := s.CreateProject(ctx, *action.Project)
		result.ProjectID = item.ID
		return result, err
	case AssistantActionUpdateProject:
		item, err := s.UpdateProject(ctx, *action.Project)
		result.ProjectID = item.ID
		return result, err
	case AssistantActionCreateRole:
		item, err := s.CreateRole(ctx, *action.Role)
		result.ProjectID = item.ProjectID
		result.RoleID = item.ID
		return result, err
	case AssistantActionUpdateRole:
		item, err := s.UpdateRole(ctx, *action.Role)
		result.ProjectID = item.ProjectID
		result.RoleID = item.ID
		return result, err
	case AssistantActionCreateWorkItem:
		item, err := s.CreateWorkItem(ctx, *action.WorkItem)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.ID
		return result, err
	case AssistantActionUpdateWorkItem:
		item, err := s.UpdateWorkItem(ctx, *action.WorkItem)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.ID
		return result, err
	case AssistantActionCreateAssignment:
		item, err := s.CreateAssignment(ctx, *action.Assignment)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.WorkItemID
		result.AssignmentID = item.ID
		return result, err
	case AssistantActionCreateEvidence:
		item, err := s.CreateEvidence(ctx, *action.Evidence)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.WorkItemID
		result.AssignmentID = item.AssignmentID
		result.ArtifactID = item.ID
		return result, err
	case AssistantActionCreateReview:
		item, err := s.CreateReview(ctx, *action.Review)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.WorkItemID
		result.AssignmentID = item.AssignmentID
		result.ArtifactID = item.ID
		return result, err
	case AssistantActionCreateHandoff:
		item, err := s.CreateHandoff(ctx, *action.Handoff)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.WorkItemID
		result.AssignmentID = firstNonEmpty(item.TargetAssignmentID, item.SourceAssignmentID)
		result.HandoffID = item.ID
		return result, err
	case AssistantActionUpdateHandoff:
		item, err := s.UpdateHandoff(ctx, *action.Handoff)
		result.ProjectID = item.ProjectID
		result.WorkItemID = item.WorkItemID
		result.AssignmentID = firstNonEmpty(item.TargetAssignmentID, item.SourceAssignmentID)
		result.HandoffID = item.ID
		return result, err
	case AssistantActionCreateMemoryCandidate:
		item, err := s.CreateMemoryCandidate(ctx, *action.MemoryCandidate)
		result.ProjectID = item.ProjectID
		result.MemoryCandidateID = item.ID
		return result, err
	default:
		return result, errors.Join(ErrInvalid, fmt.Errorf("unknown assistant action kind %q", action.Kind))
	}
}

func cloneProjectPtr(input *Project) *Project {
	if input == nil {
		return nil
	}
	item := *input
	item.Roots = append([]Root(nil), input.Roots...)
	item.ContextSources = append([]Source(nil), input.ContextSources...)
	return &item
}

func cloneRolePtr(input *Role) *Role {
	if input == nil {
		return nil
	}
	item := *input
	item.DefaultSkillIDs = append([]string(nil), input.DefaultSkillIDs...)
	return &item
}

func cloneWorkItemPtr(input *WorkItem) *WorkItem {
	if input == nil {
		return nil
	}
	item := *input
	item.ReviewerRoleIDs = append([]string(nil), input.ReviewerRoleIDs...)
	return &item
}

func cloneAssignmentPtr(input *Assignment) *Assignment {
	if input == nil {
		return nil
	}
	item := *input
	item.DesiredAgent.SkillIDs = append([]string(nil), input.DesiredAgent.SkillIDs...)
	return &item
}

func cloneEvidencePtr(input *Evidence) *Evidence {
	if input == nil {
		return nil
	}
	item := *input
	return &item
}

func cloneReviewPtr(input *Review) *Review {
	if input == nil {
		return nil
	}
	item := *input
	return &item
}

func cloneHandoffPtr(input *Handoff) *Handoff {
	if input == nil {
		return nil
	}
	item := *input
	item.LinkedArtifactIDs = append([]string(nil), input.LinkedArtifactIDs...)
	item.LinkedMemoryIDs = append([]string(nil), input.LinkedMemoryIDs...)
	item.ContextRefs = append([]string(nil), input.ContextRefs...)
	return &item
}

func cloneMemoryCandidatePtr(input *MemoryCandidate) *MemoryCandidate {
	if input == nil {
		return nil
	}
	item := *input
	item.SourceRefs = append([]MemoryCandidateSourceRef(nil), input.SourceRefs...)
	return &item
}

func applyAssistantResultToProposalRecord(record AssistantProposalRecord, result AssistantApplyResult, now time.Time) AssistantProposalRecord {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result = cloneAssistantApplyResult(result)
	if result.ProposalID == "" {
		result.ProposalID = record.ID
	}
	record.LatestResult = &result
	record.Status = assistantProposalStatusForResult(result)
	record.UpdatedAt = now
	if result.Applied {
		appliedAt := now
		record.AppliedAt = &appliedAt
	}
	return record
}

func assistantProposalStatusForResult(result AssistantApplyResult) string {
	switch result.Status {
	case AssistantApplyStatusNeedsConfirm:
		return AssistantProposalStatusNeedsConfirm
	case AssistantApplyStatusApplied:
		return AssistantProposalStatusApplied
	case AssistantApplyStatusPartial:
		return AssistantProposalStatusPartial
	case AssistantApplyStatusRejected:
		return AssistantProposalStatusRejected
	default:
		if result.Applied {
			return AssistantProposalStatusApplied
		}
		if result.Status != "" {
			return result.Status
		}
		return AssistantProposalStatusProposed
	}
}

func assistantApplyAttemptForResult(id string, confirmed bool, result AssistantApplyResult, err error, now time.Time) AssistantApplyAttempt {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	attempt := AssistantApplyAttempt{
		ID:         strings.TrimSpace(id),
		ProposalID: strings.TrimSpace(result.ProposalID),
		Status:     strings.TrimSpace(result.Status),
		Confirmed:  confirmed,
		Result:     cloneAssistantApplyResult(result),
		CreatedAt:  now,
	}
	if err != nil {
		attempt.ErrorMessage = err.Error()
	}
	if attempt.Status == "" {
		attempt.Status = AssistantApplyStatusRejected
	}
	return attempt
}

func assistantProposalProjectID(proposal AssistantProposal) string {
	if projectID := strings.TrimSpace(proposal.ProjectID); projectID != "" {
		return projectID
	}
	for _, action := range proposal.Actions {
		if projectID := strings.TrimSpace(action.Target.ProjectID); projectID != "" {
			return projectID
		}
		switch action.Kind {
		case AssistantActionCreateProject, AssistantActionUpdateProject:
			if action.Project != nil && strings.TrimSpace(action.Project.ID) != "" {
				return strings.TrimSpace(action.Project.ID)
			}
		case AssistantActionCreateRole, AssistantActionUpdateRole:
			if action.Role != nil && strings.TrimSpace(action.Role.ProjectID) != "" {
				return strings.TrimSpace(action.Role.ProjectID)
			}
		case AssistantActionCreateWorkItem, AssistantActionUpdateWorkItem:
			if action.WorkItem != nil && strings.TrimSpace(action.WorkItem.ProjectID) != "" {
				return strings.TrimSpace(action.WorkItem.ProjectID)
			}
		case AssistantActionCreateAssignment:
			if action.Assignment != nil && strings.TrimSpace(action.Assignment.ProjectID) != "" {
				return strings.TrimSpace(action.Assignment.ProjectID)
			}
		case AssistantActionCreateEvidence:
			if action.Evidence != nil && strings.TrimSpace(action.Evidence.ProjectID) != "" {
				return strings.TrimSpace(action.Evidence.ProjectID)
			}
		case AssistantActionCreateReview:
			if action.Review != nil && strings.TrimSpace(action.Review.ProjectID) != "" {
				return strings.TrimSpace(action.Review.ProjectID)
			}
		case AssistantActionCreateHandoff, AssistantActionUpdateHandoff:
			if action.Handoff != nil && strings.TrimSpace(action.Handoff.ProjectID) != "" {
				return strings.TrimSpace(action.Handoff.ProjectID)
			}
		case AssistantActionCreateMemoryCandidate:
			if action.MemoryCandidate != nil && strings.TrimSpace(action.MemoryCandidate.ProjectID) != "" {
				return strings.TrimSpace(action.MemoryCandidate.ProjectID)
			}
		}
	}
	return ""
}

func cloneAssistantProposal(input AssistantProposal) AssistantProposal {
	out := input
	out.Actions = cloneAssistantActions(input.Actions)
	return out
}

func cloneAssistantProposalRecord(input AssistantProposalRecord) AssistantProposalRecord {
	out := input
	out.Proposal = cloneAssistantProposal(input.Proposal)
	out.LatestResult = cloneAssistantApplyResultPtr(input.LatestResult)
	out.ApplyAttempts = cloneAssistantApplyAttempts(input.ApplyAttempts)
	out.AppliedAt = cloneTimePtr(input.AppliedAt)
	return out
}

func cloneAssistantActions(input []AssistantAction) []AssistantAction {
	if input == nil {
		return nil
	}
	out := make([]AssistantAction, len(input))
	for idx, action := range input {
		out[idx] = normalizeAssistantAction(action)
	}
	return out
}

func cloneAssistantApplyResult(input AssistantApplyResult) AssistantApplyResult {
	out := input
	out.FailedActionIndex = cloneIntPtr(input.FailedActionIndex)
	if input.Actions != nil {
		out.Actions = make([]AssistantActionResult, len(input.Actions))
		copy(out.Actions, input.Actions)
	}
	return out
}

func cloneAssistantApplyResultPtr(input *AssistantApplyResult) *AssistantApplyResult {
	if input == nil {
		return nil
	}
	out := cloneAssistantApplyResult(*input)
	return &out
}

func cloneAssistantApplyAttempts(input []AssistantApplyAttempt) []AssistantApplyAttempt {
	if input == nil {
		return nil
	}
	out := make([]AssistantApplyAttempt, len(input))
	for idx, attempt := range input {
		out[idx] = attempt
		out[idx].Result = cloneAssistantApplyResult(attempt.Result)
	}
	return out
}

func cloneTimePtr(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	out := input.UTC()
	return &out
}

func cloneIntPtr(input *int) *int {
	if input == nil {
		return nil
	}
	out := *input
	return &out
}
