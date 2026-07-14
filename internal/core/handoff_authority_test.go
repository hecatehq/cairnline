package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type handoffAuthorityFixture struct {
	ctx        context.Context
	store      *MemoryStore
	service    *Service
	project    Project
	role       Role
	sourceWork WorkItem
	targetWork WorkItem
	handoff    Handoff
	clock      *time.Time
}

func newHandoffAuthorityFixture(t *testing.T, rooted bool, executionMode string) *handoffAuthorityFixture {
	t.Helper()

	ctx := context.Background()
	store := NewMemoryStore()
	service := NewService(store)
	clock := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }

	projectInput := Project{ID: "proj_handoff_authority", Name: "Handoff authority"}
	targetRootID := ""
	if rooted {
		targetRootID = "root_target"
		projectInput.Roots = []Root{{ID: targetRootID, Path: "/workspace/target", Kind: "workspace", Active: true}}
		projectInput.DefaultRootID = targetRootID
	}
	project, err := service.CreateProject(ctx, projectInput)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{
		ID:                   "role_follow_up",
		ProjectID:            project.ID,
		Name:                 "Follow-up owner",
		DefaultExecutionMode: executionMode,
		DefaultSkillIDs:      []string{"skill_review", "skill_summary"},
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	sourceWork, err := service.CreateWorkItem(ctx, WorkItem{
		ID:        "work_source",
		ProjectID: project.ID,
		Title:     "Source work",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem(source) error = %v", err)
	}
	targetWork, err := service.CreateWorkItem(ctx, WorkItem{
		ID:        "work_target",
		ProjectID: project.ID,
		Title:     "Target work",
		RootID:    targetRootID,
	})
	if err != nil {
		t.Fatalf("CreateWorkItem(target) error = %v", err)
	}
	handoff, err := service.CreateHandoff(ctx, Handoff{
		ID:                    "handoff_authority",
		ProjectID:             project.ID,
		WorkItemID:            sourceWork.ID,
		ToRoleID:              role.ID,
		Title:                 "Continue the work",
		Body:                  "Carry the reviewed context into a follow-up.",
		RecommendedNextAction: "Create a focused follow-up.",
		LinkedArtifactIDs:     []string{"artifact_review"},
		LinkedMemoryIDs:       []string{"memory_decision"},
		ContextRefs:           []string{"context://brief"},
	})
	if err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}

	return &handoffAuthorityFixture{
		ctx:        ctx,
		store:      store,
		service:    service,
		project:    project,
		role:       role,
		sourceWork: sourceWork,
		targetWork: targetWork,
		handoff:    handoff,
		clock:      &clock,
	}
}

func (fixture *handoffAuthorityFixture) command(key string) AcceptHandoffWithFollowUpCommand {
	return AcceptHandoffWithFollowUpCommand{
		ProjectID:         fixture.project.ID,
		WorkItemID:        fixture.sourceWork.ID,
		HandoffID:         fixture.handoff.ID,
		ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
		IdempotencyKey:    key,
		Intent:            HandoffFollowUpIntentAcceptAndEnsure,
	}
}

func authorityString(value string) *string {
	return &value
}

func TestHandoffAuthorityRejectsStaleUpdateStatusAndDelete(t *testing.T) {
	fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
	original := fixture.handoff

	updated, err := fixture.service.PatchHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffUpdate{
			ExpectedUpdatedAt: original.UpdatedAt,
			Patch:             HandoffPatch{Body: authorityString("Authoritative content edit.")},
		},
	)
	if err != nil {
		t.Fatalf("PatchHandoff() error = %v", err)
	}
	if !updated.UpdatedAt.After(original.UpdatedAt) {
		t.Fatalf("updated_at = %s, want after %s", updated.UpdatedAt, original.UpdatedAt)
	}

	_, err = fixture.service.PatchHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffUpdate{
			ExpectedUpdatedAt: original.UpdatedAt,
			Patch:             HandoffPatch{Title: authorityString("Stale title edit")},
		},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("PatchHandoff(stale) error = %v, want ErrConflict", err)
	}

	_, err = fixture.service.UpdateHandoffStatus(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffStatusUpdate{ExpectedUpdatedAt: original.UpdatedAt, Status: HandoffStatusDismissed},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("UpdateHandoffStatus(stale) error = %v, want ErrConflict", err)
	}

	err = fixture.service.DeleteHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffDelete{ExpectedUpdatedAt: original.UpdatedAt},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteHandoff(stale) error = %v, want ErrConflict", err)
	}

	got, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, original.ID)
	if err != nil {
		t.Fatalf("GetHandoff() error = %v", err)
	}
	if got.Body != updated.Body || got.Title != original.Title || got.Status != HandoffStatusOpen {
		t.Fatalf("handoff after stale mutations = %+v, want only authoritative content edit", got)
	}
	if err := fixture.service.DeleteHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffDelete{ExpectedUpdatedAt: updated.UpdatedAt},
	); err != nil {
		t.Fatalf("DeleteHandoff(current) error = %v", err)
	}
}

