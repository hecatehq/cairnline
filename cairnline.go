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
type RequiredPermissions = core.RequiredPermissions
type Role = core.Role
type ExecutionProfile = core.ExecutionProfile
type WorkItem = core.WorkItem
type WorkItemCloseoutReadiness = core.WorkItemCloseoutReadiness
type ProjectSetupReadiness = core.ProjectSetupReadiness
type ProjectSetupReadinessSummary = core.ProjectSetupReadinessSummary
type ProjectSetupReadinessCheck = core.ProjectSetupReadinessCheck
type ProjectSetupReadinessAction = core.ProjectSetupReadinessAction
type ProjectHealth = core.ProjectHealth
type ProjectHealthSummary = core.ProjectHealthSummary
type ProjectHealthAttentionItem = core.ProjectHealthAttentionItem
type AssistantProposal = core.AssistantProposal
type AssistantProposalRecord = core.AssistantProposalRecord
type AssistantApplyAttempt = core.AssistantApplyAttempt
type AssistantAction = core.AssistantAction
type AssistantTarget = core.AssistantTarget
type AssistantApplyResult = core.AssistantApplyResult
type AssistantActionResult = core.AssistantActionResult
type ProjectOperationsBrief = core.ProjectOperationsBrief
type ProjectOperationsCounts = core.ProjectOperationsCounts
type ProjectOperationItem = core.ProjectOperationItem
type ProjectActivity = core.ProjectActivity
type ProjectActivityCounts = core.ProjectActivityCounts
type ProjectActivityBuckets = core.ProjectActivityBuckets
type ProjectActivityItem = core.ProjectActivityItem
type ReviewFollowUpReadiness = core.ReviewFollowUpReadiness
type DesiredAgent = core.DesiredAgent
type Assignment = core.Assignment
type AssignmentCompatibilityFilter = core.AssignmentCompatibilityFilter
type AssignmentContext = core.AssignmentContext
type AssignmentLaunchPacket = core.AssignmentLaunchPacket
type Artifact = core.Artifact
type Evidence = core.Evidence
type Review = core.Review
type Handoff = core.Handoff
type MemoryEntry = core.MemoryEntry
type MemoryCandidate = core.MemoryCandidate
type MemoryCandidateSourceRef = core.MemoryCandidateSourceRef
type MemoryCandidateFilter = core.MemoryCandidateFilter
type MemoryCandidatePromotion = core.MemoryCandidatePromotion
type Snapshot = core.Snapshot

