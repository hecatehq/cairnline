package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func seedAssignmentClaimLeaseTest(t *testing.T, service *Service) (context.Context, Project, Assignment) {
	t.Helper()
	ctx := context.Background()
	project, err := service.CreateProject(ctx, Project{Name: "Lease recovery"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, Role{ProjectID: project.ID, Name: "Worker"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, WorkItem{ProjectID: project.ID, Title: "Recover crashed reservation"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	return ctx, project, assignment
}

func TestService_AssignmentClaimLeaseUsesOperationClockAndSecureFence(t *testing.T) {
	clock := time.Date(2026, time.July, 22, 11, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(store, WithAssignmentClaimLeaseTTL(1500*time.Millisecond))
	service.now = func() time.Time { return clock }
	if service.AssignmentClaimLeaseTTL() != 2*time.Second {
		t.Fatalf("AssignmentClaimLeaseTTL() = %s, want rounded two-second TTL", service.AssignmentClaimLeaseTTL())
	}
	ctx, project, assignment := seedAssignmentClaimLeaseTest(t, service)
	futureRevision := clock.Add(365 * 24 * time.Hour)
	assignment.UpdatedAt = futureRevision
	if _, err := store.RestoreAssignmentSnapshot(ctx, assignment); err != nil {
		t.Fatalf("RestoreAssignmentSnapshot(future revision) error = %v", err)
	}

	entropyErr := errors.New("entropy unavailable")
	service.newAssignmentClaimID = func() (string, error) { return "", entropyErr }
	if _, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a"); !errors.Is(err, entropyErr) {
		t.Fatalf("ClaimAssignmentWithLease(entropy failure) error = %v, want propagated entropy error", err)
	}
	unchanged, err := service.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment(after entropy failure) error = %v", err)
	}
	if unchanged.Status != AssignmentQueued || unchanged.Claim != nil {
		t.Fatalf("assignment after entropy failure = %+v, want unchanged queued state", unchanged)
	}

	claimID, err := secureAssignmentClaimID()
	if err != nil {
		t.Fatalf("secureAssignmentClaimID() error = %v", err)
	}
	if !strings.HasPrefix(claimID, "claim_") || len(strings.TrimPrefix(claimID, "claim_")) != 32 {
		t.Fatalf("secureAssignmentClaimID() = %q, want 128-bit hexadecimal fence", claimID)
	}
	service.newAssignmentClaimID = func() (string, error) { return claimID, nil }
	claimed, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	if !claimed.Claim.AcquiredAt.Equal(clock) || !claimed.Claim.ExpiresAt.Equal(clock.Add(2*time.Second)) {
		t.Fatalf("claim = %+v, want lease based on operation clock %s", claimed.Claim, clock)
	}
	if !claimed.UpdatedAt.Equal(futureRevision.Add(time.Nanosecond)) {
		t.Fatalf("claim updated_at = %s, want monotonic revision after %s", claimed.UpdatedAt, futureRevision)
	}
}

func TestMemoryStoreRejectsMalformedAssignmentClaimWrites(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	ctx, project, assignment := seedAssignmentClaimLeaseTest(t, service)
	now := func() time.Time { return assignment.UpdatedAt.Add(time.Second) }
	if _, err := store.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "", AssignmentClaimLease{ID: " claim_bad "}, time.Minute, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ClaimAssignmentWithLease(malformed) error = %v, want ErrInvalid", err)
	}

	malformed := assignment
	malformed.Status = AssignmentClaimed
	malformed.ClaimedBy = "worker-a"
	malformed.Claim = &AssignmentClaimLease{ID: "claim_bad", ExpiresAt: assignment.UpdatedAt.Add(time.Minute)}
	if _, err := store.RestoreAssignmentSnapshot(ctx, malformed); !errors.Is(err, ErrInvalid) {
		t.Fatalf("RestoreAssignmentSnapshot(malformed) error = %v, want ErrInvalid", err)
	}
	stored, err := store.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if stored.Status != AssignmentQueued || stored.Claim != nil {
		t.Fatalf("stored assignment = %+v, want readable unchanged queued state", stored)
	}
	claimed, err := store.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a", AssignmentClaimLease{ID: "claim_good"}, time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease(valid) error = %v", err)
	}
	if _, err := store.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, AssignmentCompleted, ExecutionRef{}, claimed.Claim.ID, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateAssignmentStatusWithClaim(invalid target) error = %v, want ErrInvalid", err)
	}
	if _, err := store.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentRunning, ExecutionRef{}, claimed.Claim.ID, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CompleteAssignmentWithClaim(invalid target) error = %v, want ErrInvalid", err)
	}
	unchanged, err := store.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment(after invalid targets) error = %v", err)
	}
	if unchanged.Status != AssignmentClaimed || unchanged.Claim == nil || unchanged.Claim.ID != claimed.Claim.ID {
		t.Fatalf("assignment after invalid targets = %+v, want unchanged valid claim", unchanged)
	}
}

