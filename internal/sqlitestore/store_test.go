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
		ProjectID:  project.ID,
		Title:      "Persistence lesson",
		Body:       "Cairnline stores collaboration artifacts in SQLite.",
		TrustLabel: "test",
		SourceRef:  assignment.ID,
	}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
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
	memory, err := reopenedService.ListMemoryCandidates(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(memory) != 1 || memory[0].Status != core.MemoryCandidateProposed || memory[0].SourceRef != assignment.ID {
		t.Fatalf("memory candidates = %+v, want persisted candidate", memory)
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
	if len(launchPacket.Evidence) != 1 || len(launchPacket.Reviews) != 1 || len(launchPacket.Handoffs) != 1 || len(launchPacket.MemoryCandidates) != 1 {
		t.Fatalf("launch packet artifact counts evidence=%d reviews=%d handoffs=%d memory=%d, want all one", len(launchPacket.Evidence), len(launchPacket.Reviews), len(launchPacket.Handoffs), len(launchPacket.MemoryCandidates))
	}
}

func TestStore_DeleteProjectCascadesProjectScopedRows(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "cairnline.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	service := core.NewService(store)

	profile, err := service.CreateAgentProfile(ctx, core.AgentProfile{Name: "Global profile"})
	if err != nil {
		t.Fatalf("CreateAgentProfile() error = %v", err)
	}
	execution, err := service.CreateExecutionProfile(ctx, core.ExecutionProfile{Name: "Global execution"})
	if err != nil {
		t.Fatalf("CreateExecutionProfile() error = %v", err)
	}
	project, err := service.CreateProject(ctx, core.Project{Name: "Delete me"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := service.CreateProjectSkill(ctx, core.ProjectSkill{
		ProjectID:  project.ID,
		ID:         "review",
		Title:      "Review",
		Format:     core.SkillFormatMarkdown,
		Status:     core.SkillStatusAvailable,
		TrustLabel: core.SkillTrustWorkspace,
	}); err != nil {
		t.Fatalf("CreateProjectSkill() error = %v", err)
	}
	role, err := service.CreateRole(ctx, core.Role{ProjectID: project.ID, Name: "Reviewer", DefaultProfileID: profile.ID})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	work, err := service.CreateWorkItem(ctx, core.WorkItem{ProjectID: project.ID, Title: "Delete scoped rows"})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	assignment, err := service.CreateAssignment(ctx, core.Assignment{
		ProjectID:          project.ID,
		WorkItemID:         work.ID,
		RoleID:             role.ID,
		ExecutionProfileID: execution.ID,
	})
	if err != nil {
		t.Fatalf("CreateAssignment() error = %v", err)
	}
	if _, err := service.CreateEvidence(ctx, core.Evidence{ProjectID: project.ID, WorkItemID: work.ID, Title: "Evidence", Locator: "https://example.test/evidence"}); err != nil {
		t.Fatalf("CreateEvidence() error = %v", err)
	}
	if _, err := service.CreateReview(ctx, core.Review{ProjectID: project.ID, WorkItemID: work.ID, AssignmentID: assignment.ID, Body: "Pass", Verdict: core.ReviewVerdictPass}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if _, err := service.CreateHandoff(ctx, core.Handoff{ProjectID: project.ID, WorkItemID: work.ID, Title: "Handoff", Body: "Follow up"}); err != nil {
		t.Fatalf("CreateHandoff() error = %v", err)
	}
	if _, err := service.CreateMemoryCandidate(ctx, core.MemoryCandidate{ProjectID: project.ID, Title: "Memory", Body: "Remember this"}); err != nil {
		t.Fatalf("CreateMemoryCandidate() error = %v", err)
	}

	if err := service.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := service.GetProject(ctx, project.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetProject() after delete error = %v, want ErrNotFound", err)
	}
	assertGone := func(name string, count int, err error) {
		t.Helper()
		if errors.Is(err, core.ErrNotFound) {
			return
		}
		if err != nil {
			t.Fatalf("%s list error = %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0 after project delete", name, count)
		}
	}
	skills, err := service.ListProjectSkills(ctx, project.ID)
	assertGone("skills", len(skills), err)
	roles, err := service.ListRoles(ctx, project.ID)
	assertGone("roles", len(roles), err)
	workItems, err := service.ListWorkItems(ctx, project.ID)
	assertGone("work items", len(workItems), err)
	assignments, err := service.ListAssignments(ctx, project.ID)
	assertGone("assignments", len(assignments), err)
	evidence, err := service.ListEvidence(ctx, project.ID, work.ID)
	assertGone("evidence", len(evidence), err)
	reviews, err := service.ListReviews(ctx, project.ID, work.ID)
	assertGone("reviews", len(reviews), err)
	handoffs, err := service.ListHandoffs(ctx, project.ID, work.ID)
	assertGone("handoffs", len(handoffs), err)
	memory, err := service.ListMemoryCandidates(ctx, project.ID)
	assertGone("memory candidates", len(memory), err)

	profiles, err := service.ListAgentProfiles(ctx)
	if err != nil {
		t.Fatalf("ListAgentProfiles() error = %v", err)
	}
	executionProfiles, err := service.ListExecutionProfiles(ctx)
	if err != nil {
		t.Fatalf("ListExecutionProfiles() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != profile.ID || len(executionProfiles) != 1 || executionProfiles[0].ID != execution.ID {
		t.Fatalf("global profiles = %+v execution = %+v, want preserved globals", profiles, executionProfiles)
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
