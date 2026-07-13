package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

const SnapshotVersion = 1

// Snapshot is an embeddable export/import boundary for migration rehearsals.
//
// It is intentionally not exposed as an MCP bulk-mutation tool: callers that
// import snapshots have local embedding authority over the store.
type Snapshot struct {
	Version            int                       `json:"version"`
	ExportedAt         time.Time                 `json:"exported_at"`
	Projects           []Project                 `json:"projects,omitempty"`
	ProjectSkills      []ProjectSkill            `json:"project_skills,omitempty"`
	Roles              []Role                    `json:"roles,omitempty"`
	WorkItems          []WorkItem                `json:"work_items,omitempty"`
	Assignments        []Assignment              `json:"assignments,omitempty"`
	Artifacts          []Artifact                `json:"artifacts,omitempty"`
	Evidence           []Evidence                `json:"evidence,omitempty"`
	Reviews            []Review                  `json:"reviews,omitempty"`
	Handoffs           []Handoff                 `json:"handoffs,omitempty"`
	MemoryEntries      []MemoryEntry             `json:"memory_entries,omitempty"`
	MemoryCandidates   []MemoryCandidate         `json:"memory_candidates,omitempty"`
	AssistantProposals []AssistantProposalRecord `json:"assistant_proposals,omitempty"`
}

func (s *Service) ExportSnapshot(ctx context.Context) (Snapshot, error) {
	snapshot := Snapshot{
		Version:    SnapshotVersion,
		ExportedAt: s.now(),
	}
	var err error
	if snapshot.Projects, err = s.store.ListProjects(ctx); err != nil {
		return Snapshot{}, err
	}
	if snapshot.AssistantProposals, err = s.store.ListAssistantProposals(ctx, ""); err != nil {
		return Snapshot{}, err
	}
	for _, project := range snapshot.Projects {
		skills, err := s.store.ListProjectSkills(ctx, project.ID)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.ProjectSkills = append(snapshot.ProjectSkills, skills...)
		roles, err := s.store.ListRoles(ctx, project.ID)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Roles = append(snapshot.Roles, roles...)
		workItems, err := s.store.ListWorkItems(ctx, project.ID)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.WorkItems = append(snapshot.WorkItems, workItems...)
		assignments, err := s.store.ListAssignments(ctx, project.ID)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Assignments = append(snapshot.Assignments, assignments...)
		memoryEntries, err := s.store.ListMemoryEntries(ctx, project.ID, true)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.MemoryEntries = append(snapshot.MemoryEntries, memoryEntries...)
		memoryCandidates, err := s.store.ListMemoryCandidates(ctx, MemoryCandidateFilter{ProjectID: project.ID, IncludeResolved: true})
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.MemoryCandidates = append(snapshot.MemoryCandidates, memoryCandidates...)
		for _, workItem := range workItems {
			artifacts, err := s.store.ListArtifacts(ctx, project.ID, workItem.ID)
			if err != nil {
				return Snapshot{}, err
			}
			snapshot.Artifacts = append(snapshot.Artifacts, artifacts...)
			evidence, err := s.store.ListEvidence(ctx, project.ID, workItem.ID)
			if err != nil {
				return Snapshot{}, err
			}
			snapshot.Evidence = append(snapshot.Evidence, evidence...)
			reviews, err := s.store.ListReviews(ctx, project.ID, workItem.ID)
			if err != nil {
				return Snapshot{}, err
			}
			snapshot.Reviews = append(snapshot.Reviews, reviews...)
			handoffs, err := s.store.ListHandoffs(ctx, project.ID, workItem.ID)
			if err != nil {
				return Snapshot{}, err
			}
			snapshot.Handoffs = append(snapshot.Handoffs, handoffs...)
		}
	}
	sortSnapshot(&snapshot)
	return snapshot, nil
}