func TestService_AssignmentClaimLeaseRenewRecoverAndFence(t *testing.T) {
	clock := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)
	service := NewService(NewMemoryStore(), WithAssignmentClaimLeaseTTL(time.Minute))
	service.now = func() time.Time { return clock }
	ctx, project, assignment := seedAssignmentClaimLeaseTest(t, service)
	clock = clock.Add(time.Second)

	claimed, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	if claimed.Status != AssignmentClaimed || claimed.ClaimedBy != "worker-a" || claimed.Claim == nil || claimed.Claim.ID == "" {
		t.Fatalf("claimed assignment = %+v, want leased worker-a claim", claimed)
	}
	if !claimed.Claim.AcquiredAt.Equal(clock) || !claimed.Claim.ExpiresAt.Equal(clock.Add(time.Minute)) {
		t.Fatalf("claim lease = %+v, want one-minute server lease", claimed.Claim)
	}
	claimID := claimed.Claim.ID
	claimUpdatedAt := claimed.UpdatedAt
	if _, err := service.RecoverAssignmentClaim(ctx, project.ID, assignment.ID, claimID); !errors.Is(err, ErrConflict) {
		t.Fatalf("RecoverAssignmentClaim(before expiry) error = %v, want ErrConflict", err)
	}
	if _, err := service.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, AssignmentRunning, ExecutionRef{}, "claim_wrong"); !errors.Is(err, ErrConflict) {
		t.Fatalf("UpdateAssignmentStatusWithClaim(wrong fence) error = %v, want ErrConflict", err)
	}

	clock = clock.Add(20 * time.Second)
	renewed, err := service.RenewAssignmentClaim(ctx, project.ID, assignment.ID, claimID)
	if err != nil {
		t.Fatalf("RenewAssignmentClaim() error = %v", err)
	}
	if !renewed.UpdatedAt.Equal(claimUpdatedAt) || !renewed.Claim.ExpiresAt.Equal(clock.Add(time.Minute)) {
		t.Fatalf("renewed assignment = %+v, want extended lease without activity revision", renewed)
	}
	clock = claimed.Claim.AcquiredAt.Add(5 * time.Second)
	renewedAfterClockRegression, err := service.RenewAssignmentClaim(ctx, project.ID, assignment.ID, claimID)
	if err != nil {
		t.Fatalf("RenewAssignmentClaim(backward clock) error = %v", err)
	}
	if !renewedAfterClockRegression.Claim.ExpiresAt.Equal(renewed.Claim.ExpiresAt) {
		t.Fatalf("renewed assignment after clock regression = %+v, want lease not shortened below %s", renewedAfterClockRegression, renewed.Claim.ExpiresAt)
	}
	renewed = renewedAfterClockRegression

	prepared, err := service.PrepareAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentPreparation{
		ClaimID:           claimID,
		ExecutionRef:      ExecutionRef{Kind: "task_run", RunID: "run-a"},
		ContextSnapshotID: "ctx-a",
	})
	if err != nil {
		t.Fatalf("PrepareAssignmentWithClaim() error = %v", err)
	}
	if prepared.ExecutionRef.RunID != "run-a" || prepared.ContextSnapshotID != "ctx-a" {
		t.Fatalf("prepared assignment = %+v, want host references", prepared)
	}

	clock = renewed.Claim.ExpiresAt
	brief, err := service.ProjectOperationsBrief(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectOperationsBrief(expired claim) error = %v", err)
	}
	if brief.Counts.BlockedAssignments != 1 || brief.Next == nil || brief.Next.Title != "Recover expired claim for Recover crashed reservation" {
		t.Fatalf("operations brief = %+v, want expired claim recovery attention", brief)
	}
	if brief.Next.ActionKind != ProjectOperationActionRecoverClaim || brief.Next.ActionLabel != "Recover claim" {
		t.Fatalf("operations next action = %+v, want typed claim recovery action", brief.Next)
	}
	health, err := service.ProjectHealth(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectHealth(expired claim) error = %v", err)
	}
	if !health.CreatedAt.Equal(brief.CreatedAt) {
		t.Fatalf("health created_at = %s, want operations snapshot time %s", health.CreatedAt, brief.CreatedAt)
	}
	foundRecoveryAction := false
	for _, item := range health.Attention {
		if item.AssignmentID == assignment.ID && item.ActionKind == ProjectOperationActionRecoverClaim && item.ActionLabel == "Recover claim" {
			foundRecoveryAction = true
			break
		}
	}
	if !foundRecoveryAction {
		t.Fatalf("health attention = %+v, want typed claim recovery action", health.Attention)
	}
	activity, err := service.ProjectActivity(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectActivity(expired claim) error = %v", err)
	}
	if activity.Counts.Claimed != 1 || activity.Counts.Blocked != 1 || activity.Counts.Active != 0 || len(activity.Buckets.Blocked) != 1 {
		t.Fatalf("activity = %+v, want expired claimed assignment in blocked bucket", activity)
	}
	for name, mutate := range map[string]func() error{
		"renew": func() error {
			_, err := service.RenewAssignmentClaim(ctx, project.ID, assignment.ID, claimID)
			return err
		},
		"prepare": func() error {
			_, err := service.PrepareAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentPreparation{ClaimID: claimID, ContextSnapshotID: "too-late"})
			return err
		},
		"release": func() error {
			_, err := service.ReleaseAssignmentWithClaim(ctx, project.ID, assignment.ID, claimID)
			return err
		},
		"start": func() error {
			_, err := service.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, AssignmentRunning, ExecutionRef{}, claimID)
			return err
		},
		"complete": func() error {
			_, err := service.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentCompleted, ExecutionRef{}, claimID)
			return err
		},
	} {
		t.Run("expired_"+name, func(t *testing.T) {
			if err := mutate(); !errors.Is(err, ErrConflict) {
				t.Fatalf("mutation error = %v, want ErrConflict", err)
			}
		})
	}

	recovered, err := service.RecoverAssignmentClaim(ctx, project.ID, assignment.ID, claimID)
	if err != nil {
		t.Fatalf("RecoverAssignmentClaim() error = %v", err)
	}
	if recovered.Status != AssignmentQueued || recovered.ClaimedBy != "" || recovered.Claim != nil || !recovered.ExecutionRef.Empty() || recovered.ContextSnapshotID != "" {
		t.Fatalf("recovered assignment = %+v, want pristine queued coordination", recovered)
	}
	if _, err := service.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentCompleted, ExecutionRef{}, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CompleteAssignmentWithClaim(recovered without fence) error = %v, want ErrInvalid", err)
	}

	clock = clock.Add(time.Second)
	reclaimed, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-b")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease(reclaim) error = %v", err)
	}
	if reclaimed.Claim == nil || reclaimed.Claim.ID == claimID {
		t.Fatalf("reclaimed lease = %+v, want new fencing generation", reclaimed.Claim)
	}
	newClaimID := reclaimed.Claim.ID
	if _, err := service.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, AssignmentRunning, ExecutionRef{}, claimID); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale UpdateAssignmentStatusWithClaim() error = %v, want ErrConflict", err)
	}
	running, err := service.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, AssignmentRunning, ExecutionRef{RunID: "run-b"}, newClaimID)
	if err != nil {
		t.Fatalf("UpdateAssignmentStatusWithClaim() error = %v", err)
	}
	if running.Claim == nil || running.Claim.ID != newClaimID || !running.Claim.ExpiresAt.IsZero() {
		t.Fatalf("running claim = %+v, want retained fence with retired pre-start expiry", running.Claim)
	}
	clock = clock.Add(24 * time.Hour)
	if _, err := service.RecoverAssignmentClaim(ctx, project.ID, assignment.ID, newClaimID); !errors.Is(err, ErrConflict) {
		t.Fatalf("RecoverAssignmentClaim(running) error = %v, want ErrConflict", err)
	}
	if _, err := service.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentCompleted, ExecutionRef{}, claimID); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CompleteAssignmentWithClaim() error = %v, want ErrConflict", err)
	}
	completed, err := service.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, AssignmentCompleted, ExecutionRef{}, newClaimID)
	if err != nil {
		t.Fatalf("CompleteAssignmentWithClaim() error = %v", err)
	}
	if completed.Status != AssignmentCompleted || completed.Claim == nil || completed.Claim.ID != newClaimID {
		t.Fatalf("completed assignment = %+v, want fenced completion", completed)
	}
}

