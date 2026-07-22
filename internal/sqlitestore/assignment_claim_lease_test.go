package sqlitestore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hecatehq/cairnline/internal/core"
)

func seedSQLiteClaimLeaseAssignment(t *testing.T, store *Store) (context.Context, core.Project, core.Assignment) {
	t.Helper()
	ctx := context.Background()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "SQLite lease recovery"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Worker"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Fence SQLite workers"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{ProjectID: project.ID, WorkItemID: work.ID, RoleID: role.ID})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	return ctx, project, assignment
}

func TestStore_AssignmentClaimLeasePersistsAndFencesRecovery(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "claims.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	_, project, assignment := seedSQLiteClaimLeaseAssignment(t, store)
	clock := assignment.UpdatedAt.Add(time.Second)
	now := func() time.Time { return clock }
	futureRevision := clock.Add(365 * 24 * time.Hour)
	assignment.UpdatedAt = futureRevision
	if _, err := store.RestoreAssignmentSnapshot(ctx, assignment); err != nil {
		t.Fatalf("RestoreAssignmentSnapshot(future revision) error = %v", err)
	}

	claimed, err := store.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a", core.AssignmentClaimLease{ID: "claim_a"}, time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	if claimed.Claim == nil || claimed.Claim.ID != "claim_a" || !claimed.Claim.AcquiredAt.Equal(clock) || !claimed.Claim.ExpiresAt.Equal(clock.Add(time.Minute)) || !claimed.UpdatedAt.Equal(futureRevision.Add(time.Nanosecond)) {
		t.Fatalf("claimed assignment = %+v, want persisted claim_a lease", claimed)
	}
	claimUpdatedAt := claimed.UpdatedAt
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	clock = clock.Add(20 * time.Second)
	renewed, err := store.RenewAssignmentClaim(ctx, project.ID, assignment.ID, "claim_a", time.Minute, now)
	if err != nil {
		t.Fatalf("RenewAssignmentClaim() error = %v", err)
	}
	if !renewed.UpdatedAt.Equal(claimUpdatedAt) || !renewed.Claim.ExpiresAt.Equal(clock.Add(time.Minute)) {
		t.Fatalf("renewed assignment = %+v, want durable heartbeat without activity revision", renewed)
	}
	clock = claimed.Claim.AcquiredAt.Add(5 * time.Second)
	renewedAfterClockRegression, err := store.RenewAssignmentClaim(ctx, project.ID, assignment.ID, "claim_a", time.Minute, now)
	if err != nil {
		t.Fatalf("RenewAssignmentClaim(backward clock) error = %v", err)
	}
	if !renewedAfterClockRegression.Claim.ExpiresAt.Equal(renewed.Claim.ExpiresAt) {
		t.Fatalf("renewed assignment after clock regression = %+v, want lease not shortened below %s", renewedAfterClockRegression, renewed.Claim.ExpiresAt)
	}
	renewed = renewedAfterClockRegression
	if _, err := store.PrepareAssignmentWithClaim(ctx, project.ID, assignment.ID, core.AssignmentPreparation{ClaimID: "wrong", ContextSnapshotID: "ctx-wrong"}, now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("PrepareAssignmentWithClaim(wrong) error = %v, want ErrConflict", err)
	}
	prepared, err := store.PrepareAssignmentWithClaim(ctx, project.ID, assignment.ID, core.AssignmentPreparation{ClaimID: "claim_a", ContextSnapshotID: "ctx-a"}, now)
	if err != nil {
		t.Fatalf("PrepareAssignmentWithClaim() error = %v", err)
	}
	if prepared.ContextSnapshotID != "ctx-a" {
		t.Fatalf("prepared assignment = %+v, want context snapshot", prepared)
	}

	clock = renewed.Claim.ExpiresAt
	if _, err := store.ReleaseAssignmentWithClaim(ctx, project.ID, assignment.ID, "claim_a", now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("ReleaseAssignmentWithClaim(expired) error = %v, want ErrConflict", err)
	}
	recovered, err := store.RecoverAssignmentClaim(ctx, project.ID, assignment.ID, "claim_a", now)
	if err != nil {
		t.Fatalf("RecoverAssignmentClaim() error = %v", err)
	}
	if recovered.Status != core.AssignmentQueued || recovered.Claim != nil || recovered.ContextSnapshotID != "" {
		t.Fatalf("recovered assignment = %+v, want cleared queued state", recovered)
	}
	if _, err := store.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, core.AssignmentCompleted, core.ExecutionRef{}, "", now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("CompleteAssignmentWithClaim(recovered without fence) error = %v, want ErrConflict", err)
	}

	clock = clock.Add(time.Second)
	reclaimed, err := store.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-b", core.AssignmentClaimLease{ID: "claim_b"}, time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease(reclaim) error = %v", err)
	}
	if _, err := store.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, core.AssignmentRunning, core.ExecutionRef{}, "claim_a", now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("UpdateAssignmentStatusWithClaim(stale) error = %v, want ErrConflict", err)
	}
	running, err := store.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, core.AssignmentRunning, core.ExecutionRef{RunID: "run-b"}, "claim_b", now)
	if err != nil {
		t.Fatalf("UpdateAssignmentStatusWithClaim() error = %v", err)
	}
	if running.Claim == nil || running.Claim.ID != "claim_b" || !running.Claim.ExpiresAt.IsZero() {
		t.Fatalf("running claim = %+v, want retained fence without pre-start expiry", running.Claim)
	}
	clock = reclaimed.Claim.ExpiresAt.Add(time.Hour)
	if _, err := store.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, core.AssignmentCompleted, core.ExecutionRef{}, "claim_a", now); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("CompleteAssignmentWithClaim(stale) error = %v, want ErrConflict", err)
	}
	completed, err := store.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, core.AssignmentCompleted, core.ExecutionRef{}, "claim_b", now)
	if err != nil {
		t.Fatalf("CompleteAssignmentWithClaim() error = %v", err)
	}
	if completed.Status != core.AssignmentCompleted || completed.Claim == nil || completed.Claim.ID != "claim_b" {
		t.Fatalf("completed assignment = %+v, want claim_b completion", completed)
	}
}

