package sqlitestore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hecatehq/cairnline/internal/core"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.Join(core.ErrInvalid, errors.New("sqlite path is required"))
	}
	db, err := sql.Open("sqlite", sqliteConnectionDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func sqliteConnectionDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			roots_json TEXT NOT NULL DEFAULT '[]',
			default_root_id TEXT NOT NULL DEFAULT '',
			context_sources_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_skills (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL DEFAULT '',
			root_id TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL,
			suggested_tools_json TEXT NOT NULL DEFAULT '[]',
			required_permissions_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL,
			trust_label TEXT NOT NULL DEFAULT '',
			source_refs_json TEXT NOT NULL DEFAULT '[]',
			warnings_json TEXT NOT NULL DEFAULT '[]',
			discovered_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS work_items (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			title TEXT NOT NULL,
			brief TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			owner_role_id TEXT NOT NULL DEFAULT '',
			reviewer_role_ids_json TEXT NOT NULL DEFAULT '[]',
			root_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS roles (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			instructions TEXT NOT NULL DEFAULT '',
			default_skill_ids_json TEXT NOT NULL DEFAULT '[]',
			default_execution_mode TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS assignments (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			role_id TEXT NOT NULL,
			root_id TEXT NOT NULL DEFAULT '',
			execution_mode TEXT NOT NULL,
			status TEXT NOT NULL,
			desired_agent_json TEXT NOT NULL DEFAULT '{}',
			claimed_by TEXT NOT NULL DEFAULT '',
			execution_ref TEXT NOT NULL DEFAULT '',
			context_snapshot_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT '',
			completed_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			assignment_id TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL,
			author_role_id TEXT NOT NULL DEFAULT '',
			provenance_kind TEXT NOT NULL DEFAULT '',
			trust_label TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			status_changed_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS evidence (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			assignment_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			locator TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT '',
			external_id TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			trust_label TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS reviews (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			assignment_id TEXT NOT NULL DEFAULT '',
			reviewer_role_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			verdict TEXT NOT NULL,
			risk TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS handoffs (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			source_assignment_id TEXT NOT NULL DEFAULT '',
			source_run_id TEXT NOT NULL DEFAULT '',
			source_chat_session_id TEXT NOT NULL DEFAULT '',
			source_message_id TEXT NOT NULL DEFAULT '',
			from_role_id TEXT NOT NULL DEFAULT '',
			to_role_id TEXT NOT NULL DEFAULT '',
			target_assignment_id TEXT NOT NULL DEFAULT '',
			target_work_item_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			recommended_next_action TEXT NOT NULL DEFAULT '',
			linked_artifact_ids_json TEXT NOT NULL DEFAULT '[]',
			linked_memory_ids_json TEXT NOT NULL DEFAULT '[]',
			context_refs_json TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL,
			provenance_kind TEXT NOT NULL DEFAULT '',
			trust_label TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			status_changed_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS command_receipts (
			project_id TEXT NOT NULL,
			operation TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			request_hash TEXT NOT NULL,
			response_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (project_id, operation, idempotency_key),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS memory_entries (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			trust_label TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT '',
			source_id TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS memory_candidates (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			suggested_kind TEXT NOT NULL DEFAULT '',
			suggested_trust_label TEXT NOT NULL DEFAULT '',
			suggested_source_kind TEXT NOT NULL DEFAULT '',
			suggested_source_id TEXT NOT NULL DEFAULT '',
			source_refs_json TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL,
			status_reason TEXT NOT NULL DEFAULT '',
			promoted_memory_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS assistant_proposals (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			source_id TEXT NOT NULL DEFAULT '',
			proposal_json TEXT NOT NULL,
			status TEXT NOT NULL,
			latest_result_json TEXT NOT NULL DEFAULT '',
			apply_attempts_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	if err := s.ensureColumn(ctx, "projects", "default_root_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureColumn(ctx, "assignments", "root_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureColumn(ctx, "assignments", "started_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureColumn(ctx, "assignments", "completed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureAssignmentRoleSoftReference(ctx); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureColumn(ctx, "project_skills", "suggested_tools_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureColumn(ctx, "project_skills", "required_permissions_json", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureColumn(ctx, "evidence", "assignment_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	for _, column := range []string{"source_kind", "external_id", "provider"} {
		if err := s.ensureColumn(ctx, "evidence", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{"source_assignment_id", "TEXT NOT NULL DEFAULT ''"},
		{"source_run_id", "TEXT NOT NULL DEFAULT ''"},
		{"source_chat_session_id", "TEXT NOT NULL DEFAULT ''"},
		{"source_message_id", "TEXT NOT NULL DEFAULT ''"},
		{"target_assignment_id", "TEXT NOT NULL DEFAULT ''"},
		{"target_work_item_id", "TEXT NOT NULL DEFAULT ''"},
		{"recommended_next_action", "TEXT NOT NULL DEFAULT ''"},
		{"linked_artifact_ids_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"linked_memory_ids_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"context_refs_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"provenance_kind", "TEXT NOT NULL DEFAULT ''"},
		{"trust_label", "TEXT NOT NULL DEFAULT ''"},
		{"status_changed_at", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(ctx, "handoffs", column.name, column.definition); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE handoffs SET status_changed_at = created_at WHERE status_changed_at = ''`); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{"suggested_kind", "TEXT NOT NULL DEFAULT ''"},
		{"suggested_trust_label", "TEXT NOT NULL DEFAULT ''"},
		{"suggested_source_kind", "TEXT NOT NULL DEFAULT ''"},
		{"suggested_source_id", "TEXT NOT NULL DEFAULT ''"},
		{"source_refs_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"promoted_memory_id", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(ctx, "memory_candidates", column.name, column.definition); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE memory_candidates SET status = ? WHERE status = 'proposed'`, core.MemoryCandidatePending); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}

func (s *Store) ensureAssignmentRoleSoftReference(ctx context.Context) error {
	hasRoleFK, err := s.assignmentRoleForeignKeyExists(ctx)
	if err != nil {
		return err
	}
	if !hasRoleFK {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		_, _ = s.db.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`)
	}()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	statements := []string{
		`DROP TABLE IF EXISTS assignments_new`,
		`CREATE TABLE assignments_new (
			project_id TEXT NOT NULL,
			id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			role_id TEXT NOT NULL,
			root_id TEXT NOT NULL DEFAULT '',
			execution_mode TEXT NOT NULL,
			status TEXT NOT NULL,
			desired_agent_json TEXT NOT NULL DEFAULT '{}',
			claimed_by TEXT NOT NULL DEFAULT '',
			execution_ref TEXT NOT NULL DEFAULT '',
			context_snapshot_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT '',
			completed_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, id),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id, work_item_id) REFERENCES work_items(project_id, id) ON DELETE CASCADE
		)`,
		`INSERT INTO assignments_new (project_id, id, work_item_id, role_id, root_id, execution_mode, status, desired_agent_json, claimed_by, execution_ref, context_snapshot_id, created_at, updated_at, started_at, completed_at)
			SELECT project_id, id, work_item_id, role_id, root_id, execution_mode, status, desired_agent_json, claimed_by, execution_ref, context_snapshot_id, created_at, updated_at, started_at, completed_at FROM assignments`,
		`DROP TABLE assignments`,
		`ALTER TABLE assignments_new RENAME TO assignments`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Store) assignmentRoleForeignKeyExists(ctx context.Context) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `PRAGMA foreign_key_list(assignments)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, seq int
		var tableName, fromColumn, toColumn, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &tableName, &fromColumn, &toColumn, &onUpdate, &onDelete, &match); err != nil {
			return false, err
		}
		if tableName == "roles" && fromColumn == "role_id" {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) ListProjects(ctx context.Context) ([]core.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, roots_json, default_root_id, context_sources_json, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Project
	for rows.Next() {
		item, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(ctx context.Context, id string) (core.Project, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, roots_json, default_root_id, context_sources_json, created_at, updated_at FROM projects WHERE id = ?`, id)
	return scanProject(row)
}

func (s *Store) CreateProject(ctx context.Context, project core.Project) (core.Project, error) {
	roots, err := encodeJSON(project.Roots)
	if err != nil {
		return core.Project{}, err
	}
	sources, err := encodeJSON(project.ContextSources)
	if err != nil {
		return core.Project{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO projects (id, name, description, roots_json, default_root_id, context_sources_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		project.ID, project.Name, project.Description, roots, project.DefaultRootID, sources, encodeTime(project.CreatedAt), encodeTime(project.UpdatedAt))
	if err != nil {
		return core.Project{}, mapSQLiteWriteError(err)
	}
	return project, nil
}

func (s *Store) UpdateProject(ctx context.Context, project core.Project) (core.Project, error) {
	roots, err := encodeJSON(project.Roots)
	if err != nil {
		return core.Project{}, err
	}
	sources, err := encodeJSON(project.ContextSources)
	if err != nil {
		return core.Project{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE projects SET name = ?, description = ?, roots_json = ?, default_root_id = ?, context_sources_json = ?, created_at = ?, updated_at = ? WHERE id = ?`,
		project.Name, project.Description, roots, project.DefaultRootID, sources, encodeTime(project.CreatedAt), encodeTime(project.UpdatedAt), project.ID)
	if err != nil {
		return core.Project{}, err
	}
	return project, requireAffected(result)
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	for _, table := range []string{
		"artifacts",
		"evidence",
		"reviews",
		"handoffs",
		"memory_candidates",
		"memory_entries",
		"assistant_proposals",
		"assignments",
		"project_skills",
		"work_items",
		"roles",
	} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE project_id = ?`, id); err != nil {
			rollback()
			return mapSQLiteWriteError(err)
		}
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		rollback()
		return mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListProjectSkills(ctx context.Context, projectID string) ([]core.ProjectSkill, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, id, title, description, path, root_id, format, suggested_tools_json, required_permissions_json, enabled, status, trust_label, source_refs_json, warnings_json, discovered_at, created_at, updated_at FROM project_skills WHERE project_id = ? ORDER BY id ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.ProjectSkill
	for rows.Next() {
		item, err := scanProjectSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetProjectSkill(ctx context.Context, projectID, id string) (core.ProjectSkill, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return core.ProjectSkill{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, title, description, path, root_id, format, suggested_tools_json, required_permissions_json, enabled, status, trust_label, source_refs_json, warnings_json, discovered_at, created_at, updated_at FROM project_skills WHERE project_id = ? AND id = ?`, projectID, id)
	return scanProjectSkill(row)
}

func (s *Store) CreateProjectSkill(ctx context.Context, skill core.ProjectSkill) (core.ProjectSkill, error) {
	suggestedTools, err := encodeJSON(skill.SuggestedTools)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	requiredPermissions, err := encodeJSON(skill.RequiredPermissions)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	sourceRefs, err := encodeJSON(skill.SourceRefs)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	warnings, err := encodeJSON(skill.Warnings)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO project_skills (project_id, id, title, description, path, root_id, format, suggested_tools_json, required_permissions_json, enabled, status, trust_label, source_refs_json, warnings_json, discovered_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		skill.ProjectID, skill.ID, skill.Title, skill.Description, skill.Path, skill.RootID, skill.Format, suggestedTools, requiredPermissions, skill.Enabled, skill.Status, skill.TrustLabel, sourceRefs, warnings, encodeOptionalTime(skill.DiscoveredAt), encodeTime(skill.CreatedAt), encodeTime(skill.UpdatedAt))
	if err != nil {
		return core.ProjectSkill{}, mapSQLiteWriteError(err)
	}
	return skill, nil
}

func (s *Store) UpdateProjectSkill(ctx context.Context, skill core.ProjectSkill) (core.ProjectSkill, error) {
	suggestedTools, err := encodeJSON(skill.SuggestedTools)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	requiredPermissions, err := encodeJSON(skill.RequiredPermissions)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	sourceRefs, err := encodeJSON(skill.SourceRefs)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	warnings, err := encodeJSON(skill.Warnings)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE project_skills SET title = ?, description = ?, path = ?, root_id = ?, format = ?, suggested_tools_json = ?, required_permissions_json = ?, enabled = ?, status = ?, trust_label = ?, source_refs_json = ?, warnings_json = ?, discovered_at = ?, created_at = ?, updated_at = ? WHERE project_id = ? AND id = ?`,
		skill.Title, skill.Description, skill.Path, skill.RootID, skill.Format, suggestedTools, requiredPermissions, skill.Enabled, skill.Status, skill.TrustLabel, sourceRefs, warnings, encodeOptionalTime(skill.DiscoveredAt), encodeTime(skill.CreatedAt), encodeTime(skill.UpdatedAt), skill.ProjectID, skill.ID)
	if err != nil {
		return core.ProjectSkill{}, err
	}
	return skill, requireAffected(result)
}

func (s *Store) ListWorkItems(ctx context.Context, projectID string) ([]core.WorkItem, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, id, title, brief, status, priority, owner_role_id, reviewer_role_ids_json, root_id, created_at, updated_at FROM work_items WHERE project_id = ? ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.WorkItem
	for rows.Next() {
		item, err := scanWorkItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetWorkItem(ctx context.Context, projectID, id string) (core.WorkItem, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return core.WorkItem{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, title, brief, status, priority, owner_role_id, reviewer_role_ids_json, root_id, created_at, updated_at FROM work_items WHERE project_id = ? AND id = ?`, projectID, id)
	return scanWorkItem(row)
}

func (s *Store) CreateWorkItem(ctx context.Context, item core.WorkItem) (core.WorkItem, error) {
	reviewers, err := encodeJSON(item.ReviewerRoleIDs)
	if err != nil {
		return core.WorkItem{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO work_items (project_id, id, title, brief, status, priority, owner_role_id, reviewer_role_ids_json, root_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ProjectID, item.ID, item.Title, item.Brief, item.Status, item.Priority, item.OwnerRoleID, reviewers, item.RootID, encodeTime(item.CreatedAt), encodeTime(item.UpdatedAt))
	if err != nil {
		return core.WorkItem{}, mapSQLiteWriteError(err)
	}
	return item, nil
}

func (s *Store) UpdateWorkItem(ctx context.Context, item core.WorkItem) (core.WorkItem, error) {
	reviewers, err := encodeJSON(item.ReviewerRoleIDs)
	if err != nil {
		return core.WorkItem{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE work_items SET title = ?, brief = ?, status = ?, priority = ?, owner_role_id = ?, reviewer_role_ids_json = ?, root_id = ?, created_at = ?, updated_at = ? WHERE project_id = ? AND id = ?`,
		item.Title, item.Brief, item.Status, item.Priority, item.OwnerRoleID, reviewers, item.RootID, encodeTime(item.CreatedAt), encodeTime(item.UpdatedAt), item.ProjectID, item.ID)
	if err != nil {
		return core.WorkItem{}, err
	}
	return item, requireAffected(result)
}

func (s *Store) DeleteWorkItem(ctx context.Context, projectID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	for _, table := range []string{"artifacts", "evidence", "reviews", "assignments"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE project_id = ? AND work_item_id = ?`, projectID, id); err != nil {
			rollback()
			return mapSQLiteWriteError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM handoffs WHERE project_id = ? AND (work_item_id = ? OR target_work_item_id = ?)`, projectID, id, id); err != nil {
		rollback()
		return mapSQLiteWriteError(err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM work_items WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		rollback()
		return mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListRoles(ctx context.Context, projectID string) ([]core.Role, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, id, name, description, instructions, default_skill_ids_json, default_execution_mode FROM roles WHERE project_id = ? ORDER BY name ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Role
	for rows.Next() {
		item, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetRole(ctx context.Context, projectID, id string) (core.Role, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return core.Role{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, name, description, instructions, default_skill_ids_json, default_execution_mode FROM roles WHERE project_id = ? AND id = ?`, projectID, id)
	return scanRole(row)
}

func (s *Store) CreateRole(ctx context.Context, role core.Role) (core.Role, error) {
	skills, err := encodeJSON(role.DefaultSkillIDs)
	if err != nil {
		return core.Role{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO roles (project_id, id, name, description, instructions, default_skill_ids_json, default_execution_mode) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		role.ProjectID, role.ID, role.Name, role.Description, role.Instructions, skills, role.DefaultExecutionMode)
	if err != nil {
		return core.Role{}, mapSQLiteWriteError(err)
	}
	return role, nil
}

func (s *Store) UpdateRole(ctx context.Context, role core.Role) (core.Role, error) {
	skills, err := encodeJSON(role.DefaultSkillIDs)
	if err != nil {
		return core.Role{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE roles SET name = ?, description = ?, instructions = ?, default_skill_ids_json = ?, default_execution_mode = ? WHERE project_id = ? AND id = ?`,
		role.Name, role.Description, role.Instructions, skills, role.DefaultExecutionMode, role.ProjectID, role.ID)
	if err != nil {
		return core.Role{}, err
	}
	return role, requireAffected(result)
}

func (s *Store) DeleteRole(ctx context.Context, projectID, id string) error {
	if err := s.requireProject(ctx, projectID); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM roles WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		return mapSQLiteWriteError(err)
	}
	return requireAffected(result)
}

func (s *Store) ListAssignments(ctx context.Context, projectID string) ([]core.Assignment, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, assignmentSelectSQL+` WHERE project_id = ? ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Assignment
	for rows.Next() {
		item, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetAssignment(ctx context.Context, projectID, id string) (core.Assignment, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return core.Assignment{}, err
	}
	row := s.db.QueryRowContext(ctx, assignmentSelectSQL+` WHERE project_id = ? AND id = ?`, projectID, id)
	return scanAssignment(row)
}

func (s *Store) CreateAssignment(ctx context.Context, assignment core.Assignment) (core.Assignment, error) {
	if err := s.requireWorkItem(ctx, assignment.ProjectID, assignment.WorkItemID); err != nil {
		return core.Assignment{}, err
	}
	if err := s.requireRole(ctx, assignment.ProjectID, assignment.RoleID); err != nil {
		return core.Assignment{}, err
	}
	desiredAgent, err := encodeJSON(assignment.DesiredAgent)
	if err != nil {
		return core.Assignment{}, err
	}
	executionRef, err := encodeExecutionRef(assignment.ExecutionRef)
	if err != nil {
		return core.Assignment{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO assignments (project_id, id, work_item_id, role_id, root_id, execution_mode, status, desired_agent_json, claimed_by, execution_ref, context_snapshot_id, created_at, updated_at, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		assignment.ProjectID, assignment.ID, assignment.WorkItemID, assignment.RoleID, assignment.RootID, assignment.ExecutionMode, assignment.Status, desiredAgent, assignment.ClaimedBy, executionRef, assignment.ContextSnapshotID, encodeTime(assignment.CreatedAt), encodeTime(assignment.UpdatedAt), encodeOptionalTime(assignment.StartedAt), encodeOptionalTime(assignment.CompletedAt))
	if err != nil {
		return core.Assignment{}, mapSQLiteWriteError(err)
	}
	return assignment, nil
}

// RestoreAssignmentSnapshot atomically replaces an assignment during offline
// snapshot import. Live callers must use the narrow transition methods below.
func (s *Store) RestoreAssignmentSnapshot(ctx context.Context, assignment core.Assignment) (core.Assignment, error) {
	if err := s.requireWorkItem(ctx, assignment.ProjectID, assignment.WorkItemID); err != nil {
		return core.Assignment{}, err
	}
	if err := s.requireRole(ctx, assignment.ProjectID, assignment.RoleID); err != nil {
		return core.Assignment{}, err
	}
	desiredAgent, err := encodeJSON(assignment.DesiredAgent)
	if err != nil {
		return core.Assignment{}, err
	}
	executionRef, err := encodeExecutionRef(assignment.ExecutionRef)
	if err != nil {
		return core.Assignment{}, err
	}
	row := s.db.QueryRowContext(ctx, `UPDATE assignments
		SET work_item_id = ?, role_id = ?, root_id = ?, execution_mode = ?, status = ?, desired_agent_json = ?, claimed_by = ?, execution_ref = ?, context_snapshot_id = ?, created_at = ?, updated_at = ?, started_at = ?, completed_at = ?
		WHERE project_id = ? AND id = ?
		RETURNING `+assignmentColumnsSQL,
		assignment.WorkItemID, assignment.RoleID, assignment.RootID, assignment.ExecutionMode, assignment.Status, desiredAgent, assignment.ClaimedBy, executionRef, assignment.ContextSnapshotID, encodeTime(assignment.CreatedAt), encodeTime(assignment.UpdatedAt), encodeOptionalTime(assignment.StartedAt), encodeOptionalTime(assignment.CompletedAt), assignment.ProjectID, assignment.ID)
	item, err := scanAssignment(row)
	if err != nil {
		return core.Assignment{}, mapSQLiteWriteError(err)
	}
	return item, nil
}

func (s *Store) UpdateQueuedAssignment(ctx context.Context, projectID, id string, update core.QueuedAssignmentUpdate, now func() time.Time) (core.Assignment, error) {
	if err := s.requireWorkItem(ctx, projectID, update.Replacement.WorkItemID); err != nil {
		return core.Assignment{}, err
	}
	if err := s.requireRole(ctx, projectID, update.Replacement.RoleID); err != nil {
		return core.Assignment{}, err
	}
	desiredAgent, err := encodeJSON(update.Replacement.DesiredAgent)
	if err != nil {
		return core.Assignment{}, err
	}
	expectedDesiredAgent, err := encodeJSON(update.Expected.DesiredAgent)
	if err != nil {
		return core.Assignment{}, err
	}
	tx, current, err := s.beginAssignmentTransition(ctx, projectID, id)
	if err != nil {
		return core.Assignment{}, err
	}
	stamp := assignmentTransitionTime(current, now)
	row := tx.QueryRowContext(ctx, `UPDATE assignments
		SET work_item_id = ?, role_id = ?, root_id = ?, execution_mode = ?, desired_agent_json = ?, updated_at = ?
		WHERE project_id = ? AND id = ?
			AND work_item_id = ? AND role_id = ? AND root_id = ? AND execution_mode = ?
			AND status = ? AND claimed_by = '' AND desired_agent_json = ?
			AND execution_ref = '' AND context_snapshot_id = ''
			AND updated_at = ? AND started_at = '' AND completed_at = ''
		RETURNING `+assignmentColumnsSQL,
		update.Replacement.WorkItemID, update.Replacement.RoleID, update.Replacement.RootID, update.Replacement.ExecutionMode, desiredAgent, encodeTime(stamp),
		projectID, id,
		update.Expected.WorkItemID, update.Expected.RoleID, update.Expected.RootID, update.Expected.ExecutionMode,
		core.AssignmentQueued, expectedDesiredAgent,
		encodeTime(update.ExpectedUpdatedAt))
	return finishAssignmentTransition(tx, row)
}

func (s *Store) ClaimAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (core.Assignment, error) {
	tx, current, err := s.beginAssignmentTransition(ctx, projectID, id)
	if err != nil {
		return core.Assignment{}, err
	}
	if current.Status != core.AssignmentQueued {
		_ = tx.Rollback()
		return core.Assignment{}, core.ErrConflict
	}
	stamp := assignmentTransitionTime(current, now)
	row := tx.QueryRowContext(ctx, `UPDATE assignments
		SET status = ?, claimed_by = ?, updated_at = ?
		WHERE project_id = ? AND id = ? AND status = ? AND updated_at = ?
		RETURNING `+assignmentColumnsSQL,
		core.AssignmentClaimed, claimedBy, encodeTime(stamp),
		projectID, id, current.Status, encodeTime(current.UpdatedAt))
	return finishAssignmentTransition(tx, row)
}

func (s *Store) PrepareAssignment(ctx context.Context, projectID, id string, preparation core.AssignmentPreparation, now func() time.Time) (core.Assignment, error) {
	tx, current, err := s.beginAssignmentTransition(ctx, projectID, id)
	if err != nil {
		return core.Assignment{}, err
	}
	if current.Status != core.AssignmentClaimed || current.ClaimedBy != preparation.ClaimedBy {
		_ = tx.Rollback()
		return core.Assignment{}, core.ErrConflict
	}
	executionRef := current.ExecutionRef
	if !preparation.ExecutionRef.Empty() {
		executionRef = preparation.ExecutionRef
	}
	encodedRef, err := encodeExecutionRef(executionRef)
	if err != nil {
		_ = tx.Rollback()
		return core.Assignment{}, err
	}
	contextSnapshotID := current.ContextSnapshotID
	if preparation.ContextSnapshotID != "" {
		contextSnapshotID = preparation.ContextSnapshotID
	}
	stamp := assignmentTransitionTime(current, now)
	row := tx.QueryRowContext(ctx, `UPDATE assignments
		SET execution_ref = ?, context_snapshot_id = ?, updated_at = ?
		WHERE project_id = ? AND id = ? AND status = ? AND claimed_by = ? AND updated_at = ?
		RETURNING `+assignmentColumnsSQL,
		encodedRef, contextSnapshotID, encodeTime(stamp),
		projectID, id, current.Status, current.ClaimedBy, encodeTime(current.UpdatedAt))
	return finishAssignmentTransition(tx, row)
}

func (s *Store) ReleaseAssignment(ctx context.Context, projectID, id, claimedBy string, now func() time.Time) (core.Assignment, error) {
	tx, current, err := s.beginAssignmentTransition(ctx, projectID, id)
	if err != nil {
		return core.Assignment{}, err
	}
	if current.Status != core.AssignmentClaimed || current.ClaimedBy != claimedBy {
		_ = tx.Rollback()
		return core.Assignment{}, core.ErrConflict
	}
	stamp := assignmentTransitionTime(current, now)
	row := tx.QueryRowContext(ctx, `UPDATE assignments
		SET status = ?, claimed_by = '', execution_ref = '', context_snapshot_id = '', started_at = '', completed_at = '', updated_at = ?
		WHERE project_id = ? AND id = ? AND status = ? AND claimed_by = ? AND updated_at = ?
		RETURNING `+assignmentColumnsSQL,
		core.AssignmentQueued, encodeTime(stamp),
		projectID, id, current.Status, current.ClaimedBy, encodeTime(current.UpdatedAt))
	return finishAssignmentTransition(tx, row)
}

func (s *Store) CompleteAssignment(ctx context.Context, projectID, id, status string, executionRef core.ExecutionRef, now func() time.Time) (core.Assignment, error) {
	tx, current, err := s.beginAssignmentTransition(ctx, projectID, id)
	if err != nil {
		return core.Assignment{}, err
	}
	if terminalAssignmentStatus(current.Status) {
		_ = tx.Rollback()
		return core.Assignment{}, core.ErrConflict
	}
	stamp := assignmentTransitionTime(current, now)
	startedAt := current.StartedAt
	if startedAt.IsZero() && !(current.Status == core.AssignmentQueued && status == core.AssignmentCancelled) {
		startedAt = stamp
	}
	completedAt := current.CompletedAt
	if terminalAssignmentStatus(status) && completedAt.IsZero() {
		completedAt = stamp
	}
	ref := current.ExecutionRef
	if !executionRef.Empty() {
		ref = executionRef
	}
	encodedRef, err := encodeExecutionRef(ref)
	if err != nil {
		_ = tx.Rollback()
		return core.Assignment{}, err
	}
	row := tx.QueryRowContext(ctx, `UPDATE assignments
		SET status = ?, started_at = ?, completed_at = ?, execution_ref = ?, updated_at = ?
		WHERE project_id = ? AND id = ? AND status = ? AND updated_at = ?
		RETURNING `+assignmentColumnsSQL,
		status, encodeOptionalTime(startedAt), encodeOptionalTime(completedAt), encodedRef, encodeTime(stamp),
		projectID, id, current.Status, encodeTime(current.UpdatedAt))
	return finishAssignmentTransition(tx, row)
}

func (s *Store) UpdateAssignmentStatus(ctx context.Context, projectID, id, status string, executionRef core.ExecutionRef, now func() time.Time) (core.Assignment, error) {
	tx, current, err := s.beginAssignmentTransition(ctx, projectID, id)
	if err != nil {
		return core.Assignment{}, err
	}
	if current.Status == core.AssignmentQueued || terminalAssignmentStatus(current.Status) {
		_ = tx.Rollback()
		return core.Assignment{}, core.ErrConflict
	}
	stamp := assignmentTransitionTime(current, now)
	startedAt := current.StartedAt
	if startedAt.IsZero() {
		startedAt = stamp
	}
	ref := current.ExecutionRef
	if !executionRef.Empty() {
		ref = executionRef
	}
	encodedRef, err := encodeExecutionRef(ref)
	if err != nil {
		_ = tx.Rollback()
		return core.Assignment{}, err
	}
	row := tx.QueryRowContext(ctx, `UPDATE assignments
		SET status = ?, started_at = ?, execution_ref = ?, updated_at = ?
		WHERE project_id = ? AND id = ? AND status = ? AND updated_at = ?
		RETURNING `+assignmentColumnsSQL,
		status, encodeTime(startedAt), encodedRef, encodeTime(stamp),
		projectID, id, current.Status, encodeTime(current.UpdatedAt))
	return finishAssignmentTransition(tx, row)
}

type assignmentTransition struct {
	conn   *sql.Conn
	active bool
}

func (s *Store) beginImmediateTransition(ctx context.Context) (*assignmentTransition, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	transition := &assignmentTransition{conn: conn}
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		_ = conn.Close()
		return nil, mapSQLiteWriteError(err)
	}
	transition.active = true
	return transition, nil
}

func (s *Store) beginAssignmentTransition(ctx context.Context, projectID, id string) (*assignmentTransition, core.Assignment, error) {
	transition, err := s.beginImmediateTransition(ctx)
	if err != nil {
		return nil, core.Assignment{}, err
	}
	current, err := scanAssignment(transition.QueryRowContext(ctx, assignmentSelectSQL+` WHERE project_id = ? AND id = ?`, projectID, id))
	if err != nil {
		_ = transition.Rollback()
		return nil, core.Assignment{}, err
	}
	return transition, current, nil
}

func (s *Store) beginHandoffTransition(ctx context.Context, projectID, workItemID, id string) (*assignmentTransition, core.Handoff, error) {
	transition, err := s.beginImmediateTransition(ctx)
	if err != nil {
		return nil, core.Handoff{}, err
	}
	current, err := scanHandoff(transition.QueryRowContext(ctx, handoffSelectSQL+` WHERE project_id = ? AND work_item_id = ? AND id = ?`, projectID, workItemID, id))
	if err != nil {
		_ = transition.Rollback()
		return nil, core.Handoff{}, err
	}
	return transition, current, nil
}

func (transition *assignmentTransition) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return transition.conn.QueryRowContext(ctx, query, args...)
}

func (transition *assignmentTransition) Rollback() error {
	if !transition.active {
		return nil
	}
	finalizeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, rollbackErr := transition.conn.ExecContext(finalizeCtx, `ROLLBACK`)
	if rollbackErr != nil {
		transition.discard()
		return rollbackErr
	}
	transition.active = false
	closeErr := transition.conn.Close()
	return closeErr
}

func (transition *assignmentTransition) Commit() error {
	if !transition.active {
		return nil
	}
	commitCtx, cancelCommit := context.WithTimeout(context.Background(), 5*time.Second)
	_, commitErr := transition.conn.ExecContext(commitCtx, `COMMIT`)
	cancelCommit()
	if commitErr != nil {
		rollbackCtx, cancelRollback := context.WithTimeout(context.Background(), 5*time.Second)
		_, rollbackErr := transition.conn.ExecContext(rollbackCtx, `ROLLBACK`)
		cancelRollback()
		if rollbackErr != nil {
			transition.discard()
			return commitErr
		}
		transition.active = false
		_ = transition.conn.Close()
		return commitErr
	}
	transition.active = false
	closeErr := transition.conn.Close()
	return closeErr
}

func (transition *assignmentTransition) discard() {
	transition.active = false
	_ = transition.conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = transition.conn.Close()
}

func finishAssignmentTransition(transition *assignmentTransition, row *sql.Row) (core.Assignment, error) {
	item, err := scanAssignment(row)
	if err != nil {
		_ = transition.Rollback()
		if errors.Is(err, core.ErrNotFound) {
			return core.Assignment{}, core.ErrConflict
		}
		return core.Assignment{}, mapSQLiteWriteError(err)
	}
	if err := transition.Commit(); err != nil {
		return core.Assignment{}, mapSQLiteWriteError(err)
	}
	return item, nil
}

func finishHandoffTransition(transition *assignmentTransition, row *sql.Row, notFoundIsConflict bool) (core.Handoff, error) {
	item, err := scanHandoff(row)
	if err != nil {
		_ = transition.Rollback()
		if notFoundIsConflict && errors.Is(err, core.ErrNotFound) {
			return core.Handoff{}, core.ErrConflict
		}
		return core.Handoff{}, mapSQLiteWriteError(err)
	}
	if err := transition.Commit(); err != nil {
		return core.Handoff{}, mapSQLiteWriteError(err)
	}
	return item, nil
}

func assignmentTransitionTime(current core.Assignment, now func() time.Time) time.Time {
	stamp := time.Now().UTC()
	if now != nil {
		stamp = now()
	}
	if stamp.Before(current.StartedAt) {
		stamp = current.StartedAt
	}
	if !stamp.After(current.UpdatedAt) {
		stamp = current.UpdatedAt.Add(time.Nanosecond)
	}
	return stamp
}

func handoffTransitionTime(current core.Handoff, now func() time.Time) time.Time {
	stamp := time.Now().UTC()
	if now != nil {
		stamp = now()
	}
	for _, floor := range []time.Time{current.CreatedAt, current.UpdatedAt, current.StatusChangedAt} {
		if stamp.Before(floor) {
			stamp = floor
		}
	}
	if !stamp.After(current.UpdatedAt) {
		stamp = current.UpdatedAt.Add(time.Nanosecond)
	}
	return stamp
}

func terminalAssignmentStatus(status string) bool {
	switch status {
	case core.AssignmentCompleted, core.AssignmentFailed, core.AssignmentCancelled:
		return true
	default:
		return false
	}
}

func (s *Store) DeleteAssignment(ctx context.Context, projectID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM artifacts WHERE project_id = ? AND assignment_id = ?`, projectID, id); err != nil {
		rollback()
		return mapSQLiteWriteError(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM reviews WHERE project_id = ? AND assignment_id = ?`, projectID, id); err != nil {
		rollback()
		return mapSQLiteWriteError(err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM assignments WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		rollback()
		return mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListArtifacts(ctx context.Context, projectID, workItemID string) ([]core.Artifact, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, id, work_item_id, assignment_id, kind, title, body, author_role_id, provenance_kind, trust_label, created_at, updated_at FROM artifacts WHERE project_id = ? AND work_item_id = ? ORDER BY created_at ASC, id ASC`, projectID, workItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Artifact
	for rows.Next() {
		item, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetArtifact(ctx context.Context, projectID, workItemID, id string) (core.Artifact, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return core.Artifact{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, work_item_id, assignment_id, kind, title, body, author_role_id, provenance_kind, trust_label, created_at, updated_at FROM artifacts WHERE project_id = ? AND work_item_id = ? AND id = ?`, projectID, workItemID, id)
	return scanArtifact(row)
}

func (s *Store) CreateArtifact(ctx context.Context, artifact core.Artifact) (core.Artifact, error) {
	if err := s.requireWorkItem(ctx, artifact.ProjectID, artifact.WorkItemID); err != nil {
		return core.Artifact{}, err
	}
	if artifact.AssignmentID != "" {
		assignment, err := s.GetAssignment(ctx, artifact.ProjectID, artifact.AssignmentID)
		if err != nil {
			return core.Artifact{}, err
		}
		if assignment.WorkItemID != artifact.WorkItemID {
			return core.Artifact{}, core.ErrNotFound
		}
	}
	if artifact.AuthorRoleID != "" {
		if _, err := s.GetRole(ctx, artifact.ProjectID, artifact.AuthorRoleID); err != nil {
			return core.Artifact{}, err
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO artifacts (project_id, id, work_item_id, assignment_id, kind, title, body, author_role_id, provenance_kind, trust_label, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ProjectID, artifact.ID, artifact.WorkItemID, artifact.AssignmentID, artifact.Kind, artifact.Title, artifact.Body, artifact.AuthorRoleID, artifact.ProvenanceKind, artifact.TrustLabel, encodeTime(artifact.CreatedAt), encodeTime(artifact.UpdatedAt))
	if err != nil {
		return core.Artifact{}, mapSQLiteWriteError(err)
	}
	return artifact, nil
}

func (s *Store) ListEvidence(ctx context.Context, projectID, workItemID string) ([]core.Evidence, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, id, work_item_id, assignment_id, title, body, locator, source_kind, external_id, provider, trust_label, created_at, updated_at FROM evidence WHERE project_id = ? AND work_item_id = ? ORDER BY updated_at DESC`, projectID, workItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Evidence
	for rows.Next() {
		item, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetEvidence(ctx context.Context, projectID, workItemID, id string) (core.Evidence, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return core.Evidence{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, work_item_id, assignment_id, title, body, locator, source_kind, external_id, provider, trust_label, created_at, updated_at FROM evidence WHERE project_id = ? AND work_item_id = ? AND id = ?`, projectID, workItemID, id)
	return scanEvidence(row)
}

func (s *Store) CreateEvidence(ctx context.Context, evidence core.Evidence) (core.Evidence, error) {
	if err := s.requireWorkItem(ctx, evidence.ProjectID, evidence.WorkItemID); err != nil {
		return core.Evidence{}, err
	}
	if evidence.AssignmentID != "" {
		assignment, err := s.GetAssignment(ctx, evidence.ProjectID, evidence.AssignmentID)
		if err != nil {
			return core.Evidence{}, err
		}
		if assignment.WorkItemID != evidence.WorkItemID {
			return core.Evidence{}, core.ErrNotFound
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO evidence (project_id, id, work_item_id, assignment_id, title, body, locator, source_kind, external_id, provider, trust_label, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evidence.ProjectID, evidence.ID, evidence.WorkItemID, evidence.AssignmentID, evidence.Title, evidence.Body, evidence.Locator, evidence.SourceKind, evidence.ExternalID, evidence.Provider, evidence.TrustLabel, encodeTime(evidence.CreatedAt), encodeTime(evidence.UpdatedAt))
	if err != nil {
		return core.Evidence{}, mapSQLiteWriteError(err)
	}
	return evidence, nil
}

func (s *Store) ListReviews(ctx context.Context, projectID, workItemID string) ([]core.Review, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, id, work_item_id, assignment_id, reviewer_role_id, title, body, verdict, risk, status, created_at, updated_at FROM reviews WHERE project_id = ? AND work_item_id = ? ORDER BY updated_at DESC`, projectID, workItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Review
	for rows.Next() {
		item, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetReview(ctx context.Context, projectID, workItemID, id string) (core.Review, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return core.Review{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, work_item_id, assignment_id, reviewer_role_id, title, body, verdict, risk, status, created_at, updated_at FROM reviews WHERE project_id = ? AND work_item_id = ? AND id = ?`, projectID, workItemID, id)
	return scanReview(row)
}

func (s *Store) CreateReview(ctx context.Context, review core.Review) (core.Review, error) {
	if err := s.requireWorkItem(ctx, review.ProjectID, review.WorkItemID); err != nil {
		return core.Review{}, err
	}
	if review.AssignmentID != "" {
		assignment, err := s.GetAssignment(ctx, review.ProjectID, review.AssignmentID)
		if err != nil {
			return core.Review{}, err
		}
		if assignment.WorkItemID != review.WorkItemID {
			return core.Review{}, core.ErrNotFound
		}
	}
	if review.ReviewerRoleID != "" {
		if _, err := s.GetRole(ctx, review.ProjectID, review.ReviewerRoleID); err != nil {
			return core.Review{}, err
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO reviews (project_id, id, work_item_id, assignment_id, reviewer_role_id, title, body, verdict, risk, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		review.ProjectID, review.ID, review.WorkItemID, review.AssignmentID, review.ReviewerRoleID, review.Title, review.Body, review.Verdict, review.Risk, review.Status, encodeTime(review.CreatedAt), encodeTime(review.UpdatedAt))
	if err != nil {
		return core.Review{}, mapSQLiteWriteError(err)
	}
	return review, nil
}

func (s *Store) ListHandoffs(ctx context.Context, projectID, workItemID string) ([]core.Handoff, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, handoffSelectSQL+` WHERE project_id = ? AND work_item_id = ? ORDER BY updated_at DESC`, projectID, workItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Handoff
	for rows.Next() {
		item, err := scanHandoff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetHandoff(ctx context.Context, projectID, workItemID, id string) (core.Handoff, error) {
	if err := s.requireWorkItem(ctx, projectID, workItemID); err != nil {
		return core.Handoff{}, err
	}
	row := s.db.QueryRowContext(ctx, handoffSelectSQL+` WHERE project_id = ? AND work_item_id = ? AND id = ?`, projectID, workItemID, id)
	return scanHandoff(row)
}

func (s *Store) CreateHandoff(ctx context.Context, handoff core.Handoff) (core.Handoff, error) {
	if err := s.requireWorkItem(ctx, handoff.ProjectID, handoff.WorkItemID); err != nil {
		return core.Handoff{}, err
	}
	if handoff.SourceAssignmentID != "" {
		assignment, err := s.GetAssignment(ctx, handoff.ProjectID, handoff.SourceAssignmentID)
		if err != nil {
			return core.Handoff{}, err
		}
		if assignment.WorkItemID != handoff.WorkItemID {
			return core.Handoff{}, core.ErrNotFound
		}
	}
	if handoff.FromRoleID != "" {
		if _, err := s.GetRole(ctx, handoff.ProjectID, handoff.FromRoleID); err != nil {
			return core.Handoff{}, err
		}
	}
	if handoff.ToRoleID != "" {
		if _, err := s.GetRole(ctx, handoff.ProjectID, handoff.ToRoleID); err != nil {
			return core.Handoff{}, err
		}
	}
	if handoff.TargetAssignmentID != "" {
		assignment, err := s.GetAssignment(ctx, handoff.ProjectID, handoff.TargetAssignmentID)
		if err != nil {
			return core.Handoff{}, err
		}
		if handoff.TargetWorkItemID != "" && assignment.WorkItemID != handoff.TargetWorkItemID {
			return core.Handoff{}, core.ErrNotFound
		}
	}
	if handoff.TargetWorkItemID != "" {
		if err := s.requireWorkItem(ctx, handoff.ProjectID, handoff.TargetWorkItemID); err != nil {
			return core.Handoff{}, err
		}
	}
	linkedArtifactIDs, err := encodeJSON(handoff.LinkedArtifactIDs)
	if err != nil {
		return core.Handoff{}, err
	}
	linkedMemoryIDs, err := encodeJSON(handoff.LinkedMemoryIDs)
	if err != nil {
		return core.Handoff{}, err
	}
	contextRefs, err := encodeJSON(handoff.ContextRefs)
	if err != nil {
		return core.Handoff{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO handoffs (project_id, id, work_item_id, source_assignment_id, source_run_id, source_chat_session_id, source_message_id, from_role_id, to_role_id, target_assignment_id, target_work_item_id, title, body, recommended_next_action, linked_artifact_ids_json, linked_memory_ids_json, context_refs_json, status, provenance_kind, trust_label, created_at, updated_at, status_changed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		handoff.ProjectID, handoff.ID, handoff.WorkItemID, handoff.SourceAssignmentID, handoff.SourceRunID, handoff.SourceChatSessionID, handoff.SourceMessageID, handoff.FromRoleID, handoff.ToRoleID, handoff.TargetAssignmentID, handoff.TargetWorkItemID, handoff.Title, handoff.Body, handoff.RecommendedNextAction, linkedArtifactIDs, linkedMemoryIDs, contextRefs, handoff.Status, handoff.ProvenanceKind, handoff.TrustLabel, encodeTime(handoff.CreatedAt), encodeTime(handoff.UpdatedAt), encodeTime(handoff.StatusChangedAt))
	if err != nil {
		return core.Handoff{}, mapSQLiteWriteError(err)
	}
	return handoff, nil
}

func validateHandoffTransition(ctx context.Context, transition *assignmentTransition, handoff core.Handoff) error {
	if handoff.SourceAssignmentID != "" {
		assignment, err := scanAssignment(transition.conn.QueryRowContext(ctx, assignmentSelectSQL+` WHERE project_id = ? AND id = ?`, handoff.ProjectID, handoff.SourceAssignmentID))
		if err != nil {
			return err
		}
		if assignment.WorkItemID != handoff.WorkItemID {
			return core.ErrNotFound
		}
	}
	for _, roleID := range []string{handoff.FromRoleID, handoff.ToRoleID} {
		if roleID == "" {
			continue
		}
		if _, err := scanRole(transition.conn.QueryRowContext(ctx, `SELECT project_id, id, name, description, instructions, default_skill_ids_json, default_execution_mode FROM roles WHERE project_id = ? AND id = ?`, handoff.ProjectID, roleID)); err != nil {
			return err
		}
	}
	if handoff.TargetAssignmentID != "" {
		assignment, err := scanAssignment(transition.conn.QueryRowContext(ctx, assignmentSelectSQL+` WHERE project_id = ? AND id = ?`, handoff.ProjectID, handoff.TargetAssignmentID))
		if err != nil {
			return err
		}
		if handoff.TargetWorkItemID != "" && assignment.WorkItemID != handoff.TargetWorkItemID {
			return core.ErrNotFound
		}
	}
	if handoff.TargetWorkItemID != "" {
		if _, err := scanWorkItem(transition.conn.QueryRowContext(ctx, `SELECT project_id, id, title, brief, status, priority, owner_role_id, reviewer_role_ids_json, root_id, created_at, updated_at FROM work_items WHERE project_id = ? AND id = ?`, handoff.ProjectID, handoff.TargetWorkItemID)); err != nil {
			return err
		}
	}
	return nil
}

func updateHandoffRow(ctx context.Context, transition *assignmentTransition, handoff core.Handoff, expectedUpdatedAt string) (*sql.Row, error) {
	linkedArtifactIDs, err := encodeJSON(handoff.LinkedArtifactIDs)
	if err != nil {
		return nil, err
	}
	linkedMemoryIDs, err := encodeJSON(handoff.LinkedMemoryIDs)
	if err != nil {
		return nil, err
	}
	contextRefs, err := encodeJSON(handoff.ContextRefs)
	if err != nil {
		return nil, err
	}
	args := []any{
		handoff.SourceAssignmentID, handoff.SourceRunID, handoff.SourceChatSessionID, handoff.SourceMessageID,
		handoff.FromRoleID, handoff.ToRoleID, handoff.TargetAssignmentID, handoff.TargetWorkItemID,
		handoff.Title, handoff.Body, handoff.RecommendedNextAction, linkedArtifactIDs, linkedMemoryIDs, contextRefs,
		handoff.Status, handoff.ProvenanceKind, handoff.TrustLabel, encodeTime(handoff.CreatedAt), encodeTime(handoff.UpdatedAt),
		encodeTime(handoff.StatusChangedAt), handoff.ProjectID, handoff.WorkItemID, handoff.ID,
	}
	query := `UPDATE handoffs SET source_assignment_id = ?, source_run_id = ?, source_chat_session_id = ?, source_message_id = ?, from_role_id = ?, to_role_id = ?, target_assignment_id = ?, target_work_item_id = ?, title = ?, body = ?, recommended_next_action = ?, linked_artifact_ids_json = ?, linked_memory_ids_json = ?, context_refs_json = ?, status = ?, provenance_kind = ?, trust_label = ?, created_at = ?, updated_at = ?, status_changed_at = ? WHERE project_id = ? AND work_item_id = ? AND id = ?`
	if expectedUpdatedAt != "" {
		query += ` AND updated_at = ?`
		args = append(args, expectedUpdatedAt)
	}
	query += ` RETURNING ` + handoffColumnsSQL
	return transition.conn.QueryRowContext(ctx, query, args...), nil
}

// RestoreHandoffSnapshot atomically replaces a handoff during offline snapshot
// import. Live callers must use UpdateHandoff's compare-and-set contract.
func (s *Store) RestoreHandoffSnapshot(ctx context.Context, handoff core.Handoff) (core.Handoff, error) {
	transition, _, err := s.beginHandoffTransition(ctx, handoff.ProjectID, handoff.WorkItemID, handoff.ID)
	if err != nil {
		return core.Handoff{}, err
	}
	if err := validateHandoffTransition(ctx, transition, handoff); err != nil {
		_ = transition.Rollback()
		return core.Handoff{}, err
	}
	row, err := updateHandoffRow(ctx, transition, handoff, "")
	if err != nil {
		_ = transition.Rollback()
		return core.Handoff{}, err
	}
	return finishHandoffTransition(transition, row, false)
}

func (s *Store) UpdateHandoff(ctx context.Context, projectID, workItemID, id string, update core.HandoffUpdate, now func() time.Time) (core.Handoff, error) {
	transition, current, err := s.beginHandoffTransition(ctx, projectID, workItemID, id)
	if err != nil {
		return core.Handoff{}, err
	}
	if !current.UpdatedAt.Equal(update.ExpectedUpdatedAt) {
		_ = transition.Rollback()
		return core.Handoff{}, core.ErrConflict
	}
	replacement := update.Patch.Apply(current)
	if current.SameContent(replacement) {
		_ = transition.Rollback()
		return current, nil
	}
	if err := validateHandoffTransition(ctx, transition, replacement); err != nil {
		_ = transition.Rollback()
		return core.Handoff{}, err
	}
	replacement.CreatedAt = current.CreatedAt
	replacement.UpdatedAt = handoffTransitionTime(current, now)
	replacement.StatusChangedAt = current.StatusChangedAt
	if replacement.StatusChangedAt.IsZero() {
		replacement.StatusChangedAt = current.CreatedAt
	}
	if replacement.Status != current.Status {
		replacement.StatusChangedAt = replacement.UpdatedAt
	}
	row, err := updateHandoffRow(ctx, transition, replacement, encodeTime(update.ExpectedUpdatedAt))
	if err != nil {
		_ = transition.Rollback()
		return core.Handoff{}, err
	}
	return finishHandoffTransition(transition, row, true)
}

func (s *Store) DeleteHandoff(ctx context.Context, projectID, workItemID, id string, deletion core.HandoffDelete) error {
	transition, current, err := s.beginHandoffTransition(ctx, projectID, workItemID, id)
	if err != nil {
		return err
	}
	if !current.UpdatedAt.Equal(deletion.ExpectedUpdatedAt) {
		_ = transition.Rollback()
		return core.ErrConflict
	}
	result, err := transition.conn.ExecContext(ctx, `DELETE FROM handoffs WHERE project_id = ? AND work_item_id = ? AND id = ? AND updated_at = ?`, projectID, workItemID, id, encodeTime(deletion.ExpectedUpdatedAt))
	if err != nil {
		_ = transition.Rollback()
		return mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		_ = transition.Rollback()
		return core.ErrConflict
	}
	if err := transition.Commit(); err != nil {
		return mapSQLiteWriteError(err)
	}
	return nil
}

func (s *Store) AcceptHandoffWithFollowUp(ctx context.Context, command core.AcceptHandoffWithFollowUpCommand, newAssignmentID string, now func() time.Time) (core.HandoffFollowUpResult, error) {
	transition, err := s.beginImmediateTransition(ctx)
	if err != nil {
		return core.HandoffFollowUpResult{}, err
	}
	rollback := func(err error) (core.HandoffFollowUpResult, error) {
		_ = transition.Rollback()
		return core.HandoffFollowUpResult{}, err
	}

	requestHash := command.RequestHash()
	var storedHash, responseJSON string
	receiptErr := transition.conn.QueryRowContext(ctx, `SELECT request_hash, response_json FROM command_receipts WHERE project_id = ? AND operation = ? AND idempotency_key = ?`, command.ProjectID, acceptHandoffWithFollowUpOperation, command.IdempotencyKey).Scan(&storedHash, &responseJSON)
	switch {
	case receiptErr == nil:
		if storedHash != requestHash {
			return rollback(core.ErrConflict)
		}
		var replay core.HandoffFollowUpResult
		if err := json.Unmarshal([]byte(responseJSON), &replay); err != nil {
			return rollback(fmt.Errorf("decode handoff command receipt: %w", err))
		}
		currentHandoff, err := scanHandoff(transition.conn.QueryRowContext(ctx, handoffSelectSQL+` WHERE project_id = ? AND work_item_id = ? AND id = ?`, command.ProjectID, command.WorkItemID, command.HandoffID))
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				err = core.ErrConflict
			}
			return rollback(err)
		}
		currentAssignment, err := scanAssignment(transition.conn.QueryRowContext(ctx, assignmentSelectSQL+` WHERE project_id = ? AND id = ?`, command.ProjectID, replay.Assignment.ID))
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				err = core.ErrConflict
			}
			return rollback(err)
		}
		if replay.Assignment.ID == "" || currentHandoff.Status != core.HandoffStatusAccepted || currentHandoff.TargetAssignmentID != replay.Assignment.ID || currentHandoff.TargetWorkItemID != currentAssignment.WorkItemID || currentHandoff.ToRoleID == "" || currentHandoff.ToRoleID != currentAssignment.RoleID {
			return rollback(core.ErrConflict)
		}
		replay.Handoff = currentHandoff
		replay.Assignment = currentAssignment
		replay.Replayed = true
		_ = transition.Rollback()
		return replay, nil
	case !errors.Is(receiptErr, sql.ErrNoRows):
		return rollback(mapSQLiteReadError(receiptErr))
	}

	current, err := scanHandoff(transition.conn.QueryRowContext(ctx, handoffSelectSQL+` WHERE project_id = ? AND work_item_id = ? AND id = ?`, command.ProjectID, command.WorkItemID, command.HandoffID))
	if err != nil {
		return rollback(err)
	}
	if !current.UpdatedAt.Equal(command.ExpectedUpdatedAt) {
		return rollback(core.ErrConflict)
	}
	if current.Status == core.HandoffStatusDismissed || current.Status == core.HandoffStatusSuperseded {
		return rollback(core.ErrConflict)
	}
	if current.ToRoleID == "" {
		return rollback(errors.Join(core.ErrInvalid, errors.New("handoff to_role_id is required")))
	}
	role, err := scanRole(transition.conn.QueryRowContext(ctx, `SELECT project_id, id, name, description, instructions, default_skill_ids_json, default_execution_mode FROM roles WHERE project_id = ? AND id = ?`, command.ProjectID, current.ToRoleID))
	if err != nil {
		return rollback(err)
	}

	var assignment core.Assignment
	outcome := core.HandoffFollowUpCreated
	if current.TargetAssignmentID != "" {
		assignment, err = scanAssignment(transition.conn.QueryRowContext(ctx, assignmentSelectSQL+` WHERE project_id = ? AND id = ?`, command.ProjectID, current.TargetAssignmentID))
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				err = core.ErrConflict
			}
			return rollback(err)
		}
		if assignment.RoleID != current.ToRoleID || (current.TargetWorkItemID != "" && assignment.WorkItemID != current.TargetWorkItemID) {
			return rollback(core.ErrConflict)
		}
		if _, err := scanWorkItem(transition.conn.QueryRowContext(ctx, `SELECT project_id, id, title, brief, status, priority, owner_role_id, reviewer_role_ids_json, root_id, created_at, updated_at FROM work_items WHERE project_id = ? AND id = ?`, command.ProjectID, assignment.WorkItemID)); err != nil {
			if errors.Is(err, core.ErrNotFound) {
				err = core.ErrConflict
			}
			return rollback(err)
		}
		if current.Status == core.HandoffStatusAccepted && current.TargetWorkItemID == assignment.WorkItemID {
			outcome = core.HandoffFollowUpAlreadySatisfied
		} else {
			outcome = core.HandoffFollowUpLinkedExisting
		}
		current.TargetWorkItemID = assignment.WorkItemID
	} else {
		targetWorkItemID := current.TargetWorkItemID
		if targetWorkItemID == "" {
			targetWorkItemID = current.WorkItemID
		}
		workItem, err := scanWorkItem(transition.conn.QueryRowContext(ctx, `SELECT project_id, id, title, brief, status, priority, owner_role_id, reviewer_role_ids_json, root_id, created_at, updated_at FROM work_items WHERE project_id = ? AND id = ?`, command.ProjectID, targetWorkItemID))
		if err != nil {
			return rollback(err)
		}
		if workItem.RootID != "" {
			project, err := scanProject(transition.conn.QueryRowContext(ctx, `SELECT id, name, description, roots_json, default_root_id, context_sources_json, created_at, updated_at FROM projects WHERE id = ?`, command.ProjectID))
			if err != nil {
				return rollback(err)
			}
			rootFound := false
			for _, root := range project.Roots {
				if root.ID == workItem.RootID {
					rootFound = true
					break
				}
			}
			if !rootFound {
				return rollback(core.ErrNotFound)
			}
		}
		executionMode := role.DefaultExecutionMode
		if executionMode == "" {
			executionMode = core.ExecutionMCPPull
		}
		desiredKind := core.DesiredAgentAny
		if executionMode == core.ExecutionManual {
			desiredKind = "human"
		}
		stamp := time.Now().UTC()
		if now != nil {
			stamp = now()
		}
		assignment = core.Assignment{
			ID:            newAssignmentID,
			ProjectID:     command.ProjectID,
			WorkItemID:    targetWorkItemID,
			RoleID:        current.ToRoleID,
			RootID:        workItem.RootID,
			ExecutionMode: executionMode,
			Status:        core.AssignmentQueued,
			DesiredAgent: core.DesiredAgent{
				Kind:     desiredKind,
				SkillIDs: append([]string(nil), role.DefaultSkillIDs...),
			},
			CreatedAt: stamp,
			UpdatedAt: stamp,
		}
		desiredAgent, err := encodeJSON(assignment.DesiredAgent)
		if err != nil {
			return rollback(err)
		}
		_, err = transition.conn.ExecContext(ctx, `INSERT INTO assignments (project_id, id, work_item_id, role_id, root_id, execution_mode, status, desired_agent_json, claimed_by, execution_ref, context_snapshot_id, created_at, updated_at, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '', '', ?, ?, '', '')`,
			assignment.ProjectID, assignment.ID, assignment.WorkItemID, assignment.RoleID, assignment.RootID, assignment.ExecutionMode, assignment.Status, desiredAgent, encodeTime(assignment.CreatedAt), encodeTime(assignment.UpdatedAt))
		if err != nil {
			return rollback(mapSQLiteWriteError(err))
		}
		current.TargetAssignmentID = assignment.ID
		current.TargetWorkItemID = targetWorkItemID
	}

	if outcome != core.HandoffFollowUpAlreadySatisfied {
		previousStatus := current.Status
		current.Status = core.HandoffStatusAccepted
		current.UpdatedAt = handoffTransitionTime(current, now)
		if previousStatus != current.Status {
			current.StatusChangedAt = current.UpdatedAt
		}
		row, err := updateHandoffRow(ctx, transition, current, encodeTime(command.ExpectedUpdatedAt))
		if err != nil {
			return rollback(err)
		}
		current, err = scanHandoff(row)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				err = core.ErrConflict
			}
			return rollback(mapSQLiteWriteError(err))
		}
	}

	result := core.HandoffFollowUpResult{Handoff: current, Assignment: assignment, Outcome: outcome}
	encodedResult, err := json.Marshal(result)
	if err != nil {
		return rollback(fmt.Errorf("encode handoff command receipt: %w", err))
	}
	createdAt := time.Now().UTC()
	if now != nil {
		createdAt = now()
	}
	if _, err := transition.conn.ExecContext(ctx, `INSERT INTO command_receipts (project_id, operation, idempotency_key, request_hash, response_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`, command.ProjectID, acceptHandoffWithFollowUpOperation, command.IdempotencyKey, requestHash, string(encodedResult), encodeTime(createdAt)); err != nil {
		return rollback(mapSQLiteWriteError(err))
	}
	if err := transition.Commit(); err != nil {
		return core.HandoffFollowUpResult{}, mapSQLiteWriteError(err)
	}
	return result, nil
}

func (s *Store) ListMemoryEntries(ctx context.Context, projectID string, includeDisabled bool) ([]core.MemoryEntry, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	query := `SELECT project_id, id, title, body, trust_label, source_kind, source_id, enabled, created_at, updated_at FROM memory_entries WHERE project_id = ?`
	if !includeDisabled {
		query += ` AND enabled = 1`
	}
	query += ` ORDER BY enabled DESC, updated_at DESC, title ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.MemoryEntry
	for rows.Next() {
		item, err := scanMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetMemoryEntry(ctx context.Context, projectID, id string) (core.MemoryEntry, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return core.MemoryEntry{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT project_id, id, title, body, trust_label, source_kind, source_id, enabled, created_at, updated_at FROM memory_entries WHERE project_id = ? AND id = ?`, projectID, id)
	return scanMemoryEntry(row)
}

func (s *Store) CreateMemoryEntry(ctx context.Context, entry core.MemoryEntry) (core.MemoryEntry, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_entries (project_id, id, title, body, trust_label, source_kind, source_id, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ProjectID, entry.ID, entry.Title, entry.Body, entry.TrustLabel, entry.SourceKind, entry.SourceID, entry.Enabled, encodeTime(entry.CreatedAt), encodeTime(entry.UpdatedAt))
	if err != nil {
		return core.MemoryEntry{}, mapSQLiteWriteError(err)
	}
	return entry, nil
}

func (s *Store) UpdateMemoryEntry(ctx context.Context, entry core.MemoryEntry) (core.MemoryEntry, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE memory_entries SET title = ?, body = ?, trust_label = ?, source_kind = ?, source_id = ?, enabled = ?, created_at = ?, updated_at = ? WHERE project_id = ? AND id = ?`,
		entry.Title, entry.Body, entry.TrustLabel, entry.SourceKind, entry.SourceID, entry.Enabled, encodeTime(entry.CreatedAt), encodeTime(entry.UpdatedAt), entry.ProjectID, entry.ID)
	if err != nil {
		return core.MemoryEntry{}, mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		return core.MemoryEntry{}, err
	}
	return entry, nil
}

func (s *Store) DeleteMemoryEntry(ctx context.Context, projectID, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM memory_entries WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		return mapSQLiteWriteError(err)
	}
	return requireAffected(result)
}

func (s *Store) ListMemoryCandidates(ctx context.Context, filter core.MemoryCandidateFilter) ([]core.MemoryCandidate, error) {
	if err := s.requireProject(ctx, filter.ProjectID); err != nil {
		return nil, err
	}
	query := memoryCandidateSelectSQL + ` WHERE project_id = ?`
	args := []any{filter.ProjectID}
	switch {
	case filter.Status != "":
		query += ` AND status = ?`
		args = append(args, filter.Status)
	case !filter.IncludeResolved:
		query += ` AND status = ?`
		args = append(args, core.MemoryCandidatePending)
	}
	query += ` ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'promoted' THEN 1 WHEN 'rejected' THEN 2 ELSE 3 END ASC, updated_at DESC, title ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.MemoryCandidate
	for rows.Next() {
		item, err := scanMemoryCandidate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetMemoryCandidate(ctx context.Context, projectID, id string) (core.MemoryCandidate, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return core.MemoryCandidate{}, err
	}
	row := s.db.QueryRowContext(ctx, memoryCandidateSelectSQL+` WHERE project_id = ? AND id = ?`, projectID, id)
	return scanMemoryCandidate(row)
}

func (s *Store) CreateMemoryCandidate(ctx context.Context, candidate core.MemoryCandidate) (core.MemoryCandidate, error) {
	sourceRefs, err := encodeJSON(candidate.SourceRefs)
	if err != nil {
		return core.MemoryCandidate{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO memory_candidates (project_id, id, title, body, suggested_kind, suggested_trust_label, suggested_source_kind, suggested_source_id, source_refs_json, status, status_reason, promoted_memory_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		candidate.ProjectID, candidate.ID, candidate.Title, candidate.Body, candidate.SuggestedKind, candidate.SuggestedTrustLabel, candidate.SuggestedSourceKind, candidate.SuggestedSourceID, sourceRefs, candidate.Status, candidate.StatusReason, candidate.PromotedMemoryID, encodeTime(candidate.CreatedAt), encodeTime(candidate.UpdatedAt))
	if err != nil {
		return core.MemoryCandidate{}, mapSQLiteWriteError(err)
	}
	return candidate, nil
}

func (s *Store) UpdateMemoryCandidate(ctx context.Context, candidate core.MemoryCandidate) (core.MemoryCandidate, error) {
	sourceRefs, err := encodeJSON(candidate.SourceRefs)
	if err != nil {
		return core.MemoryCandidate{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE memory_candidates SET title = ?, body = ?, suggested_kind = ?, suggested_trust_label = ?, suggested_source_kind = ?, suggested_source_id = ?, source_refs_json = ?, status = ?, status_reason = ?, promoted_memory_id = ?, created_at = ?, updated_at = ? WHERE project_id = ? AND id = ?`,
		candidate.Title, candidate.Body, candidate.SuggestedKind, candidate.SuggestedTrustLabel, candidate.SuggestedSourceKind, candidate.SuggestedSourceID, sourceRefs, candidate.Status, candidate.StatusReason, candidate.PromotedMemoryID, encodeTime(candidate.CreatedAt), encodeTime(candidate.UpdatedAt), candidate.ProjectID, candidate.ID)
	if err != nil {
		return core.MemoryCandidate{}, mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		return core.MemoryCandidate{}, err
	}
	return candidate, nil
}

func (s *Store) DeleteMemoryCandidate(ctx context.Context, projectID, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM memory_candidates WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		return mapSQLiteWriteError(err)
	}
	return requireAffected(result)
}

func (s *Store) PromoteMemoryCandidate(ctx context.Context, projectID, id string, entry core.MemoryEntry) (core.MemoryCandidate, core.MemoryEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.MemoryCandidate{}, core.MemoryEntry{}, err
	}
	defer tx.Rollback()

	candidate, err := scanMemoryCandidate(tx.QueryRowContext(ctx, memoryCandidateSelectSQL+` WHERE project_id = ? AND id = ?`, projectID, id))
	if err != nil {
		return core.MemoryCandidate{}, core.MemoryEntry{}, err
	}
	if candidate.Status == core.MemoryCandidatePromoted && candidate.PromotedMemoryID != "" {
		promoted, err := scanMemoryEntry(tx.QueryRowContext(ctx, `SELECT project_id, id, title, body, trust_label, source_kind, source_id, enabled, created_at, updated_at FROM memory_entries WHERE project_id = ? AND id = ?`, projectID, candidate.PromotedMemoryID))
		if err != nil {
			return core.MemoryCandidate{}, core.MemoryEntry{}, err
		}
		if err := tx.Commit(); err != nil {
			return core.MemoryCandidate{}, core.MemoryEntry{}, err
		}
		return candidate, promoted, nil
	}
	if candidate.Status != core.MemoryCandidatePending {
		return core.MemoryCandidate{}, core.MemoryEntry{}, core.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO memory_entries (project_id, id, title, body, trust_label, source_kind, source_id, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ProjectID, entry.ID, entry.Title, entry.Body, entry.TrustLabel, entry.SourceKind, entry.SourceID, entry.Enabled, encodeTime(entry.CreatedAt), encodeTime(entry.UpdatedAt))
	if err != nil {
		return core.MemoryCandidate{}, core.MemoryEntry{}, mapSQLiteWriteError(err)
	}
	candidate.Status = core.MemoryCandidatePromoted
	candidate.StatusReason = ""
	candidate.PromotedMemoryID = entry.ID
	candidate.UpdatedAt = entry.UpdatedAt
	result, err := tx.ExecContext(ctx, `UPDATE memory_candidates SET status = ?, status_reason = '', promoted_memory_id = ?, updated_at = ? WHERE project_id = ? AND id = ?`,
		candidate.Status, candidate.PromotedMemoryID, encodeTime(candidate.UpdatedAt), projectID, id)
	if err != nil {
		return core.MemoryCandidate{}, core.MemoryEntry{}, mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		return core.MemoryCandidate{}, core.MemoryEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.MemoryCandidate{}, core.MemoryEntry{}, err
	}
	return candidate, entry, nil
}

func (s *Store) ListAssistantProposals(ctx context.Context, projectID string) ([]core.AssistantProposalRecord, error) {
	query := assistantProposalSelectSQL
	var args []any
	if projectID != "" {
		query += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.AssistantProposalRecord
	for rows.Next() {
		item, err := scanAssistantProposalRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetAssistantProposal(ctx context.Context, id string) (core.AssistantProposalRecord, error) {
	row := s.db.QueryRowContext(ctx, assistantProposalSelectSQL+` WHERE id = ?`, id)
	return scanAssistantProposalRecord(row)
}

func (s *Store) CreateAssistantProposal(ctx context.Context, record core.AssistantProposalRecord) (core.AssistantProposalRecord, error) {
	proposal, latestResult, attempts, err := encodeAssistantProposalRecordJSON(record)
	if err != nil {
		return core.AssistantProposalRecord{}, err
	}
	appliedAt := ""
	if record.AppliedAt != nil {
		appliedAt = encodeTime(*record.AppliedAt)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO assistant_proposals (id, project_id, source, source_id, proposal_json, status, latest_result_json, apply_attempts_json, created_at, updated_at, applied_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.ProjectID, record.Source, record.SourceID, proposal, record.Status, latestResult, attempts, encodeTime(record.CreatedAt), encodeTime(record.UpdatedAt), appliedAt)
	if err != nil {
		return core.AssistantProposalRecord{}, mapSQLiteWriteError(err)
	}
	return record, nil
}

func (s *Store) UpdateAssistantProposal(ctx context.Context, record core.AssistantProposalRecord) (core.AssistantProposalRecord, error) {
	proposal, latestResult, attempts, err := encodeAssistantProposalRecordJSON(record)
	if err != nil {
		return core.AssistantProposalRecord{}, err
	}
	appliedAt := ""
	if record.AppliedAt != nil {
		appliedAt = encodeTime(*record.AppliedAt)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE assistant_proposals SET project_id = ?, source = ?, source_id = ?, proposal_json = ?, status = ?, latest_result_json = ?, apply_attempts_json = ?, created_at = ?, updated_at = ?, applied_at = ? WHERE id = ?`,
		record.ProjectID, record.Source, record.SourceID, proposal, record.Status, latestResult, attempts, encodeTime(record.CreatedAt), encodeTime(record.UpdatedAt), appliedAt, record.ID)
	if err != nil {
		return core.AssistantProposalRecord{}, mapSQLiteWriteError(err)
	}
	if err := requireAffected(result); err != nil {
		return core.AssistantProposalRecord{}, err
	}
	return record, nil
}

const assignmentColumnsSQL = `project_id, id, work_item_id, role_id, root_id, execution_mode, status, desired_agent_json, claimed_by, execution_ref, context_snapshot_id, created_at, updated_at, started_at, completed_at`
const assignmentSelectSQL = `SELECT ` + assignmentColumnsSQL + ` FROM assignments`

const handoffColumnsSQL = `project_id, id, work_item_id, source_assignment_id, source_run_id, source_chat_session_id, source_message_id, from_role_id, to_role_id, target_assignment_id, target_work_item_id, title, body, recommended_next_action, linked_artifact_ids_json, linked_memory_ids_json, context_refs_json, status, provenance_kind, trust_label, created_at, updated_at, status_changed_at`
const handoffSelectSQL = `SELECT ` + handoffColumnsSQL + ` FROM handoffs`

const acceptHandoffWithFollowUpOperation = "accept_handoff_with_follow_up"

const memoryCandidateSelectSQL = `SELECT project_id, id, title, body, suggested_kind, suggested_trust_label, suggested_source_kind, suggested_source_id, source_refs_json, status, status_reason, promoted_memory_id, created_at, updated_at FROM memory_candidates`

const assistantProposalSelectSQL = `SELECT id, project_id, source, source_id, proposal_json, status, latest_result_json, apply_attempts_json, created_at, updated_at, applied_at FROM assistant_proposals`

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (core.Project, error) {
	var item core.Project
	var rootsJSON, sourcesJSON, createdAt, updatedAt string
	if err := row.Scan(&item.ID, &item.Name, &item.Description, &rootsJSON, &item.DefaultRootID, &sourcesJSON, &createdAt, &updatedAt); err != nil {
		return core.Project{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(rootsJSON, &item.Roots); err != nil {
		return core.Project{}, err
	}
	if err := decodeJSON(sourcesJSON, &item.ContextSources); err != nil {
		return core.Project{}, err
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.Project{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.Project{}, err
	}
	return item, nil
}

func scanProjectSkill(row scanner) (core.ProjectSkill, error) {
	var item core.ProjectSkill
	var suggestedToolsJSON, requiredPermissionsJSON, sourceRefsJSON, warningsJSON, discoveredAt, createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.Title, &item.Description, &item.Path, &item.RootID, &item.Format, &suggestedToolsJSON, &requiredPermissionsJSON, &item.Enabled, &item.Status, &item.TrustLabel, &sourceRefsJSON, &warningsJSON, &discoveredAt, &createdAt, &updatedAt); err != nil {
		return core.ProjectSkill{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(suggestedToolsJSON, &item.SuggestedTools); err != nil {
		return core.ProjectSkill{}, err
	}
	if err := decodeJSON(requiredPermissionsJSON, &item.RequiredPermissions); err != nil {
		return core.ProjectSkill{}, err
	}
	if err := decodeJSON(sourceRefsJSON, &item.SourceRefs); err != nil {
		return core.ProjectSkill{}, err
	}
	if err := decodeJSON(warningsJSON, &item.Warnings); err != nil {
		return core.ProjectSkill{}, err
	}
	var err error
	if item.DiscoveredAt, err = decodeOptionalTime(discoveredAt); err != nil {
		return core.ProjectSkill{}, err
	}
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.ProjectSkill{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.ProjectSkill{}, err
	}
	return item, nil
}

func scanWorkItem(row scanner) (core.WorkItem, error) {
	var item core.WorkItem
	var reviewerIDsJSON, createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.Title, &item.Brief, &item.Status, &item.Priority, &item.OwnerRoleID, &reviewerIDsJSON, &item.RootID, &createdAt, &updatedAt); err != nil {
		return core.WorkItem{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(reviewerIDsJSON, &item.ReviewerRoleIDs); err != nil {
		return core.WorkItem{}, err
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.WorkItem{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.WorkItem{}, err
	}
	return item, nil
}

func scanRole(row scanner) (core.Role, error) {
	var item core.Role
	var skillIDsJSON string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.Name, &item.Description, &item.Instructions, &skillIDsJSON, &item.DefaultExecutionMode); err != nil {
		return core.Role{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(skillIDsJSON, &item.DefaultSkillIDs); err != nil {
		return core.Role{}, err
	}
	return item, nil
}

func scanAssignment(row scanner) (core.Assignment, error) {
	var item core.Assignment
	var desiredAgentJSON, executionRef, createdAt, updatedAt, startedAt, completedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.WorkItemID, &item.RoleID, &item.RootID, &item.ExecutionMode, &item.Status, &desiredAgentJSON, &item.ClaimedBy, &executionRef, &item.ContextSnapshotID, &createdAt, &updatedAt, &startedAt, &completedAt); err != nil {
		return core.Assignment{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(desiredAgentJSON, &item.DesiredAgent); err != nil {
		return core.Assignment{}, err
	}
	var err error
	if item.ExecutionRef, err = decodeExecutionRef(executionRef); err != nil {
		return core.Assignment{}, err
	}
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.Assignment{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.Assignment{}, err
	}
	if item.StartedAt, err = decodeOptionalTime(startedAt); err != nil {
		return core.Assignment{}, err
	}
	if item.CompletedAt, err = decodeOptionalTime(completedAt); err != nil {
		return core.Assignment{}, err
	}
	return item, nil
}

func scanArtifact(row scanner) (core.Artifact, error) {
	var item core.Artifact
	var createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.WorkItemID, &item.AssignmentID, &item.Kind, &item.Title, &item.Body, &item.AuthorRoleID, &item.ProvenanceKind, &item.TrustLabel, &createdAt, &updatedAt); err != nil {
		return core.Artifact{}, mapSQLiteReadError(err)
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.Artifact{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.Artifact{}, err
	}
	return item, nil
}

func scanEvidence(row scanner) (core.Evidence, error) {
	var item core.Evidence
	var createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.WorkItemID, &item.AssignmentID, &item.Title, &item.Body, &item.Locator, &item.SourceKind, &item.ExternalID, &item.Provider, &item.TrustLabel, &createdAt, &updatedAt); err != nil {
		return core.Evidence{}, mapSQLiteReadError(err)
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.Evidence{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.Evidence{}, err
	}
	return item, nil
}

func scanReview(row scanner) (core.Review, error) {
	var item core.Review
	var createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.WorkItemID, &item.AssignmentID, &item.ReviewerRoleID, &item.Title, &item.Body, &item.Verdict, &item.Risk, &item.Status, &createdAt, &updatedAt); err != nil {
		return core.Review{}, mapSQLiteReadError(err)
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.Review{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.Review{}, err
	}
	return item, nil
}

func scanHandoff(row scanner) (core.Handoff, error) {
	var item core.Handoff
	var linkedArtifactIDsJSON, linkedMemoryIDsJSON, contextRefsJSON, createdAt, updatedAt, statusChangedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.WorkItemID, &item.SourceAssignmentID, &item.SourceRunID, &item.SourceChatSessionID, &item.SourceMessageID, &item.FromRoleID, &item.ToRoleID, &item.TargetAssignmentID, &item.TargetWorkItemID, &item.Title, &item.Body, &item.RecommendedNextAction, &linkedArtifactIDsJSON, &linkedMemoryIDsJSON, &contextRefsJSON, &item.Status, &item.ProvenanceKind, &item.TrustLabel, &createdAt, &updatedAt, &statusChangedAt); err != nil {
		return core.Handoff{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(linkedArtifactIDsJSON, &item.LinkedArtifactIDs); err != nil {
		return core.Handoff{}, err
	}
	if err := decodeJSON(linkedMemoryIDsJSON, &item.LinkedMemoryIDs); err != nil {
		return core.Handoff{}, err
	}
	if err := decodeJSON(contextRefsJSON, &item.ContextRefs); err != nil {
		return core.Handoff{}, err
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.Handoff{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.Handoff{}, err
	}
	if item.StatusChangedAt, err = decodeOptionalTime(statusChangedAt); err != nil {
		return core.Handoff{}, err
	}
	if item.StatusChangedAt.IsZero() {
		item.StatusChangedAt = item.CreatedAt
	}
	return item, nil
}

func scanMemoryEntry(row scanner) (core.MemoryEntry, error) {
	var item core.MemoryEntry
	var createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.Title, &item.Body, &item.TrustLabel, &item.SourceKind, &item.SourceID, &item.Enabled, &createdAt, &updatedAt); err != nil {
		return core.MemoryEntry{}, mapSQLiteReadError(err)
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.MemoryEntry{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.MemoryEntry{}, err
	}
	return item, nil
}

func scanMemoryCandidate(row scanner) (core.MemoryCandidate, error) {
	var item core.MemoryCandidate
	var sourceRefsJSON, createdAt, updatedAt string
	if err := row.Scan(&item.ProjectID, &item.ID, &item.Title, &item.Body, &item.SuggestedKind, &item.SuggestedTrustLabel, &item.SuggestedSourceKind, &item.SuggestedSourceID, &sourceRefsJSON, &item.Status, &item.StatusReason, &item.PromotedMemoryID, &createdAt, &updatedAt); err != nil {
		return core.MemoryCandidate{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(sourceRefsJSON, &item.SourceRefs); err != nil {
		return core.MemoryCandidate{}, err
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.MemoryCandidate{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.MemoryCandidate{}, err
	}
	return item, nil
}

func scanAssistantProposalRecord(row scanner) (core.AssistantProposalRecord, error) {
	var item core.AssistantProposalRecord
	var proposalJSON, latestResultJSON, attemptsJSON, createdAt, updatedAt, appliedAt string
	if err := row.Scan(&item.ID, &item.ProjectID, &item.Source, &item.SourceID, &proposalJSON, &item.Status, &latestResultJSON, &attemptsJSON, &createdAt, &updatedAt, &appliedAt); err != nil {
		return core.AssistantProposalRecord{}, mapSQLiteReadError(err)
	}
	if err := decodeJSON(proposalJSON, &item.Proposal); err != nil {
		return core.AssistantProposalRecord{}, err
	}
	if latestResultJSON != "" {
		var result core.AssistantApplyResult
		if err := decodeJSON(latestResultJSON, &result); err != nil {
			return core.AssistantProposalRecord{}, err
		}
		item.LatestResult = &result
	}
	if err := decodeJSON(attemptsJSON, &item.ApplyAttempts); err != nil {
		return core.AssistantProposalRecord{}, err
	}
	var err error
	if item.CreatedAt, err = decodeTime(createdAt); err != nil {
		return core.AssistantProposalRecord{}, err
	}
	if item.UpdatedAt, err = decodeTime(updatedAt); err != nil {
		return core.AssistantProposalRecord{}, err
	}
	if appliedAt != "" {
		stamp, err := decodeTime(appliedAt)
		if err != nil {
			return core.AssistantProposalRecord{}, err
		}
		item.AppliedAt = &stamp
	}
	return item, nil
}

func (s *Store) requireProject(ctx context.Context, projectID string) error {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id = ?`, projectID).Scan(&id)
	return mapSQLiteReadError(err)
}

func (s *Store) requireWorkItem(ctx context.Context, projectID, workItemID string) error {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM work_items WHERE project_id = ? AND id = ?`, projectID, workItemID).Scan(&id)
	return mapSQLiteReadError(err)
}

func (s *Store) requireRole(ctx context.Context, projectID, roleID string) error {
	if roleID == "" {
		return nil
	}
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM roles WHERE project_id = ? AND id = ?`, projectID, roleID).Scan(&id)
	return mapSQLiteReadError(err)
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func encodeJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func encodeAssistantProposalRecordJSON(record core.AssistantProposalRecord) (string, string, string, error) {
	proposal, err := encodeJSON(record.Proposal)
	if err != nil {
		return "", "", "", err
	}
	latestResult := ""
	if record.LatestResult != nil {
		latestResult, err = encodeJSON(record.LatestResult)
		if err != nil {
			return "", "", "", err
		}
	}
	attempts, err := encodeJSON(record.ApplyAttempts)
	if err != nil {
		return "", "", "", err
	}
	return proposal, latestResult, attempts, nil
}

func decodeJSON(raw string, target any) error {
	if raw == "" {
		raw = "null"
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("decode sqlite json: %w", err)
	}
	return nil
}

// encodeExecutionRef stores an empty ref as the empty string so the release path — which
// resets execution_ref with plain SQL, no Go encoding — and structured writes
// agree on what "no execution" looks like in the column.
func encodeExecutionRef(ref core.ExecutionRef) (string, error) {
	if ref.Empty() {
		return "", nil
	}
	return encodeJSON(ref)
}

// decodeExecutionRef reads the JSON-object execution_ref column. Rows written
// before the ref was structured held one opaque host string; those fail
// decode on purpose — this contract broke in alpha, and a pre-structured
// store must be rebuilt or re-seeded from the host's authoritative data
// rather than silently reinterpreted.
func decodeExecutionRef(raw string) (core.ExecutionRef, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return core.ExecutionRef{}, nil
	}
	var ref core.ExecutionRef
	if err := json.Unmarshal([]byte(trimmed), &ref); err != nil {
		return core.ExecutionRef{}, fmt.Errorf("decode sqlite execution_ref: pre-structured execution refs are unsupported, rebuild or re-seed this store from the host's authoritative data: %w", err)
	}
	return ref, nil
}

func encodeTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func encodeOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return encodeTime(value)
}

func decodeTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("decode sqlite time: %w", err)
	}
	return parsed, nil
}

func decodeOptionalTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return decodeTime(value)
}

func mapSQLiteReadError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return core.ErrNotFound
	}
	return err
}

func mapSQLiteWriteError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "UNIQUE constraint failed"):
		return core.ErrDuplicate
	case strings.Contains(message, "FOREIGN KEY constraint failed"):
		return core.ErrNotFound
	case strings.Contains(message, "database is locked"), strings.Contains(message, "database table is locked"), strings.Contains(message, "SQLITE_BUSY"), strings.Contains(message, "SQLITE_LOCKED"):
		return core.ErrConflict
	default:
		return err
	}
}