func TestService_AssignmentClaimRecoveryRaceHasOneWinner(t *testing.T) {
	clock := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	service := NewService(NewMemoryStore(), WithAssignmentClaimLeaseTTL(time.Second))
	service.now = func() time.Time { return clock }
	ctx, project, assignment := seedAssignmentClaimLeaseTest(t, service)
	claimed, err := service.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	clock = claimed.Claim.ExpiresAt

	var successes atomic.Int32
	var conflicts atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.RecoverAssignmentClaim(ctx, project.ID, assignment.ID, claimed.Claim.ID)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("RecoverAssignmentClaim() error = %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 || conflicts.Load() != 7 {
		t.Fatalf("recover results successes=%d conflicts=%d, want 1 and 7", successes.Load(), conflicts.Load())
	}
}

func TestService_AssignmentClaimLeaseSnapshotV2AndV1Import(t *testing.T) {
	clock := time.Date(2026, time.July, 22, 16, 0, 0, 0, time.UTC)
	source := NewService(NewMemoryStore(), WithAssignmentClaimLeaseTTL(time.Minute))
	source.now = func() time.Time { return clock }
	ctx, project, assignment := seedAssignmentClaimLeaseTest(t, source)
	claimed, err := source.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a")
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	exported, err := source.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	if exported.Version != 2 {
		t.Fatalf("snapshot version = %d, want 2", exported.Version)
	}

	target := NewService(NewMemoryStore())
	if _, err := target.ImportSnapshot(ctx, exported); err != nil {
		t.Fatalf("ImportSnapshot(v2) error = %v", err)
	}
	roundTripped, err := target.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment(v2) error = %v", err)
	}
	if roundTripped.Claim == nil || roundTripped.Claim.ID != claimed.Claim.ID || !roundTripped.Claim.ExpiresAt.Equal(claimed.Claim.ExpiresAt) {
		t.Fatalf("round-tripped claim = %+v, want %+v", roundTripped.Claim, claimed.Claim)
	}

	legacy := exported
	legacy.Version = 1
	legacy.Assignments = append([]Assignment(nil), exported.Assignments...)
	legacy.Assignments[0].Claim = nil
	legacyTarget := NewService(NewMemoryStore())
	if _, err := legacyTarget.ImportSnapshot(ctx, legacy); err != nil {
		t.Fatalf("ImportSnapshot(v1) error = %v", err)
	}
	legacyClaim, err := legacyTarget.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment(v1) error = %v", err)
	}
	if legacyClaim.Status != AssignmentClaimed || legacyClaim.Claim != nil || legacyClaim.ClaimedBy != "worker-a" {
		t.Fatalf("legacy assignment = %+v, want preserved host-authoritative unleased claim", legacyClaim)
	}
}