func TestHandoffAuthorityTimestampsRemainMonotonicWithFixedAndBackwardClock(t *testing.T) {
	fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
	original := fixture.handoff
	*fixture.clock = original.UpdatedAt

	first, err := fixture.service.PatchHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffUpdate{
			ExpectedUpdatedAt: original.UpdatedAt,
			Patch:             HandoffPatch{Title: authorityString("First monotonic edit")},
		},
	)
	if err != nil {
		t.Fatalf("PatchHandoff(fixed clock) error = %v", err)
	}
	if want := original.UpdatedAt.Add(time.Nanosecond); !first.UpdatedAt.Equal(want) {
		t.Fatalf("first updated_at = %s, want %s", first.UpdatedAt, want)
	}

	*fixture.clock = original.UpdatedAt.Add(-24 * time.Hour)
	second, err := fixture.service.UpdateHandoffStatus(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffStatusUpdate{ExpectedUpdatedAt: first.UpdatedAt, Status: HandoffStatusAccepted},
	)
	if err != nil {
		t.Fatalf("UpdateHandoffStatus(backward clock) error = %v", err)
	}
	if want := first.UpdatedAt.Add(time.Nanosecond); !second.UpdatedAt.Equal(want) {
		t.Fatalf("second updated_at = %s, want %s", second.UpdatedAt, want)
	}
	if !second.StatusChangedAt.Equal(second.UpdatedAt) {
		t.Fatalf("status_changed_at = %s, want transition timestamp %s", second.StatusChangedAt, second.UpdatedAt)
	}

	_, err = fixture.service.PatchHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		original.ID,
		HandoffUpdate{
			ExpectedUpdatedAt: original.UpdatedAt,
			Patch:             HandoffPatch{Title: authorityString(original.Title)},
		},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("PatchHandoff(original token after two transitions) error = %v, want ErrConflict", err)
	}
}

func TestHandoffAuthorityMemoryStoreClonesSlicesAcrossBoundaries(t *testing.T) {
	t.Run("create result", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		fixture.handoff.LinkedArtifactIDs[0] = "mutated_create_result"

		got, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		if got.LinkedArtifactIDs[0] != "artifact_review" {
			t.Fatalf("stored linked_artifact_ids = %v, want isolated create result", got.LinkedArtifactIDs)
		}
	})

	t.Run("get result", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		got, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		got.LinkedMemoryIDs[0] = "mutated_get_result"

		again, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff(second) error = %v", err)
		}
		if again.LinkedMemoryIDs[0] != "memory_decision" {
			t.Fatalf("stored linked_memory_ids = %v, want isolated get result", again.LinkedMemoryIDs)
		}
	})

	t.Run("list result", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		items, err := fixture.service.ListHandoffs(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID)
		if err != nil {
			t.Fatalf("ListHandoffs() error = %v", err)
		}
		items[0].ContextRefs[0] = "mutated_list_result"

		again, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		if again.ContextRefs[0] != "context://brief" {
			t.Fatalf("stored context_refs = %v, want isolated list result", again.ContextRefs)
		}
	})

	t.Run("patch input and result", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		artifacts := []string{" artifact_one ", "artifact_two"}
		updated, err := fixture.service.PatchHandoff(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffUpdate{
				ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
				Patch:             HandoffPatch{LinkedArtifactIDs: &artifacts},
			},
		)
		if err != nil {
			t.Fatalf("PatchHandoff() error = %v", err)
		}
		artifacts[0] = "mutated_patch_input"
		updated.LinkedArtifactIDs[0] = "mutated_patch_result"

		got, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		if want := []string{"artifact_one", "artifact_two"}; !reflect.DeepEqual(got.LinkedArtifactIDs, want) {
			t.Fatalf("stored linked_artifact_ids = %v, want %v", got.LinkedArtifactIDs, want)
		}
	})
}