func TestStore_AssignmentClaimRecoveryAcrossConnectionsHasOneWinner(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "claim-race.db")
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	defer first.Close()
	_, project, assignment := seedSQLiteClaimLeaseAssignment(t, first)
	clock := time.Date(2026, time.July, 22, 15, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	claimed, err := first.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a", core.AssignmentClaimLease{ID: "claim_race"}, time.Second, now)
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	clock = claimed.Claim.ExpiresAt
	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer second.Close()

	var successes atomic.Int32
	var conflicts atomic.Int32
	var wg sync.WaitGroup
	for _, store := range []*Store{first, second} {
		wg.Add(1)
		go func(store *Store) {
			defer wg.Done()
			_, err := store.RecoverAssignmentClaim(ctx, project.ID, assignment.ID, "claim_race", now)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, core.ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("RecoverAssignmentClaim() error = %v", err)
			}
		}(store)
	}
	wg.Wait()
	if successes.Load() != 1 || conflicts.Load() != 1 {
		t.Fatalf("recover results successes=%d conflicts=%d, want 1 and 1", successes.Load(), conflicts.Load())
	}
}

func TestStore_RejectsMalformedAssignmentClaimWrites(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "malformed-claim.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	_, project, assignment := seedSQLiteClaimLeaseAssignment(t, store)
	now := func() time.Time { return assignment.UpdatedAt.Add(time.Second) }
	if _, err := store.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "", core.AssignmentClaimLease{ID: " claim_bad "}, time.Minute, now); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("ClaimAssignmentWithLease(malformed) error = %v, want ErrInvalid", err)
	}

	malformed := assignment
	malformed.Status = core.AssignmentClaimed
	malformed.ClaimedBy = "worker-a"
	malformed.Claim = &core.AssignmentClaimLease{ID: "claim_bad", ExpiresAt: assignment.UpdatedAt.Add(time.Minute)}
	if _, err := store.RestoreAssignmentSnapshot(ctx, malformed); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("RestoreAssignmentSnapshot(malformed) error = %v, want ErrInvalid", err)
	}
	stored, err := store.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}
	if stored.Status != core.AssignmentQueued || stored.Claim != nil {
		t.Fatalf("stored assignment = %+v, want readable unchanged queued state", stored)
	}
	claimed, err := store.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a", core.AssignmentClaimLease{ID: "claim_good"}, time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimAssignmentWithLease(valid) error = %v", err)
	}
	if _, err := store.UpdateAssignmentStatusWithClaim(ctx, project.ID, assignment.ID, core.AssignmentCompleted, core.ExecutionRef{}, claimed.Claim.ID, now); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("UpdateAssignmentStatusWithClaim(invalid target) error = %v, want ErrInvalid", err)
	}
	if _, err := store.CompleteAssignmentWithClaim(ctx, project.ID, assignment.ID, core.AssignmentRunning, core.ExecutionRef{}, claimed.Claim.ID, now); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("CompleteAssignmentWithClaim(invalid target) error = %v, want ErrInvalid", err)
	}
	unchanged, err := store.GetAssignment(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("GetAssignment(after invalid targets) error = %v", err)
	}
	if unchanged.Status != core.AssignmentClaimed || unchanged.Claim == nil || unchanged.Claim.ID != claimed.Claim.ID {
		t.Fatalf("assignment after invalid targets = %+v, want unchanged valid claim", unchanged)
	}

	malformed.ID = "assignment_bad_claim"
	if _, err := store.CreateAssignment(ctx, malformed); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("CreateAssignment(malformed) error = %v, want ErrInvalid", err)
	}
	if _, err := store.GetAssignment(ctx, project.ID, malformed.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetAssignment(malformed create) error = %v, want ErrNotFound", err)
	}
}

