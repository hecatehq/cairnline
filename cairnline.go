// Package cairnline exposes the embeddable project coordination core.
//
// The root package is the public API intended for applications such as Hecate.
// Transport details, MCP wiring, and concrete storage internals remain in
// internal packages.
package cairnline

import (
	"context"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/sqlitestore"
)

var (
	ErrNotFound  = core.ErrNotFound
	ErrInvalid   = core.ErrInvalid
	ErrDuplicate = core.ErrDuplicate
	ErrConflict  = core.ErrConflict
)

type Store = core.Store
type Service = core.Service
type MemoryStore = core.MemoryStore
type SQLiteStore = sqlitestore.Store

type Project = core.Project
type Root = core.Root
type Source = core.Source
type ProjectSkill = core.ProjectSkill
type Role = core.Role
type AgentProfile = core.AgentProfile
type ExecutionProfile = core.ExecutionProfile
type WorkItem = core.WorkItem
type DesiredAgent = core.DesiredAgent
type Assignment = core.Assignment
type AssignmentCompatibilityFilter = core.AssignmentCompatibilityFilter
type AssignmentContext = core.AssignmentContext
type AssignmentLaunchPacket = core.AssignmentLaunchPacket
type Evidence = core.Evidence
type Review = core.Review
type Handoff = core.Handoff
type MemoryEntry = core.MemoryEntry
type MemoryCandidate = core.MemoryCandidate

const (
	WorkStatusReady = core.WorkStatusReady
	PriorityNormal  = core.PriorityNormal

	ExecutionManual          = core.ExecutionManual
	ExecutionMCPPull         = core.ExecutionMCPPull
	ExecutionExternalAdapter = core.ExecutionExternalAdapter
	ExecutionOrchestrated    = core.ExecutionOrchestrated

	AssignmentQueued    = core.AssignmentQueued
	AssignmentClaimed   = core.AssignmentClaimed
	AssignmentRunning   = core.AssignmentRunning
	AssignmentReview    = core.AssignmentReview
	AssignmentCompleted = core.AssignmentCompleted
	AssignmentFailed    = core.AssignmentFailed
	AssignmentCancelled = core.AssignmentCancelled

	DesiredAgentAny = core.DesiredAgentAny

	SkillFormatMarkdown  = core.SkillFormatMarkdown
	SkillStatusAvailable = core.SkillStatusAvailable
	SkillStatusMissing   = core.SkillStatusMissing
	SkillStatusInvalid   = core.SkillStatusInvalid
	SkillStatusConflict  = core.SkillStatusConflict
	SkillTrustWorkspace  = core.SkillTrustWorkspace
	SkillPathAgents      = core.SkillPathAgents
	SkillPathCairnline   = core.SkillPathCairnline

	EvidenceTrustOperator = core.EvidenceTrustOperator

	ReviewVerdictPass     = core.ReviewVerdictPass
	ReviewVerdictConcerns = core.ReviewVerdictConcerns
	ReviewVerdictBlocked  = core.ReviewVerdictBlocked
	ReviewRiskLow         = core.ReviewRiskLow
	ReviewRiskMedium      = core.ReviewRiskMedium
	ReviewRiskHigh        = core.ReviewRiskHigh
	ReviewStatusRecorded  = core.ReviewStatusRecorded

	HandoffStatusOpen = core.HandoffStatusOpen

	MemoryTrustOperator   = core.MemoryTrustOperator
	MemoryTrustGenerated  = core.MemoryTrustGenerated
	MemorySourceOperator  = core.MemorySourceOperator
	MemorySourceGenerated = core.MemorySourceGenerated

	MemoryCandidateProposed = core.MemoryCandidateProposed

	LaunchPacketKindAssignment = core.LaunchPacketKindAssignment
)

func NewService(store Store) *Service {
	return core.NewService(store)
}

func NewMemoryStore() *MemoryStore {
	return core.NewMemoryStore()
}

func NewMemoryService() *Service {
	return NewService(NewMemoryStore())
}

func OpenSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	return sqlitestore.Open(ctx, path)
}

func NewSQLiteService(ctx context.Context, path string) (*Service, *SQLiteStore, error) {
	store, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	return NewService(store), store, nil
}