func TestHandoffAuthorityAcceptWithFollowUpCreatesQueuedRootlessAssignmentWithoutLaunch(t *testing.T) {
	fixture := newHandoffAuthorityFixture(t, false, ExecutionManual)
	command := fixture.command("rootless-follow-up")

	result, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
	}
	assertAuthorityFollowUp(t, fixture, result, fixture.sourceWork.ID, "", ExecutionManual, "human")
	if result.Outcome != HandoffFollowUpCreated || result.Replayed {
		t.Fatalf("result outcome = %q replayed=%t, want created fresh result", result.Outcome, result.Replayed)
	}
}

func TestHandoffAuthorityAcceptWithFollowUpUsesCrossWorkRootAndRoleDefaults(t *testing.T) {
	fixture := newHandoffAuthorityFixture(t, true, "")
	updated, err := fixture.service.PatchHandoff(
		fixture.ctx,
		fixture.project.ID,
		fixture.sourceWork.ID,
		fixture.handoff.ID,
		HandoffUpdate{
			ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
			Patch:             HandoffPatch{TargetWorkItemID: authorityString(fixture.targetWork.ID)},
		},
	)
	if err != nil {
		t.Fatalf("PatchHandoff(target work) error = %v", err)
	}
	fixture.handoff = updated

	result, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, fixture.command("cross-work-follow-up"))
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
	}
	assertAuthorityFollowUp(t, fixture, result, fixture.targetWork.ID, fixture.targetWork.RootID, ExecutionMCPPull, DesiredAgentAny)
	if result.Outcome != HandoffFollowUpCreated || result.Replayed {
		t.Fatalf("result outcome = %q replayed=%t, want created fresh result", result.Outcome, result.Replayed)
	}
}

func assertAuthorityFollowUp(t *testing.T, fixture *handoffAuthorityFixture, result HandoffFollowUpResult, workItemID, rootID, executionMode, desiredKind string) {
	t.Helper()

	if result.Handoff.Status != HandoffStatusAccepted {
		t.Fatalf("handoff status = %q, want %q", result.Handoff.Status, HandoffStatusAccepted)
	}
	if result.Handoff.TargetAssignmentID == "" || result.Handoff.TargetAssignmentID != result.Assignment.ID {
		t.Fatalf("handoff target_assignment_id = %q, assignment id = %q", result.Handoff.TargetAssignmentID, result.Assignment.ID)
	}
	if result.Handoff.TargetWorkItemID != workItemID || result.Assignment.WorkItemID != workItemID {
		t.Fatalf("handoff/assignment target work = %q/%q, want %q", result.Handoff.TargetWorkItemID, result.Assignment.WorkItemID, workItemID)
	}
	if !result.Handoff.UpdatedAt.After(fixture.handoff.UpdatedAt) {
		t.Fatalf("accepted handoff updated_at = %s, want after %s", result.Handoff.UpdatedAt, fixture.handoff.UpdatedAt)
	}
	if !result.Handoff.StatusChangedAt.Equal(result.Handoff.UpdatedAt) {
		t.Fatalf("status_changed_at = %s, want accepted timestamp %s", result.Handoff.StatusChangedAt, result.Handoff.UpdatedAt)
	}
	assignment := result.Assignment
	if assignment.ProjectID != fixture.project.ID || assignment.RoleID != fixture.role.ID {
		t.Fatalf("assignment authority = %+v, want project %q role %q", assignment, fixture.project.ID, fixture.role.ID)
	}
	if assignment.RootID != rootID || assignment.ExecutionMode != executionMode || assignment.Status != AssignmentQueued {
		t.Fatalf("assignment root/mode/status = %q/%q/%q, want %q/%q/%q", assignment.RootID, assignment.ExecutionMode, assignment.Status, rootID, executionMode, AssignmentQueued)
	}
	if assignment.DesiredAgent.Kind != desiredKind || !reflect.DeepEqual(assignment.DesiredAgent.SkillIDs, fixture.role.DefaultSkillIDs) {
		t.Fatalf("assignment desired agent = %+v, want kind %q skills %v", assignment.DesiredAgent, desiredKind, fixture.role.DefaultSkillIDs)
	}
	if assignment.ClaimedBy != "" || !assignment.ExecutionRef.Empty() || assignment.ContextSnapshotID != "" || !assignment.StartedAt.IsZero() || !assignment.CompletedAt.IsZero() {
		t.Fatalf("assignment contains launch state = %+v, want a portable unlaunched queue row", assignment)
	}
	items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != assignment.ID {
		t.Fatalf("assignments = %+v, want exactly created assignment %q", items, assignment.ID)
	}
}