const (
	SnapshotVersion = core.SnapshotVersion

	WorkStatusReady = core.WorkStatusReady
	WorkStatusDone  = core.WorkStatusDone
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
	SkillPathHecate      = core.SkillPathHecate
	SkillPathCairnline   = core.SkillPathCairnline

	EvidenceTrustOperator = core.EvidenceTrustOperator

	ReviewVerdictApproved         = core.ReviewVerdictApproved
	ReviewVerdictChangesRequested = core.ReviewVerdictChangesRequested
	ReviewVerdictBlocked          = core.ReviewVerdictBlocked
	ReviewVerdictRisk             = core.ReviewVerdictRisk
	ReviewVerdictPass             = core.ReviewVerdictPass
	ReviewVerdictConcerns         = core.ReviewVerdictConcerns
	ReviewRiskLow                 = core.ReviewRiskLow
	ReviewRiskMedium              = core.ReviewRiskMedium
	ReviewRiskHigh                = core.ReviewRiskHigh
	ReviewRiskUnknown             = core.ReviewRiskUnknown
	ReviewStatusRecorded          = core.ReviewStatusRecorded

	HandoffStatusOpen       = core.HandoffStatusOpen
	HandoffStatusAccepted   = core.HandoffStatusAccepted
	HandoffStatusSuperseded = core.HandoffStatusSuperseded
	HandoffStatusDismissed  = core.HandoffStatusDismissed

	MemoryTrustOperator   = core.MemoryTrustOperator
	MemoryTrustGenerated  = core.MemoryTrustGenerated
	MemorySourceOperator  = core.MemorySourceOperator
	MemorySourceGenerated = core.MemorySourceGenerated

	MemoryCandidatePending  = core.MemoryCandidatePending
	MemoryCandidatePromoted = core.MemoryCandidatePromoted
	MemoryCandidateRejected = core.MemoryCandidateRejected
	MemoryCandidateProposed = core.MemoryCandidateProposed

	ProjectOperationsStatusClear     = core.ProjectOperationsStatusClear
	ProjectOperationsStatusAttention = core.ProjectOperationsStatusAttention

	ProjectSetupStatusReady    = core.ProjectSetupStatusReady
	ProjectSetupStatusTodo     = core.ProjectSetupStatusTodo
	ProjectSetupStatusOptional = core.ProjectSetupStatusOptional

	ProjectSetupActionSetupProject   = core.ProjectSetupActionSetupProject
	ProjectSetupActionCreateWorkItem = core.ProjectSetupActionCreateWorkItem
	ProjectSetupActionUpdateProject  = core.ProjectSetupActionUpdateProject
	ProjectSetupActionManageContext  = core.ProjectSetupActionManageContext
	ProjectSetupActionManageRoles    = core.ProjectSetupActionManageRoles

	ProjectHealthStatusClear     = core.ProjectHealthStatusClear
	ProjectHealthStatusAttention = core.ProjectHealthStatusAttention

	AssistantProposalSourceAPI       = core.AssistantProposalSourceAPI
	AssistantProposalSourceAssistant = core.AssistantProposalSourceAssistant

	AssistantProposalStatusProposed     = core.AssistantProposalStatusProposed
	AssistantProposalStatusNeedsConfirm = core.AssistantProposalStatusNeedsConfirm
	AssistantProposalStatusApplied      = core.AssistantProposalStatusApplied
	AssistantProposalStatusPartial      = core.AssistantProposalStatusPartial
	AssistantProposalStatusRejected     = core.AssistantProposalStatusRejected

	AssistantActionCreateProject         = core.AssistantActionCreateProject
	AssistantActionUpdateProject         = core.AssistantActionUpdateProject
	AssistantActionAttachProjectRoot     = core.AssistantActionAttachProjectRoot
	AssistantActionRemoveProjectRoot     = core.AssistantActionRemoveProjectRoot
	AssistantActionSetProjectDefaults    = core.AssistantActionSetProjectDefaults
	AssistantActionCreateRole            = core.AssistantActionCreateRole
	AssistantActionUpdateRole            = core.AssistantActionUpdateRole
	AssistantActionCreateWorkItem        = core.AssistantActionCreateWorkItem
	AssistantActionUpdateWorkItem        = core.AssistantActionUpdateWorkItem
	AssistantActionCreateAssignment      = core.AssistantActionCreateAssignment
	AssistantActionCreateEvidence        = core.AssistantActionCreateEvidence
	AssistantActionCreateReview          = core.AssistantActionCreateReview
	AssistantActionCreateHandoff         = core.AssistantActionCreateHandoff
	AssistantActionUpdateHandoff         = core.AssistantActionUpdateHandoff
	AssistantActionCreateMemoryCandidate = core.AssistantActionCreateMemoryCandidate

	AssistantApplyStatusApplied      = core.AssistantApplyStatusApplied
	AssistantApplyStatusNeedsConfirm = core.AssistantApplyStatusNeedsConfirm
	AssistantApplyStatusPartial      = core.AssistantApplyStatusPartial
	AssistantApplyStatusRejected     = core.AssistantApplyStatusRejected

	ProjectOperationKindAssignment      = core.ProjectOperationKindAssignment
	ProjectOperationKindCloseoutReady   = core.ProjectOperationKindCloseoutReady
	ProjectOperationKindHandoff         = core.ProjectOperationKindHandoff
	ProjectOperationKindMemoryCandidate = core.ProjectOperationKindMemoryCandidate
	ProjectOperationKindMissingEvidence = core.ProjectOperationKindMissingEvidence
	ProjectOperationKindReviewFollowUp  = core.ProjectOperationKindReviewFollowUp
	ProjectOperationKindProjectSetup    = core.ProjectOperationKindProjectSetup
	ProjectOperationKindProfile         = core.ProjectOperationKindProfile
	ProjectOperationKindSkill           = core.ProjectOperationKindSkill
	ProjectOperationKindWorkItem        = core.ProjectOperationKindWorkItem

	ProjectOperationSeverityBlocked = core.ProjectOperationSeverityBlocked
	ProjectOperationSeverityAction  = core.ProjectOperationSeverityAction
	ProjectOperationSeverityActive  = core.ProjectOperationSeverityActive
	ProjectOperationSeverityReady   = core.ProjectOperationSeverityReady
	ProjectOperationSeverityInfo    = core.ProjectOperationSeverityInfo

	ProjectActivityBucketActive    = core.ProjectActivityBucketActive
	ProjectActivityBucketBlocked   = core.ProjectActivityBucketBlocked
	ProjectActivityBucketCompleted = core.ProjectActivityBucketCompleted
	ProjectActivityBucketOther     = core.ProjectActivityBucketOther

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