func (s *Service) ImportSnapshot(ctx context.Context, snapshot Snapshot) (Snapshot, error) {
	if snapshot.Version != SnapshotVersion {
		return Snapshot{}, errors.Join(ErrInvalid, fmt.Errorf("snapshot version %d is unsupported", snapshot.Version))
	}
	for _, item := range snapshot.Projects {
		if err := upsertProject(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.ProjectSkills {
		if err := upsertProjectSkill(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.Roles {
		if err := upsertRole(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.WorkItems {
		if err := upsertWorkItem(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.Assignments {
		if err := upsertAssignment(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.Artifacts {
		if err := createIfMissingArtifact(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.Evidence {
		if err := createIfMissingEvidence(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.Reviews {
		if err := createIfMissingReview(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.Handoffs {
		if err := upsertHandoff(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.MemoryEntries {
		if err := upsertMemoryEntry(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.MemoryCandidates {
		if err := upsertMemoryCandidate(ctx, s.store, item); err != nil {
			return Snapshot{}, err
		}
	}
	for _, item := range snapshot.AssistantProposals {
		if _, err := s.ImportAssistantProposalRecord(ctx, item); err != nil {
			return Snapshot{}, err
		}
	}
	return s.ExportSnapshot(ctx)
}

func sortSnapshot(snapshot *Snapshot) {
	sort.Slice(snapshot.Projects, func(i, j int) bool {
		return snapshot.Projects[i].ID < snapshot.Projects[j].ID
	})
	sort.Slice(snapshot.ProjectSkills, func(i, j int) bool {
		a, b := snapshot.ProjectSkills[i], snapshot.ProjectSkills[j]
		return a.ProjectID+"/"+a.ID < b.ProjectID+"/"+b.ID
	})
	sort.Slice(snapshot.Roles, func(i, j int) bool {
		a, b := snapshot.Roles[i], snapshot.Roles[j]
		return a.ProjectID+"/"+a.ID < b.ProjectID+"/"+b.ID
	})
	sort.Slice(snapshot.WorkItems, func(i, j int) bool {
		a, b := snapshot.WorkItems[i], snapshot.WorkItems[j]
		return a.ProjectID+"/"+a.ID < b.ProjectID+"/"+b.ID
	})
	sort.Slice(snapshot.Assignments, func(i, j int) bool {
		a, b := snapshot.Assignments[i], snapshot.Assignments[j]
		return a.ProjectID+"/"+a.ID < b.ProjectID+"/"+b.ID
	})
	sort.Slice(snapshot.Artifacts, func(i, j int) bool {
		a, b := snapshot.Artifacts[i], snapshot.Artifacts[j]
		return a.ProjectID+"/"+a.WorkItemID+"/"+a.ID < b.ProjectID+"/"+b.WorkItemID+"/"+b.ID
	})
	sort.Slice(snapshot.Evidence, func(i, j int) bool {
		a, b := snapshot.Evidence[i], snapshot.Evidence[j]
		return a.ProjectID+"/"+a.WorkItemID+"/"+a.ID < b.ProjectID+"/"+b.WorkItemID+"/"+b.ID
	})
	sort.Slice(snapshot.Reviews, func(i, j int) bool {
		a, b := snapshot.Reviews[i], snapshot.Reviews[j]
		return a.ProjectID+"/"+a.WorkItemID+"/"+a.ID < b.ProjectID+"/"+b.WorkItemID+"/"+b.ID
	})
	sort.Slice(snapshot.Handoffs, func(i, j int) bool {
		a, b := snapshot.Handoffs[i], snapshot.Handoffs[j]
		return a.ProjectID+"/"+a.WorkItemID+"/"+a.ID < b.ProjectID+"/"+b.WorkItemID+"/"+b.ID
	})
	sort.Slice(snapshot.MemoryEntries, func(i, j int) bool {
		a, b := snapshot.MemoryEntries[i], snapshot.MemoryEntries[j]
		return a.ProjectID+"/"+a.ID < b.ProjectID+"/"+b.ID
	})
	sort.Slice(snapshot.MemoryCandidates, func(i, j int) bool {
		a, b := snapshot.MemoryCandidates[i], snapshot.MemoryCandidates[j]
		return a.ProjectID+"/"+a.ID < b.ProjectID+"/"+b.ID
	})
	sort.Slice(snapshot.AssistantProposals, func(i, j int) bool {
		return snapshot.AssistantProposals[i].ID < snapshot.AssistantProposals[j].ID
	})
}

func upsertProject(ctx context.Context, store Store, item Project) error {
	if _, err := store.CreateProject(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateProject(ctx, item)
		}
		return err
	}
	return nil
}

func upsertProjectSkill(ctx context.Context, store Store, item ProjectSkill) error {
	if _, err := store.CreateProjectSkill(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateProjectSkill(ctx, item)
		}
		return err
	}
	return nil
}

func upsertRole(ctx context.Context, store Store, item Role) error {
	if _, err := store.CreateRole(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateRole(ctx, item)
		}
		return err
	}
	return nil
}

func upsertWorkItem(ctx context.Context, store Store, item WorkItem) error {
	if _, err := store.CreateWorkItem(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateWorkItem(ctx, item)
		}
		return err
	}
	return nil
}

func upsertAssignment(ctx context.Context, store Store, item Assignment) error {
	if _, err := store.CreateAssignment(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.RestoreAssignmentSnapshot(ctx, item)
		}
		return err
	}
	return nil
}

func createIfMissingArtifact(ctx context.Context, store Store, item Artifact) error {
	if _, err := store.CreateArtifact(ctx, item); err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	return nil
}

func createIfMissingEvidence(ctx context.Context, store Store, item Evidence) error {
	if _, err := store.CreateEvidence(ctx, item); err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	return nil
}

func createIfMissingReview(ctx context.Context, store Store, item Review) error {
	if _, err := store.CreateReview(ctx, item); err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	return nil
}

func upsertHandoff(ctx context.Context, store Store, item Handoff) error {
	if _, err := store.CreateHandoff(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateHandoff(ctx, item)
		}
		return err
	}
	return nil
}

func upsertMemoryEntry(ctx context.Context, store Store, item MemoryEntry) error {
	if _, err := store.CreateMemoryEntry(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateMemoryEntry(ctx, item)
		}
		return err
	}
	return nil
}

func upsertMemoryCandidate(ctx context.Context, store Store, item MemoryCandidate) error {
	if _, err := store.CreateMemoryCandidate(ctx, item); err != nil {
		if errors.Is(err, ErrDuplicate) {
			_, err = store.UpdateMemoryCandidate(ctx, item)
		}
		return err
	}
	return nil
}