func TestHandoffAuthorityAcceptWithFollowUpReplaysIdempotentlyAndRejectsKeyReuse(t *testing.T) {
	fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
	command := fixture.command("stable-retry-key")

	first, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(first) error = %v", err)
	}
	first.Handoff.LinkedArtifactIDs[0] = "mutated_first_result"
	first.Assignment.DesiredAgent.SkillIDs[0] = "mutated_first_assignment"

	replay, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(replay) error = %v", err)
	}
	if !replay.Replayed || replay.Outcome != HandoffFollowUpCreated {
		t.Fatalf("replay outcome = %q replayed=%t, want created receipt replay", replay.Outcome, replay.Replayed)
	}
	if replay.Assignment.ID != first.Assignment.ID || replay.Handoff.TargetAssignmentID != first.Assignment.ID {
		t.Fatalf("replay assignment/handoff = %+v / %+v, want original assignment %q", replay.Assignment, replay.Handoff, first.Assignment.ID)
	}
	if replay.Handoff.LinkedArtifactIDs[0] != "artifact_review" || !reflect.DeepEqual(replay.Assignment.DesiredAgent.SkillIDs, fixture.role.DefaultSkillIDs) {
		t.Fatalf("replay payload aliased prior result: handoff=%+v assignment=%+v", replay.Handoff, replay.Assignment)
	}
	items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("assignment count after replay = %d, want 1", len(items))
	}
	*fixture.clock = replay.Assignment.UpdatedAt.Add(time.Minute)
	claimed, err := fixture.service.ClaimAssignment(fixture.ctx, fixture.project.ID, replay.Assignment.ID, "worker_review")
	if err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	progressReplay, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
	if err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(after assignment progress) error = %v", err)
	}
	if !progressReplay.Replayed || progressReplay.Assignment.ID != claimed.ID || progressReplay.Assignment.Status != AssignmentClaimed || progressReplay.Assignment.ClaimedBy != "worker_review" {
		t.Fatalf("progress replay assignment = %+v replayed=%t, want current claimed assignment %q", progressReplay.Assignment, progressReplay.Replayed, claimed.ID)
	}

	mismatch := command
	mismatch.ExpectedUpdatedAt = progressReplay.Handoff.UpdatedAt
	if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, mismatch); !errors.Is(err, ErrConflict) {
		t.Fatalf("AcceptHandoffWithFollowUp(same key, changed request) error = %v, want ErrConflict", err)
	}
}

func TestHandoffAuthorityRequestHashIsUnambiguousForOpaqueIDs(t *testing.T) {
	stamp := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	first := AcceptHandoffWithFollowUpCommand{
		ProjectID:         "project",
		WorkItemID:        "work\x00segment",
		HandoffID:         "handoff",
		ExpectedUpdatedAt: stamp,
		Intent:            HandoffFollowUpIntentAcceptAndEnsure,
	}
	second := first
	second.ProjectID = "project\x00work"
	second.WorkItemID = "segment"
	if first.RequestHash() == second.RequestHash() {
		t.Fatalf("distinct opaque-id commands produced the same request hash %q", first.RequestHash())
	}
}

func TestHandoffAuthorityMemoryReceiptsAreScopedByOpaqueProjectAndKey(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	clock := time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	setup := func(projectID, label string) AcceptHandoffWithFollowUpCommand {
		t.Helper()
		project, err := service.CreateProject(ctx, Project{ID: projectID, Name: "Receipt scope " + label})
		if err != nil {
			t.Fatalf("CreateProject(%s) error = %v", label, err)
		}
		role, err := service.CreateRole(ctx, Role{ID: "role", ProjectID: project.ID, Name: "Owner"})
		if err != nil {
			t.Fatalf("CreateRole(%s) error = %v", label, err)
		}
		work, err := service.CreateWorkItem(ctx, WorkItem{ID: "work", ProjectID: project.ID, Title: "Work"})
		if err != nil {
			t.Fatalf("CreateWorkItem(%s) error = %v", label, err)
		}
		handoff, err := service.CreateHandoff(ctx, Handoff{ID: "handoff", ProjectID: project.ID, WorkItemID: work.ID, ToRoleID: role.ID, Title: "Continue", Body: "Follow up."})
		if err != nil {
			t.Fatalf("CreateHandoff(%s) error = %v", label, err)
		}
		return AcceptHandoffWithFollowUpCommand{
			ProjectID:         project.ID,
			WorkItemID:        work.ID,
			HandoffID:         handoff.ID,
			ExpectedUpdatedAt: handoff.UpdatedAt,
			Intent:            HandoffFollowUpIntentAcceptAndEnsure,
		}
	}

	separator := "\x00accept_handoff_with_follow_up\x00"
	first := setup("project", "first")
	first.IdempotencyKey = "suffix" + separator + "retry"
	second := setup("project"+separator+"suffix", "second")
	second.IdempotencyKey = "retry"
	if _, err := service.AcceptHandoffWithFollowUp(ctx, first); err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(first project) error = %v", err)
	}
	if _, err := service.AcceptHandoffWithFollowUp(ctx, second); err != nil {
		t.Fatalf("AcceptHandoffWithFollowUp(independent second project) error = %v, want independently scoped receipt", err)
	}
}