func TestStore_AssignmentClaimSnapshotValidationPreflightsSQLiteImport(t *testing.T) {
	ctx := context.Background()
	sourceStore, err := Open(ctx, filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	t.Cleanup(func() { _ = sourceStore.Close() })
	sourceService := core.NewService(sourceStore)
	_, project, assignment := seedSQLiteClaimLeaseAssignment(t, sourceStore)
	claimTime := assignment.UpdatedAt.Add(time.Second)
	if _, err := sourceStore.ClaimAssignmentWithLease(ctx, project.ID, assignment.ID, "worker-a", core.AssignmentClaimLease{ID: "claim_valid"}, time.Minute, func() time.Time { return claimTime }); err != nil {
		t.Fatalf("ClaimAssignmentWithLease() error = %v", err)
	}
	exported, err := sourceService.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}

	for name, mutate := range map[string]func(*core.AssignmentClaimLease){
		"missing acquired at": func(claim *core.AssignmentClaimLease) { claim.AcquiredAt = time.Time{} },
		"whitespace claim id": func(claim *core.AssignmentClaimLease) { claim.ID = " claim_bad " },
	} {
		t.Run(name, func(t *testing.T) {
			malformed := exported
			malformed.Assignments = append([]core.Assignment(nil), exported.Assignments...)
			claim := *exported.Assignments[0].Claim
			malformed.Assignments[0].Claim = &claim
			mutate(&claim)

			targetStore, err := Open(ctx, filepath.Join(t.TempDir(), "target.db"))
			if err != nil {
				t.Fatalf("Open(target) error = %v", err)
			}
			t.Cleanup(func() { _ = targetStore.Close() })
			targetService := core.NewService(targetStore)
			if _, err := targetService.ImportSnapshot(ctx, malformed); !errors.Is(err, core.ErrInvalid) {
				t.Fatalf("ImportSnapshot() error = %v, want ErrInvalid", err)
			}
			projects, err := targetService.ListProjects(ctx)
			if err != nil {
				t.Fatalf("ListProjects() error = %v", err)
			}
			if len(projects) != 0 {
				t.Fatalf("projects after rejected import = %+v, want no partial writes", projects)
			}
		})
	}
}
