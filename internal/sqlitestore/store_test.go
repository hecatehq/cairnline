package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
)

func TestStore_PersistsAssignmentLifecycle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cairnline.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := core.NewService(store)
	profile, err := service.CreateAgentProfile(ctx, core.AgentProfile{
		Name:          "Reviewer profile",
		Instructions:  "Review persisted context.",
		ContextPolicy: "include_enabled",
		SkillIDs:      []string{"review"},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	executionProfile, err := service.CreateExecutionProfile(ctx, core.ExecutionProfile{
		Name:           "SQLite execution",
		AgentKind:      "any",
		ModelHint:      "local",
		ProviderHint:   "local",
		ToolsPolicy:    "readonly",
		WritesPolicy:   "block",
		NetworkPolicy:  "block",
		ApprovalPolicy: "require",
		AdapterOptions: map[string]any{"mode": "test"},
	})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, core.Project{
		Name:        "Persistent project",
		Description: "Survives process restart.",
		Roots: []core.Root{{
			ID:     "root_main",
			Path:   "/tmp/example",
			Kind:   "workspace",
			Active: true,
		}},
		ContextSources: []core.Source{{
			ID:         "src_agents",
			Kind:       "workspace_instruction",
			Title:      "AGENTS.md",
			Enabled:    true,
			TrustLabel: "workspace_guidance",
		}},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	skill, err := service.CreateProjectSkill(ctx, core.ProjectSkill{
		ProjectID:   project.ID,
		ID:          "review",
		Title:       "Review skill",
		Description: "Review work with evidence.",
		Format:      core.SkillFormatMarkdown,
		Status:      core.SkillStatusAvailable,
		TrustLabel:  core.SkillTrustWorkspace,
		SourceRefs:  []string{"src_agents"},
	})
	if err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{
		ProjectID:            project.ID,
		Name:                 "Reviewer",
		Instructions:         "Review the durable trail.",
		DefaultProfileID:     profile.ID,
		DefaultSkillIDs:      []string{"review", "evidence"},
		DefaultExecutionMode: core.ExecutionMCPPull,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{
		ProjectID:       project.ID,
		Title:           "Check persistence",
		Brief:           "Create, claim, complete, reopen.",
		ReviewerRoleIDs: []string{role.ID},
		RootID:          "root_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		RoleID:             role.ID,
		RootID:             "root_main",
		ExecutionProfileID: executionProfile.ID,
		ExecutionMode:      core.ExecutionMCPPull,
		DesiredAgent: core.DesiredAgent{
			Kind:     "any",
			SkillIDs: []string{"review"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-a"); err != nil {
		t.Fatalf("ClaimAssignment() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, core.Evidence{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		Title:      "Output link",
		Locator:    "https://example.test/report",
	}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{
		ProjectID:      project.ID,
		WorkItemID:     work.ID,
		AssignmentID:   assignment.ID,
		ReviewerRoleID: role.ID,
		Title:          "Review pass",
		Body:           "The persistence flow works.",
		Verdict:        core.ReviewVerdictPass,
		Risk:           core.ReviewRiskLow,
	}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, core.Handoff{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		FromRoleID: role.ID,
		ToRoleID:   role.ID,
		Title:      "Next pass",
		Body:       "Use the persisted context.",
	}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ProjectID:           project.ID,
		Title:               "Persistence lesson",
		Body:                "Cairnline stores collaboration artifacts in SQLite.",
		SuggestedTrustLabel: "test",
		SuggestedSourceKind: "assignment",
		SuggestedSourceID:   assignment.ID,
	}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	memoryEntry, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
		ProjectID:  project.ID,
		Title:      "Persisted memory",
		Body:       "Accepted memory is available to assignment launch packets.",
		SourceKind: core.MemorySourceOperator,
		SourceID:   assignment.ID,
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	if _, err := service.CompleteAssignment(ctx, project.ID, assignment.ID, core.AssignmentCompleted, "run-1"); err != nil {
		t.Fatalf("CompleteAssignment() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen Open() error = %v", err)
	}
	defer reopened.Close()
	reopenedService := core.NewService(reopened)

	projects, err := reopenedService.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID || len(projects[0].Roots) != 1 || len(projects[0].ContextSources) != 1 {
		t.Fatalf("projects = %+v, want persisted project metadata", projects)
	}
	profiles, err := reopenedService.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != profile.ID || len(profiles[0].SkillIDs) != 1 {
		t.Fatalf("profiles = %+v, want persisted profile metadata", profiles)
	}
	executionProfiles, err := reopenedService.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() error = %v", err)
	}
	if len(executionProfiles) != 1 || executionProfiles[0].ID != executionProfile.ID || executionProfiles[0].AdapterOptions["mode"] != "test" {
		t.Fatalf("execution profiles = %+v, want persisted execution profile metadata", executionProfiles)
	}
	roles, err := reopenedService.ListRoles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != role.ID || len(roles[0].DefaultSkillIDs) != 2 {
		t.Fatalf("roles = %+v, want persisted role metadata", roles)
	}
	skills, err := reopenedService.ListProjectSkills(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListProjectSkills() error = %v", err)
	}
	if len(skills) != 1 || skills[0].ID != skill.ID || len(skills[0].SourceRefs) != 1 || !skills[0].Enabled {
		t.Fatalf("skills = %+v, want persisted enabled project skill", skills)
	}
	assignments, err := reopenedService.ListAssignments(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListAssignments() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].ID != assignment.ID || assignments[0].Status != core.AssignmentCompleted || assignments[0].ExecutionRef != "run-1" {
		t.Fatalf("assignments = %+v, want completed assignment", assignments)
	}
	if assignments[0].RootID != "root_main" {
		t.Fatalf("assignment root = %q, want root_main", assignments[0].RootID)
	}
	packet, err := reopenedService.AssignmentContext(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentContext() error = %v", err)
	}
	if packet.Role == nil || packet.Role.ID != role.ID || packet.WorkItem.ID != work.ID {
		t.Fatalf("packet = %+v, want persisted context metadata", packet)
	}
	if packet.WorkItem.RootID != "root_main" || packet.Assignment.RootID != "root_main" {
		t.Fatalf("packet roots work=%q assignment=%q, want root_main", packet.WorkItem.RootID, packet.Assignment.RootID)
	}
	evidence, err := reopenedService.ListEvidence(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListEvidence() error = %v", err)
	}
	if len(evidence) != 1 || evidence[0].Title != "Output link" || evidence[0].TrustLabel != core.EvidenceTrustOperator {
		t.Fatalf("evidence = %+v, want persisted evidence", evidence)
	}
	reviews, err := reopenedService.ListReviews(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 1 || reviews[0].AssignmentID != assignment.ID || reviews[0].Verdict != core.ReviewVerdictPass {
		t.Fatalf("reviews = %+v, want persisted review", reviews)
	}
	handoffs, err := reopenedService.ListHandoffs(ctx, project.ID, work.ID)
	if err != nil {
		t.Fatalf("ListHandoffs() error = %v", err)
	}
	if len(handoffs) != 1 || handoffs[0].Status != core.HandoffStatusOpen {
		t.Fatalf("handoffs = %+v, want persisted handoff", handoffs)
	}
	memory, err := reopenedService.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(memory) != 1 || memory[0].Status != core.MemoryCandidatePending || memory[0].SuggestedSourceID != assignment.ID {
		t.Fatalf("memory candidates = %+v, want persisted candidate", memory)
	}
	memoryEntries, err := reopenedService.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries() error = %v", err)
	}
	if len(memoryEntries) != 1 || memoryEntries[0].ID != memoryEntry.ID || memoryEntries[0].TrustLabel != core.MemoryTrustOperator {
		t.Fatalf("memory entries = %+v, want persisted accepted memory", memoryEntries)
	}
	launchPacket, err := reopenedService.AssignmentLaunchPacket(ctx, project.ID, assignment.ID)
	if err != nil {
		t.Fatalf("AssignmentLaunchPacket() error = %v", err)
	}
	if launchPacket.Kind != core.LaunchPacketKindAssignment || launchPacket.Assignment.ID != assignment.ID || launchPacket.Role == nil || launchPacket.Role.ID != role.ID {
		t.Fatalf("launch packet = %+v, want persisted launch packet metadata", launchPacket)
	}
	if launchPacket.Assignment.RootID != "root_main" {
		t.Fatalf("launch packet assignment root = %q, want root_main", launchPacket.Assignment.RootID)
	}
	if launchPacket.Profile == nil || launchPacket.Profile.ID != profile.ID || launchPacket.ExecutionProfile == nil || launchPacket.ExecutionProfile.ID != executionProfile.ID {
		t.Fatalf("launch packet = %+v, want persisted resolved profiles", launchPacket)
	}
	if len(launchPacket.Skills) != 1 || launchPacket.Skills[0].ID != skill.ID {
		t.Fatalf("launch packet skills = %+v, want persisted resolved skill", launchPacket.Skills)
	}
	if len(launchPacket.Memory) != 1 || launchPacket.Memory[0].ID != memoryEntry.ID {
		t.Fatalf("launch packet memory = %+v, want persisted accepted memory", launchPacket.Memory)
	}
	if len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 1 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}
}

func TestStore_MemoryCandidateDecisionLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "candidate.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Candidates"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	candidate, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ProjectID:           project.ID,
		Title:               "Generated convention",
		Body:                "Use durable memory only after review.",
		SuggestedKind:       "note",
		SuggestedTrustLabel: core.MemoryTrustGenerated,
		SuggestedSourceKind: core.MemorySourceGenerated,
		SuggestedSourceID:   "run_1",
		SourceRefs: []core.MemoryCandidateSourceRef{{
			Kind: "task_run",
			ID:   "run_1",
		}},
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}
	promoted, entry, err := service.PromoteMemoryCandidate(ctx, core.MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: candidate.ID,
	})
	if err != nil {
		t.Fatalf("PromoteMemoryCandidate() error = %v", err)
	}
	if promoted.Status != core.MemoryCandidatePromoted || promoted.PromotedMemoryID != entry.ID {
		t.Fatalf("promoted candidate = %+v entry=%+v, want promoted linked entry", promoted, entry)
	}
	retried, retriedEntry, err := service.PromoteMemoryCandidate(ctx, core.MemoryCandidatePromotion{
		ProjectID:   project.ID,
		CandidateID: candidate.ID,
	})
	if err != nil {
		t.Fatalf("PromoteMemoryCandidate(retry) error = %v", err)
	}
	if retried.PromotedMemoryID != entry.ID || retriedEntry.ID != entry.ID {
		t.Fatalf("retry candidate=%+v entry=%+v, want idempotent promoted entry", retried, retriedEntry)
	}

	rejectCandidate, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{
		ProjectID: project.ID,
		Title:     "Speculative convention",
		Body:      "Maybe skip tests.",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate(reject) error = %v", err)
	}
	rejected, err := service.RejectMemoryCandidate(ctx, project.ID, rejectCandidate.ID, "Speculative.")
	if err != nil {
		t.Fatalf("RejectMemoryCandidate() error = %v", err)
	}
	if rejected.Status != core.MemoryCandidateRejected || rejected.StatusReason != "Speculative." {
		t.Fatalf("rejected candidate = %+v, want rejected reason", rejected)
	}
	if _, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID, Status: "bogus"}); !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("ListMemoryCandidates(invalid status) error = %v, want ErrInvalid", err)
	}
	pending, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(pending) error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending candidates = %+v, want none after resolution", pending)
	}
	resolved, err := service.ListMemoryCandidates(ctx, core.MemoryCandidateFilter{ProjectID: project.ID, IncludeResolved: true})
	if err != nil {
		t.Fatalf("ListMemoryCandidates(resolved) error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved candidates = %+v, want promoted and rejected", resolved)
	}
	if err := service.DeleteMemoryCandidate(ctx, project.ID, rejectCandidate.ID); err != nil {
		t.Fatalf("DeleteMemoryCandidate() error = %v", err)
	}
	if _, err := service.GetMemoryCandidate(ctx, project.ID, rejectCandidate.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetMemoryCandidate(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStore_MemoryEntryLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Memory"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	entry, err := service.CreateMemoryEntry(ctx, core.MemoryEntry{
		ProjectID:  project.ID,
		Title:      "Keep reviews concrete",
		Body:       "Accepted review memory should cite evidence.",
		TrustLabel: core.MemoryTrustGenerated,
		SourceKind: core.MemorySourceGenerated,
		SourceID:   "memcand_1",
	})
	if err != nil {
		t.Fatalf("CreateMemoryEntry() error = %v", err)
	}
	got, err := service.GetMemoryEntry(ctx, project.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetMemoryEntry() error = %v", err)
	}
	if got.ID != entry.ID || got.SourceKind != core.MemorySourceGenerated || !got.Enabled {
		t.Fatalf("got memory = %+v, want created enabled entry", got)
	}

	got.Enabled = false
	got.Title = "Keep review memory concrete"
	updated, err := service.UpdateMemoryEntry(ctx, got)
	if err != nil {
		t.Fatalf("UpdateMemoryEntry() error = %v", err)
	}
	if updated.Enabled || updated.Title != "Keep review memory concrete" || updated.CreatedAt.IsZero() {
		t.Fatalf("updated memory = %+v, want disabled updated entry preserving created_at", updated)
	}
	enabled, err := service.ListMemoryEntries(ctx, project.ID, false)
	if err != nil {
		t.Fatalf("ListMemoryEntries(enabled) error = %v", err)
	}
	if len(enabled) != 0 {
		t.Fatalf("enabled entries = %+v, want disabled entry omitted", enabled)
	}
	all, err := service.ListMemoryEntries(ctx, project.ID, true)
	if err != nil {
		t.Fatalf("ListMemoryEntries(all) error = %v", err)
	}
	if len(all) != 1 || all[0].ID != entry.ID {
		t.Fatalf("all entries = %+v, want disabled entry included", all)
	}

	if err := service.DeleteMemoryEntry(ctx, project.ID, entry.ID); err != nil {
		t.Fatalf("DeleteMemoryEntry() error = %v", err)
	}
	if _, err := service.GetMemoryEntry(ctx, project.ID, entry.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetMemoryEntry(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStore_MigrateAddsAssignmentRootID(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE assignments (
		project_id TEXT NOT NULL,
		id TEXT NOT NULL,
		work_item_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		profile_id TEXT NOT NULL DEFAULT '',
		execution_profile_id TEXT NOT NULL DEFAULT '',
		execution_mode TEXT NOT NULL,
		status TEXT NOT NULL,
		desired_agent_json TEXT NOT NULL DEFAULT '{}',
		claimed_by TEXT NOT NULL DEFAULT '',
		execution_ref TEXT NOT NULL DEFAULT '',
		context_snapshot_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (project_id, id)
	)`)
	if err != nil {
		t.Fatalf("create old assignments table error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close old db error = %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	rows, err := store.db.QueryContext(ctx, `PRAGMA table_info(assignments)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info error = %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info error = %v", err)
		}
		if name == "root_id" {
			found = true
			if columnType != "TEXT" || notNull != 1 {
				t.Fatalf("root_id column type=%q notNull=%d, want TEXT NOT NULL", columnType, notNull)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows error = %v", err)
	}
	if !found {
		t.Fatalf("assignments root_id column was not added")
	}
}

func TestStore_ClaimAssignmentRaceHasOneWinner(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "race.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Race"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Implementer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Claim once"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	const contenders = 20
	var wins atomic.Int32
	var conflicts atomic.Int32
	var wg sync.WaitGroup
	for i := range contenders {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := service.ClaimAssignment(ctx, project.ID, assignment.ID, "agent-"+string(rune('a'+i)))
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, core.ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("ClaimAssignment() unexpected error = %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != contenders-1 {
		t.Fatalf("wins=%d conflicts=%d, want one winner and %d conflicts", wins.Load(), conflicts.Load(), contenders-1)
	}
}

func TestStore_CreateAssignmentValidatesReferences(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "validation.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Validation"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Needs role"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	_, err = service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: work.ID,
		RoleID:     "role_missing",
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("CreateAssignment() error = %v, want ErrNotFound", err)
	}
}

func TestStore_CreateReviewValidatesAssignmentWorkItem(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "review-validation.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)
	project, err := service.CreateProject(ctx, core.Project{Name: "Review validation"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	workA, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "A"})
	if err != nil {
		t.Fatalf("CreateWorkItem(A) error = %v", err)
	}
	workB, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "B"})
	if err != nil {
		t.Fatalf("CreateWorkItem(B) error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:  project.ID,
		WorkItemID: workA.ID,
		RoleID:     role.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}

	_, err = service.CreateReview(ctx, core.Review{
		ProjectID:    project.ID,
		WorkItemID:   workB.ID,
		AssignmentID: assignment.ID,
		Body:         "Assignment belongs to another work item.",
		Verdict:      core.ReviewVerdictBlocked,
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("CreateReview() error = %v, want ErrNotFound", err)
	}
}