func TestHandoffAuthorityAcceptWithFollowUpHandlesAcceptedLegacyAndSatisfiedLinks(t *testing.T) {
	t.Run("accepted legacy handoff without target creates follow-up", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		*fixture.clock = fixture.handoff.UpdatedAt.Add(time.Minute)
		accepted, err := fixture.service.UpdateHandoffStatus(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffStatusUpdate{ExpectedUpdatedAt: fixture.handoff.UpdatedAt, Status: HandoffStatusAccepted},
		)
		if err != nil {
			t.Fatalf("UpdateHandoffStatus(accepted legacy) error = %v", err)
		}
		fixture.handoff = accepted
		*fixture.clock = accepted.UpdatedAt.Add(time.Minute)

		result, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, fixture.command("accepted-unlinked-legacy"))
		if err != nil {
			t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
		}
		if result.Outcome != HandoffFollowUpCreated || result.Replayed || result.Handoff.Status != HandoffStatusAccepted {
			t.Fatalf("result = %+v, want fresh created result preserving accepted status", result)
		}
		if result.Handoff.TargetAssignmentID != result.Assignment.ID || result.Handoff.TargetWorkItemID != fixture.sourceWork.ID {
			t.Fatalf("legacy handoff target = %q/%q, want created assignment %q on %q", result.Handoff.TargetAssignmentID, result.Handoff.TargetWorkItemID, result.Assignment.ID, fixture.sourceWork.ID)
		}
		if !result.Handoff.StatusChangedAt.Equal(accepted.StatusChangedAt) {
			t.Fatalf("status_changed_at = %s, want preserved accepted timestamp %s", result.Handoff.StatusChangedAt, accepted.StatusChangedAt)
		}
		if !result.Handoff.UpdatedAt.After(accepted.UpdatedAt) || result.Assignment.Status != AssignmentQueued || !result.Assignment.ExecutionRef.Empty() {
			t.Fatalf("result = %+v, want a newer link plus queued unlaunched assignment", result)
		}
	})

	t.Run("accepted linked handoff is already satisfied", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		linked, err := fixture.service.CreateAssignment(fixture.ctx, Assignment{
			ID:           "assignment_already_satisfied",
			ProjectID:    fixture.project.ID,
			WorkItemID:   fixture.targetWork.ID,
			RoleID:       fixture.role.ID,
			DesiredAgent: DesiredAgent{Kind: DesiredAgentAny},
		})
		if err != nil {
			t.Fatalf("CreateAssignment() error = %v", err)
		}
		linked, err = fixture.service.CompleteAssignment(
			fixture.ctx,
			fixture.project.ID,
			linked.ID,
			AssignmentCompleted,
			ExecutionRef{},
		)
		if err != nil {
			t.Fatalf("CompleteAssignment(linked) error = %v", err)
		}
		linkedHandoff, err := fixture.service.PatchHandoff(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffUpdate{
				ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
				Patch: HandoffPatch{
					TargetAssignmentID: authorityString(linked.ID),
					TargetWorkItemID:   authorityString(linked.WorkItemID),
				},
			},
		)
		if err != nil {
			t.Fatalf("PatchHandoff(target) error = %v", err)
		}
		accepted, err := fixture.service.UpdateHandoffStatus(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffStatusUpdate{ExpectedUpdatedAt: linkedHandoff.UpdatedAt, Status: HandoffStatusAccepted},
		)
		if err != nil {
			t.Fatalf("UpdateHandoffStatus(accepted) error = %v", err)
		}
		fixture.handoff = accepted

		result, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, fixture.command("already-satisfied"))
		if err != nil {
			t.Fatalf("AcceptHandoffWithFollowUp() error = %v", err)
		}
		if result.Outcome != HandoffFollowUpAlreadySatisfied || result.Replayed || result.Assignment.ID != linked.ID || result.Assignment.Status != AssignmentCompleted {
			t.Fatalf("result = %+v, want fresh already_satisfied response with current completed assignment %q", result, linked.ID)
		}
		if !result.Handoff.UpdatedAt.Equal(accepted.UpdatedAt) || !result.Handoff.StatusChangedAt.Equal(accepted.StatusChangedAt) {
			t.Fatalf("already-satisfied handoff timestamps = %s/%s, want unchanged %s/%s", result.Handoff.UpdatedAt, result.Handoff.StatusChangedAt, accepted.UpdatedAt, accepted.StatusChangedAt)
		}
		items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
		if err != nil {
			t.Fatalf("ListAssignments() error = %v", err)
		}
		if len(items) != 1 || items[0].ID != linked.ID {
			t.Fatalf("assignments = %+v, want only linked assignment %q", items, linked.ID)
		}
	})
}