func TestService_AssignmentClaimLeaseSnapshotRejectsMalformedClaimsBeforeWrites(t *testing.T) {
	clock := time.Date(2026, time.July, 22, 17, 0, 0, 0, time.UTC)
	source := NewService(NewMemoryStore(), WithAssignmentClaimLeaseTTL(time.Minute))
	source.now = func() time.Time { return clock }
	ctx, project, assignment := seedAssignmentClaimLeaseTest(t, source)
	if _, err := source.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a"); err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	exported, err := source.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}

	tests := map[string]func(*Snapshot){
		"missing acquired at": func(snapshot *Snapshot) {
			snapshot.Assignments[0].Claim.AcquiredAt = time.Time{}
		},
		"whitespace claim id": func(snapshot *Snapshot) {
			snapshot.Assignments[0].Claim.ID = " claim_bad "
		},
		"missing claimed expiry": func(snapshot *Snapshot) {
			snapshot.Assignments[0].Claim.ExpiresAt = time.Time{}
		},
		"active expiry after start": func(snapshot *Snapshot) {
			snapshot.Assignments[0].Status = AssignmentRunning
		},
		"version one claim": func(snapshot *Snapshot) {
			snapshot.Version = legacySnapshotVersion
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			malformed := exported
			malformed.Assignments = append([]Assignment(nil), exported.Assignments...)
			claim := *exported.Assignments[0].Claim
			malformed.Assignments[0].Claim = &claim
			mutate(&malformed)
			target := NewService(NewMemoryStore())
			if _, err := target.ImportSnapshot(ctx, malformed); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ImportSnapshot() error = %v, want ErrInvalid", err)
			}
			projects, err := target.ListProjects(ctx)
			if err != nil {
				t.Fatalf("ListProjects() error = %v", err)
			}
			if len(projects) != 0 {
				t.Fatalf("projects after rejected import = %+v, want no partial writes", projects)
			}
		})
	}
}