func TestHandoffAuthorityAssistantUpdateRequiresCurrentRevision(t *testing.T) {
	fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
	proposalFor := func(handoff Handoff, id string) AssistantProposal {
		return AssistantProposal{
			ID:        id,
			ProjectID: fixture.project.ID,
			Title:     "Update handoff safely",
			Actions: []AssistantAction{{
				Kind:    AssistantActionUpdateHandoff,
				Handoff: &handoff,
			}},
		}
	}

	tokenless := fixture.handoff
	tokenless.Title = "Tokenless edit"
	tokenless.UpdatedAt = time.Time{}
	if _, err := fixture.service.AssistantPropose(fixture.ctx, proposalFor(tokenless, "prop_tokenless_handoff")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AssistantPropose(tokenless update) error = %v, want ErrInvalid", err)
	}

	current := fixture.handoff
	current.Title = "Current revision edit"
	*fixture.clock = current.UpdatedAt.Add(time.Minute)
	result, err := fixture.service.ApplyAssistantProposal(fixture.ctx, proposalFor(current, "prop_current_handoff"), true)
	if err != nil {
		t.Fatalf("ApplyAssistantProposal(current update) error = %v result=%+v", err, result)
	}
	updated, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff(current update) error = %v", err)
	}
	if updated.Title != current.Title || !updated.UpdatedAt.After(fixture.handoff.UpdatedAt) {
		t.Fatalf("updated handoff = %+v, want current edit with newer revision", updated)
	}

	stale := fixture.handoff
	stale.Title = "Stale overwrite"
	if result, err := fixture.service.ApplyAssistantProposal(fixture.ctx, proposalFor(stale, "prop_stale_handoff"), true); !errors.Is(err, ErrConflict) || result.AppliedActionCount != 0 {
		t.Fatalf("ApplyAssistantProposal(stale update) error = %v result=%+v, want conflict without applied action", err, result)
	}
	afterConflict, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff(stale conflict) error = %v", err)
	}
	if afterConflict.Title != current.Title || !afterConflict.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("handoff after stale proposal = %+v, want current authoritative state %+v", afterConflict, updated)
	}
}

func TestHandoffAuthorityAcceptWithFollowUpUsesSchemaCharacterLimit(t *testing.T) {
	t.Run("accepts 128 non-ASCII characters", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		command := fixture.command(strings.Repeat("é", 128))
		if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command); err != nil {
			t.Fatalf("AcceptHandoffWithFollowUp(128 characters) error = %v", err)
		}
	})

	t.Run("rejects 129 characters without mutation", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		command := fixture.command(strings.Repeat("é", 129))
		if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command); !errors.Is(err, ErrInvalid) {
			t.Fatalf("AcceptHandoffWithFollowUp(129 characters) error = %v, want ErrInvalid", err)
		}
		assignments, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
		if err != nil {
			t.Fatalf("ListAssignments() error = %v", err)
		}
		if len(assignments) != 0 {
			t.Fatalf("assignments = %+v, want none after invalid key", assignments)
		}
		current, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		if current.Status != HandoffStatusOpen || current.TargetAssignmentID != "" || !current.UpdatedAt.Equal(fixture.handoff.UpdatedAt) {
			t.Fatalf("handoff after invalid key = %+v, want unchanged open handoff", current)
		}
	})
}

func TestHandoffAuthorityAcceptWithFollowUpSerializesConcurrentRetries(t *testing.T) {
	t.Run("same key replays one assignment", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		command := fixture.command("concurrent-same-key")
		const attempts = 12
		results := make([]HandoffFollowUpResult, attempts)
		errs := make([]error, attempts)
		start := make(chan struct{})
		var group sync.WaitGroup
		for index := range attempts {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				results[index], errs[index] = fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
			}(index)
		}
		close(start)
		group.Wait()

		fresh := 0
		replayed := 0
		assignmentID := ""
		for index, err := range errs {
			if err != nil {
				t.Fatalf("attempt %d error = %v", index, err)
			}
			if results[index].Replayed {
				replayed++
			} else {
				fresh++
			}
			if assignmentID == "" {
				assignmentID = results[index].Assignment.ID
			}
			if results[index].Assignment.ID != assignmentID {
				t.Fatalf("attempt %d assignment id = %q, want %q", index, results[index].Assignment.ID, assignmentID)
			}
		}
		if fresh != 1 || replayed != attempts-1 {
			t.Fatalf("fresh/replayed attempts = %d/%d, want 1/%d", fresh, replayed, attempts-1)
		}
		items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
		if err != nil {
			t.Fatalf("ListAssignments() error = %v", err)
		}
		if len(items) != 1 || items[0].ID != assignmentID {
			t.Fatalf("assignments = %+v, want one assignment %q", items, assignmentID)
		}
	})

	t.Run("different keys race through CAS", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		commands := []AcceptHandoffWithFollowUpCommand{
			fixture.command("concurrent-key-a"),
			fixture.command("concurrent-key-b"),
		}
		results := make([]HandoffFollowUpResult, len(commands))
		errs := make([]error, len(commands))
		start := make(chan struct{})
		var group sync.WaitGroup
		for index := range commands {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				results[index], errs[index] = fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, commands[index])
			}(index)
		}
		close(start)
		group.Wait()

		successes := 0
		conflicts := 0
		for index, err := range errs {
			switch {
			case err == nil:
				successes++
				if results[index].Outcome != HandoffFollowUpCreated {
					t.Fatalf("successful attempt %d outcome = %q, want created", index, results[index].Outcome)
				}
			case errors.Is(err, ErrConflict):
				conflicts++
			default:
				t.Fatalf("attempt %d error = %v, want success or ErrConflict", index, err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("success/conflict count = %d/%d, want 1/1", successes, conflicts)
		}
		items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
		if err != nil {
			t.Fatalf("ListAssignments() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("assignments after different-key race = %+v, want one", items)
		}
	})
}

func TestHandoffAuthorityAcceptWithFollowUpRejectsStaleClosedAndBrokenLinks(t *testing.T) {
	t.Run("target work references removed root", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, true, ExecutionMCPPull)
		updated, err := fixture.service.PatchHandoff(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffUpdate{
				ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
				Patch:             HandoffPatch{TargetWorkItemID: authorityString(fixture.targetWork.ID)},
			},
		)
		if err != nil {
			t.Fatalf("PatchHandoff(target work) error = %v", err)
		}
		fixture.handoff = updated
		if _, _, err := fixture.service.DeleteRoot(fixture.ctx, fixture.project.ID, fixture.targetWork.RootID); err != nil {
			t.Fatalf("DeleteRoot() error = %v", err)
		}
		if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, fixture.command("removed-root")); !errors.Is(err, ErrNotFound) {
			t.Fatalf("AcceptHandoffWithFollowUp(removed root) error = %v, want ErrNotFound", err)
		}
		assignments, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
		if err != nil {
			t.Fatalf("ListAssignments() error = %v", err)
		}
		if len(assignments) != 0 {
			t.Fatalf("assignments after removed-root command = %+v, want none", assignments)
		}
	})

	t.Run("stale authority token", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		command := fixture.command("stale-follow-up")
		updated, err := fixture.service.PatchHandoff(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffUpdate{
				ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
				Patch:             HandoffPatch{Body: authorityString("A newer operator edit.")},
			},
		)
		if err != nil {
			t.Fatalf("PatchHandoff() error = %v", err)
		}
		if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command); !errors.Is(err, ErrConflict) {
			t.Fatalf("AcceptHandoffWithFollowUp(stale) error = %v, want ErrConflict", err)
		}
		got, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		if got.Body != updated.Body || got.Status != HandoffStatusOpen || got.TargetAssignmentID != "" {
			t.Fatalf("handoff after stale command = %+v, want newer open authority row", got)
		}
		items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
		if err != nil {
			t.Fatalf("ListAssignments() error = %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("assignments after stale command = %+v, want none", items)
		}
	})

	for _, status := range []string{HandoffStatusDismissed, HandoffStatusSuperseded} {
		t.Run("closed "+status, func(t *testing.T) {
			fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
			closed, err := fixture.service.UpdateHandoffStatus(
				fixture.ctx,
				fixture.project.ID,
				fixture.sourceWork.ID,
				fixture.handoff.ID,
				HandoffStatusUpdate{ExpectedUpdatedAt: fixture.handoff.UpdatedAt, Status: status},
			)
			if err != nil {
				t.Fatalf("UpdateHandoffStatus(%s) error = %v", status, err)
			}
			fixture.handoff = closed
			if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, fixture.command("closed-"+status)); !errors.Is(err, ErrConflict) {
				t.Fatalf("AcceptHandoffWithFollowUp(%s) error = %v, want ErrConflict", status, err)
			}
			items, err := fixture.service.ListAssignments(fixture.ctx, fixture.project.ID)
			if err != nil {
				t.Fatalf("ListAssignments() error = %v", err)
			}
			if len(items) != 0 {
				t.Fatalf("assignments after closed handoff = %+v, want none", items)
			}
		})
	}

	t.Run("closed after successful receipt", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		command := fixture.command("receipt-then-closed")
		accepted, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
		if err != nil {
			t.Fatalf("AcceptHandoffWithFollowUp(first) error = %v", err)
		}
		*fixture.clock = accepted.Handoff.UpdatedAt.Add(time.Minute)
		closed, err := fixture.service.UpdateHandoffStatus(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffStatusUpdate{ExpectedUpdatedAt: accepted.Handoff.UpdatedAt, Status: HandoffStatusDismissed},
		)
		if err != nil {
			t.Fatalf("UpdateHandoffStatus(dismissed) error = %v", err)
		}
		if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command); !errors.Is(err, ErrConflict) {
			t.Fatalf("AcceptHandoffWithFollowUp(replay after dismissal) error = %v, want ErrConflict", err)
		}
		got, err := fixture.service.GetHandoff(fixture.ctx, fixture.project.ID, fixture.sourceWork.ID, fixture.handoff.ID)
		if err != nil {
			t.Fatalf("GetHandoff() error = %v", err)
		}
		if got.Status != HandoffStatusDismissed || !got.UpdatedAt.Equal(closed.UpdatedAt) {
			t.Fatalf("handoff after rejected replay = %+v, want authoritative dismissal", got)
		}
	})

	t.Run("broken target assignment link", func(t *testing.T) {
		fixture := newHandoffAuthorityFixture(t, false, ExecutionMCPPull)
		linked, err := fixture.service.CreateAssignment(fixture.ctx, Assignment{
			ID:           "assignment_linked_then_deleted",
			ProjectID:    fixture.project.ID,
			WorkItemID:   fixture.targetWork.ID,
			RoleID:       fixture.role.ID,
			DesiredAgent: DesiredAgent{Kind: DesiredAgentAny},
		})
		if err != nil {
			t.Fatalf("CreateAssignment() error = %v", err)
		}
		updated, err := fixture.service.PatchHandoff(
			fixture.ctx,
			fixture.project.ID,
			fixture.sourceWork.ID,
			fixture.handoff.ID,
			HandoffUpdate{
				ExpectedUpdatedAt: fixture.handoff.UpdatedAt,
				Patch: HandoffPatch{
					TargetAssignmentID: authorityString(linked.ID),
					TargetWorkItemID:   authorityString(linked.WorkItemID),
				},
			},
		)
		if err != nil {
			t.Fatalf("PatchHandoff(target assignment) error = %v", err)
		}
		fixture.handoff = updated
		if err := fixture.service.DeleteAssignment(fixture.ctx, fixture.project.ID, linked.ID); err != nil {
			t.Fatalf("DeleteAssignment() error = %v", err)
		}
		command := fixture.command("broken-link-retry")
		if _, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command); !errors.Is(err, ErrConflict) {
			t.Fatalf("AcceptHandoffWithFollowUp(broken target) error = %v, want ErrConflict", err)
		}

		restored, err := fixture.service.CreateAssignment(fixture.ctx, Assignment{
			ID:           linked.ID,
			ProjectID:    fixture.project.ID,
			WorkItemID:   fixture.targetWork.ID,
			RoleID:       fixture.role.ID,
			DesiredAgent: DesiredAgent{Kind: DesiredAgentAny},
		})
		if err != nil {
			t.Fatalf("CreateAssignment(restored) error = %v", err)
		}
		result, err := fixture.service.AcceptHandoffWithFollowUp(fixture.ctx, command)
		if err != nil {
			t.Fatalf("AcceptHandoffWithFollowUp(after restoring link) error = %v", err)
		}
		if result.Outcome != HandoffFollowUpLinkedExisting || result.Assignment.ID != restored.ID || result.Handoff.Status != HandoffStatusAccepted {
			t.Fatalf("result after restoring link = %+v, want accepted linked_existing assignment %q", result, restored.ID)
		}
	})
}
